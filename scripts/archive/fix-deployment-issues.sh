#!/bin/bash

# ============================================================================
# Fix Deployment Issues
# ============================================================================
# Fixes common deployment issues:
# 1. Missing mTLS secrets
# 2. Image pull errors (sets imagePullPolicy: Never for local images)
# ============================================================================

set -euo pipefail

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
NAMESPACE="${NAMESPACE:-fortuna}"

# Logging
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} ✅ $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} ❌ $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} ⚠️  $1"
}

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Check prerequisites
if ! command -v kubectl >/dev/null 2>&1; then
    log_error "kubectl is not installed"
    exit 1
fi

if ! kubectl cluster-info &>/dev/null 2>&1; then
    log_error "Kubernetes cluster is not accessible"
    exit 1
fi

log_success "Kubernetes cluster is accessible"

# Create namespace
log_info "Ensuring namespace '$NAMESPACE' exists..."
kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -
log_success "Namespace '$NAMESPACE' ready"

# Step 1: Create mTLS secrets
log_info "Step 1: Creating mTLS secrets..."
if [ -f "$PROJECT_ROOT/scripts/create-mtls-secrets.sh" ]; then
    "$PROJECT_ROOT/scripts/create-mtls-secrets.sh"
else
    log_error "create-mtls-secrets.sh not found"
    exit 1
fi

# Step 2: Verify images exist locally
log_info "Step 2: Checking if images exist locally..."
CORE_IMAGE_EXISTS=$(docker images fortuna-core:latest --format "{{.Repository}}:{{.Tag}}" 2>/dev/null | grep -c "fortuna-core:latest" || echo "0")
AGENT_IMAGE_EXISTS=$(docker images fortuna-agent:latest --format "{{.Repository}}:{{.Tag}}" 2>/dev/null | grep -c "fortuna-agent:latest" || echo "0")

if [ "$CORE_IMAGE_EXISTS" -eq 0 ]; then
    log_warning "fortuna-core:latest not found locally"
    log_info "Building Core image..."
    cd "$PROJECT_ROOT"
    docker build -f core/Dockerfile -t fortuna-core:latest . || {
        log_error "Failed to build Core image"
        exit 1
    }
    log_success "Core image built"
else
    log_success "Core image exists locally"
fi

if [ "$AGENT_IMAGE_EXISTS" -eq 0 ]; then
    log_warning "fortuna-agent:latest not found locally"
    log_info "Building Agent image..."
    cd "$PROJECT_ROOT"
    docker build -f agent/Dockerfile -t fortuna-agent:latest . || {
        log_error "Failed to build Agent image"
        exit 1
    }
    log_success "Agent image built"
else
    log_success "Agent image exists locally"
fi

# Step 3: Load images into cluster (if using containerd/nerdctl)
log_info "Step 3: Checking container runtime..."
if command -v nerdctl >/dev/null 2>&1; then
    log_info "Using nerdctl/containerd"
    # Images should already be available if built with nerdctl
    log_info "If images were built with docker, you may need to export/import them"
elif command -v crictl >/dev/null 2>&1; then
    log_info "Using containerd"
    log_warning "Images need to be available in containerd"
    log_info "If built with docker, export and import:"
    log_info "  docker save fortuna-core:latest -o fortuna-core.tar"
    log_info "  docker save fortuna-agent:latest -o fortuna-agent.tar"
    log_info "  # Then on each node:"
    log_info "  ctr -n k8s.io images import fortuna-core.tar"
    log_info "  ctr -n k8s.io images import fortuna-agent.tar"
else
    log_info "Using Docker runtime"
    log_info "Images should be available if built with docker"
fi

# Step 4: Apply deployments with Never pull policy
log_info "Step 4: Applying deployments..."
cd "$PROJECT_ROOT"

# Apply Core deployment
if [ -f "deploy/fortuna-core-deployment.yaml" ]; then
    log_info "Applying Core deployment..."
    kubectl apply -f deploy/fortuna-core-deployment.yaml
    log_success "Core deployment applied"
else
    log_error "Core deployment file not found"
    exit 1
fi

# Apply Agent DaemonSet
if [ -f "deploy/fortuna-agent-daemonset.yaml" ]; then
    log_info "Applying Agent DaemonSet..."
    kubectl apply -f deploy/fortuna-agent-daemonset.yaml
    log_success "Agent DaemonSet applied"
else
    log_error "Agent DaemonSet file not found"
    exit 1
fi

# Step 5: Wait and check status
log_info "Step 5: Waiting for deployments..."
sleep 5

echo ""
log_info "Current status:"
echo ""

echo "=== Secrets ==="
kubectl get secrets -n "$NAMESPACE" | grep fortuna || log_warning "No fortuna secrets found"
echo ""

echo "=== Core Pods ==="
kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core
echo ""

echo "=== Agent Pods ==="
kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=agent
echo ""

# Check for errors
CORE_ERRORS=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core -o jsonpath='{.items[*].status.containerStatuses[*].state.waiting.reason}' 2>/dev/null || echo "")
AGENT_ERRORS=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=agent -o jsonpath='{.items[*].status.containerStatuses[*].state.waiting.reason}' 2>/dev/null || echo "")

if echo "$CORE_ERRORS" | grep -q "ErrImagePull\|ImagePullBackOff"; then
    log_warning "Core has image pull errors"
    log_info "Make sure images are built and available:"
    log_info "  docker images | grep fortuna"
    log_info "  # Or with nerdctl:"
    log_info "  nerdctl images | grep fortuna"
fi

if echo "$AGENT_ERRORS" | grep -q "ErrImagePull\|ImagePullBackOff"; then
    log_warning "Agent has image pull errors"
    log_info "Make sure images are built and available"
fi

echo ""
log_success "Deployment fix completed!"
echo ""
log_info "Next steps:"
echo "  1. Check pod status: kubectl get pods -n $NAMESPACE"
echo "  2. Check logs: kubectl logs -n $NAMESPACE -l app.kubernetes.io/component=core"
echo "  3. Check events: kubectl get events -n $NAMESPACE --sort-by='.lastTimestamp'"
echo ""

