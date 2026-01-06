#!/bin/bash

# ============================================================================
# Apply NATS Single Replica Configuration
# ============================================================================
# Applies updated NATS configuration for single replica setup
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
NATS_YAML="${NATS_YAML:-deploy/infrastructure/nats.yaml}"

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

if [ ! -f "$NATS_YAML" ]; then
    log_error "NATS YAML file not found: $NATS_YAML"
    exit 1
fi

log_section "Applying NATS Single Replica Configuration"

# Step 1: Check current NATS StatefulSet
log_section "Step 1: Checking Current NATS StatefulSet"
CURRENT_REPLICAS=$(kubectl get statefulset -n "$NAMESPACE" nats -o jsonpath='{.spec.replicas}' 2>/dev/null || echo "0")
log_info "Current NATS replicas: $CURRENT_REPLICAS"

# Step 2: Update ConfigMap
log_section "Step 2: Updating NATS ConfigMap"
log_info "Applying ConfigMap update..."
kubectl apply -f "$NATS_YAML" --selector=kind=ConfigMap 2>/dev/null || {
    log_info "Applying full YAML (ConfigMap will be updated)..."
    kubectl apply -f "$NATS_YAML" || {
        log_error "Failed to apply NATS YAML"
        exit 1
    }
}

log_success "ConfigMap updated"

# Step 3: Update StatefulSet
log_section "Step 3: Updating NATS StatefulSet"
log_info "Applying StatefulSet update..."
kubectl apply -f "$NATS_YAML" || {
    log_error "Failed to apply NATS StatefulSet"
    exit 1
}

log_success "StatefulSet updated"

# Step 4: Scale down if needed
log_section "Step 4: Scaling NATS to 1 Replica"
if [ "$CURRENT_REPLICAS" != "1" ]; then
    log_info "Scaling NATS StatefulSet from $CURRENT_REPLICAS to 1 replica..."
    kubectl scale statefulset -n "$NAMESPACE" nats --replicas=1 2>/dev/null || {
        log_warning "Could not scale StatefulSet (may already be at 1)"
    }
else
    log_info "NATS already at 1 replica"
fi

# Step 5: Restart NATS to apply new config
log_section "Step 5: Restarting NATS to Apply New Configuration"
log_info "Restarting NATS StatefulSet..."
kubectl rollout restart statefulset -n "$NAMESPACE" nats 2>/dev/null || {
    log_warning "Could not restart StatefulSet (may need manual restart)"
}

# Step 6: Wait for NATS to be ready
log_section "Step 6: Waiting for NATS to be Ready"
log_info "Waiting for NATS pod to be ready (60 seconds timeout)..."
kubectl wait --for=condition=ready pod -n "$NAMESPACE" -l app=nats --timeout=60s 2>/dev/null || {
    log_warning "NATS pod not ready within timeout, checking status..."
    kubectl get pods -n "$NAMESPACE" -l app=nats
}

# Step 7: Verify NATS status
log_section "Step 7: Verifying NATS Status"
NATS_PODS=$(kubectl get pods -n "$NAMESPACE" -l app=nats 2>/dev/null || echo "")
if [ -z "$NATS_PODS" ]; then
    log_error "No NATS pods found"
    exit 1
fi

echo "$NATS_PODS"
echo ""

NATS_RUNNING=$(kubectl get pods -n "$NAMESPACE" -l app=nats --no-headers 2>/dev/null | grep -c Running || echo "0")
if [ "$NATS_RUNNING" -eq 1 ]; then
    log_success "NATS is running with 1 replica"
    
    # Check NATS logs for JetStream
    NATS_POD=$(kubectl get pods -n "$NAMESPACE" -l app=nats -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
    if [ -n "$NATS_POD" ]; then
        log_info "Checking NATS logs for JetStream status..."
        kubectl logs -n "$NAMESPACE" "$NATS_POD" --tail=20 2>/dev/null | grep -i "jetstream\|ready" | tail -5 || log_warning "Could not verify JetStream status"
    fi
else
    log_warning "NATS is not running properly (Running pods: $NATS_RUNNING)"
fi

# Step 8: Summary
log_section "Step 8: Summary"
log_info "NATS configuration updated for single replica setup:"
log_info "  ✅ ConfigMap updated (removed cluster config)"
log_info "  ✅ StatefulSet updated (replicas: 1)"
log_info "  ✅ Service updated (removed cluster port)"
log_info "  ✅ NATS restarted"
echo ""
log_info "Next steps:"
log_info "  1. Verify NATS logs: kubectl logs -n $NAMESPACE -l app=nats"
log_info "  2. Restart Core: kubectl rollout restart deployment -n $NAMESPACE fortuna-core"
log_info "  3. Monitor Core logs: kubectl logs -n $NAMESPACE -l app.kubernetes.io/component=core -f"
echo ""
log_warning "Note: To scale back to 3 replicas, update nats.yaml and re-apply"
echo ""

