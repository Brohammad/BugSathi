#!/usr/bin/env bash
# First-time VPS bootstrap for BugSathi (Ubuntu/Debian). Re-run safe for git pull + redeploy.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP_DOMAIN="${APP_DOMAIN:-}"
S3_DOMAIN="${S3_DOMAIN:-}"
ACME_EMAIL="${ACME_EMAIL:-}"
REPO_URL="${REPO_URL:-https://github.com/Brohammad/BugSathi.git}"
BRANCH="${BRANCH:-cursor/m29-multimodal-ai}"
INSTALL_DIR="${INSTALL_DIR:-$HOME/BugSathi}"

usage() {
  cat <<EOF
Usage: APP_DOMAIN=app.example.com S3_DOMAIN=s3.example.com ACME_EMAIL=you@example.com $0

Optional env:
  REPO_URL     (default: $REPO_URL)
  BRANCH       (default: $BRANCH)
  INSTALL_DIR  (default: $INSTALL_DIR)

Requires: Docker Engine + Compose plugin, git, curl, openssl.
Run as a user in the docker group (or root).
EOF
}

if [[ -z "$APP_DOMAIN" || -z "$S3_DOMAIN" || -z "$ACME_EMAIL" ]]; then
  usage >&2
  exit 1
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "Docker not found. Install Docker Engine + Compose plugin first." >&2
  exit 1
fi

if command -v ufw >/dev/null 2>&1 && [[ "$(id -u)" -eq 0 ]]; then
  ufw default deny incoming || true
  ufw default allow outgoing || true
  ufw allow OpenSSH || true
  ufw allow 80/tcp || true
  ufw allow 443/tcp || true
  ufw --force enable || true
fi

if [[ -d "$INSTALL_DIR/.git" ]]; then
  git -C "$INSTALL_DIR" fetch origin "$BRANCH"
  git -C "$INSTALL_DIR" checkout "$BRANCH"
  git -C "$INSTALL_DIR" pull --ff-only origin "$BRANCH" || true
else
  git clone --branch "$BRANCH" --depth 1 "$REPO_URL" "$INSTALL_DIR"
fi

cd "$INSTALL_DIR"

if [[ ! -f .env.prod ]]; then
  ./scripts/gen-prod-secrets.sh --write
fi

APP_ORIGIN="https://${APP_DOMAIN}"
update_env() {
  local key=$1 val=$2 file=.env.prod
  if grep -q "^${key}=" "$file"; then
    if [[ "$(uname -s)" == "Darwin" ]]; then
      sed -i '' "s|^${key}=.*|${key}=${val}|" "$file"
    else
      sed -i "s|^${key}=.*|${key}=${val}|" "$file"
    fi
  else
    echo "${key}=${val}" >>"$file"
  fi
}

update_env CADDY_SITE "$APP_DOMAIN"
update_env CADDY_S3_SITE "$S3_DOMAIN"
update_env ACME_EMAIL "$ACME_EMAIL"
update_env MINIO_PUBLIC_ENDPOINT "$S3_DOMAIN"
update_env MINIO_PUBLIC_USE_SSL "true"
update_env MINIO_CORS_ORIGINS "$APP_ORIGIN"
update_env CORS_ORIGINS ""

make check-prod-compose
make up-prod

echo "Waiting for TLS + readiness..."
for i in $(seq 1 90); do
  if curl -fsS "https://${APP_DOMAIN}/readyz" >/dev/null 2>&1; then
    echo "Ready: https://${APP_DOMAIN}/readyz"
    break
  fi
  sleep 2
done

BASE="https://${APP_DOMAIN}" ./scripts/smoke-prod.sh

echo ""
echo "Deploy complete."
echo "  App:  https://${APP_DOMAIN}"
echo "  S3:   https://${S3_DOMAIN}"
echo "  Backup: ./scripts/backup-prod.sh"
echo "  E2E:    BASE=https://${APP_DOMAIN} ./scripts/e2e-prod.sh  (needs ffmpeg on host)"
