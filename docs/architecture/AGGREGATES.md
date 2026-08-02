# Aggregates

Aggregates are consistency boundaries. Commands mutate one aggregate at a time; cross-aggregate work happens via events.

---

## Workspace (optional early; may fold into Project for MVP)

| Field | Notes |
|-------|--------|
| `id` | UUID |
| `name` | Display name |
| `owner_user_id` | Creator |

**MVP simplification:** treat **Project** as the tenancy root; introduce Workspace later if multi-project orgs appear.

---

## Project

| Field | Notes |
|-------|--------|
| `id` | UUID |
| `name` | |
| `created_by` | user id |
| `created_at` | |

**Entities inside:** `ProjectMember` (user_id, role: owner|member).

**Invariants:** at least one owner; members can only access project-scoped recordings/reports.

---

## Recording

| Field | Notes |
|-------|--------|
| `id` | UUID |
| `project_id` | FK |
| `status` | state machine (see STATE-MACHINES.md) |
| `storage_key` | MinIO object key for source |
| `content_type` | e.g. video/webm |
| `byte_size` | |
| `checksum` | optional |
| `metadata` | browser, OS, viewport, user agent (JSONB) |
| `processing_state` | fine-grained worker progress (optional JSON) |
| `created_at` / `updated_at` | |

**Entities inside:** none required; `MediaArtifact` may be a separate aggregate or child entity owned by Media context referencing `recording_id`.

**Invariants:**
- Illegal status transitions rejected.
- `storage_key` set before `UPLOADED`.
- Idempotent complete-upload: second complete is a no-op if already `UPLOADED+`.

---

## Report

| Field | Notes |
|-------|--------|
| `id` | UUID |
| `recording_id` | 1:1 for MVP |
| `project_id` | denormalized for authz queries |
| `status` | PENDING → GENERATING → READY | FAILED |
| `title` | from AI |
| `summary` | from AI |
| `steps` | ordered reproduction steps (JSONB) |
| `ai_status` | mirrors analysis outcome |
| `prompt_version` | reproducibility |
| `created_at` / `updated_at` | |

**Invariants:**
- Only one report per recording (MVP).
- `READY` requires non-empty summary/steps (or explicit empty-allow flag).
- Replayed `AnalysisCompleted` upserts; does not create duplicates.

---

## ShareLink

| Field | Notes |
|-------|--------|
| `id` | UUID |
| `report_id` | |
| `token` | high-entropy, unique |
| `expires_at` | nullable = no expiry |
| `revoked_at` | nullable |
| `created_by` | user id |
| `created_at` | |

**Invariants:** public access only if not revoked and not expired; exposes limited report fields.

---

## Ownership map

```text
Project
  └── Recording
        ├── MediaArtifact[]   (Media context)
        ├── Analysis          (AI context)
        └── Report
              └── ShareLink[]
```

Commands never reach across aggregates in one DB transaction **except** within the same context’s explicit use case (e.g., Media updating recording status + inserting artifacts in one tx).
