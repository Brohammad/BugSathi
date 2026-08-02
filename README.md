# BugSathi

Local-first, AI-native bug reporting platform — built as a production-grade systems learning project (not a BetterBugs clone).

## Current status

**Milestone 1 — Approved**  
**Milestone 2 — Development Environment** (this tree)

| Doc | Path |
|-----|------|
| System design | [docs/architecture/MILESTONE-1-SYSTEM-DESIGN.md](docs/architecture/MILESTONE-1-SYSTEM-DESIGN.md) |
| Data lifecycle | [docs/architecture/DATA-LIFECYCLE.md](docs/architecture/DATA-LIFECYCLE.md) |
| Dev environment | [docs/architecture/MILESTONE-2-DEV-ENVIRONMENT.md](docs/architecture/MILESTONE-2-DEV-ENVIRONMENT.md) |
| ADRs | [docs/adr/](docs/adr/) |
| Roadmap | [docs/roadmap/ROADMAP.md](docs/roadmap/ROADMAP.md) |

## Quick start (Milestone 2)

```bash
cp .env.example .env
make up          # Postgres, MinIO, Redpanda
make build
make run-api     # :8080  → GET /healthz /readyz
make run-worker  # :8081  → GET /healthz /readyz
make test
make down
```

Requires Docker Compose and Go 1.24+. A local toolchain may live under `.tools/go` (gitignored).

## MVP capabilities (planned)

- Screen recording upload → MinIO
- Frame extraction → ffmpeg in **Media Worker only**
- AI summary + reproduction steps (provider port)
- Browser/OS metadata
- Projects/workspaces
- Shareable report links
- Ordered pipeline via Kafka partition keys
- Workers write Postgres directly; HTTP API for browsers
- PostgreSQL metadata

## Philosophy

Teach → decide → implement one milestone at a time. Approve before the next milestone starts.
