# Deployment

| Path | Purpose |
|------|---------|
| `compose/docker-compose.yml` | Local **dependencies** (+ observability) for `make up` |
| `compose/docker-compose.prod.yml` | Single-VPS stack: Caddy + web + api + worker + private infra |
| `caddy/Caddyfile` | TLS reverse proxy. Public: UI, `/v1`, `/s`, `/healthz`. Blocks `/metrics`. |
| `docker/Dockerfile.*` | Multi-stage images for api, worker, migrate, web |
| `k8s/` | Minimal Kubernetes sketches (probes, Job, Secret/ConfigMap) |

## Public surface

Caddy is the only process published to the host network:

| Host port | Traffic |
|-----------|---------|
| `80` / `443` | SPA + API (`/v1`, `/s`, `/healthz`, `/readyz`) |
| `9000` on **127.0.0.1** (local) or `s3.<domain>` | Presigned MinIO PUT/GET |

Postgres, Redis, Redpanda, MinIO console, API `:8080`, worker `:8081`, and `/metrics` stay on the Compose network. Observability (profile `obs`) binds Grafana/Prometheus to **127.0.0.1** only.

## Production Compose

```bash
cp .env.prod.example .env.prod
# edit secrets in .env.prod
make check-prod-compose
make up-prod          # build + start
make up-prod-obs      # also Prometheus + Grafana on localhost
make down-prod
curl -sS http://127.0.0.1/readyz
curl -sS -o /dev/null -w '%{http_code}\n' http://127.0.0.1/metrics   # expect 404
./scripts/chaos-drill.sh
```

VPS DNS, TLS, firewall, and backup steps: [docs/operations/vps.md](../docs/operations/vps.md).

Operations docs: [docs/operations](../docs/operations/README.md)

## Images

```bash
make build-images
```

Tags: `bugsathi/api:latest`, `bugsathi/worker:latest`, `bugsathi/migrate:latest`, `bugsathi/web:latest`.
