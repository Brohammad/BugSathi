# ADR 0022: Production Hardening

## Status

Accepted

## Context

Before treating BugSathi as production-ready, the API needs baseline abuse protection, safe HTTP defaults, operable SLOs, and documented incident response. Workers need smarter retry backoff than fixed sleeps.

## Decision

1. **Security headers** on all API responses (`X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`, `Permissions-Policy`).
2. **Rate limiting** — in-process token bucket per client IP (`golang.org/x/time/rate`). Stricter bucket for `/v1/auth/*`. Returns `429` + `Retry-After`. Disabled when `RATE_LIMIT_RPS=0`. Forwarding headers honored only from `TRUSTED_PROXIES` (ADR 0027).
3. **Request body cap** — `MAX_BODY_BYTES` (default 1 MiB) on mutating routes.
4. **Worker retry** — exponential backoff with jitter between Kafka consume retries (`KAFKA_RETRY_BASE`, `KAFKA_RETRY_MAX`).
5. **Operations docs** — SLO definitions + runbooks under `docs/operations/`.
6. **Chaos drill script** — `scripts/chaos-drill.sh` exercises readiness failure/recovery (Postgres stop/start).

## Consequences

**Positive** — safer defaults, operable on-call path, teaches backoff vs blind retry.  
**Negative** — in-memory rate limits do not span API replicas (Redis later); chaos script is Compose-local.
