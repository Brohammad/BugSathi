# ADR 0015: AI Analysis Pipeline

## Status

Accepted

## Context

After frames exist, we need a bug title, summary, and reproduction steps. Providers differ; tests must not call paid APIs.

## Decision

1. `AnalyzerPort.Analyze(ctx, Input) (Result, error)` — hexagonal port.
2. Adapters: **Mock** (default locally/CI) and **OpenAI-compatible** HTTP (`AI_BASE_URL` + `AI_API_KEY` + `AI_MODEL`).
3. Prompt version constant stored with each analysis (`prompt_v1`).
4. Consumer group `bugsathi-ai` on `bugsathi.recording.frames-extracted`.
5. Idempotent on `(recording_id, prompt_version)` — skip LLM if analysis already `completed`.
6. Upsert `reports` to `READY` with title/summary/steps; outbox `AnalysisCompleted` + `ReportGenerated`.
7. Timeouts via context (default 60s); max keyframes sent to provider (default 5).
8. **OpenAI adapter (MVP)** — sends frame object keys and metadata as **text** in the
   chat prompt, not image bytes. True multimodal is deferred; mock remains default
   for CI. Results are validated (`title`, `summary`, ≥1 step) before persist (ADR 0030).
9. Idempotent replay re-emits outbox with the **existing** `report_id`, not a new UUID.

## Consequences

**Positive** — offline-first DX; vendor swap by config.  
**Negative** — mock quality ≠ production LLM; OpenAI path is text-context only until multimodal lands.

## Alternatives

| Alternative | Rejected |
|-------------|----------|
| Hard-code OpenAI SDK | Untestable / lock-in |
| Sync AI in media worker | Couples CPU and rate-limited IO |
