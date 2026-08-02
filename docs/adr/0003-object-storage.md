# ADR 0003: Object Storage (MinIO / S3 API)

## Status

Accepted

## Context

Screen recordings are large binary objects. Storing them in PostgreSQL (BYTEA/OID) couples DB backup size to media growth, hurts streaming, and blocks CDN-style access patterns.

## Decision

- Store all media in **S3-compatible object storage**.
- **Local:** MinIO.
- **Production:** AWS S3 / GCS / R2 via same SDK (endpoint + creds).
- Postgres stores only keys, checksums, sizes, content types.
- Prefer **presigned URLs** for upload/download to keep API out of the byte path.

## Bucket key schema

```text
projects/{project_id}/recordings/{recording_id}/source.{ext}
projects/{project_id}/recordings/{recording_id}/frames/{index:05d}.jpg
projects/{project_id}/recordings/{recording_id}/thumb.jpg
```

## Consequences

**Positive**

- Horizontal object scale; cheap migration to cloud.
- API remains thin; teaches real upload patterns.

**Negative**

- Two-phase failure modes (object exists, DB missing — and reverse).
- Need lifecycle rules for abandoned uploads (hardening milestone).

## Alternatives

| Alternative | Rejected because |
|-------------|------------------|
| Postgres BYTEA | Backup/scale nightmare |
| Local filesystem only | Breaks multi-instance workers later |
| Dedicated NFS | Ops complexity without S3 skill transfer |
