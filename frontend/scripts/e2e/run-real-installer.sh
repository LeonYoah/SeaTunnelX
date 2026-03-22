#!/usr/bin/env bash
# Licensed to the Apache Software Foundation (ASF) under one or more
# contributor license agreements.  See the NOTICE file distributed with
# this work for additional information regarding copyright ownership.
# The ASF licenses this file to You under the Apache License, Version 2.0
# (the "License"); you may not use this file except in compliance with
# the License.  You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FRONTEND_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
ROOT_DIR="$(cd "${FRONTEND_DIR}/.." && pwd)"
BASE_TMP_DIR="${ROOT_DIR}/tmp/e2e"
TMP_DIR=""
MINIO_NAME="${E2E_INSTALLER_REAL_MINIO_NAME:-stx-installer-real-minio}"
KEEP_ARTIFACTS="${E2E_INSTALLER_REAL_KEEP_ARTIFACTS:-0}"
SKIP_LOCAL_CLEANUP="${E2E_INSTALLER_REAL_SKIP_LOCAL_CLEANUP:-}"
BACKEND_TEMPLATE="${ROOT_DIR}/config.e2e.installer-real.yaml"
AGENT_TEMPLATE="${ROOT_DIR}/config.e2e.agent-real.yaml"
MINIO_MC_IMAGE="${E2E_INSTALLER_REAL_MINIO_MC_IMAGE:-minio/mc:RELEASE.2025-03-12T17-29-24Z}"
PLAYWRIGHT_PROJECT="${PLAYWRIGHT_PROJECT:-}"
PLAYWRIGHT_GREP="${PLAYWRIGHT_GREP:-}"
PLAYWRIGHT_SPEC="${E2E_REAL_PLAYWRIGHT_SPEC:-e2e/install-wizard-real.spec.ts}"

if [[ -z "${SKIP_LOCAL_CLEANUP}" ]]; then
  if [[ "${CI:-}" == "true" || "${GITHUB_ACTIONS:-}" == "true" ]]; then
    SKIP_LOCAL_CLEANUP="1"
  else
    SKIP_LOCAL_CLEANUP="0"
  fi
fi

pick_port() {
  local start_port="$1"
  python3 - "$start_port" <<'PY'
import socket
import sys

start = int(sys.argv[1])
for port in range(start, start + 200):
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        try:
            sock.bind(("127.0.0.1", port))
        except OSError:
            continue
        print(port)
        sys.exit(0)

raise SystemExit("no free port found")
PY
}

cleanup() {
  docker rm -f "${MINIO_NAME}" >/dev/null 2>&1 || true
  if [[ -n "${TMP_DIR}" && "${KEEP_ARTIFACTS}" != "1" && "${SKIP_LOCAL_CLEANUP}" != "1" ]]; then
    rm -rf "${TMP_DIR}" || true
  fi
}

trap cleanup EXIT

pkill -f 'node ./scripts/e2e/real-agent-supervisor.mjs' >/dev/null 2>&1 || true

mkdir -p "${BASE_TMP_DIR}"
TMP_DIR="$(mktemp -d "${BASE_TMP_DIR}/installer-real.XXXXXX")"
mkdir -p "${TMP_DIR}/logs" "${TMP_DIR}/storage" "${TMP_DIR}/install" "${TMP_DIR}/minio"
docker rm -f "${MINIO_NAME}" >/dev/null 2>&1 || true

BACKEND_HTTP_PORT="$(pick_port "${E2E_INSTALLER_REAL_BACKEND_PORT:-18000}")"
BACKEND_GRPC_PORT="$(pick_port "${E2E_INSTALLER_REAL_GRPC_PORT:-19090}")"
FRONTEND_PORT="$(pick_port "${E2E_FRONTEND_PORT:-3300}")"
SUPERVISOR_PORT="$(pick_port "${E2E_AGENT_SUPERVISOR_PORT:-18181}")"
CLUSTER_PORT_PRIMARY="$(pick_port "${E2E_INSTALLER_REAL_CLUSTER_PORT_PRIMARY:-38181}")"
CLUSTER_PORT_SECONDARY="$(pick_port "$((CLUSTER_PORT_PRIMARY + 1))")"
HTTP_PORT_PRIMARY="$(pick_port "${E2E_INSTALLER_REAL_HTTP_PORT_PRIMARY:-38080}")"
HTTP_PORT_SECONDARY="$(pick_port "$((HTTP_PORT_PRIMARY + 1))")"

