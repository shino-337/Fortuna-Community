#!/usr/bin/env bash
# Clean Evicted, Completed, Failed pods in the cluster (all namespaces).
# Run periodically or after deploy to avoid accumulation of old deployment pods.

set -euo pipefail

echo "[clean] Listing Evicted/Completed/Failed pods..."
LIST=$(kubectl get pods -A -o json 2>/dev/null | jq -r '
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
  echo "[clean] Deleting $COUNT pod(s)..."
  echo "$LIST" | while IFS=' ' read -r ns name; do
    kubectl delete pod -n "$ns" "$name" --grace-period=0 --force --ignore-not-found 2>/dev/null && true
  done
  echo "[clean] Done."
else
  echo "[clean] No Evicted/Completed/Failed pods found."
fi
exit 0
