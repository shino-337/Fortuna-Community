#!/bin/bash

# ============================================================================
# Apply Core Master-Only Configuration
# ============================================================================
# Applies deploy/fortuna-core-deployment.yaml, deploy/fortuna-agent-daemonset.yaml
# (Core chạy trên master node). Deploy files cũ (core-deployment.yaml, agent-daemonset.yaml) đã xóa.
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# Configuration
NAMESPACE="${NAMESPACE:-fortuna}"

cd "$PROJECT_ROOT"

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

log_section "Applying Core Master-Only Configuration"

# Step 0: Clear old deployments (if exist)
log_section "Step 0: Clearing Old Deployments"
log_info "Checking for old deployment files..."

# Old deploy files (core-deployment.yaml, agent-daemonset.yaml) removed; using fortuna-* only
OLD_DEPLOYMENTS=()

# Check and delete old deployments if they exist
for old_deploy in "${OLD_DEPLOYMENTS[@]}"; do
    if [ -f "$old_deploy" ]; then
        log_warning "Found old deployment file: $old_deploy"
        log_info "Checking if resources from this file exist in cluster..."
        
        # Try to extract resource name and kind from file
        RESOURCE_NAME=$(grep -E "^  name:|^    name:" "$old_deploy" | head -1 | awk '{print $2}' | tr -d '"' || echo "")
        RESOURCE_KIND=$(grep -E "^kind:" "$old_deploy" | head -1 | awk '{print $2}' || echo "")
        RESOURCE_NAMESPACE=$(grep -E "^  namespace:" "$old_deploy" | head -1 | awk '{print $2}' || echo "$NAMESPACE")
        
        if [ -n "$RESOURCE_NAME" ] && [ -n "$RESOURCE_KIND" ]; then
            log_info "Attempting to delete: $RESOURCE_KIND/$RESOURCE_NAME in namespace $RESOURCE_NAMESPACE"
            
            # Convert kind to lowercase for kubectl
            RESOURCE_TYPE=$(echo "$RESOURCE_KIND" | tr '[:upper:]' '[:lower:]')
            
            if kubectl get "$RESOURCE_TYPE" -n "$RESOURCE_NAMESPACE" "$RESOURCE_NAME" >/dev/null 2>&1; then
                log_warning "Found existing $RESOURCE_KIND: $RESOURCE_NAME"
                log_info "Deleting old $RESOURCE_KIND..."
                kubectl delete "$RESOURCE_TYPE" -n "$RESOURCE_NAMESPACE" "$RESOURCE_NAME" --wait=false 2>/dev/null || true
                log_success "Deleted old $RESOURCE_KIND: $RESOURCE_NAME"
            else
                log_info "No existing $RESOURCE_KIND found: $RESOURCE_NAME"
            fi
        fi
    fi
done

# Delete old Core deployments by label (if any exist with old labels)
log_info "Checking for Core deployments with old labels..."
OLD_CORE_DEPLOYS=$(kubectl get deployment -n "$NAMESPACE" -l app=ksam-core --no-headers 2>/dev/null | awk '{print $1}' || echo "")
if [ -n "$OLD_CORE_DEPLOYS" ]; then
    log_warning "Found old Core deployments: $OLD_CORE_DEPLOYS"
    for deploy in $OLD_CORE_DEPLOYS; do
        log_info "Deleting old Core deployment: $deploy"
        kubectl delete deployment -n "$NAMESPACE" "$deploy" --wait=false 2>/dev/null || true
    done
fi

# Delete old Agent DaemonSets by label (if any exist with old labels)
log_info "Checking for Agent DaemonSets with old labels..."
OLD_AGENT_DS=$(kubectl get daemonset -n "$NAMESPACE" -l app=ksam-agent --no-headers 2>/dev/null | awk '{print $1}' || echo "")
if [ -n "$OLD_AGENT_DS" ]; then
    log_warning "Found old Agent DaemonSets: $OLD_AGENT_DS"
    for ds in $OLD_AGENT_DS; do
        log_info "Deleting old Agent DaemonSet: $ds"
        kubectl delete daemonset -n "$NAMESPACE" "$ds" --wait=false 2>/dev/null || true
    done
