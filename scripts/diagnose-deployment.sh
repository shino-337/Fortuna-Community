#!/bin/bash

# ============================================================================
# Diagnose Fortuna Deployment Issues
# ============================================================================
# Comprehensive diagnosis of Core and Agent deployment issues
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

# 1. Check namespace
log_section "1. Namespace Check"
if kubectl get namespace "$NAMESPACE" &>/dev/null 2>&1; then
    log_success "Namespace '$NAMESPACE' exists"
else
    log_error "Namespace '$NAMESPACE' does not exist"
    log_info "Creating namespace..."
    kubectl create namespace "$NAMESPACE"
fi

# 2. Check secrets
log_section "2. Secrets Check"
SECRETS=$(kubectl get secrets -n "$NAMESPACE" 2>/dev/null | grep fortuna || echo "")
if [ -z "$SECRETS" ]; then
    log_error "No fortuna secrets found"
    log_info "Run: ./scripts/create-mtls-secrets.sh"
else
    log_success "Secrets found:"
    echo "$SECRETS" | while read -r line; do
        echo "  - $line"
    done
fi

# Check specific secrets
CORE_SECRET=$(kubectl get secret -n "$NAMESPACE" fortuna-core-server-tls 2>/dev/null | grep -c "fortuna-core-server-tls" || echo "0")
AGENT_SECRET=$(kubectl get secret -n "$NAMESPACE" fortuna-agent-client-tls 2>/dev/null | grep -c "fortuna-agent-client-tls" || echo "0")

if [ "$CORE_SECRET" -eq 0 ]; then
    log_error "Secret 'fortuna-core-server-tls' not found"
fi
if [ "$AGENT_SECRET" -eq 0 ]; then
    log_error "Secret 'fortuna-agent-client-tls' not found"
fi

# 3. Check images in containerd
log_section "3. Images in Containerd"
if command -v ctr >/dev/null 2>&1; then
    log_info "Checking images in k8s.io namespace..."
    CORE_IMAGE=$(ctr -n k8s.io images ls 2>/dev/null | grep "fortuna-core:latest" || echo "")
    AGENT_IMAGE=$(ctr -n k8s.io images ls 2>/dev/null | grep "fortuna-agent:latest" || echo "")
    
    if [ -n "$CORE_IMAGE" ]; then
        log_success "Core image found in k8s.io namespace"
        echo "  $CORE_IMAGE"
    else
        log_error "Core image NOT found in k8s.io namespace"
        log_info "Run: ./scripts/load-images-to-containerd.sh"
    fi
    
    if [ -n "$AGENT_IMAGE" ]; then
        log_success "Agent image found in k8s.io namespace"
        echo "  $AGENT_IMAGE"
    else
        log_error "Agent image NOT found in k8s.io namespace"
        log_info "Run: ./scripts/load-images-to-containerd.sh"
    fi
    
    # Check default namespace too
    CORE_DEFAULT=$(ctr images ls 2>/dev/null | grep "fortuna-core:latest" || echo "")
    if [ -n "$CORE_DEFAULT" ] && [ -z "$CORE_IMAGE" ]; then
        log_warning "Core image found in default namespace but not in k8s.io"
        log_info "Run: ./scripts/fix-containerd-images.sh"
    fi
else
    log_warning "ctr not available, skipping image check"
fi

# 4. Check Core deployment
log_section "4. Core Deployment Status"
CORE_PODS=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core 2>/dev/null || echo "")
if [ -z "$CORE_PODS" ]; then
    log_error "No Core pods found"
else
    echo "$CORE_PODS"
    echo ""
    
    # Check each pod
    kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.phase}{"\t"}{.status.containerStatuses[0].state.waiting.reason}{"\n"}{end}' 2>/dev/null | while read -r pod_name phase reason; do
        if [ "$phase" = "Running" ]; then
            log_success "Pod $pod_name is Running"
        elif [ "$phase" = "Pending" ]; then
            log_warning "Pod $pod_name is Pending"
            if [ -n "$reason" ]; then
                log_info "  Reason: $reason"
            fi
        elif [ -n "$reason" ]; then
            log_error "Pod $pod_name has issue: $reason"
        fi
    done
    
    # Get detailed status
    echo ""
    log_info "Detailed pod status:"
    kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core -o wide
fi

# 5. Check Agent DaemonSet
log_section "5. Agent DaemonSet Status"
AGENT_PODS=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=agent 2>/dev/null || echo "")
if [ -z "$AGENT_PODS" ]; then
    log_error "No Agent pods found"
