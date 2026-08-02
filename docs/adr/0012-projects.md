# ADR 0012: Projects & Membership

## Status

Accepted

## Context

Uploads and reports are scoped to a tenancy unit. Auth gives us *who*; Projects answers *which workspace* and *are they allowed*.

## Decision

- Aggregate: **Project** with members (`owner` | `member`).
- Creator becomes **owner** in the same transaction as project insert.
- Roles:
  - `owner` — full control (update/delete, manage members)
  - `member` — read + create recordings (later milestones)
- HTTP (all require Bearer access):
  - `POST /v1/projects` — create
  - `GET /v1/projects` — list mine
  - `GET /v1/projects/{id}` — get if member
  - `PATCH /v1/projects/{id}` — owner only
  - `DELETE /v1/projects/{id}` — owner only
  - `POST /v1/projects/{id}/members` — owner adds member by user id (email lookup later)
  - `GET /v1/projects/{id}/members` — list members if member
- Authorization via `ProjectRepository.GetMembership(userID, projectID)`.

## Consequences

**Positive** — clear tenancy for M5+ uploads.  
**Negative** — no invites/email yet; add member requires knowing user UUID.

## Alternatives

| Alternative | Rejected |
|-------------|----------|
| Flat user-owned folders only | No collaboration path |
| Org → Workspace → Project hierarchy | Premature for MVP |
