#!/bin/bash

# Test Admission Metrics Exposure
# This script tests if admission webhook metrics are properly exposed

set -e

echo "=========================================="
echo "Testing Admission Metrics Exposure"
echo "=========================================="

# Get core service endpoint
CORE_SERVICE="${CORE_SERVICE:-ksam-core.ksam.svc.cluster.local}"
CORE_PORT="${CORE_PORT:-8080}"
METRICS_URL="http://${CORE_SERVICE}:${CORE_PORT}/metrics"

echo ""
echo "1. Checking if metrics endpoint is accessible..."
echo "   URL: ${METRICS_URL}"

# Try to access metrics endpoint
if kubectl exec -n ksam deployment/ksam-core -- wget -q -O- ${METRICS_URL} > /tmp/metrics_output.txt 2>&1; then
    echo "   ✅ Metrics endpoint is accessible"
else
    echo "   ⚠️  Cannot access from pod, trying port-forward..."
    
    # Try port-forward
    kubectl port-forward -n ksam deployment/ksam-core 8080:8080 &
    PF_PID=$!
    sleep 2
    
    if curl -s http://localhost:8080/metrics > /tmp/metrics_output.txt 2>&1; then
        echo "   ✅ Metrics endpoint accessible via port-forward"
        METRICS_URL="http://localhost:8080/metrics"
    else
        echo "   ❌ Cannot access metrics endpoint"
        kill $PF_PID 2>/dev/null || true
        exit 1
    fi
fi

echo ""
echo "2. Checking for admission metrics..."

METRICS_FILE="/tmp/metrics_output.txt"
ADMISSION_METRICS=(
    "admission_latency_ms"
    "admission_denied_count"
    "admission_allowed_count"
    "cel_eval_ms"
    "event_publish_fail_count"
    "event_publish_success_count"
    "admission_errors_total"
)

ALL_FOUND=true
for metric in "${ADMISSION_METRICS[@]}"; do
    if grep -q "^${metric}" "${METRICS_FILE}" || grep -q "^# HELP ${metric}" "${METRICS_FILE}"; then
        echo "   ✅ Found: ${metric}"
    else
        echo "   ❌ Missing: ${metric}"
        ALL_FOUND=false
    fi
done

echo ""
echo "3. Sample metrics values:"
echo "   ---"
grep -E "^admission_|^cel_eval_|^event_publish_" "${METRICS_FILE}" | head -20
echo "   ---"

echo ""
if [ "$ALL_FOUND" = true ]; then
    echo "✅ All admission metrics are exposed!"
    echo ""
    echo "4. Metrics summary:"
    echo "   - Admission Latency: $(grep -c 'admission_latency_ms' "${METRICS_FILE}" || echo 0) entries"
    echo "   - CEL Evaluation: $(grep -c 'cel_eval_ms' "${METRICS_FILE}" || echo 0) entries"
    echo "   - Event Publish: $(grep -c 'event_publish' "${METRICS_FILE}" || echo 0) entries"
    echo ""
    echo "✅ Test PASSED"
else
    echo "❌ Some metrics are missing"
    echo ""
    echo "⚠️  Check if:"
    echo "   1. Admission webhook is initialized"
    echo "   2. Metrics are registered"
    echo "   3. Webhook handler is being called"
    exit 1
fi

# Cleanup
if [ ! -z "$PF_PID" ]; then
    kill $PF_PID 2>/dev/null || true
fi

echo ""
echo "=========================================="
echo "Test Complete"
echo "=========================================="

