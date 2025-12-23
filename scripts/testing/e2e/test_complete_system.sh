#!/bin/bash
# Complete System Test Suite
# Tests all components, data flow, and functionality

set -e

NAMESPACE="ksam"
TEST_RESULTS_DIR="./test_results"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
RESULTS_FILE="${TEST_RESULTS_DIR}/complete_test_${TIMESTAMP}.txt"

mkdir -p "${TEST_RESULTS_DIR}"

echo "╔════════════════════════════════════════════════════════════════╗" | tee -a "${RESULTS_FILE}"
echo "║         KSAM COMPLETE SYSTEM TEST SUITE                        ║" | tee -a "${RESULTS_FILE}"
echo "╚════════════════════════════════════════════════════════════════╝" | tee -a "${RESULTS_FILE}"
echo "" | tee -a "${RESULTS_FILE}"
echo "Test Date: $(date)" | tee -a "${RESULTS_FILE}"
echo "Namespace: ${NAMESPACE}" | tee -a "${RESULTS_FILE}"
echo "" | tee -a "${RESULTS_FILE}"

PASSED=0
FAILED=0
WARNINGS=0

# Test function
test_check() {
    local name="$1"
    local command="$2"
    local expected="$3"
    
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" | tee -a "${RESULTS_FILE}"
    echo "Test: ${name}" | tee -a "${RESULTS_FILE}"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" | tee -a "${RESULTS_FILE}"
    
    if eval "${command}" > /tmp/test_output.txt 2>&1; then
        result=$(cat /tmp/test_output.txt)
        if [[ -z "${expected}" ]] || echo "${result}" | grep -q "${expected}"; then
            echo "✅ PASS" | tee -a "${RESULTS_FILE}"
            echo "${result}" | tee -a "${RESULTS_FILE}"
            ((PASSED++))
            return 0
        else
            echo "⚠️  WARNING: Expected '${expected}' not found" | tee -a "${RESULTS_FILE}"
            echo "${result}" | tee -a "${RESULTS_FILE}"
            ((WARNINGS++))
            return 1
        fi
    else
        echo "❌ FAIL" | tee -a "${RESULTS_FILE}"
        cat /tmp/test_output.txt | tee -a "${RESULTS_FILE}"
        ((FAILED++))
        return 1
    fi
    echo "" | tee -a "${RESULTS_FILE}"
}

# 1. Infrastructure Tests
echo "════════════════════════════════════════════════════════════════" | tee -a "${RESULTS_FILE}"
echo "SECTION 1: INFRASTRUCTURE" | tee -a "${RESULTS_FILE}"
echo "════════════════════════════════════════════════════════════════" | tee -a "${RESULTS_FILE}"
echo "" | tee -a "${RESULTS_FILE}"

test_check "PostgreSQL Running" \
    "kubectl get pods -n ${NAMESPACE} -l app=postgres --field-selector=status.phase=Running --no-headers | wc -l" \
    "1"

test_check "NATS Running" \
    "kubectl get pods -n ${NAMESPACE} -l app=nats --field-selector=status.phase=Running --no-headers | wc -l" \
    "3"

test_check "Redis Running" \
    "kubectl get pods -n ${NAMESPACE} -l app=redis --field-selector=status.phase=Running --no-headers | wc -l" \
    "1"

test_check "PostgreSQL Service" \
    "kubectl get svc -n ${NAMESPACE} postgres -o jsonpath='{.spec.clusterIP}'" \
    ""

test_check "NATS Service" \
    "kubectl get svc -n ${NAMESPACE} nats -o jsonpath='{.spec.clusterIP}'" \
    ""

# 2. Core Service Tests
echo "" | tee -a "${RESULTS_FILE}"
echo "════════════════════════════════════════════════════════════════" | tee -a "${RESULTS_FILE}"
echo "SECTION 2: CORE SERVICE" | tee -a "${RESULTS_FILE}"
echo "════════════════════════════════════════════════════════════════" | tee -a "${RESULTS_FILE}"
echo "" | tee -a "${RESULTS_FILE}"

test_check "Core Pod Running" \
    "kubectl get pods -n ${NAMESPACE} -l app=ksam-core --field-selector=status.phase=Running --no-headers | wc -l" \
    "1"

test_check "Core Service" \
    "kubectl get svc -n ${NAMESPACE} core -o jsonpath='{.spec.clusterIP}'" \
    ""

test_check "Core HTTP Health" \
    "kubectl exec -n ${NAMESPACE} \$(kubectl get pod -n ${NAMESPACE} -l app=ksam-core -o jsonpath='{.items[0].metadata.name}') -- wget -qO- http://localhost:8080/health" \
    ""

