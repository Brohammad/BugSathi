#!/usr/bin/env bash
# Restore Postgres from a logical dump. MinIO restore is manual (mc mirror).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE=(docker compose -f "$ROOT/deploy/compose/docker-compose.prod.yml" --env-file "$ROOT/.env.prod")

if [[ $# -lt 2 || "$2" != "--confirm" ]]; then
  echo "Usage: $0 <postgres.sql> --confirm" >&2
  echo "WARNING: replaces the current Postgres database." >&2
  exit 1
fi

DUMP=$1
if [[ ! -f "$DUMP" ]]; then
  echo "Dump not found: $DUMP" >&2
  exit 1
fi

# shellcheck disable=SC1091
set -a
# shellcheck source=/dev/null
source "$ROOT/.env.prod"
set +a
PG_USER="${POSTGRES_USER:-bugsathi}"
PG_DB="${POSTGRES_DB:-bugsathi}"

echo "== Restore Postgres from $DUMP =="
echo "Stopping api and worker..."
"${COMPOSE[@]}" stop api worker

echo "Restoring database..."
"${COMPOSE[@]}" exec -T postgres psql -U "$PG_USER" -d postgres -c \
  "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '$PG_DB' AND pid <> pg_backend_pid();" \
  >/dev/null 2>&1 || true
"${COMPOSE[@]}" exec -T postgres dropdb -U "$PG_USER" --if-exists "$PG_DB"
"${COMPOSE[@]}" exec -T postgres createdb -U "$PG_USER" "$PG_DB"
cat "$DUMP" | "${COMPOSE[@]}" exec -T postgres psql -U "$PG_USER" -d "$PG_DB" -v ON_ERROR_STOP=1

echo "Starting api and worker..."
"${COMPOSE[@]}" start api worker

echo "Restore complete. Verify: make health-prod"
