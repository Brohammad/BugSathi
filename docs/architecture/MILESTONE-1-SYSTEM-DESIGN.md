# Milestone 1 — Project Planning & System Architecture

> **Status:** Approved (with refinements)  
> **Code:** None in M1. Architecture and decisions only.

---

## 1. Problem Statement

Bug reporting today is fragmented:

1. A user records a screen or takes screenshots.
2. They paste media into Slack/Jira/Linear.
3. They manually write steps, environment, and expected vs actual.
4. Engineers reconstruct context from incomplete reports.

**BetterBugs** (and similar tools) productize this: capture → process media → AI-summarize → shareable report.

**Our goal is not to clone BetterBugs.** We build a **local-first, AI-native bug reporting platform** that is a deliberate vehicle for learning:

| Domain | What this product forces you to learn |
|--------|----------------------------------------|
| Distributed systems | Ordered pipelines, retries, idempotency |
| Media processing | Blob storage, codecs, frame extraction |
| AI orchestration | Provider abstraction, prompts, cost/latency |
| Backend engineering | Auth, tenancy, APIs, observability, deploy |

---

## 2. Real-World Motivation

At FAANG scale, “a bug report” is rarely a form. It is a **pipeline**:

```
capture → store → transform → enrich → index → notify → collaborate
```

Each stage has different SLAs, failure modes, and scaling characteristics. Interviewers care whether you can name those stages and defend boundaries—not whether you used the trendy framework.

---

## 3. Functional Requirements (MVP)

| ID | Requirement | Notes |
|----|-------------|-------|
| F1 | User authentication | Register / login / session |
| F2 | Projects (workspaces) | Multi-project tenancy |
| F3 | Screen recording upload | Large binary → object storage |
| F4 | Screenshot extraction | Frames from video |
| F5 | Browser / OS metadata | Attached to report |
| F6 | AI bug summary | Title + description |
| F7 | AI reproduction steps | Ordered steps |
| F8 | Shareable report links | Public or tokenized URL |
| F9 | Background processing | Async after upload |
| F10 | Ordered processing per upload | Kafka partition key + sequential worker |

Out of MVP (later): annotations, browser extension, live collab editing, SSO, billing.

---

## 4. Non-Functional Requirements

| ID | Category | Target |
|----|----------|--------|
| N1 | Local-first | Entire stack runs on one machine via Docker Compose |
| N2 | Durability | Upload ACK only after object storage + DB metadata persist |
| N3 | Ordering | Per-recording pipeline stages never race |
| N4 | Idempotency | Replayed Kafka messages must not duplicate reports |
| N5 | Observability | `request_id` / `recording_id` / `correlation_id` in logs from M2; full metrics/traces in M11 |
| N6 | Security | Authn/z, signed share links, no secrets in repo |
| N7 | Testability | Unit + integration tests; interfaces at boundaries |
| N8 | Extensibility | Swap AI provider / storage without rewriting domain |
| N9 | Production path | Same architecture runs locally and in cloud (config only) |

---

## 5. System Constraints

1. **Solo / learning pace** — modular monolith first; not a fleet of 12 microservices.
2. **Local hardware** — video processing and AI calls must be bounded (chunk sizes, timeouts).
3. **Cost control** — AI behind an interface; mock provider for tests/offline.
4. **Kafka locally** — accept operational complexity for the learning payoff (ordering, consumer groups).
5. **HTTP for browsers; workers write their own DB** — gRPC reserved for cross-service behavior later (`api/proto/v1/`).
6. **ffmpeg only in Media Worker** — never on the API request path.

---

## 6. User Journey (MVP)

```text
┌──────────┐     ┌───────────┐     ┌────────────┐     ┌─────────────┐
│  Sign up │────▶│  Create   │────▶│  Upload    │────▶│  Wait for   │
│  / login │     │  project  │     │  recording │     │  processing │
└──────────┘     └───────────┘     └────────────┘     └──────┬──────┘
                                                             │
                                                             ▼
┌──────────┐     ┌───────────┐     ┌────────────┐     ┌─────────────┐
│  Share   │◀────│  Review   │◀────│  AI steps  │◀────│  Frames +   │
│  link    │     │  report   │     │  + summary │     │  metadata   │
└──────────┘     └───────────┘     └────────────┘     └─────────────┘
```

