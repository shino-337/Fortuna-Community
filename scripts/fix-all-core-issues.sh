#!/bin/bash

# ============================================================================
# Fix All Core Issues - Comprehensive Fix
# ============================================================================
# Fixes all Core deployment issues: multiple deployments, DNS, configuration
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

log_section "Comprehensive Core Issues Fix"

# Step 1: Analyze current state
log_section "Step 1: Analyzing Current State"
log_info "Checking all Core deployments..."
ALL_CORE_DEPLOYS=$(kubectl get deployment -n "$NAMESPACE" -o jsonpath='{.items[*].metadata.name}' 2>/dev/null | tr ' ' '\n' | grep -i core || echo "")
if [ -n "$ALL_CORE_DEPLOYS" ]; then
    log_warning "Found multiple Core deployments:"
    for deploy in $ALL_CORE_DEPLOYS; do
        REPLICAS=$(kubectl get deployment -n "$NAMESPACE" "$deploy" -o jsonpath='{.spec.replicas}' 2>/dev/null || echo "?")
        READY=$(kubectl get deployment -n "$NAMESPACE" "$deploy" -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
        log_info "  - $deploy (replicas: $REPLICAS, ready: $READY)"
    done
else
    log_info "No Core deployments found"
fi

log_info "Checking all Core pods..."
ALL_CORE_PODS=$(kubectl get pods -n "$NAMESPACE" -o jsonpath='{.items[*].metadata.name}' 2>/dev/null | tr ' ' '\n' | grep -i core || echo "")
if [ -n "$ALL_CORE_PODS" ]; then
    log_warning "Found Core pods:"
    for pod in $ALL_CORE_PODS; do
        STATUS=$(kubectl get pod -n "$NAMESPACE" "$pod" -o jsonpath='{.status.phase}' 2>/dev/null || echo "Unknown")
        READY=$(kubectl get pod -n "$NAMESPACE" "$pod" -o jsonpath='{.status.containerStatuses[0].ready}' 2>/dev/null || echo "false")
        NODE=$(kubectl get pod -n "$NAMESPACE" "$pod" -o jsonpath='{.spec.nodeName}' 2>/dev/null || echo "Unknown")
        log_info "  - $pod (status: $STATUS, ready: $READY, node: $NODE)"
    done
fi

# Step 2: Delete ALL existing Core deployments and pods
log_section "Step 2: Deleting ALL Existing Core Resources"
log_warning "This will delete all Core deployments and pods..."

# Delete all deployments with "core" in name
for deploy in $ALL_CORE_DEPLOYS; do
    log_info "Deleting deployment: $deploy"
    kubectl delete deployment -n "$NAMESPACE" "$deploy" --wait=false 2>/dev/null || true
done

# Delete all pods with "core" in name
for pod in $ALL_CORE_PODS; do
    log_info "Deleting pod: $pod"
    kubectl delete pod -n "$NAMESPACE" "$pod" --wait=false 2>/dev/null || true
done

# Delete by labels
log_info "Deleting by labels..."
kubectl delete deployment -n "$NAMESPACE" -l app.kubernetes.io/component=core --wait=false 2>/dev/null || true
kubectl delete pods -n "$NAMESPACE" -l app.kubernetes.io/component=core --wait=false 2>/dev/null || true
kubectl delete deployment -n "$NAMESPACE" -l app=ksam-core --wait=false 2>/dev/null || true
kubectl delete deployment -n "$NAMESPACE" -l app=fortuna-core --wait=false 2>/dev/null || true

log_info "Waiting for resources to be deleted (15 seconds)..."
sleep 15

# Verify deletion
REMAINING_PODS=$(kubectl get pods -n "$NAMESPACE" -o jsonpath='{.items[*].metadata.name}' 2>/dev/null | tr ' ' '\n' | grep -i core | wc -l || echo "0")
if [ "$REMAINING_PODS" -gt 0 ]; then
    log_warning "Still have $REMAINING_PODS Core pods, forcing deletion..."
    kubectl delete pods -n "$NAMESPACE" -l app.kubernetes.io/component=core --force --grace-period=0 2>/dev/null || true
    sleep 5
fi

log_success "All old Core resources deleted"

# Step 3: Get PostgreSQL IP
log_section "Step 3: Getting PostgreSQL Service IP"
POSTGRES_IP=$(kubectl get svc -n "$NAMESPACE" postgres -o jsonpath='{.spec.clusterIP}' 2>/dev/null || echo "")
if [ -z "$POSTGRES_IP" ]; then
    log_error "PostgreSQL service not found"
    exit 1
fi
log_success "PostgreSQL ClusterIP: $POSTGRES_IP"

# Step 4: Update deployment file with IP (temporary)
log_section "Step 4: Preparing Core Deployment"
log_info "Checking if deployment file exists..."
if [ ! -f "deploy/fortuna-core-deployment.yaml" ]; then
    log_error "Core deployment file not found: deploy/fortuna-core-deployment.yaml"
    exit 1
fi

# Create a temporary deployment file with IP
TEMP_DEPLOYMENT="/tmp/fortuna-core-deployment-ip.yaml"
log_info "Creating temporary deployment file with IP..."
cp deploy/fortuna-core-deployment.yaml "$TEMP_DEPLOYMENT"

# Replace DATABASE_URL in temp file
if command -v sed >/dev/null 2>&1; then
    sed -i "s|postgres://postgres:postgres@postgres.fortuna.svc.cluster.local:5432/ksam|postgres://postgres:postgres@${POSTGRES_IP}:5432/ksam|g" "$TEMP_DEPLOYMENT"
    log_success "Updated DATABASE_URL in temporary file"
else
    log_warning "sed not available, will update via kubectl set env"
fi

# Step 5: Apply Core deployment
log_section "Step 5: Applying Core Deployment"
log_info "Applying Core deployment..."
kubectl apply -f "$TEMP_DEPLOYMENT" || {
    log_error "Failed to apply Core deployment"
    exit 1
}

# Update DATABASE_URL via kubectl (more reliable)
log_info "Updating DATABASE_URL via kubectl..."
NEW_DB_URL="postgres://postgres:postgres@${POSTGRES_IP}:5432/ksam?sslmode=disable"
kubectl set env deployment/fortuna-core -n "$NAMESPACE" DATABASE_URL="$NEW_DB_URL" 2>/dev/null || {
    log_warning "kubectl set env failed, trying patch..."
    kubectl patch deployment -n "$NAMESPACE" fortuna-core -p "{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"core\",\"env\":[{\"name\":\"DATABASE_URL\",\"value\":\"$NEW_DB_URL\"}]}]}}}}" || {
        log_error "Failed to update DATABASE_URL"
        exit 1
    }
}

log_success "Core deployment applied with IP-based DATABASE_URL"

# Step 6: Wait and verify
log_section "Step 6: Waiting for Core Pod"
log_info "Waiting for Core pod to be created (20 seconds)..."
sleep 20

# Check pod status
CORE_POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -z "$CORE_POD" ]; then
    log_error "No Core pod found after deployment"
    log_info "Checking deployments..."
    kubectl get deployment -n "$NAMESPACE" | grep core
    exit 1