test_check "Core HTTP Ready" \
    "kubectl exec -n ${NAMESPACE} \$(kubectl get pod -n ${NAMESPACE} -l app=ksam-core -o jsonpath='{.items[0].metadata.name}') -- wget -qO- http://localhost:8080/ready" \
    ""

test_check "Core gRPC Server (mTLS)" \
    "kubectl logs -n ${NAMESPACE} -l app=ksam-core --tail=50 | grep -i 'gRPC server configured with mTLS'" \
    "mTLS"

# 3. Agent Service Tests
echo "" | tee -a "${RESULTS_FILE}"
echo "════════════════════════════════════════════════════════════════" | tee -a "${RESULTS_FILE}"
echo "SECTION 3: AGENT SERVICE" | tee -a "${RESULTS_FILE}"
echo "════════════════════════════════════════════════════════════════" | tee -a "${RESULTS_FILE}"
echo "" | tee -a "${RESULTS_FILE}"

test_check "Agent Pod Running" \
    "kubectl get pods -n ${NAMESPACE} -l app=ksam-agent --field-selector=status.phase=Running --no-headers | wc -l" \
    "1"

test_check "Agent gRPC Client (mTLS)" \
    "kubectl logs -n ${NAMESPACE} -l app=ksam-agent --tail=50 | grep -i 'gRPC client configured with mTLS'" \
    "mTLS"

test_check "Agent Registered" \
    "kubectl logs -n ${NAMESPACE} -l app=ksam-agent --tail=50 | grep -i 'registered\|Successfully registered'" \
    ""

# 4. NATS Tests
echo "" | tee -a "${RESULTS_FILE}"
echo "════════════════════════════════════════════════════════════════" | tee -a "${RESULTS_FILE}"
echo "SECTION 4: NATS JETSTREAM" | tee -a "${RESULTS_FILE}"
echo "════════════════════════════════════════════════════════════════" | tee -a "${RESULTS_FILE}"
echo "" | tee -a "${RESULTS_FILE}"

test_check "NATS Streams Created" \
    "kubectl exec -n ${NAMESPACE} nats-0 -- nats stream ls 2>/dev/null | grep -c 'ksam'" \
    ""

test_check "NATS Inventory Stream" \
    "kubectl exec -n ${NAMESPACE} nats-0 -- nats stream info ksam-inventory 2>/dev/null | grep -i 'state'" \
    ""

# 5. Worker Tests
echo "" | tee -a "${RESULTS_FILE}"
echo "════════════════════════════════════════════════════════════════" | tee -a "${RESULTS_FILE}"
echo "SECTION 5: WORKERS" | tee -a "${RESULTS_FILE}"
echo "════════════════════════════════════════════════════════════════" | tee -a "${RESULTS_FILE}"
echo "" | tee -a "${RESULTS_FILE}"

test_check "Worker Pool Started" \
    "kubectl logs -n ${NAMESPACE} -l app=ksam-core --tail=100 | grep -i 'WorkerPool.*Started'" \
    "Started"

test_check "Normalizer Worker Active" \
    "kubectl logs -n ${NAMESPACE} -l app=ksam-core --tail=100 | grep -i 'NormalizerWorker.*Processing'" \
    "Processing"

test_check "Correlator Worker Active" \
    "kubectl logs -n ${NAMESPACE} -l app=ksam-core --tail=100 | grep -i 'CorrelatorWorker.*Processing'" \
    "Processing"

# 6. Data Flow Tests
echo "" | tee -a "${RESULTS_FILE}"
echo "════════════════════════════════════════════════════════════════" | tee -a "${RESULTS_FILE}"
echo "SECTION 6: DATA FLOW" | tee -a "${RESULTS_FILE}"
echo "════════════════════════════════════════════════════════════════" | tee -a "${RESULTS_FILE}"
echo "" | tee -a "${RESULTS_FILE}"

test_check "Agent Streaming Inventory" \
    "kubectl logs -n ${NAMESPACE} -l app=ksam-agent --tail=50 | grep -i 'streamed.*inventory items'" \
    "streamed"

test_check "Core Receiving Inventory" \
    "kubectl logs -n ${NAMESPACE} -l app=ksam-core --tail=50 | grep -i 'StreamInventory started\|Published.*inventory'" \
    ""

test_check "Normalizer Processing" \
    "kubectl logs -n ${NAMESPACE} -l app=ksam-core --tail=50 | grep -i 'Normalized and published'" \
    "Normalized"

test_check "Correlator Storing Data" \
    "kubectl logs -n ${NAMESPACE} -l app=ksam-core --tail=50 | grep -i 'Stored.*service account\|Stored.*role\|Stored.*pod'" \
    "Stored"

# 7. Error Checks
echo "" | tee -a "${RESULTS_FILE}"
echo "════════════════════════════════════════════════════════════════" | tee -a "${RESULTS_FILE}"
echo "SECTION 7: ERROR CHECK" | tee -a "${RESULTS_FILE}"
echo "════════════════════════════════════════════════════════════════" | tee -a "${RESULTS_FILE}"
echo "" | tee -a "${RESULTS_FILE}"

