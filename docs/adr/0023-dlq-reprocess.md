# ADR 0023: Dead-Letter Queue + Recording Reprocess

## Status

Accepted

## Context

After M14, Kafka consumers retry failed handlers forever with exponential backoff. Poison messages stall a partition. Ops docs already describe FAILED recordings and “admin SQL” as the escape hatch. Domain already allows `FAILED → PROCESSING`; no HTTP path or DLQ existed.

## Decision

1. **Max attempts** — `KAFKA_RETRY_MAX_ATTEMPTS` (default 5). Track attempts in-process by `topic/partition/offset`. On handler failure the consumer **retries the same message** (does not `FetchMessage` the next offset) with exponential backoff. After the cap, publish a dead-letter envelope to `{source}.dlq` and **commit** the source offset so the consumer advances.
2. **Invalid JSON** — also dead-lettered (was silently committed).
3. **DLQ envelope** — source topic/partition/offset, key, attempt count, error, original payload, timestamp.
4. **Reprocess API** — `POST /v1/projects/{projectID}/recordings/{id}/reprocess` (project **owner** only). Allowed for `FAILED`, `UPLOADED`, `PROCESSING`, or `READY`. Re-inserts `RecordingUploaded` into the outbox (no status change); media/AI handlers remain the source of truth for transitions and idempotency.

## Consequences

**Positive** — partitions no longer stall forever; operators have a first-class retry; teaches DLQ vs blind retry.  
**Negative** — attempt counters reset on process restart (extra retries before DLQ); DLQ is not auto-consumed (inspect/replay via reprocess).
