#!/bin/bash
# test-webhook-complete.sh
# Complete webhook testing script

set -e

NAMESPACE="ksam"
SERVICE="ksam-core"
WEBHOOK_SERVICE="ksam-webhook"

echo "=========================================="
echo "Complete Webhook Testing"
echo "=========================================="
echo ""

# Step 1: Verify service is running
echo "Step 1: Verifying service status..."
POD_NAME=$(kubectl get pod -n ${NAMESPACE} -l app=${SERVICE} -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -z "$POD_NAME" ]; then
    echo "❌ Pod not found"
    exit 1
fi

POD_STATUS=$(kubectl get pod -n ${NAMESPACE} ${POD_NAME} -o jsonpath='{.status.phase}')
if [ "$POD_STATUS" != "Running" ]; then
    echo "⚠️  Pod status: $POD_STATUS"
    echo "   Checking logs..."
    kubectl logs -n ${NAMESPACE} ${POD_NAME} --tail=30
    echo ""
    echo "   Waiting for pod to be ready..."
    kubectl wait --for=condition=ready pod/${POD_NAME} -n ${NAMESPACE} --timeout=60s || {
        echo "❌ Pod not ready after 60s"
        exit 1
    }
fi
echo "✅ Pod is running: $POD_NAME"
echo ""

# Step 2: Port-forward and test endpoints
echo "Step 2: Testing webhook endpoints..."
kubectl port-forward -n ${NAMESPACE} pod/${POD_NAME} 8080:8080 > /dev/null 2>&1 &
PF_PID=$!
sleep 3

# Test health endpoint
echo "   Testing /admission/health..."
HEALTH_RESPONSE=$(curl -s http://localhost:8080/admission/health || echo "ERROR")
if echo "$HEALTH_RESPONSE" | grep -qE "ok|healthy|OK"; then
    echo "✅ Health endpoint: OK"
else
    echo "⚠️  Health endpoint response: $HEALTH_RESPONSE"
fi

# Test metrics endpoint
echo "   Testing /metrics for admission metrics..."
METRICS=$(curl -s http://localhost:8080/metrics)
ADMISSION_METRICS=$(echo "$METRICS" | grep -E "^admission_|^cel_eval_|^event_publish_" | wc -l | tr -d ' ')
if [ "$ADMISSION_METRICS" -gt 0 ]; then
    echo "✅ Found $ADMISSION_METRICS admission metrics"
    echo "$METRICS" | grep -E "^admission_|^cel_eval_|^event_publish_" | head -10
else
    echo "⚠️  No admission metrics found (will appear after first webhook call)"
fi

kill $PF_PID 2>/dev/null || true
echo ""

# Step 3: Verify webhook service
echo "Step 3: Verifying webhook service..."
if kubectl get svc -n ${NAMESPACE} ${WEBHOOK_SERVICE} &>/dev/null; then
    echo "✅ Webhook service exists"
    kubectl get svc -n ${NAMESPACE} ${WEBHOOK_SERVICE} -o wide
else
    echo "❌ Webhook service not found"
fi
echo ""

# Step 4: Verify webhook configuration
echo "Step 4: Verifying ValidatingWebhookConfiguration..."
if kubectl get validatingwebhookconfiguration ksam-policy-webhook &>/dev/null; then
    echo "✅ ValidatingWebhookConfiguration exists"
    WEBHOOK_COUNT=$(kubectl get validatingwebhookconfiguration ksam-policy-webhook -o jsonpath='{.webhooks[*].name}' | wc -w)
    echo "   Webhooks configured: $WEBHOOK_COUNT"
else
    echo "❌ ValidatingWebhookConfiguration not found"
fi
echo ""

# Step 5: Test with real Kubernetes resource
echo "Step 5: Testing with real Kubernetes resource..."
echo "   Creating test namespace..."
kubectl create namespace test-webhook-complete --dry-run=client -o yaml | kubectl apply -f - > /dev/null
kubectl label namespace test-webhook-complete ksam.io/policy-enabled=true --overwrite > /dev/null 2>&1

echo "   Creating test pod..."
cat <<EOF | kubectl apply -f - > /dev/null
apiVersion: v1
kind: Pod
metadata:
  name: test-webhook-pod-complete
  namespace: test-webhook-complete
spec:
  containers:
  - name: test
    image: nginx:alpine
EOF

sleep 5

POD_STATUS=$(kubectl get pod -n test-webhook-complete test-webhook-pod-complete -o jsonpath='{.status.phase}' 2>/dev/null || echo "NOT_FOUND")
echo "   Test pod status: $POD_STATUS"

if [ "$POD_STATUS" = "Pending" ]; then
    echo "   Checking events..."
    kubectl get events -n test-webhook-complete --sort-by='.lastTimestamp' | grep test-webhook-pod-complete | tail -5
fi

echo "   Checking webhook logs..."
kubectl logs -n ${NAMESPACE} ${POD_NAME} --tail=100 | grep -iE "webhook|admission|validate" | tail -10 || echo "   No webhook logs found"

# Cleanup
echo ""
echo "   Cleaning up..."
kubectl delete pod -n test-webhook-complete test-webhook-pod-complete --ignore-not-found=true > /dev/null
kubectl delete namespace test-webhook-complete --ignore-not-found=true > /dev/null

echo ""
echo "=========================================="
echo "Testing Complete"
echo "=========================================="
echo ""
echo "Summary:"
echo "  - Pod status: $POD_STATUS"
echo "  - Admission metrics: $ADMISSION_METRICS found"
echo "  - Webhook service: $(kubectl get svc -n ${NAMESPACE} ${WEBHOOK_SERVICE} &>/dev/null && echo 'OK' || echo 'NOT FOUND')"
echo "  - Webhook config: $(kubectl get validatingwebhookconfiguration ksam-policy-webhook &>/dev/null && echo 'OK' || echo 'NOT FOUND')"
echo ""



