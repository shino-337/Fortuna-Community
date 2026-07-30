#!/bin/bash
# Port-forward Core + Dashboard for local testing
# Usage: ./scripts/utils/port-forward-dashboard.sh [dashboard-local-port] [core-local-port] [bind-address]

set -euo pipefail

NAMESPACE="${NAMESPACE:-fortuna}"
DASH_LOCAL_PORT="${1:-8081}"
CORE_LOCAL_PORT="${2:-8080}"
PF_ADDRESS="${3:-${PORT_FORWARD_ADDRESS:-0.0.0.0}}"

echo "=========================================="
echo "Core + Dashboard Port-Forward"
echo "=========================================="
echo ""

# Resolve service names (prefer labels, fallback to fixed names)
CORE_SVC=$(kubectl get svc -n "$NAMESPACE" -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -z "$CORE_SVC" ]; then
  CORE_SVC=$(kubectl get svc -n "$NAMESPACE" fortuna-core -o jsonpath='{.metadata.name}' 2>/dev/null || echo "")
fi
if [ -z "$CORE_SVC" ]; then
  echo "❌ Core service not found in namespace $NAMESPACE"
  exit 1
fi

DASH_SVC=$(kubectl get svc -n "$NAMESPACE" -l app.kubernetes.io/component=dashboard -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -z "$DASH_SVC" ]; then
  DASH_SVC=$(kubectl get svc -n "$NAMESPACE" -l app=fortuna-dashboard -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
fi
if [ -z "$DASH_SVC" ]; then
  DASH_SVC=$(kubectl get svc -n "$NAMESPACE" fortuna-dashboard -o jsonpath='{.metadata.name}' 2>/dev/null || echo "")
fi
if [ -z "$DASH_SVC" ]; then
  echo "❌ Dashboard service not found in namespace $NAMESPACE"
  exit 1
fi

CORE_SVC_PORT=$(kubectl get svc -n "$NAMESPACE" "$CORE_SVC" -o jsonpath='{.spec.ports[?(@.port==8080)].port}' 2>/dev/null || echo "")
if [ -z "$CORE_SVC_PORT" ]; then
  CORE_SVC_PORT=$(kubectl get svc -n "$NAMESPACE" "$CORE_SVC" -o jsonpath='{.spec.ports[0].port}' 2>/dev/null || echo "8080")
fi
DASH_SVC_PORT=$(kubectl get svc -n "$NAMESPACE" "$DASH_SVC" -o jsonpath='{.spec.ports[0].port}' 2>/dev/null || echo "80")

cleanup() {
  [ -n "${CORE_PF_PID:-}" ] && kill "$CORE_PF_PID" 2>/dev/null || true
  [ -n "${DASH_PF_PID:-}" ] && kill "$DASH_PF_PID" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

echo "Core Service: $CORE_SVC (svc port: $CORE_SVC_PORT -> $PF_ADDRESS:$CORE_LOCAL_PORT)"
echo "Dashboard Service: $DASH_SVC (svc port: $DASH_SVC_PORT -> $PF_ADDRESS:$DASH_LOCAL_PORT)"
echo ""
echo "Starting port-forward..."
kubectl port-forward --address "$PF_ADDRESS" -n "$NAMESPACE" svc/"$CORE_SVC" "$CORE_LOCAL_PORT:$CORE_SVC_PORT" >/tmp/core-pf.log 2>&1 &
CORE_PF_PID=$!
kubectl port-forward --address "$PF_ADDRESS" -n "$NAMESPACE" svc/"$DASH_SVC" "$DASH_LOCAL_PORT:$DASH_SVC_PORT" >/tmp/dashboard-pf.log 2>&1 &
DASH_PF_PID=$!
sleep 2

if ! kill -0 "$CORE_PF_PID" 2>/dev/null; then
  echo "❌ Failed to start Core port-forward. See /tmp/core-pf.log"
  exit 1
fi
if ! kill -0 "$DASH_PF_PID" 2>/dev/null; then
  echo "❌ Failed to start Dashboard port-forward. See /tmp/dashboard-pf.log"
  exit 1
fi

echo "✅ Core API: http://$PF_ADDRESS:$CORE_LOCAL_PORT"
echo "✅ Dashboard: http://$PF_ADDRESS:$DASH_LOCAL_PORT"
echo ""
echo "Logs:"
echo "  - /tmp/core-pf.log"
echo "  - /tmp/dashboard-pf.log"
echo ""
echo "Press Ctrl+C to stop both port-forwards"
echo ""

wait "$CORE_PF_PID" "$DASH_PF_PID"
