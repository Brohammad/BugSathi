# Event Flow Overview

## Topics (MVP)

| Topic | Key | Producers | Consumers | Purpose |
|-------|-----|-----------|-----------|---------|
| `recording.uploaded` | `recording_id` | API | Media worker | Start media pipeline |
| `recording.media.ready` | `recording_id` | Media worker | AI worker | Start AI analysis |
| `recording.analysis.ready` | `recording_id` | AI worker | API / notifier (optional) | Mark report READY |
| `recording.pipeline.dlq` | `recording_id` | Workers | Ops / reprocessor | Poison / exhausted retries |

> Alternative: single topic `recording.pipeline` with a `stage` field.  
> **MVP preference:** separate topics for clearer consumer groups and backpressure isolation (media CPU vs AI rate limits).  
> Ordering is still preserved **per recording** because every topic uses the same key.

## Message Contracts (conceptual)

```text
RecordingUploaded
  recording_id   uuid
  project_id     uuid
  object_key     string
  content_type   string
  byte_size      int64
  checksum       string (optional)
  metadata       { browser, os, viewport, ... }
  occurred_at    timestamp

MediaReady
  recording_id   uuid
  frame_keys     []string
  thumb_key      string
  duration_ms    int64
  occurred_at    timestamp

AnalysisReady
  recording_id   uuid
  report_id      uuid
  occurred_at    timestamp
```

## Ordering Guarantees

1. **Partition key = `recording_id`** on every pipeline topic.
2. Single-threaded processing **per partition** in the consumer (or max-in-flight=1 per partition).
3. Do **not** process stage N+1 until stage N committed (offset commit after successful side effects + idempotent writes).

## Idempotency

| Stage | Idempotency key | Strategy |
|-------|-----------------|----------|
| Upload complete | `recording_id` | Status transition UPLOADED only once |
| Media | `recording_id` + stage | Upsert artifacts; overwrite same keys |
| AI | `recording_id` + prompt_version | Upsert analysis row |
| Kafka | consumer group + offset | Commit only after DB/S3 success |

## Outbox (introduced with uploads)

To avoid “DB committed but Kafka produce failed”:

```text
API transaction:
  1. UPDATE recordings SET status=UPLOADED
  2. INSERT INTO outbox(event...)
Commit

Relay (same process or sidecar):
  READ outbox → PRODUCE Kafka → MARK published
```

Detailed in ADR-0005 and Milestone 5.

## Backpressure

- Media slow → lag on `recording.uploaded` (scale media workers / partitions).
- AI slow → lag on `recording.media.ready` (rate limit, don't starve media).
- Separate topics prevent a stuck AI provider from blocking media consumers.
