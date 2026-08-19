# ADR 0027: Trusted Proxy Rate Limits & Production Secret Guards

## Status

Accepted

## Context

Rate limiting keyed on the first `X-Forwarded-For` hop allowed any direct client
to pick an arbitrary bucket (audit H4). Separately, `APP_ENV=production` did
not reject known dev defaults for JWT and storage credentials, so a mis-copy of
`.env` could ship unsafe secrets.

## Decision

1. **`TRUSTED_PROXIES`** — comma-separated IPs/CIDRs. Forwarding headers affect
   the rate-limit client key only when `RemoteAddr` matches a trusted entry.
   Empty (default) means never trust XFF; use `RemoteAddr` only.
2. When trusted, prefer `X-Real-IP`, then leftmost `X-Forwarded-For`.
3. **`config.Load()` production guards** when `APP_ENV=production`:
   - Reject dev JWT secret, default Postgres password, default MinIO secret key
   - Reject `ENABLE_PPROF=true`

## Consequences

**Positive** — spoof-resistant rate limits behind a reverse proxy; fail-fast on
unsafe prod config.  
**Negative** — operators must set `TRUSTED_PROXIES` correctly behind nginx/ALB;
empty default is safe but ignores XFF until configured.

## Alternatives

| Alternative | Rejected |
|-------------|----------|
| Always trust first XFF | Spoofable on direct exposure |
| Always ignore XFF | Breaks limits behind one proxy without extra config |
| Warn-only on dev secrets | Process still starts with known credentials |
