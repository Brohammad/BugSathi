# Single-VPS deploy (Docker Compose + Caddy)

Target: one 4 GB Linux VPS. Day 5 is the **repository foundation**. DNS, TLS issuance, and live smoke tests are Day 6.

## Topology

```
Internet
   │
   ▼
Caddy :80/:443          ← only public process
   ├─ /                 → web (nginx SPA)
   ├─ /v1 /s /healthz   → api :8080
   ├─ /metrics          → 404
   └─ s3.<domain> or 127.0.0.1:9000 → minio :9000 (presigned PUT/GET)
         │
         ▼  Compose network (no host ports)
   postgres · redis · redpanda · minio · api · worker
```

Grafana/Prometheus (optional `--profile obs`) bind `127.0.0.1` so they are reachable via SSH tunnel, not the public internet.

## Host requirements

- Docker Engine + Compose v2
- 4 GB RAM (2 GB will swap under ffmpeg + Redpanda)
- Ports 80 and 443 free; firewall drop 5432, 6379, 8080, 8081, 9001, 9090, 3000
- A domain with `A` records for the app and `s3.<domain>` (VPS only)

## Configure

```bash
cp .env.prod.example .env.prod
```

Change every `change-me-*` secret. Then:

**Local HTTP verify (no DNS)**

```
CADDY_SITE=:80
CADDY_S3_SITE=:9000
MINIO_PUBLIC_ENDPOINT=localhost:9000
MINIO_PUBLIC_USE_SSL=false
MINIO_CORS_ORIGINS=http://localhost,http://127.0.0.1
```

**VPS with Let's Encrypt**

```
CADDY_SITE=bugsathi.example.com
CADDY_S3_SITE=s3.bugsathi.example.com
ACME_EMAIL=you@example.com
MINIO_PUBLIC_ENDPOINT=s3.bugsathi.example.com
MINIO_PUBLIC_USE_SSL=true
MINIO_CORS_ORIGINS=https://bugsathi.example.com
```

`MINIO_PUBLIC_ENDPOINT` is required because the API talks to `minio:9000` internally. Presigned URLs must use the hostname the **browser** will call, or uploads and frame playback fail.

Leave `CORS_ORIGINS` empty so the SPA and API stay same-origin behind Caddy.

## Bring up

```bash
make check-prod-compose
make up-prod
curl -sS http://127.0.0.1/readyz
curl -sS -o /dev/null -w '%{http_code}\n' http://127.0.0.1/metrics   # 404
```

Worker readiness is internal:

```bash
docker compose -f deploy/compose/docker-compose.prod.yml --env-file .env.prod \
  exec -T worker curl -fsS http://127.0.0.1:8081/readyz
```

Observability (localhost only):

```bash
make up-prod-obs
ssh -L 3000:127.0.0.1:3000 -L 9090:127.0.0.1:9090 user@vps
```

## Firewall (VPS)

```bash
ufw default deny incoming
ufw default allow outgoing
ufw allow OpenSSH
ufw allow 80/tcp
ufw allow 443/tcp
ufw enable
```

Do not publish 9000 on a public interface. Default `CADDY_S3_BIND=127.0.0.1:9000`. On a VPS, point `CADDY_S3_SITE` at `s3.<domain>` (TLS on 443) and leave the loopback bind as a local debug hatch.

## Volumes and backup

Named volumes: `postgres_prod_data`, `redis_prod_data`, `minio_prod_data`, `redpanda_prod_data`, `caddy_prod_data`.

Postgres dump (logical, preferred):

```bash
docker compose -f deploy/compose/docker-compose.prod.yml --env-file .env.prod \
  exec -T postgres pg_dump -U bugsathi bugsathi > bugsathi-$(date +%F).sql
```

MinIO is the recording/frame store — back up `minio_prod_data` or `mc mirror` off-box. Full restore drill is Day 6.

## Rollback

Images are built locally. Keep the previous git SHA:

```bash
git checkout <previous-sha>
make up-prod
```

Migrations are forward-only. If a migration shipped, restore the Postgres dump taken before deploy, then start the matching SHA.

## Chaos drill

`./scripts/chaos-drill.sh` hits Caddy `:80` for API readiness and `compose exec` for the worker. Postgres stop must return 503, start must recover.

## Honest limits

- Single host, not HA. Redpanda runs `--mode dev-container` (one broker).
- Let's Encrypt needs a public DNS name; local verify stays HTTP.
- Worker `/metrics` is never on Caddy; scrape only from the Compose network or SSH tunnel.
