#!/bin/bash

# ============================================================================
# Fix NATS Storage Issue
# ============================================================================
# Diagnoses and fixes NATS storage/placement issues preventing stream creation
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

log_section "NATS Storage Issue Diagnosis and Fix"

# Step 1: Check NATS pods
log_section "Step 1: Checking NATS Pods"
NATS_PODS=$(kubectl get pods -n "$NAMESPACE" -l app=nats 2>/dev/null || echo "")
if [ -z "$NATS_PODS" ]; then
    log_error "No NATS pods found"
    exit 1
fi

echo "$NATS_PODS"
echo ""

NATS_RUNNING=$(kubectl get pods -n "$NAMESPACE" -l app=nats --no-headers 2>/dev/null | grep -c Running || echo "0")
NATS_TOTAL=$(kubectl get pods -n "$NAMESPACE" -l app=nats --no-headers 2>/dev/null | wc -l || echo "0")

log_info "NATS pods: $NATS_RUNNING/$NATS_TOTAL Running"

if [ "$NATS_RUNNING" -lt "$NATS_TOTAL" ]; then
    log_warning "Not all NATS pods are Running"
    log_info "Checking pod status..."
    kubectl get pods -n "$NAMESPACE" -l app=nats -o wide
fi

# Step 2: Check NATS PVCs
log_section "Step 2: Checking NATS PVCs"
NATS_PVCS=$(kubectl get pvc -n "$NAMESPACE" | grep nats || echo "")
if [ -z "$NATS_PVCS" ]; then
    log_error "No NATS PVCs found"
    exit 1
fi

echo "$NATS_PVCS"
echo ""

# Check if PVCs are bound
UNBOUND_PVCS=$(kubectl get pvc -n "$NAMESPACE" | grep nats | grep -v Bound || echo "")
if [ -n "$UNBOUND_PVCS" ]; then
    log_error "Some NATS PVCs are not Bound"
    echo "$UNBOUND_PVCS"
    log_info "This may cause storage issues"
fi

# Step 3: Check NATS cluster status
log_section "Step 3: Checking NATS Cluster Status"
NATS_POD_0=$(kubectl get pods -n "$NAMESPACE" -l app=nats -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -n "$NATS_POD_0" ]; then
    log_info "Checking NATS cluster status from $NATS_POD_0..."
    
    # Check if NATS is ready
    NATS_READY=$(kubectl get pod -n "$NAMESPACE" "$NATS_POD_0" -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || echo "Unknown")
    if [ "$NATS_READY" = "True" ]; then
        log_info "NATS pod is Ready, checking JetStream status..."
        
        # Try to get JetStream info (if nats-box or similar tool available)
        # For now, just check if we can connect
        log_info "NATS pod is ready, but cannot directly check JetStream status without nats CLI"
        log_info "You can check manually: kubectl exec -n $NAMESPACE $NATS_POD_0 -- nats stream ls"
    else
        log_warning "NATS pod is not Ready (status: $NATS_READY)"
    fi
else
    log_warning "No NATS pods found to check status"
fi

# Step 4: Check NATS logs
log_section "Step 4: Checking NATS Logs (last 20 lines)"
if [ -n "$NATS_POD_0" ]; then
    kubectl logs -n "$NAMESPACE" "$NATS_POD_0" --tail=20 2>/dev/null | tail -10 || log_warning "Could not get NATS logs"
else
    log_warning "No NATS pod found to check logs"
fi

# Step 5: Check storage usage
log_section "Step 5: Checking Storage Usage"
if [ -n "$NATS_POD_0" ]; then
    log_info "Checking disk usage in NATS pod..."
    kubectl exec -n "$NAMESPACE" "$NATS_POD_0" -- df -h /data/jetstream 2>/dev/null || log_warning "Could not check disk usage"
fi

# Step 6: Fix options
log_section "Step 6: Fix Options"

