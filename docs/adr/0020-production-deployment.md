# ADR 0020: Production Deployment Topology

## Status

Accepted

## Context

Local Compose runs dependencies only; apps are usually launched with `make run-*`. Production needs the same topology with containerized API/worker, a migrations job, secrets, and health probes — without requiring a full cloud account to learn the pattern.

## Decision

1. **Primary deliverable: Production Compose** (`deploy/compose/docker-compose.prod.yml`)
   - Runs Postgres, MinIO, Redpanda, API, Worker, one-shot migrate, and optional observability.
   - Config via `.env.prod` (never commit real secrets). Same env var names as local (ADR 0007).
2. **Images:** multi-stage Dockerfiles for `api`, `worker`, `migrate` under `deploy/docker/`.
   - Container `HEALTHCHECK` hits `/healthz`; Compose/K8s use `/readyz` for readiness.
3. **Secondary: minimal Kubernetes manifests** under `deploy/k8s/` teaching Deployments, Service, Job (migrate), ConfigMap/Secret, liveness/readiness probes.
4. **Secrets:** Compose `env_file`; K8s `Secret` (example only). No secrets in git.

## Consequences

**Positive** — one path to “full stack in Docker”; K8s teaches the delta (scheduling, probes, secrets).  
**Negative** — Prod Compose is still single-host; not HA.
