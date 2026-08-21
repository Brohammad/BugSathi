# ADR 0033 — Remaining audit gaps (M27)

## Status

Accepted

## Context

After M26 closed the highest-impact audit findings, several correctness,
ops, and docs↔code gaps remained. They are small enough to land as **one
commit per issue** so each fix stays reviewable and bisectable.

## Decision

Ship Milestone 27 as discrete commits:

1. **Kafka client ID** — advertise `KAFKA_CLIENT_ID` on readers/writers.
2. **Header correlation** — restore `correlation_id` / `recording_id` from Kafka headers (ADR 0008).
3. **Worker drain** — wait for consumers + final outbox flush on shutdown.
4. **Redis rate limit** — fail closed on Redis errors; window capacity = max(burst, RPS×window).
5. **Share owner-only** — create/revoke require project owner; list stays member.
6. **Member remove** — `DELETE …/members/{userID}` with last-owner conflict.
7. **CSP + HSTS** — API CSP deny-by-default; HSTS only when `APP_ENV=production`.
8. **Abandoned upload GC** — worker sweeps stale `UPLOADING` after `UPLOAD_ABANDONED_TTL`.
9. **AnalysisStarted / GENERATING** — mark report generating and emit started before Analyze.
10. **Per-recording delete** — owner `DELETE …/recordings/{id}` + prefix object cleanup.

## Consequences

**Positive** — closes the remaining audit checklist items without bundling;
operators get GC, drain, and clearer rate-limit behavior.  
**Negative** — `AnalysisStarted` was emit-only until M28; DLQ attempt durability
and an observability consumer land in ADR 0034. Multimodal OpenAI remains deferred.

## Alternatives

| Option | Why not now |
|--------|-------------|
| One mega-commit | Harder to review/bisect (explicitly rejected) |
| Multimodal OpenAI | Separate product milestone |
| Durable DLQ attempt store | Schema + ops work deferred |
