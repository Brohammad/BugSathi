# Runbook: Elevated API errors or latency

## Symptoms

- Spike in `bugsathi_http_requests_total{status_class="5xx"}`
- Grafana/API p95 latency above SLO (see [slos.md](../slos.md))
- Users report timeouts or 429 responses

## Triage

1. **Scope:** one route vs all routes — check `bugsathi_http_request_duration_seconds` and request totals by handler.
2. **Dependencies:** `/readyz` checks Postgres only; confirm DB is up.
3. **Rate limits:** check `bugsathi_rate_limit_rejected_total`. Legitimate bursts may need higher `RATE_LIMIT_RPS` / `RATE_LIMIT_BURST`; auth brute-force shows up on `/v1/auth/*` (stricter `AUTH_RATE_LIMIT_*`).
4. **Body limits:** oversized uploads return 413 from `MaxBytesReader` — verify `MAX_BODY_BYTES`.
5. **Resource saturation:** enable `ENABLE_PPROF=true` temporarily and inspect `/debug/pprof/` (never expose publicly without auth).

## Common causes

| Cause | Signal | Mitigation |
|-------|--------|------------|
| Postgres slow/down | `/readyz` 503, pool timeouts | [postgres-down.md](postgres-down.md) |
| Rate limit mis-tune | 429 + `rate_limit_rejected` metric | Raise limits or fix client retry storm |
| Report cache cold | High latency on report GET only | Normal after restart; check `REPORT_CACHE_TTL` |
| OOM / CPU | Container restarts, high `go_memstats` | Scale resources, reduce `POSTGRES_MAX_CONNS` |

## Escalation data to capture

- Correlation / request ID from failing response headers or logs
- Approximate time range and affected routes
- Recent deploy or config change
- Prometheus snapshot of error rate and latency histograms
