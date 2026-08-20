#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${NAMESPACE:-c2c-monitor}"
DOMAIN="${DOMAIN:-c2c.agenticim.xyz}"
K8S_DIR="${K8S_DIR:-deploy/k8s}"
SECRETS_FILE="${SECRETS_FILE:-/opt/c2c-monitor/secrets.env}"
KUBECTL="${KUBECTL:-kubectl}"
CONFIG_FILE=""
RENDERED_FILE=""

cleanup() {
  if [[ -n "$CONFIG_FILE" ]]; then
    rm -f "$CONFIG_FILE"
  fi
  if [[ -n "$RENDERED_FILE" ]]; then
    rm -f "$RENDERED_FILE"
  fi
}

require() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "missing required command: $1" >&2
    exit 1
  }
}

random_secret() {
  openssl rand -hex 24
}

write_secret_value() {
  printf '%s=%q\n' "$1" "$2"
}

initialize_secrets() {
  if [[ -f "$SECRETS_FILE" ]]; then
    return
  fi

  echo "creating first-run secrets at $SECRETS_FILE"
  install -d -m 0700 "$(dirname "$SECRETS_FILE")"

  local smtp_host_value="${SMTP_HOST:-127.0.0.1}"
  local smtp_port_value="${SMTP_PORT:-587}"
  local smtp_username_value="${SMTP_USERNAME:-disabled}"
  local smtp_password_value="${SMTP_PASSWORD:-$(random_secret)}"
  local smtp_from_value="${SMTP_FROM:-c2c-monitor@agenticim.xyz}"
  local smtp_to_value="${SMTP_TO:-disabled@agenticim.xyz}"
  local smtp_configured_value="false"
  if [[ -n "${SMTP_HOST:-}" && -n "${SMTP_USERNAME:-}" && -n "${SMTP_PASSWORD:-}" && -n "${SMTP_FROM:-}" && -n "${SMTP_TO:-}" ]]; then
    smtp_configured_value="true"
  fi

  umask 077
  {
    write_secret_value MYSQL_ROOT_PASSWORD "$(random_secret)"
    write_secret_value MYSQL_USER "c2c_monitor"
    write_secret_value MYSQL_PASSWORD "$(random_secret)"
    write_secret_value C2C_APP_ADMIN_TOKEN "$(random_secret)"
    write_secret_value SMTP_HOST "$smtp_host_value"
    write_secret_value SMTP_PORT "$smtp_port_value"
    write_secret_value SMTP_USERNAME "$smtp_username_value"
    write_secret_value SMTP_PASSWORD "$smtp_password_value"
    write_secret_value SMTP_FROM "$smtp_from_value"
    write_secret_value SMTP_TO "$smtp_to_value"
    write_secret_value SMTP_CONFIGURED "$smtp_configured_value"
  } >"$SECRETS_FILE"
  chmod 0600 "$SECRETS_FILE"
}

load_and_validate_secrets() {
  set -a
  # shellcheck disable=SC1090
  source "$SECRETS_FILE"
  set +a
  SMTP_CONFIGURED="${SMTP_CONFIGURED:-false}"

  local required=(
    MYSQL_ROOT_PASSWORD
    MYSQL_USER
    MYSQL_PASSWORD
    C2C_APP_ADMIN_TOKEN
    SMTP_HOST
    SMTP_PORT
    SMTP_USERNAME
    SMTP_PASSWORD
    SMTP_FROM
    SMTP_TO
  )
  local name
  for name in "${required[@]}"; do
    if [[ -z "${!name:-}" ]]; then
      echo "$name is missing in $SECRETS_FILE" >&2
      exit 1
    fi
  done
}

create_runtime_secrets() {
  CONFIG_FILE="$(mktemp)"
  SMTP_TO="$SMTP_TO" SMTP_CONFIGURED="$SMTP_CONFIGURED" python3 - "$CONFIG_FILE" <<'PY'
import json
import os
import sys

recipient = os.environ["SMTP_TO"].strip()
enabled = os.environ.get("SMTP_CONFIGURED", "false").lower() == "true"
config = f"""app:
  port: 8001
  admin_token: ""
  allowed_origins: []

monitor:
  c2c_interval_minutes: 6
  forex_interval_hours: 1
  forex_max_age_hours: 6
  target_amounts: [0, 30, 50, 200, 500, 1000]
  exchanges: ["Binance", "Gate", "OKX"]

database:
  dsn: ""

notification:
  email:
    enabled: {str(enabled).lower()}
    smtp_host: ""
    smtp_port: 587
    username: ""
    password: ""
    from: ""
    to: {json.dumps([recipient])}
"""
with open(sys.argv[1], "w", encoding="utf-8") as handle:
    handle.write(config)
PY

  local dsn
  dsn="${MYSQL_USER}:${MYSQL_PASSWORD}@tcp(mysql:3306)/c2c_monitor?charset=utf8mb4&parseTime=True&loc=UTC"

  "$KUBECTL" -n "$NAMESPACE" create secret generic c2c-monitor-runtime \
    --from-literal=MYSQL_ROOT_PASSWORD="$MYSQL_ROOT_PASSWORD" \
    --from-literal=MYSQL_USER="$MYSQL_USER" \
    --from-literal=MYSQL_PASSWORD="$MYSQL_PASSWORD" \
    --from-literal=C2C_APP_ADMIN_TOKEN="$C2C_APP_ADMIN_TOKEN" \
    --from-literal=C2C_DATABASE_DSN="$dsn" \
    --from-literal=SMTP_HOST="$SMTP_HOST" \
    --from-literal=SMTP_PORT="$SMTP_PORT" \
    --from-literal=SMTP_USERNAME="$SMTP_USERNAME" \
    --from-literal=SMTP_PASSWORD="$SMTP_PASSWORD" \
    --from-literal=SMTP_FROM="$SMTP_FROM" \
    --dry-run=client -o yaml | "$KUBECTL" apply -f -

  "$KUBECTL" -n "$NAMESPACE" create secret generic c2c-monitor-config \
    --from-file=config.yaml="$CONFIG_FILE" \
    --dry-run=client -o yaml | "$KUBECTL" apply -f -
  rm -f "$CONFIG_FILE"
  CONFIG_FILE=""
}

