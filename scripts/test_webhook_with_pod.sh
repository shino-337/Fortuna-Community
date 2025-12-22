#!/bin/bash

# Test Admission Webhook with Real Kubernetes Pod
# This script creates a test pod to trigger the admission webhook

set -e

echo "=========================================="
echo "Testing Admission Webhook with Real Pod"
echo "=========================================="

echo ""
echo "1. Creating test pod that should trigger policy violation..."

# Create a pod that violates policy (privileged container)
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: test-violating-pod
  namespace: default
spec:
  containers:
  - name: test-container
    image: nginx:alpine
    securityContext:
      privileged: true
      runAsNonRoot: false
EOF

echo "   ✅ Test pod created"

echo ""
echo "2. Checking pod status..."

sleep 3

POD_STATUS=$(kubectl get pod test-violating-pod -n default -o jsonpath='{.status.phase}' 2>/dev/null || echo "NOT_FOUND")

if [ "$POD_STATUS" = "NOT_FOUND" ]; then
    echo "   ⚠️  Pod not found (may have been blocked by webhook)"
elif [ "$POD_STATUS" = "Pending" ]; then
    echo "   ⚠️  Pod is Pending (may be blocked by webhook)"
    kubectl describe pod test-violating-pod -n default | grep -A 5 "Events:" | head -10
elif [ "$POD_STATUS" = "Running" ]; then
    echo "   ✅ Pod is Running (webhook allowed it)"
else
    echo "   Status: $POD_STATUS"
fi

echo ""
echo "3. Checking webhook logs..."

kubectl logs -n ksam -l app=ksam-core --tail=50 2>&1 | grep -E "Webhook|Admission|BLOCKED|ALLOWED" | tail -10

echo ""
echo "4. Checking metrics..."

METRICS=$(kubectl exec -n ksam $(kubectl get pods -n ksam -l app=ksam-core -o jsonpath='{.items[0].metadata.name}') -- \
    wget -q -O- http://localhost:8080/metrics 2>&1)

echo "   Admission metrics:"
for metric in admission_latency_ms admission_denied_count admission_allowed_count cel_eval_ms; do
    VALUE=$(echo "$METRICS" | grep "^${metric}" | head -1)
    if [ ! -z "$VALUE" ]; then
        echo "   ✅ ${metric}: $VALUE"
    else
        HELP=$(echo "$METRICS" | grep "^# HELP ${metric}")
        if [ ! -z "$HELP" ]; then
            echo "   ✅ ${metric}: (registered, no value yet)"
        else
            echo "   ⚠️  ${metric}: (not found)"
        fi
    fi
done

echo ""
echo "5. Cleaning up test pod..."

kubectl delete pod test-violating-pod -n default --ignore-not-found=true 2>&1 | head -1

echo ""
echo "=========================================="
echo "Test Complete"
echo "=========================================="

