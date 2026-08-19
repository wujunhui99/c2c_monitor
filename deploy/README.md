# Deployment Layout

This directory is organized by deployment target:

- `compose/`: Docker Compose deployment files (current production-ready path)
- `k8s/`: Kubernetes manifests (reserved for upcoming migration)

## Compose Quick Start

```bash
cd deploy/compose
cp .env.example .env
cp config.yaml.example config.yaml
# Fill every blank value in .env and set a published 40-character commit SHA as IMAGE_TAG.
# Add the real notification recipients to config.yaml.
docker compose pull
docker compose up -d
curl -fsS http://127.0.0.1:8080/healthz
curl -fsS http://127.0.0.1:8080/readyz
```

The frontend binds to `127.0.0.1:8080` by default. Keep that setting when a host-level
reverse proxy exposes the service. Set `FRONTEND_BIND_ADDRESS=0.0.0.0` only when direct
network exposure is intentional and protected by the surrounding network.

The backend and MySQL services are not published to the host. Runtime secrets come from
the ignored `.env` file, while notification recipients remain in the ignored
`config.yaml`.

## Why this structure

Keeping Compose and Kubernetes assets separated avoids config drift and lets both deployment styles evolve independently.
