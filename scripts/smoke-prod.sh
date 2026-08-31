#!/usr/bin/env bash
# Fast smoke tests for production Compose behind Caddy.
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE=(docker compose -f "$ROOT/deploy/compose/docker-compose.prod.yml" --env-file "$ROOT/.env.prod")
BASE="${BASE:-http://127.0.0.1}"
S3="${S3:-http://127.0.0.1:9000}"
PASS=0
FAIL=0

ok() { PASS=$((PASS + 1)); echo "PASS  $*"; }
bad() { FAIL=$((FAIL + 1)); echo "FAIL  $*" >&2; }

code() {
  curl -sS -o /dev/null -w '%{http_code}' "$@" 2>/dev/null || echo "000"
}

echo "== BugSathi prod smoke (BASE=$BASE) =="

if [[ ! -f "$ROOT/.env.prod" ]]; then
  echo "Missing .env.prod — run: ./scripts/gen-prod-secrets.sh --write" >&2
  exit 2
fi

c=$(code "$BASE/healthz")
[[ "$c" == "200" ]] && ok "GET /healthz = 200" || bad "GET /healthz = $c"

c=$(code "$BASE/readyz")
[[ "$c" == "200" ]] && ok "GET /readyz = 200" || bad "GET /readyz = $c"

c=$(code "$BASE/metrics")
[[ "$c" == "404" ]] && ok "GET /metrics blocked = 404" || bad "GET /metrics = $c (expected 404)"

UI=$(curl -sS "$BASE/" 2>/dev/null || true)
echo "$UI" | grep -q '<div id="root">' && ok "SPA shell served" || bad "SPA shell missing"

c=$(code "$BASE/v1/auth/login" -X POST -H 'Content-Type: application/json' -d '{"email":"x","password":"y"}')
[[ "$c" == "400" || "$c" == "401" || "$c" == "422" ]] && ok "POST /v1/auth/login reachable ($c)" || bad "POST /v1/auth/login = $c"

if command -v docker >/dev/null 2>&1; then
  wc=$("${COMPOSE[@]}" exec -T worker curl -fsS -o /dev/null -w '%{http_code}' http://127.0.0.1:8081/readyz 2>/dev/null || echo "000")
  [[ "$wc" == "200" ]] && ok "worker /readyz (internal) = 200" || bad "worker /readyz = $wc"

  prom=$("${COMPOSE[@]}" exec -T api curl -fsS http://127.0.0.1:8080/metrics 2>/dev/null || true)
  echo "$prom" | grep -q 'bugsathi_http_requests_total' && ok "api /metrics (internal scrape)" || bad "api internal metrics missing"
else
  echo "SKIP  docker exec checks (docker unavailable)"
fi

c=$(code "$S3/minio/health/live")
[[ "$c" == "200" ]] && ok "MinIO via Caddy S3 bind = 200" || bad "MinIO health = $c (S3=$S3)"

for port in 5432 6379 8080 8081; do
  if (echo >/dev/tcp/127.0.0.1/"$port") >/dev/null 2>&1; then
    bad "host port ${port} unexpectedly open"
  else
    ok "host port ${port} closed"
  fi
done

echo "== smoke PASS=$PASS FAIL=$FAIL =="
[[ "$FAIL" -eq 0 ]]
