# ADR 0031: Optional Redis for Multi-Replica API

## Status

Accepted

## Context

Single-process SSE hub, in-memory rate limits, and report detail cache work for
Compose and one API replica (ADRs 0018, 0021, 0022). Running multiple API
instances breaks comment SSE fanout and splits rate-limit buckets unless state is
shared.

## Decision

1. **`REDIS_URL`** — when set, API uses Redis-backed adapters; when empty, keep
   current in-memory behavior (default for local dev).
2. **SSE** — `RedisHub` publishes comment/presence events to Redis pub/sub;
   each replica forwards to local subscribers. Presence lists remain per-instance
   (sticky sessions or future Redis presence SET optional).
3. **Rate limits** — fixed-window counters in Redis keyed by route + client IP.
4. **Report cache** — JSON-serialized report detail aggregates with TTL.
5. Compose adds optional `redis:7-alpine` on port 6379; `/readyz` pings Redis when configured.

## Consequences

**Positive** — horizontal API scaling without forking business logic; ports unchanged.  
**Negative** — extra infra and ops; presence not global until a later iteration.

## Alternatives

| Alternative | Rejected |
|-------------|----------|
| Require Redis always | Overkill for single-replica MVP |
| Kafka for SSE | Heavier than pub/sub for ephemeral events |
| Sticky sessions only | Does not fix shared rate limits or cache |
