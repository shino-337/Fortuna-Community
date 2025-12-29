#!/bin/bash

# ============================================================================
# Check Core gRPC Connection
# ============================================================================
# Diagnoses why Agent cannot connect to Core gRPC endpoint
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

# 1. Check Core pod status
log_section "1. Core Pod Status"
CORE_PODS=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core 2>/dev/null || echo "")
if [ -z "$CORE_PODS" ]; then
    log_error "No Core pods found"
    exit 1
fi

echo "$CORE_PODS"
echo ""

CORE_POD_NAME=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
CORE_POD_STATUS=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core -o jsonpath='{.items[0].status.phase}' 2>/dev/null || echo "Unknown")

if [ "$CORE_POD_STATUS" = "Running" ]; then
    log_success "Core pod is Running: $CORE_POD_NAME"
else
    log_error "Core pod is not Running. Status: $CORE_POD_STATUS"
    log_info "Check pod: kubectl describe pod -n $NAMESPACE $CORE_POD_NAME"
    log_info "Check logs: kubectl logs -n $NAMESPACE $CORE_POD_NAME"
fi

# 2. Check Core service
log_section "2. Core Service"
CORE_SVC=$(kubectl get svc -n "$NAMESPACE" fortuna-core 2>/dev/null || echo "")
if [ -z "$CORE_SVC" ]; then
    log_error "Core service not found"
else
    echo "$CORE_SVC"
    echo ""
    
    # Get service endpoints
    ENDPOINTS=$(kubectl get endpoints -n "$NAMESPACE" fortuna-core -o jsonpath='{.subsets[0].addresses[*].ip}' 2>/dev/null || echo "")
    if [ -z "$ENDPOINTS" ]; then
        log_error "No endpoints found for Core service"
        log_info "This means no Core pods are ready"
    else
        log_success "Service endpoints: $ENDPOINTS"
    fi
fi

# 3. Check Core logs
log_section "3. Core Logs (last 20 lines)"
if [ -n "$CORE_POD_NAME" ]; then
    kubectl logs -n "$NAMESPACE" "$CORE_POD_NAME" --tail=20 2>/dev/null || log_warning "Could not get logs"
else
    log_warning "No Core pod found to check logs"
fi

# 4. Check if gRPC server is listening
log_section "4. Core gRPC Server Check"
if [ -n "$CORE_POD_NAME" ] && [ "$CORE_POD_STATUS" = "Running" ]; then
    log_info "Checking if gRPC server is listening on port 9090..."
    
    # Check if port 9090 is listening in pod
    PORT_CHECK=$(kubectl exec -n "$NAMESPACE" "$CORE_POD_NAME" -- netstat -tlnp 2>/dev/null | grep ":9090" || echo "")
    if [ -n "$PORT_CHECK" ]; then
        log_success "Port 9090 is listening"
        echo "  $PORT_CHECK"
    else
        log_warning "Port 9090 may not be listening"
        log_info "Checking with ss (if available)..."
        kubectl exec -n "$NAMESPACE" "$CORE_POD_NAME" -- ss -tlnp 2>/dev/null | grep ":9090" || log_warning "Could not verify port 9090"
    fi
fi

# 5. Test connectivity from Agent pod
log_section "5. Network Connectivity Test"
AGENT_POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=agent -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -n "$AGENT_POD" ]; then
    log_info "Testing connectivity from Agent pod: $AGENT_POD"
    
    # Test DNS resolution
    log_info "Testing DNS resolution..."
    DNS_RESULT=$(kubectl exec -n "$NAMESPACE" "$AGENT_POD" -- nslookup fortuna-core.fortuna.svc.cluster.local 2>/dev/null || echo "Failed")
    if echo "$DNS_RESULT" | grep -q "10."; then
        log_success "DNS resolution works"
        echo "$DNS_RESULT" | grep "Address" | head -2
    else
        log_error "DNS resolution failed"
    fi
    
    # Test port connectivity
    log_info "Testing port 9090 connectivity..."
    PORT_TEST=$(kubectl exec -n "$NAMESPACE" "$AGENT_POD" -- nc -zv fortuna-core.fortuna.svc.cluster.local 9090 2>&1 || echo "Failed")
    if echo "$PORT_TEST" | grep -q "succeeded\|open"; then
        log_success "Port 9090 is reachable"
    else
        log_error "Port 9090 is NOT reachable"
        echo "  $PORT_TEST"
    fi
else
    log_warning "No Agent pod found for connectivity test"
fi

# 6. Check Core service endpoints
log_section "6. Service Endpoints Details"
kubectl get endpoints -n "$NAMESPACE" fortuna-core -o yaml 2>/dev/null | grep -A 10 "subsets:" || log_warning "No endpoints details"

# 7. Recommendations
log_section "7. Recommendations"

if [ "$CORE_POD_STATUS" != "Running" ]; then
    log_warning "Core pod is not Running"
    log_info "Fix: Check Core pod logs and events"
    log_info "  kubectl logs -n $NAMESPACE $CORE_POD_NAME"
    log_info "  kubectl describe pod -n $NAMESPACE $CORE_POD_NAME"
fi

if [ -z "$ENDPOINTS" ]; then
    log_warning "No service endpoints"
    log_info "Fix: Ensure Core pod is Ready (not just Running)"
    log_info "  Check readiness probe: kubectl describe pod -n $NAMESPACE $CORE_POD_NAME | grep -A 5 Readiness"
fi

# Check if Core is actually listening
if [ -n "$CORE_POD_NAME" ]; then
    READY=$(kubectl get pod -n "$NAMESPACE" "$CORE_POD_NAME" -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || echo "Unknown")
    if [ "$READY" != "True" ]; then
        log_warning "Core pod is not Ready (status: $READY)"
        log_info "Pod may be Running but not passing readiness probe"
        log_info "Check: kubectl describe pod -n $NAMESPACE $CORE_POD_NAME"
    fi
fi

echo ""