fi

log_info "Waiting for old resources to be deleted (10 seconds)..."
sleep 10

# Step 1: Check master node labels
log_section "Step 1: Checking Master Node Labels"
MASTER_NODES=$(kubectl get nodes -l node-role.kubernetes.io/control-plane --no-headers 2>/dev/null | awk '{print $1}' || echo "")
if [ -z "$MASTER_NODES" ]; then
    # Try old label
    MASTER_NODES=$(kubectl get nodes -l node-role.kubernetes.io/master --no-headers 2>/dev/null | awk '{print $1}' || echo "")
fi

if [ -z "$MASTER_NODES" ]; then
    log_warning "No master nodes found with standard labels"
    log_info "Available nodes:"
    kubectl get nodes
    log_info "Please label master node manually:"
    log_info "  kubectl label node <master-node-name> node-role.kubernetes.io/control-plane="
else
    log_success "Master nodes found: $MASTER_NODES"
    for node in $MASTER_NODES; do
        log_info "  - $node"
        kubectl get node "$node" --show-labels | grep -E "control-plane|master" || true
    done
fi

# Step 2: Delete existing Core deployments (to avoid conflicts)
log_section "Step 2: Cleaning Up Existing Core Deployments"
log_info "Deleting existing Core deployments and pods..."

# Delete by new labels
kubectl delete deployment -n "$NAMESPACE" -l app.kubernetes.io/component=core --wait=false 2>/dev/null || true
kubectl delete pods -n "$NAMESPACE" -l app.kubernetes.io/component=core --wait=false 2>/dev/null || true

# Delete by old name (if exists)
kubectl delete deployment -n "$NAMESPACE" ksam-core --wait=false 2>/dev/null || true
kubectl delete deployment -n "$NAMESPACE" fortuna-core --wait=false 2>/dev/null || true

log_info "Waiting for old Core resources to be deleted (5 seconds)..."
sleep 5

# Step 3: Apply Core deployment
log_section "Step 3: Applying Core Deployment"
log_info "Applying updated Core deployment (master-only)..."
kubectl apply -f "$PROJECT_ROOT/deploy/fortuna-core-deployment.yaml" || {
    log_error "Failed to apply Core deployment"
    exit 1
}

log_success "Core deployment applied"

# Step 4: Delete existing Agent DaemonSets (to avoid conflicts)
log_section "Step 4: Cleaning Up Existing Agent DaemonSets"
log_info "Deleting existing Agent DaemonSets and pods..."

# Delete by new labels
kubectl delete daemonset -n "$NAMESPACE" -l app.kubernetes.io/component=agent --wait=false 2>/dev/null || true
kubectl delete pods -n "$NAMESPACE" -l app.kubernetes.io/component=agent --wait=false 2>/dev/null || true

# Delete by old name (if exists)
kubectl delete daemonset -n "$NAMESPACE" ksam-agent --wait=false 2>/dev/null || true
kubectl delete daemonset -n "$NAMESPACE" fortuna-agent --wait=false 2>/dev/null || true

log_info "Waiting for old Agent resources to be deleted (5 seconds)..."
sleep 5

# Step 5: Apply Agent DaemonSet
log_section "Step 5: Applying Agent DaemonSet"
log_info "Applying updated Agent DaemonSet (all nodes)..."
kubectl apply -f "$PROJECT_ROOT/deploy/fortuna-agent-daemonset.yaml" || {
    log_error "Failed to apply Agent DaemonSet"
    exit 1
}

log_success "Agent DaemonSet applied"

# Step 6: Wait for Core to be rescheduled
log_section "Step 6: Waiting for Core Pod Rescheduling"
log_info "Waiting for Core pod to be scheduled on master node (30 seconds)..."
sleep 30

