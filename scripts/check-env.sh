#!/usr/bin/env bash
set -euo pipefail
echo "Go: $(go version 2>/dev/null || echo 'missing')"
echo "Docker: $(docker version --format '{{.Server.Version}}' 2>/dev/null || echo 'daemon not reachable')"
echo "Compose file: deploy/compose/docker-compose.yml"
