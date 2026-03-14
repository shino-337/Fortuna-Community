#!/bin/bash

# ============================================================================
# Robust Fortuna Deployment Script
# ============================================================================
# Deploy only (no clean/rebuild). Core runs DB migrations on startup.
# Called by full-clean-database-rebuild-deploy.sh and full-rebuild-sync-deploy-and-e2e.sh.
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
SCRIPTS="$PROJECT_ROOT/scripts"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

NAMESPACE="${NAMESPACE:-fortuna}"
USE_IP_FALLBACK="${USE_IP_FALLBACK:-true}"
AUTO_LOAD_CVE_ON_DEPLOY="${AUTO_LOAD_CVE_ON_DEPLOY:-true}"

echo "=========================================="
echo "Fortuna Robust Deployment"
echo "=========================================="
echo ""

# Step 1: Pre-deployment checks
echo -e "${BLUE}Step 1: Pre-deployment checks...${NC}"
if [ -f "$SCRIPTS/deploy/pre-deployment-checks.sh" ]; then
    if ! bash "$SCRIPTS/deploy/pre-deployment-checks.sh"; then
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
kubectl delete deployment -n "$NAMESPACE" -l app.kubernetes.io/component=core --ignore-not-found=true
kubectl delete deployment -n "$NAMESPACE" -l app=ksam-core --ignore-not-found=true

# Delete by name patterns
kubectl delete deployment -n "$NAMESPACE" fortuna-core ksam-core --ignore-not-found=true

# Delete services
kubectl delete service -n "$NAMESPACE" fortuna-core ksam-core --ignore-not-found=true

# Delete pods (orphaned)
kubectl delete pods -n "$NAMESPACE" -l app.kubernetes.io/component=core --ignore-not-found=true
kubectl delete pods -n "$NAMESPACE" -l app.kubernetes.io/component=core --ignore-not-found=true

# Wait for cleanup so next apply does not conflict
echo "Waiting for resources to be deleted (8s)..."
sleep 8

echo -e "${GREEN}✅${NC} Cleanup completed"

# Step 3a: Ensure Flannel CNI so pod network works (avoids subnet.env missing; required for local-path-provisioner and all pods)
echo ""
echo -e "${BLUE}Step 3a: Ensuring Flannel CNI (install if missing)...${NC}"
if [ -x "$SCRIPTS/deploy/ensure-flannel.sh" ]; then
    if bash "$SCRIPTS/deploy/ensure-flannel.sh"; then
        echo -e "${GREEN}✅${NC} Flannel CNI ready"
    else
        echo -e "${YELLOW}⚠️${NC}  Flannel check/install had issues; if pods stay ContainerCreating, run: ./scripts/deploy/ensure-flannel.sh"
    fi
    echo "Sleep 5s for Flannel to stabilize..."
    sleep 5
else
    echo -e "${YELLOW}⚠️${NC}  ensure-flannel.sh not found; if pod network fails (subnet.env), install Flannel before deploying infra"
fi

# Step 3b: Ensure StorageClass (local-path) so PostgreSQL/NATS PVCs can bind
echo ""
echo -e "${BLUE}Step 3b: Ensuring StorageClass (local-path) for PVCs...${NC}"
if [ -x "$SCRIPTS/deploy/ensure-storage-class.sh" ]; then
    if bash "$SCRIPTS/deploy/ensure-storage-class.sh"; then
        echo -e "${GREEN}✅${NC} StorageClass ready"
    else
        echo -e "${YELLOW}⚠️${NC}  StorageClass check/install had issues; PVCs may stay Pending. Install manually: kubectl apply -f https://raw.githubusercontent.com/rancher/local-path-provisioner/v0.0.24/deploy/local-path-storage.yaml"
    fi
    echo "Sleep 5s for provisioner to be ready..."
    sleep 5
else
    echo -e "${YELLOW}⚠️${NC}  ensure-storage-class.sh not found; if PVCs stay Pending, run: ./scripts/deploy/ensure-storage-class.sh"
fi

# Step 4: Deploy/update infrastructure (apply so spec updates e.g. nodeSelector changes)
echo ""
echo -e "${BLUE}Step 4: Deploying/updating infrastructure...${NC}"