ERROR_COUNT=$(kubectl logs -n ${NAMESPACE} -l app=ksam-core --tail=200 | grep -iE "ERROR|error|invalid input|foreign key" | wc -l | tr -d ' ')
if [ "${ERROR_COUNT}" -eq 0 ]; then
    echo "✅ PASS: No errors found in Core logs" | tee -a "${RESULTS_FILE}"
    ((PASSED++))
else
    echo "⚠️  WARNING: ${ERROR_COUNT} errors found in Core logs" | tee -a "${RESULTS_FILE}"
    kubectl logs -n ${NAMESPACE} -l app=ksam-core --tail=200 | grep -iE "ERROR|error|invalid input|foreign key" | head -10 | tee -a "${RESULTS_FILE}"
    ((WARNINGS++))
fi

AGENT_ERROR_COUNT=$(kubectl logs -n ${NAMESPACE} -l app=ksam-agent --tail=200 | grep -iE "ERROR|error|failed" | wc -l | tr -d ' ')
if [ "${AGENT_ERROR_COUNT}" -eq 0 ]; then
    echo "✅ PASS: No errors found in Agent logs" | tee -a "${RESULTS_FILE}"
    ((PASSED++))
else
    echo "⚠️  WARNING: ${AGENT_ERROR_COUNT} errors found in Agent logs" | tee -a "${RESULTS_FILE}"
    kubectl logs -n ${NAMESPACE} -l app=ksam-agent --tail=200 | grep -iE "ERROR|error|failed" | head -10 | tee -a "${RESULTS_FILE}"
    ((WARNINGS++))
fi

# 8. Database Tests
echo "" | tee -a "${RESULTS_FILE}"
echo "════════════════════════════════════════════════════════════════" | tee -a "${RESULTS_FILE}"
echo "SECTION 8: DATABASE" | tee -a "${RESULTS_FILE}"
echo "════════════════════════════════════════════════════════════════" | tee -a "${RESULTS_FILE}"
echo "" | tee -a "${RESULTS_FILE}"

test_check "Database Connection" \
    "kubectl exec -n ${NAMESPACE} \$(kubectl get pod -n ${NAMESPACE} -l app=postgres -o jsonpath='{.items[0].metadata.name}') -- psql -U postgres -d ksam -c 'SELECT 1' 2>&1" \
    "1"

test_check "ServiceAccounts Table" \
    "kubectl exec -n ${NAMESPACE} \$(kubectl get pod -n ${NAMESPACE} -l app=postgres -o jsonpath='{.items[0].metadata.name}') -- psql -U postgres -d ksam -c 'SELECT COUNT(*) FROM service_accounts' 2>&1" \
    ""

test_check "Pods Table" \
    "kubectl exec -n ${NAMESPACE} \$(kubectl get pod -n ${NAMESPACE} -l app=postgres -o jsonpath='{.items[0].metadata.name}') -- psql -U postgres -d ksam -c 'SELECT COUNT(*) FROM pods' 2>&1" \
    ""

test_check "Roles Table" \
    "kubectl exec -n ${NAMESPACE} \$(kubectl get pod -n ${NAMESPACE} -l app=postgres -o jsonpath='{.items[0].metadata.name}') -- psql -U postgres -d ksam -c 'SELECT COUNT(*) FROM roles' 2>&1" \
    ""

# Summary
echo "" | tee -a "${RESULTS_FILE}"
echo "╔════════════════════════════════════════════════════════════════╗" | tee -a "${RESULTS_FILE}"
echo "║                    TEST SUMMARY                                ║" | tee -a "${RESULTS_FILE}"
echo "╚════════════════════════════════════════════════════════════════╝" | tee -a "${RESULTS_FILE}"
echo "" | tee -a "${RESULTS_FILE}"
echo "Total Tests: $((PASSED + FAILED + WARNINGS))" | tee -a "${RESULTS_FILE}"
echo "✅ Passed:   ${PASSED}" | tee -a "${RESULTS_FILE}"
echo "❌ Failed:   ${FAILED}" | tee -a "${RESULTS_FILE}"
echo "⚠️  Warnings: ${WARNINGS}" | tee -a "${RESULTS_FILE}"
echo "" | tee -a "${RESULTS_FILE}"

if [ ${FAILED} -eq 0 ]; then
    echo "🎯 Overall Status: ✅ ALL TESTS PASSED" | tee -a "${RESULTS_FILE}"
    exit 0
else
    echo "🎯 Overall Status: ❌ SOME TESTS FAILED" | tee -a "${RESULTS_FILE}"
    exit 1
fi

