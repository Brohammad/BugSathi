# ADR 0018: Realtime Collaboration (SSE)

## Status

Accepted

## Context

Project members need to discuss a bug report together and see who else is viewing it, without polling.

## Decision

1. **Transport: Server-Sent Events (SSE)** for server → client fanout (`text/event-stream`).
   - One-way push fits comment + presence updates.
   - Simpler than WebSockets for auth proxies and Go `net/http`.
2. **Persistent comments** in `report_comments` (project-scoped, author FK).
3. **Ephemeral presence** tracked in an in-process hub keyed by `report_id` (SSE connect = join, disconnect = leave).
4. **Auth required** on all collab routes; `EnsureMember` before subscribe or write.
5. **Fanout:** in-memory hub for single-API-instance MVP. Multi-instance later uses Redis pub/sub (or Kafka) behind the same `Hub` port.
6. HTTP:
   - `POST/GET .../reports/{reportID}/comments`
   - `GET .../reports/{reportID}/events` (SSE: `comment.created`, `presence.updated`, `heartbeat`)

## Consequences

**Positive** — low complexity; works locally with one API process; clear upgrade path.  
**Negative** — presence/comments fanout does not cross API replicas until a distributed hub is added.
