#!/bin/bash

# Script to rebuild, clear old images, and deploy core and agent
# Usage: ./scripts/rebuild_and_deploy.sh [namespace]

set -e

NAMESPACE="${1:-ksam}"

echo "🚀 Rebuilding and Deploying Core and Agent"
echo "Namespace: $NAMESPACE"
echo ""

# Step 1: Stop deployments
echo "Step 1: Stopping deployments..."
kubectl scale deployment ksam-core -n $NAMESPACE --replicas=0 2>/dev/null || echo "Core deployment not found or already scaled"
kubectl scale daemonset ksam-agent -n $NAMESPACE --replicas=0 2>/dev/null || echo "Agent daemonset not found or already scaled"

# Wait for pods to terminate
echo "Waiting for pods to terminate..."
sleep 5

# Step 2: Delete old images from minikube
echo ""
echo "Step 2: Clearing old images from minikube..."
minikube ssh -- docker images | grep ksam | awk '{print $3}' | xargs -r minikube ssh -- docker rmi -f 2>/dev/null || echo "No old images to remove"
minikube ssh -- docker system prune -f || true

# Step 3: Clear Docker build cache
echo ""
echo "Step 3: Clearing Docker build cache..."
docker system prune -f || true

# Step 4: Rebuild Core
echo ""
echo "Step 4: Rebuilding Core image..."
cd "$(dirname "$0")/../core"
docker build --no-cache -t ksam-core:latest .
echo "✅ Core image built successfully"

# Step 5: Load Core image to minikube
echo ""
echo "Step 5: Loading Core image to minikube..."
minikube image load ksam-core:latest
echo "✅ Core image loaded to minikube"

# Step 6: Rebuild Agent
echo ""
echo "Step 6: Rebuilding Agent image..."
cd "$(dirname "$0")/../agent"
docker build --no-cache -t ksam-agent:latest .
echo "✅ Agent image built successfully"

# Step 7: Load Agent image to minikube
echo ""
echo "Step 7: Loading Agent image to minikube..."
minikube image load ksam-agent:latest
echo "✅ Agent image loaded to minikube"

# Step 8: Delete old deployments (if they exist)
echo ""
echo "Step 8: Deleting old deployments..."
kubectl delete deployment ksam-core -n $NAMESPACE --ignore-not-found=true
kubectl delete daemonset ksam-agent -n $NAMESPACE --ignore-not-found=true
sleep 3

# Step 9: Apply new deployments
echo ""
echo "Step 9: Applying new deployments..."
cd "$(dirname "$0")/.."

# Check if helm chart exists
if [ -d "helm/ksam" ]; then
    echo "Using Helm chart..."
    helm upgrade --install ksam ./helm/ksam -n $NAMESPACE --create-namespace --set imagePullPolicy=Always
else
    echo "Helm chart not found, checking for deployment files..."
    if [ -f "agent/deploy/daemonset.yaml" ] && [ -f "core/deploy/deployment.yaml" ]; then
        kubectl apply -f core/deploy/deployment.yaml -n $NAMESPACE
        kubectl apply -f agent/deploy/daemonset.yaml -n $NAMESPACE
    else
        echo "⚠️  Deployment files not found. Please apply deployments manually."
    fi
fi

# Step 10: Wait for deployments
echo ""
echo "Step 10: Waiting for deployments to be ready..."
kubectl wait --for=condition=available deployment/ksam-core -n $NAMESPACE --timeout=120s || echo "Core deployment not ready"
kubectl wait --for=condition=ready pod -n $NAMESPACE -l app=ksam-agent --timeout=120s || echo "Agent pods not ready"

# Step 11: Verify
echo ""
echo "Step 11: Verifying deployments..."
kubectl get pods -n $NAMESPACE
kubectl get deployment ksam-core -n $NAMESPACE -o jsonpath='{.spec.template.spec.containers[0].image}'
echo ""
kubectl get daemonset ksam-agent -n $NAMESPACE -o jsonpath='{.spec.template.spec.containers[0].image}'
echo ""

echo "✅ Rebuild and deployment completed!"


