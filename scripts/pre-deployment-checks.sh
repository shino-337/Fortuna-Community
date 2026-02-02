#!/bin/bash

# ============================================================================
# Pre-Deployment Checks for Fortuna
# ============================================================================
# Validates cluster readiness before deploying Fortuna components.
# Target: Kubernetes with containerd + nerdctl for building images (no Docker/Podman required).
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

NAMESPACE="${NAMESPACE:-fortuna}"

echo "=========================================="
echo "Fortuna Pre-Deployment Checks"
echo "=========================================="
echo ""

ERRORS=0
WARNINGS=0

# Function to check command
check_cmd() {
    if command -v "$1" >/dev/null 2>&1; then
        echo -e "${GREEN}✅${NC} $1 found"
        return 0
    else
        echo -e "${RED}❌${NC} $1 not found"
        ERRORS=$((ERRORS+1))
        return 1
    fi
}

# Function to check Kubernetes resource
check_k8s_resource() {
    local resource=$1
    local name=$2
    local namespace=${3:-}
    
    if [ -n "$namespace" ]; then
        if kubectl get "$resource" "$name" -n "$namespace" >/dev/null 2>&1; then
            echo -e "${GREEN}✅${NC} $resource/$name exists in namespace $namespace"
            return 0
        else
            echo -e "${RED}❌${NC} $resource/$name not found in namespace $namespace"
            ERRORS=$((ERRORS+1))
            return 1
        fi
    else
        if kubectl get "$resource" "$name" >/dev/null 2>&1; then
            echo -e "${GREEN}✅${NC} $resource/$name exists"
            return 0
        else
            echo -e "${RED}❌${NC} $resource/$name not found"
            ERRORS=$((ERRORS+1))
            return 1
        fi
    fi
}

# Function to check DNS resolution (warning only: ephemeral pod can fail on image pull/timeout)
check_dns() {
    local service=$1
    local namespace=$2
    
    echo -n "Testing DNS resolution for $service.$namespace.svc.cluster.local... "
    
    TEST_POD="dns-test-$(date +%s)"
    if kubectl run "$TEST_POD" --image=busybox:1.36 --rm -i --restart=Never \
        --namespace="$namespace" --timeout=15s -- \
        nslookup "$service.$namespace.svc.cluster.local" >/dev/null 2>&1; then
        echo -e "${GREEN}✅${NC}"
        return 0
    else
        echo -e "${YELLOW}⚠️${NC}  DNS test failed (pod/image may be unavailable; deploy can continue)"
        WARNINGS=$((WARNINGS+1))
        return 0
    fi
}

# Function to check CoreDNS
check_coredns() {
    echo ""
    echo "=== Checking CoreDNS ==="
    
    # Check CoreDNS pods
    COREDNS_PODS=$(kubectl get pods -n kube-system -l k8s-app=kube-dns --no-headers 2>/dev/null | wc -l)
    if [ "$COREDNS_PODS" -eq 0 ]; then
        echo -e "${RED}❌${NC} No CoreDNS pods found"
        ERRORS=$((ERRORS+1))
        return 1
    else
        echo -e "${GREEN}✅${NC} Found $COREDNS_PODS CoreDNS pod(s)"
    fi
    
    # Check CoreDNS status
    COREDNS_READY=$(kubectl get pods -n kube-system -l k8s-app=kube-dns --no-headers 2>/dev/null | grep -c "Running" || true)
    if [ "$COREDNS_READY" -eq 0 ]; then
        echo -e "${RED}❌${NC} No CoreDNS pods in Running state"
        ERRORS=$((ERRORS+1))
    else
        echo -e "${GREEN}✅${NC} $COREDNS_READY CoreDNS pod(s) Running"
    fi
    
    # Check restart count
    RESTART_COUNT=$(kubectl get pods -n kube-system -l k8s-app=kube-dns --no-headers 2>/dev/null | \
        awk '{sum+=$4} END {print sum}' 2>/dev/null || echo "0")
    if [ "$RESTART_COUNT" -gt 10 ]; then
        echo -e "${YELLOW}⚠️${NC}  High CoreDNS restart count: $RESTART_COUNT (may indicate issues)"
        WARNINGS=$((WARNINGS+1))
    else
        echo -e "${GREEN}✅${NC} CoreDNS restart count: $RESTART_COUNT"
    fi
    
    # Check CoreDNS service
    if kubectl get svc -n kube-system kube-dns >/dev/null 2>&1; then
        COREDNS_IP=$(kubectl get svc -n kube-system kube-dns -o jsonpath='{.spec.clusterIP}' 2>/dev/null || echo "")
        if [ -n "$COREDNS_IP" ]; then
            echo -e "${GREEN}✅${NC} CoreDNS service IP: $COREDNS_IP"
        else
            echo -e "${RED}❌${NC} CoreDNS service has no ClusterIP"
            ERRORS=$((ERRORS+1))
        fi
    else
        echo -e "${RED}❌${NC} CoreDNS service not found"
        ERRORS=$((ERRORS+1))
    fi
}

