# Development Roadmap

## Principles

1. One milestone at a time; stop for approval.
2. Teach → decide → implement → quiz.
3. No technical debt for speed.
4. Tests and interfaces land with the feature, not “later.”

## Milestones

### M1 — Project Planning & System Architecture ✅ (this milestone)
**Deliverables:** system design, diagrams, ADRs, folder structure, stack, event flow.  
**Exit:** architecture approved.

### M2 — Development Environment
- Docker Compose: Postgres, MinIO, Redpanda (Kafka API)
- `cmd/api` + `cmd/worker` with `/healthz` + `/readyz`
- Makefile: `up`, `down`, `test`, `build`, `ci`
- Structured logging + config + request/correlation IDs
- Versioned proto stub `api/proto/v1/health`
- GitHub Actions CI (vet/test/build)

### M3 — Authentication
- User model, argon2id password hashing
- JWT access + rotating opaque refresh tokens
- Auth middleware (`Authorization: Bearer`)
- Migration `0001_auth.sql`, `make migrate`
- Unit tests (memory repos) + HTTP handler tests

### M4 — Projects
- Create/list/get/update/delete project
- Membership roles (owner/member)
- Authorization on all project-scoped routes
- Migration `0002_projects.sql`

### M5 — Recording Upload
- Presigned MinIO upload
- Recording metadata + browser/OS fields (JSONB)
- Outbox → `bugsathi.recording.uploaded`
- Idempotent complete-upload API
- Migration `0003_recordings.sql`

### M6 — Media Processing Pipeline
- Kafka consumer for `RecordingUploaded`
- ffmpeg frame extraction in worker only
- Artifact keys in DB; recording → READY
- Ordered consume; idempotent redelivery
- Migration `0004_media_artifacts.sql`

### M7 — AI Analysis Pipeline
- `AnalyzerPort` + OpenAI-compatible + Mock (default)
- Prompt versioning (`prompt_v1`)
- Persist analysis + upsert READY report
- Outbox `AnalysisCompleted` / `ReportGenerated`
- Migration `0005_ai_reports.sql`

### M8 — Bug Report Generation
- Read API for report aggregate + frames (presigned GET)
- `GET /v1/projects/{id}/reports` and `.../reports/{id}`
- `GET /v1/projects/{id}/recordings/{rid}/report`

### M9 — Sharing
- Share tokens, optional expiry, revoke
- Public `GET /s/{token}` (limited fields + presigned frames)
- Migration `0006_share_links.sql`

### M10 — Realtime Collaboration
- Report comments + SSE presence (`GET .../events`)
- Auth on channels; in-process hub (Redis later)
- Migration `0007_report_comments.sql`

### M11 — Observability
- Prometheus RED metrics on `/metrics` (API + worker)
- OTLP traces (optional via `OTEL_EXPORTER_OTLP_ENDPOINT`)
- Compose: Prometheus, Grafana dashboard, OTel Collector, Jaeger
- Outbox lag + AI latency + pipeline stage metrics

### M12 — Deployment
- Production Compose (`docker-compose.prod.yml`) with migrate job + api + worker
- Docker HEALTHCHECK / Compose readiness on `/healthz` + `/readyz`
- Secrets via `.env.prod` (gitignored); K8s Secret/ConfigMap sketches
- `make up-prod` / `make build-images`

### M13 — Performance Optimization
- Configurable Postgres pool (`POSTGRES_MAX_CONNS`, …)
- Evenly spaced keyframe selection for AI (`AI_MAX_FRAMES`)
- In-process report detail TTL cache (`REPORT_CACHE_TTL`)
- Optional `ENABLE_PPROF=true` → `/debug/pprof/`

### M14 — Production Hardening ✅
- Rate limits, security headers, body caps, Kafka exponential retry
- SLOs and runbooks (`docs/operations/`)
- Chaos drill: `./scripts/chaos-drill.sh` (Postgres stop/start)

### M15 — DLQ + Recording Reprocess
- Max Kafka handler attempts → `{topic}.dlq` + commit source offset
- Owner-gated `POST .../recordings/{id}/reprocess` re-emits `RecordingUploaded`
- Metric `bugsathi_dlq_published_total`

### M16 — Web UI
- Vite + React + TypeScript SPA in `web/`
- Auth, projects, screen/file capture upload, report review, comments, share links
- API CORS via `CORS_ORIGINS` (Vite defaults)
- ADR 0024

