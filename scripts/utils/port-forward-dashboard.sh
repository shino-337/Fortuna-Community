#!/bin/bash
# Port-forward Dashboard for UI testing
# Usage: ./scripts/utils/port-forward-dashboard.sh [local-port]
set -e
NAMESPACE="${NAMESPACE:-fortuna}"
LOCAL_PORT="${1:-8081}"

echo "=========================================="
echo "Dashboard Port-Forward"
echo "=========================================="
echo ""

# Check if dashboard service exists
DASH_SVC=$(kubectl get svc -n "$NAMESPACE" -l app=fortuna-dashboard -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -z "$DASH_SVC" ]; then
  DASH_SVC=$(kubectl get svc -n "$NAMESPACE" fortuna-dashboard -o jsonpath='{.metadata.name}' 2>/dev/null || echo "")
fi

if [ -z "$DASH_SVC" ]; then
  echo "❌ Dashboard service not found in namespace $NAMESPACE"
  exit 1
fi

DASH_POD=$(kubectl get pods -n "$NAMESPACE" -l app=fortuna-dashboard -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
SVC_PORT=$(kubectl get svc -n "$NAMESPACE" "$DASH_SVC" -o jsonpath='{.spec.ports[0].port}' 2>/dev/null || echo "80")

echo "Dashboard Service: $DASH_SVC"
echo "Dashboard Pod: $DASH_POD"
echo "Service Port: $SVC_PORT"
echo "Local Port: $LOCAL_PORT"
echo ""
echo "Starting port-forward..."
echo "Access Dashboard at: http://localhost:$LOCAL_PORT"
echo ""
echo "Press Ctrl+C to stop"
echo ""

kubectl port-forward -n "$NAMESPACE" svc/"$DASH_SVC" "$LOCAL_PORT:$SVC_PORT"
