#!/bin/bash

# ============================================================================
# Fix Core Database DNS Issue
# ============================================================================
# Updates Core deployment to use PostgreSQL IP directly instead of DNS
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

log_section "Fixing Core Database DNS Issue"

# Step 1: Get PostgreSQL service IP
log_section "Step 1: Getting PostgreSQL Service IP"
POSTGRES_IP=$(kubectl get svc -n "$NAMESPACE" postgres -o jsonpath='{.spec.clusterIP}' 2>/dev/null || echo "")
if [ -z "$POSTGRES_IP" ]; then
    log_error "PostgreSQL service not found or has no ClusterIP"
    log_info "Checking if PostgreSQL service exists..."
    kubectl get svc -n "$NAMESPACE" | grep postgres || log_error "PostgreSQL service not found"
    exit 1
fi

log_success "PostgreSQL ClusterIP: $POSTGRES_IP"

# Step 2: Check current DATABASE_URL
log_section "Step 2: Checking Current DATABASE_URL"
CURRENT_DB_URL=$(kubectl get deployment -n "$NAMESPACE" fortuna-core -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="DATABASE_URL")].value}' 2>/dev/null || echo "")
if [ -n "$CURRENT_DB_URL" ]; then
    log_info "Current DATABASE_URL: $CURRENT_DB_URL"
    
    # Check if already using IP
    if echo "$CURRENT_DB_URL" | grep -q "$POSTGRES_IP"; then
        log_success "DATABASE_URL already uses IP: $POSTGRES_IP"
        log_info "No changes needed"
        exit 0
    fi
else
    log_warning "DATABASE_URL not found in deployment"
fi

# Step 3: Update DATABASE_URL to use IP
log_section "Step 3: Updating DATABASE_URL to Use IP"
NEW_DB_URL="postgres://postgres:postgres@${POSTGRES_IP}:5432/ksam?sslmode=disable"

log_info "Updating DATABASE_URL to use IP: $NEW_DB_URL"

# Try using kubectl set env first
if kubectl set env deployment/fortuna-core -n "$NAMESPACE" DATABASE_URL="$NEW_DB_URL" 2>/dev/null; then
    log_success "DATABASE_URL updated using kubectl set env"
else
    log_warning "kubectl set env failed, trying patch method..."
    
    # Alternative: Use patch
    kubectl patch deployment -n "$NAMESPACE" fortuna-core -p "{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"core\",\"env\":[{\"name\":\"DATABASE_URL\",\"value\":\"$NEW_DB_URL\"}]}]}}}}" || {
        log_error "Failed to patch deployment"
        exit 1
    }
    
    log_success "DATABASE_URL updated using patch"
fi

# Step 4: Restart Core deployment
log_section "Step 4: Restarting Core Deployment"
log_info "Restarting Core deployment to apply changes..."
kubectl rollout restart deployment -n "$NAMESPACE" fortuna-core 2>/dev/null || {
    log_error "Failed to restart Core deployment"
    exit 1
}

log_info "Waiting for Core deployment to restart (10 seconds)..."
sleep 10

# Step 5: Monitor Core pod
log_section "Step 5: Monitoring Core Pod Status"
log_info "Waiting for Core pod to start (30 seconds)..."
sleep 30

CORE_POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -n "$CORE_POD" ]; then
    CORE_STATUS=$(kubectl get pod -n "$NAMESPACE" "$CORE_POD" -o jsonpath='{.status.phase}' 2>/dev/null || echo "Unknown")
    log_info "Core pod: $CORE_POD (Status: $CORE_STATUS)"
    
    if [ "$CORE_STATUS" = "Running" ]; then
        log_success "Core pod is Running!"
        
        # Check if it's Ready
        CORE_READY=$(kubectl get pod -n "$NAMESPACE" "$CORE_POD" -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || echo "Unknown")
        if [ "$CORE_READY" = "True" ]; then
            log_success "Core pod is Ready!"
        else
            log_warning "Core pod is Running but not Ready yet"
        fi
        
        # Show recent logs
        log_info "Recent Core logs (checking for database connection):"
        kubectl logs -n "$NAMESPACE" "$CORE_POD" --tail=20 2>/dev/null | grep -i "database\|postgres\|connected\|error" | tail -10 || log_info "No database-related logs found"
    else
        log_warning "Core pod status: $CORE_STATUS"
        log_info "Check logs: kubectl logs -n $NAMESPACE $CORE_POD"
    fi
else
    log_warning "Could not get Core pod name"
fi

# Step 6: Summary
log_section "Step 6: Summary"
log_info "Actions taken:"
log_info "  1. ✅ Got PostgreSQL service IP: $POSTGRES_IP"
log_info "  2. ✅ Updated DATABASE_URL to use IP directly"
log_info "  3. ✅ Restarted Core deployment"
echo ""
log_info "Next steps:"
log_info "  1. Monitor Core logs: kubectl logs -n $NAMESPACE -l app.kubernetes.io/component=core -f"
log_info "  2. Check Core status: kubectl get pods -n $NAMESPACE -l app.kubernetes.io/component=core"
log_info "  3. Verify service endpoints: kubectl get endpoints -n $NAMESPACE fortuna-core"
echo ""
log_warning "Note: Using IP directly is a workaround. DNS issue should be fixed for production."
log_info "To fix DNS properly, check:"
log_info "  - Network connectivity between nodes"
log_info "  - CoreDNS logs: kubectl logs -n kube-system -l k8s-app=kube-dns"
log_info "  - Network policies blocking DNS traffic"
echo ""

