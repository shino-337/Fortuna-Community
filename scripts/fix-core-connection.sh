#!/bin/bash

# ============================================================================
# Fix Core gRPC Connection Issues
# ============================================================================
# Fixes common issues preventing Agent from connecting to Core gRPC
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

log_section "Diagnosing Core gRPC Connection Issue"

# 1. Check Core pod status
log_section "1. Checking Core Pod Status"
CORE_POD_NAME=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -z "$CORE_POD_NAME" ]; then
    log_error "No Core pod found"
    exit 1
fi

log_info "Core pod: $CORE_POD_NAME"
CORE_POD_STATUS=$(kubectl get pod -n "$NAMESPACE" "$CORE_POD_NAME" -o jsonpath='{.status.phase}' 2>/dev/null || echo "Unknown")
CORE_POD_READY=$(kubectl get pod -n "$NAMESPACE" "$CORE_POD_NAME" -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || echo "Unknown")

log_info "Status: $CORE_POD_STATUS"
log_info "Ready: $CORE_POD_READY"

if [ "$CORE_POD_STATUS" != "Running" ]; then
    log_error "Core pod is not Running"
    log_info "Checking pod events..."
    kubectl describe pod -n "$NAMESPACE" "$CORE_POD_NAME" | grep -A 10 "Events:" || true
    exit 1
fi

# 2. Check Core logs for gRPC server startup
log_section "2. Checking Core Logs for gRPC Server"
log_info "Searching for gRPC server startup messages..."
GRPC_LOG=$(kubectl logs -n "$NAMESPACE" "$CORE_POD_NAME" 2>/dev/null | grep -i "gRPC\|grpc" | tail -20 || echo "")
if [ -z "$GRPC_LOG" ]; then
    log_warning "No gRPC-related logs found"
    log_info "Showing last 30 lines of Core logs..."
    kubectl logs -n "$NAMESPACE" "$CORE_POD_NAME" --tail=30
else
    echo "$GRPC_LOG"
fi

# 3. Check if gRPC port is listening
log_section "3. Checking if gRPC Port 9090 is Listening"
log_info "Checking port 9090 in Core pod..."
PORT_CHECK=$(kubectl exec -n "$NAMESPACE" "$CORE_POD_NAME" -- sh -c "netstat -tlnp 2>/dev/null | grep ':9090' || ss -tlnp 2>/dev/null | grep ':9090' || echo 'Port check failed'" 2>/dev/null || echo "Exec failed")
if echo "$PORT_CHECK" | grep -q ":9090"; then
    log_success "Port 9090 is listening"
    echo "  $PORT_CHECK"
else
    log_error "Port 9090 is NOT listening"
    echo "  $PORT_CHECK"
fi

# 4. Check service endpoints
log_section "4. Checking Service Endpoints"
ENDPOINTS=$(kubectl get endpoints -n "$NAMESPACE" fortuna-core -o jsonpath='{.subsets[0].addresses[*].ip}' 2>/dev/null || echo "")
if [ -z "$ENDPOINTS" ]; then
    log_error "No service endpoints found"
    log_info "This means Core pod is not Ready (readiness probe failing)"
    
    # Check readiness probe
    log_info "Checking readiness probe configuration..."
    READINESS_PROBE=$(kubectl get deployment -n "$NAMESPACE" fortuna-core -o jsonpath='{.spec.template.spec.containers[0].readinessProbe}' 2>/dev/null || echo "")
    if [ -z "$READINESS_PROBE" ] || [ "$READINESS_PROBE" = "null" ]; then
        log_warning "No readiness probe configured"
        log_info "Core pod may be Running but not Ready, causing no endpoints"
    else
        log_info "Readiness probe is configured"
        echo "$READINESS_PROBE" | head -5
    fi
else
    log_success "Service endpoints: $ENDPOINTS"
fi

# 5. Check Core container status
log_section "5. Checking Container Status"
CONTAINER_STATUS=$(kubectl get pod -n "$NAMESPACE" "$CORE_POD_NAME" -o jsonpath='{.status.containerStatuses[0].state}' 2>/dev/null || echo "")
log_info "Container state: $CONTAINER_STATUS"

# Check if container is waiting or crashed
WAITING_REASON=$(kubectl get pod -n "$NAMESPACE" "$CORE_POD_NAME" -o jsonpath='{.status.containerStatuses[0].state.waiting.reason}' 2>/dev/null || echo "")
if [ -n "$WAITING_REASON" ] && [ "$WAITING_REASON" != "null" ]; then
    log_error "Container is waiting: $WAITING_REASON"
    WAITING_MESSAGE=$(kubectl get pod -n "$NAMESPACE" "$CORE_POD_NAME" -o jsonpath='{.status.containerStatuses[0].state.waiting.message}' 2>/dev/null || echo "")
    if [ -n "$WAITING_MESSAGE" ]; then
        log_info "Message: $WAITING_MESSAGE"
    fi
fi

# 6. Recommendations
log_section "6. Recommendations and Fixes"

if [ "$CORE_POD_READY" != "True" ]; then
    log_warning "Core pod is not Ready"
    log_info "Possible causes:"
    log_info "  1. Readiness probe failing (check if HTTP port 8080 is responding)"
    log_info "  2. gRPC server not starting (check logs for errors)"
    log_info "  3. Database connection failing"
    log_info "  4. NATS connection failing"
    echo ""
    log_info "Check readiness probe:"
    log_info "  kubectl describe pod -n $NAMESPACE $CORE_POD_NAME | grep -A 10 Readiness"
    echo ""
    log_info "Test HTTP endpoint manually:"
    log_info "  kubectl exec -n $NAMESPACE $CORE_POD_NAME -- wget -qO- http://localhost:8080/health || echo 'HTTP not responding'"
fi

if [ -z "$ENDPOINTS" ]; then
    log_warning "No service endpoints means Agent cannot connect"
    log_info "Fix: Ensure Core pod becomes Ready"
    log_info "  - Check Core logs for startup errors"
    log_info "  - Verify database connection"
    log_info "  - Verify NATS connection"
    log_info "  - Check if gRPC server starts successfully"
fi

# 7. Show full Core logs if needed
log_section "7. Full Core Logs (last 50 lines)"
log_info "If needed, check full logs with: kubectl logs -n $NAMESPACE $CORE_POD_NAME"
kubectl logs -n "$NAMESPACE" "$CORE_POD_NAME" --tail=50 2>/dev/null || log_warning "Could not get logs"

echo ""
log_info "To monitor Core logs in real-time:"
log_info "  kubectl logs -n $NAMESPACE $CORE_POD_NAME -f"
echo ""