# Function to check network connectivity
check_network() {
    echo ""
    echo "=== Checking Network Connectivity ==="
    
    # Check if we can reach CoreDNS IP
    COREDNS_IP=$(kubectl get svc -n kube-system kube-dns -o jsonpath='{.spec.clusterIP}' 2>/dev/null || echo "")
    if [ -n "$COREDNS_IP" ]; then
        echo -n "Testing connectivity to CoreDNS ($COREDNS_IP:53)... "
        if timeout 2 nc -u -w 1 "$COREDNS_IP" 53 >/dev/null 2>&1 || true; then
            echo -e "${GREEN}✅${NC}"
        else
            echo -e "${YELLOW}⚠️${NC}  Cannot reach CoreDNS (may be normal if not on cluster node)"
            WARNINGS=$((WARNINGS+1))
        fi
    fi
}

# Main checks
echo "=== Checking Prerequisites ==="
check_cmd kubectl

# Container runtime: target is containerd + nerdctl for build; docker/podman optional
if command -v nerdctl >/dev/null 2>&1 && command -v ctr >/dev/null 2>&1; then
    echo -e "${GREEN}✅${NC} nerdctl found (build with nerdctl)"
    echo -e "${GREEN}✅${NC} ctr found (containerd)"
elif command -v docker >/dev/null 2>&1; then
    echo -e "${GREEN}✅${NC} docker found"
elif command -v podman >/dev/null 2>&1; then
    echo -e "${GREEN}✅${NC} podman found"
else
    echo -e "${RED}❌${NC} No container runtime found. For k8s deploy with containerd: install nerdctl and ctr (containerd)."
    ERRORS=$((ERRORS+1))
fi

echo ""
echo "=== Checking Kubernetes Cluster ==="
if kubectl cluster-info >/dev/null 2>&1; then
    echo -e "${GREEN}✅${NC} Kubernetes cluster accessible"
    kubectl cluster-info | head -1
else
    echo -e "${RED}❌${NC} Cannot access Kubernetes cluster"
    ERRORS=$((ERRORS+1))
    exit 1
fi

echo ""
echo "=== Checking Namespace ==="
if kubectl get namespace "$NAMESPACE" >/dev/null 2>&1; then
    echo -e "${GREEN}✅${NC} Namespace $NAMESPACE exists"
else
    echo -e "${YELLOW}⚠️${NC}  Namespace $NAMESPACE does not exist (will be created)"
    WARNINGS=$((WARNINGS+1))
fi

echo ""
echo "=== Checking Infrastructure ==="
# Optional: postgres/nats may not exist yet (deploy script will create them). Treat missing as warning, not error.
if kubectl get service postgres -n "$NAMESPACE" >/dev/null 2>&1; then
    echo -e "${GREEN}✅${NC} service/postgres exists in namespace $NAMESPACE"
else
    echo -e "${YELLOW}⚠️${NC}  PostgreSQL service not found (will be created by deploy)"
    WARNINGS=$((WARNINGS+1))
fi
if kubectl get service nats -n "$NAMESPACE" >/dev/null 2>&1; then
    echo -e "${GREEN}✅${NC} service/nats exists in namespace $NAMESPACE"
else
    echo -e "${YELLOW}⚠️${NC}  NATS service not found (will be created by deploy)"
    WARNINGS=$((WARNINGS+1))
fi

# Check CoreDNS
check_coredns

# Check network
check_network

echo ""
echo "=== Checking DNS Resolution ==="
if kubectl get namespace "$NAMESPACE" >/dev/null 2>&1; then
    if kubectl get svc -n "$NAMESPACE" postgres >/dev/null 2>&1; then
        check_dns postgres "$NAMESPACE" || true
    else
        echo -e "${YELLOW}⚠️${NC}  PostgreSQL service not found, skipping DNS test"
    fi
    
    if kubectl get svc -n "$NAMESPACE" nats >/dev/null 2>&1; then
        check_dns nats "$NAMESPACE" || true
    else
        echo -e "${YELLOW}⚠️${NC}  NATS service not found, skipping DNS test"
    fi
else
    echo -e "${YELLOW}⚠️${NC}  Namespace $NAMESPACE not found, skipping DNS tests"
fi

echo ""
echo "=== Checking Node Labels ==="
WORKER_NODES=$(kubectl get nodes -l node-role.kubernetes.io/worker --no-headers 2>/dev/null | wc -l)
if [ "$WORKER_NODES" -gt 0 ]; then
    echo -e "${GREEN}✅${NC} Found $WORKER_NODES worker node(s) with label"
else
    echo -e "${YELLOW}⚠️${NC}  No worker nodes labeled (Core will use nodeSelector workaround)"
    WARNINGS=$((WARNINGS+1))
fi

MASTER_NODES=$(kubectl get nodes -l node-role.kubernetes.io/control-plane --no-headers 2>/dev/null | wc -l)
if [ "$MASTER_NODES" -gt 0 ]; then
    echo -e "${GREEN}✅${NC} Found $MASTER_NODES master node(s)"
fi

echo ""
echo "=========================================="
echo "Summary"
echo "=========================================="
echo -e "Errors: ${RED}$ERRORS${NC}"
echo -e "Warnings: ${YELLOW}$WARNINGS${NC}"
echo ""

if [ "$ERRORS" -eq 0 ]; then
    echo -e "${GREEN}✅ All critical checks passed${NC}"
    echo ""
    echo "Ready to deploy Fortuna!"
    exit 0
else
    echo -e "${RED}❌ Some critical checks failed${NC}"
    echo ""
    echo "Please fix the errors above before deploying."
    exit 1
fi

