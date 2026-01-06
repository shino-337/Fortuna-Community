#!/bin/bash

# ============================================================================
# Check Agent-Core Connection
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

log_section "Agent-Core Connection Diagnosis"

# Step 1: Check Core pod status
log_section "Step 1: Checking Core Pod Status"
CORE_POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -z "$CORE_POD" ]; then
    log_error "No Core pod found"
    exit 1
fi

CORE_STATUS=$(kubectl get pod -n "$NAMESPACE" "$CORE_POD" -o jsonpath='{.status.phase}' 2>/dev/null || echo "Unknown")
CORE_READY=$(kubectl get pod -n "$NAMESPACE" "$CORE_POD" -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || echo "Unknown")
CORE_NODE=$(kubectl get pod -n "$NAMESPACE" "$CORE_POD" -o jsonpath='{.spec.nodeName}' 2>/dev/null || echo "Unknown")

log_info "Core pod: $CORE_POD"
log_info "Status: $CORE_STATUS"
log_info "Ready: $CORE_READY"
log_info "Node: $CORE_NODE"

if [ "$CORE_STATUS" != "Running" ]; then
    log_error "Core pod is not Running"
    exit 1
fi

if [ "$CORE_READY" != "True" ]; then
    log_warning "Core pod is not Ready (status: $CORE_READY)"
    log_info "This means service has no endpoints"
fi

# Step 2: Check Core service endpoints
log_section "Step 2: Checking Core Service Endpoints"
CORE_SVC=$(kubectl get svc -n "$NAMESPACE" fortuna-core 2>/dev/null || echo "")
if [ -z "$CORE_SVC" ]; then
    log_error "Core service not found"
    exit 1
fi

echo "$CORE_SVC"
echo ""

CORE_SVC_IP=$(kubectl get svc -n "$NAMESPACE" fortuna-core -o jsonpath='{.spec.clusterIP}' 2>/dev/null || echo "")
log_info "Core service ClusterIP: $CORE_SVC_IP"

ENDPOINTS=$(kubectl get endpoints -n "$NAMESPACE" fortuna-core -o jsonpath='{.subsets[0].addresses[*].ip}' 2>/dev/null || echo "")
if [ -z "$ENDPOINTS" ]; then
    log_error "No service endpoints found"
    log_info "This means Core pod is not Ready (readiness probe failing)"
    log_info "Check: kubectl describe pod -n $NAMESPACE $CORE_POD | grep -A 10 Readiness"
    exit 1
fi

log_success "Service endpoints: $ENDPOINTS"

# Step 3: Check Core logs for gRPC server
log_section "Step 3: Checking Core Logs for gRPC Server"
log_info "Searching for gRPC server startup..."
GRPC_LOG=$(kubectl logs -n "$NAMESPACE" "$CORE_POD" 2>/dev/null | grep -i "gRPC\|grpc" | tail -10 || echo "")
if [ -z "$GRPC_LOG" ]; then
    log_warning "No gRPC-related logs found"
else
    echo "$GRPC_LOG"
fi

# Check if gRPC server started successfully
if kubectl logs -n "$NAMESPACE" "$CORE_POD" 2>/dev/null | grep -q "gRPC server listening"; then
    log_success "gRPC server is listening"
else
    log_warning "gRPC server may not be listening"
fi

# Step 4: Check if gRPC port is listening in Core pod
log_section "Step 4: Checking if gRPC Port 9090 is Listening"
log_info "Checking port 9090 in Core pod..."
PORT_CHECK=$(kubectl exec -n "$NAMESPACE" "$CORE_POD" -- sh -c "netstat -tlnp 2>/dev/null | grep ':9090' || ss -tlnp 2>/dev/null | grep ':9090' || echo 'Port check failed'" 2>/dev/null || echo "Exec failed")
if echo "$PORT_CHECK" | grep -q ":9090"; then
    log_success "Port 9090 is listening"
    echo "  $PORT_CHECK"
else
    log_error "Port 9090 is NOT listening"
    echo "  $PORT_CHECK"
fi

# Step 5: Test connectivity from Agent pod
log_section "Step 5: Testing Connectivity from Agent Pod"
AGENT_POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=agent -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -n "$AGENT_POD" ]; then
    log_info "Testing from Agent pod: $AGENT_POD"
    
    # Test DNS resolution
    log_info "Testing DNS resolution..."
    DNS_RESULT=$(kubectl exec -n "$NAMESPACE" "$AGENT_POD" -- nslookup fortuna-core.fortuna.svc.cluster.local 2>/dev/null || echo "DNS failed")
    if echo "$DNS_RESULT" | grep -q "10."; then
        log_success "DNS resolution works"
        echo "$DNS_RESULT" | grep "Address" | head -2
    else
        log_error "DNS resolution failed"
        echo "$DNS_RESULT"
    fi
    
    # Test port connectivity
    log_info "Testing port 9090 connectivity..."
    PORT_TEST=$(kubectl exec -n "$NAMESPACE" "$AGENT_POD" -- nc -zv "$CORE_SVC_IP" 9090 2>&1 || echo "Port test failed")
    if echo "$PORT_TEST" | grep -q "succeeded\|open"; then
        log_success "Port 9090 is reachable"
    else
        log_error "Port 9090 is NOT reachable"
        echo "  $PORT_TEST"
    fi
else
    log_warning "No Agent pod found for connectivity test"
fi

# Step 6: Check Core container status
log_section "Step 6: Checking Core Container Status"
CONTAINER_STATUS=$(kubectl get pod -n "$NAMESPACE" "$CORE_POD" -o jsonpath='{.status.containerStatuses[0].state}' 2>/dev/null || echo "")
log_info "Container state: $CONTAINER_STATUS"

# Step 7: Recommendations
log_section "Step 7: Recommendations"

if [ "$CORE_READY" != "True" ]; then
    log_warning "Core pod is not Ready"
    log_info "Fix: Check Core logs for startup errors"
    log_info "  kubectl logs -n $NAMESPACE $CORE_POD --tail=100"
    log_info "Common issues:"
    log_info "  - Database connection failed"
    log_info "  - NATS connection failed"
    log_info "  - HTTP server not starting (readiness probe checks port 8080)"
fi

if [ -z "$ENDPOINTS" ]; then
    log_warning "No service endpoints"
    log_info "Fix: Ensure Core pod becomes Ready"
fi

if echo "$PORT_CHECK" | grep -qv ":9090"; then
    log_warning "gRPC port 9090 not listening"
    log_info "Fix: Check Core logs for gRPC server errors"
    log_info "  kubectl logs -n $NAMESPACE $CORE_POD | grep -i grpc"
fi

# Step 8: Show recent Core logs
log_section "Step 8: Recent Core Logs (last 30 lines)"
kubectl logs -n "$NAMESPACE" "$CORE_POD" --tail=30 2>/dev/null || log_warning "Could not get logs"

echo ""
log_info "To monitor Core logs: kubectl logs -n $NAMESPACE $CORE_POD -f"
echo ""

