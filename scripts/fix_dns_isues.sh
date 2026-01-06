#!/bin/bash

# ============================================================================
# Fix DNS Issue for Core Pod
# ============================================================================
# Diagnoses and fixes DNS resolution issues preventing Core from starting
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

log_section "DNS Issue Diagnosis and Fix"

# Step 1: Check CoreDNS pods
log_section "Step 1: Checking CoreDNS Pods"
COREDNS_PODS=$(kubectl get pods -n kube-system -l k8s-app=kube-dns 2>/dev/null || echo "")
if [ -z "$COREDNS_PODS" ]; then
    log_error "No CoreDNS pods found"
    exit 1
fi

echo "$COREDNS_PODS"
echo ""

COREDNS_COUNT=$(kubectl get pods -n kube-system -l k8s-app=kube-dns --no-headers 2>/dev/null | grep -c Running || echo "0")
if [ "$COREDNS_COUNT" -lt 1 ]; then
    log_error "No CoreDNS pods are Running"
    log_info "Checking CoreDNS logs..."
    kubectl logs -n kube-system -l k8s-app=kube-dns --tail=20 2>/dev/null || true
    exit 1
fi

log_success "CoreDNS pods are Running ($COREDNS_COUNT pods)"

# Step 2: Check CoreDNS service
log_section "Step 2: Checking CoreDNS Service"
COREDNS_SVC=$(kubectl get svc -n kube-system kube-dns 2>/dev/null || echo "")
if [ -z "$COREDNS_SVC" ]; then
    log_error "CoreDNS service not found"
    exit 1
fi

echo "$COREDNS_SVC"
echo ""

COREDNS_IP=$(kubectl get svc -n kube-system kube-dns -o jsonpath='{.spec.clusterIP}' 2>/dev/null || echo "")
if [ -z "$COREDNS_IP" ]; then
    log_error "Could not get CoreDNS ClusterIP"
    exit 1
fi

log_info "CoreDNS ClusterIP: $COREDNS_IP"

# Step 3: Check CoreDNS endpoints
log_section "Step 3: Checking CoreDNS Endpoints"
COREDNS_ENDPOINTS=$(kubectl get endpoints -n kube-system kube-dns -o jsonpath='{.subsets[0].addresses[*].ip}' 2>/dev/null || echo "")
if [ -z "$COREDNS_ENDPOINTS" ]; then
    log_error "No CoreDNS endpoints found"
    log_info "CoreDNS pods may not be ready"
    exit 1
fi

log_success "CoreDNS endpoints: $COREDNS_ENDPOINTS"

# Step 4: Test DNS resolution from a test pod or node
log_section "Step 4: Testing DNS Resolution"
log_info "Creating a test pod to check DNS resolution..."

# Create a temporary test pod
kubectl run dns-test --image=busybox:1.36 --rm -i --restart=Never -- nslookup postgres.fortuna.svc.cluster.local 2>&1 | head -20 || {
    log_warning "Could not create test pod, trying alternative method..."
    
    # Try to use an existing pod
    TEST_POD=$(kubectl get pods -n "$NAMESPACE" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null | head -1 || echo "")
    if [ -n "$TEST_POD" ]; then
        log_info "Using existing pod: $TEST_POD"
        kubectl exec -n "$NAMESPACE" "$TEST_POD" -- nslookup postgres.fortuna.svc.cluster.local 2>&1 || log_warning "DNS resolution test failed"
    else
        log_warning "No pods available for DNS test"
    fi
}

# Step 5: Check Core pod status
log_section "Step 5: Checking Core Pod Status"
CORE_POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -z "$CORE_POD" ]; then
    log_error "No Core pod found"
    exit 1
fi

CORE_STATUS=$(kubectl get pod -n "$NAMESPACE" "$CORE_POD" -o jsonpath='{.status.phase}' 2>/dev/null || echo "Unknown")
log_info "Core pod: $CORE_POD (Status: $CORE_STATUS)"

if [ "$CORE_STATUS" = "CrashLoopBackOff" ] || [ "$CORE_STATUS" = "Error" ]; then
    log_warning "Core pod is in $CORE_STATUS state"
    log_info "Checking Core logs for DNS errors..."
    kubectl logs -n "$NAMESPACE" "$CORE_POD" --tail=50 2>/dev/null | grep -i "dns\|resolve\|timeout" || true
fi

