#!/bin/bash

# Script to open Landing Page with proper setup
# Ensures port-forward is running and opens in browser

set -e

NAMESPACE="ksam"
SERVICE="ksam-dashboard"
LOCAL_PORT=3000
SERVICE_PORT=80

echo "🚀 K8s Workload Management Platform - Landing Page Launcher"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    echo "❌ kubectl not found. Please install kubectl first."
    exit 1
fi

# Check if service exists
if ! kubectl get svc -n $NAMESPACE $SERVICE &> /dev/null; then
    echo "❌ Service $SERVICE not found in namespace $NAMESPACE"
    exit 1
fi

# Kill existing port-forward on this port
echo "🔍 Checking for existing port-forward on port $LOCAL_PORT..."
if lsof -ti:$LOCAL_PORT &> /dev/null; then
    echo "⚠️  Port $LOCAL_PORT is in use. Killing existing process..."
    lsof -ti:$LOCAL_PORT | xargs kill -9 2>/dev/null || true
    sleep 2
fi

# Start port-forward in background
echo "🔌 Starting port-forward: localhost:$LOCAL_PORT → $SERVICE:$SERVICE_PORT"
kubectl port-forward -n $NAMESPACE svc/$SERVICE $LOCAL_PORT:$SERVICE_PORT > /tmp/ksam-port-forward.log 2>&1 &
PORT_FORWARD_PID=$!

# Wait for port-forward to be ready
echo "⏳ Waiting for port-forward to be ready..."
for i in {1..10}; do
    if lsof -ti:$LOCAL_PORT &> /dev/null; then
        echo "✅ Port-forward ready!"
        break
    fi
    sleep 1
    if [ $i -eq 10 ]; then
        echo "❌ Port-forward failed to start. Check logs: tail -f /tmp/ksam-port-forward.log"
        exit 1
    fi
done

# Get URLs
LOCALHOST_URL="http://localhost:$LOCAL_PORT"
NODEPORT=$(kubectl get svc -n $NAMESPACE $SERVICE -o jsonpath='{.spec.ports[0].nodePort}')
MINIKUBE_IP=$(minikube ip 2>/dev/null || echo "N/A")

if [ "$MINIKUBE_IP" != "N/A" ]; then
    MINIKUBE_URL="http://$MINIKUBE_IP:$NODEPORT"
else
    MINIKUBE_URL="N/A (minikube not running)"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✅ Landing Page is now accessible:"
echo ""
echo "   📍 Port-Forward (Recommended):"
echo "      $LOCALHOST_URL"
echo ""
echo "   📍 NodePort (Direct):"
echo "      $MINIKUBE_URL"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "⚠️  IMPORTANT: Open in INCOGNITO/PRIVATE window to bypass cache!"
echo ""
echo "   Chrome/Edge: Cmd+Shift+N"
echo "   Firefox:     Cmd+Shift+P"
echo "   Safari:      Cmd+Shift+N"
echo ""
echo "🔧 To stop port-forward: kill $PORT_FORWARD_PID"
echo "📋 Port-forward logs:    tail -f /tmp/ksam-port-forward.log"
echo ""

# Try to open in default browser (will use cached version, so warn user)
if command -v open &> /dev/null; then
    echo "🌐 Opening in default browser..."
    open "$LOCALHOST_URL"
    echo ""
    echo "⚠️  If you see Login page instead of Landing page:"
    echo "   → Your browser has cached auth token"
    echo "   → Open a NEW INCOGNITO window and visit: $LOCALHOST_URL"
    echo ""
elif command -v xdg-open &> /dev/null; then
    xdg-open "$LOCALHOST_URL"
else
    echo "ℹ️  Please open this URL manually: $LOCALHOST_URL"
fi

echo "✨ Done! Landing page should be visible."
echo ""