create_image_pull_secret() {
  if [[ -n "${GHCR_USERNAME:-}" && -n "${GHCR_TOKEN:-}" ]]; then
    "$KUBECTL" -n "$NAMESPACE" create secret docker-registry ghcr-pull-secret \
      --docker-server=ghcr.io \
      --docker-username="$GHCR_USERNAME" \
      --docker-password="$GHCR_TOKEN" \
      --dry-run=client -o yaml | "$KUBECTL" apply -f -
    return
  fi

  if "$KUBECTL" -n agents-im get secret ghcr-pull-secret >/dev/null 2>&1; then
    "$KUBECTL" -n agents-im get secret ghcr-pull-secret -o json | \
      NAMESPACE="$NAMESPACE" python3 -c '
import json
import os
import sys

secret = json.load(sys.stdin)
metadata = secret["metadata"]
for key in ("annotations", "creationTimestamp", "managedFields", "ownerReferences", "resourceVersion", "uid"):
    metadata.pop(key, None)
metadata["name"] = "ghcr-pull-secret"
metadata["namespace"] = os.environ["NAMESPACE"]
print(json.dumps(secret))
' | "$KUBECTL" apply -f -
    return
  fi

  echo "GHCR credentials are unavailable and agents-im/ghcr-pull-secret does not exist" >&2
  exit 1
}

render_manifests() {
  local output="$1"
  "$KUBECTL" kustomize "$K8S_DIR" | sed "s/__IMAGE_TAG_REQUIRED__/$IMAGE_TAG/g" >"$output"
  if grep -q '__IMAGE_TAG_REQUIRED__' "$output"; then
    echo "rendered manifests still contain image placeholders" >&2
    exit 1
  fi
  if grep -Eq 'image: .*:latest([[:space:]]|$)' "$output"; then
    echo "rendered manifests contain mutable latest image tags" >&2
    exit 1
  fi
}

diagnostics() {
  echo "deployment diagnostics"
  "$KUBECTL" -n "$NAMESPACE" get pod,svc,ingress,certificate,pvc -o wide || true
  "$KUBECTL" -n "$NAMESPACE" logs deployment/backend --tail=100 || true
  "$KUBECTL" -n "$NAMESPACE" logs deployment/frontend --tail=50 || true
}

wait_for_certificate() {
  local _
  for _ in $(seq 1 60); do
    if "$KUBECTL" -n "$NAMESPACE" get certificate c2c-agenticim-xyz-tls >/dev/null 2>&1; then
      "$KUBECTL" -n "$NAMESPACE" wait certificate/c2c-agenticim-xyz-tls \
        --for=condition=Ready --timeout=300s
      return
    fi
    sleep 2
  done
  echo "certificate resource was not created" >&2
  exit 1
}

verify_public_endpoints() {
  local _
  for _ in $(seq 1 60); do
    if curl -fsS "https://${DOMAIN}/healthz" >/dev/null \
      && curl -fsS "https://${DOMAIN}/readyz" >/dev/null \
      && curl -fsS "https://${DOMAIN}/api/meta" >/dev/null \
      && curl -fsS "https://${DOMAIN}/api/alerts/benchmark" >/dev/null; then
      curl -fsS "https://${DOMAIN}/" >/dev/null
      return
    fi
    sleep 5
  done
  echo "public endpoint verification failed for https://${DOMAIN}" >&2
  return 1
}

main() {
  require "$KUBECTL"
  require curl
  require openssl
  require python3
  require sed

  if [[ ! "${IMAGE_TAG:-}" =~ ^[0-9a-f]{40}$ ]]; then
    echo "IMAGE_TAG must be a full lowercase 40-character commit SHA" >&2
    exit 1
  fi
  if [[ ! -f "$K8S_DIR/kustomization.yaml" ]]; then
    echo "missing $K8S_DIR/kustomization.yaml" >&2
    exit 1
  fi
  if ! "$KUBECTL" get clusterissuer letsencrypt-prod >/dev/null 2>&1; then
    echo "cluster issuer letsencrypt-prod is not available" >&2
    exit 1
  fi

  trap diagnostics ERR
  trap cleanup EXIT

  "$KUBECTL" apply -f "$K8S_DIR/namespace.yaml"
  initialize_secrets
  load_and_validate_secrets
  create_runtime_secrets
  create_image_pull_secret

  RENDERED_FILE="$(mktemp)"
  render_manifests "$RENDERED_FILE"
  "$KUBECTL" apply -f "$RENDERED_FILE"
  "$KUBECTL" -n "$NAMESPACE" rollout restart deployment/backend deployment/frontend

  "$KUBECTL" -n "$NAMESPACE" rollout status statefulset/mysql --timeout=300s
  "$KUBECTL" -n "$NAMESPACE" rollout status deployment/backend --timeout=300s
  "$KUBECTL" -n "$NAMESPACE" rollout status deployment/frontend --timeout=300s
  wait_for_certificate
  verify_public_endpoints

  "$KUBECTL" -n "$NAMESPACE" get pod,svc,ingress,certificate,pvc -o wide
  echo "deployment succeeded: https://${DOMAIN}/"
  echo "runtime secrets remain at $SECRETS_FILE (SMTP_CONFIGURED=${SMTP_CONFIGURED:-false})"
}

main "$@"
