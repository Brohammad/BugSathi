# Runbook: Postgres unavailable

## Symptoms

- `GET /readyz` returns **503** with `{"status":"not_ready","checks":{"postgres":"down"}}`
- Compose/K8s marks API and worker **not ready**; load balancers stop routing traffic
- `/healthz` may still return 200 (process is alive)
- Logs: `ping postgres` / pool errors on API and worker

## Immediate checks

1. Confirm Postgres container/pod status:
   ```bash
   docker compose -f deploy/compose/docker-compose.prod.yml --env-file .env.prod ps postgres
   ```
2. Curl readiness:
   ```bash
   curl -sS -o /dev/null -w '%{http_code}\n' http://127.0.0.1:8080/readyz
   ```
3. Inspect Postgres logs:
   ```bash
   docker compose -f deploy/compose/docker-compose.prod.yml --env-file .env.prod logs --tail=100 postgres
   ```

## Recovery

1. Restart Postgres (or restore from backup if data corruption):
   ```bash
   docker compose -f deploy/compose/docker-compose.prod.yml --env-file .env.prod start postgres
   ```
2. Wait for `pg_isready` healthcheck to pass.
3. Verify `/readyz` returns 200 on API (`:8080`) and worker (`:8081`).
4. Watch `bugsathi_outbox_pending` — relay should drain without manual intervention.

## Expected behavior

- API and worker **fail closed**: no partial writes while DB is down.
- Kafka consumers retry with exponential backoff (`KAFKA_RETRY_BASE` / `KAFKA_RETRY_MAX`); offsets are not committed until handlers succeed.
- No automatic failover in single-host Compose — treat as **planned downtime** for SLO accounting.

## Drill

Run `./scripts/chaos-drill.sh` against prod Compose to validate readiness flapping and recovery.
