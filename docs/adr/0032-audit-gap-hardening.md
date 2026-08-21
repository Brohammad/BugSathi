# ADR 0032 — Audit-gap hardening (M26)

## Status

Accepted

## Context

A learning architecture audit found several correctness and security gaps in an otherwise working MVP:

1. AI redelivery while `analyses.status = running` could call the LLM twice.
2. Report detail caches were never invalidated after AI writes.
3. Refresh-token reuse returned 401 but did not revoke the rest of the user’s sessions.
4. Presigned uploads ignored content type binding, size limits, and checksum capture.
5. Collab SSE existed on the API but the React report page did not subscribe.

## Decision

Ship these as **Milestone 26** without new product features:

1. **AI soft claim** — `TryClaimRunning` only claims when the row is new/failed/stale (`updated_at` older than `AI_CLAIM_LEASE`). Fresh `running` → `ErrAnalysisInFlight`; the AI Kafka consumer commits and skips (same pattern as media claim held). Lease heartbeats via `TouchRunning` during `Analyze`.
2. **Cache invalidation** — when `REDIS_URL` is set, the worker wraps the AI service with `RedisReportCache.Invalidate` after complete/fail.
3. **Refresh family revoke** — after a failed rotate, if the presented hash is already revoked and outside a 10s grace window, `RevokeAllForUser`. Grace protects concurrent double-refresh races.
4. **Upload hardening** — allowlist `video/webm|mp4|quicktime`; bind `Content-Type` on MinIO `PresignHeader`; enforce `UPLOAD_MAX_BYTES` (default 500MiB) on complete; store object ETag as `checksum`; reject content-type mismatches.
5. **Web SSE** — `ReportPage` opens `EventSource` on `/events?access_token=…`. `RequireAccess` accepts that query param only for paths ending in `/events` (EventSource cannot set `Authorization`).

## Consequences

**Positive** — closes the highest-impact audit gaps with tests; aligns AI concurrency with media leases; shareable multi-replica report freshness when Redis is on.

**Negative** — SSE still puts a short-lived access JWT in the query string (Acceptable for local/learning; prefer cookies later). Soft AI lease is weaker than a dedicated owner column (acceptable; reclaim via `updated_at`). In-process report cache still relies on TTL when Redis is off.

## Alternatives

| Option | Why not now |
|--------|-------------|
| Full analysis claim columns like media | Soft lease is enough for the duplicate-LLM bug; schema change deferred |
| HttpOnly cookie auth for EventSource | Larger auth redesign |
| Multimodal OpenAI frames | Separate product milestone |
