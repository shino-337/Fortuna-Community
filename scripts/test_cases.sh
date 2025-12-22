#!/bin/bash

# Test Cases for KSAM System
# Usage: ./scripts/test_cases.sh [namespace]

set -e

NAMESPACE="${1:-ksam}"
PASSED=0
FAILED=0
TOTAL=0

echo "🧪 Running KSAM Test Cases"
echo "Namespace: $NAMESPACE"
echo "================================"
echo ""

# Helper function
test_case() {
    local test_name="$1"
    local test_command="$2"
    TOTAL=$((TOTAL + 1))
    
    echo "Test $TOTAL: $test_name"
    if eval "$test_command" > /dev/null 2>&1; then
        echo "  ✅ PASSED"
        PASSED=$((PASSED + 1))
        return 0
    else
        echo "  ❌ FAILED"
        FAILED=$((FAILED + 1))
        return 1
    fi
}

# Test 1: Core Pod Status
test_case "Core pod is running" "kubectl get pods -n $NAMESPACE -l app=ksam-core -o jsonpath='{.items[0].status.phase}' | grep -q Running"

# Test 2: Agent Pod Status
test_case "Agent pod is running" "kubectl get pods -n $NAMESPACE -l app=ksam-agent -o jsonpath='{.items[0].status.phase}' | grep -q Running"

# Test 3: Core Pod Ready
test_case "Core pod is ready" "kubectl get pods -n $NAMESPACE -l app=ksam-core -o jsonpath='{.items[0].status.containerStatuses[0].ready}' | grep -q true"

# Test 4: Agent Pod Ready
test_case "Agent pod is ready" "kubectl get pods -n $NAMESPACE -l app=ksam-agent -o jsonpath='{.items[0].status.containerStatuses[0].ready}' | grep -q true"

# Test 5: Core Service Exists
test_case "Core service exists" "kubectl get svc ksam-core -n $NAMESPACE > /dev/null"

# Test 6: Core Service HTTP Port
test_case "Core service exposes HTTP port" "kubectl get svc ksam-core -n $NAMESPACE -o jsonpath='{.spec.ports[?(@.name==\"http\")].port}' | grep -q 8080"

# Test 7: Core Service gRPC Port
test_case "Core service exposes gRPC port" "kubectl get svc ksam-core -n $NAMESPACE -o jsonpath='{.spec.ports[?(@.name==\"grpc\")].port}' | grep -q 9090"

# Test 8: Database Pod Running
test_case "PostgreSQL pod is running" "kubectl get pods -n $NAMESPACE -l app=postgres -o jsonpath='{.items[0].status.phase}' | grep -q Running"

# Test 9: Database Tables - nodes
test_case "Database table 'nodes' exists" "kubectl exec -n $NAMESPACE \$(kubectl get pods -n $NAMESPACE -l app=postgres -o jsonpath='{.items[0].metadata.name}') -- psql -U postgres -d ksam -c '\dt nodes' > /dev/null 2>&1"

# Test 10: Database Tables - policies
test_case "Database table 'policies' exists" "kubectl exec -n $NAMESPACE \$(kubectl get pods -n $NAMESPACE -l app=postgres -o jsonpath='{.items[0].metadata.name}') -- psql -U postgres -d ksam -c '\dt policies' > /dev/null 2>&1"

# Test 11: Database Tables - insights
test_case "Database table 'insights' exists" "kubectl exec -n $NAMESPACE \$(kubectl get pods -n $NAMESPACE -l app=postgres -o jsonpath='{.items[0].metadata.name}') -- psql -U postgres -d ksam -c '\dt insights' > /dev/null 2>&1"

# Test 12: Database Tables - events_index
test_case "Database table 'events_index' exists" "kubectl exec -n $NAMESPACE \$(kubectl get pods -n $NAMESPACE -l app=postgres -o jsonpath='{.items[0].metadata.name}') -- psql -U postgres -d ksam -c '\dt events_index' > /dev/null 2>&1"

