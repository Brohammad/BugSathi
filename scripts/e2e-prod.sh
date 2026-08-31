#!/usr/bin/env bash
# E2E against production Compose (API via Caddy, worker checks via compose exec).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE=(docker compose -f "$ROOT/deploy/compose/docker-compose.prod.yml" --env-file "$ROOT/.env.prod")

if [[ ! -f "$ROOT/.env.prod" ]]; then
  echo "Missing .env.prod — run: ./scripts/gen-prod-secrets.sh --write && make up-prod" >&2
  exit 2
fi

echo "== prod E2E preflight =="
"${COMPOSE[@]}" exec -T worker curl -fsS http://127.0.0.1:8081/healthz >/dev/null
"${COMPOSE[@]}" exec -T worker curl -fsS http://127.0.0.1:8081/readyz >/dev/null
echo "Worker health OK (internal)"

export API="${API:-http://127.0.0.1}"
export PUBLIC_MODE=1
export SKIP_WORKER_DIRECT=1

exec "$ROOT/scripts/e2e-hardcore.sh"
