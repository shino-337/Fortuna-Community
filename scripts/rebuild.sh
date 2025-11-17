#!/bin/bash

# Script to rebuild and redeploy KSAM components
# Usage: ./scripts/rebuild.sh [component]
# Components: dashboard, core, agent, all (default)

set -e

COMPONENT="${1:-all}"
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
echo "KSAM Rebuild Script"
echo "=========================================="
echo "Component: $COMPONENT"
echo ""

# Setup Minikube Docker environment
eval $(minikube docker-env)

rebuild_dashboard() {
  echo -e "${YELLOW}Rebuilding Dashboard...${NC}"
  cd dashboard
  
  # Clean and install dependencies
  echo "Installing dependencies..."
  npm install
  
  # Build
  echo "Building Dashboard..."
  npm run build
  
  # Build Docker image with latest tag
  echo "Building Docker image..."
  docker build -t ksam/dashboard:latest -f Dockerfile .
  
  cd ..
  echo -e "${GREEN}✓ Dashboard rebuilt${NC}"
  echo "  Image: ksam/dashboard:latest"
}

rebuild_core() {
  echo -e "${YELLOW}Rebuilding Core Controller...${NC}"
  cd core
  
  # Tidy Go modules
  echo "Tidying Go modules..."
  go mod tidy
  
  # Build Docker image
  echo "Building Docker image..."
  docker build -t ksam-core:latest -f Dockerfile .
  
  cd ..
  echo -e "${GREEN}✓ Core Controller rebuilt${NC}"
}

rebuild_agent() {
  echo -e "${YELLOW}Rebuilding Agent...${NC}"
  cd agent
  
  # Tidy Go modules
  echo "Tidying Go modules..."
  go mod tidy
  
  # Build Docker image
  echo "Building Docker image..."
  docker build -t ksam/agent:latest -f Dockerfile .
  
  cd ..
  echo -e "${GREEN}✓ Agent rebuilt${NC}"
}

redeploy_dashboard() {
  echo -e "${YELLOW}Redeploying Dashboard...${NC}"
  
  # Update deployment with latest image
  echo "Updating deployment with image: ksam/dashboard:latest..."
  kubectl set image deployment/ksam-dashboard dashboard=ksam/dashboard:latest -n ksam
  
  # Wait for rollout
  kubectl rollout status deployment/ksam-dashboard -n ksam --timeout=120s
  echo -e "${GREEN}✓ Dashboard redeployed${NC}"
}

redeploy_core() {
  echo -e "${YELLOW}Redeploying Core Controller...${NC}"
  kubectl rollout restart deployment/ksam-core -n ksam
  kubectl rollout status deployment/ksam-core -n ksam --timeout=120s
  echo -e "${GREEN}✓ Core Controller redeployed${NC}"
}

redeploy_agent() {
  echo -e "${YELLOW}Redeploying Agent...${NC}"
  kubectl rollout restart daemonset/ksam-agent -n kube-system
  kubectl rollout status daemonset/ksam-agent -n kube-system --timeout=120s
  echo -e "${GREEN}✓ Agent redeployed${NC}"
}

# Main logic
case "$COMPONENT" in
  dashboard)
    rebuild_dashboard
    redeploy_dashboard
    ;;
  core)
    rebuild_core
    redeploy_core
    ;;
  agent)
    rebuild_agent
    redeploy_agent
    ;;
  all)
    rebuild_dashboard
    rebuild_core
    rebuild_agent
    redeploy_dashboard
    redeploy_core
    redeploy_agent
    ;;
  *)
    echo -e "${RED}Error: Unknown component '$COMPONENT'${NC}"
    echo "Usage: $0 [dashboard|core|agent|all]"
    exit 1
    ;;
esac

echo ""
echo "=========================================="
echo -e "${GREEN}Rebuild complete!${NC}"
echo "=========================================="
echo ""
echo "To check status:"
echo "  kubectl get pods -n ksam"
echo "  kubectl get pods -n kube-system -l app=ksam-agent"
echo ""
echo "To view logs:"
echo "  kubectl logs -n ksam -l app=ksam-dashboard -f"
echo "  kubectl logs -n ksam -l app=ksam-core -f"
echo "  kubectl logs -n kube-system -l app=ksam-agent -f"