kubectl apply -f "${PROJECT_ROOT}/deploy/infrastructure/postgresql-with-age.yaml"
echo "Waiting for PostgreSQL (max 300s)..."
kubectl wait --for=condition=ready pod -n "$NAMESPACE" -l app=postgres --timeout=300s || true
echo "Sleep 5s for DB to accept connections..."
sleep 5
echo -e "${GREEN}✅${NC} PostgreSQL applied"

kubectl apply -f "${PROJECT_ROOT}/deploy/infrastructure/nats.yaml"
echo "Waiting for NATS (max 300s)..."
kubectl wait --for=condition=ready pod -n "$NAMESPACE" -l app=nats --timeout=300s || true
echo "Sleep 5s for NATS to stabilize..."
sleep 5
echo -e "${GREEN}✅${NC} NATS applied"

# Step 4b: Ensure pod network (multi-node) so Agent on worker can reach Core
# Use short CNI wait when called from rebuild/deploy so script does not hang (CNI_WAIT_SECONDS=10)
NODE_COUNT=$(kubectl get nodes --no-headers 2>/dev/null | wc -l)
if [ "${NODE_COUNT:-0}" -gt 1 ]; then
    echo ""
    echo -e "${BLUE}Step 4b: Ensuring pod network (multi-node cluster, CNI)...${NC}"
    if [ -x "$SCRIPTS/deploy/fix-flannel-vxlan.sh" ]; then
        if CNI_WAIT_SECONDS="${CNI_WAIT_SECONDS:-10}" bash "$SCRIPTS/deploy/fix-flannel-vxlan.sh" 2>/dev/null; then
            echo -e "${GREEN}✅${NC} Pod network (Flannel) verified/fixed"
        else
            echo -e "${YELLOW}⚠️${NC}  Flannel fix had warnings (Agent on worker may need manual check)"
        fi
    else
        echo -e "${YELLOW}⚠️${NC}  fix-flannel-vxlan.sh not found; if Agent on worker cannot reach Core, run: ./scripts/deploy/fix-flannel-vxlan.sh"
    fi
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
        echo "Build images with: ./scripts/build/build-and-load-containerd.sh"
    fi
fi

# Step 5b: Push images to worker nodes (multi-node) so Agent/Core find image on all nodes
NODE_COUNT=$(kubectl get nodes --no-headers 2>/dev/null | wc -l)
if [ "${NODE_COUNT:-0}" -gt 1 ] && [ -x "$SCRIPTS/utils/push-images-to-workers.sh" ]; then
    echo ""
    echo -e "${BLUE}Step 5b: Pushing images to worker nodes...${NC}"
    if bash "$SCRIPTS/utils/push-images-to-workers.sh" 2>&1; then
        echo -e "${GREEN}✅${NC} Images pushed to all nodes"
    else
        echo -e "${YELLOW}⚠️${NC}  Push to workers failed (SSH or keys). Add scripts/utils/push-images.config (see push-images.config.example) or set SSH_USER/SSH_PASS; then run: $SCRIPTS/utils/push-images-to-workers.sh"
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

# Step 7: Deploy RBAC (Core + Agent with serviceaccounts/roles/clusterroles for sync)
echo ""
echo -e "${BLUE}Step 7: Deploying RBAC...${NC}"
kubectl apply -f "${PROJECT_ROOT}/deploy/fortuna-rbac.yaml"
echo "Sleep 5s for API server to apply RBAC..."
sleep 5
echo -e "${GREEN}✅${NC} RBAC deployed"

# Step 7b: Ensure at least one node has control-plane label so Core can schedule (avoids Pending / 0 nodes available)
echo ""
echo -e "${BLUE}Step 7b: Ensuring control-plane node label...${NC}"
if [ -x "$SCRIPTS/deploy/ensure-control-plane-label.sh" ]; then
    if bash "$SCRIPTS/deploy/ensure-control-plane-label.sh"; then
        echo -e "${GREEN}✅${NC} Control-plane label OK"
    else
        echo -e "${YELLOW}⚠️${NC}  ensure-control-plane-label.sh had issues; if Core stays Pending, run: kubectl label node <master-node> node-role.kubernetes.io/control-plane= --overwrite"
    fi
