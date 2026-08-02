# Event Flow Overview

## Naming convention

- **Past tense** for facts that already happened (`RecordingUploaded`, not `UploadRecording`).
- Kafka topic names: `bugsathi.<aggregate>.<event-kebab>` (stable, lowercase).
- Payload field: `schema_version` (int) for evolution.
- Message **key** = `recording_id` for all recording pipeline events.

## Topics (MVP)

| Topic | Event type | Key | Producers | Consumers |
|-------|------------|-----|-----------|-----------|
| `bugsathi.recording.uploaded` | `RecordingUploaded` | `recording_id` | Uploads (API/outbox) | Media |
| `bugsathi.recording.frames-extracted` | `FramesExtracted` | `recording_id` | Media | AI Analysis |
| `bugsathi.recording.metadata-collected` | `MetadataCollected` | `recording_id` | Uploads or Media | AI (optional) |
| `bugsathi.analysis.started` | `AnalysisStarted` | `recording_id` | AI | (observability / UI later) |
| `bugsathi.analysis.completed` | `AnalysisCompleted` | `recording_id` | AI | Reports |
| `bugsathi.report.generated` | `ReportGenerated` | `recording_id` | Reports | Sharing / notify |
| `bugsathi.share.created` | `ShareCreated` | `report_id` | Sharing | (audit) |
| `bugsathi.pipeline.dlq` | (wrapped failed) | `recording_id` | Workers | Ops |

> `MetadataCollected` may be embedded inside `RecordingUploaded.metadata` for MVP to reduce topic count. Keep the **name** reserved for clarity in docs.

## Message contracts (conceptual)

```text
RecordingUploaded
  schema_version  1
  recording_id    uuid
  project_id      uuid
  object_key      string
  content_type    string
  byte_size       int64
  checksum        string?
  metadata        { browser, os, viewport, ... }
  correlation_id  string
  occurred_at     timestamp

FramesExtracted
  schema_version  1
  recording_id    uuid
  frame_keys      []string
  thumb_key       string
  duration_ms     int64
  correlation_id  string
  occurred_at     timestamp

AnalysisStarted
  schema_version  1
  recording_id    uuid
  report_id       uuid
  prompt_version  string
  correlation_id  string
  occurred_at     timestamp

AnalysisCompleted
  schema_version  1
  recording_id    uuid
  report_id       uuid
  prompt_version  string
  correlation_id  string
  occurred_at     timestamp

ReportGenerated
  schema_version  1
  report_id       uuid
  recording_id    uuid
  project_id      uuid
  correlation_id  string
  occurred_at     timestamp

ShareCreated
  schema_version  1
  share_id        uuid
  report_id       uuid
  expires_at      timestamp?
  correlation_id  string
  occurred_at     timestamp
```

## Ordering guarantees

1. Partition key = `recording_id` on recording/analysis/report pipeline topics.
2. Per-partition max in-flight = 1 (or equivalent sequential processing).
3. Commit Kafka offset only after successful DB + object side effects.

## Idempotency (mandatory)

Kafka retries; it does **not** guarantee uniqueness.

| Handler | If already done | Action |
|---------|-----------------|--------|
| Media / `RecordingUploaded` | Frames exist for recording | Skip ffmpeg; ensure `FramesExtracted` published |
| AI / `FramesExtracted` | Analysis row for `prompt_version` | Skip LLM; ensure downstream events |
| Reports / `AnalysisCompleted` | Report `READY` with same version | No-op |
| Uploads complete | Status already `UPLOADED+` | No-op |

Prefer **deterministic object keys** so re-upload of frames overwrites the same keys.

## Outbox

```text
Worker or API transaction:
  1. Mutate aggregate (legal transition)
  2. INSERT outbox(event_type, payload, partition_key, correlation_id)
Commit

Relay:
  READ unpublished → PRODUCE Kafka → MARK published
```

## Backpressure

- Media lag → scale media consumers / partitions.
- AI lag → isolate on `frames-extracted` topic; do not block media.
- DLQ for poison messages after N attempts.
