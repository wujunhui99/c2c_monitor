#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

rendered="$(mktemp "${TMPDIR:-/tmp}/c2c-monitor-k8s.XXXXXX")"
trap 'rm -f "$rendered"' EXIT

if command -v kubectl >/dev/null 2>&1; then
  kubectl kustomize deploy/k8s >"$rendered"
elif command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then
  docker run --rm \
    -v "$ROOT_DIR:/workspace:ro" \
    registry.k8s.io/kustomize/kustomize:v5.7.1 \
    build /workspace/deploy/k8s >"$rendered"
else
  echo "kubectl or a running Docker daemon is required to render deploy/k8s" >&2
  exit 1
fi

if [[ "$(grep -c '__IMAGE_TAG_REQUIRED__' "$rendered")" -ne 2 ]]; then
  echo "rendered Kubernetes manifests must contain exactly two immutable image placeholders" >&2
  exit 1
fi
if grep -Eq 'image: .*:latest([[:space:]]|$)' "$rendered"; then
  echo "rendered Kubernetes manifests must not use mutable latest image tags" >&2
  exit 1
fi

for required in \
  "kind: StatefulSet" \
  "name: backend" \
  "name: frontend" \
  "host: c2c.agenticim.xyz" \
  "cert-manager.io/cluster-issuer: letsencrypt-prod" \
  "traefik.ingress.kubernetes.io/router.middlewares: c2c-monitor-https-redirect@kubernetescrd" \
  "ingressClassName: traefik"; do
  if ! grep -Fq "$required" "$rendered"; then
    echo "rendered Kubernetes manifests are missing: $required" >&2
    exit 1
  fi
done

ruby -e 'require "yaml"; YAML.parse_stream(File.read(ARGV.fetch(0)))' "$rendered"
echo "Kubernetes manifests rendered successfully"
