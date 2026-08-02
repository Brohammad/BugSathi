# Deployment

| Path | Purpose |
|------|---------|
| `compose/docker-compose.yml` | Local **dependencies** (+ observability) for `make up` |
| `compose/docker-compose.prod.yml` | Production-like full stack (migrate + api + worker) |
| `docker/Dockerfile.*` | Multi-stage images for api, worker, migrate |
| `k8s/` | Minimal Kubernetes sketches (probes, Job, Secret/ConfigMap) |

## Production Compose

```bash
cp .env.prod.example .env.prod
# edit secrets in .env.prod
make up-prod          # build + start
make up-prod-obs      # also Prometheus + Grafana
make down-prod
curl -s localhost:8080/readyz
```

## Images

```bash
make build-images
```

Tags: `bugsathi/api:latest`, `bugsathi/worker:latest`, `bugsathi/migrate:latest`.
