# State Machines

Illegal transitions must be rejected in domain code (not only in UI).

---

## Recording

```text
                 ┌─────────────────────────────────────────┐
                 │                                         │
                 ▼                                         │
        ┌──────────────┐                                   │
        │  UPLOADING   │  (presign issued / session open)   │
        └──────┬───────┘                                   │
               │ complete upload OK                        │
               ▼                                           │
        ┌──────────────┐                                   │
        │   UPLOADED   │───────────────────────────────────┤
        └──────┬───────┘     (retry from FAILED if         │
               │             RecordingUploaded re-emitted) │
               │ media worker starts                       │
               ▼                                           │
        ┌──────────────┐                                   │
        │  PROCESSING  │──▶ FAILED ◀───────────────────────┘
        └──────┬───────┘       ▲
               │ frames OK     │ terminal / exhausted retries
               ▼               │
        ┌──────────────┐       │
        │    READY     │       │  (media done; report may still generate)
        └──────────────┘       │
                               │
        FAILED ────────────────┘
```

| From | To | Trigger |
|------|----|---------|
| (new) | `UPLOADING` | Create upload session |
| `UPLOADING` | `UPLOADED` | Complete upload + object exists |
| `UPLOADED` | `PROCESSING` | Media worker claims job |
| `PROCESSING` | `READY` | Frames extracted successfully (claim holder only) |
| `UPLOADING`/`UPLOADED`/`PROCESSING` | `FAILED` | Irrecoverable error after retries |
| `FAILED` | `PROCESSING` | Media worker claims after reprocess / retry |
| `PROCESSING` | `PROCESSING` | Another worker reclaims an expired lease |
| `FAILED` / `READY` / … | (outbox only) | Owner `POST .../reprocess` re-emits `RecordingUploaded` (ADR 0023) |

`READY` on Recording means **media artifacts available**. Report has its own machine.

`PROCESSING` carries a claim (`processing_owner`, `processing_expires_at`). Only
the current owner may move the recording to `READY` or `FAILED`; a delivery that
finds a live claim owned by someone else does nothing. An expired claim means
the holder died and the recording is free to be picked up again (ADR 0025).

---

## Report

```text
        ┌──────────┐
        │ PENDING  │  (recording exists; waiting for AI)
        └────┬─────┘
             │ AnalysisStarted / worker picks up
             ▼
        ┌────────────┐
        │ GENERATING │──▶ FAILED
        └────┬───────┘
             │ AnalysisCompleted + assemble
             ▼
        ┌──────────┐
        │  READY   │
        └──────────┘
```

| From | To | Trigger |
|------|----|---------|
| (new) | `PENDING` | Recording uploaded (or FramesExtracted) |
| `PENDING` | `GENERATING` | AI worker starts |
| `GENERATING` | `READY` | Analysis persisted + report assembled |
| `GENERATING` | `FAILED` | Exhausted AI retries |
| `FAILED` | `GENERATING` | Reprocess via owner recording reprocess (re-emits upstream events) |

---

## Enforcement

```text
func (r *Recording) Transition(to Status) error {
    if !allowed[r.Status][to] {
        return ErrIllegalTransition
    }
    r.Status = to
    return nil
}
```

Workers load aggregate → transition → save in one transaction with outbox/event insert.
