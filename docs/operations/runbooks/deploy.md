# Runbook: Deploy and rollback (single VPS)

## Preconditions

- DNS `A` records for app + `s3.` subdomain point at the VPS
- Ports 80/443 open; 5432, 6379, 8080, 8081, 9001 closed on the public interface
- `.env.prod` secrets rotated from examples (`./scripts/gen-prod-secrets.sh --write`)

## First deploy

```bash
APP_DOMAIN=bugsathi.example.com \
S3_DOMAIN=s3.bugsathi.example.com \
ACME_EMAIL=you@example.com \
./scripts/deploy-vps.sh
```

Manual equivalent:

```bash
cp .env.prod.example .env.prod   # or gen-prod-secrets --write
# set CADDY_SITE, CADDY_S3_SITE, ACME_EMAIL, MINIO_PUBLIC_*, MINIO_CORS_ORIGINS
make check-prod-compose
./scripts/backup-prod.sh         # empty DB still fine — establishes backup habit
make up-prod
./scripts/smoke-prod.sh
./scripts/e2e-prod.sh            # needs host ffmpeg
./scripts/chaos-drill.sh
```

## Verify after every deploy

| Check | Command | Expect |
|-------|---------|--------|
| Readiness | `curl -fsS https://$APP/readyz` | 200 |
| Metrics private | `curl -o /dev/null -w '%{http_code}' https://$APP/metrics` | 404 |
| Infra closed | `./scripts/smoke-prod.sh` | PASS only |
| Pipeline | `./scripts/e2e-prod.sh` | PASS=… FAIL=0 |
| Outage drill | `./scripts/chaos-drill.sh` | Drill passed |

Worker metrics stay internal — scrape from Prometheus on `127.0.0.1:9090` via SSH tunnel (`make up-prod-obs`), not from the internet.

## Rolling update (same host)

```bash
cd ~/BugSathi
./scripts/backup-prod.sh
git fetch && git checkout <tag-or-sha> && git pull --ff-only
make up-prod                     # rebuild changed images, migrate job runs once
./scripts/smoke-prod.sh
```

Migrate is a one-shot service: each `up` runs pending SQL. If migrate fails, **do not** restart api/worker on a half-applied schema — fix migrate logs first.

## Rollback

1. `./scripts/backup-prod.sh` **before** deploy (always).
2. If the new release is bad **without** a breaking migration:
   ```bash
   git checkout <previous-sha>
   make up-prod
   ./scripts/smoke-prod.sh
   ```
3. If a **migration shipped**, code rollback alone is unsafe:
   ```bash
   git checkout <previous-sha>
   ./scripts/restore-prod.sh backups/<stamp>/postgres.sql --confirm
   make up-prod
   ```
4. MinIO objects are not reverted by Postgres restore. Mirror back with `mc mirror` from `backups/<stamp>/minio/` if needed.

## TLS failures

- Confirm DNS propagates: `dig +short $APP_DOMAIN`
- Caddy logs: `docker compose -f deploy/compose/docker-compose.prod.yml --env-file .env.prod logs caddy --tail=100`
- Port 80 must reach the host for HTTP-01 (default Caddy ACME)

## Local prod verify (no DNS)

```bash
./scripts/gen-prod-secrets.sh --write
make up-prod
./scripts/smoke-prod.sh
./scripts/e2e-prod.sh
./scripts/chaos-drill.sh
make down-prod
```
