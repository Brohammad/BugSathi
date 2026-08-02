# ADR 0007: Local-First Architecture & Deploy Parity

## Status

Accepted

## Context

The product must run fully on a developer laptop to maximize learning feedback loops, while remaining faithful to production topology (API, workers, Postgres, object storage, Kafka).

## Decision

- **Docker Compose** is the system of record for local runtime dependencies.
- Application config differs by environment (**env vars**), not by code forks.
- Same S3 API, same SQL migrations, same Kafka topic names in local and prod.
- No cloud account required until Milestone 12.
- “Local-first” means **local control plane + data plane**, not “offline-only AI” (AI may call out; Mock adapter covers offline).

## Consequences

**Positive**

- Reproducible onboarding; CI can boot compose for integration tests.
- Production migration is configuration + hosting, not rewrite.

**Negative**

- Laptop resource use (Kafka especially).
- Compose ≠ K8s; M12 must teach the delta (probes, scheduling, secrets).

## Alternatives

| Alternative | Rejected because |
|-------------|------------------|
| Cloud-dev from day 1 | Cost + latency for learning loop |
| SQLite + local disk + goroutines | Too far from target production architecture |
| Nix-only / bare metal installs | Harder onboarding across OS |
