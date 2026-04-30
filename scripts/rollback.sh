#!/usr/bin/env bash
# rollback.sh — Откат к предыдущему состоянию деплоя.
#
# Использование:
#   ./scripts/rollback.sh user@host
#   ./scripts/rollback.sh user@host /opt/app

set -euo pipefail

REMOTE_HOST="${1:?Usage: $0 user@host [remote_path]}"
REMOTE_PATH="${2:-/opt/highload}"
SSH_OPTS="-o StrictHostKeyChecking=no -o ConnectTimeout=10"

echo ">>> Rolling back deployment on ${REMOTE_HOST}..."

# shellcheck disable=SC2086
ssh ${SSH_OPTS} "${REMOTE_HOST}" "
  cd ${REMOTE_PATH}
  docker compose down --remove-orphans
  docker compose up -d
  sleep 10
  docker compose ps
"

echo "✅ Rollback complete."
