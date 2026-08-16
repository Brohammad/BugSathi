#!/usr/bin/env bash
# Chaos drill: stop Postgres in prod Compose, verify /readyz fails, recover.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE=(docker compose -f "$ROOT/deploy/compose/docker-compose.prod.yml" --env-file "$ROOT/.env.prod")
API_URL="${API_URL:-http://127.0.0.1:8080}"
WORKER_URL="${WORKER_URL:-http://127.0.0.1:8081}"
TIMEOUT="${DRILL_TIMEOUT:-120}"

if [[ ! -f "$ROOT/.env.prod" ]]; then
  echo "Missing .env.prod — copy .env.prod.example and run: make up-prod" >&2
  exit 1
fi

ready_code() {
  local url=$1
  curl -fsS -o /dev/null -w '%{http_code}' "$url/readyz" 2>/dev/null || echo "000"
}

wait_for() {
  local url=$1 want=$2 label=$3
  local deadline=$((SECONDS + TIMEOUT))
  while (( SECONDS < deadline )); do
    local code
    code=$(ready_code "$url")
    if [[ "$code" == "$want" ]]; then
      echo "OK: $label /readyz = $code (expected $want)"
      return 0
    fi
    sleep 2
  done
  echo "FAIL: $label /readyz did not reach $want within ${TIMEOUT}s (last=$(ready_code "$url"))" >&2
  return 1
}

echo "== BugSathi chaos drill (Postgres stop/start) =="
echo "Compose: ${COMPOSE[*]}"

code=$(ready_code "$API_URL")
if [[ "$code" != "200" ]]; then
  echo "API not ready ($code). Start stack: make up-prod" >&2
  exit 1
fi
echo "Baseline: API /readyz = $code"

echo "Stopping postgres..."
"${COMPOSE[@]}" stop postgres

wait_for "$API_URL" "503" "API"
wait_for "$WORKER_URL" "503" "Worker"

echo "Starting postgres..."
"${COMPOSE[@]}" start postgres

echo "Waiting for postgres healthcheck..."
deadline=$((SECONDS + TIMEOUT))
while (( SECONDS < deadline )); do
  if "${COMPOSE[@]}" ps postgres 2>/dev/null | grep -qE 'healthy'; then
    echo "OK: postgres healthy"
    break
  fi
  sleep 2
done
if (( SECONDS >= deadline )); then
  echo "FAIL: postgres did not become healthy within ${TIMEOUT}s" >&2
  exit 1
fi

wait_for "$API_URL" "200" "API"
wait_for "$WORKER_URL" "200" "Worker"

echo "== Drill passed: readiness failed during outage and recovered =="
