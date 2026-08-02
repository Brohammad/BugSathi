# BugSathi

Local-first, AI-native bug reporting platform — built as a production-grade systems learning project (not a BetterBugs clone).

## Current status

**Milestone 3 — Authentication**

| Doc | Path |
|-----|------|
| Auth design | [docs/architecture/MILESTONE-3-AUTH.md](docs/architecture/MILESTONE-3-AUTH.md) |
| Auth ADR | [docs/adr/0011-authentication.md](docs/adr/0011-authentication.md) |
| System design | [docs/architecture/MILESTONE-1-SYSTEM-DESIGN.md](docs/architecture/MILESTONE-1-SYSTEM-DESIGN.md) |
| Roadmap | [docs/roadmap/ROADMAP.md](docs/roadmap/ROADMAP.md) |

## Quick start

```bash
cp .env.example .env
make up
make migrate
make run-api
```

```bash
# Register
curl -s localhost:8080/v1/auth/register -H 'Content-Type: application/json' \
  -d '{"email":"dev@example.com","password":"password123","name":"Dev"}'

# Me (use access_token from register/login response)
curl -s localhost:8080/v1/auth/me -H "Authorization: Bearer $ACCESS"
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
