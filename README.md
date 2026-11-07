# BugSathi

Local-first, AI-native bug reporting platform — Go API, ordered Kafka pipeline, MinIO, ffmpeg, and AI-drafted reports.

**Portfolio:** [Case study](https://brohammad.tech/projects/bugsathi) · [Engineer](https://brohammad.tech) · **Metrics:** 116 Go tests (`make test-race`), 35 ADRs in [`docs/adr/`](docs/adr/)

## Why it exists

Bug reports lose evidence. BugSathi keeps processing inspectable: upload → object storage → ordered events → media workers → schema-bound report, with JWT auth and OpenTelemetry so you can trace failures across API and workers.

Built as a production-grade systems learning project (not a BetterBugs clone).

## Current status

**Milestone 29 — grounded multimodal analysis** (auth, upload, pipeline, web UI, production Compose, and prod AI/MinIO guards are on `master`)

```bash
# API + worker (separate terminals)
make up && make migrate
make run-api
make run-worker

# Browser UI
make web-dev
# open http://localhost:5173
```

```bash
# after auth + create project:
curl -s localhost:8080/v1/projects/$PID/recordings -H "Authorization: Bearer $ACCESS" \
  -H 'Content-Type: application/json' \
  -d '{"content_type":"video/webm","filename":"bug.webm","metadata":{"browser":"chrome"}}'
# PUT bytes to upload_url, then:
curl -s -X POST localhost:8080/v1/projects/$PID/recordings/$RID/complete \
  -H "Authorization: Bearer $ACCESS"

# after worker finishes AI:
curl -s localhost:8080/v1/projects/$PID/recordings/$RID/report \
  -H "Authorization: Bearer $ACCESS"
```

## Quick start

```bash
cp .env.example .env
make up
make migrate
make run-api
```

```bash
# Auth
curl -s localhost:8080/v1/auth/register -H 'Content-Type: application/json' \
  -d '{"email":"dev@example.com","password":"password123","name":"Dev"}'

# Projects (Bearer access token)
curl -s localhost:8080/v1/projects -H "Authorization: Bearer $ACCESS" \
  -H 'Content-Type: application/json' -d '{"name":"Demo"}'
```

Requires Docker Compose, Go 1.25+, and Node 20+ for the UI. A local Go toolchain may live under `.tools/go` (gitignored).

## MVP capabilities

- Screen recording upload → MinIO
- Frame extraction → ffmpeg in **Media Worker only**
- AI summary + reproduction steps (provider port)
- Browser/OS metadata
- Projects/workspaces
- Shareable report links
- Ordered pipeline via Kafka partition keys
- Workers write Postgres directly; HTTP API for browsers
- React web UI (`web/`) for capture + report review
- PostgreSQL metadata

## Architecture

| Topic | Doc |
|--------|-----|
| Diagrams | [`docs/architecture/DIAGRAMS.md`](docs/architecture/DIAGRAMS.md) |
| Event flow | [`docs/architecture/EVENT-FLOW.md`](docs/architecture/EVENT-FLOW.md) |
| Bounded contexts | [`docs/architecture/BOUNDED-CONTEXTS.md`](docs/architecture/BOUNDED-CONTEXTS.md) |
| Data lifecycle | [`docs/architecture/DATA-LIFECYCLE.md`](docs/architecture/DATA-LIFECYCLE.md) |

```text
Browser → API (Go) → MinIO + Postgres
              ↓
         Kafka (per-recording ordering)
              ↓
    Worker: ffmpeg frames → AI analysis → report
```

## Configuration

Copy `.env.example` → `.env`. Docker Compose brings up Kafka/Redpanda, Postgres, MinIO, Redis, and observability stacks. See `Makefile` targets: `up`, `up-prod`, `migrate`.

## Testing

```bash
make test        # all packages
make test-race   # CI gate
make ci          # fmt, vet, race, coverage, web lint/build
```

## Deployment

- **Local:** `make up` + `make run-api` / `make run-worker`
- **Production Compose:** `make up-prod` (see `docs/operations/`)
- **Kubernetes:** manifests under `deploy/` with Caddy ingress

## Design decisions

35 architecture decision records: [`docs/adr/`](docs/adr/).

## Limitations

- AI provider requires configuration for non-mock analysis; production guards enforce MinIO/AI settings.
- Milestone-driven scope — see **Current status** above before expecting SaaS parity.

## Roadmap

[`docs/roadmap/`](docs/roadmap/)

## Contributing

Open issues for defects; ADRs for architectural changes. Run `make ci` before PRs.

## License

See [`LICENSE`](LICENSE) in the repository root.

## Philosophy

Teach → decide → implement one milestone at a time. Approve before the next milestone starts.
