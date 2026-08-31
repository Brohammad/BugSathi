# Single-VPS deploy (Docker Compose + Caddy)

Target: one 4 GB Linux VPS. Repository foundation is Day 5; **Day 6** adds deploy scripts, smoke/E2E, backup/restore, and verification.

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

## Quick start (VPS)

```bash
# On the VPS (after Docker + git)
APP_DOMAIN=bugsathi.example.com \
S3_DOMAIN=s3.bugsathi.example.com \
ACME_EMAIL=you@example.com \
./scripts/deploy-vps.sh
```

That clones/updates the repo, writes secrets, configures `.env.prod`, runs `make up-prod`, and executes `./scripts/smoke-prod.sh` over HTTPS.

Full runbook: [runbooks/deploy.md](runbooks/deploy.md)

## Configure manually

```bash
cp .env.prod.example .env.prod
# or: ./scripts/gen-prod-secrets.sh --write
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

## Bring up and verify

```bash
make check-prod-compose
make up-prod
./scripts/smoke-prod.sh          # health, SPA, /metrics 404, closed ports
./scripts/e2e-prod.sh            # full pipeline (host ffmpeg)
./scripts/chaos-drill.sh         # Postgres stop/start readiness
make health-prod
```

Worker readiness is internal only:

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

Do not publish 9000 on a public interface. Default `CADDY_S3_BIND=127.0.0.1:9000`. On a VPS, point `CADDY_S3_SITE` at `s3.<domain>` (TLS on 443).

## Backup and restore

```bash
./scripts/backup-prod.sh              # -> backups/<timestamp>/
./scripts/restore-prod.sh backups/<timestamp>/postgres.sql --confirm
```

Schedule daily Postgres dumps (cron) and periodic MinIO mirrors off-box. Keep `git-rev.txt` in each backup folder to match code at backup time.

## Rollback

See [runbooks/deploy.md](runbooks/deploy.md). Short version: backup → `git checkout <sha>` → `make up-prod` → smoke. If migrations moved, restore Postgres from the pre-deploy dump.

## Scripts reference

| Script | Purpose |
|--------|---------|
| `gen-prod-secrets.sh --write` | Create `.env.prod` with random secrets |
| `deploy-vps.sh` | First-time / update VPS deploy |
| `smoke-prod.sh` | Fast public-surface checks |
| `e2e-prod.sh` | Full E2E via Caddy + internal worker check |
| `backup-prod.sh` | Postgres dump + MinIO mirror |
| `restore-prod.sh` | Postgres restore (destructive) |
| `chaos-drill.sh` | Postgres outage readiness drill |
| `check-prod-compose.sh` | Guard unpublished ports + render Compose |

## Honest limits

- Single host, not HA. Redpanda runs `--mode dev-container` (one broker).
- Let's Encrypt needs a public DNS name; local verify stays HTTP.
- Worker `/metrics` is never on Caddy; scrape only from the Compose network or SSH tunnel.
