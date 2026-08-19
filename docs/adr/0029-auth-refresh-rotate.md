# ADR 0029: Atomic Refresh Token Rotation

## Status

Accepted

## Context

Refresh rotation used separate lookup, revoke, and create steps. Two concurrent
`/v1/auth/refresh` calls with the same token could both pass validation; the
loser's revoke returned `ErrNotFound`, which surfaced as HTTP 404/500 instead of
a clean unauthorized response (audit H5).

## Decision

1. Add `RefreshTokenRepository.Rotate` — one transaction that:
   - `UPDATE ... SET revoked_at` where `token_hash` matches, `revoked_at IS NULL`,
     and `expires_at > now`, returning the consumed row
   - inserts the replacement token
2. `Service.Refresh` calls `Rotate` only; reuse/race/expired → `ErrUnauthorized` → HTTP **401**.
3. `Logout` treats post-lookup revoke conflicts as idempotent success.

## Consequences

**Positive** — no double-rotation window; replay after redeem is consistently 401.  
**Negative** — refresh path holds a DB transaction slightly longer (negligible).

## Alternatives

| Alternative | Rejected |
|-------------|----------|
| Optimistic revoke + map `ErrNotFound` in handler | Still a race between lookup and revoke |
| Keep separate revoke/create | Loser gets ambiguous errors |
| Redis lock per token | Extra infra for single-replica MVP |
