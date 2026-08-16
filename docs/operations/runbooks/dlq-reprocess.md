# Runbook: Dead-letter queue / poison messages

## Symptoms

- Recording stays `FAILED` after retries; pipeline error metrics spike then stop
- `bugsathi_dlq_published_total` increments
- Worker logs: `message dead-lettered` with topic/partition/offset

## What happened

After `KAFKA_RETRY_MAX_ATTEMPTS` (default 5) consecutive handler failures for the same offset, the worker:

1. Publishes an envelope to `{source}.dlq` (e.g. `bugsathi.recording.uploaded.dlq`)
2. Commits the source offset so the partition advances

Invalid JSON payloads are dead-lettered immediately (attempt=1).

## Inspect

```bash
# Redpanda / rpk (prod compose network)
docker compose -f deploy/compose/docker-compose.prod.yml --env-file .env.prod \
  exec redpanda rpk topic consume bugsathi.recording.uploaded.dlq -n 5
```

Envelope fields: `source_topic`, `source_partition`, `source_offset`, `key`, `attempts`, `error`, `payload`.

## Recover

1. Fix root cause (corrupt object, AI key, ffmpeg, etc.).
2. Owner reprocess:
   ```bash
   curl -s -X POST "localhost:8080/v1/projects/$PID/recordings/$RID/reprocess" \
     -H "Authorization: Bearer $ACCESS"
   ```
3. Outbox relay re-publishes `RecordingUploaded`; media/AI run again.
4. Confirm recording / report leave `FAILED`.

## Notes

- Reprocess is **owner-only**.
- Allowed statuses: `FAILED`, `UPLOADED`, `PROCESSING`, `READY`.
- Attempt counters are in-process; worker restart may allow a few more retries before DLQ.
