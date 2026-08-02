# ADR 0006: Internal Communication & Worker Persistence

## Status

Accepted (supersedes earlier “workers write via API gRPC” sketch)

## Context

Workers must update recording/report state after media and AI work. Routing every write through the HTTP API (even via gRPC) makes the API a bottleneck, couples worker availability to API deploy, and blurs bounded-context ownership.

We still want typed contracts for future service extraction and for any *behavior* that truly spans contexts.

## Decision

1. **Workers write Postgres directly** through their context’s repositories (same DB in the modular monolith).
2. **API owns external HTTP** only (plus its own context writes: Auth, Projects, Uploads, Sharing).
3. **gRPC + Protobuf** live under `api/proto/v1/` for:
   - Health / readiness where useful
   - Future cross-service **behavior** (not “please UPDATE this row for me”)
   - Optional query APIs when contexts split into separate deployables
4. Cross-context reactions inside the monolith prefer **domain events (Kafka/outbox)** over synchronous gRPC.

## Consequences

**Positive**

- Clear write ownership; no API hop on the hot pipeline path.
- Matches how extracted workers would work in production (own DB access or own DB).
- gRPC remains a real skill without misuse as a persistence proxy.

**Negative**

- Shared DB means migration discipline across contexts.
- Must not let workers violate another context’s invariants (use repositories, not raw SQL across boundaries).

## Alternatives

| Alternative | Rejected because |
|-------------|------------------|
| Workers → gRPC API → DB | API becomes write gateway / bottleneck |
| Workers → raw SQL anywhere | Breaks encapsulation |
| Only in-process function calls forever | Weakens extraction / contract practice |

## Protobuf layout

```text
api/proto/v1/
  health/v1/health.proto
  # add domain protos when a real RPC behavior exists
```
