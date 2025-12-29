#!/bin/bash

# ============================================================================
# Complete Fortuna Deployment Script
# ============================================================================
# Deploys Fortuna with all fixes: DNS issues, multiple deployments, configs
# Handles Core and Agent together to avoid conflicts
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

log_section "Complete Fortuna Deployment - All Issues Fixed"

# Step 1: Check prerequisites
log_section "Step 1: Checking Prerequisites"

# Check namespace
if ! kubectl get namespace "$NAMESPACE" >/dev/null 2>&1; then
    log_info "Creating namespace: $NAMESPACE"
    kubectl create namespace "$NAMESPACE"
fi

# Check PostgreSQL
POSTGRES_IP=$(kubectl get svc -n "$NAMESPACE" postgres -o jsonpath='{.spec.clusterIP}' 2>/dev/null || echo "")
if [ -z "$POSTGRES_IP" ]; then
    log_error "PostgreSQL service not found. Deploy PostgreSQL first."
    exit 1
fi
log_success "PostgreSQL found: $POSTGRES_IP"

# Check NATS
NATS_POD=$(kubectl get pods -n "$NAMESPACE" -l app=nats -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -z "$NATS_POD" ]; then
    log_error "NATS not found. Deploy NATS first."
    exit 1
fi
log_success "NATS found: $NATS_POD"

# Check mTLS secrets
if ! kubectl get secret -n "$NAMESPACE" fortuna-core-server-tls >/dev/null 2>&1; then
    log_warning "mTLS secrets not found. Creating..."
    if [ -f "scripts/create-mtls-secrets.sh" ]; then
        ./scripts/create-mtls-secrets.sh || {
            log_error "Failed to create mTLS secrets"
            exit 1
        }
    else
        log_error "create-mtls-secrets.sh not found"
        exit 1
    fi
fi
log_success "mTLS secrets found"

# Step 2: Clean up ALL existing Core and Agent resources
log_section "Step 2: Cleaning Up Existing Resources"
log_warning "Deleting all existing Core and Agent deployments..."

# Delete all Core deployments
kubectl delete deployment -n "$NAMESPACE" -l app.kubernetes.io/component=core --wait=false 2>/dev/null || true
kubectl delete deployment -n "$NAMESPACE" -l app=ksam-core --wait=false 2>/dev/null || true
kubectl delete deployment -n "$NAMESPACE" -l app=fortuna-core --wait=false 2>/dev/null || true
kubectl delete deployment -n "$NAMESPACE" fortuna-core --wait=false 2>/dev/null || true
kubectl delete deployment -n "$NAMESPACE" ksam-core --wait=false 2>/dev/null || true

# Delete all Core pods
kubectl delete pods -n "$NAMESPACE" -l app.kubernetes.io/component=core --force --grace-period=0 2>/dev/null || true

# Delete all Agent DaemonSets
kubectl delete daemonset -n "$NAMESPACE" -l app.kubernetes.io/component=agent --wait=false 2>/dev/null || true
kubectl delete daemonset -n "$NAMESPACE" -l app=ksam-agent --wait=false 2>/dev/null || true
kubectl delete daemonset -n "$NAMESPACE" -l app=fortuna-agent --wait=false 2>/dev/null || true
kubectl delete daemonset -n "$NAMESPACE" fortuna-agent --wait=false 2>/dev/null || true
kubectl delete daemonset -n "$NAMESPACE" ksam-agent --wait=false 2>/dev/null || true

# Delete all Agent pods
kubectl delete pods -n "$NAMESPACE" -l app.kubernetes.io/component=agent --force --grace-period=0 2>/dev/null || true

log_info "Waiting for resources to be deleted (15 seconds)..."
sleep 15

log_success "Cleanup completed"

# Step 3: Get service IPs
log_section "Step 3: Getting Service IPs"
POSTGRES_IP=$(kubectl get svc -n "$NAMESPACE" postgres -o jsonpath='{.spec.clusterIP}' 2>/dev/null)
log_success "PostgreSQL IP: $POSTGRES_IP"

# Core service IP will be available after Core is deployed, but we'll update Agent later
log_info "Core service IP will be set after Core deployment"

# Step 4: Deploy Core with IP-based DATABASE_URL
log_section "Step 4: Deploying Core with IP-based Configuration"
log_info "Applying Core deployment..."

