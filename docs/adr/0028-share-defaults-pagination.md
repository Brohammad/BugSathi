# ADR 0028: Share Defaults, Token Hash-at-Rest & List Pagination

## Status

Accepted

## Context

Share links could be created with no expiry (indefinite exposure) and tokens were
stored plaintext in Postgres. List endpoints returned unbounded arrays (reports,
comments, shares, projects, members), risking memory and response-size blowups
as tenants grow.

## Decision

1. **Share TTL** — `SHARE_DEFAULT_TTL` (default 720h) applies when the client
   omits `expires_in_seconds`. `SHARE_MAX_TTL` (default 2160h) caps explicit
   requests. Non-positive expiry is rejected (no never-expire via API).
2. **Token hash-at-rest** — when `SHARE_HASH_TOKENS=true` (default), store
   SHA-256 hex of the raw token; return raw token only on create. List omits
   token. `APP_ENV=production` requires hash-at-rest.
3. **Migration `0009`** — delete legacy plaintext share rows; add keyset indexes.
4. **Pagination** — list routes accept `limit` and `cursor` query params. Responses
   keep the existing collection key (`reports`, `comments`, etc.) and add optional
   `next_cursor`. Keyset paging on `(created_at, id)` (ASC for comments/members,
   DESC for reports/shares/projects). Defaults via `LIST_DEFAULT_LIMIT` (50) and
   max `LIST_MAX_LIMIT` (100).

## Consequences

**Positive** — bounded responses, safer token storage, sane share lifetimes.  
**Negative** — existing plaintext share links are invalidated by migration;
clients must follow `next_cursor` for large lists.

## Alternatives

| Alternative | Rejected |
|-------------|----------|
| Offset pagination | Degrades on large tables; unstable under concurrent inserts |
| Keep plaintext tokens | DB leak exposes live share URLs |
| Never-expire default | Indefinite public exposure if link leaks |
