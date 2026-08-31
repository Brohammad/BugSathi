#!/usr/bin/env bash
# Generate strong secrets for .env.prod (stdout or --write).
set -euo pipefail

write=0
if [[ "${1:-}" == "--write" ]]; then
  write=1
  shift
fi

rand() {
  openssl rand -base64 48 | tr -d '/+=' | head -c "$1"
}

JWT=$(rand 48)
PG=$(rand 32)
MINIO=$(rand 32)
GRAFANA=$(rand 24)

if [[ "$write" -eq 1 ]]; then
  ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
  OUT="$ROOT/.env.prod"
  if [[ -f "$OUT" ]]; then
    echo "Refusing to overwrite existing $OUT (delete or edit manually)" >&2
    exit 1
  fi
  cp "$ROOT/.env.prod.example" "$OUT"
  if [[ "$(uname -s)" == "Darwin" ]]; then
    sed -i '' "s|^JWT_SECRET=.*|JWT_SECRET=${JWT}|" "$OUT"
    sed -i '' "s|^POSTGRES_PASSWORD=.*|POSTGRES_PASSWORD=${PG}|" "$OUT"
    sed -i '' "s|^MINIO_SECRET_KEY=.*|MINIO_SECRET_KEY=${MINIO}|" "$OUT"
    sed -i '' "s|^GRAFANA_ADMIN_PASSWORD=.*|GRAFANA_ADMIN_PASSWORD=${GRAFANA}|" "$OUT"
  else
    sed -i "s|^JWT_SECRET=.*|JWT_SECRET=${JWT}|" "$OUT"
    sed -i "s|^POSTGRES_PASSWORD=.*|POSTGRES_PASSWORD=${PG}|" "$OUT"
    sed -i "s|^MINIO_SECRET_KEY=.*|MINIO_SECRET_KEY=${MINIO}|" "$OUT"
    sed -i "s|^GRAFANA_ADMIN_PASSWORD=.*|GRAFANA_ADMIN_PASSWORD=${GRAFANA}|" "$OUT"
  fi
  chmod 600 "$OUT" 2>/dev/null || true
  echo "Wrote $OUT with generated secrets. Edit CADDY_SITE / ACME_EMAIL for VPS TLS."
else
  cat <<EOF
JWT_SECRET=${JWT}
POSTGRES_PASSWORD=${PG}
MINIO_SECRET_KEY=${MINIO}
GRAFANA_ADMIN_PASSWORD=${GRAFANA}
EOF
fi
