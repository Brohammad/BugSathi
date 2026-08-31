# Operations

Runbooks and SLOs for on-call and production readiness drills.

| Doc | Purpose |
|-----|---------|
| [vps.md](vps.md) | Single-VPS Compose + Caddy topology, firewall, volumes, rollback |
| [slos.md](slos.md) | Service level objectives and alert hints |
| [runbooks/deploy.md](runbooks/deploy.md) | Deploy, verify, rollback on single VPS |
| [runbooks/postgres-down.md](runbooks/postgres-down.md) | Readiness failure when Postgres is unavailable |
| [runbooks/api-errors.md](runbooks/api-errors.md) | Elevated HTTP 5xx or latency |
| [runbooks/pipeline-stuck.md](runbooks/pipeline-stuck.md) | Recordings not reaching READY / outbox lag |
| [runbooks/dlq-reprocess.md](runbooks/dlq-reprocess.md) | Poison messages on `*.dlq` and owner reprocess |

## Local drills

```bash
make up-prod
./scripts/smoke-prod.sh
./scripts/chaos-drill.sh
```

The chaos drill stops Postgres in prod Compose, asserts `/readyz` fails, then verifies recovery after Postgres restarts.
