# ADR 0024 — Web UI (React + Vite)

## Context

Backend milestones M1–M15 expose a complete HTTP API for auth, projects, upload, reports, sharing, and comments. Operators still exercise the product via curl. Architecture diagrams always deferred a browser client (“Web UI — React/TS later”).

## Decision

Ship a **Vite + React + TypeScript** SPA under `web/` that talks to the existing API:

- Auth: register / login / refresh / logout (Bearer access + opaque refresh in `localStorage`)
- Projects list + create
- Screen capture (`getDisplayMedia` + `MediaRecorder`) or WebM file upload → complete → poll until READY report
- Report detail: summary, steps, frames, comments, share-link create, owner reprocess
- Public share page at `/s/:token` (no auth)

API adds **CORS** via `CORS_ORIGINS` (default Vite `http://localhost:5173` and `127.0.0.1:5173`). Vite also proxies `/v1` and `/s` in dev so same-origin works without CORS when desired.

## Consequences

**Positive** — product demo path without curl; validates real browser upload/CORS; keeps UI out of the Go binary.

**Negative** — tokens in `localStorage` are XSS-sensitive (acceptable for local/learning MVP; HttpOnly cookies later); collab SSE not wired in UI yet; no production static hosting in Compose yet.

## Alternatives

| Option | Why not now |
|--------|-------------|
| Server-rendered Go templates | Weaker SPA UX for recorder/polling; less transferable frontend skill |
| Next.js full stack | Extra framework weight; API already exists |
| Embed UI in API image | Couples release cycles; defer to a later deploy milestone |
