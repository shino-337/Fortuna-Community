#!/bin/bash
# Start port-forwards for API and Dashboard

echo "=========================================="
echo "Starting Port-Forwards"
echo "=========================================="
echo ""

# Kill existing port-forwards
pkill -f "kubectl port-forward.*8080" 2>/dev/null
pkill -f "kubectl port-forward.*3000" 2>/dev/null
sleep 2

# Start Core API port-forward
echo "Starting Core API port-forward (8080)..."
kubectl port-forward -n ksam svc/ksam-core 8080:8080 >/tmp/ksam-core-pf.log 2>&1 &
CORE_PF=$!
echo "  PID: $CORE_PF"

# Start Dashboard port-forward
echo "Starting Dashboard port-forward (3000)..."
kubectl port-forward -n ksam svc/ksam-dashboard 3000:80 >/tmp/ksam-dashboard-pf.log 2>&1 &
DASH_PF=$!
echo "  PID: $DASH_PF"

sleep 5

# Test connections
echo ""
echo "Testing connections..."
if curl -s http://localhost:8080/health >/dev/null 2>&1; then
  echo "  ✅ Core API: http://localhost:8080"
else
  echo "  ⚠️  Core API: Not ready yet (check logs: /tmp/ksam-core-pf.log)"
fi

if curl -s http://localhost:3000 >/dev/null 2>&1; then
  echo "  ✅ Dashboard: http://localhost:3000"
else
  echo "  ⚠️  Dashboard: Not ready yet (check logs: /tmp/ksam-dashboard-pf.log)"
fi

echo ""
echo "=========================================="
echo "Port-Forwards Running"
echo "=========================================="
echo ""
echo "To stop port-forwards:"
echo "  kill $CORE_PF $DASH_PF"
echo "  or: pkill -f 'kubectl port-forward'"
echo ""
echo "Access:"
echo "  - API: http://localhost:8080"
echo "  - Dashboard: http://localhost:3000"
echo ""

