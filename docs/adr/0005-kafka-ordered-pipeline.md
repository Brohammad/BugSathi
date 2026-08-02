# ADR 0005: Kafka Ordered Pipeline Per Recording

## Status

Accepted

## Context

After upload, work must proceed: extract frames → AI analyze → materialize report. Stages must not race for the same recording (e.g., AI must not run before frames exist). Multiple recordings should still process in parallel.

## Decision

- Use **Apache Kafka** as the pipeline backbone.
- **Message key = `recording_id`** on all pipeline topics.
- Separate topics per stage for independent consumer scaling:
  - `recording.uploaded`
  - `recording.media.ready`
  - `recording.analysis.ready`
  - `recording.pipeline.dlq`
- Consumers process with **per-partition ordering** (max in-flight 1 per partition, or equivalent).
- **Idempotent** handlers; commit offsets only after successful side effects.
- Introduce **transactional outbox** when emitting first events from the API (Milestone 5).

## Consequences

**Positive**

- Clear ordering story for interviews.
- Replay for debugging; DLQ for poison messages.
- Stage-specific backpressure.

**Negative**

- Heavier local stack than Redis.
- Exactly-once is hard; we aim for **at-least-once + idempotency** (honest production model).

## Alternatives

| Alternative | Notes |
|-------------|-------|
| Single DB poller (“SELECT FOR UPDATE SKIP LOCKED”) | Simpler; weaker replay/fanout lessons |
| Redis Streams | Lighter; keep as emergency fallback |
| Temporal / Cadence workflows | Excellent orchestration; steeper early curve — candidate post-MVP |
| In-process goroutine queue | Lost on crash; no multi-worker story |

## Complexity note

Throughput scales with **partition count** and distinct keys, not by parallelizing one key. One recording is intentionally serial across stages.