### M17 — Kafka consumer at-least-once retries
- Retry the **same** message on handler failure (no fetch-next-on-error skip)
- DLQ + commit only after `KAFKA_RETRY_MAX_ATTEMPTS`
- Transient `FetchMessage` errors backoff-retry instead of killing the worker
- Shared `HandleWithRetries` / `FetchWithRetry` in `internal/platform/kafka`

### M18 — Outbox claim locking
- `WithClaimed` + Postgres `FOR UPDATE SKIP LOCKED` so API and worker relays cannot double-publish
- Publish happens while rows are locked; mark `published_at` in the same transaction
- Concurrent flush tests prove single publish; failed publish leaves row claimable

### M19 — Media processing claim
- `recordings.processing_owner` + `processing_expires_at` lease (migration `0008`)
- One ffmpeg run per recording: overlapping deliveries are skipped and committed
- Heartbeat renews the lease; owner-gated `FinalizeReady` / `MarkFailed` release it
- Expired lease from a dead worker is reclaimed automatically
- Metric `bugsathi_claim_skipped_total`; ADR 0025

### M20 — Object cleanup on project delete
- `DeletePrefix` on MinIO (+ memory) removes `projects/{id}/…` after DB cascade
- HTTP delete stays 204 if MinIO cleanup fails (logged); missing keys are OK
- Abandoned `UPLOADING` GC via `UPLOAD_ABANDONED_TTL`; per-recording delete API for owners
- ADR 0026

### M21 — Trusted proxy & production guards
- `TRUSTED_PROXIES` gates when XFF/X-Real-IP affect rate-limit keys
- `APP_ENV=production` rejects dev JWT/Postgres/MinIO defaults and pprof
- ADR 0027

### M22 — Share defaults & list pagination
- `SHARE_DEFAULT_TTL` / `SHARE_MAX_TTL`; reject non-positive expiry
- `SHARE_HASH_TOKENS` hash-at-rest (required in production); migration `0009`
- Keyset pagination on projects, reports, shares, comments, members
- ADR 0028

### M23 — Atomic auth refresh rotation
- `Rotate` revokes + inserts in one DB transaction
- Reuse/race on refresh maps to HTTP 401 (not 404/500)
- ADR 0029

### M24 — AI result validation & report_id replay fix
- Validate LLM title/summary/steps before persist
- Idempotent replay emits existing `report_id` in outbox events
- Document text-only OpenAI context (not image bytes); ADR 0030

### M25 — Optional Redis for multi-replica API
- `REDIS_URL` enables shared SSE pub/sub, rate limits, report cache
- Default off for local single-replica dev; Compose includes Redis service
- ADR 0031

### M26 — Audit-gap hardening
- AI soft claim / single-flight while analysis is `running` (lease + Kafka skip)
- Worker invalidates Redis report cache after AI complete/fail
- Refresh-token reuse outside grace → revoke all user refresh tokens
- Upload allowlist + Content-Type-bound presign + size cap + ETag checksum
- React report page wires SSE (`EventSource` + `access_token` query)
- ADR 0032

### M27 — Remaining audit gaps (one commit each)
- Kafka client ID, header correlation restore, worker drain
- Redis rate-limit fail-closed + capacity alignment
- Share create/revoke owner-only; project member remove + last-owner guard
- CSP + production HSTS
- Abandoned `UPLOADING` GC; `AnalysisStarted` + report `GENERATING`
- Per-recording delete API
- ADR 0033

### M28 — Deferred audit follow-ups
- Durable Kafka retry attempts in Postgres (`kafka_retry_attempts`)
- `AnalysisStarted` observability consumer + metric
- Lifecycle docs aligned with upload GC / recording delete
- ADR 0034

### M29 — Grounded multimodal analysis
- Provider-neutral, byte-bounded frame reader backed by private object storage
- Chronological selected frames sent as OpenAI image content
- `prompt_v2` prevents text-only analyses from satisfying multimodal replays
- Ten-case grounded-output evaluation gate in `docs/evaluation/VISION-EVAL.md`

## Suggested weekly cadence (flexible)

| Week | Focus |
|------|--------|
| 1 | M1–M2 |
| 2 | M3–M4 |
| 3 | M5–M6 |
| 4 | M7–M8 |
| 5 | M9–M10 |
| 6 | M11–M12 |
| 7 | M13–M14 |
| 8 | M15–M16 |

Depth over speed: a milestone can span multiple sessions.
