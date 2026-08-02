# ADR 0009: ffmpeg Only in Media Worker

## Status

Accepted

## Context

Frame extraction is CPU-heavy and slow relative to HTTP request budgets. Running ffmpeg inside the API process couples user latency to media work and prevents independent scaling.

## Decision

```text
API  →  never calls ffmpeg
         only: auth, metadata, presign, complete upload, outbox

Media Worker
  → download source from MinIO
  → run ffmpeg
  → upload frames/thumb to MinIO
  → update Postgres (artifacts + recording status)
  → outbox FramesExtracted
```

ffmpeg is installed in the **worker** container image (or invoked via an explicit sidecar later). The API image stays slim.

## Consequences

**Positive**

- Clear scalability and failure isolation.
- Interview-friendly “don’t do heavy work on the request path” principle.

**Negative**

- Local Compose must include ffmpeg in worker image.
- Debugging requires worker logs + MinIO, not only API logs.

## Alternatives

| Alternative | Rejected because |
|-------------|------------------|
| Sync ffmpeg in upload handler | Timeouts; no retry/order story |
| Separate ffmpeg microservice day 1 | Premature; worker boundary enough |
