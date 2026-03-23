#!/usr/bin/env bash
# ============================================================================
# Xóa các Pod có trạng thái Evicted (dọn namespace, không giải phóng đĩa trên node)
# ============================================================================
# Chạy nơi có kubectl + quyền cluster. Chỉ xóa pod có .status.reason == Evicted.
#
# Usage:
#   ./scripts/utils/cleanup-evicted-pods.sh              # all namespaces
#   NAMESPACE=fortuna ./scripts/utils/cleanup-evicted-pods.sh
#   DRY_RUN=1 ./scripts/utils/cleanup-evicted-pods.sh
# ============================================================================
set -euo pipefail

NAMESPACE="${NAMESPACE:-}"
DRY_RUN="${DRY_RUN:-0}"

if ! command -v kubectl >/dev/null 2>&1; then
  echo "kubectl required"
  exit 1
fi
if ! command -v jq >/dev/null 2>&1; then
  echo "jq required (apt install jq / brew install jq)"
  exit 1
fi

NS_ARG=(--all-namespaces)
[ -n "$NAMESPACE" ] && NS_ARG=(-n "$NAMESPACE")

mapfile -t LINES < <(kubectl get pods "${NS_ARG[@]}" -o json 2>/dev/null | jq -r '
  .items[]
  | select(.status.reason == "Evicted")
  | "\(.metadata.namespace)\t\(.metadata.name)"')

if [ "${#LINES[@]}" -eq 0 ] || [ -z "${LINES[0]:-}" ]; then
  echo "No Evicted pods found."
  exit 0
fi

echo "Found ${#LINES[@]} Evicted pod(s)."
for line in "${LINES[@]}"; do
  ns="${line%%$'\t'*}"
  name="${line#*$'\t'}"
  if [ "$DRY_RUN" = "1" ]; then
    echo "DRY_RUN: would delete $ns/$name"
  else
    echo "Deleting $ns/$name ..."
    kubectl delete pod -n "$ns" "$name" --ignore-not-found=true 2>/dev/null || true
  fi
done
echo "Done."
