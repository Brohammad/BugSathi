# ADR 0008: Correlation & Observability from Day One

## Status

Accepted

## Context

Milestone 11 adds metrics dashboards and full tracing exporters, but without stable identifiers in logs from the start, later instrumentation cannot stitch a single recording’s journey across API and workers.

## Decision

Every log line and Kafka message carries:

| Field | Source |
|-------|--------|
| `request_id` | Generated at HTTP edge; absent in pure consumers |
| `recording_id` | When processing a recording |
| `correlation_id` | Propagated end-to-end; set on upload complete (often copy of `request_id` or new UUID) |

Implementation rules (from M2 onward):

- Structured JSON logs (`slog`).
- Middleware injects `request_id` / `correlation_id` into context.
- Kafka producer writes these as headers; consumers restore into context.
- `/healthz` stays cheap; deeper `/readyz` checks dependencies.

Full OpenTelemetry exporters and Grafana dashboards remain Milestone 11; **fields and context propagation are not deferred**.

## Consequences

**Positive**

- Debuggable pipelines immediately.
- Tracing later is additive, not a rewrite.

**Negative**

- Slight log volume / discipline overhead.

## Alternatives

| Alternative | Rejected because |
|-------------|------------------|
| Wait until M11 | Lost early debugging; retrofit pain |
| Only `recording_id` | Harder to debug pre-recording HTTP issues |
