#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE_NAME="${IMAGE_NAME:-fortuna-dashboard:latest}"

if command -v nerdctl >/dev/null 2>&1; then
  echo "[build-dashboard] Using nerdctl (namespace k8s.io)"
  nerdctl --namespace k8s.io build -f "${ROOT_DIR}/dashboard/Dockerfile" -t "${IMAGE_NAME}" "${ROOT_DIR}"
elif command -v docker >/dev/null 2>&1; then
  echo "[build-dashboard] Using docker"
  docker build -f "${ROOT_DIR}/dashboard/Dockerfile" -t "${IMAGE_NAME}" "${ROOT_DIR}"
else
  echo "ERROR: nerdctl or docker is required to build the dashboard image." >&2
  exit 1
fi

echo "[build-dashboard] ✅ Image built: ${IMAGE_NAME}"
