# Deployment Layout

This directory is organized by deployment target:

- `compose/`: Docker Compose deployment files (current production-ready path)
- `k8s/`: Kubernetes manifests (reserved for upcoming migration)

## Compose Quick Start

```bash
cd deploy/compose
cp config.yaml.example config.yaml
# edit config.yaml with real SMTP/DB values
docker compose pull
docker compose up -d
```

## Why this structure

Keeping Compose and Kubernetes assets separated avoids config drift and lets both deployment styles evolve independently.
