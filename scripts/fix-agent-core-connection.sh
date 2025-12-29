#!/bin/bash

# ============================================================================
# Fix Agent-Core Connection (DNS Issue)
# ============================================================================
# Updates Agent DaemonSet to use Core service IP directly instead of DNS
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

log_section "Fixing Agent-Core Connection (DNS Issue)"

# Step 1: Check Core service and endpoints
log_section "Step 1: Checking Core Service"
CORE_SVC=$(kubectl get svc -n "$NAMESPACE" fortuna-core 2>/dev/null || echo "")
if [ -z "$CORE_SVC" ]; then
    log_error "Core service not found"
    exit 1
fi

echo "$CORE_SVC"
echo ""

CORE_SVC_IP=$(kubectl get svc -n "$NAMESPACE" fortuna-core -o jsonpath='{.spec.clusterIP}' 2>/dev/null || echo "")
if [ -z "$CORE_SVC_IP" ]; then
    log_error "Could not get Core service ClusterIP"
    exit 1
fi

log_success "Core service ClusterIP: $CORE_SVC_IP"

# Check endpoints
ENDPOINTS=$(kubectl get endpoints -n "$NAMESPACE" fortuna-core -o jsonpath='{.subsets[0].addresses[*].ip}' 2>/dev/null || echo "")
if [ -z "$ENDPOINTS" ]; then
    log_error "No service endpoints found"
    log_info "Core pod is not Ready. Fix Core first, then retry this script."
    exit 1
fi

log_success "Service endpoints: $ENDPOINTS"

# Step 2: Check Core pod status
log_section "Step 2: Checking Core Pod Status"
CORE_POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -z "$CORE_POD" ]; then
    log_error "No Core pod found"
    exit 1
fi

CORE_STATUS=$(kubectl get pod -n "$NAMESPACE" "$CORE_POD" -o jsonpath='{.status.phase}' 2>/dev/null || echo "Unknown")
CORE_READY=$(kubectl get pod -n "$NAMESPACE" "$CORE_POD" -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || echo "Unknown")

log_info "Core pod: $CORE_POD"
log_info "Status: $CORE_STATUS"
log_info "Ready: $CORE_READY"

if [ "$CORE_READY" != "True" ]; then
    log_warning "Core pod is not Ready. Agent cannot connect even with IP."
    log_info "Fix Core readiness first, then retry."
fi

# Step 3: Check current CORE_GRPC_ENDPOINT
log_section "Step 3: Checking Current Agent Configuration"
CURRENT_ENDPOINT=$(kubectl get daemonset -n "$NAMESPACE" fortuna-agent -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="CORE_GRPC_ENDPOINT")].value}' 2>/dev/null || echo "")
if [ -n "$CURRENT_ENDPOINT" ]; then
    log_info "Current CORE_GRPC_ENDPOINT: $CURRENT_ENDPOINT"
    
    # Check if already using IP
    if echo "$CURRENT_ENDPOINT" | grep -q "$CORE_SVC_IP"; then
        log_success "CORE_GRPC_ENDPOINT already uses IP: $CORE_SVC_IP"
        log_info "No changes needed for endpoint"
    fi
else
    log_warning "CORE_GRPC_ENDPOINT not found in DaemonSet"
fi

# Step 4: Update CORE_GRPC_ENDPOINT to use IP
log_section "Step 4: Updating CORE_GRPC_ENDPOINT to Use IP"
NEW_ENDPOINT="${CORE_SVC_IP}:9090"

log_info "Updating CORE_GRPC_ENDPOINT to: $NEW_ENDPOINT"

# Update via kubectl set env
if kubectl set env daemonset/fortuna-agent -n "$NAMESPACE" CORE_GRPC_ENDPOINT="$NEW_ENDPOINT" 2>/dev/null; then
    log_success "CORE_GRPC_ENDPOINT updated using kubectl set env"
else
    log_warning "kubectl set env failed, trying patch method..."
    
    # Alternative: Use patch
    kubectl patch daemonset -n "$NAMESPACE" fortuna-agent -p "{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"agent\",\"env\":[{\"name\":\"CORE_GRPC_ENDPOINT\",\"value\":\"$NEW_ENDPOINT\"}]}]}}}}" || {
        log_error "Failed to patch DaemonSet"
        exit 1
    }
    
    log_success "CORE_GRPC_ENDPOINT updated using patch"
fi