# Test 13: Core gRPC Server Started
test_case "Core gRPC server started" "kubectl logs -n $NAMESPACE \$(kubectl get pods -n $NAMESPACE -l app=ksam-core -o jsonpath='{.items[0].metadata.name}') | grep -q 'Starting gRPC server'"

# Test 14: Core HTTP Server Started
test_case "Core HTTP server started" "kubectl logs -n $NAMESPACE \$(kubectl get pods -n $NAMESPACE -l app=ksam-core -o jsonpath='{.items[0].metadata.name}') | grep -q 'GIN-debug'"

# Test 15: Agent ServiceAccount Exists
test_case "Agent ServiceAccount exists" "kubectl get serviceaccount ksam-agent -n $NAMESPACE > /dev/null"

# Test 16: Agent RBAC Configured
test_case "Agent ClusterRole exists" "kubectl get clusterrole ksam-agent-reader > /dev/null"

# Test 17: Agent ClusterRoleBinding Exists
test_case "Agent ClusterRoleBinding exists" "kubectl get clusterrolebinding ksam-agent-reader > /dev/null"

# Test 18: Core Image
CORE_IMAGE=$(kubectl get pods -n $NAMESPACE -l app=ksam-core -o jsonpath='{.items[0].spec.containers[0].image}' 2>/dev/null || echo "")
test_case "Core pod uses correct image" "[ -n \"$CORE_IMAGE\" ] && echo \"$CORE_IMAGE\" | grep -q ksam-core"

# Test 19: Agent Image
AGENT_IMAGE=$(kubectl get pods -n $NAMESPACE -l app=ksam-agent -o jsonpath='{.items[0].spec.containers[0].image}' 2>/dev/null || echo "")
test_case "Agent pod uses correct image" "[ -n \"$AGENT_IMAGE\" ] && echo \"$AGENT_IMAGE\" | grep -q ksam-agent"

# Test 20: Core Health Check
test_case "Core health endpoint responds" "kubectl exec -n $NAMESPACE \$(kubectl get pods -n $NAMESPACE -l app=ksam-core -o jsonpath='{.items[0].metadata.name}') -- wget -q -O- http://localhost:8080/ready > /dev/null 2>&1 || kubectl run -n $NAMESPACE --rm -i --restart=Never test-curl --image=curlimages/curl -- curl -s http://ksam-core.$NAMESPACE.svc.cluster.local:8080/ready > /dev/null 2>&1"

# Test 21: Database Connection
test_case "Database is accessible" "kubectl exec -n $NAMESPACE \$(kubectl get pods -n $NAMESPACE -l app=postgres -o jsonpath='{.items[0].metadata.name}') -- psql -U postgres -d ksam -c 'SELECT 1' > /dev/null 2>&1"

# Test 22: Agent Logs Available
test_case "Agent logs are accessible" "kubectl logs -n $NAMESPACE \$(kubectl get pods -n $NAMESPACE -l app=ksam-agent -o jsonpath='{.items[0].metadata.name}') --tail=1 > /dev/null 2>&1"

# Test 23: Core Logs Available
test_case "Core logs are accessible" "kubectl logs -n $NAMESPACE \$(kubectl get pods -n $NAMESPACE -l app=ksam-core -o jsonpath='{.items[0].metadata.name}') --tail=1 > /dev/null 2>&1"

# Test 24: Agent DaemonSet Exists
test_case "Agent DaemonSet exists" "kubectl get daemonset ksam-agent -n $NAMESPACE > /dev/null"

# Test 25: Core Deployment Exists
test_case "Core Deployment exists" "kubectl get deployment ksam-core -n $NAMESPACE > /dev/null"

echo ""
echo "================================"
echo "Test Results:"
echo "  Total:  $TOTAL"
echo "  Passed: $PASSED"
echo "  Failed: $FAILED"
echo "================================"

if [ $FAILED -eq 0 ]; then
    echo "✅ All tests passed!"
    exit 0
else
    echo "❌ Some tests failed"
    exit 1
fi


