#!/bin/bash

# ============================================================================
# DNS Issues Fix Script
# ============================================================================
# Attempts to fix DNS resolution issues in Kubernetes cluster
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "=========================================="
echo "DNS Issues Fix Script"
echo "=========================================="
echo ""

# Step 1: Restart CoreDNS
echo "Step 1: Restarting CoreDNS..."
if kubectl rollout restart deployment -n kube-system coredns >/dev/null 2>&1; then
    echo -e "${GREEN}✅${NC} CoreDNS restart initiated"
    
    echo "Waiting for CoreDNS to be ready..."
    if kubectl wait --for=condition=ready pod -n kube-system -l k8s-app=kube-dns --timeout=120s >/dev/null 2>&1; then
        echo -e "${GREEN}✅${NC} CoreDNS is ready"
    else
        echo -e "${YELLOW}⚠️${NC}  CoreDNS may not be ready yet"
    fi
else
    echo -e "${RED}❌${NC} Failed to restart CoreDNS"
    exit 1
fi

# Step 2: Check CoreDNS pods
echo ""
echo "Step 2: Checking CoreDNS pods..."
COREDNS_PODS=$(kubectl get pods -n kube-system -l k8s-app=kube-dns --no-headers 2>/dev/null | wc -l)
echo "Found $COREDNS_PODS CoreDNS pod(s)"

kubectl get pods -n kube-system -l k8s-app=kube-dns

# Step 3: Check CoreDNS logs for errors
echo ""
echo "Step 3: Checking CoreDNS logs (last 20 lines)..."
kubectl logs -n kube-system -l k8s-app=kube-dns --tail=20 2>&1 | head -20 || true

# Step 4: Test DNS resolution
echo ""
echo "Step 4: Testing DNS resolution..."
NAMESPACE="${NAMESPACE:-fortuna}"

if kubectl get namespace "$NAMESPACE" >/dev/null 2>&1; then
    # Test with a temporary pod
    TEST_POD="dns-test-$(date +%s)"
    echo "Creating test pod to verify DNS..."
    
    if kubectl run "$TEST_POD" --image=busybox:1.36 --rm -i --restart=Never \
        --namespace="$NAMESPACE" -- \
        nslookup kubernetes.default.svc.cluster.local >/dev/null 2>&1; then
        echo -e "${GREEN}✅${NC} DNS resolution working for kubernetes.default"
    else
        echo -e "${RED}❌${NC} DNS resolution failed for kubernetes.default"
    fi
    
    # Test service DNS if services exist
    if kubectl get svc -n "$NAMESPACE" postgres >/dev/null 2>&1; then
        if kubectl run "dns-test-postgres-$(date +%s)" --image=busybox:1.36 --rm -i --restart=Never \
            --namespace="$NAMESPACE" -- \
            nslookup "postgres.$NAMESPACE.svc.cluster.local" >/dev/null 2>&1; then
            echo -e "${GREEN}✅${NC} DNS resolution working for postgres.$NAMESPACE"
        else
            echo -e "${RED}❌${NC} DNS resolution failed for postgres.$NAMESPACE"
        fi
    fi
else
    echo -e "${YELLOW}⚠️${NC}  Namespace $NAMESPACE not found, skipping service DNS tests"
fi

# Step 5: Check network policies
echo ""
echo "Step 5: Checking NetworkPolicies..."
NP_COUNT=$(kubectl get networkpolicies -A --no-headers 2>/dev/null | wc -l)
if [ "$NP_COUNT" -gt 0 ]; then
    echo -e "${YELLOW}⚠️${NC}  Found $NP_COUNT NetworkPolicy(ies) - may block DNS"
    echo "NetworkPolicies:"
    kubectl get networkpolicies -A
    echo ""
    echo "If DNS is blocked, you may need to allow DNS traffic (UDP port 53)"
else
    echo -e "${GREEN}✅${NC} No NetworkPolicies found"
fi

# Step 6: Check CNI plugin
echo ""
echo "Step 6: Checking CNI plugin..."
CNI_PODS=$(kubectl get pods -n kube-system --no-headers 2>/dev/null | \
    grep -E "flannel|calico|weave|cilium" | wc -l)
if [ "$CNI_PODS" -gt 0 ]; then
    echo -e "${GREEN}✅${NC} Found CNI plugin pods"
    kubectl get pods -n kube-system | grep -E "flannel|calico|weave|cilium" | head -5
else
    echo -e "${YELLOW}⚠️${NC}  No CNI plugin pods found (may be using host network)"
fi

echo ""
echo "=========================================="
echo "DNS Fix Summary"
echo "=========================================="
echo ""
echo "If DNS issues persist:"
echo "1. Check CoreDNS logs: kubectl logs -n kube-system -l k8s-app=kube-dns"
echo "2. Check network connectivity between nodes"
echo "3. Verify firewall rules allow DNS (UDP port 53)"
echo "4. Check CNI plugin status"
echo "5. Consider using IP-based endpoints as temporary workaround"
echo ""

