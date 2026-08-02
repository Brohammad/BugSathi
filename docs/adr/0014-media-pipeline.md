# ADR 0014: Media Pipeline — Kafka Consumer + ffmpeg in Worker

## Status

Accepted

## Context

After `RecordingUploaded`, we must extract frames without blocking the API. Ordering per `recording_id` is required; handlers must be idempotent under at-least-once delivery.

## Decision

1. Worker consumes `bugsathi.recording.uploaded` (consumer group `bugsathi-media`).
2. ffmpeg runs **only in the worker** (ADR-0009).
3. Sample frames with bounded rate (default `fps=0.5`, max 20 frames).
4. Persist `media_artifacts`, set recording `UPLOADED→PROCESSING→READY`, outbox `FramesExtracted` in one TX.
5. Idempotent: if recording already `READY` with artifacts → skip ffmpeg; ensure outbox event exists/published.
6. Failures → mark `FAILED` after logging (retry via Kafka redelivery until max; DLQ later).

## Consequences

**Positive** — clear CPU isolation; replay-safe.  
**Negative** — needs ffmpeg on worker image/host; invalid uploads fail extraction.

## Alternatives

| Alternative | Rejected |
|-------------|----------|
| Extract in API | Violates ADR-0009 |
| Cloud transcoder day 1 | Premature |
