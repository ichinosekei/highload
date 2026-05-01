#!/usr/bin/env bash
# deploy.sh — Сборка, упаковка и деплой на удалённый сервер.
#
# Использование:
#   ./scripts/deploy.sh user@host              # Деплой с параметрами по умолчанию
#   ./scripts/deploy.sh user@host /opt/app     # Указание пути на сервере

set -euo pipefail

# --- Configuration ---
REMOTE_HOST="${1:?Usage: $0 user@host [remote_path]}"
REMOTE_PATH="${2:-highload}"
IMAGES=(
  "highload-catalog"
  "highload-order"
  "highload-payment"
  "highload-notification"
)
ARCHIVE_NAME="highload-images.tar.gz"
SSH_OPTS="-o StrictHostKeyChecking=no -o ConnectTimeout=10 -o IdentitiesOnly=yes"

echo "============================================"
echo "  Deploying to: ${REMOTE_HOST}:${REMOTE_PATH}"
echo "============================================"

echo ""
echo ">>> Step 1/5: Building Docker images locally..."
docker compose build --parallel
echo "✅ Images built successfully."

echo ""
echo ">>> Step 2/5: Saving images to archive..."
docker save "${IMAGES[@]}" | gzip > "${ARCHIVE_NAME}"
ARCHIVE_SIZE=$(du -sh "${ARCHIVE_NAME}" | cut -f1)
echo "✅ Archive created: ${ARCHIVE_NAME} (${ARCHIVE_SIZE})"

echo ""
echo ">>> Step 3/5: Transferring files to remote..."

ssh ${SSH_OPTS} "${REMOTE_HOST}" "mkdir -p ${REMOTE_PATH}"

scp ${SSH_OPTS} "${ARCHIVE_NAME}" "${REMOTE_HOST}:${REMOTE_PATH}/"

scp ${SSH_OPTS} \
  docker-compose.yaml \
  docker-compose.scaled.yaml \
  traefik_dynamic.yaml \
  "${REMOTE_HOST}:${REMOTE_PATH}/"

ssh ${SSH_OPTS} "${REMOTE_HOST}" "mkdir -p ${REMOTE_PATH}/deployments/postgres"
scp ${SSH_OPTS} \
  deployments/postgres/init-db.sql \
  "${REMOTE_HOST}:${REMOTE_PATH}/deployments/postgres/"

echo "✅ Files transferred."

echo ""
echo ">>> Step 4/5: Loading images on remote..."
ssh ${SSH_OPTS} "${REMOTE_HOST}" "
  cd ${REMOTE_PATH}
  echo 'Loading images...'
  docker load < ${ARCHIVE_NAME}
  rm -f ${ARCHIVE_NAME}
  echo 'Images loaded.'
"
echo "✅ Images loaded on remote."

echo ""
echo ">>> Step 5/5: Deploying containers..."
ssh ${SSH_OPTS} "${REMOTE_HOST}" "
  cd ${REMOTE_PATH}
  docker compose down --remove-orphans 2>/dev/null || true
  docker compose up -d
  echo ''
  echo 'Waiting for services to start...'
  sleep 10
  echo ''
  echo '--- Container Status ---'
  docker compose ps
"

# --- Cleanup local archive ---
rm -f "${ARCHIVE_NAME}"

echo ""
echo "============================================"
echo "  ✅ Deployment complete!"
echo "  Remote: ${REMOTE_HOST}:${REMOTE_PATH}"
echo ""
echo "  Verify with:"
echo "    ssh ${REMOTE_HOST} 'cd ${REMOTE_PATH} && docker compose ps'"
echo ""
echo "  Health checks:"
echo "    curl http://<VM_IP>:8080/health  # Catalog"
echo "    curl http://<VM_IP>:8082/health  # Order"
echo "    curl http://<VM_IP>:8083/health  # Payment"
echo "    curl http://<VM_IP>:8081/health  # Notification"
echo "============================================"
