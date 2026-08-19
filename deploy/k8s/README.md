# Kubernetes Placeholder

Kubernetes deployment is not currently production-ready. The supported deployment path
is `deploy/compose/`.

Planned structure:

- `base/`: reusable manifests (`Deployment`, `Service`, `ConfigMap`, `Secret` references)
- `overlays/dev/`: dev-specific patches/values
- `overlays/prod/`: production-specific patches/values

Suggested next step:

Generate initial Kustomize files in `base/`, use Secret references for the administrator,
database, and SMTP credentials, and wire liveness to `/healthz` and readiness to
`/readyz`.
