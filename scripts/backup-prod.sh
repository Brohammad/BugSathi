#!/usr/bin/env bash
# Logical backup: Postgres dump + MinIO bucket mirror.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE=(docker compose -f "$ROOT/deploy/compose/docker-compose.prod.yml" --env-file "$ROOT/.env.prod")
BACKUP_DIR="${1:-$ROOT/backups}"
STAMP="$(date +%F-%H%M%S)"
DEST="$BACKUP_DIR/$STAMP"

if [[ ! -f "$ROOT/.env.prod" ]]; then
  echo "Missing .env.prod" >&2
  exit 1
fi

mkdir -p "$DEST/minio"

# shellcheck disable=SC1091
set -a
# shellcheck source=/dev/null
source "$ROOT/.env.prod"
set +a

PG_USER="${POSTGRES_USER:-bugsathi}"
PG_DB="${POSTGRES_DB:-bugsathi}"
BUCKET="${MINIO_BUCKET:-bugsathi}"

echo "== Backup to $DEST =="

echo "Postgres dump..."
"${COMPOSE[@]}" exec -T postgres pg_dump -U "$PG_USER" "$PG_DB" >"$DEST/postgres.sql"

echo "MinIO mirror (bucket=$BUCKET)..."
"${COMPOSE[@]}" run --rm --no-deps \
  -v "$DEST/minio:/backup" \
  --entrypoint /bin/sh minio-init -c "
    mc alias set local http://minio:9000 $$MINIO_ACCESS_KEY $$MINIO_SECRET_KEY
    mc mirror --overwrite local/$$MINIO_BUCKET /backup
  "

git -C "$ROOT" rev-parse HEAD >"$DEST/git-rev.txt" 2>/dev/null || true

echo "Done: $DEST"
echo "Restore Postgres: ./scripts/restore-prod.sh $DEST/postgres.sql --confirm"
