# ADR 0021: Performance Optimizations

## Status

Accepted

## Context

Hot report reads hit Postgres + MinIO presign on every request. AI cost/latency scales with frames sent. Connection pools and CPU profiles need to be tunable without code changes.

## Decision

1. **Postgres pool** — configurable `POSTGRES_MAX_CONNS`, `POSTGRES_MIN_CONNS`, `POSTGRES_MAX_CONN_LIFETIME` (defaults remain conservative).
2. **Keyframe selection** — evenly spaced sample including first and last frames (not “first N”), capped by `AI_MAX_FRAMES`.
3. **Report detail cache** — in-process TTL cache of report aggregate metadata (not presigned URLs). URLs are re-signed on every hit. Default TTL 30s; disable with `REPORT_CACHE_TTL=0`.
4. **Profiling** — `net/http/pprof` mounted at `/debug/pprof/` when `ENABLE_PPROF=true` (off by default in production).

## Consequences

**Positive** — lower DB load on hot reads; better AI coverage of recording timeline; operable profiling.  
**Negative** — cache is single-process (fine for Compose; Redis later for multi-replica); pprof must stay auth-gated/network-restricted in real prod.
