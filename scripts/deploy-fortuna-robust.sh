#!/bin/bash

# ============================================================================
# Robust Fortuna Deployment Script
# ============================================================================
# Comprehensive deployment with DNS fallback, cleanup, and verification
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

NAMESPACE="${NAMESPACE:-fortuna}"
USE_IP_FALLBACK="${USE_IP_FALLBACK:-true}"

echo "=========================================="
echo "Fortuna Robust Deployment"
echo "=========================================="
echo ""

# Step 1: Pre-deployment checks
echo -e "${BLUE}Step 1: Pre-deployment checks...${NC}"
if [ -f "${SCRIPT_DIR}/pre-deployment-checks.sh" ]; then
    if ! bash "${SCRIPT_DIR}/pre-deployment-checks.sh"; then
        echo -e "${YELLOW}⚠️${NC}  Pre-deployment checks failed, but continuing..."
    fi
else
    echo -e "${YELLOW}⚠️${NC}  Pre-deployment checks script not found, skipping..."
fi

# Step 2: Create namespace
echo ""
echo -e "${BLUE}Step 2: Creating namespace...${NC}"
if kubectl get namespace "$NAMESPACE" >/dev/null 2>&1; then
    echo -e "${GREEN}✅${NC} Namespace $NAMESPACE already exists"
else
    kubectl create namespace "$NAMESPACE"
    echo -e "${GREEN}✅${NC} Namespace $NAMESPACE created"
fi

# Step 3: Comprehensive cleanup
echo ""
echo -e "${BLUE}Step 3: Cleaning up old resources...${NC}"

# Delete by labels
kubectl delete deployment -n "$NAMESPACE" -l app.kubernetes.io/name=fortuna --ignore-not-found=true
kubectl delete deployment -n "$NAMESPACE" -l app=fortuna-core --ignore-not-found=true
kubectl delete deployment -n "$NAMESPACE" -l app=ksam-core --ignore-not-found=true

# Delete by name patterns
kubectl delete deployment -n "$NAMESPACE" fortuna-core ksam-core --ignore-not-found=true

# Delete services
kubectl delete service -n "$NAMESPACE" fortuna-core ksam-core --ignore-not-found=true

# Delete pods (orphaned)
kubectl delete pods -n "$NAMESPACE" -l app.kubernetes.io/component=core --ignore-not-found=true
kubectl delete pods -n "$NAMESPACE" -l app=fortuna-core --ignore-not-found=true

# Wait for cleanup
echo "Waiting for resources to be deleted..."
sleep 5

echo -e "${GREEN}✅${NC} Cleanup completed"

# Step 4: Deploy infrastructure (if not exists)
echo ""
echo -e "${BLUE}Step 4: Checking infrastructure...${NC}"

if ! kubectl get svc -n "$NAMESPACE" postgres >/dev/null 2>&1; then
    echo "PostgreSQL not found, deploying..."
    kubectl apply -f "${PROJECT_ROOT}/deploy/infrastructure/postgresql-with-age.yaml"
    echo "Waiting for PostgreSQL..."
    kubectl wait --for=condition=ready pod -n "$NAMESPACE" -l app=postgres --timeout=300s || true
else
    echo -e "${GREEN}✅${NC} PostgreSQL already exists"
fi

if ! kubectl get svc -n "$NAMESPACE" nats >/dev/null 2>&1; then
    echo "NATS not found, deploying..."
    kubectl apply -f "${PROJECT_ROOT}/deploy/infrastructure/nats.yaml"
    echo "Waiting for NATS..."
    kubectl wait --for=condition=ready pod -n "$NAMESPACE" -l app=nats --timeout=300s || true
else
    echo -e "${GREEN}✅${NC} NATS already exists"
fi

# Step 5: Verify images in containerd (if using containerd)
echo ""
echo -e "${BLUE}Step 5: Verifying images in containerd...${NC}"

# Check if using containerd
if command -v ctr >/dev/null 2>&1 && [ -S /run/containerd/containerd.sock ] || [ -S /var/run/containerd/containerd.sock ]; then
    echo "Containerd detected, checking images..."
    if ctr -n k8s.io images ls 2>/dev/null | grep -q "fortuna-core"; then
        echo -e "${GREEN}✅${NC} Fortuna images found in containerd"
    else
        echo -e "${YELLOW}⚠️${NC}  Fortuna images not found in containerd"
        echo "Build images with: ./scripts/build-with-containerd.sh"
    fi
fi

# Step 6: Test DNS or use IP fallback
echo ""
echo -e "${BLUE}Step 6: Testing DNS resolution...${NC}"

USE_DNS=true
if [ "$USE_IP_FALLBACK" = "true" ]; then
    # Test DNS resolution
    TEST_POD="dns-test-$(date +%s)"
    if kubectl run "$TEST_POD" --image=busybox:1.36 --rm -i --restart=Never \
        --namespace="$NAMESPACE" -- \
        nslookup "postgres.$NAMESPACE.svc.cluster.local" >/dev/null 2>&1; then
        echo -e "${GREEN}✅${NC} DNS resolution working"
        USE_DNS=true
    else
        echo -e "${YELLOW}⚠️${NC}  DNS resolution failed, using IP fallback"
        USE_DNS=false
    fi
fi

# Step 7: Deploy RBAC
echo ""
echo -e "${BLUE}Step 7: Deploying RBAC...${NC}"
kubectl apply -f "${PROJECT_ROOT}/deploy/fortuna-rbac.yaml"
echo -e "${GREEN}✅${NC} RBAC deployed"

