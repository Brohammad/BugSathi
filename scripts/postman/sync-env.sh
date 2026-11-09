#!/usr/bin/env bash
# Populate Postman environment secrets from the live API (same as k6 bootstrap).
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ENV_YAML="$ROOT/postman/environments/BugSathi Local.environment.yaml"

"$ROOT/scripts/loadtest/bootstrap-env.sh"

set -a
# shellcheck disable=SC1090
source "$ROOT/scripts/loadtest/.env.loadtest"
set +a

postman environment var set access_token "$ACCESS_TOKEN" --environment "$ENV_YAML"
postman environment var set project_id "$PROJECT_ID" --environment "$ENV_YAML"
