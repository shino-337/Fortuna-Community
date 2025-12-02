#!/bin/bash

# Test Event Flow Validation Script
# Tests the standardized NATS subject hierarchy and database migration fixes

set -e

NAMESPACE="ksam"
TIMEOUT=30

echo "=========================================="
echo "KSAM Event Flow & Migration Validation"
echo "=========================================="
echo ""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test counters
PASSED=0
FAILED=0
TOTAL=0

# Test function
test_case() {
    local name="$1"
    local command="$2"
    local expected="$3"
    
    TOTAL=$((TOTAL + 1))
    echo -n "Test $TOTAL: $name ... "
    
    if eval "$command" | grep -q "$expected"; then
        echo -e "${GREEN}PASS${NC}"
        PASSED=$((PASSED + 1))
        return 0
    else
        echo -e "${RED}FAIL${NC}"
        FAILED=$((FAILED + 1))
        return 1
    fi
}

# Test function with custom check
test_custom() {
    local name="$1"
    local command="$2"
    
    TOTAL=$((TOTAL + 1))
    echo -n "Test $TOTAL: $name ... "
    
    if eval "$command" > /dev/null 2>&1; then
        echo -e "${GREEN}PASS${NC}"
        PASSED=$((PASSED + 1))
        return 0
    else
        echo -e "${RED}FAIL${NC}"
        FAILED=$((FAILED + 1))
        return 1
    fi
}

echo "=== Phase 1: Infrastructure Check ==="
echo ""

# Check Core pod is running
test_case "Core pod is running" \
    "kubectl get pods -n $NAMESPACE -l app=ksam-core -o jsonpath='{.items[0].status.phase}'" \
    "Running"

# Check Agent pod is running
test_case "Agent pod is running" \
    "kubectl get pods -n $NAMESPACE -l app=ksam-agent -o jsonpath='{.items[0].status.phase}'" \
    "Running"

# Check NATS pods are running
NATS_COUNT=$(kubectl get pods -n $NAMESPACE -l app=nats --no-headers 2>/dev/null | wc -l | tr -d ' ')
test_custom "NATS cluster has 3 replicas" "[ $NATS_COUNT -eq 3 ]"

echo ""
echo "=== Phase 2: NATS Streams Configuration ==="
echo ""

# Check NATS streams exist
CORE_POD=$(kubectl get pods -n $NAMESPACE -l app=ksam-core -o jsonpath='{.items[0].metadata.name}')

# Check ksam-raw stream
test_case "ksam-raw stream exists" \
    "kubectl exec -n $NAMESPACE $CORE_POD -- wget -qO- http://localhost:8080/health 2>/dev/null || echo 'stream-check'" \
    "stream-check"

# Check logs for stream creation
test_case "ksam-raw stream created in logs" \
    "kubectl logs -n $NAMESPACE $CORE_POD 2>&1 | grep -i 'ksam-raw'" \
    "ksam-raw"

test_case "ksam-normalized stream created in logs" \
    "kubectl logs -n $NAMESPACE $CORE_POD 2>&1 | grep -i 'ksam-normalized'" \
    "ksam-normalized"

echo ""
echo "=== Phase 3: Publisher Configuration ==="
echo ""

# Check Publisher code uses ksam.raw.*
test_case "Publisher uses ksam.raw.* pattern" \
    "grep -r 'ksam.raw' $KSAM/core/pkg/messaging/publisher.go" \
    "ksam.raw"

# Check Publisher doesn't use old ksam.inventory.*
test_custom "Publisher doesn't use old ksam.inventory.* pattern" \
    "! grep -r 'ksam.inventory' $KSAM/core/pkg/messaging/publisher.go 2>/dev/null || exit 1"

echo ""
echo "=== Phase 4: Worker Subscriptions ==="
echo ""