BACKEND_CONFIG_PATH="${TMP_DIR}/config.e2e.installer-real.yaml"
AGENT_CONFIG_PATH="${TMP_DIR}/config.e2e.agent-real.yaml"
TMP_DIR_ESCAPED="$(printf '%s\n' "${TMP_DIR}" | sed 's/[\/&]/\\&/g')"

sed \
  -e "s/:18000/:${BACKEND_HTTP_PORT}/g" \
  -e "s|http://127.0.0.1:18000|http://127.0.0.1:${BACKEND_HTTP_PORT}|g" \
  -e "s/port: 19090/port: ${BACKEND_GRPC_PORT}/g" \
  -e "s|\.\./tmp/e2e/installer-real/seatunnelx\\.db|${TMP_DIR_ESCAPED}/seatunnelx.db|g" \
  -e "s|\.\./tmp/e2e/installer-real/storage|${TMP_DIR_ESCAPED}/storage|g" \
  -e "s|\.\./tmp/e2e/installer-real/logs/seatunnelx\\.log|${TMP_DIR_ESCAPED}/logs/seatunnelx.log|g" \
  "${BACKEND_TEMPLATE}" > "${BACKEND_CONFIG_PATH}"

sed \
  -e "s/127.0.0.1:19090/127.0.0.1:${BACKEND_GRPC_PORT}/g" \
  -e "s|tmp/e2e/installer-real/logs/seatunnelx-agent\\.log|${TMP_DIR_ESCAPED}/logs/seatunnelx-agent.log|g" \
  -e "s|tmp/e2e/installer-real/install/default|${TMP_DIR_ESCAPED}/install/default|g" \
  "${AGENT_TEMPLATE}" > "${AGENT_CONFIG_PATH}"

MINIO_CONTAINER_ID="$(
docker run -d \
  --name "${MINIO_NAME}" \
  -p 127.0.0.1::9000 \
  -p 127.0.0.1::9001 \
  -e MINIO_ROOT_USER=minioadmin \
  -e MINIO_ROOT_PASSWORD=minioadmin \
  -v "${TMP_DIR}/minio:/data" \
  quay.io/minio/minio:RELEASE.2025-02-18T16-25-55Z \
  server /data --console-address :9001
)"

MINIO_API_PORT="$(
  docker port "${MINIO_CONTAINER_ID}" 9000/tcp | awk -F: 'NR==1{print $NF}'
)"
MINIO_CONSOLE_PORT="$(
  docker port "${MINIO_CONTAINER_ID}" 9001/tcp | awk -F: 'NR==1{print $NF}'
)"

for _ in $(seq 1 30); do
  if curl -fsS "http://127.0.0.1:${MINIO_API_PORT}/minio/health/live" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

