# BugSathi — Postman (local Compose)

Infrastructure: `make up` (Postgres, MinIO, Redpanda). API on the host:

```bash
make migrate
make run-api          # :8080
# make run-worker     # optional for read-only load
```

## Collections

| Collection | Purpose |
|------------|---------|
| **BugSathi Setup** | Register (idempotent), login, create project → sets `access_token`, `project_id` |
| **BugSathi Read Load** | Health + authenticated reads (no login loop — avoids auth rate limit) |

Environment: `postman/environments/BugSathi Local.environment.yaml`

## Functional run

```bash
postman collection run "postman/collections/BugSathi Setup" \
  -e "postman/environments/BugSathi Local.environment.yaml"
postman collection run "postman/collections/BugSathi Read Load" \
  -e "postman/environments/BugSathi Local.environment.yaml"
```

## Load test (local runner)

```bash
make postman-loadtest
# or:
POSTMAN_PEAK_VU=10 POSTMAN_DURATION=2 make postman-loadtest
```

Default profile: **fixed**, 5 peak VUs, 1 minute (`--duration` is whole minutes only), thresholds `p95 < 750ms` and `error_rate < 10%`.

Expect some **429**s near default `RATE_LIMIT_RPS=20` if you raise VUs; auth routes are not in the load mix.

Performance runs **reuse the collection as fast as the runner allows** (no per-request sleep). At 5 VUs we saw ~550 req/s and most **test assertions failed** (likely 429) while transport `error_rate` stayed 0% — good for stressing the limiter, not for latency SLOs. For calmer runs use `POSTMAN_PEAK_VU=1` or bump `RATE_LIMIT_RPS` in `.env` for the test only.

`make postman-smoke` / `make postman-loadtest` call `scripts/postman/sync-env.sh` to login and refresh `access_token` / `project_id` (tokens are not committed; keep `access_token` as `type: secret` in the env file).

Workspace binding: `.postman/resources.yaml` (guest init workspace — run `postman signup` or `postman login` to keep cloud sync).
