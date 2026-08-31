#!/usr/bin/env bash
# Guardrail: production Compose must not publish infra or app ports on 0.0.0.0.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FILE="$ROOT/deploy/compose/docker-compose.prod.yml"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

grep -q 'Caddyfile' "$FILE" || fail "Caddyfile mount missing"

# Host-published ports that would leak infra if they appear without 127.0.0.1.
if grep -E '[[:space:]]+-[[:space:]]+["'\'']?(5432|6379|8080|8081|9001|19092):' "$FILE"; then
  fail "infra/app host port published (must stay on the Compose network)"
fi

if grep -E '[[:space:]]+-[[:space:]]+["'\'']?9090:9090' "$FILE"; then
  fail "Prometheus must bind 127.0.0.1, not 0.0.0.0"
fi

if grep -E '[[:space:]]+-[[:space:]]+["'\'']?3000:3000' "$FILE"; then
  fail "Grafana must bind 127.0.0.1, not 0.0.0.0"
fi

grep -q '127.0.0.1:\${CADDY_S3_BIND' "$FILE" || grep -q 'CADDY_S3_BIND:-127.0.0.1:9000' "$FILE" || fail "S3 Caddy bind must default to 127.0.0.1"
grep -q '127.0.0.1:\${PROMETHEUS_HOST_PORT' "$FILE" || fail "Prometheus localhost bind missing"
grep -q '127.0.0.1:\${GRAFANA_HOST_PORT' "$FILE" || fail "Grafana localhost bind missing"
grep -q 'handle /metrics' "$ROOT/deploy/caddy/Caddyfile" || fail "Caddy must block /metrics"

if command -v docker >/dev/null 2>&1; then
  STAGED_ENV=0
  if [[ ! -f "$ROOT/.env.prod" ]]; then
    # Compose env_file: ../../.env.prod is a required path; stage the example for render only.
    cp "$ROOT/.env.prod.example" "$ROOT/.env.prod"
    STAGED_ENV=1
  fi
  cleanup() {
    if [[ "$STAGED_ENV" -eq 1 ]]; then
      rm -f "$ROOT/.env.prod"
    fi
  }
  trap cleanup EXIT
  docker compose -f "$FILE" --env-file "$ROOT/.env.prod" config >/tmp/bugsathi-prod-compose.yml
  echo "OK: docker compose config validated"
else
  echo "WARN: docker not on PATH; skipped compose config render"
fi

echo "OK: production Compose public-surface checks passed"