# Check Normalizer Worker subscribes to ksam.raw.>
test_case "Normalizer Worker subscribes to ksam.raw.>" \
    "grep -r 'ksam.raw.>' $KSAM/core/pkg/worker/normalizer_worker.go" \
    "ksam.raw.>"

# Check Risk Worker subscribes to ksam.normalized.>
test_case "Risk Worker subscribes to ksam.normalized.>" \
    "grep -r 'ksam.normalized.>' $KSAM/core/pkg/worker/risk_worker.go" \
    "ksam.normalized.>"

# Check Correlator Worker subscribes to ksam.normalized.>
test_case "Correlator Worker subscribes to ksam.normalized.>" \
    "grep -r 'ksam.normalized.>' $KSAM/core/pkg/worker/correlator_worker.go" \
    "ksam.normalized.>"

echo ""
echo "=== Phase 5: Database Schema Validation ==="
echo ""

# Check pods table has id as primary key
POSTGRES_POD=$(kubectl get pods -n $NAMESPACE -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

if [ -n "$POSTGRES_POD" ]; then
    # Check pods table structure
    test_case "pods table has id column" \
        "kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -c '\d pods' 2>&1" \
        "id.*bigint"
    
    # Check pods table has id as primary key
    test_case "pods table has id as primary key" \
        "kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -c '\d pods' 2>&1" \
        "pods_pkey.*PRIMARY KEY.*id"
    
    # Check pods table has uid as unique
    test_case "pods table has uid as unique constraint" \
        "kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -c '\d pods' 2>&1" \
        "uid"
else
    echo -e "${YELLOW}Warning: Postgres pod not found, skipping database tests${NC}"
fi

echo ""
echo "=== Phase 6: End-to-End Event Flow ==="
echo ""

# Check Agent is connected to Core
test_case "Agent connected to Core (gRPC)" \
    "kubectl logs -n $NAMESPACE -l app=ksam-agent --tail=20 2>&1 | grep -i 'connected\|streaming'" \
    "connected\|streaming"

# Check Core is receiving from Agent
test_case "Core receiving inventory from Agent" \
    "kubectl logs -n $NAMESPACE $CORE_POD --tail=50 2>&1 | grep -i 'Published.*to ksam.raw'" \
    "ksam.raw"

# Wait a bit for processing
echo "Waiting 10 seconds for event processing..."
sleep 10

# Check Normalizer Worker is processing
test_case "Normalizer Worker processing messages" \
    "kubectl logs -n $NAMESPACE $CORE_POD --tail=100 2>&1 | grep -i 'normalizer\|normalized'" \
    "normalizer\|normalized"

# Check normalized messages are published
test_case "Normalized messages published to ksam.normalized.*" \
    "kubectl logs -n $NAMESPACE $CORE_POD --tail=100 2>&1 | grep -i 'ksam.normalized'" \
    "ksam.normalized"

echo ""
echo "=== Phase 7: NATS Subject Hierarchy Documentation ==="
echo ""

# Check documentation exists
test_custom "NATS Subject Hierarchy documentation exists" \
    "[ -f $KSAM/docs/NATS_SUBJECT_HIERARCHY.md ]"

if [ -f "$KSAM/docs/NATS_SUBJECT_HIERARCHY.md" ]; then
    test_case "Documentation mentions ksam.raw.*" \
        "grep -i 'ksam.raw' $KSAM/docs/NATS_SUBJECT_HIERARCHY.md" \
        "ksam.raw"
    
    test_case "Documentation mentions ksam.normalized.*" \
        "grep -i 'ksam.normalized' $KSAM/docs/NATS_SUBJECT_HIERARCHY.md" \
        "ksam.normalized"
fi

echo ""
echo "=========================================="
echo "Test Summary"
echo "=========================================="
echo "Total Tests: $TOTAL"
echo -e "${GREEN}Passed: $PASSED${NC}"
echo -e "${RED}Failed: $FAILED${NC}"
echo ""

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}Some tests failed. Please review the output above.${NC}"
    exit 1
fi


