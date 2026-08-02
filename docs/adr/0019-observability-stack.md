# ADR 0019: Observability Stack (Metrics + Traces)

## Status

Accepted

## Context

Correlation IDs land in logs from M2 (ADR 0008). Milestone 11 needs RED metrics, distributed traces, and dashboards for pipeline lag, failure rate, and AI latency.

## Decision

1. **Metrics — Prometheus** exposition at `GET /metrics` on API and worker.
   - HTTP RED: `bugsathi_http_requests_total`, `bugsathi_http_request_duration_seconds`
   - Pipeline: `bugsathi_pipeline_jobs_total{stage,result}`, `bugsathi_pipeline_duration_seconds{stage}`
   - AI: `bugsathi_ai_analyze_duration_seconds{provider,result}`
   - Lag: `bugsathi_outbox_pending` (gauge scraped from Postgres)
2. **Traces — OpenTelemetry → OTLP/HTTP** when `OTEL_EXPORTER_OTLP_ENDPOINT` is set; otherwise noop.
   - HTTP server spans; worker spans around media/AI handlers.
   - Span attributes include `correlation_id` / `recording_id` when present.
3. **Local stack (Compose):** Prometheus, Grafana (provisioned dashboard), OpenTelemetry Collector, Jaeger (trace UI).
4. **Logs** remain structured `slog` with existing correlation fields (no log shipper required in M11).

## Consequences

**Positive** — industry-standard scrape + OTLP; works offline; dashboards as code.  
**Negative** — dual signal systems (Prom + OTel traces); multi-replica lag metrics need aggregation later.
