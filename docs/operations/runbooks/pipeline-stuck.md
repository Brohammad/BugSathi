# Runbook: Pipeline stuck / outbox lag

## Symptoms

- Recording stays in non-READY state long after upload complete
- `bugsathi_outbox_pending` gauge rising or flat above threshold
- `bugsathi_pipeline_jobs_total{result="error"}` increasing for `media` or `ai` stage

## Triage

1. **Outbox relay running?** Worker process publishes outbox rows; confirm worker is healthy (`/readyz` on `:8081`).
2. **Kafka reachable?** Check Redpanda health and worker logs for consumer errors.
3. **Media stage:** ffmpeg failures — inspect worker logs for `recording_id`; verify object exists in MinIO.
4. **AI stage:** provider errors — check `AI_PROVIDER`, `AI_API_KEY`, and `bugsathi_ai_analyze_duration_seconds{result="error"}`.
5. **Retry / DLQ:** handlers retry with backoff up to `KAFKA_RETRY_MAX_ATTEMPTS`, then publish to `*.dlq` and commit (see [dlq-reprocess.md](dlq-reprocess.md)).
6. **Recording stuck in `PROCESSING`:** a worker holds (or held) the claim. Check who and until when:

```sql
SELECT id, status, processing_owner, processing_expires_at
FROM recordings
WHERE status = 'PROCESSING'
ORDER BY updated_at;
```

An expired `processing_expires_at` means the holder died; the next delivery reclaims it automatically, so trigger a reprocess rather than editing the row. A lease that keeps moving forward means ffmpeg is genuinely still running (ADR 0025).

## Metrics

```promql
bugsathi_outbox_pending
sum by (stage) (rate(bugsathi_pipeline_jobs_total{result="error"}[15m]))
histogram_quantile(0.95, sum by (le, stage) (rate(bugsathi_pipeline_duration_seconds_bucket[15m])))
bugsathi_dlq_published_total
sum by (stage, reason) (rate(bugsathi_claim_skipped_total[15m]))
```

A steady rate of `bugsathi_claim_skipped_total{reason="held"}` is normal duplicate delivery being deduplicated. A rising `reason="lost"` rate means jobs are outliving their lease — raise `MEDIA_CLAIM_LEASE` or check for stalled workers.

## Recovery steps

1. Fix root cause (storage, AI key, corrupt upload).
2. For poison messages: inspect `*.dlq`, then owner `POST .../recordings/{id}/reprocess` ([dlq-reprocess.md](dlq-reprocess.md)).
3. Confirm outbox gauge returns toward zero and report reaches READY.

## Prevention

- Keep `AI_MAX_FRAMES` reasonable to bound AI latency/cost.
- Monitor pipeline error rate alert from [slos.md](../slos.md).
