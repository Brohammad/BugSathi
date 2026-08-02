# Data Lifecycle

End-to-end journey of one bug capture. This is the primary mental model of the system.

```text
Browser
  │
  │ 1. Auth + select Project
  ▼
API (Uploads)
  │ 2. Create Recording (UPLOADING), return presigned PUT URL
  ▼
Browser ──PUT bytes──▶ MinIO
  │
  │ 3. Complete upload
  ▼
API (Uploads)
  │ 4. Verify object (optional HEAD), Recording → UPLOADED
  │ 5. Insert outbox row: RecordingUploaded  } same DB tx
  │ 6. Outbox relay publishes to Kafka (key = recording_id)
  ▼
Kafka topic: bugsathi.recording.uploaded
  │
  ▼
Media Worker
  │ 7. Idempotent check: frames already exist? → skip extract, still ensure event
  │ 8. Download source from MinIO
  │ 9. Run ffmpeg (API never calls ffmpeg)
  │10. Upload frames + thumb to MinIO
  │11. Tx: insert media_artifacts, Recording → PROCESSING → READY,
  │        outbox FramesExtracted
  ▼
Kafka topic: bugsathi.recording.frames-extracted
  │
  ▼
AI Worker
  │12. Idempotent check: analysis for prompt_version exists? → skip LLM
  │13. AnalysisStarted (status GENERATING on Report)
  │14. AnalyzerPort.Analyze(keyframes + metadata)
  │15. Tx: upsert analysis + report content, Report → READY,
  │        outbox AnalysisCompleted / ReportGenerated
  ▼
Kafka (optional notify) / DB read model ready
  │
  ▼
User opens Report (HTTP)
  │
  │16. Create ShareLink
  ▼
ShareCreated → public GET /s/{token}
```

## Where bytes live

| Artifact | Store | Key pattern |
|----------|-------|-------------|
| Source recording | MinIO | `projects/{project_id}/recordings/{recording_id}/source.{ext}` |
| Frames | MinIO | `.../frames/{index:05d}.jpg` |
| Thumb | MinIO | `.../thumb.jpg` |
| Metadata / status | Postgres | `recordings`, `media_artifacts`, `reports`, … |

## Who writes what

| Actor | Writes |
|-------|--------|
| API | `recordings` (upload path), `users`, `projects`, `share_links`, outbox for upload events |
| Media Worker | `media_artifacts`, `recordings.status`, outbox `FramesExtracted` |
| AI Worker | `analyses`, `reports`, outbox `AnalysisCompleted` / `ReportGenerated` |

API is **not** the write gateway for workers.

## Correlation

Every hop logs and propagates:

| ID | Meaning |
|----|---------|
| `request_id` | One HTTP request |
| `recording_id` | Aggregate under processing |
| `correlation_id` | End-to-end pipeline (often = recording_id or upload request_id) |

Kafka headers carry `correlation_id` + `recording_id`.
