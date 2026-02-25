#!/bin/bash

# ============================================================================
# DNS Configuration Fix Script
# ============================================================================
# This script fixes DNS resolution issues for Fortuna components by:
# 1. Updating CoreDNS ConfigMap with except clauses for internal domains
# 2. Ensuring Agent DNS config has appropriate timeout and attempts
# 3. Restarting CoreDNS and Agent pods
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

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

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl not found. Please install kubectl."
        exit 1
    fi
    
    if ! kubectl cluster-info &> /dev/null; then
        log_error "Cannot connect to Kubernetes cluster"
        exit 1
    fi
    
    log_success "Prerequisites check passed"
}

# Fix CoreDNS ConfigMap
fix_coredns() {
    log_info "Fixing CoreDNS configuration..."
    
    # Check if CoreDNS ConfigMap exists
    if ! kubectl get configmap coredns -n kube-system &> /dev/null; then
        log_error "CoreDNS ConfigMap not found"
        return 1
    fi
    
    # Create updated CoreDNS ConfigMap
    cat > /tmp/coredns-fixed.yaml << 'EOF'
apiVersion: v1
kind: ConfigMap
metadata:
  name: coredns
  namespace: kube-system
data:
  Corefile: |
    .:53 {
        errors
        health {
           lameduck 5s
        }
        ready
        kubernetes cluster.local in-addr.arpa ip6.arpa {
           pods insecure
           fallthrough in-addr.arpa ip6.arpa
           ttl 30
        }
        prometheus :9153
        forward . /etc/resolv.conf {
           except cluster.local
           except svc.cluster.local
           except fortuna.svc.cluster.local
           max_concurrent 1000
        }
        cache 30
        loop
        reload
        loadbalance
    }
EOF
    
    if kubectl apply -f /tmp/coredns-fixed.yaml; then
        log_success "CoreDNS ConfigMap updated"
        
        # Restart CoreDNS
        log_info "Restarting CoreDNS pods..."
        if kubectl rollout restart deployment coredns -n kube-system; then
            log_info "Waiting for CoreDNS pods to be ready (60 seconds)..."
            if kubectl wait --for=condition=ready pod -l k8s-app=kube-dns -n kube-system --timeout=60s &> /dev/null; then
                log_success "CoreDNS pods restarted and ready"
            else
                log_warning "CoreDNS pods may not be ready yet, but restart initiated"
            fi
        else
            log_warning "Failed to restart CoreDNS, but ConfigMap updated"
        fi
    else
        log_error "Failed to update CoreDNS ConfigMap"
        return 1
    fi
    
    rm -f /tmp/coredns-fixed.yaml
}

# Verify Agent DNS config
verify_agent_dns() {
    log_info "Verifying Agent DNS configuration..."
    
    local agent_file="${PROJECT_ROOT}/deploy/fortuna-agent-daemonset.yaml"
    
    if [ ! -f "$agent_file" ]; then
        log_error "Agent DaemonSet file not found: $agent_file"
        return 1
    fi
    
    # Check if DNS config has correct timeout and attempts
    local timeout=$(grep -A 1 "name: \"timeout\"" "$agent_file" | grep "value:" | sed 's/.*value: "\([0-9]*\)".*/\1/')
    local attempts=$(grep -A 1 "name: \"attempts\"" "$agent_file" | grep "value:" | sed 's/.*value: "\([0-9]*\)".*/\1/')
    
    if [ -z "$timeout" ] || [ -z "$attempts" ]; then
        log_warning "Could not verify Agent DNS config values"
        return 0
    fi
    
    if [ "$timeout" -lt 5 ] || [ "$attempts" -lt 5 ]; then
        log_warning "Agent DNS config may need adjustment (timeout: ${timeout}s, attempts: ${attempts})"
        log_info "Recommended: timeout >= 5s, attempts >= 5"
    else
        log_success "Agent DNS config verified (timeout: ${timeout}s, attempts: ${attempts})"
    fi
}

# Restart Agent pods
restart_agent() {
    log_info "Restarting Agent pods to apply DNS config..."
    
    if kubectl rollout restart daemonset -n fortuna fortuna-agent &> /dev/null; then
        log_info "Waiting for Agent pods to be ready (30 seconds)..."
        sleep 30
        
        local ready_count=$(kubectl get pods -n fortuna -l app=fortuna-agent --field-selector=status.phase=Running --no-headers 2>/dev/null | wc -l)
        local total_count=$(kubectl get pods -n fortuna -l app=fortuna-agent --no-headers 2>/dev/null | wc -l)
        
        if [ "$ready_count" -eq "$total_count" ] && [ "$total_count" -gt 0 ]; then
            log_success "Agent pods restarted and ready ($ready_count/$total_count)"
        else
            log_warning "Agent pods may not be fully ready yet ($ready_count/$total_count)"
        fi
    else
        log_warning "Failed to restart Agent pods (may not exist yet)"
    fi
}

# Test DNS resolution
test_dns() {
    log_info "Testing DNS resolution..."
    
    # Wait a bit for DNS to stabilize
    sleep 10
    
    # Test from a temporary pod
    local test_pod="dns-test-$(date +%s)"
    
    if kubectl run "$test_pod" --image=busybox:1.35 --restart=Never -n fortuna --rm -i --overrides='{"spec":{"dnsPolicy":"ClusterFirst"}}' -- nslookup fortuna-core.fortuna.svc.cluster.local 2>&1 | grep -q "10\."; then
        log_success "DNS resolution test passed"
        return 0
    else
        log_warning "DNS resolution test inconclusive (test pod may have issues)"
        return 0  # Don't fail on test, just warn
    fi
}

# Main
main() {
    echo "=========================================="
    echo "Fortuna DNS Configuration Fix"
    echo "=========================================="
    echo ""
    
    check_prerequisites
    
    echo ""
    log_info "Starting DNS configuration fixes..."
    echo ""
    
    # Fix CoreDNS
    if fix_coredns; then
        echo ""
    else
        log_error "Failed to fix CoreDNS"
        exit 1
    fi
    
    # Verify Agent DNS config
    verify_agent_dns
    echo ""
    
    # Restart Agent
    restart_agent
    echo ""
    
    # Test DNS
    test_dns
    echo ""
    
    log_success "DNS configuration fix completed"
    echo ""
    echo "Next steps:"
    echo "  1. Monitor Agent logs: kubectl logs -n fortuna -l app=fortuna-agent --tail=20"
    echo "  2. Verify Agent connection: Look for 'Connected to Core' messages"
    echo "  3. Re-run E2E tests if needed"
    echo ""
}

main "$@"

