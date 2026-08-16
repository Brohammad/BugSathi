# ADR 0026: Object Cleanup on Project Delete

## Status

Accepted

## Context

`DELETE /v1/projects/{id}` cascades Postgres rows (recordings, artifacts, reports,
shares, comments) but left MinIO objects in place. Over time that orphans
source videos, frames, and thumbs under `projects/{id}/…` (ADR 0003).

There is no per-recording delete API yet; recordings disappear only via project
cascade.

## Decision

1. After a successful project DB delete, the API best-effort deletes every
   object under prefix `projects/{project_id}/` via `ObjectStore.DeletePrefix`.
2. MinIO failures are logged and **do not** fail the HTTP delete (204 still).
   Retrying after a DB success cannot re-find the project; failing the request
   would strand the client with a deleted project and a confusing error.
3. Missing keys are treated as success so cleanup is idempotent.
4. Abandoned `UPLOADING` sessions whose project still exists are **out of
   scope** (lifecycle sweeper later; ADR 0003 / 0013).

## Consequences

**Positive** — project delete removes both metadata and bytes; prefix cleanup
covers source + frames + thumb without enumerating DB keys after cascade.  
**Negative** — a MinIO outage during delete leaves orphans until a future
reconcile job; no recording-level delete API yet.

## Alternatives

| Alternative | Rejected |
|-------------|----------|
| MinIO first, then DB | Leaves a live project with no media if DB delete fails |
| Fail HTTP when MinIO fails | Client cannot retry usefully after cascade |
| List DB keys only (no prefix) | Misses any object written under the prefix but not in DB |
| Async outbox cleanup worker | Correct long-term, heavier than needed for M20 |