else
    echo "$AGENT_PODS"
    echo ""
    
    # Count running vs not running
    TOTAL_AGENT=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=agent --no-headers 2>/dev/null | wc -l || echo "0")
    RUNNING_AGENT=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=agent --no-headers 2>/dev/null | grep -c "Running" || echo "0")
    
    log_info "Agent pods: $RUNNING_AGENT/$TOTAL_AGENT running"
    
    # Check each pod
    kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=agent -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.phase}{"\t"}{.status.containerStatuses[0].state.waiting.reason}{"\t"}{.spec.nodeName}{"\n"}{end}' 2>/dev/null | while read -r pod_name phase reason node; do
        if [ "$phase" = "Running" ]; then
            log_success "Pod $pod_name is Running on node $node"
        elif [ "$phase" = "Pending" ]; then
            log_warning "Pod $pod_name is Pending on node $node"
            if [ -n "$reason" ]; then
                log_info "  Reason: $reason"
            fi
        elif [ -n "$reason" ]; then
            log_error "Pod $pod_name on node $node has issue: $reason"
        fi
    done
    
    # Get detailed status
    echo ""
    log_info "Detailed pod status:"
    kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=agent -o wide
fi

# 6. Check events
log_section "6. Recent Events"
log_info "Recent events in namespace '$NAMESPACE':"
kubectl get events -n "$NAMESPACE" --sort-by='.lastTimestamp' | tail -20

# 7. Check specific errors
log_section "7. Error Analysis"

# Core errors
CORE_IMAGE_ERROR=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core -o jsonpath='{.items[*].status.containerStatuses[*].state.waiting.reason}' 2>/dev/null | grep -o "ErrImagePull\|ErrImageNeverPull\|ImagePullBackOff" || echo "")
if [ -n "$CORE_IMAGE_ERROR" ]; then
    log_error "Core has image pull error: $CORE_IMAGE_ERROR"
    log_info "Solution:"
    log_info "  1. Ensure image is in k8s.io namespace: ctr -n k8s.io images ls | grep fortuna-core"
    log_info "  2. If missing, run: ./scripts/load-images-to-containerd.sh"
    log_info "  3. Or: ./scripts/fix-containerd-images.sh"
fi

# Agent errors
AGENT_IMAGE_ERROR=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=agent -o jsonpath='{.items[*].status.containerStatuses[*].state.waiting.reason}' 2>/dev/null | grep -o "ErrImagePull\|ErrImageNeverPull\|ImagePullBackOff" || echo "")
if [ -n "$AGENT_IMAGE_ERROR" ]; then
    log_error "Agent has image pull error: $AGENT_IMAGE_ERROR"
    log_info "Solution: Same as Core above"
fi

# Secret errors
SECRET_ERROR=$(kubectl get events -n "$NAMESPACE" --field-selector reason=FailedMount 2>/dev/null | grep -c "secret.*not found" || echo "0")
if [ "$SECRET_ERROR" -gt 0 ]; then
    log_error "Secret mount errors detected"
    log_info "Solution: Run ./scripts/create-mtls-secrets.sh"
fi

# 8. Check nodes
log_section "8. Node Status"
kubectl get nodes -o wide

# 9. Recommendations
log_section "9. Recommendations"

ISSUES_FOUND=0

if [ "$CORE_SECRET" -eq 0 ] || [ "$AGENT_SECRET" -eq 0 ]; then
    ISSUES_FOUND=1
    log_warning "Missing secrets - Run: ./scripts/create-mtls-secrets.sh"
fi

if [ -z "$CORE_IMAGE" ] && command -v ctr >/dev/null 2>&1; then
    ISSUES_FOUND=1
    log_warning "Core image missing - Run: ./scripts/load-images-to-containerd.sh"
fi

if [ -z "$AGENT_IMAGE" ] && command -v ctr >/dev/null 2>&1; then
    ISSUES_FOUND=1
    log_warning "Agent image missing - Run: ./scripts/load-images-to-containerd.sh"
fi

if [ "$ISSUES_FOUND" -eq 0 ]; then
    log_success "No obvious issues found"
    log_info "If pods are still not running, check:"
    log_info "  - Pod logs: kubectl logs -n $NAMESPACE <pod-name>"
    log_info "  - Pod describe: kubectl describe pod -n $NAMESPACE <pod-name>"
    log_info "  - Events: kubectl get events -n $NAMESPACE --sort-by='.lastTimestamp'"
fi

echo ""

