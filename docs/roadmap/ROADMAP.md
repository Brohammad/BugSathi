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
- ffmpeg frame extraction
- Artifact keys in DB
- Ordered consume; retries; DLQ hook
- Integration test with sample video

### M7 — AI Analysis Pipeline
- `AnalyzerPort` + OpenAI-compatible + Mock
- Prompt versioning
- Persist summary + reproduction steps
- Cost/latency timeouts

### M8 — Bug Report Generation
- Assemble report aggregate
- Status READY
- Read API for report + frames

### M9 — Sharing
- Share tokens, expiry, revoke
- Public report view (limited fields)

### M10 — Realtime Collaboration
- Presence or comments via SSE/WebSocket
- Auth on channels; fanout design

### M11 — Observability
- Metrics (RED), traces (OTLP), log correlation IDs
- Dashboards for lag, failure rate, AI latency

### M12 — Deployment
- Production Compose or K8s manifests
- Secrets, migrations job, health probes

### M13 — Performance Optimization
- Profiling, connection pools, keyframe selection
- Caching hot report reads

### M14 — Production Hardening
- Rate limits, security headers, chaos/retry drills
- SLOs and runbooks

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

Depth over speed: a milestone can span multiple sessions.