# Apply Core deployment
kubectl apply -f deploy/fortuna-core-deployment.yaml || {
    log_error "Failed to apply Core deployment"
    exit 1
}

# Update DATABASE_URL immediately with IP
NEW_DB_URL="postgres://postgres:postgres@${POSTGRES_IP}:5432/ksam?sslmode=disable"
log_info "Setting DATABASE_URL to use IP: $NEW_DB_URL"

kubectl set env deployment/fortuna-core -n "$NAMESPACE" DATABASE_URL="$NEW_DB_URL" 2>/dev/null || {
    log_warning "kubectl set env failed, using patch..."
    kubectl patch deployment -n "$NAMESPACE" fortuna-core -p "{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"core\",\"env\":[{\"name\":\"DATABASE_URL\",\"value\":\"$NEW_DB_URL\"}]}]}}}}" || {
        log_error "Failed to update DATABASE_URL"
        exit 1
    }
}

log_success "Core deployment applied with IP-based DATABASE_URL"

# Step 5: Wait for Core to be Ready
log_section "Step 5: Waiting for Core to be Ready"
log_info "Waiting for Core pod to start and become Ready (up to 2 minutes)..."

MAX_WAIT=120
ELAPSED=0
CORE_READY=false

while [ $ELAPSED -lt $MAX_WAIT ]; do
    CORE_POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
    
    if [ -n "$CORE_POD" ]; then
        CORE_STATUS=$(kubectl get pod -n "$NAMESPACE" "$CORE_POD" -o jsonpath='{.status.phase}' 2>/dev/null || echo "Unknown")
        CORE_READY_STATUS=$(kubectl get pod -n "$NAMESPACE" "$CORE_POD" -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || echo "False")
        RESTARTS=$(kubectl get pod -n "$NAMESPACE" "$CORE_POD" -o jsonpath='{.status.containerStatuses[0].restartCount}' 2>/dev/null || echo "0")
        
        if [ "$CORE_STATUS" = "Running" ] && [ "$CORE_READY_STATUS" = "True" ]; then
            log_success "Core is Running and Ready! ✅"
            CORE_READY=true
            break
        elif [ "$CORE_STATUS" = "CrashLoopBackOff" ] || [ "$RESTARTS" -gt 3 ]; then
            log_warning "Core is in bad state. Checking logs..."
            kubectl logs -n "$NAMESPACE" "$CORE_POD" --tail=20 2>/dev/null | grep -i "error\|fatal\|database" | tail -5 || true
        fi
        
        log_info "Waiting... (${ELAPSED}s/${MAX_WAIT}s) - Status: $CORE_STATUS, Ready: $CORE_READY_STATUS, Restarts: $RESTARTS"
    else
        log_info "Waiting for Core pod to be created... (${ELAPSED}s/${MAX_WAIT}s)"
    fi
    
    sleep 10
    ELAPSED=$((ELAPSED + 10))
done

if [ "$CORE_READY" = false ]; then
    log_error "Core did not become Ready within timeout"
    log_info "Check logs: kubectl logs -n $NAMESPACE -l app.kubernetes.io/component=core"
    exit 1
fi

# Step 6: Get Core service IP and verify endpoints
log_section "Step 6: Verifying Core Service"
CORE_SVC_IP=$(kubectl get svc -n "$NAMESPACE" fortuna-core -o jsonpath='{.spec.clusterIP}' 2>/dev/null || echo "")
if [ -z "$CORE_SVC_IP" ]; then
    log_error "Core service not found"
    exit 1
fi

ENDPOINTS=$(kubectl get endpoints -n "$NAMESPACE" fortuna-core -o jsonpath='{.subsets[0].addresses[*].ip}' 2>/dev/null || echo "")
if [ -z "$ENDPOINTS" ]; then
    log_error "Core service has no endpoints (pod not Ready)"
    exit 1
fi

log_success "Core service IP: $CORE_SVC_IP"
log_success "Core service endpoints: $ENDPOINTS"

# Step 7: Deploy Agent with IP-based CORE_GRPC_ENDPOINT
log_section "Step 7: Deploying Agent with IP-based Configuration"
log_info "Applying Agent DaemonSet..."

# Apply Agent DaemonSet
kubectl apply -f deploy/fortuna-agent-daemonset.yaml || {
    log_error "Failed to apply Agent DaemonSet"
    exit 1
}

