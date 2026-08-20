# Kubernetes Production Deployment

Production runs in the existing single-node K3s cluster at `c2c.agenticim.xyz`.

## Resources

- Namespace: `c2c-monitor`
- MySQL: one `StatefulSet` with an `8Gi` `local-path` PVC
- Backend: one `Recreate` deployment to prevent overlapping monitor processes
- Frontend: one Nginx deployment with same-origin API proxying
- Ingress: Traefik with cert-manager `letsencrypt-prod`
- HTTP requests are permanently redirected to HTTPS
- TLS secret: `c2c-agenticim-xyz-tls`

Runtime secrets are not stored in Git. The deployment script creates
`/opt/c2c-monitor/secrets.env` on first deployment and mirrors its required values into
the `c2c-monitor-runtime` and `c2c-monitor-config` Kubernetes Secrets.

## Render

```bash
bash scripts/verify-k8s.sh
```

The image fields intentionally contain `__IMAGE_TAG_REQUIRED__`. Deployment replaces
both placeholders with a published full commit SHA before applying the manifests.

## Deploy

Use the `Deploy To K3s` GitHub workflow and provide a published 40-character image
SHA. The workflow uploads these manifests through SSH and runs:

```bash
IMAGE_TAG=<full-sha> bash scripts/deploy-k3s.sh
```

The deployment is successful only after MySQL, backend, frontend, certificate, and
public `/healthz`, `/readyz`, `/api/meta`, and `/api/alerts/benchmark` checks pass.

## SMTP

When `C2C_SMTP_*` GitHub secrets are absent, the first deployment explicitly disables
email notifications so the dashboard can start without repeated SMTP failures. Update
`/opt/c2c-monitor/secrets.env` on the server with real SMTP values before relying on
email alerts, then rerun the deployment workflow.
