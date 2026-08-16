# ADR 0025: Media Processing Claim (Expiring Lease)

## Status

Accepted

## Context

`HandleUploaded` treated `PROCESSING` as a resumable state: any delivery that
found a recording in `PROCESSING` re-ran ffmpeg. Status alone cannot tell
"a worker is busy with this right now" apart from "a worker died halfway", so
both cases were handled by redoing the work.

That is wasteful whenever a second delivery overlaps the first — consumer group
rebalance, an owner-triggered reprocess (ADR 0023) arriving while the first job
is still running, or a redelivery after the M17 retry fix. Two ffmpeg runs for
one recording burn CPU, write the same MinIO keys twice, and can produce two
`FramesExtracted` events for one recording.

## Decision

A worker takes an **expiring lease** on the recording before doing any work.

1. `recordings` gains `processing_owner` and `processing_expires_at`, kept in
   sync by a CHECK constraint (both set or both null).
2. `ClaimProcessing` is a single conditional `UPDATE`. It succeeds from
   `UPLOADED` / `FAILED`, or from `PROCESSING` when the lease is unowned,
   already ours, or expired. Zero rows updated means a live foreign lease:
   the service returns `ErrClaimHeld` and the consumer commits the offset
   instead of re-running ffmpeg.
3. The holder renews the lease on a ticker (`MEDIA_CLAIM_RENEW`) so long ffmpeg
   runs do not lose it. If a renewal reports the claim is gone, the job context
   is canceled and the work is abandoned.
4. `FinalizeReady` and `MarkFailed` are owner-gated and clear the claim in the
   same statement, so a job that lost its lease cannot publish
   `FramesExtracted` or flip the status underneath the new owner.
5. A crashed worker leaves a lease that expires after `MEDIA_CLAIM_LEASE`
   (default 2m); the next delivery reclaims it. No operator action, no
   janitor process.

Claim skips are counted by `bugsathi_claim_skipped_total{stage,reason}`.

## Consequences

**Positive**

- One ffmpeg run per recording under concurrent delivery.
- Stuck `PROCESSING` recovers on its own instead of needing a manual reset.
- Deduplication is decided by Postgres, not by consumer-side coordination, so
  it holds across worker replicas.

**Negative**

- Two more columns and a heartbeat goroutine per in-flight job.
- A worker killed mid-job blocks that recording for up to one lease period.
  Lease length trades recovery latency against duplicate-work risk.
- The lease is advisory across process boundaries: correctness rests on the
  owner-gated writes, not on the lease alone.

## Alternatives

| Alternative | Rejected |
|-------------|----------|
| Keep re-running ffmpeg on `PROCESSING` | The bug being fixed: duplicate work and duplicate events |
| Refuse to process anything in `PROCESSING` | Crashed workers would strand recordings forever |
| Postgres advisory lock for the job duration | Ties the claim to one connection/session; invisible in queries and to ops |
| `SELECT … FOR UPDATE` held across ffmpeg | Holds a DB transaction open for minutes |
| Idempotency keyed only on artifact upsert | Deduplicates writes but not the expensive ffmpeg run |
