# BugSathi

Local-first, AI-native bug reporting platform — built as a production-grade systems learning project (not a BetterBugs clone).

## Current status

**Milestone 1 — Approved** (architecture locked with refinements)  
**Next:** Milestone 2 — Development Environment

| Doc | Path |
|-----|------|
| System design | [docs/architecture/MILESTONE-1-SYSTEM-DESIGN.md](docs/architecture/MILESTONE-1-SYSTEM-DESIGN.md) |
| Data lifecycle | [docs/architecture/DATA-LIFECYCLE.md](docs/architecture/DATA-LIFECYCLE.md) |
| Bounded contexts | [docs/architecture/BOUNDED-CONTEXTS.md](docs/architecture/BOUNDED-CONTEXTS.md) |
| Aggregates | [docs/architecture/AGGREGATES.md](docs/architecture/AGGREGATES.md) |
| State machines | [docs/architecture/STATE-MACHINES.md](docs/architecture/STATE-MACHINES.md) |
| Event flow | [docs/architecture/EVENT-FLOW.md](docs/architecture/EVENT-FLOW.md) |
| Diagrams | [docs/architecture/DIAGRAMS.md](docs/architecture/DIAGRAMS.md) |
| ADRs | [docs/adr/](docs/adr/) |
| Roadmap | [docs/roadmap/ROADMAP.md](docs/roadmap/ROADMAP.md) |

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
