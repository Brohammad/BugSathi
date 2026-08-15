# Operations

Runbooks and SLOs for on-call and production readiness drills.

| Doc | Purpose |
|-----|---------|
| [slos.md](slos.md) | Service level objectives and alert hints |
| [runbooks/postgres-down.md](runbooks/postgres-down.md) | Readiness failure when Postgres is unavailable |
| [runbooks/api-errors.md](runbooks/api-errors.md) | Elevated HTTP 5xx or latency |
| [runbooks/pipeline-stuck.md](runbooks/pipeline-stuck.md) | Recordings not reaching READY / outbox lag |

## Local drills

```bash
make up-prod          # full stack (needs .env.prod)
./scripts/chaos-drill.sh
```

The chaos drill stops Postgres in prod Compose, asserts `/readyz` fails, then verifies recovery after Postgres restarts.
