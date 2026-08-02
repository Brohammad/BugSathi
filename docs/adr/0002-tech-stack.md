# ADR 0002: Tech Stack

## Status

Accepted

## Context

The project must teach systems engineering (concurrency, networking, messaging) while remaining implementable by one developer. Stack choices should maximize interview-transferable concepts and production realism.

## Decision

| Concern | Choice |
|---------|--------|
| Language | Go 1.22+ |
| Public API | HTTP/JSON |
| Internal RPC | gRPC + Protobuf |
| Metadata DB | PostgreSQL 16 |
| Object storage | MinIO (S3 API) |
| Messaging | Apache Kafka (KRaft mode locally) |
| Media | ffmpeg |
| AI | Port + OpenAI-compatible adapter + Mock |
| Config | Environment variables → typed struct |
| Logging | Structured JSON (`log/slog` or zap) |
| Containers | Docker Compose |

## Consequences

**Positive**

- One language for API and workers.
- S3/Kafka/Postgres are industry-standard interview topics.
- Static binaries simplify deployment lessons.

**Negative**

- ffmpeg/AI SDKs are thinner in Go than Python (shell-out / HTTP is fine).
- Kafka local footprint is heavier than Redis.

## Alternatives

| Alternative | Notes |
|-------------|-------|
| TypeScript full stack | Faster UI; weaker systems signaling in interviews |
| Python | Best for AI/media; need strict architecture to stay clean |
| Redis Streams instead of Kafka | Simpler ops; weaker replay/ordering curriculum |
| NATS / RabbitMQ | Valid; Kafka chosen for partition-key ordering pedagogy |

## Revisit triggers

- Strong preference for TS/Python before Milestone 2 starts.
- Kafka too heavy for the machine → document fallback to Redpanda (Kafka API compatible).
