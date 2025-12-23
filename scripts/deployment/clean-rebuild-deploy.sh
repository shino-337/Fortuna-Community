#!/bin/bash
# clean-rebuild-deploy.sh
# Complete cleanup and rebuild process

set -e

NAMESPACE="ksam"
DEPLOYMENT="ksam-core"
IMAGE_NAME="ksam/core"
IMAGE_TAG="latest"

echo "=========================================="
echo "Complete Clean Rebuild & Deploy"
echo "=========================================="
echo ""

# Step 1: Stop and delete deployment
echo "Step 1: Stopping and deleting deployment..."
kubectl delete deployment ${DEPLOYMENT} -n ${NAMESPACE} 2>/dev/null || echo "  Deployment not found (OK)"
kubectl wait --for=delete deployment/${DEPLOYMENT} -n ${NAMESPACE} --timeout=60s 2>/dev/null || echo "  Deployment deleted"
echo "✅ Deployment deleted"
echo ""

# Step 2: Delete old pods (if any)
echo "Step 2: Deleting old pods..."
kubectl delete pods -n ${NAMESPACE} -l app=${DEPLOYMENT} 2>/dev/null || echo "  No pods to delete"
sleep 5
echo "✅ Old pods cleaned"
echo ""

# Step 3: Remove old Docker images
echo "Step 3: Removing old Docker images..."
docker images | grep "${IMAGE_NAME}" | awk '{print $3}' | xargs -r docker rmi -f 2>/dev/null || echo "  No old images to remove"
echo "✅ Old images removed"
echo ""

# Step 4: Clear Docker build cache
echo "Step 4: Clearing Docker build cache..."
docker builder prune -f
echo "✅ Build cache cleared"
echo ""

# Step 5: Rebuild image with no cache
echo "Step 5: Rebuilding Docker image (no cache)..."
cd KSAM/core
docker build --no-cache --pull -t ${IMAGE_NAME}:${IMAGE_TAG} . || {
    echo "❌ Build failed"
    exit 1
}
echo "✅ Image rebuilt: ${IMAGE_NAME}:${IMAGE_TAG}"
echo ""

# Step 6: Verify image
echo "Step 6: Verifying image..."
docker images | grep "${IMAGE_NAME}" | head -3
echo ""

# Step 7: Apply deployment
echo "Step 7: Applying deployment..."
cd ../..
kubectl apply -f KSAM/deploy/core-deployment.yaml
echo "✅ Deployment applied"
echo ""

# Step 8: Wait for deployment
echo "Step 8: Waiting for deployment to be ready..."
sleep 10
kubectl wait --for=condition=available deployment/${DEPLOYMENT} -n ${NAMESPACE} --timeout=120s || {
    echo "⚠️  Deployment not ready yet, checking status..."
    kubectl get pods -n ${NAMESPACE} -l app=${DEPLOYMENT}
}
echo ""

# Step 9: Check pod status
echo "Step 9: Checking pod status..."
kubectl get pods -n ${NAMESPACE} -l app=${DEPLOYMENT}
echo ""

# Step 10: Check image and pull policy
echo "Step 10: Verifying pod configuration..."
POD_IMAGE=$(kubectl get pod -n ${NAMESPACE} -l app=${DEPLOYMENT} -o jsonpath='{.items[0].spec.containers[0].image}' 2>/dev/null || echo "N/A")
POD_POLICY=$(kubectl get pod -n ${NAMESPACE} -l app=${DEPLOYMENT} -o jsonpath='{.items[0].spec.containers[0].imagePullPolicy}' 2>/dev/null || echo "N/A")
POD_AGE=$(kubectl get pod -n ${NAMESPACE} -l app=${DEPLOYMENT} -o jsonpath='{.items[0].metadata.creationTimestamp}' 2>/dev/null || echo "N/A")
echo "  Pod Image: $POD_IMAGE"
echo "  Image Pull Policy: $POD_POLICY"
echo "  Pod Created: $POD_AGE"
echo ""

# Step 11: Wait for logs
echo "Step 11: Waiting for pod to start..."
sleep 15
echo ""

# Step 12: Check logs
echo "Step 12: Checking webhook logs..."
kubectl logs -n ${NAMESPACE} -l app=${DEPLOYMENT} --tail=200 2>&1 | grep -E "Main\]|Webhook\]|Phase 2.7|About to add|Policy Evaluator|Admission webhook|HTTPS server" | tail -30 || {
    echo "  No webhook logs found yet, showing all logs:"
    kubectl logs -n ${NAMESPACE} -l app=${DEPLOYMENT} --tail=50
}
echo ""

echo "=========================================="
echo "Clean Rebuild & Deploy Complete"
echo "=========================================="
echo ""
echo "Next steps:"
echo "  1. Check pod status: kubectl get pods -n ksam -l app=ksam-core"
echo "  2. Check all logs: kubectl logs -n ksam -l app=ksam-core --tail=500"
echo "  3. Check webhook logs: kubectl logs -n ksam -l app=ksam-core | grep Webhook"
echo ""



