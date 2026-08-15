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
5. **Retry backoff:** repeated handler failures sleep with exponential backoff; sustained errors look like a stuck consumer but offsets are not committed.

## Metrics

```promql
bugsathi_outbox_pending
sum by (stage) (rate(bugsathi_pipeline_jobs_total{result="error"}[15m]))
histogram_quantile(0.95, sum by (le, stage) (rate(bugsathi_pipeline_duration_seconds_bucket[15m])))
```

## Recovery steps

1. Fix root cause (storage, AI key, corrupt upload).
2. For poison messages: identify `recording_id` in logs, fix data or skip via admin SQL (last resort — document change).
3. After fix, consumer retries automatically; no offset commit means message will be reprocessed.
4. Confirm outbox gauge returns toward zero and report reaches READY.

## Prevention

- Keep `AI_MAX_FRAMES` reasonable to bound AI latency/cost.
- Monitor pipeline error rate alert from [slos.md](../slos.md).