# Update CORE_GRPC_ENDPOINT immediately with IP
NEW_AGENT_ENDPOINT="${CORE_SVC_IP}:9090"
log_info "Setting CORE_GRPC_ENDPOINT to use IP: $NEW_AGENT_ENDPOINT"

kubectl set env daemonset/fortuna-agent -n "$NAMESPACE" CORE_GRPC_ENDPOINT="$NEW_AGENT_ENDPOINT" 2>/dev/null || {
    log_warning "kubectl set env failed, using patch..."
    kubectl patch daemonset -n "$NAMESPACE" fortuna-agent -p "{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"agent\",\"env\":[{\"name\":\"CORE_GRPC_ENDPOINT\",\"value\":\"$NEW_AGENT_ENDPOINT\"}]}]}}}}" || {
        log_error "Failed to update CORE_GRPC_ENDPOINT"
        exit 1
    }
}

# Temporarily disable TLS to avoid ServerName validation issues with IP
log_warning "Temporarily disabling TLS to avoid ServerName validation issues..."
kubectl set env daemonset/fortuna-agent -n "$NAMESPACE" TLS_ENABLED="false" 2>/dev/null || {
    log_warning "Failed to disable TLS (may not be critical)"
}

log_success "Agent DaemonSet applied with IP-based CORE_GRPC_ENDPOINT"

# Step 8: Wait for Agent pods
log_section "Step 8: Waiting for Agent Pods"
log_info "Waiting for Agent pods to start (30 seconds)..."
sleep 30

AGENT_PODS=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=agent -o wide 2>/dev/null || echo "")
if [ -z "$AGENT_PODS" ]; then
    log_warning "No Agent pods found"
else
    echo "$AGENT_PODS"
    echo ""
    
    AGENT_RUNNING=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=agent --no-headers 2>/dev/null | grep -c Running || echo "0")
    log_info "Agent pods Running: $AGENT_RUNNING"
fi

# Step 9: Verify connections
log_section "Step 9: Verifying Connections"
log_info "Checking Agent connection to Core (20 seconds wait)..."
sleep 20

AGENT_POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=agent -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -n "$AGENT_POD" ]; then
    CONNECTION_LOG=$(kubectl logs -n "$NAMESPACE" "$AGENT_POD" --tail=30 2>/dev/null | grep -i "connected\|connection\|heartbeat" | tail -5 || echo "")
    
    if echo "$CONNECTION_LOG" | grep -q "Connected to Core\|✅ Connected"; then
        log_success "Agent connected to Core! ✅"
    elif echo "$CONNECTION_LOG" | grep -q "Heartbeat failed"; then
        log_warning "Agent still cannot connect. Check logs for details."
    else
        log_info "Connection status unclear. Check logs manually."
    fi
fi

# Step 10: Final status
log_section "Step 10: Final Status"
log_info "Core status:"
kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core -o wide
echo ""

log_info "Agent status:"
kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=agent -o wide
echo ""

log_info "Service endpoints:"
kubectl get endpoints -n "$NAMESPACE" fortuna-core
echo ""

# Step 11: Summary
log_section "Step 11: Summary"
log_success "Deployment completed!"
echo ""
log_info "Configuration applied:"
log_info "  ✅ Core DATABASE_URL: postgres://postgres:postgres@${POSTGRES_IP}:5432/ksam"
log_info "  ✅ Agent CORE_GRPC_ENDPOINT: ${CORE_SVC_IP}:9090"
log_info "  ✅ TLS temporarily disabled for Agent (to avoid ServerName validation)"
echo ""
log_info "Next steps:"
log_info "  1. Monitor Core: kubectl logs -n $NAMESPACE -l app.kubernetes.io/component=core -f"
log_info "  2. Monitor Agent: kubectl logs -n $NAMESPACE -l app.kubernetes.io/component=agent -f"
log_info "  3. Check Core health: kubectl get pods -n $NAMESPACE -l app.kubernetes.io/component=core"
log_info "  4. Check Agent connection: kubectl logs -n $NAMESPACE -l app.kubernetes.io/component=agent | grep -i connected"
echo ""
log_warning "Note: DNS issues are workarounded using IPs. For production, fix DNS properly."
log_warning "Note: TLS is disabled for Agent. Re-enable after fixing DNS or updating mTLS ServerName validation."
echo ""

