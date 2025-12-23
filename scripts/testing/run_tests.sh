#!/bin/bash

# Script to run all tests
# Usage: ./scripts/run_tests.sh [namespace]

set -e

NAMESPACE="${1:-ksam}"

echo "🧪 Running All Tests"
echo "Namespace: $NAMESPACE"
echo ""

# Test 1: Check Core Pod
echo "Test 1: Checking Core Pod..."
CORE_POD=$(kubectl get pods -n $NAMESPACE -l app=ksam-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

if [ -z "$CORE_POD" ]; then
    echo "❌ Core pod not found"
    exit 1
fi

echo "✅ Core pod: $CORE_POD"
kubectl get pod -n $NAMESPACE $CORE_POD -o jsonpath='{.status.phase}'
echo ""

# Test 2: Check Migration
echo ""
echo "Test 2: Checking Migration..."
kubectl logs -n $NAMESPACE $CORE_POD | grep -E "Running migration|Migration.*010|Implementation Guide" | head -5 || echo "⚠️  No migration logs found"

POSTGRES_POD=$(kubectl get pods -n $NAMESPACE -l app=postgres -o jsonpath='{.items[0].metadata.name}')
echo "Checking for new tables..."
kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -c "\dt" | grep -E "nodes|policies|insights|events_index" && echo "✅ New tables found" || echo "⚠️  New tables not found"

# Test 3: Check gRPC Server
echo ""
echo "Test 3: Checking gRPC Server..."
kubectl logs -n $NAMESPACE $CORE_POD | grep -E "Starting gRPC|AgentService|Register" | head -5 || echo "⚠️  No gRPC logs found"

# Test 4: Check Agent
echo ""
echo "Test 4: Checking Agent..."
AGENT_PODS=$(kubectl get pods -n $NAMESPACE -l app=ksam-agent -o jsonpath='{.items[*].metadata.name}' 2>/dev/null || echo "")

if [ -z "$AGENT_PODS" ]; then
    echo "⚠️  Agent pods not found"
else
    echo "✅ Agent pods found: $AGENT_PODS"
    for pod in $AGENT_PODS; do
        echo "  Checking $pod..."
        kubectl logs -n $NAMESPACE $pod --tail=20 | grep -E "Register|Stream|New gRPC|Using new" | head -5 || echo "    No registration logs found"
    done
fi

# Test 5: Check Database Tables
echo ""
echo "Test 5: Checking Database Tables..."
kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -c "SELECT COUNT(*) as total_tables FROM pg_tables WHERE schemaname = 'public';" 2>&1

# Test 6: Check Nodes Table (for registered agents)
echo ""
echo "Test 6: Checking Nodes Table..."
kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -c "SELECT * FROM nodes LIMIT 5;" 2>&1 || echo "⚠️  Nodes table not found or empty"

# Test 7: Check Core Logs for Activity
echo ""
echo "Test 7: Checking Core Logs for Recent Activity..."
kubectl logs -n $NAMESPACE $CORE_POD --tail=30 | tail -10

echo ""
echo "✅ All tests completed!"