**States of a recording/report:** see [STATE-MACHINES.md](STATE-MACHINES.md).

```text
Recording: UPLOADING → UPLOADED → PROCESSING → READY | FAILED
Report:    PENDING → GENERATING → READY | FAILED
```

Further detail: [BOUNDED-CONTEXTS.md](BOUNDED-CONTEXTS.md), [AGGREGATES.md](AGGREGATES.md), [DATA-LIFECYCLE.md](DATA-LIFECYCLE.md).
---

## 7. Why Modular Monolith First (Not Microservices)

### First principles

A **microservice** is a deployable unit with an independent lifecycle. You pay:

- network latency & partial failure
- distributed transactions (or eventual consistency)
- versioning & ops overhead

You buy: independent scaling, team autonomy, blast-radius isolation.

### Industry approaches

| Approach | When |
|----------|------|
| Modular monolith | Early product, one team, unclear boundaries |
| Microservices | Multiple teams, different scale profiles, proven domain seams |
| Serverless functions | Spiky, short-lived work; careful with long media jobs |

FAANG often starts **monolith-shaped** and extracts services when a module’s scale or ownership diverges (e.g., media transcoding farm vs API).

### Our choice

**Modular monolith** with clear bounded contexts + **Kafka** for async boundaries. Workers persist via **their own repositories**. gRPC/protobuf under `api/proto/v1/` for future behavioral RPCs — not as a write gateway.

```text
Today: one deployable "api" process + workers (can be same binary, different entrypoints)
Tomorrow: extract media-worker and ai-worker as separate deployments without rewriting domain
```

### Alternatives considered

| Option | Why not (now) |
|--------|----------------|
| Pure CRUD + sync ffmpeg in request | Blocks HTTP; no ordering story; teaches little |
| Full microservices day 1 | Ops tax dominates learning; premature boundaries |
| Only Redis queues | Weaker ordering/retention/replay story than Kafka for this curriculum |

---

## 8. High-Level Architecture

```text
                         ┌─────────────────────────────────────┐
                         │            Clients                  │
                         │   Web UI  ·  (future: extension)    │
                         └─────────────────┬───────────────────┘
                                           │ HTTPS / JSON
                                           ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                     Modular Monolith (API process)                       │
│  Auth · Projects · Uploads · Reports · Sharing                           │
│  (HTTP edge + own-context DB writes + outbox produce)                    │
└───────────────┬──────────────────────┬───────────────────┬───────────────┘
                │                      │                   │
                ▼                      ▼                   ▼
         ┌─────────────┐      ┌──────────────┐    ┌────────────────┐
         │ PostgreSQL  │      │ MinIO (S3)   │    │ Kafka          │
         │  metadata   │◀─────│  objects     │    │  events       │
         └──────▲──────┘      └──────▲───────┘    └───────┬────────┘
                │                    │                    │
                │                    │                    ▼
                │             ┌──────┴─────────────────────────────┐
                │             │         Worker process(es)         │
                └─────────────│  Media (ffmpeg) · AI (AnalyzerPort)│
                  direct repo │  own DB txs + outbox events        │
                  writes      └────────────────────────────────────┘
```

**Local-first:** Docker Compose runs Postgres, MinIO, Kafka (KRaft), API, workers. Cloud later swaps MinIO→S3, Compose→K8s, same ports.

---

## 9. Service / Module Boundaries

| Module | Responsibility | Owns data |
|--------|----------------|-----------|
| `auth` | Identity, sessions/JWT, password hashing | users, sessions |
| `projects` | Workspaces, membership | projects, members |
| `uploads` | Initiate upload, complete, metadata | recordings, blobs refs |
| `media` | Frame extraction, thumbnails | media_artifacts |
| `ai` | Summaries, repro steps | analysis_jobs, results |
| `reports` | Assemble bug report entity | reports |
| `sharing` | Public tokens, link expiry | share_links |
| `platform` | config, logging, DI, health | — |

Workers consume Kafka and persist through **their context repositories** (direct Postgres). See [BOUNDED-CONTEXTS.md](BOUNDED-CONTEXTS.md). Cross-context table access only via owning context’s interfaces.

---

