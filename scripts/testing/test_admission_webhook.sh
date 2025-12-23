#!/bin/bash

# Test Admission Webhook Endpoint
# This script tests the admission webhook by sending a test admission request

set -e

echo "=========================================="
echo "Testing Admission Webhook"
echo "=========================================="

CORE_SERVICE="${CORE_SERVICE:-ksam-core.ksam.svc.cluster.local}"
CORE_PORT="${CORE_PORT:-8080}"
WEBHOOK_URL="http://${CORE_SERVICE}:${CORE_PORT}/admission/validate"

echo ""
echo "1. Checking webhook health endpoint..."

# Try to access health endpoint
if kubectl exec -n ksam $(kubectl get pods -n ksam -l app=ksam-core -o jsonpath='{.items[0].metadata.name}') -- wget -q -O- http://localhost:8080/admission/health 2>&1 | grep -q "OK"; then
    echo "   ✅ Webhook health endpoint is accessible"
else
    echo "   ⚠️  Cannot access health endpoint directly, trying port-forward..."
    
    # Try port-forward
    kubectl port-forward -n ksam deployment/ksam-core 8080:8080 > /tmp/pf.log 2>&1 &
    PF_PID=$!
    sleep 2
    
    if curl -s http://localhost:8080/admission/health | grep -q "OK"; then
        echo "   ✅ Webhook health endpoint accessible via port-forward"
        WEBHOOK_URL="http://localhost:8080/admission/validate"
    else
        echo "   ❌ Cannot access webhook health endpoint"
        kill $PF_PID 2>/dev/null || true
        exit 1
    fi
fi

echo ""
echo "2. Creating test admission request..."

# Create a test admission review request
cat > /tmp/test-admission-request.json <<'EOF'
{
  "kind": "AdmissionReview",
  "apiVersion": "admission.k8s.io/v1",
  "request": {
    "uid": "test-request-123",
    "kind": {
      "group": "",
      "version": "v1",
      "kind": "Pod"
    },
    "resource": {
      "group": "",
      "version": "v1",
      "resource": "pods"
    },
    "namespace": "default",
    "operation": "CREATE",
    "object": {
      "apiVersion": "v1",
      "kind": "Pod",
      "metadata": {
        "name": "test-pod",
        "namespace": "default",
        "uid": "test-pod-uid-123"
      },
      "spec": {
        "containers": [
          {
            "name": "test-container",
            "image": "nginx:alpine",
            "securityContext": {
              "privileged": true,
              "runAsNonRoot": false
            }
          }
        ]
      }
    }
  }
}
EOF

echo "   ✅ Test admission request created"

echo ""
echo "3. Sending admission request to webhook..."

# Send request
RESPONSE=$(kubectl exec -n ksam $(kubectl get pods -n ksam -l app=ksam-core -o jsonpath='{.items[0].metadata.name}') -- \
    wget -q -O- --post-file=/tmp/test-admission-request.json \
    --header="Content-Type: application/json" \
    http://localhost:8080/admission/validate 2>&1 || echo "ERROR")

if [ "$RESPONSE" = "ERROR" ] || [ -z "$RESPONSE" ]; then
    echo "   ⚠️  Cannot send request directly, trying port-forward..."
    
    if [ -z "$PF_PID" ]; then
        kubectl port-forward -n ksam deployment/ksam-core 8080:8080 > /tmp/pf.log 2>&1 &
        PF_PID=$!
        sleep 2
    fi
    
    RESPONSE=$(curl -s -X POST http://localhost:8080/admission/validate \
        -H "Content-Type: application/json" \
        -d @/tmp/test-admission-request.json 2>&1)
fi

echo "   Response received:"
echo "   ---"
echo "$RESPONSE" | head -20
echo "   ---"

# Check if response is valid JSON
if echo "$RESPONSE" | grep -q "\"kind\".*\"AdmissionReview\""; then
    echo "   ✅ Valid admission response received"
    
    # Check if allowed
    if echo "$RESPONSE" | grep -q "\"allowed\".*true"; then
        echo "   ✅ Request was ALLOWED"
    elif echo "$RESPONSE" | grep -q "\"allowed\".*false"; then
        echo "   🚫 Request was DENIED"
        REASON=$(echo "$RESPONSE" | grep -o "\"message\":\"[^\"]*\"" | head -1)
        echo "   Reason: $REASON"
    fi
else
    echo "   ⚠️  Response may not be valid JSON"
fi

echo ""
echo "4. Checking metrics after webhook call..."

sleep 2

# Check metrics
METRICS=$(kubectl exec -n ksam $(kubectl get pods -n ksam -l app=ksam-core -o jsonpath='{.items[0].metadata.name}') -- \
    wget -q -O- http://localhost:8080/metrics 2>&1)

ADMISSION_METRICS=(
    "admission_latency_ms"
    "admission_denied_count"
    "admission_allowed_count"
    "cel_eval_ms"
    "event_publish_fail_count"
    "event_publish_success_count"
    "admission_errors_total"
)

echo "   Checking for admission metrics..."
for metric in "${ADMISSION_METRICS[@]}"; do
    if echo "$METRICS" | grep -q "^${metric}\|^# HELP ${metric}\|^# TYPE ${metric}"; then
        echo "   ✅ Found: ${metric}"
        # Show value if exists
        VALUE=$(echo "$METRICS" | grep "^${metric}" | head -1)
        if [ ! -z "$VALUE" ]; then
            echo "      Value: $VALUE"
        fi
    else
        echo "   ⚠️  Missing: ${metric} (may appear after first request)"
    fi
done

# Cleanup
if [ ! -z "$PF_PID" ]; then
    kill $PF_PID 2>/dev/null || true
fi

rm -f /tmp/test-admission-request.json

echo ""
echo "=========================================="
echo "Test Complete"
echo "=========================================="

