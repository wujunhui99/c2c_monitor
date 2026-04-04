#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

required_files=(
  "AGENTS.md"
  "README.md"
  "docs/README.md"
  "docs/releases.json"
  "docs/architecture/overview.md"
  "docs/product/monitoring.md"
  "docs/operations/runbook.md"
  "docs/exec-plans/active/2026-04-harness-foundation.md"
  "docs/exec-plans/completed/README.md"
  "docs/tech-debt-tracker.md"
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

agents_lines="$(wc -l < AGENTS.md | tr -d '[:space:]')"
if (( agents_lines > 160 )); then
  echo "AGENTS.md should stay concise (<= 160 lines). current lines: $agents_lines" >&2
  exit 1
fi

go test ./...
go build ./cmd/monitor

echo "doctor checks passed"
