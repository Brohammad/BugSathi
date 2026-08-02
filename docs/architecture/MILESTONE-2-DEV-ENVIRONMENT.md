# Milestone 2 — Development Environment

> **Status:** Implemented — awaiting your approval before Milestone 3  
> **Depends on:** Milestone 1 approved

---

## Problem statement

Before domain features, we need a **reproducible runtime**: databases, brokers, object storage, and two process entrypoints (API + worker) that boot, log structured JSON, and expose health checks. Without this, every later milestone fights environment drift.

## Real-world motivation

FAANG onboarding almost always starts with “clone → one command → stack up.” That command is usually Compose, Tilt, or an internal devbox. The CS concepts hiding underneath:

| Concept | Where it appears |
|---------|------------------|
| Process vs container | API/worker images vs host `go run` |
| Service discovery | Compose DNS names (`postgres`, `kafka`, `minio`) |
| Health vs readiness | liveness (`/healthz`) vs dependency checks (`/readyz`) |
| Configuration | 12-factor env → typed config; never commit secrets |
| Observability seeds | `request_id` / `correlation_id` in context from day one |

## Architecture choices for M2

1. **Docker Compose for dependencies** (Postgres, MinIO, Kafka/Redpanda). App binaries can run on host for fast iteration *or* in Compose.
2. **Redpanda** instead of full Apache Kafka broker image when possible — Kafka API compatible, lighter on laptops (ADR revisit of 0002).
3. **Two entrypoints**, one module: `cmd/api`, `cmd/worker`.
4. **No domain features yet** — only platform skeleton + health.

## Alternatives

| Option | Tradeoff |
|--------|----------|
| Install Postgres/Kafka on host | Faster sometimes; unreproducible across machines |
| Devcontainer only | Great later; Compose is enough now |
| Full K8s locally (kind/minikube) | Correct for M12; overkill for M2 |

## What we implement now

- `deploy/compose/docker-compose.yml` — postgres, minio, redpanda, (optional app services)
- `internal/platform/{config,logging,httpx}` — config + slog + request IDs
- `cmd/api`, `cmd/worker` — `/healthz`, `/readyz`
- `api/proto/v1/health` — versioned stub
- `Makefile`, `.env.example`, `.gitignore`, GitHub Actions CI (build/test)
- Worker Dockerfile notes ffmpeg for later (image may install ffmpeg now for parity)

## Out of scope

Auth, uploads, Kafka consumers, migrations with business tables (empty migrate hook only).
