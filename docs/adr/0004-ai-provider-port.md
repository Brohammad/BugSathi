# ADR 0004: AI Provider Port / Adapter

## Status

Accepted

## Context

LLM vendors differ in APIs, pricing, multimodal support, and rate limits. Hard-coding one SDK couples domain logic to a vendor and makes CI non-deterministic/expensive.

## Decision

Define an application **port**:

```text
AnalyzerPort.Analyze(ctx, AnalysisInput) (BugAnalysis, error)
```

Adapters:

1. **OpenAI-compatible HTTP adapter** (works with OpenAI, many proxies, some local servers).
2. **Mock adapter** — deterministic fixtures for unit/integration tests and offline demos.
3. Future: Anthropic, Ollama, etc. without changing callers.

Cross-cutting rules:

- Explicit timeouts and max tokens.
- Prompt **version** stored with results (reproducibility).
- Never log raw API keys; redact PII in prompts/logs where possible.
- Keyframe selection happens **before** the port (control cost).

## Consequences

**Positive**

- Testable without network.
- Vendor swap is config + adapter.
- Interview-ready Hexagonal Architecture example.

**Negative**

- Lowest-common-denominator features across providers.
- Multimodal quirks may need adapter-specific options behind careful interfaces.

## Alternatives

| Alternative | Rejected because |
|-------------|------------------|
| Call OpenAI directly in worker | Untestable; vendor lock-in |
| LangChain-style mega-framework | Opaque for learning first principles |
| Only local LLM | Good later; weak default DX for MVP quality |
