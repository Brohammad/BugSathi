# ADR 0034 — Deferred audit follow-ups (M28)

## Status

Accepted

## Context

ADR 0033 left three explicit follow-ups: durable DLQ attempt counters,
consumers for emit-only `AnalysisStarted`, and multimodal OpenAI (product).

## Decision

1. **Durable Kafka retries** — table `kafka_retry_attempts` keyed by
   `(topic, partition, offset)`; worker media/AI consumers share a
   `PostgresAttemptStore` (memory fallback if Postgres blips).
2. **AnalysisStarted consumer** — group `bugsathi-ai-started` logs + increments
   `bugsathi_analysis_started_total`; does not drive Analyze.
3. **Docs** — DATA-LIFECYCLE reflects recording delete + abandoned upload GC.
4. **Multimodal OpenAI** — still deferred (ADR 0015 / 0030).

> Follow-up: M29 later resolved item 4 with bounded private-frame loading,
> chronological OpenAI image content, and `prompt_v2`.

## Consequences

**Positive** — worker restart no longer soft-resets poison-message attempt
counts; AnalysisStarted is observable end-to-end.  
**Negative** — attempt rows need occasional ops hygiene for abandoned offsets;
multimodal still text-only.

## Alternatives

| Option | Why not |
|--------|---------|
| Redis attempt counters | Extra dependency for a low-churn table Postgres already hosts |
| SSE fan-out on AnalysisStarted | UI already polls/SSE on report; metric/log is enough for M28 |
