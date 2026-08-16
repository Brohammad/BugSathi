# ADR Index

| ID | Title | Status |
|----|-------|--------|
| [0001](0001-modular-monolith.md) | Modular monolith first | Accepted |
| [0002](0002-tech-stack.md) | Tech stack (Go, Postgres, MinIO, Kafka, gRPC) | Accepted |
| [0003](0003-object-storage.md) | Object storage via S3 API (MinIO local) | Accepted |
| [0004](0004-ai-provider-port.md) | AI provider port/adapter | Accepted |
| [0005](0005-kafka-ordered-pipeline.md) | Kafka ordered pipeline per recording | Accepted |
| [0006](0006-grpc-internal.md) | Workers write DB directly; gRPC for behavior | Accepted |
| [0007](0007-local-first.md) | Local-first development & deploy parity | Accepted |
| [0008](0008-observability-ids.md) | Correlation IDs from day one | Accepted |
| [0009](0009-ffmpeg-media-worker.md) | ffmpeg only in Media Worker | Accepted |
| [0010](0010-redpanda-local.md) | Redpanda for local Kafka API | Accepted |
| [0011](0011-authentication.md) | Argon2id + JWT + rotating refresh | Accepted |
| [0012](0012-projects.md) | Projects & membership roles | Accepted |
| [0013](0013-recording-upload.md) | Presigned upload + transactional outbox | Accepted |
| [0014](0014-media-pipeline.md) | Media worker + ffmpeg frames | Accepted |
| [0015](0015-ai-analysis.md) | AI AnalyzerPort + mock/openai | Accepted |
| [0016](0016-bug-report-api.md) | Bug report read API | Accepted |
| [0017](0017-sharing.md) | Shareable report links | Accepted |
| [0018](0018-realtime-collaboration.md) | Realtime comments + SSE presence | Accepted |
| [0019](0019-observability-stack.md) | Prometheus metrics + OTLP traces | Accepted |
| [0020](0020-production-deployment.md) | Production Compose + K8s sketches | Accepted |
| [0021](0021-performance.md) | Pools, keyframes, report cache, pprof | Accepted |
| [0022](0022-production-hardening.md) | Rate limits, headers, backoff, SLOs | Accepted |
| [0023](0023-dlq-reprocess.md) | Kafka DLQ + owner reprocess API | Accepted |
| [0024](0024-web-ui.md) | React + Vite web UI | Accepted |
| [0025](0025-media-processing-claim.md) | Media processing claim (expiring lease) | Accepted |

Format: Context → Decision → Consequences → Alternatives.