log_info "Waiting for Core pod to be scheduled on master node (30 seconds)..."
sleep 30

# Step 7: Verify Core pod location
log_section "Step 7: Verifying Core Pod Location"
CORE_PODS=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core -o wide 2>/dev/null || echo "")
if [ -z "$CORE_PODS" ]; then
    log_error "No Core pods found"
    exit 1
fi

echo "$CORE_PODS"
echo ""

CORE_NODE=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core -o jsonpath='{.items[0].spec.nodeName}' 2>/dev/null || echo "")
if [ -n "$CORE_NODE" ]; then
    log_info "Core pod is running on node: $CORE_NODE"
    
    # Check if it's a master node
    if kubectl get node "$CORE_NODE" --show-labels 2>/dev/null | grep -qE "control-plane|master"; then
        log_success "Core pod is on master node ✅"
    else
        log_warning "Core pod is NOT on master node (may need to check node labels)"
    fi
fi

# Step 8: Verify Agent pods
log_section "Step 8: Verifying Agent Pods"
AGENT_PODS=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=agent -o wide 2>/dev/null || echo "")
if [ -z "$AGENT_PODS" ]; then
    log_warning "No Agent pods found"
else
    echo "$AGENT_PODS"
    echo ""
    
    AGENT_COUNT=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=agent --no-headers 2>/dev/null | wc -l || echo "0")
    NODE_COUNT=$(kubectl get nodes --no-headers 2>/dev/null | wc -l || echo "0")
    
    log_info "Agent pods: $AGENT_COUNT (expected: $NODE_COUNT - one per node)"
    
    if [ "$AGENT_COUNT" -eq "$NODE_COUNT" ]; then
        log_success "Agent pods match node count ✅"
    else
        log_warning "Agent pod count ($AGENT_COUNT) does not match node count ($NODE_COUNT)"
    fi
fi

# Step 9: Check Core pod status
log_section "Step 9: Checking Core Pod Status"
CORE_POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -n "$CORE_POD" ]; then
    CORE_STATUS=$(kubectl get pod -n "$NAMESPACE" "$CORE_POD" -o jsonpath='{.status.phase}' 2>/dev/null || echo "Unknown")
    CORE_READY=$(kubectl get pod -n "$NAMESPACE" "$CORE_POD" -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || echo "Unknown")
    
    log_info "Core pod: $CORE_POD"
    log_info "Status: $CORE_STATUS"
    log_info "Ready: $CORE_READY"
    
    if [ "$CORE_STATUS" = "Running" ] && [ "$CORE_READY" = "True" ]; then
        log_success "Core pod is Running and Ready ✅"
    else
        log_warning "Core pod is not fully ready"
        log_info "Check logs: kubectl logs -n $NAMESPACE $CORE_POD"
    fi
fi

# Step 10: Summary
log_section "Step 10: Summary"
log_info "Configuration applied:"
log_info "  ✅ Core deployment: master-only (nodeSelector + tolerations)"
log_info "  ✅ Agent DaemonSet: all nodes (with tolerations for master)"
echo ""
log_info "Next steps:"
log_info "  1. Monitor Core pod: kubectl get pods -n $NAMESPACE -l app.kubernetes.io/component=core -w"
log_info "  2. Monitor Agent pods: kubectl get pods -n $NAMESPACE -l app.kubernetes.io/component=agent -w"
log_info "  3. Check Core logs: kubectl logs -n $NAMESPACE -l app.kubernetes.io/component=core -f"
log_info "  4. Check Agent logs: kubectl logs -n $NAMESPACE -l app.kubernetes.io/component=agent -f"
echo ""
log_warning "Note: If Core pod is not scheduled on master, check node labels:"
log_info "  kubectl get nodes --show-labels"
log_info "  kubectl label node <master-node> node-role.kubernetes.io/control-plane="
echo ""

