# BugSathi Web UI (Milestone 16)

Vite + React + TypeScript client for the BugSathi API.

```bash
# from repo root (API must be on :8080)
make run-api
make web-dev
```

Dev server: http://localhost:5173  
Vite proxies `/v1` and `/s` to the API. CORS is also enabled via `CORS_ORIGINS`.

Optional: `VITE_API_URL=http://127.0.0.1:8080` to call the API without the proxy.
