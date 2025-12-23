#!/bin/bash

# Quick Deployment Script for KSAM
# Usage: bash scripts/deploy.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

NAMESPACE="ksam"

echo "=========================================="
echo "KSAM Deployment Script"
echo "=========================================="
echo ""

# Step 1: Start Minikube
echo "[1/12] Starting Minikube..."
minikube start --memory=4096 --cpus=2 2>&1 | tail -5

# Step 2: Setup Docker
echo ""
echo "[2/12] Setting up Docker environment..."
eval $(minikube docker-env)
echo "✅ Docker environment set"

# Step 3: Create Namespace
echo ""
echo "[3/12] Creating namespace..."
kubectl create namespace "$NAMESPACE" 2>&1 || echo "Namespace already exists"

# Step 4: Generate Certificates (if needed)
echo ""
echo "[4/12] Checking certificates..."
if [ ! -f "$PROJECT_ROOT/certs/ca.crt" ]; then
    echo "⚠️  Certificates not found. Please generate them first:"
    echo "   bash scripts/generate_certs.sh"
    echo "   Or create manually in certs/ directory"
else
    echo "✅ Certificates found"
fi

# Step 5: Create TLS Secrets
echo ""
echo "[5/12] Creating TLS secrets..."
if [ -f "$PROJECT_ROOT/certs/ca.crt" ] && [ -f "$PROJECT_ROOT/certs/core.crt" ] && [ -f "$PROJECT_ROOT/certs/core.key" ]; then
    kubectl create secret generic ksam-ca-cert \
        --from-file=ca.crt="$PROJECT_ROOT/certs/ca.crt" \
        -n "$NAMESPACE" 2>&1 || kubectl delete secret ksam-ca-cert -n "$NAMESPACE" && \
    kubectl create secret generic ksam-ca-cert \
        --from-file=ca.crt="$PROJECT_ROOT/certs/ca.crt" \
        -n "$NAMESPACE"
    
    kubectl create secret tls ksam-core-tls \
        --cert="$PROJECT_ROOT/certs/core.crt" \
        --key="$PROJECT_ROOT/certs/core.key" \
        -n "$NAMESPACE" 2>&1 || kubectl delete secret ksam-core-tls -n "$NAMESPACE" && \
    kubectl create secret tls ksam-core-tls \
        --cert="$PROJECT_ROOT/certs/core.crt" \
        --key="$PROJECT_ROOT/certs/core.key" \
        -n "$NAMESPACE"
    
    kubectl create secret tls ksam-agent-tls \
        --cert="$PROJECT_ROOT/certs/core.crt" \
        --key="$PROJECT_ROOT/certs/core.key" \
        -n "$NAMESPACE" 2>&1 || kubectl delete secret ksam-agent-tls -n "$NAMESPACE" && \
    kubectl create secret tls ksam-agent-tls \
        --cert="$PROJECT_ROOT/certs/core.crt" \
        --key="$PROJECT_ROOT/certs/core.key" \
        -n "$NAMESPACE"
    echo "✅ TLS secrets created"
else
    echo "⚠️  Skipping TLS secrets (certificates not found)"
fi

# Step 6: Create Core Secrets
echo ""
echo "[6/12] Creating core secrets..."
kubectl apply -f "$PROJECT_ROOT/deploy/core-secrets.yaml" -n "$NAMESPACE"
echo "✅ Core secrets created"

# Step 7: Build Core Image
echo ""
echo "[7/12] Building Core image..."
cd "$PROJECT_ROOT/core"
docker build -t ksam-core:latest . 2>&1 | tail -5
echo "✅ Core image built"

# Step 8: Build Agent Image
echo ""
echo "[8/12] Building Agent image..."
cd "$PROJECT_ROOT/agent"
docker build -t ksam-agent:latest . 2>&1 | tail -5
echo "✅ Agent image built"

# Step 9: Deploy Infrastructure
echo ""
echo "[9/12] Deploying infrastructure..."
kubectl apply -f "$PROJECT_ROOT/deploy/infrastructure/postgresql-with-age.yaml" -n "$NAMESPACE"
kubectl apply -f "$PROJECT_ROOT/deploy/infrastructure/nats.yaml" -n "$NAMESPACE"
echo "✅ Infrastructure deployed"
echo "   Waiting for infrastructure to be ready..."
sleep 20

# Step 10: Deploy Core
echo ""
echo "[10/12] Deploying Core service..."
kubectl apply -f "$PROJECT_ROOT/deploy/core-service.yaml" -n "$NAMESPACE"
kubectl apply -f "$PROJECT_ROOT/deploy/core-deployment.yaml" -n "$NAMESPACE"
echo "✅ Core deployed"
echo "   Waiting for Core to be ready..."
sleep 15

# Step 11: Deploy Agent
echo ""
echo "[11/12] Deploying Agent..."
kubectl apply -f "$PROJECT_ROOT/deploy/agent-rbac.yaml" -n "$NAMESPACE"
kubectl apply -f "$PROJECT_ROOT/deploy/agent-daemonset.yaml" -n "$NAMESPACE"
echo "✅ Agent deployed"
sleep 10

# Step 12: Verify
echo ""
echo "[12/12] Verifying deployment..."
echo ""
echo "Pods:"
kubectl get pods -n "$NAMESPACE"
echo ""
echo "Services:"
kubectl get services -n "$NAMESPACE"
echo ""
echo "=========================================="
echo "Deployment Complete!"
echo "=========================================="
echo ""
echo "To check logs:"
echo "  kubectl logs -n $NAMESPACE -l app=ksam-core"
echo "  kubectl logs -n $NAMESPACE -l app=ksam-agent"
echo ""
echo "To port-forward:"
echo "  kubectl port-forward -n $NAMESPACE svc/ksam-core 8080:8080"