fi

log_info "Core pod: $CORE_POD"

# Step 7: Monitor Core pod
log_section "Step 7: Monitoring Core Pod"
for i in {1..6}; do
    CORE_STATUS=$(kubectl get pod -n "$NAMESPACE" "$CORE_POD" -o jsonpath='{.status.phase}' 2>/dev/null || echo "Unknown")
    CORE_READY=$(kubectl get pod -n "$NAMESPACE" "$CORE_POD" -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || echo "Unknown")
    RESTARTS=$(kubectl get pod -n "$NAMESPACE" "$CORE_POD" -o jsonpath='{.status.containerStatuses[0].restartCount}' 2>/dev/null || echo "0")
    
    log_info "Status check $i/6: Phase=$CORE_STATUS, Ready=$CORE_READY, Restarts=$RESTARTS"
    
    if [ "$CORE_STATUS" = "Running" ] && [ "$CORE_READY" = "True" ]; then
        log_success "Core pod is Running and Ready! ✅"
        break
    fi
    
    if [ "$CORE_STATUS" = "CrashLoopBackOff" ] || [ "$RESTARTS" -gt 3 ]; then
        log_warning "Core pod is in bad state, checking logs..."
        kubectl logs -n "$NAMESPACE" "$CORE_POD" --tail=30 2>/dev/null | grep -i "error\|fatal\|database" | tail -10 || true
    fi
    
    if [ $i -lt 6 ]; then
        sleep 10
    fi
done

# Step 8: Final status
log_section "Step 8: Final Status Check"
kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core -o wide

CORE_STATUS=$(kubectl get pod -n "$NAMESPACE" "$CORE_POD" -o jsonpath='{.status.phase}' 2>/dev/null || echo "Unknown")
CORE_READY=$(kubectl get pod -n "$NAMESPACE" "$CORE_POD" -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || echo "Unknown")

if [ "$CORE_STATUS" = "Running" ] && [ "$CORE_READY" = "True" ]; then
    log_success "✅ Core is Running and Ready!"
else
    log_warning "Core is not fully ready"
    log_info "Check logs: kubectl logs -n $NAMESPACE $CORE_POD"
    log_info "Check events: kubectl describe pod -n $NAMESPACE $CORE_POD"
fi

# Step 9: Verify only one deployment
log_section "Step 9: Verifying Single Deployment"
DEPLOY_COUNT=$(kubectl get deployment -n "$NAMESPACE" -l app.kubernetes.io/component=core --no-headers 2>/dev/null | wc -l || echo "0")
if [ "$DEPLOY_COUNT" -eq 1 ]; then
    log_success "Only one Core deployment exists ✅"
else
    log_warning "Found $DEPLOY_COUNT Core deployments (expected 1)"
    kubectl get deployment -n "$NAMESPACE" | grep core
fi

# Step 10: Summary
log_section "Step 10: Summary"
log_info "Actions completed:"
log_info "  1. ✅ Analyzed current state"
log_info "  2. ✅ Deleted all old Core deployments and pods"
log_info "  3. ✅ Got PostgreSQL IP: $POSTGRES_IP"
log_info "  4. ✅ Applied Core deployment with IP-based DATABASE_URL"
log_info "  5. ✅ Monitored Core pod status"
echo ""
log_info "Current state:"
log_info "  - Core pod: $CORE_POD"
log_info "  - Status: $CORE_STATUS"
log_info "  - Ready: $CORE_READY"
echo ""
log_info "Next steps:"
log_info "  1. Monitor Core logs: kubectl logs -n $NAMESPACE $CORE_POD -f"
log_info "  2. Check service endpoints: kubectl get endpoints -n $NAMESPACE fortuna-core"
log_info "  3. Test Agent connection: kubectl logs -n $NAMESPACE -l app.kubernetes.io/component=agent | grep -i core"
echo ""

# Cleanup temp file
rm -f "$TEMP_DEPLOYMENT"

