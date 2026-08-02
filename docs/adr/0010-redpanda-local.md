# ADR 0010: Redpanda for Local Kafka API

## Status

Accepted

## Context

ADR-0002 selected Apache Kafka for ordering pedagogy. Full Kafka (KRaft) images are heavier on developer laptops. Redpanda speaks the Kafka protocol and preserves partition keys, consumer groups, and replay semantics we care about teaching.

## Decision

- **Local Compose:** Redpanda exposing Kafka API on `localhost:19092`.
- **Client libraries:** standard Kafka clients (segmentio/kafka-go, franz-go, etc.) against Redpanda.
- **Production:** either managed Kafka (MSK, Confluent) or Redpanda Cloud — same client code.
- Topic naming and key=`recording_id` rules from EVENT-FLOW.md unchanged.

## Consequences

**Positive**

- Faster `make up`; same mental model as Kafka.
- Interview answer remains valid: “Kafka-compatible log; partition key ordering.”

**Negative**

- Slight behavioral differences vs Apache Kafka in edge admin APIs (rare for our use).

## Alternatives

| Alternative | Notes |
|-------------|-------|
| Bitnami Kafka KRaft | Heavier; still fine if preferred |
| Redis Streams | Rejected for curriculum depth |
