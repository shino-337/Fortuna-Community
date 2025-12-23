#!/bin/bash
# test-webhook-deployment.sh
# Test admission webhook deployment

set -e

NAMESPACE="ksam"

echo "=========================================="
echo "Testing Admission Webhook Deployment"
echo "=========================================="
echo ""

# Test 1: Check pods
echo "Test 1: Checking pods..."
kubectl get pods -n ${NAMESPACE} | grep core
echo ""

# Test 2: Check webhook logs
echo "Test 2: Checking webhook logs..."
kubectl logs -n ${NAMESPACE} -l app=ksam-core 2>&1 | grep -E "Webhook|HTTPS server|8443" | head -10
echo ""

# Test 3: Check service
echo "Test 3: Checking webhook service..."
kubectl get svc -n ${NAMESPACE} ksam-webhook
echo ""

# Test 4: Check endpoints
echo "Test 4: Checking webhook endpoints..."
kubectl get endpoints -n ${NAMESPACE} ksam-webhook
echo ""

# Test 5: Check certificate mount
echo "Test 5: Checking certificate mount..."
POD_NAME=$(kubectl get pod -n ${NAMESPACE} -l app=ksam-core -o jsonpath='{.items[0].metadata.name}')
if [ -n "$POD_NAME" ]; then
    kubectl exec -n ${NAMESPACE} ${POD_NAME} -- ls -la /etc/webhook/certs/ 2>&1 || echo "Certificate mount check failed"
else
    echo "Pod not found"
fi
echo ""

# Test 6: Check webhook configuration
echo "Test 6: Checking webhook configuration..."
kubectl get validatingwebhookconfiguration ksam-policy-webhook -o yaml | grep -A 2 "caBundle" | head -5
echo ""

# Test 7: Check namespace label
echo "Test 7: Checking namespace label..."
kubectl get namespace default -o jsonpath='{.metadata.labels.ksam\.io/policy-enabled}' || echo "Label not set"
echo ""

echo "=========================================="
echo "Testing complete"
echo "=========================================="

