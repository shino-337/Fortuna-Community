#!/bin/bash

# Port forwarding script for easy access

set -e

NAMESPACE="ksam"

echo "🔌 Setting up port forwarding..."

# Function to check if port is in use
check_port() {
    lsof -Pi :$1 -sTCP:LISTEN -t >/dev/null 2>&1
}

# Port forward Core Controller
if ! check_port 8080; then
    echo "Port forwarding Core Controller (8080)..."
    kubectl port-forward -n $NAMESPACE svc/ksam-core 8080:8080 >/dev/null 2>&1 &
    CORE_PID=$!
    echo "Core Controller: http://localhost:8080 (PID: $CORE_PID)"
else
    echo "Port 8080 already in use, skipping Core Controller"
fi

# Port forward Dashboard
if ! check_port 3000; then
    echo "Port forwarding Dashboard (3000)..."
    kubectl port-forward -n $NAMESPACE svc/ksam-dashboard 3000:80 >/dev/null 2>&1 &
    DASHBOARD_PID=$!
    echo "Dashboard: http://localhost:3000 (PID: $DASHBOARD_PID)"
else
    echo "Port 3000 already in use, skipping Dashboard"
fi

# Port forward PostgreSQL (optional)
if ! check_port 5432; then
    read -p "Port forward PostgreSQL? (y/N) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        kubectl port-forward -n $NAMESPACE svc/postgres 5432:5432 >/dev/null 2>&1 &
        POSTGRES_PID=$!
        echo "PostgreSQL: localhost:5432 (PID: $POSTGRES_PID)"
    fi
fi

echo ""
echo "✅ Port forwarding setup complete!"
echo ""
echo "Access:"
echo "  - Dashboard: http://localhost:3000"
echo "  - Core API: http://localhost:8080"
echo "  - Metrics: http://localhost:8080/metrics"
echo ""
echo "To stop port forwarding, run:"
echo "  pkill -f 'kubectl port-forward'"

