# Local load test (Compose + API on :8080)

BugSathi **Compose** (`make up`) runs Postgres, MinIO, Redpanda, observability — **not** the API. Run the API (and usually the worker) on the host:

```bash
cp .env.example .env   # once
make up && make migrate
make run-api           # terminal 1
make run-worker        # terminal 2 (optional for read-only load)
```

## k6 (recommended)

```bash
brew install k6
make loadtest                    # bootstrap user + 5 VUs × 30s
VUS=10 DURATION=1m make loadtest # heavier (watch 429s)
```

Bootstrap writes `scripts/loadtest/.env.loadtest` with a JWT and project id. The script hits **read paths only**: `/healthz`, `/readyz`, `/v1/auth/me`, list/get project. It does **not** hammer `/v1/auth/login` (auth bucket is 5 RPS).

Default `.env` rate limits: `RATE_LIMIT_RPS=20`, `AUTH_RATE_LIMIT_RPS=5`. If `rate_limited_count` is high, lower `VUS` or temporarily raise limits in `.env` for the test machine only.

Metrics: Prometheus `:9090`, Grafana `:3000` while Compose is up.

## Postman

No collection is checked in yet. You can mirror the bootstrap flow:

1. Register/login → save `access_token` as collection variable.
2. Create project → save `project_id`.
3. **Collection Runner** or **Performance** on `GET /healthz`, `GET /v1/projects`, `GET /v1/projects/{{project_id}}` with Bearer auth.
4. Avoid looping login in performance runs (same limiter behavior as above).

Import [k6](https://k6.io) results are easier to archive in CI than Postman Performance exports.
