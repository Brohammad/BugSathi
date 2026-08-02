# Milestone 3 — Authentication

## Problem

Everything project-scoped needs a **principal** (who is calling). Without auth, uploads and reports cannot be owned or authorized.

## Decision (summary)

See [ADR-0011](../adr/0011-authentication.md): argon2id passwords, short JWT access, rotating opaque refresh tokens.

## Endpoints

| Method | Path | Auth |
|--------|------|------|
| POST | `/v1/auth/register` | public |
| POST | `/v1/auth/login` | public |
| POST | `/v1/auth/refresh` | refresh body |
| POST | `/v1/auth/logout` | refresh body |
| GET | `/v1/auth/me` | Bearer access |

## Failure modes

- Duplicate email → 409
- Bad credentials → 401 (same message for unknown user — no user enumeration)
- Expired/revoked refresh → 401
- Invalid access JWT → 401

## Out of scope

OAuth/SSO, email verification, password reset (later hardening).