# Step 5: Check TLS configuration
log_section "Step 5: Checking TLS Configuration"
TLS_ENABLED=$(kubectl get daemonset -n "$NAMESPACE" fortuna-agent -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="TLS_ENABLED")].value}' 2>/dev/null || echo "true")
log_info "TLS_ENABLED: $TLS_ENABLED"

if [ "$TLS_ENABLED" = "true" ]; then
    log_warning "TLS is enabled. mTLS may require DNS name for ServerName validation."
    log_info "If connection still fails, you may need to temporarily disable TLS:"
    log_info "  kubectl set env daemonset/fortuna-agent -n $NAMESPACE TLS_ENABLED=false"
    log_info "Or update mTLS ServerName in Agent code to accept IP addresses."
fi

# Step 6: Restart Agent DaemonSet
log_section "Step 6: Restarting Agent DaemonSet"
log_info "Restarting Agent DaemonSet to apply changes..."
kubectl rollout restart daemonset -n "$NAMESPACE" fortuna-agent 2>/dev/null || {
    log_error "Failed to restart Agent DaemonSet"
    exit 1
}

log_info "Waiting for Agent pods to restart (20 seconds)..."
sleep 20

# Step 7: Verify Agent pods
log_section "Step 7: Verifying Agent Pods"
AGENT_PODS=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=agent -o wide 2>/dev/null || echo "")
if [ -z "$AGENT_PODS" ]; then
    log_warning "No Agent pods found"
else
    echo "$AGENT_PODS"
    echo ""
    
    AGENT_RUNNING=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=agent --no-headers 2>/dev/null | grep -c Running || echo "0")
    log_info "Agent pods Running: $AGENT_RUNNING"
fi

# Step 8: Monitor Agent logs
log_section "Step 8: Monitoring Agent Connection"
log_info "Checking Agent logs for connection status (10 seconds wait)..."
sleep 10

AGENT_POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=agent -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -n "$AGENT_POD" ]; then
    log_info "Checking logs from Agent pod: $AGENT_POD"
    CONNECTION_LOG=$(kubectl logs -n "$NAMESPACE" "$AGENT_POD" --tail=20 2>/dev/null | grep -i "connected\|connection\|heartbeat\|ping" | tail -5 || echo "")
    
    if [ -n "$CONNECTION_LOG" ]; then
        echo "$CONNECTION_LOG"
        
        if echo "$CONNECTION_LOG" | grep -q "Connected to Core\|✅ Connected"; then
            log_success "Agent connected to Core! ✅"
        elif echo "$CONNECTION_LOG" | grep -q "Heartbeat failed\|connection refused"; then
            log_warning "Agent still cannot connect"
            log_info "Possible issues:"
            log_info "  1. Core pod not Ready (check: kubectl get pods -n $NAMESPACE -l app.kubernetes.io/component=core)"
            log_info "  2. TLS ServerName mismatch (may need to disable TLS or update code)"
            log_info "  3. Network policy blocking traffic"
        fi
    else
        log_info "No connection-related logs found yet"
    fi
fi

# Step 9: Summary
log_section "Step 9: Summary"
log_info "Actions completed:"
log_info "  1. ✅ Got Core service IP: $CORE_SVC_IP"
log_info "  2. ✅ Updated CORE_GRPC_ENDPOINT to use IP: $NEW_ENDPOINT"
log_info "  3. ✅ Restarted Agent DaemonSet"
echo ""
log_info "Current configuration:"
log_info "  - Core service IP: $CORE_SVC_IP"
log_info "  - Agent endpoint: $NEW_ENDPOINT"
log_info "  - TLS enabled: $TLS_ENABLED"
echo ""
log_info "Next steps:"
log_info "  1. Monitor Agent logs: kubectl logs -n $NAMESPACE -l app.kubernetes.io/component=agent -f"
log_info "  2. Check Core logs: kubectl logs -n $NAMESPACE -l app.kubernetes.io/component=core -f"
log_info "  3. Verify Core is Ready: kubectl get pods -n $NAMESPACE -l app.kubernetes.io/component=core"
echo ""
if [ "$TLS_ENABLED" = "true" ]; then
    log_warning "If connection still fails, try disabling TLS temporarily:"
    log_info "  kubectl set env daemonset/fortuna-agent -n $NAMESPACE TLS_ENABLED=false"
    log_info "  kubectl rollout restart daemonset -n $NAMESPACE fortuna-agent"
fi
echo ""

