#!/bin/bash

# ============================================================================
# Quick Deploy Script for Containerd
# ============================================================================
# Quick script to build with nerdctl and deploy to K8s cluster
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo "=========================================="
echo "Fortuna Quick Deploy (Containerd)"
echo "=========================================="
echo ""

# Check prerequisites
if ! command -v nerdctl >/dev/null 2>&1; then
    echo -e "${RED}❌ nerdctl not found${NC}"
    echo "Install nerdctl: https://github.com/containerd/nerdctl"
    exit 1
fi

if ! command -v kubectl >/dev/null 2>&1; then
    echo -e "${RED}❌ kubectl not found${NC}"
    exit 1
fi

# Step 1: Build images
echo -e "${BLUE}Step 1: Building images with nerdctl...${NC}"
if [ -f "${SCRIPT_DIR}/build-with-containerd.sh" ]; then
    bash "${SCRIPT_DIR}/build-with-containerd.sh"
else
    echo -e "${RED}❌${NC} build-with-containerd.sh not found"
    exit 1
fi

# Step 2: Verify images
echo ""
echo -e "${BLUE}Step 2: Verifying images in containerd...${NC}"
if ctr -n k8s.io images ls 2>/dev/null | grep -q "fortuna-core"; then
    echo -e "${GREEN}✅${NC} Images found in containerd"
    ctr -n k8s.io images ls | grep fortuna
else
    echo -e "${RED}❌${NC} Images not found in containerd"
    exit 1
fi

# Step 3: Create namespace
echo ""
echo -e "${BLUE}Step 3: Creating namespace...${NC}"
kubectl create namespace fortuna --dry-run=client -o yaml | kubectl apply -f -
echo -e "${GREEN}✅${NC} Namespace ready"

# Step 4: Deploy infrastructure (if not exists)
echo ""
echo -e "${BLUE}Step 4: Checking infrastructure...${NC}"
if ! kubectl get svc -n fortuna postgres >/dev/null 2>&1; then
    echo "Deploying PostgreSQL..."
    kubectl apply -f "${PROJECT_ROOT}/deploy/infrastructure/postgresql-with-age.yaml"
    kubectl wait --for=condition=ready pod -n fortuna -l app=postgres --timeout=300s || true
fi

if ! kubectl get svc -n fortuna nats >/dev/null 2>&1; then
    echo "Deploying NATS..."
    kubectl apply -f "${PROJECT_ROOT}/deploy/infrastructure/nats.yaml"
    kubectl wait --for=condition=ready pod -n fortuna -l app=nats --timeout=300s || true
fi

# Step 5: Deploy Fortuna
echo ""
echo -e "${BLUE}Step 5: Deploying Fortuna...${NC}"

# Deploy RBAC
kubectl apply -f "${PROJECT_ROOT}/deploy/fortuna-rbac.yaml"

# Deploy Core
kubectl apply -f "${PROJECT_ROOT}/deploy/fortuna-core-deployment.yaml"

# Wait for Core
echo "Waiting for Core to be ready..."
kubectl wait --for=condition=available deployment/fortuna-core -n fortuna --timeout=300s || true

# Deploy Agent
kubectl apply -f "${PROJECT_ROOT}/deploy/fortuna-agent-daemonset.yaml"

# Step 6: Show status
echo ""
echo -e "${BLUE}Step 6: Deployment status...${NC}"
echo ""
echo "=== Pods ==="
kubectl get pods -n fortuna -l app.kubernetes.io/component=core
kubectl get pods -n fortuna -l app.kubernetes.io/component=agent

echo ""
echo "=== Images in Containerd ==="
ctr -n k8s.io images ls | grep fortuna | head -5

echo ""
echo "=========================================="
echo -e "${GREEN}✅ Deployment completed!${NC}"
echo "=========================================="
echo ""
echo "Check status with:"
echo "  kubectl get pods -n fortuna"
echo "  kubectl get deployments -n fortuna"
echo "  kubectl get daemonsets -n fortuna"
echo ""

