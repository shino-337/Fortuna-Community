#!/bin/bash
# Start Dashboard Port-Forward

set -e

PORT=${1:-3000}
NAMESPACE="ksam"
SERVICE="ksam-dashboard"

echo "=========================================="
echo "Starting Dashboard Port-Forward"
echo "=========================================="
echo ""
echo "Service: $SERVICE"
echo "Namespace: $NAMESPACE"
echo "Port: $PORT"
echo ""

# Kill existing port-forwards
echo "Cleaning up existing port-forwards..."
pkill -f "kubectl port-forward.*$SERVICE" 2>/dev/null || true
sleep 2

# Check if service exists
if ! kubectl get svc -n $NAMESPACE $SERVICE >/dev/null 2>&1; then
    echo "❌ Error: Service $SERVICE not found in namespace $NAMESPACE"
    exit 1
fi

# Check if pods are running
PODS_READY=$(kubectl get pods -n $NAMESPACE -l app=$SERVICE -o jsonpath='{.items[*].status.containerStatuses[0].ready}' 2>/dev/null | grep -c true || echo "0")
if [ "$PODS_READY" = "0" ]; then
    echo "⚠️  Warning: No ready pods found for $SERVICE"
    echo "Checking pod status..."
    kubectl get pods -n $NAMESPACE -l app=$SERVICE
    echo ""
    echo "Waiting for pods to be ready..."
    kubectl wait --for=condition=ready pod -l app=$SERVICE -n $NAMESPACE --timeout=60s 2>/dev/null || true
fi

# Start port-forward
echo "Starting port-forward..."
kubectl port-forward -n $NAMESPACE svc/$SERVICE $PORT:80 >/dev/null 2>&1 &
PF_PID=$!

sleep 3

# Check if port-forward is running
if ps -p $PF_PID > /dev/null 2>&1; then
    echo "✅ Port-forward started (PID: $PF_PID)"
    echo ""
    echo "🌐 Dashboard URL: http://localhost:$PORT"
    echo ""
    echo "Testing connection..."
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:$PORT 2>/dev/null || echo "000")
    if [ "$HTTP_CODE" = "200" ]; then
        echo "✅ Dashboard is accessible!"
    else
        echo "⚠️  Dashboard returned HTTP $HTTP_CODE"
        echo "   Port-forward may still be starting..."
    fi
    echo ""
    echo "To stop port-forward, run:"
    echo "  kill $PF_PID"
    echo "  or"
    echo "  pkill -f 'kubectl port-forward.*$SERVICE'"
    echo ""
    echo "Port-forward is running in background."
    echo "Access dashboard at: http://localhost:$PORT"
else
    echo "❌ Failed to start port-forward"
    exit 1
fi