# Step 6: Get PostgreSQL service IP (workaround)
log_section "Step 6: Getting PostgreSQL Service IP"
POSTGRES_IP=$(kubectl get svc -n "$NAMESPACE" postgres -o jsonpath='{.spec.clusterIP}' 2>/dev/null || echo "")
if [ -z "$POSTGRES_IP" ]; then
    log_error "PostgreSQL service not found or has no ClusterIP"
    log_info "Checking if PostgreSQL service exists..."
    kubectl get svc -n "$NAMESPACE" | grep postgres || log_error "PostgreSQL service not found"
    exit 1
fi

log_success "PostgreSQL ClusterIP: $POSTGRES_IP"

# Step 7: Check current DATABASE_URL
log_section "Step 7: Checking Current DATABASE_URL"
CURRENT_DB_URL=$(kubectl get deployment -n "$NAMESPACE" fortuna-core -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="DATABASE_URL")].value}' 2>/dev/null || echo "")
if [ -n "$CURRENT_DB_URL" ]; then
    log_info "Current DATABASE_URL: $CURRENT_DB_URL"
else
    log_warning "DATABASE_URL not found in deployment"
fi

# Step 8: Fix DNS issue - Option 1: Restart CoreDNS
log_section "Step 8: Attempting to Fix DNS Issue"

# Option 1: Restart CoreDNS
log_info "Option 1: Restarting CoreDNS..."
kubectl rollout restart deployment -n kube-system coredns 2>/dev/null || log_warning "Could not restart CoreDNS deployment"

log_info "Waiting for CoreDNS to be ready..."
kubectl wait --for=condition=ready pod -n kube-system -l k8s-app=kube-dns --timeout=60s 2>/dev/null || log_warning "CoreDNS not ready within timeout"

# Step 9: Workaround - Use IP directly
log_section "Step 9: Applying Workaround - Using PostgreSQL IP Directly"
NEW_DB_URL="postgres://postgres:postgres@${POSTGRES_IP}:5432/ksam?sslmode=disable"

log_info "Updating DATABASE_URL to use IP: $NEW_DB_URL"
kubectl set env deployment/fortuna-core -n "$NAMESPACE" DATABASE_URL="$NEW_DB_URL" 2>/dev/null || {
    log_error "Failed to update DATABASE_URL"
    log_info "Trying alternative method with patch..."
    
    # Alternative: Use patch
    kubectl patch deployment -n "$NAMESPACE" fortuna-core -p "{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"core\",\"env\":[{\"name\":\"DATABASE_URL\",\"value\":\"$NEW_DB_URL\"}]}]}}}}" 2>/dev/null || {
        log_error "Failed to patch deployment"
        exit 1
    }
}

log_success "DATABASE_URL updated successfully"

# Step 10: Restart Core deployment
log_section "Step 10: Restarting Core Deployment"
log_info "Restarting Core deployment to apply changes..."
kubectl rollout restart deployment -n "$NAMESPACE" fortuna-core 2>/dev/null || {
    log_error "Failed to restart Core deployment"
    exit 1
}

log_info "Waiting for Core deployment to restart..."
sleep 5

# Step 11: Monitor Core pod
log_section "Step 11: Monitoring Core Pod Status"
log_info "Waiting for Core pod to start (30 seconds)..."
sleep 30

CORE_POD_NEW=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -n "$CORE_POD_NEW" ]; then
    CORE_STATUS_NEW=$(kubectl get pod -n "$NAMESPACE" "$CORE_POD_NEW" -o jsonpath='{.status.phase}' 2>/dev/null || echo "Unknown")
    log_info "New Core pod: $CORE_POD_NEW (Status: $CORE_STATUS_NEW)"
    
    if [ "$CORE_STATUS_NEW" = "Running" ]; then
        log_success "Core pod is Running!"
        
        # Check if it's Ready
        CORE_READY=$(kubectl get pod -n "$NAMESPACE" "$CORE_POD_NEW" -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || echo "Unknown")
        if [ "$CORE_READY" = "True" ]; then
            log_success "Core pod is Ready!"
        else
            log_warning "Core pod is Running but not Ready yet"
        fi
        
        # Show recent logs
        log_info "Recent Core logs:"
        kubectl logs -n "$NAMESPACE" "$CORE_POD_NEW" --tail=20 2>/dev/null | tail -10 || true
    else
        log_warning "Core pod status: $CORE_STATUS_NEW"
        log_info "Check logs: kubectl logs -n $NAMESPACE $CORE_POD_NEW"
    fi
else
    log_warning "Could not get new Core pod name"
fi

# Step 12: Summary
log_section "Step 12: Summary"
log_info "Actions taken:"
log_info "  1. ✅ Checked CoreDNS pods and service"
log_info "  2. ✅ Updated DATABASE_URL to use PostgreSQL IP directly"
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