# Option 1: Restart NATS StatefulSet
log_info "Option 1: Restarting NATS StatefulSet..."
kubectl rollout restart statefulset -n "$NAMESPACE" nats 2>/dev/null || log_warning "Could not restart NATS StatefulSet"

log_info "Waiting for NATS pods to restart (30 seconds)..."
sleep 30

# Check NATS pods after restart
NATS_RUNNING_AFTER=$(kubectl get pods -n "$NAMESPACE" -l app=nats --no-headers 2>/dev/null | grep -c Running || echo "0")
log_info "NATS pods after restart: $NATS_RUNNING_AFTER/$NATS_TOTAL Running"

# Step 7: Check Core pod
log_section "Step 7: Checking Core Pod Status"
CORE_POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -n "$CORE_POD" ]; then
    CORE_STATUS=$(kubectl get pod -n "$NAMESPACE" "$CORE_POD" -o jsonpath='{.status.phase}' 2>/dev/null || echo "Unknown")
    log_info "Core pod: $CORE_POD (Status: $CORE_STATUS)"
    
    if [ "$CORE_STATUS" = "Running" ]; then
        log_info "Checking Core logs for NATS connection..."
        kubectl logs -n "$NAMESPACE" "$CORE_POD" --tail=30 2>/dev/null | grep -i "nats\|stream" | tail -10 || true
    fi
fi

# Step 8: Recommendations
log_section "Step 8: Recommendations"

if [ "$NATS_RUNNING" -lt 3 ]; then
    log_warning "NATS cluster has less than 3 running pods"
    log_info "Fix: Wait for all NATS pods to be Running"
    log_info "  kubectl get pods -n $NAMESPACE -l app=nats -w"
fi

if [ -n "$UNBOUND_PVCS" ]; then
    log_warning "Some NATS PVCs are not Bound"
    log_info "Fix: Check PVC status and ensure StorageClass is available"
    log_info "  kubectl get pvc -n $NAMESPACE"
    log_info "  kubectl describe pvc -n $NAMESPACE <pvc-name>"
fi

log_info "If NATS storage issue persists, possible solutions:"
log_info "  1. Increase storage in NATS PVCs (edit StatefulSet volumeClaimTemplates)"
log_info "  2. Reduce NATS cluster replicas from 3 to 1 (for testing)"
log_info "  3. Check NATS cluster quorum (all pods must be ready)"
log_info "  4. Verify network connectivity between NATS pods"

# Step 9: Quick fix - Reduce NATS replicas (if needed)
log_section "Step 9: Quick Fix - Reduce NATS Replicas (Optional)"

read -p "Do you want to reduce NATS replicas from 3 to 1 for testing? (y/N): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    log_info "Scaling NATS StatefulSet to 1 replica..."
    kubectl scale statefulset -n "$NAMESPACE" nats --replicas=1 2>/dev/null || log_error "Failed to scale NATS"
    
    log_info "Waiting for NATS to scale down..."
    kubectl wait --for=delete pod -n "$NAMESPACE" -l app=nats --timeout=60s 2>/dev/null || true
    
    log_info "NATS scaled to 1 replica. Restart Core to retry stream creation."
    log_info "  kubectl rollout restart deployment -n $NAMESPACE fortuna-core"
else
    log_info "Skipping replica reduction"
fi

# Step 10: Summary
log_section "Step 10: Summary"
log_info "Actions taken:"
log_info "  1. ✅ Checked NATS pods status"
log_info "  2. ✅ Checked NATS PVCs status"
log_info "  3. ✅ Restarted NATS StatefulSet"
echo ""
log_info "Next steps:"
log_info "  1. Wait for all NATS pods to be Running: kubectl get pods -n $NAMESPACE -l app=nats -w"
log_info "  2. Restart Core: kubectl rollout restart deployment -n $NAMESPACE fortuna-core"
log_info "  3. Monitor Core logs: kubectl logs -n $NAMESPACE -l app.kubernetes.io/component=core -f"
echo ""

