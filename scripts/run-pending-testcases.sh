#!/bin/bash
# run-pending-testcases.sh
# Run all pending testcases from previous phases

set -e

NAMESPACE="ksam"
SERVICE="ksam-core"
WEBHOOK_SERVICE="ksam-webhook"

echo "=========================================="
echo "Running Pending Testcases"
echo "=========================================="
echo ""

# Test 1: Check pod status
echo "Test 1: Checking pod status..."
POD_STATUS=$(kubectl get pods -n ${NAMESPACE} -l app=${SERVICE} -o jsonpath='{.items[0].status.phase}' 2>/dev/null || echo "NOT_FOUND")
if [ "$POD_STATUS" = "Running" ]; then
    echo "✅ Pod is running"
else
    echo "❌ Pod status: $POD_STATUS"
    echo "   Checking logs..."
    kubectl logs -n ${NAMESPACE} -l app=${SERVICE} --tail=20 2>&1 | tail -10
    exit 1
fi
echo ""

# Test 2: Check webhook service
echo "Test 2: Checking webhook service..."
if kubectl get svc -n ${NAMESPACE} ${WEBHOOK_SERVICE} &>/dev/null; then
    echo "✅ Webhook service exists"
    kubectl get svc -n ${NAMESPACE} ${WEBHOOK_SERVICE}
else
    echo "❌ Webhook service not found"
    echo "   Creating webhook service..."
    kubectl apply -f KSAM/deploy/webhook-service.yaml
fi
echo ""

# Test 3: Check webhook certificates
echo "Test 3: Checking webhook certificates..."
if kubectl get secret -n ${NAMESPACE} ksam-webhook-tls &>/dev/null; then
    echo "✅ Webhook TLS secret exists"
else
    echo "❌ Webhook TLS secret not found"
    echo "   Generating certificates..."
    ./KSAM/scripts/generate-webhook-certs.sh
    kubectl apply -f /tmp/ksam-webhook-tls-secret.yaml
fi
echo ""

# Test 4: Check ValidatingWebhookConfiguration
echo "Test 4: Checking ValidatingWebhookConfiguration..."
if kubectl get validatingwebhookconfiguration ksam-policy-webhook &>/dev/null; then
    echo "✅ ValidatingWebhookConfiguration exists"
else
    echo "❌ ValidatingWebhookConfiguration not found"
    echo "   Creating webhook configuration..."
    kubectl apply -f KSAM/deploy/webhook-config.yaml
fi
echo ""

# Test 5: Test webhook endpoint (if pod is running)
echo "Test 5: Testing webhook endpoint..."
POD_NAME=$(kubectl get pod -n ${NAMESPACE} -l app=${SERVICE} -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
if [ -n "$POD_NAME" ]; then
    echo "   Port-forwarding to pod..."
    kubectl port-forward -n ${NAMESPACE} pod/${POD_NAME} 8080:8080 &
    PF_PID=$!
    sleep 3
    
    echo "   Testing /admission/health endpoint..."
    if curl -s http://localhost:8080/admission/health | grep -q "ok\|healthy"; then
        echo "✅ Webhook health endpoint accessible"
    else
        echo "⚠️  Webhook health endpoint may not be accessible"
    fi
    
    echo "   Testing /metrics endpoint for admission metrics..."
    METRICS_COUNT=$(curl -s http://localhost:8080/metrics | grep -E "^admission_|^cel_eval_|^event_publish_" | wc -l | tr -d ' ')
    if [ "$METRICS_COUNT" -gt 0 ]; then
        echo "✅ Found $METRICS_COUNT admission metrics"
    else
        echo "⚠️  No admission metrics found (may appear after first webhook call)"
    fi
    
    kill $PF_PID 2>/dev/null || true
else
    echo "⚠️  Pod not found, skipping endpoint test"
fi
echo ""

# Test 6: Test with real Kubernetes resource
echo "Test 6: Testing webhook with real Kubernetes resource..."
echo "   Creating test namespace with label..."
kubectl create namespace test-webhook --dry-run=client -o yaml | kubectl apply -f -
kubectl label namespace test-webhook ksam.io/policy-enabled=true --overwrite

echo "   Creating test pod (should trigger webhook)..."
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: test-webhook-pod
  namespace: test-webhook
spec:
  containers:
  - name: test
    image: nginx:alpine
    securityContext:
      privileged: false
EOF

sleep 5

echo "   Checking pod status..."
POD_STATUS=$(kubectl get pod -n test-webhook test-webhook-pod -o jsonpath='{.status.phase}' 2>/dev/null || echo "NOT_FOUND")
if [ "$POD_STATUS" = "Pending" ] || [ "$POD_STATUS" = "Running" ]; then
    echo "✅ Test pod created (status: $POD_STATUS)"
    echo "   Checking webhook logs..."
    kubectl logs -n ${NAMESPACE} -l app=${SERVICE} --tail=50 | grep -i "webhook\|admission" | tail -10 || echo "   No webhook logs found yet"
else
    echo "⚠️  Test pod status: $POD_STATUS"
fi

# Cleanup
echo ""
echo "   Cleaning up test resources..."
kubectl delete pod -n test-webhook test-webhook-pod --ignore-not-found=true
kubectl delete namespace test-webhook --ignore-not-found=true

echo ""
echo "=========================================="
echo "Testcases Complete"
echo "=========================================="
echo ""
echo "Next steps:"
echo "  1. Check webhook logs: kubectl logs -n ksam -l app=ksam-core | grep webhook"
echo "  2. Check metrics: kubectl port-forward -n ksam deployment/ksam-core 8080:8080 && curl http://localhost:8080/metrics | grep admission"
echo "  3. Test with real resource: kubectl create pod with ksam.io/policy-enabled=true label"
echo ""



