#!/bin/bash

# Performance Test Cases for KSAM System
# Usage: ./scripts/test_performance.sh [namespace]

set -e

NAMESPACE="${1:-ksam}"

echo "⚡ Running KSAM Performance Tests"
echo "Namespace: $NAMESPACE"
echo "================================"
echo ""

# Test 1: Pod Startup Time
echo "Test 1: Pod Startup Time"
CORE_POD=$(kubectl get pods -n $NAMESPACE -l app=ksam-core -o jsonpath='{.items[0].metadata.name}')
AGENT_POD=$(kubectl get pods -n $NAMESPACE -l app=ksam-agent -o jsonpath='{.items[0].metadata.name}')

if [ -n "$CORE_POD" ]; then
    CORE_AGE=$(kubectl get pod -n $NAMESPACE $CORE_POD -o jsonpath='{.metadata.creationTimestamp}')
    echo "  Core pod created: $CORE_AGE"
fi

if [ -n "$AGENT_POD" ]; then
    AGENT_AGE=$(kubectl get pod -n $NAMESPACE $AGENT_POD -o jsonpath='{.metadata.creationTimestamp}')
    echo "  Agent pod created: $AGENT_AGE"
fi

# Test 2: Response Time - Health Check
echo ""
echo "Test 2: Health Check Response Time"
if [ -n "$CORE_POD" ]; then
    START_TIME=$(date +%s%N)
    kubectl exec -n $NAMESPACE $CORE_POD -- wget -q -O- http://localhost:8080/ready > /dev/null 2>&1 || true
    END_TIME=$(date +%s%N)
    DURATION=$(( (END_TIME - START_TIME) / 1000000 ))
    echo "  Health check response time: ${DURATION}ms"
    
    if [ $DURATION -lt 1000 ]; then
        echo "  ✅ Response time acceptable (< 1s)"
    else
        echo "  ⚠️  Response time slow (> 1s)"
    fi
fi

# Test 3: Log Volume
echo ""
echo "Test 3: Log Volume"
if [ -n "$CORE_POD" ]; then
    CORE_LOG_LINES=$(kubectl logs -n $NAMESPACE $CORE_POD 2>&1 | wc -l)
    echo "  Core log lines: $CORE_LOG_LINES"
fi

if [ -n "$AGENT_POD" ]; then
    AGENT_LOG_LINES=$(kubectl logs -n $NAMESPACE $AGENT_POD 2>&1 | wc -l)
    echo "  Agent log lines: $AGENT_LOG_LINES"
fi

# Test 4: Database Query Performance
echo ""
echo "Test 4: Database Query Performance"
POSTGRES_POD=$(kubectl get pods -n $NAMESPACE -l app=postgres -o jsonpath='{.items[0].metadata.name}')

if [ -n "$POSTGRES_POD" ]; then
    START_TIME=$(date +%s%N)
    kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -c "SELECT COUNT(*) FROM service_accounts;" > /dev/null 2>&1
    END_TIME=$(date +%s%N)
    DURATION=$(( (END_TIME - START_TIME) / 1000000 ))
    echo "  Database query time: ${DURATION}ms"
    
    if [ $DURATION -lt 500 ]; then
        echo "  ✅ Query performance acceptable (< 500ms)"
    else
        echo "  ⚠️  Query performance slow (> 500ms)"
    fi
fi

# Test 5: Memory Usage
echo ""
echo "Test 5: Memory Usage"
if command -v kubectl top &> /dev/null; then
    kubectl top pods -n $NAMESPACE 2>/dev/null | grep -E "NAME|ksam" || echo "  Metrics not available"
else
    echo "  Metrics server not available"
fi

# Test 6: CPU Usage
echo ""
echo "Test 6: CPU Usage"
if command -v kubectl top &> /dev/null; then
    kubectl top pods -n $NAMESPACE 2>/dev/null | grep -E "NAME|ksam" || echo "  Metrics not available"
else
    echo "  Metrics server not available"
fi

echo ""
echo "================================"
echo "✅ Performance tests completed"


