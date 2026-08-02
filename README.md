# Bugbot

Local-first, AI-native bug reporting platform — built as a production-grade systems learning project (not a BetterBugs clone).

## Current status

**Milestone 1 — Project Planning & System Architecture** (no application code yet)

| Doc | Path |
|-----|------|
| System design | [docs/architecture/MILESTONE-1-SYSTEM-DESIGN.md](docs/architecture/MILESTONE-1-SYSTEM-DESIGN.md) |
| Diagrams | [docs/architecture/DIAGRAMS.md](docs/architecture/DIAGRAMS.md) |
| Event flow | [docs/architecture/EVENT-FLOW.md](docs/architecture/EVENT-FLOW.md) |
| ADRs | [docs/adr/](docs/adr/) |
| Roadmap | [docs/roadmap/ROADMAP.md](docs/roadmap/ROADMAP.md) |

## MVP capabilities (planned)

- Screen recording upload → MinIO
- Frame extraction → ffmpeg workers
- AI summary + reproduction steps (provider port)
- Browser/OS metadata
- Projects/workspaces
- Shareable report links
- Ordered pipeline via Kafka partition keys
- Internal gRPC; public HTTP/JSON
- PostgreSQL metadata

## Philosophy

Teach → decide → implement one milestone at a time. Approve before the next milestone starts.
