#!/bin/bash
# fix-deployment-old-image.sh
# Fix deployment using old cached image

set -e

NAMESPACE="ksam"
DEPLOYMENT="ksam-core"

echo "=========================================="
echo "Fixing Deployment - Old Image Issue"
echo "=========================================="
echo ""

# Step 1: Rebuild image
echo "Step 1: Rebuilding Docker image..."
cd KSAM/core
docker build --no-cache -t ksam/core:latest . || {
    echo "❌ Build failed"
    exit 1
}
echo "✅ Image rebuilt"
echo ""

# Step 2: Delete old deployment
echo "Step 2: Deleting old deployment..."
kubectl delete deployment ${DEPLOYMENT} -n ${NAMESPACE} || {
    echo "⚠️  Deployment not found or already deleted"
}
echo "✅ Old deployment deleted"
echo ""

# Step 3: Wait a bit
echo "Step 3: Waiting for cleanup..."
sleep 5
echo ""

# Step 4: Apply new deployment
echo "Step 4: Applying new deployment..."
cd ../..
kubectl apply -f KSAM/deploy/core-deployment.yaml
echo "✅ New deployment applied"
echo ""

# Step 5: Wait for pod
echo "Step 5: Waiting for pod to be ready..."
sleep 20
kubectl get pods -n ${NAMESPACE} -l app=${DEPLOYMENT}
echo ""

# Step 6: Check image
echo "Step 6: Checking pod image..."
POD_IMAGE=$(kubectl get pod -n ${NAMESPACE} -l app=${DEPLOYMENT} -o jsonpath='{.items[0].spec.containers[0].image}' 2>/dev/null)
POD_POLICY=$(kubectl get pod -n ${NAMESPACE} -l app=${DEPLOYMENT} -o jsonpath='{.items[0].spec.containers[0].imagePullPolicy}' 2>/dev/null)
echo "Pod Image: $POD_IMAGE"
echo "Image Pull Policy: $POD_POLICY"
echo ""

# Step 7: Check logs
echo "Step 7: Checking webhook logs..."
sleep 5
kubectl logs -n ${NAMESPACE} -l app=${DEPLOYMENT} --tail=100 | grep -E "Main\]|Webhook\]|Phase 2.7" | tail -20 || echo "No logs found yet"
echo ""

echo "=========================================="
echo "Deployment fix complete"
echo "=========================================="
echo ""
echo "Next: Check logs for webhook startup:"
echo "  kubectl logs -n ksam -l app=ksam-core | grep Webhook"
echo ""

