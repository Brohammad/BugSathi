# ADR 0006: gRPC for Internal Service-to-Service Communication

## Status

Accepted

## Context

Workers need to report progress, write analysis results, and potentially call shared application services. Browser clients need REST/JSON. Using JSON everywhere internally loses typed contracts and makes future service extraction messier.

## Decision

- **External (browser):** HTTP/JSON.
- **Internal (worker ↔ API / future services):** **gRPC + Protobuf** under `api/proto/`.
- Define protos early even while both sides share a monolith process (worker may call in-process implementations **or** gRPC — prefer gRPC client against local server to practice the real path).

Initial services (sketch):

```text
RecordingService.UpdateStatus
ReportService.UpsertAnalysis
Health.Check
```

## Consequences

**Positive**

- Strong contracts; codegen; interview-relevant.
- Extraction to real services later is mostly deployment.

**Negative**

- Protobuf tooling overhead in M2.
- Browser won’t speak gRPC (we don’t need it to).

## Alternatives

| Alternative | Rejected because |
|-------------|------------------|
| Internal HTTP/JSON only | Weaker contracts; less learning |
| Workers write DB directly only | Faster short-term; bypasses application invariants |
| Connect/gRPC-Web to browser | Unnecessary for MVP |