else
    echo -e "${YELLOW}⚠️${NC}  ensure-control-plane-label.sh not found; if Core pod stays Pending (node affinity), label master: kubectl label node <node> node-role.kubernetes.io/control-plane= --overwrite"
fi

# Step 7c: Ensure mTLS secrets (Core and Agent need fortuna-core-tls, fortuna-agent-tls, fortuna-ca-cert, fortuna-webhook-tls)
echo ""
echo -e "${BLUE}Step 7c: Ensuring mTLS secrets (Core/Agent TLS)...${NC}"
if [ -x "$SCRIPTS/utils/create_mtls_secret.sh" ]; then
    if NAMESPACE="$NAMESPACE" bash "$SCRIPTS/utils/create_mtls_secret.sh"; then
        echo -e "${GREEN}✅${NC} mTLS secrets ready"
    else
        echo -e "${RED}❌${NC} mTLS secret creation failed; Core/Agent pods will stay ContainerCreating until secrets exist"
        echo "  Run manually: NAMESPACE=$NAMESPACE $SCRIPTS/utils/create_mtls_secret.sh"
        exit 1
    fi
else
    echo -e "${YELLOW}⚠️${NC}  create_mtls_secret.sh not found; if Core/Agent stay ContainerCreating (secret not found), run: ./scripts/utils/create_mtls_secret.sh"
    exit 1
fi

# Step 7d: Prerequisites check before Core/Agent (Finding #7.2 – fail fast)
echo ""
echo -e "${BLUE}Step 7d: Prerequisites check (secrets + postgres + nats)...${NC}"
if [ -x "$SCRIPTS/deploy/check-prerequisites-core-agent.sh" ]; then
    if ! NAMESPACE="$NAMESPACE" bash "$SCRIPTS/deploy/check-prerequisites-core-agent.sh"; then
        echo -e "${RED}❌${NC} Prerequisites check failed. Fix the errors above before deploying Core/Agent."
        exit 1
    fi
else
    echo -e "${YELLOW}⚠️${NC}  check-prerequisites-core-agent.sh not found; continuing without strict check"
fi

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
            DATABASE_URL="postgres://postgres:postgres@${POSTGRES_IP}:5432/fortuna?sslmode=disable"
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

# Step 8a: Verify Core service has endpoints
echo ""
echo -e "${BLUE}Step 8a: Verifying Core service...${NC}"
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

# Step 8b: Quick DNS check so Agent can resolve fortuna-core (avoids "no such host" after deploy)
echo ""
echo -e "${BLUE}Step 8b: Checking DNS for fortuna-core...${NC}"
CORE_FQDN="fortuna-core.${NAMESPACE}.svc.cluster.local"
if kubectl run "dns-check-$(date +%s)" --image=busybox:1.36 --rm -i --restart=Never \
    --namespace="$NAMESPACE" -- nslookup "$CORE_FQDN" >/dev/null 2>&1; then
    echo -e "${GREEN}✅${NC} DNS resolves: $CORE_FQDN"
else
    echo -e "${YELLOW}⚠️${NC}  DNS lookup for $CORE_FQDN failed (Agent may log 'no such host' until DNS propagates; Agent will retry/reconnect)"
fi

# Step 8c: Load package_vulnerabilities from existing cve-data/all if present (no OSV sync during deploy).
# During deploy we skip the heavy OSV sync (AUTO_SYNC_CVE_SOURCE=false). To sync and load manually:
#   AUTO_SYNC_CVE_SOURCE=true ./scripts/utils/load-cve-data.sh
echo ""
echo -e "${BLUE}Step 8c: Loading package vulnerability references (if CVE data present)...${NC}"
if [ "$AUTO_LOAD_CVE_ON_DEPLOY" = "true" ] && [ -x "$SCRIPTS/utils/load-cve-data.sh" ]; then
    if NAMESPACE="$NAMESPACE" PROJECT_ROOT="$PROJECT_ROOT" \
       AUTO_SYNC_CVE_SOURCE="${AUTO_SYNC_CVE_SOURCE:-false}" \
       RESET_CVE_TABLES="${RESET_CVE_TABLES:-false}" \
       CVE_DATA_DIR="${CVE_DATA_DIR:-$PROJECT_ROOT/cve-data}" \
       bash "$SCRIPTS/utils/load-cve-data.sh"; then
        echo -e "${GREEN}✅${NC} package_vulnerabilities source ready or skipped (no CVE data dir)"
    else
        echo -e "${YELLOW}⚠️${NC}  Failed to auto-load package_vulnerabilities source (continuing deploy)"
    fi
