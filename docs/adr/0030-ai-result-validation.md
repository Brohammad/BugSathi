# ADR 0030: AI Result Validation & Report ID on Replay

## Status

Accepted

## Context

The OpenAI adapter sends frame **storage keys as text** in the prompt, not image
bytes — the “multimodal” path is metadata-only today (audit). LLM JSON could also
arrive with empty title/summary/steps and still be persisted. On idempotent
replay of a completed analysis, `emitCompletedEvents` minted a new `report_id` for
Kafka while Postgres kept the existing row (`ON CONFLICT` does not update `id`).

## Decision

1. **`ValidateAnalysisResult`** — require non-empty title and summary and at least
   one non-blank step after trim; reject before `CompleteAnalysis`.
2. **`NormalizeAnalysisResult`** — trim strings and drop empty steps before marshal.
3. **`GetReportByRecording`** — replay path loads the persisted report and emits
   outbox events with that `report_id`.
4. **Docs** — ADR 0015 updated: OpenAI adapter is text-context only until a future
   multimodal adapter downloads frame bytes.

## Consequences

**Positive** — consistent event IDs; no empty reports from malformed LLM JSON.  
**Negative** — stricter provider output; invalid JSON still fails at parse time.

## Alternatives

| Alternative | Rejected |
|-------------|----------|
| Trust LLM shape | Empty reports in production |
| Return new report_id on replay | Downstream consumers see wrong ID |
| Block replay emits entirely | Breaks at-least-once outbox recovery |
