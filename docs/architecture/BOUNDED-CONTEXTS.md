# Bounded Contexts

Each context owns its data and publishes/consumes events across seams.
Workers that belong to a context write **their own** tables via repositories — they do not route persistence through the HTTP API.

---

## Auth

| | |
|--|--|
| **Responsibilities** | Register/login, credential verification, session/JWT issuance, auth middleware |
| **Owned tables** | `users`, `sessions` (or refresh tokens) |
| **Publishes** | `UserRegistered` (optional; MVP may skip) |
| **Consumes** | — |
| **Public interface** | HTTP: `POST /auth/register`, `POST /auth/login`, `POST /auth/refresh`, `POST /auth/logout` |

---

## Projects

| | |
|--|--|
| **Responsibilities** | Workspaces/projects, membership, authorization helpers for project-scoped resources |
| **Owned tables** | `projects`, `project_members` |
| **Publishes** | `ProjectCreated` (optional) |
| **Consumes** | — |
| **Public interface** | HTTP: CRUD `/projects`, membership endpoints; internal: `ProjectRepository`, `CanAccess(user, project)` |

---

## Uploads

| | |
|--|--|
| **Responsibilities** | Initiate/complete recording upload, store object keys + client metadata, emit pipeline start via outbox |
| **Owned tables** | `recordings`, `outbox` (or shared platform outbox with Uploads as primary producer) |
| **Publishes** | `RecordingUploaded` |
| **Consumes** | — |
| **Public interface** | HTTP: create upload session, complete upload, get recording; ports: `ObjectStorage`, `EventPublisher` |

**Invariant:** API never runs ffmpeg. Bytes go Browser → MinIO (presigned).

---

## Media

| | |
|--|--|
| **Responsibilities** | Download source from MinIO, run ffmpeg, upload frames/thumb, update recording processing state |
| **Owned tables** | `media_artifacts` (frames, thumb); updates `recordings.status` / `processing_state` |
| **Publishes** | `FramesExtracted` |
| **Consumes** | `RecordingUploaded` |
| **Public interface** | Worker consumer only (no public HTTP); `MediaProcessor` service + repositories |

---

## AI Analysis

| | |
|--|--|
| **Responsibilities** | Select keyframes, call `AnalyzerPort`, persist analysis results, advance report generation |
| **Owned tables** | `analyses` (prompt_version, summary raw, steps raw, status) |
| **Publishes** | `AnalysisStarted`, `AnalysisCompleted` |
| **Consumes** | `FramesExtracted` (and optionally `MetadataCollected` if split) |
| **Public interface** | Worker + `AnalyzerPort`; no public HTTP |

---

## Reports

| | |
|--|--|
| **Responsibilities** | Assemble user-facing bug report aggregate from recording + media + analysis |
| **Owned tables** | `reports` |
| **Publishes** | `ReportGenerated` |
| **Consumes** | `AnalysisCompleted` |
| **Public interface** | HTTP: `GET /projects/{id}/reports/{id}`; worker upserts via `ReportRepository` |

---

## Sharing

| | |
|--|--|
| **Responsibilities** | Tokenized share links, expiry, revoke, public read of limited report fields |
| **Owned tables** | `share_links` |
| **Publishes** | `ShareCreated` |
| **Consumes** | — |
| **Public interface** | HTTP: create/revoke share; public `GET /s/{token}` |

---

## Platform

| | |
|--|--|
| **Responsibilities** | Config, DI, logging, DB pool, Kafka clients, health, correlation IDs, migrations runner |
| **Owned tables** | shared infra only (`schema_migrations`; optionally central `outbox`) |
| **Publishes** | — |
| **Consumes** | — |
| **Public interface** | `GET /healthz`, `GET /readyz`; libraries imported by other contexts |

---

## Cross-cutting rules

1. **No cross-context table writes.** Read another context only via its repository/query API.
2. **Workers own DB transactions** for their context.
3. **gRPC** is for exposing *behavior* across deployable boundaries later — not a persistence gateway for workers in the monolith.
4. **Events are past tense** and versioned in payloads (`schema_version`).
