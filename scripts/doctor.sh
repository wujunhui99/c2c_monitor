#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

required_files=(
  "AGENTS.md"
  "README.md"
  ".github/dependabot.yml"
  ".github/workflows/codeql.yml"
  ".github/workflows/verify.yml"
  "docs/README.md"
  "docs/releases.json"
  "docs/architecture/external-dependencies.md"
  "docs/architecture/overview.md"
  "docs/product/monitoring.md"
  "docs/operations/runbook.md"
  "docs/exec-plans/active/2026-08-reliability-security.md"
  "docs/exec-plans/active/2026-08-k3s-production-deploy.md"
  "docs/exec-plans/completed/README.md"
  "docs/tech-debt-tracker.md"
  "deploy/compose/.env.example"
  "deploy/compose/config.yaml.example"
  "deploy/k8s/application.yaml"
  "deploy/k8s/database.yaml"
  "deploy/k8s/ingress.yaml"
  "deploy/k8s/kustomization.yaml"
  "deploy/k8s/namespace.yaml"
  "scripts/deploy-k3s.sh"
  "scripts/verify-k8s.sh"
  "cmd/monitor/main.go"
)

for file in "${required_files[@]}"; do
  if [[ ! -f "$file" ]]; then
    echo "missing required repo map file: $file" >&2
    exit 1
  fi
done

if [[ -f "PRD.md" ]]; then
  echo "PRD.md should be removed; keep the repo map in AGENTS.md and docs/ instead" >&2
  exit 1
fi

if git ls-files --error-unmatch deploy/compose/config.yaml >/dev/null 2>&1 \
  && [[ -f "deploy/compose/config.yaml" ]]; then
  echo "deploy/compose/config.yaml must stay untracked; use config.yaml.example as the template" >&2
  exit 1
fi

agents_lines="$(wc -l < AGENTS.md | tr -d '[:space:]')"
if (( agents_lines > 160 )); then
  echo "AGENTS.md should stay concise (<= 160 lines). current lines: $agents_lines" >&2
  exit 1
fi

go_files=()
while IFS= read -r file; do
  go_files+=("$file")
done < <(find cmd config internal -name '*.go' -type f | sort)
unformatted="$(gofmt -l "${go_files[@]}")"
if [[ -n "$unformatted" ]]; then
  echo "Go files need gofmt:" >&2
  echo "$unformatted" >&2
  exit 1
fi

if ! command -v node >/dev/null 2>&1; then
  echo "node is required for frontend JavaScript syntax checks" >&2
  exit 1
fi
if ! command -v ruby >/dev/null 2>&1; then
  echo "ruby is required for YAML syntax checks" >&2
  exit 1
fi

bash -n scripts/deploy-k3s.sh
bash scripts/verify-k8s.sh
git diff --check
go test ./...
go vet ./...
build_output="$(mktemp "${TMPDIR:-/tmp}/c2c-monitor-doctor.XXXXXX")"
trap 'rm -f "$build_output"' EXIT
go build -o "$build_output" ./cmd/monitor
node --check frontend/js/app.js
node --check frontend/js/release-notes.js

echo "doctor checks passed"
