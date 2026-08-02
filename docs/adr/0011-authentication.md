# ADR 0011: Authentication — Argon2id + JWT Access + Rotating Refresh

## Status

Accepted

## Context

Milestone 3 needs identity for project-scoped features. Choices: sessions-in-DB vs JWT, bcrypt vs argon2id, cookie vs Bearer.

## Decision

| Concern | Choice |
|---------|--------|
| Password hash | **argon2id** (memory-hard; OWASP recommended) |
| Access credential | **JWT** (HS256), short TTL (~15m), `sub` = user id |
| Refresh credential | **Opaque token**, stored **hashed** (SHA-256) in `refresh_tokens`, TTL ~7d |
| Refresh strategy | **Rotation**: redeem refresh → revoke old → issue new pair |
| Transport | `Authorization: Bearer <access>` for APIs; JSON body for refresh/logout |
| Public routes | `/v1/auth/register`, `/login`, `/refresh` |
| Protected | `/v1/auth/me`, `/logout` (logout needs valid refresh; me needs access) |

## Consequences

**Positive**

- Stateless access checks on API (no DB hit per request).
- Stolen refresh can be revoked; rotation limits replay window.
- Interview-standard pattern.

**Negative**

- Access JWT cannot be revoked instantly without a deny-list (acceptable for short TTL).
- Must protect `JWT_SECRET` in env.

## Alternatives

| Alternative | Rejected because |
|-------------|------------------|
| bcrypt only | Fine, but argon2id is stronger default today |
| Session cookies only | Harder for future mobile/extension clients |
| Long-lived JWT only | No rotation/revocation story |