## 10. Event Flow Overview (Ordered Pipeline)

**Invariant:** For a given `recording_id`, stages run in order. Concurrent recordings may run in parallel.

### Partitioning strategy

```text
Kafka topic: recording.pipeline
Key:         recording_id
→ Same recording always lands on same partition
→ One consumer per partition processes sequentially
→ Ordering within recording preserved
```

### Happy path

```text
1. Client uploads bytes → MinIO (presigned PUT)
2. API writes Recording(UPLOADED) + outbox RecordingUploaded
3. Media worker: ffmpeg → frames → DB + outbox FramesExtracted
4. AI worker: AnalyzerPort → analyses/reports + AnalysisCompleted / ReportGenerated
5. User opens report / creates ShareLink (ShareCreated)
```

Full walkthrough: [DATA-LIFECYCLE.md](DATA-LIFECYCLE.md). Event names: [EVENT-FLOW.md](EVENT-FLOW.md).
### Failure scenarios

| Failure | Behavior |
|---------|----------|
| Upload to MinIO fails | No Kafka event; client retries |
| Kafka produce fails after DB write | Outbox pattern (M5/M6) or compensating retry |
| Media worker crash mid-job | Kafka redelivery; idempotent frame writes |
| AI provider timeout | Retry with backoff; mark FAILED after N attempts |
| Poison message | Dead-letter topic + alert |

---

## 11. Tech Stack Selection

| Layer | Choice | Why |
|-------|--------|-----|
| Language | **Go** | Concurrency model, gRPC ergonomics, single static binary, interview-friendly systems language |
| HTTP API | Chi or Echo (thin) | Explicit middleware; no magic |
| RPC | gRPC + protobuf | Typed internal contracts; future service extraction |
| DB | **PostgreSQL 16** | Relational integrity for users/projects/reports; JSONB where useful |
| Object storage | **MinIO** (S3 API) | Local-first; swap to AWS S3 with endpoint change |
| Messaging | **Apache Kafka API** via **Redpanda** locally (ADR-0010) | Ordering via keys, replay, consumer groups — curriculum core |
| Media | ffmpeg (sidecar/CLI) | Industry standard for frame extraction |
| AI | Port + OpenAI-compatible adapter (+ mock) | Provider-agnostic |
| Auth | JWT access + refresh (or session cookies) | Decide in M3 ADR |
| Config | env + typed config struct | 12-factor |
| Logging | structured JSON (slog / zap) | Machine-parseable |
| Tests | go test + testcontainers | Real Postgres/Kafka in integration tests |
| Containers | Docker Compose | Local parity |

### Why Go (not Node/Python/Java)?

- **Go:** excellent for workers, gRPC, low-ops binaries; slightly more ceremony for AI SDKs.
- **Python:** best AI/media libs; weaker typed boundaries unless disciplined.
- **Node:** fine for API; media/AI workers often still shell out.
- **Java/Kotlin:** enterprise strength; heavier for a learning solo project.

**Decision: Go** for systems depth + one language across API and workers.

*(If you strongly prefer TypeScript or Python, we can revise before M2 — say so.)*

---

## 12. Database Choice — Deep Dive

**PostgreSQL** for metadata:

- ACID for auth, membership, report state transitions
- Unique constraints → idempotency keys
- JSONB for flexible AI output / client metadata
- Mature Kafka outbox / listen-notify patterns later

**Not chosen now:**

| Store | Role later (optional) |
|-------|------------------------|
| Redis | Cache, rate limits, ephemeral presence (M10/M13) |
| Elasticsearch/OpenSearch | Full-text search across reports |
| Vector DB | Semantic search over reports (post-MVP) |

Object bytes **never** live in Postgres. Only keys/URLs/checksums.

---

## 13. Object Storage Choice

**MinIO** (S3-compatible):

- Local Docker volume
- Presigned URLs (same as AWS)
- Bucket layout:

```text
s3://bugbot/
  projects/{project_id}/recordings/{recording_id}/source.webm
  projects/{project_id}/recordings/{recording_id}/frames/{n}.jpg
  projects/{project_id}/recordings/{recording_id}/thumb.jpg
```

Cloud migration: change endpoint + credentials; keep key schema.

---

## 14. AI Provider Abstraction

