# ADR 0013: Recording Upload — Presigned S3 + Transactional Outbox

## Status

Accepted

## Context

Screen recordings are large. Proxying bytes through the API wastes memory/bandwidth and couples upload latency to API capacity. After upload we must start the async pipeline without dual-write bugs (DB committed, Kafka lost).

## Decision

1. **Presigned PUT** to MinIO/S3 — browser uploads directly.
2. Flow:
   - `POST .../recordings` → status `UPLOADING`, return `upload_url` + `recording_id` + `object_key`
   - Client PUT bytes to MinIO
   - `POST .../recordings/{id}/complete` → HEAD object, transition `UPLOADED`, insert **outbox** `RecordingUploaded` in same TX
3. **Outbox relay** in API process publishes to Kafka topic `bugsathi.recording.uploaded` (key=`recording_id`).
4. Complete is **idempotent**: already `UPLOADED+` → 200 no-op.
5. Only project **members** may create/complete/get recordings.

## Consequences

**Positive** — thin API; durable event emission; interview-standard pattern.  
**Negative** — orphan objects if client never calls complete (lifecycle cleanup in hardening).

## Alternatives

| Alternative | Rejected |
|-------------|----------|
| Multipart through API | Memory/timeout pain |
| Produce Kafka in handler without outbox | Dual-write failure |
| Sync media in complete | Violates ADR-0009 |