else
    echo -e "${YELLOW}⚠️${NC}  AUTO_LOAD_CVE_ON_DEPLOY disabled or loader script missing"
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

# Wait for Agent DaemonSet rollout and pods
echo "Waiting for Agent DaemonSet rollout (max 90s)..."
kubectl rollout status daemonset/fortuna-agent -n "$NAMESPACE" --timeout=90s 2>/dev/null || true
echo "Sleep 10s for Agent pods to start and sync..."
sleep 10
AGENT_PODS=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=agent --no-headers 2>/dev/null | wc -l)
echo -e "${GREEN}✅${NC} Found $AGENT_PODS Agent pod(s)"

# Step 9b: Deploy Dashboard
echo ""
echo -e "${BLUE}Step 9b: Deploying Dashboard...${NC}"
[ -f "${PROJECT_ROOT}/deploy/dashboard-nginx-configmap.yaml" ] && kubectl apply -f "${PROJECT_ROOT}/deploy/dashboard-nginx-configmap.yaml"
[ -f "${PROJECT_ROOT}/deploy/dashboard-deployment.yaml" ] && kubectl apply -f "${PROJECT_ROOT}/deploy/dashboard-deployment.yaml"
echo -e "${GREEN}✅${NC} Dashboard deployed"

# Step 10: Rollout restart workloads so new images (from rebuild) are used
echo ""
echo -e "${BLUE}Step 10: Rollout restart (Core, Dashboard, Agent) to use new images...${NC}"
kubectl rollout restart deployment/fortuna-core -n "$NAMESPACE" --timeout=60s 2>/dev/null || true
kubectl rollout restart deployment/fortuna-dashboard -n "$NAMESPACE" --timeout=90s 2>/dev/null || true
kubectl rollout restart daemonset/fortuna-agent -n "$NAMESPACE" --timeout=90s 2>/dev/null || true
echo "Waiting for Core to be available (max 120s)..."
kubectl wait --for=condition=available deployment/fortuna-core -n "$NAMESPACE" --timeout=120s 2>/dev/null || true
echo "Waiting for Core rollout to complete..."
kubectl rollout status deployment/fortuna-core -n "$NAMESPACE" --timeout=120s 2>/dev/null || true
echo "Sleep 15s for Core migrations and Agent sync..."
sleep 15
echo -e "${GREEN}✅${NC} Rollout restart done"

# Step 11: Final verification
echo ""
echo -e "${BLUE}Step 11: Final verification...${NC}"
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
echo "=== Dashboard Status ==="
kubectl get deployment -n "$NAMESPACE" fortuna-dashboard 2>/dev/null || true
kubectl get pods -n "$NAMESPACE" -l app=fortuna-dashboard 2>/dev/null || true

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
    echo -e "${YELLOW}⚠️${NC}  Using IP-based connections (workaround). Core gRPC requires mTLS; Agent was set to TLS=false for IP fallback."
    echo "After fixing pod network/DNS (e.g. ./scripts/deploy/fix-flannel-vxlan.sh), restore DNS + mTLS for SBOM sync:"
    echo "  kubectl set env daemonset/fortuna-agent -n $NAMESPACE CORE_GRPC_ENDPOINT=\"fortuna-core.$NAMESPACE.svc.cluster.local:9090\" TLS_ENABLED=\"true\""
    echo "  kubectl rollout restart daemonset/fortuna-agent -n $NAMESPACE"
fi
echo ""