```text
┌─────────────────────────────────────────┐
│              AI Analyzer                │
│                                         │
│   depends on: AnalyzerPort (interface)  │
│                                         │
│   Analyze(ctx, input) → BugAnalysis     │
└───────────────────┬─────────────────────┘
                    │
        ┌───────────┼───────────┐
        ▼           ▼           ▼
   OpenAIAdapter  MockAdapter  (future: Anthropic, local Ollama)
```

**Input:** frames (or frame URLs), metadata, optional transcript.  
**Output:** title, summary, reproduction steps, confidence.  
**Rules:** timeouts, token budgets, no PII in logs, mock for CI.

---

## 15. Folder Structure (Target)

```text
bugsathi/   # repo: BugSathi
├── docs/
│   ├── adr/
│   ├── architecture/
│   └── roadmap/
├── cmd/
│   ├── api/                      # HTTP entrypoint (slim; no ffmpeg)
│   └── worker/                   # Kafka consumers (ffmpeg in this image)
├── internal/
│   ├── auth/
│   ├── projects/
│   ├── uploads/
│   ├── media/
│   ├── ai/
│   ├── reports/
│   ├── sharing/
│   └── platform/                 # config, logging, db, kafka, correlation IDs
├── api/
│   ├── proto/v1/                 # versioned protobufs
│   └── openapi/                  # public HTTP contract (later)
├── migrations/
├── deploy/
│   ├── docker/
│   └── compose/
├── scripts/
├── test/
│   ├── integration/
│   └── testdata/
├── web/
├── Makefile
├── go.mod
└── README.md
```

Clean architecture per context:

```text
internal/uploads/
  domain/       # entities, aggregates, state transitions
  port/         # interfaces (repos, storage, bus)
  service/      # use cases
  adapter/      # postgres, s3, http handlers
```

---

## 16. Future Migration Path (Monolith → Services)

When justified (load, team, or blast radius):

1. Extract `worker` deployment already (same codebase, different `cmd`).
2. Split Kafka consumer groups: `media` vs `ai`.
3. Promote protobuf contracts; API becomes the only HTTP edge.
4. Move media to GPU/CPU-optimized pool; AI to GPU/rate-limited pool.
5. Introduce API gateway / BFF only if multiple clients need aggregation.

**Do not extract** until a module has a clear scale or failure profile difference.

---

## 17. Development Roadmap (Milestones)

| # | Milestone | Focus |
|---|-----------|--------|
| 1 | Architecture | This document |
| 2 | Dev environment | Compose, Makefile, healthchecks |
| 3 | Authentication | Users, tokens, middleware |
| 4 | Projects | CRUD + membership |
| 5 | Recording upload | Presigned/MinIO + metadata + Kafka produce |
| 6 | Media pipeline | ffmpeg frames, ordering, idempotency |
| 7 | AI pipeline | Port + real/mock adapters |
| 8 | Bug report generation | Assemble READY report |
| 9 | Sharing | Tokenized links |
| 10 | Realtime collaboration | Presence / comments (WS or SSE) |
| 11 | Observability | Metrics, traces, dashboards |
| 12 | Deployment | Cloud/K8s or single-VPS path |
| 13 | Performance | Caching, backpressure, profiling |
| 14 | Production hardening | Security, DLQ, chaos, SLOs |

---

## 18. Complexity Implications (Big-O / Systems)

- **Upload:** O(size) network + storage; API should not hold whole file in memory (streaming / presigned).
- **Frame extract:** O(duration × fps_sample); bound sampling rate.
- **AI:** O(frames_sent × tokens); dominate cost/latency — batch/select keyframes.
- **Kafka ordering:** throughput per partition is serial; scale by more recordings (keys), not by parallelizing one recording.

---

## 19. Scalability Considerations

| Bottleneck | Scale lever |
|------------|-------------|
| API | Horizontal replicas (stateless) |
| Postgres | Read replicas later; careful with writes |
| MinIO/S3 | Virtually unlimited object scale |
| Media CPU | More worker replicas; more partitions |
| AI rate limits | Queue depth, backoff, multiple keys/providers |
| Ordering | More partitions for more parallelism across recordings |

---

## End of Milestone 1 design narrative

Implementation begins only after approval → **Milestone 2: Development Environment**.
