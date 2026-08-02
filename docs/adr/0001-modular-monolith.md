# ADR 0001: Modular Monolith First

## Status

Accepted

## Context

We need clear module boundaries (auth, uploads, media, AI, reports) and a path to independent scaling of CPU-heavy media work vs rate-limited AI work. Starting with many microservices would force distributed failure handling, local orchestration pain, and premature API versioning before the domain is stable.

## Decision

Build a **modular monolith**:

- One primary codebase and deployable API binary.
- Separate **worker** entrypoint (`cmd/worker`) sharing domain packages.
- Module boundaries enforced by package structure + ports (interfaces).
- Async boundaries via Kafka topics (logical services before physical ones).
- gRPC contracts defined early so extraction later is mechanical.

## Consequences

**Positive**

- Fast local iteration; single debugger story.
- Transactional consistency within a request where needed.
- Boundaries taught without ops explosion.

**Negative**

- Discipline required to avoid a “ball of mud.”
- One bad dependency can couple modules if ports are skipped.
- Shared DB means careful migration ownership.

## Alternatives

| Alternative | Rejected because |
|-------------|------------------|
| Microservices day 1 | Ops & distributed tx cost before product clarity |
| Serverless-only | Long media jobs + local Kafka learning fit poorly |
| Sync monolith (no queue) | No ordering/retry curriculum; HTTP timeouts |

## Migration path

1. Scale workers independently (already separate process).
2. Split consumer groups by stage.
3. Extract media/AI into separate repos/deployments behind existing protobuf + Kafka contracts.
