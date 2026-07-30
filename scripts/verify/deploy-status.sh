#!/usr/bin/env bash
# One-screen snapshot of Fortuna workloads + recent warning events (no webhook; local kubectl only).
# Usage:
#   ./scripts/verify/deploy-status.sh [namespace]
set -euo pipefail
NS="${1:-${NAMESPACE:-fortuna}}"
echo "=== Namespace: $NS ==="
kubectl get ns "$NS" &>/dev/null || { echo "Namespace $NS not found." >&2; exit 1; }
echo ""
echo "=== Pods ==="
kubectl get pods -n "$NS" -o wide 2>/dev/null || true
echo ""
echo "=== Deployments / DaemonSets ==="
kubectl get deploy,ds -n "$NS" 2>/dev/null || true
echo ""
echo "=== Recent warning events (last 25) ==="
if kubectl get events -n "$NS" --field-selector type=Warning --sort-by='.lastTimestamp' &>/dev/null; then
  kubectl get events -n "$NS" --field-selector type=Warning --sort-by='.lastTimestamp' 2>/dev/null | tail -25
else
  kubectl get events -n "$NS" --sort-by='.lastTimestamp' 2>/dev/null | tail -25
fi
