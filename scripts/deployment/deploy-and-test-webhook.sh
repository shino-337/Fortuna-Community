#!/bin/bash
# deploy-and-test-webhook.sh
# Complete deployment and testing script for admission webhook

set -e

NAMESPACE="ksam"

echo "=========================================="
echo "Admission Webhook - Deployment and Testing"
echo "=========================================="
echo ""

# Step 1: Apply secret
echo "Step 1: Applying webhook TLS secret..."
kubectl apply -f /tmp/ksam-webhook-tls-secret.yaml
echo "✅ Secret applied"
echo ""

# Step 2: Apply deployment
echo "Step 2: Applying core deployment..."
kubectl apply -f KSAM/deploy/core-deployment.yaml
echo "✅ Deployment applied"
echo ""

# Step 3: Apply webhook service
echo "Step 3: Applying webhook service..."
kubectl apply -f KSAM/deploy/webhook-service.yaml
echo "✅ Service applied"
echo ""

# Step 4: Apply webhook configuration
echo "Step 4: Applying webhook configuration..."
kubectl apply -f KSAM/deploy/webhook-config.yaml
echo "✅ Webhook configuration applied"
echo ""

# Step 5: Wait for pods
echo "Step 5: Waiting for pods to be ready..."
sleep 20
kubectl get pods -n ${NAMESPACE} -l app=ksam-core
echo ""

# Step 6: Check webhook logs
echo "Step 6: Checking webhook server logs..."
kubectl logs -n ${NAMESPACE} -l app=ksam-core --tail=100 | grep -E "Webhook|HTTPS server|8443|Certificate:" | head -20 || echo "No webhook logs found yet"
echo ""

# Step 7: Check service
echo "Step 7: Checking webhook service..."
kubectl get svc -n ${NAMESPACE} ksam-webhook
echo ""

# Step 8: Check endpoints
echo "Step 8: Checking webhook endpoints..."
kubectl get endpoints -n ${NAMESPACE} ksam-webhook
echo ""

# Step 9: Check certificate mount
echo "Step 9: Checking certificate mount..."
POD_NAME=$(kubectl get pod -n ${NAMESPACE} -l app=ksam-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
if [ -n "$POD_NAME" ]; then
    echo "Pod: $POD_NAME"
    kubectl exec -n ${NAMESPACE} ${POD_NAME} -- ls -la /etc/webhook/certs/ 2>&1 || echo "Certificate mount check failed"
else
    echo "Pod not found"
fi
echo ""

# Step 10: Check webhook configuration
echo "Step 10: Checking webhook configuration..."
CA_BUNDLE_LENGTH=$(kubectl get validatingwebhookconfiguration ksam-policy-webhook -o jsonpath='{.webhooks[0].clientConfig.caBundle}' 2>/dev/null | wc -c)
if [ "$CA_BUNDLE_LENGTH" -gt 100 ]; then
    echo "✅ CA bundle configured (length: $CA_BUNDLE_LENGTH)"
else
    echo "⚠️  CA bundle may be missing or too short"
fi
echo ""

# Step 11: Label namespace
echo "Step 11: Labeling default namespace..."
kubectl label namespace default ksam.io/policy-enabled=true --overwrite
echo "✅ Namespace labeled"
echo ""

# Step 12: Create test pod
echo "Step 12: Creating test pod..."
kubectl apply -f - <<'EOF'
apiVersion: v1
kind: Pod
metadata:
  name: test-pod-webhook
  namespace: default
spec:
  containers:
    - name: nginx
      image: nginx:latest
      securityContext:
        runAsNonRoot: false
EOF
echo "✅ Test pod created"
echo ""

# Step 13: Wait and check pod status
echo "Step 13: Checking test pod status..."
sleep 5
kubectl get pod test-pod-webhook -n default
echo ""

# Step 14: Check webhook logs for AdmissionReview
echo "Step 14: Checking webhook logs for AdmissionReview..."
kubectl logs -n ${NAMESPACE} -l app=ksam-core --tail=100 | grep -E "AdmissionReview|admission/validate|Policy.*violation|Handle.*request" | tail -20 || echo "No AdmissionReview logs found"
echo ""

# Step 15: Check pod events
echo "Step 15: Checking test pod events..."
kubectl describe pod test-pod-webhook -n default 2>&1 | grep -E "Events:|Warning:|Error:" | head -20 || echo "No events found"
echo ""

echo "=========================================="
echo "Deployment and Testing Complete"
echo "=========================================="
echo ""
echo "Next steps:"
echo "1. Check webhook logs: kubectl logs -n ksam -l app=ksam-core | grep Webhook"
echo "2. Test health endpoint: kubectl port-forward -n ksam deployment/ksam-core 8443:8443"
echo "3. Then in another terminal: curl -k https://localhost:8443/admission/health"
echo ""

