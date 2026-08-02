# ADR 0017: Shareable Report Links

## Status

Accepted

## Context

Engineers outside the workspace need read-only access to a bug report without a BugSathi account.

## Decision

1. `share_links` table: opaque high-entropy `token` (stored as-is; entropy is the secret), optional `expires_at`, `revoked_at`.
2. Auth endpoints (project member):
   - `POST /v1/projects/{projectID}/reports/{reportID}/shares`
   - `GET /v1/projects/{projectID}/reports/{reportID}/shares`
   - `DELETE /v1/projects/{projectID}/shares/{shareID}` (revoke)
3. Public (no auth): `GET /s/{token}` — limited fields: title, summary, steps, status, frame URLs (presigned), no internal IDs beyond report id optional.
4. Outbox event `ShareCreated` (topic `bugsathi.share.created`) for audit/future notify.
5. Only `READY` reports can be shared.

## Consequences

**Positive** — simple share UX; revocable.  
**Negative** — token leakage = read access until revoke/expiry (use long random tokens).
