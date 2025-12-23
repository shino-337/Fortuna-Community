#!/bin/bash

# Rebuild and Deploy V2 Scorer
# This script rebuilds the core image, clears cache, and deploys

set -e

echo "=========================================="
echo "REBUILD & DEPLOY V2 SCORER"
echo "=========================================="
echo ""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Step 1: Clean old images
echo -e "${YELLOW}Step 1: Cleaning old images...${NC}"
docker images | grep "ksam/core" | awk '{print $3}' | xargs -r docker rmi -f 2>/dev/null || true
echo -e "${GREEN}✅ Old images cleaned${NC}"
echo ""

# Step 2: Clear Docker build cache
echo -e "${YELLOW}Step 2: Clearing Docker build cache...${NC}"
docker builder prune -af --filter "until=24h" || true
echo -e "${GREEN}✅ Build cache cleared${NC}"
echo ""

# Step 3: Rebuild image with --no-cache
echo -e "${YELLOW}Step 3: Rebuilding Docker image (no cache)...${NC}"
cd KSAM/core
docker build --no-cache -t ksam/core:latest -f Dockerfile .
if [ $? -ne 0 ]; then
    echo -e "${RED}❌ Build failed${NC}"
    exit 1
fi
echo -e "${GREEN}✅ Image rebuilt successfully${NC}"
echo ""

# Step 4: Load image into Minikube (if using Minikube)
if command -v minikube &> /dev/null; then
    echo -e "${YELLOW}Step 4: Loading image into Minikube...${NC}"
    minikube image load ksam/core:latest || true
    echo -e "${GREEN}✅ Image loaded into Minikube${NC}"
    echo ""
fi

# Step 5: Delete old deployment
echo -e "${YELLOW}Step 5: Deleting old deployment...${NC}"
kubectl delete deployment ksam-core -n ksam --ignore-not-found=true
kubectl wait --for=delete deployment/ksam-core -n ksam --timeout=60s 2>/dev/null || true
echo -e "${GREEN}✅ Old deployment deleted${NC}"
echo ""

# Step 6: Apply new deployment
echo -e "${YELLOW}Step 6: Applying new deployment...${NC}"
cd ../..
kubectl apply -f KSAM/deploy/core-deployment.yaml
echo -e "${GREEN}✅ Deployment applied${NC}"
echo ""

# Step 7: Wait for pod to be ready
echo -e "${YELLOW}Step 7: Waiting for pod to be ready...${NC}"
kubectl wait --for=condition=ready pod -l app=ksam-core -n ksam --timeout=300s
if [ $? -ne 0 ]; then
    echo -e "${RED}❌ Pod not ready after 5 minutes${NC}"
    echo "Checking pod status..."
    kubectl get pods -n ksam -l app=ksam-core
    kubectl describe pod -n ksam -l app=ksam-core | tail -30
    exit 1
fi
echo -e "${GREEN}✅ Pod is ready${NC}"
echo ""

# Step 8: Check pod status
echo -e "${YELLOW}Step 8: Checking pod status...${NC}"
kubectl get pods -n ksam -l app=ksam-core
POD_NAME=$(kubectl get pod -n ksam -l app=ksam-core -o jsonpath='{.items[0].metadata.name}')
echo -e "${GREEN}✅ Pod name: $POD_NAME${NC}"
echo ""

# Step 9: Check logs for migration
echo -e "${YELLOW}Step 9: Checking migration logs...${NC}"
sleep 5
kubectl logs -n ksam $POD_NAME --tail=50 | grep -i "migration\|Migration018\|V2\|scorer" || echo "No migration logs found yet"
echo ""

# Step 10: Verify service is running
echo -e "${YELLOW}Step 10: Verifying service...${NC}"
kubectl get svc -n ksam ksam-core || echo "Service check..."
echo ""

echo "=========================================="
echo -e "${GREEN}✅ REBUILD & DEPLOY COMPLETE${NC}"
echo "=========================================="
echo ""
echo "Next steps:"
echo "  1. Check pod logs: kubectl logs -n ksam $POD_NAME"
echo "  2. Verify migration: kubectl logs -n ksam $POD_NAME | grep Migration018"
echo "  3. Test V2 scorer API endpoints"
echo ""


