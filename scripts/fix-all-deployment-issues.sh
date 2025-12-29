#!/bin/bash

# ============================================================================
# Fix All Deployment Issues
# ============================================================================
# Comprehensive fix for all common deployment issues
# ============================================================================

set -euo pipefail

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
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

log_section() {
    echo ""
    echo -e "${CYAN}========================================${NC}"
    echo -e "${CYAN}$1${NC}"
    echo -e "${CYAN}========================================${NC}"
    echo ""
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

log_success "Starting comprehensive fix..."

# Step 1: Create namespace
log_section "Step 1: Creating namespace"
kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -
log_success "Namespace ready"

# Step 2: Create mTLS secrets
log_section "Step 2: Creating mTLS secrets"
if [ -f "$PROJECT_ROOT/scripts/create-mtls-secrets.sh" ]; then
    "$PROJECT_ROOT/scripts/create-mtls-secrets.sh"
else
    log_error "create-mtls-secrets.sh not found"
    exit 1
fi

# Step 3: Check and load images
log_section "Step 3: Checking images in containerd"

if command -v ctr >/dev/null 2>&1; then
    # Check if images exist in k8s.io namespace
    CORE_IN_K8S=$(ctr -n k8s.io images ls 2>/dev/null | grep -c "fortuna-core:latest" || echo "0")
    AGENT_IN_K8S=$(ctr -n k8s.io images ls 2>/dev/null | grep -c "fortuna-agent:latest" || echo "0")
    
    if [ "$CORE_IN_K8S" -eq 0 ] || [ "$AGENT_IN_K8S" -eq 0 ]; then
        log_warning "Images missing in k8s.io namespace"
        
        # Try to copy from default namespace to k8s.io
        if [ -f "$PROJECT_ROOT/scripts/copy-images-to-k8s-namespace.sh" ]; then
            log_info "Copying images to k8s.io namespace..."
            "$PROJECT_ROOT/scripts/copy-images-to-k8s-namespace.sh"
        elif [ -f "$PROJECT_ROOT/scripts/fix-containerd-images.sh" ]; then
            log_info "Running fix-containerd-images.sh..."
            "$PROJECT_ROOT/scripts/fix-containerd-images.sh"
        fi
        
        # If still missing, try loading from Docker
        CORE_IN_K8S_AFTER=$(ctr -n k8s.io images ls 2>/dev/null | grep -c "fortuna-core:latest" || echo "0")
        AGENT_IN_K8S_AFTER=$(ctr -n k8s.io images ls 2>/dev/null | grep -c "fortuna-agent:latest" || echo "0")
        
        if [ "$CORE_IN_K8S_AFTER" -eq 0 ] || [ "$AGENT_IN_K8S_AFTER" -eq 0 ]; then
            if [ -f "$PROJECT_ROOT/scripts/load-images-to-containerd.sh" ]; then
                log_info "Loading images from Docker to containerd..."
                "$PROJECT_ROOT/scripts/load-images-to-containerd.sh"
            else
                log_warning "load-images-to-containerd.sh not found"
                log_info "Manual steps:"
                log_info "  1. Build images: docker build -f core/Dockerfile -t fortuna-core:latest ."
                log_info "  2. Export: docker save fortuna-core:latest -o fortuna-core.tar"
                log_info "  3. Import: ctr -n k8s.io images import fortuna-core.tar"
            fi
        fi
    else
        log_success "Images are available in k8s.io namespace"
    fi
    
    # Final verification
    log_info "Verifying images:"
    ctr -n k8s.io images ls | grep fortuna || log_warning "No fortuna images found"
else
    log_warning "ctr not available, skipping image check"
fi

# Step 4: Delete and recreate deployments
log_section "Step 4: Applying deployments"

# Delete existing deployments to force recreation
log_info "Deleting existing deployments..."
kubectl delete deployment fortuna-core -n "$NAMESPACE" 2>/dev/null || true
kubectl delete daemonset fortuna-agent -n "$NAMESPACE" 2>/dev/null || true
sleep 2

# Apply deployments
log_info "Applying Core deployment..."
if [ -f "$PROJECT_ROOT/deploy/fortuna-core-deployment.yaml" ]; then
    kubectl apply -f "$PROJECT_ROOT/deploy/fortuna-core-deployment.yaml"
    log_success "Core deployment applied"
else
    log_error "Core deployment file not found"
    exit 1
fi

log_info "Applying Agent DaemonSet..."
if [ -f "$PROJECT_ROOT/deploy/fortuna-agent-daemonset.yaml" ]; then
    kubectl apply -f "$PROJECT_ROOT/deploy/fortuna-agent-daemonset.yaml"
    log_success "Agent DaemonSet applied"
else
    log_error "Agent DaemonSet file not found"
    exit 1
fi

# Step 5: Wait and check status
log_section "Step 5: Waiting for pods"
sleep 5

log_info "Current pod status:"
echo ""

echo "=== Core Pods ==="
kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core -o wide
echo ""

echo "=== Agent Pods ==="
kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=agent -o wide
echo ""

# Check for specific issues
CORE_PENDING=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core --no-headers 2>/dev/null | grep -c "Pending\|ImagePull\|ErrImage" || echo "0")
AGENT_PENDING=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=agent --no-headers 2>/dev/null | grep -c "Pending\|ImagePull\|ErrImage" || echo "0")

if [ "$CORE_PENDING" -gt 0 ] || [ "$AGENT_PENDING" -gt 0 ]; then
    log_warning "Some pods are still having issues"
    echo ""
    log_info "Troubleshooting steps:"
    echo ""
    
    if [ "$CORE_PENDING" -gt 0 ]; then
        log_info "Core pod issues:"
        CORE_POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
        if [ -n "$CORE_POD" ]; then
            log_info "  Describe: kubectl describe pod -n $NAMESPACE $CORE_POD"
            log_info "  Events: kubectl get events -n $NAMESPACE --field-selector involvedObject.name=$CORE_POD"
        fi
    fi
    
    if [ "$AGENT_PENDING" -gt 0 ]; then
        log_info "Agent pod issues:"
        AGENT_PODS=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=agent -o jsonpath='{.items[*].metadata.name}' 2>/dev/null || echo "")
        for pod in $AGENT_PODS; do
            STATUS=$(kubectl get pod -n "$NAMESPACE" "$pod" -o jsonpath='{.status.phase}' 2>/dev/null || echo "Unknown")
            if [ "$STATUS" != "Running" ]; then
                log_info "  Pod $pod: kubectl describe pod -n $NAMESPACE $pod"
            fi
        done
    fi
else
    log_success "All pods should be starting!"
fi

# Final summary
log_section "Summary"
log_info "Run diagnosis script for detailed analysis:"
log_info "  ./scripts/diagnose-deployment.sh"
echo ""

log_info "Useful commands:"
echo "  - Check pods: kubectl get pods -n $NAMESPACE"
echo "  - Check logs: kubectl logs -n $NAMESPACE -l app.kubernetes.io/component=core"
echo "  - Check events: kubectl get events -n $NAMESPACE --sort-by='.lastTimestamp'"
echo ""