# Step 8: Deploy Core
echo ""
echo -e "${BLUE}Step 8: Deploying Core...${NC}"
kubectl apply -f "${PROJECT_ROOT}/deploy/fortuna-core-deployment.yaml"

# Configure DATABASE_URL
if [ "$USE_DNS" = "false" ] && [ "$USE_IP_FALLBACK" = "true" ]; then
    echo "Configuring DATABASE_URL with IP address..."
    POSTGRES_IP=$(kubectl get svc -n "$NAMESPACE" postgres -o jsonpath='{.spec.clusterIP}')
    if [ -n "$POSTGRES_IP" ]; then
        kubectl set env deployment/fortuna-core -n "$NAMESPACE" \
            DATABASE_URL="postgres://postgres:postgres@${POSTGRES_IP}:5432/ksam?sslmode=disable"
        echo -e "${GREEN}✅${NC} DATABASE_URL configured with IP: $POSTGRES_IP"
    else
        echo -e "${RED}❌${NC} Could not get PostgreSQL IP"
    fi
fi

# Wait for Core to be ready
echo "Waiting for Core to be ready..."
if kubectl wait --for=condition=available deployment/fortuna-core -n "$NAMESPACE" --timeout=300s; then
    echo -e "${GREEN}✅${NC} Core deployment ready"
else
    echo -e "${YELLOW}⚠️${NC}  Core deployment not ready yet, checking status..."
    kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core
    kubectl logs -n "$NAMESPACE" -l app.kubernetes.io/component=core --tail=20 || true
fi

# Step 8: Verify Core service has endpoints
echo ""
echo -e "${BLUE}Step 8: Verifying Core service...${NC}"
sleep 5
ENDPOINTS=$(kubectl get endpoints -n "$NAMESPACE" fortuna-core -o jsonpath='{.subsets[0].addresses[*].ip}' 2>/dev/null || echo "")
if [ -n "$ENDPOINTS" ]; then
    echo -e "${GREEN}✅${NC} Core service has endpoints: $ENDPOINTS"
else
    echo -e "${RED}❌${NC} Core service has no endpoints"
    echo "Checking Core pod status..."
    kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core
    kubectl describe pod -n "$NAMESPACE" -l app.kubernetes.io/component=core | tail -20
fi

# Step 9: Deploy Agent
echo ""
echo -e "${BLUE}Step 9: Deploying Agent...${NC}"
kubectl apply -f "${PROJECT_ROOT}/deploy/fortuna-agent-daemonset.yaml"

# Configure CORE_GRPC_ENDPOINT
if [ "$USE_DNS" = "false" ] && [ "$USE_IP_FALLBACK" = "true" ]; then
    echo "Configuring CORE_GRPC_ENDPOINT with IP address..."
    CORE_IP=$(kubectl get svc -n "$NAMESPACE" fortuna-core -o jsonpath='{.spec.clusterIP}')
    if [ -n "$CORE_IP" ]; then
        kubectl set env daemonset/fortuna-agent -n "$NAMESPACE" \
            CORE_GRPC_ENDPOINT="${CORE_IP}:9090"
        echo -e "${GREEN}✅${NC} CORE_GRPC_ENDPOINT configured with IP: $CORE_IP"
        
        # Disable TLS when using IP (ServerName validation issue)
        echo "Disabling TLS for IP-based connection..."
        kubectl set env daemonset/fortuna-agent -n "$NAMESPACE" TLS_ENABLED="false"
        echo -e "${YELLOW}⚠️${NC}  TLS disabled (required for IP-based connection)"
    else
        echo -e "${RED}❌${NC} Could not get Core service IP"
    fi
fi

# Wait for Agent pods
echo "Waiting for Agent pods..."
sleep 10
AGENT_PODS=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=agent --no-headers 2>/dev/null | wc -l)
echo -e "${GREEN}✅${NC} Found $AGENT_PODS Agent pod(s)"

# Step 10: Final verification
echo ""
echo -e "${BLUE}Step 10: Final verification...${NC}"
echo ""
echo "=== Core Status ==="
kubectl get deployment -n "$NAMESPACE" fortuna-core
kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core
kubectl get svc -n "$NAMESPACE" fortuna-core

echo ""
echo "=== Agent Status ==="
kubectl get daemonset -n "$NAMESPACE" fortuna-agent
kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=agent

echo ""
echo "=== Service Endpoints ==="
kubectl get endpoints -n "$NAMESPACE" fortuna-core

echo ""
echo "=========================================="
echo -e "${GREEN}✅ Deployment completed!${NC}"
echo "=========================================="
echo ""
echo "Next steps:"
echo "1. Check Core logs: kubectl logs -n $NAMESPACE -l app.kubernetes.io/component=core"
echo "2. Check Agent logs: kubectl logs -n $NAMESPACE -l app.kubernetes.io/component=agent"
echo "3. Test Core API: kubectl port-forward -n $NAMESPACE svc/fortuna-core 8080:8080"
echo ""
if [ "$USE_DNS" = "false" ]; then
    echo -e "${YELLOW}⚠️${NC}  Using IP-based connections (workaround)"
    echo "To fix DNS and use service names:"
    echo "1. Fix DNS issues: ./scripts/fix-dns-issues.sh"
    echo "2. Update DATABASE_URL to use postgres.$NAMESPACE.svc.cluster.local"
    echo "3. Update CORE_GRPC_ENDPOINT to use fortuna-core.$NAMESPACE.svc.cluster.local"
    echo "4. Re-enable TLS: kubectl set env daemonset/fortuna-agent -n $NAMESPACE TLS_ENABLED=true"
fi
echo ""

