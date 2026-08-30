# Vision Analysis Evaluation

This evaluation prevents BugSathi's AI report quality from being judged only by
whether the provider returned valid JSON. A report must be grounded in facts
visible in the selected recording frames.

## Release gate

- Evaluate 10 fixed recordings with stable expected observations.
- A case passes when it scores at least 8/10.
- The release passes when at least 8 of 10 cases pass.
- Any invented user action, error message, or UI state is a case failure,
  regardless of the numeric score.
- Record model, prompt version, selected frame count, latency, and approximate
  input/output cost for every run.

## Per-case rubric

| Criterion | Points | Rule |
|---|---:|---|
| Primary visible defect | 0–3 | Correctly identifies the main broken UI state |
| Exact visible evidence | 0–2 | Quotes or accurately describes text, control, value, or layout |
| Reproduction sequence | 0–2 | Steps are supported by frame order and metadata |
| Environment use | 0–1 | Uses supplied browser/OS metadata without invention |
| Concision and actionability | 0–2 | Report is specific enough for an engineer to start triage |

Hallucination penalty: subtract 3 points for each unsupported material claim.

## Fixed evaluation cases

1. Registration form showing an exact validation error.
2. Submit button disabled after a repeated action.
3. Mobile-width layout with overlapping controls.
4. Loading spinner that remains visible after content should appear.
5. Navigation ending on a visible 404 page.
6. Checkout field with an inline formatting error.
7. Dark-theme screen with unreadable low-contrast text.
8. Counter or status badge displaying a stale value.
9. Upload progress UI visibly stuck at a fixed percentage.
10. Data table clipping a long value or action control.

Each recording fixture must have a companion note containing:

- expected primary defect;
- exact visible text or value;
- minimum supported reproduction steps;
- claims the model must not infer;
- frame indexes that contain the decisive evidence.

## Baseline

Before the multimodal change, the OpenAI adapter sends frame object-key strings
and metadata but no image bytes. Therefore:

- visual frames delivered to the model: **0**;
- visually grounded cases measurable: **0/10**;
- quality score: **not measured** (a code-path capability failure, not an
  empirical model evaluation).

The first post-change baseline is recorded only after all 10 fixtures exist and
the provider request test proves that selected image bytes are included.

## Run record

For each evaluation run, commit a dated result containing:

| Case | Pass | Score / 10 | Grounded facts | Unsupported claims | Latency | Cost |
|---|---|---:|---|---|---:|---:|

Do not tune the prompt against only one case. Review failures as a group, make
one prompt or input-selection change, then rerun the complete set.
