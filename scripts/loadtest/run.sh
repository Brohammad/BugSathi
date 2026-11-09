#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ENVFILE="$ROOT/scripts/loadtest/.env.loadtest"

if ! command -v k6 >/dev/null 2>&1; then
  echo "k6 not found. Install: brew install k6" >&2
  exit 1
fi

"$ROOT/scripts/loadtest/bootstrap-env.sh"

set -a
# shellcheck disable=SC1090
source "$ENVFILE"
set +a

export VUS="${VUS:-5}"
export DURATION="${DURATION:-30s}"

echo "Running k6: VUS=$VUS DURATION=$DURATION API_BASE=$API_BASE"
exec k6 run "$ROOT/scripts/loadtest/k6-read.js"
