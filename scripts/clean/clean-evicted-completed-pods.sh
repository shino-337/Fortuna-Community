#!/usr/bin/env bash
# Clean Evicted, Completed, Failed pods in the cluster.
#
# Usage:
#   ./scripts/clean/clean-evicted-completed-pods.sh
#   NAMESPACE=fortuna ./scripts/clean/clean-evicted-completed-pods.sh
#   DRY_RUN=1 ./scripts/clean/clean-evicted-completed-pods.sh
#
# By default this checks all namespaces. Set NAMESPACE to limit scope.

set -euo pipefail

NAMESPACE="${NAMESPACE:-}"
DRY_RUN="${DRY_RUN:-0}"

if ! command -v kubectl >/dev/null 2>&1; then
  echo "kubectl required"
  exit 1
fi
if ! command -v jq >/dev/null 2>&1; then
  echo "jq required"
  exit 1
fi

NS_ARG=(-A)
if [ -n "$NAMESPACE" ]; then
  NS_ARG=(-n "$NAMESPACE")
fi

echo "[clean] Listing Evicted/Completed/Failed pods..."
LIST=$(kubectl get pods "${NS_ARG[@]}" -o json 2>/dev/null | jq -r '
  .items[] |
  select(
    .status.phase == "Failed" or
    .status.phase == "Succeeded" or
    (.status.reason != null and (.status.reason | test("Evicted|OOMKilled|Error"; "i")))
  ) |
  "\(.metadata.namespace) \(.metadata.name)"
' 2>/dev/null || true)

COUNT=0
if [ -n "$LIST" ]; then
  COUNT=$(echo "$LIST" | wc -l | tr -d ' ')
  echo "[clean] Found $COUNT pod(s)."
  echo "$LIST" | while IFS=' ' read -r ns name; do
    if [ "$DRY_RUN" = "1" ]; then
      echo "DRY_RUN: would delete $ns/$name"
    else
      echo "Deleting $ns/$name ..."
      kubectl delete pod -n "$ns" "$name" --grace-period=0 --force --ignore-not-found 2>/dev/null || true
    fi
  done
  echo "[clean] Done."
else
  echo "[clean] No Evicted/Completed/Failed pods found."
fi
