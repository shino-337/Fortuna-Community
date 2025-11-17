#!/bin/bash

# Force rebuild Dashboard with new tag to bypass Docker cache
# This ensures Kubernetes picks up the new image

set -e

MINIKUBE_DOCKER_ENV=$(minikube docker-env 2>/dev/null || echo "")

if [ -z "$MINIKUBE_DOCKER_ENV" ]; then
  echo "Error: Minikube is not running or not configured"
  exit 1
fi

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo "=========================================="
echo "Force Rebuild Dashboard (No Cache)"
echo "=========================================="
echo ""

# Setup Minikube Docker environment
eval $(minikube docker-env)

cd dashboard

# Clean all caches
echo -e "${YELLOW}Cleaning caches...${NC}"
rm -rf dist/ node_modules/.vite .vite .next 2>/dev/null || true
echo "✓ Cache cleared"

# Install dependencies
echo -e "${YELLOW}Installing dependencies...${NC}"
npm install
echo "✓ Dependencies installed"

# Build
echo -e "${YELLOW}Building Dashboard...${NC}"
npm run build
echo "✓ Build complete"

# Generate new tag based on timestamp
NEW_TAG="v$(date +%s)"
echo -e "${YELLOW}Building Docker image with tag: ${NEW_TAG}...${NC}"

# Build Docker image with no cache
docker build --no-cache -t ksam/dashboard:${NEW_TAG} -f Dockerfile .
docker tag ksam/dashboard:${NEW_TAG} ksam/dashboard:latest
# Also tag with the name that deployment uses
docker tag ksam/dashboard:${NEW_TAG} ksam-dashboard:latest

echo "✓ Docker image built: ksam/dashboard:${NEW_TAG}, ksam/dashboard:latest, and ksam-dashboard:latest"

cd ..

# Update deployment with new image (using the name that matches deployment)
echo -e "${YELLOW}Updating deployment...${NC}"
kubectl set image deployment/ksam-dashboard dashboard=ksam-dashboard:latest -n ksam

# Force rollout restart
echo -e "${YELLOW}Restarting deployment...${NC}"
kubectl rollout restart deployment/ksam-dashboard -n ksam

# Wait for rollout
echo -e "${YELLOW}Waiting for rollout to complete...${NC}"
kubectl rollout status deployment/ksam-dashboard -n ksam --timeout=120s

echo ""
echo "=========================================="
echo -e "${GREEN}✓ Dashboard force rebuild complete!${NC}"
echo "=========================================="
echo ""
echo "New image tag: ${NEW_TAG}"
echo ""
echo "To verify:"
echo "  kubectl get pods -n ksam -l app=ksam-dashboard"
echo "  kubectl describe pod -n ksam -l app=ksam-dashboard | grep Image"
echo ""
echo "To clear browser cache:"
echo "  - Chrome/Edge: Ctrl+Shift+Delete (Windows) or Cmd+Shift+Delete (Mac)"
echo "  - Firefox: Ctrl+Shift+Delete (Windows) or Cmd+Shift+Delete (Mac)"
echo "  - Or use Hard Refresh: Ctrl+F5 (Windows) or Cmd+Shift+R (Mac)"

