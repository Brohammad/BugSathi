# BugSathi

Local-first, AI-native bug reporting platform — built as a production-grade systems learning project (not a BetterBugs clone).

## Current status

**Milestone 16 — Web UI**

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

Requires Docker Compose, Go 1.24+, and Node 20+ for the UI. A local Go toolchain may live under `.tools/go` (gitignored).

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

## Philosophy

Teach → decide → implement one milestone at a time. Approve before the next milestone starts.
