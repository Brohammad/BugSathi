# ADR 0016: Bug Report Read API

## Status

Accepted

## Context

AI pipeline already materializes `reports` rows. Clients need an authenticated read model that includes frames for review before sharing.

## Decision

1. Reports context owns **read** aggregation (report + recording meta + media artifacts).
2. HTTP (Bearer + project membership):
   - `GET /v1/projects/{projectID}/reports`
   - `GET /v1/projects/{projectID}/reports/{id}`
   - `GET /v1/projects/{projectID}/recordings/{recordingID}/report`
3. Frame responses include **presigned GET** URLs (short TTL) so the browser never needs MinIO credentials.
4. Writes remain in the AI worker (no duplicate write path in API).

## Consequences

**Positive** — clear CQRS-ish split; safe media access.  
**Negative** — report detail does a few joins/queries (fine at MVP scale).
