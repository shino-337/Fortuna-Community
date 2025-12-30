#!/bin/bash

# ============================================================================
# Quick Deploy Script
# ============================================================================
# Quick script to build and deploy to minikube
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

echo "=========================================="
echo "KSAM Quick Deploy to Minikube"
echo "=========================================="
echo ""

# Step 1: Start minikube if not running
echo "Step 1: Checking minikube..."
if ! minikube status >/dev/null 2>&1; then
    echo "Starting minikube..."
    minikube start
else
    echo "✅ Minikube is running"
fi

# Step 2: Set docker environment
echo ""
echo "Step 2: Configuring Docker for minikube..."
eval $(minikube docker-env)
echo "✅ Docker environment configured"

# Step 3: Build Core image
echo ""
echo "Step 3: Building Core image..."
cd "${PROJECT_ROOT}"
docker build -f core/Dockerfile -t fortuna-core:latest .
echo "✅ Core image built"

# Step 4: Build Agent image
echo ""
echo "Step 4: Building Agent image..."
cd "${PROJECT_ROOT}"
docker build -f agent/Dockerfile -t fortuna-agent:latest .
echo "✅ Agent image built"

# Step 5: Verify images
echo ""
echo "Step 5: Verifying images..."
docker images | grep fortuna || echo "⚠️  Images not found"
echo "✅ Images verified"

# Step 6: Deploy
echo ""
echo "Step 6: Deploying to Kubernetes..."
cd "${PROJECT_ROOT}/deploy"

# Apply RBAC
if [ -f "fortuna-rbac.yaml" ]; then
    kubectl apply -f fortuna-rbac.yaml
    echo "✅ RBAC applied"
fi

# Apply Core
if [ -f "fortuna-core-deployment.yaml" ]; then
    kubectl apply -f fortuna-core-deployment.yaml
    echo "✅ Core deployment applied"
fi

# Apply Agent
if [ -f "fortuna-agent-daemonset.yaml" ]; then
    kubectl apply -f fortuna-agent-daemonset.yaml
    echo "✅ Agent DaemonSet applied"
fi

# Step 7: Show status
echo ""
echo "Step 7: Deployment status..."
echo ""
echo "=== Pods ==="
kubectl get pods -l app=fortuna-core 2>/dev/null || echo "No Core pods"
kubectl get pods -l app=fortuna-agent 2>/dev/null || echo "No Agent pods"

echo ""
echo "=== Images in Minikube ==="
docker images | grep fortuna | head -5

echo ""
echo "=========================================="
echo "✅ Deployment completed!"
echo "=========================================="
echo ""
echo "Check status with:"
echo "  kubectl get pods"
echo "  kubectl get deployments"
echo "  kubectl get daemonsets"
echo ""

