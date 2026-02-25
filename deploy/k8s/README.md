# Kubernetes Placeholder

Planned structure:

- `base/`: reusable manifests (`Deployment`, `Service`, `ConfigMap`, `Secret` references)
- `overlays/dev/`: dev-specific patches/values
- `overlays/prod/`: production-specific patches/values

Suggested next step:

Generate initial Kustomize files in `base/` and wire `overlays/dev|prod`.