export E2E_INSTALLER_REAL=1
export E2E_API_MODE=real
export E2E_BACKEND_BASE_URL="${E2E_BACKEND_BASE_URL:-http://127.0.0.1:${BACKEND_HTTP_PORT}}"
export E2E_FRONTEND_HOST="${E2E_FRONTEND_HOST:-127.0.0.1}"
export E2E_FRONTEND_PORT="${E2E_FRONTEND_PORT:-${FRONTEND_PORT}}"
export E2E_FRONTEND_BASE_URL="${E2E_FRONTEND_BASE_URL:-http://${E2E_FRONTEND_HOST}:${E2E_FRONTEND_PORT}}"
export E2E_AGENT_SUPERVISOR_PORT="${E2E_AGENT_SUPERVISOR_PORT:-${SUPERVISOR_PORT}}"
export E2E_INSTALLER_REAL_CONFIG_PATH="${E2E_INSTALLER_REAL_CONFIG_PATH:-${BACKEND_CONFIG_PATH}}"
export E2E_AGENT_REAL_CONFIG_PATH="${E2E_AGENT_REAL_CONFIG_PATH:-${AGENT_CONFIG_PATH}}"
export E2E_INSTALLER_REAL_VERSION="${E2E_INSTALLER_REAL_VERSION:-2.3.13}"
export E2E_INSTALLER_REAL_INSTALL_DIR="${E2E_INSTALLER_REAL_INSTALL_DIR:-${TMP_DIR}/install/seatunnel-${E2E_INSTALLER_REAL_VERSION}}"
export E2E_INSTALLER_REAL_CLUSTER_PORT_PRIMARY="${E2E_INSTALLER_REAL_CLUSTER_PORT_PRIMARY:-${CLUSTER_PORT_PRIMARY}}"
export E2E_INSTALLER_REAL_CLUSTER_PORT_SECONDARY="${E2E_INSTALLER_REAL_CLUSTER_PORT_SECONDARY:-${CLUSTER_PORT_SECONDARY}}"
export E2E_INSTALLER_REAL_HTTP_PORT_PRIMARY="${E2E_INSTALLER_REAL_HTTP_PORT_PRIMARY:-${HTTP_PORT_PRIMARY}}"
export E2E_INSTALLER_REAL_HTTP_PORT_SECONDARY="${E2E_INSTALLER_REAL_HTTP_PORT_SECONDARY:-${HTTP_PORT_SECONDARY}}"
export E2E_INSTALLER_REAL_MINIO_ENDPOINT="${E2E_INSTALLER_REAL_MINIO_ENDPOINT:-http://127.0.0.1:${MINIO_API_PORT}}"
export E2E_INSTALLER_REAL_MINIO_ACCESS_KEY="${E2E_INSTALLER_REAL_MINIO_ACCESS_KEY:-minioadmin}"
export E2E_INSTALLER_REAL_MINIO_SECRET_KEY="${E2E_INSTALLER_REAL_MINIO_SECRET_KEY:-minioadmin}"
export E2E_INSTALLER_REAL_CHECKPOINT_BUCKET="${E2E_INSTALLER_REAL_CHECKPOINT_BUCKET:-s3a://seatunnel-checkpoint}"
export E2E_INSTALLER_REAL_IMAP_BUCKET="${E2E_INSTALLER_REAL_IMAP_BUCKET:-s3a://seatunnel-imap}"
export E2E_INSTALLER_REAL_TMP_DIR="${TMP_DIR}"

CHECKPOINT_BUCKET_NAME="${E2E_INSTALLER_REAL_CHECKPOINT_BUCKET#s3a://}"
IMAP_BUCKET_NAME="${E2E_INSTALLER_REAL_IMAP_BUCKET#s3a://}"

docker run --rm --network host --entrypoint /bin/sh "${MINIO_MC_IMAGE}" -c "\
  mc alias set e2e ${E2E_INSTALLER_REAL_MINIO_ENDPOINT} ${E2E_INSTALLER_REAL_MINIO_ACCESS_KEY} ${E2E_INSTALLER_REAL_MINIO_SECRET_KEY} && \
  mc mb -p e2e/${CHECKPOINT_BUCKET_NAME} && \
  mc mb -p e2e/${IMAP_BUCKET_NAME}" >/dev/null

cd "${FRONTEND_DIR}"
playwright_args=("${PLAYWRIGHT_SPEC}")
if [[ -n "${PLAYWRIGHT_PROJECT}" ]]; then
  playwright_args+=(--project "${PLAYWRIGHT_PROJECT}")
fi
if [[ -n "${PLAYWRIGHT_GREP}" ]]; then
  playwright_args+=(--grep "${PLAYWRIGHT_GREP}")
fi
pnpm exec playwright test "${playwright_args[@]}"
