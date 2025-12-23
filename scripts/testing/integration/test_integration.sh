#!/bin/bash

# Integration Test Cases for KSAM System
# Usage: ./scripts/test_integration.sh [namespace]

set -e

NAMESPACE="${1:-ksam}"

echo "🔗 Running KSAM Integration Tests"
echo "Namespace: $NAMESPACE"
echo "================================"
echo ""

# Test 1: Agent to Core Connection
echo "Test 1: Agent to Core Connection"
CORE_POD=$(kubectl get pods -n $NAMESPACE -l app=ksam-core -o jsonpath='{.items[0].metadata.name}')
AGENT_POD=$(kubectl get pods -n $NAMESPACE -l app=ksam-agent -o jsonpath='{.items[0].metadata.name}')

if [ -z "$CORE_POD" ] || [ -z "$AGENT_POD" ]; then
    echo "  ❌ FAILED: Core or Agent pod not found"
    exit 1
fi

echo "  Core pod: $CORE_POD"
echo "  Agent pod: $AGENT_POD"

# Check if agent can resolve core service
if kubectl exec -n $NAMESPACE $AGENT_POD -- nslookup ksam-core.$NAMESPACE.svc.cluster.local > /dev/null 2>&1; then
    echo "  ✅ Agent can resolve core service"
else
    echo "  ⚠️  Agent cannot resolve core service (may be normal)"
fi

# Test 2: Data Flow - Check Core Logs for Agent Activity
echo ""
echo "Test 2: Data Flow - Agent Activity in Core Logs"
CORE_LOGS=$(kubectl logs -n $NAMESPACE $CORE_POD --tail=100 | grep -E "AgentService|agent/sync" | head -5 || echo "")
if [ -n "$CORE_LOGS" ]; then
    echo "  ✅ Core receiving data from agent"
    echo "  Sample logs:"
    echo "$CORE_LOGS" | sed 's/^/    /'
else
    echo "  ⚠️  No agent activity in core logs yet"
fi

# Test 3: Database Tables Data
echo ""
echo "Test 3: Database Tables Data"
POSTGRES_POD=$(kubectl get pods -n $NAMESPACE -l app=postgres -o jsonpath='{.items[0].metadata.name}')

echo "  Checking nodes table..."
NODE_COUNT=$(kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM nodes;" 2>/dev/null | tr -d ' ' || echo "0")
if [ "$NODE_COUNT" -gt 0 ]; then
    echo "  ✅ Nodes table has $NODE_COUNT entries"
else
    echo "  ⚠️  Nodes table is empty (may be normal if agent hasn't registered)"
fi

echo "  Checking policies table..."
POLICY_COUNT=$(kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM policies;" 2>/dev/null | tr -d ' ' || echo "0")
echo "  Policies table has $POLICY_COUNT entries"

echo "  Checking insights table..."
INSIGHT_COUNT=$(kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM insights;" 2>/dev/null | tr -d ' ' || echo "0")
echo "  Insights table has $INSIGHT_COUNT entries"

echo "  Checking events_index table..."
EVENT_COUNT=$(kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM events_index;" 2>/dev/null | tr -d ' ' || echo "0")
echo "  Events_index table has $EVENT_COUNT entries"

# Test 4: gRPC Server Status
echo ""
echo "Test 4: gRPC Server Status"
GRPC_LOGS=$(kubectl logs -n $NAMESPACE $CORE_POD | grep -E "Starting gRPC|AgentService" | head -3 || echo "")
if [ -n "$GRPC_LOGS" ]; then
    echo "  ✅ gRPC server started"
    echo "$GRPC_LOGS" | sed 's/^/    /'
else
    echo "  ⚠️  gRPC server logs not found"
fi

# Test 5: Agent Registration
echo ""
echo "Test 5: Agent Registration"
AGENT_LOGS=$(kubectl logs -n $NAMESPACE $AGENT_POD --tail=100 | grep -E "Register|Stream|New gRPC|Using new" | head -5 || echo "")
if [ -n "$AGENT_LOGS" ]; then
    echo "  ✅ Agent registration activity found"
    echo "$AGENT_LOGS" | sed 's/^/    /'
else
    echo "  ⚠️  No registration activity in agent logs"
fi

# Test 6: Service Endpoints
echo ""
echo "Test 6: Service Endpoints"
CORE_SVC=$(kubectl get svc ksam-core -n $NAMESPACE -o jsonpath='{.spec.clusterIP}' 2>/dev/null || echo "")
if [ -n "$CORE_SVC" ]; then
    echo "  ✅ Core service IP: $CORE_SVC"
    HTTP_PORT=$(kubectl get svc ksam-core -n $NAMESPACE -o jsonpath='{.spec.ports[?(@.name=="http")].port}' 2>/dev/null || echo "")
    GRPC_PORT=$(kubectl get svc ksam-core -n $NAMESPACE -o jsonpath='{.spec.ports[?(@.name=="grpc")].port}' 2>/dev/null || echo "")
    echo "  HTTP port: ${HTTP_PORT:-N/A}"
    echo "  gRPC port: ${GRPC_PORT:-N/A}"
else
    echo "  ❌ Core service not found"
fi

# Test 7: Resource Usage
echo ""
echo "Test 7: Resource Usage"
echo "  Core pod resources:"
kubectl top pod -n $NAMESPACE $CORE_POD 2>/dev/null || echo "    Metrics not available"
echo "  Agent pod resources:"
kubectl top pod -n $NAMESPACE $AGENT_POD 2>/dev/null || echo "    Metrics not available"

echo ""
echo "================================"
echo "✅ Integration tests completed"


