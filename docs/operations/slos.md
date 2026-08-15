# Service Level Objectives

Targets for a single-region, single-replica deployment. Tighten when running HA or multi-tenant production.

## API availability

| SLI | Target | Window |
|-----|--------|--------|
| Successful readiness (`GET /readyz` = 200) | **99.5%** | 30 days |
| Successful liveness (`GET /healthz` = 200) | **99.9%** | 30 days |

**Measurement:** synthetic probe every 30s against `/readyz` (Compose healthcheck, K8s readiness, or external uptime check).

**Error budget:** ~3.6 h downtime / 30 days at 99.5%.

## API latency (authenticated read paths)

| SLI | Target |
|-----|--------|
| `GET /v1/projects/{id}/reports/{id}` p95 | **< 500 ms** |
| All other authenticated routes p95 | **< 1 s** |

**Measurement:** Prometheus histogram `bugsathi_http_request_duration_seconds` filtered by `status_class="2xx"`.

## Pipeline throughput

| SLI | Target |
|-----|--------|
| Upload → report READY (mock AI) | **< 5 min** p95 |
| Upload → report READY (real AI) | **< 15 min** p95 |
| Pipeline job success rate | **≥ 99%** rolling 24 h |

**Measurement:**

- Duration: time from `bugsathi_pipeline_jobs_total{stage="media",result="ok"}` to `stage="ai",result="ok"` for the same recording (trace or log correlation).
- Success: `sum(rate(bugsathi_pipeline_jobs_total{result="ok"}[24h])) / sum(rate(bugsathi_pipeline_jobs_total[24h]))`.

## Outbox relay

| SLI | Target |
|-----|--------|
| Pending outbox rows (`bugsathi_outbox_pending`) | **< 100** steady state |
| Time to publish after insert | **< 30 s** p95 |

## Abuse protection

| SLI | Target |
|-----|--------|
| Rate-limit rejections (`bugsathi_rate_limit_rejected_total`) | Alert if **> 100/min** sustained (possible attack or misconfigured client) |

## Alert starting points

Use these as Grafana/Prometheus rule seeds; tune thresholds after baseline traffic.

```promql
# Readiness failing
up{job="bugsathi-api"} == 0

# Elevated 5xx
sum(rate(bugsathi_http_requests_total{status_class="5xx"}[5m])) /
sum(rate(bugsathi_http_requests_total[5m])) > 0.01

# Outbox backlog
bugsathi_outbox_pending > 500

# Pipeline failures
sum(rate(bugsathi_pipeline_jobs_total{result="error"}[15m])) > 0
```
