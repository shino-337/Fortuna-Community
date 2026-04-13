#!/usr/bin/env bash
# ============================================================================
# Full Rebuild → Sync Image Tag in Deploy YAMLs → Deploy → E2E Report
# ============================================================================
# Unlike full-clean-database-rebuild-deploy.sh, this script syncs image tag into
# deploy YAMLs and runs E2E report. Core runs DB migrations on startup (e.g. 062:
# clusters.region/endpoint/kubeconfig — required for agent sync to succeed).
#
# 1. Clean: port-forwards, E2E ns, fortuna images (nerdctl/docker), prune.
# 2. Rebuild: core, agent, dashboard (NO_CACHE) via build-and-load-containerd.sh (supports nerdctl/docker/buildctl).
# 3. Sync tag: get VERSION from build, update deploy/*.yaml image: fortuna-*:VERSION.
# 4. Deploy: infra, RBAC, core, agent, dashboard; rollout restart.
# 5. Push images to worker nodes if multi-node (optional, set WORKER_NODES).
# 6. Wait for Core/Dashboard/Agent ready.
# 7. Run E2E comprehensive + capability tests; write report to docs/test-results/.
#
# Usage:
#   ./scripts/pipeline/full-rebuild-sync-deploy-and-e2e.sh
#   ./scripts/pipeline/full-rebuild-sync-deploy-and-e2e.sh --skip-push-workers
#   ./scripts/pipeline/full-rebuild-sync-deploy-and-e2e.sh --skip-e2e
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
SCRIPTS="$PROJECT_ROOT/scripts"
NAMESPACE="${NAMESPACE:-fortuna}"
CONTAINERD_NS="${CONTAINERD_NAMESPACE:-k8s.io}"
SKIP_PUSH_WORKERS=false
SKIP_E2E=false
for arg in "$@"; do
  case "$arg" in
    --skip-push-workers) SKIP_PUSH_WORKERS=true ;;
    --skip-e2e)         SKIP_E2E=true ;;
  esac
done

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'
log_info()    { echo -e "${BLUE}[INFO]${NC} $1"; }
log_ok()      { echo -e "${GREEN}[OK]${NC} $1"; }
log_warn()    { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_err()     { echo -e "${RED}[ERR]${NC} $1"; }

echo "=========================================="
echo "Full Rebuild → Sync Tag → Deploy → E2E"
echo "=========================================="
echo "  Skip push workers: $SKIP_PUSH_WORKERS | Skip E2E: $SKIP_E2E"
echo ""

# ---- Phase 1: Clean ----
log_info "Phase 1: Clean (port-forwards, E2E ns, fortuna images, prune)..."
pkill -f "kubectl.*port-forward" 2>/dev/null || true
for ns in fortuna-e2e fortuna-e2e-2025; do
  kubectl get namespace "$ns" 2>/dev/null && kubectl delete namespace "$ns" --timeout=60s 2>/dev/null || true
done
log_info "Removing fortuna images..."
if command -v nerdctl &>/dev/null; then
  nerdctl --namespace "$CONTAINERD_NS" images 2>/dev/null | grep fortuna | awk '{print $3}' | xargs -r nerdctl --namespace "$CONTAINERD_NS" rmi --force 2>/dev/null || true
  nerdctl --namespace "$CONTAINERD_NS" system prune -f 2>/dev/null || true
  nerdctl builder prune --namespace "$CONTAINERD_NS" -a -f 2>/dev/null || true
fi
if command -v docker &>/dev/null && docker info &>/dev/null 2>&1; then
  for img in fortuna-core fortuna-agent fortuna-dashboard; do
    docker images --format '{{.Repository}}:{{.Tag}}' 2>/dev/null | grep "^${img}:" | xargs -r docker rmi --force 2>/dev/null || true
  done
  docker image prune -f 2>/dev/null || true
fi
log_ok "Clean complete"
echo ""

# ---- Phase 2: Rebuild ----
log_info "Phase 2: Rebuild (core, agent, dashboard) NO_CACHE..."
cd "$PROJECT_ROOT"
export NO_CACHE=true
export VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo 'latest')}"
if [ -x "$SCRIPTS/build/build-and-load-containerd.sh" ]; then
  SKIP_DASHBOARD="${SKIP_DASHBOARD:-false}" "$SCRIPTS/build/build-and-load-containerd.sh" || {
    log_warn "Build had errors; continuing with existing images if any."
  }
else
  log_err "build-and-load-containerd.sh not found or not executable"
  exit 1
fi
TAG="${VERSION}"
log_ok "Rebuild done. Image tag: $TAG"
echo ""

# ---- Phase 3: Sync image tag in deploy YAMLs ----
log_info "Phase 3: Syncing image tag ($TAG) in deploy YAMLs..."
[ -f "$PROJECT_ROOT/deploy/fortuna-core-deployment.yaml" ] && sed -i "s@image: fortuna-core:[^[:space:]]*@image: fortuna-core:${TAG}@g" "$PROJECT_ROOT/deploy/fortuna-core-deployment.yaml" && log_ok "Updated fortuna-core-deployment.yaml"
[ -f "$PROJECT_ROOT/deploy/fortuna-agent-daemonset.yaml" ] && sed -i "s@image: fortuna-agent:[^[:space:]]*@image: fortuna-agent:${TAG}@g" "$PROJECT_ROOT/deploy/fortuna-agent-daemonset.yaml" && log_ok "Updated fortuna-agent-daemonset.yaml"
[ -f "$PROJECT_ROOT/deploy/dashboard-deployment.yaml" ] && sed -i "s@image: fortuna-dashboard:[^[:space:]]*@image: fortuna-dashboard:${TAG}@g" "$PROJECT_ROOT/deploy/dashboard-deployment.yaml" && log_ok "Updated dashboard-deployment.yaml"
log_ok "Sync tag complete"
echo ""

# ---- Phase 4: Deploy ----
log_info "Phase 4: Deploy (infra, RBAC, core, agent, dashboard)..."
if [ -x "$SCRIPTS/deploy/deploy-fortuna-robust.sh" ]; then
  "$SCRIPTS/deploy/deploy-fortuna-robust.sh"
else
  kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -
  [ -f "$PROJECT_ROOT/deploy/infrastructure/postgresql.yaml" ] && kubectl apply -f "$PROJECT_ROOT/deploy/infrastructure/postgresql.yaml" || true
  [ -f "$PROJECT_ROOT/deploy/infrastructure/postgresql-with-age.yaml" ] && kubectl apply -f "$PROJECT_ROOT/deploy/infrastructure/postgresql-with-age.yaml" || true
  [ -f "$PROJECT_ROOT/deploy/infrastructure/nats.yaml" ] && kubectl apply -f "$PROJECT_ROOT/deploy/infrastructure/nats.yaml" || true
  [ -f "$PROJECT_ROOT/deploy/fortuna-rbac.yaml" ] && kubectl apply -f "$PROJECT_ROOT/deploy/fortuna-rbac.yaml" || true
  [ -f "$PROJECT_ROOT/deploy/fortuna-core-deployment.yaml" ] && kubectl apply -f "$PROJECT_ROOT/deploy/fortuna-core-deployment.yaml" || true
  [ -f "$PROJECT_ROOT/deploy/fortuna-agent-daemonset.yaml" ] && kubectl apply -f "$PROJECT_ROOT/deploy/fortuna-agent-daemonset.yaml" || true
fi
[ -f "$PROJECT_ROOT/deploy/dashboard-nginx-configmap.yaml" ] && kubectl apply -f "$PROJECT_ROOT/deploy/dashboard-nginx-configmap.yaml" || true
[ -f "$PROJECT_ROOT/deploy/dashboard-deployment.yaml" ] && kubectl apply -f "$PROJECT_ROOT/deploy/dashboard-deployment.yaml" || true
log_info "Rollout restart Core, Dashboard, Agent..."
kubectl rollout restart deployment/fortuna-core -n "$NAMESPACE" 2>/dev/null || true
kubectl rollout restart deployment/fortuna-dashboard -n "$NAMESPACE" 2>/dev/null || true
kubectl rollout restart daemonset/fortuna-agent -n "$NAMESPACE" 2>/dev/null || true
log_ok "Deploy complete"
echo ""

# ---- Phase 5: Push images to workers (optional) ----
if [ "$SKIP_PUSH_WORKERS" = false ]; then
  NODE_COUNT=$(kubectl get nodes --no-headers 2>/dev/null | wc -l || echo "0")
  if [ "${NODE_COUNT:-0}" -gt 1 ] && [ -x "$SCRIPTS/utils/push-images-to-workers.sh" ]; then
    log_info "Phase 5: Pushing images to worker nodes..."
    export CORE_IMAGE="fortuna-core:${TAG}"
    export AGENT_IMAGE="fortuna-agent:${TAG}"
    export WORKER_NODES="${WORKER_NODES:-}"
    if [ -n "$WORKER_NODES" ]; then
      "$SCRIPTS/utils/push-images-to-workers.sh" || log_warn "Push to workers had errors"
    else
      log_info "WORKER_NODES not set; skip push. Set e.g. WORKER_NODES=192.168.56.101 to push."
    fi
  else
    log_info "Phase 5: Single node or push script missing; skip push to workers."
  fi
else
  log_info "Phase 5: Skip push to workers (--skip-push-workers)."
fi
echo ""

# ---- Phase 6: Wait for workloads ----
log_info "Phase 6: Waiting for Core/Dashboard/Agent..."
kubectl rollout status deployment/fortuna-core -n "$NAMESPACE" --timeout=120s 2>/dev/null || log_warn "Core rollout status timeout"
kubectl rollout status deployment/fortuna-dashboard -n "$NAMESPACE" --timeout=90s 2>/dev/null || log_warn "Dashboard rollout status timeout"
kubectl rollout status daemonset/fortuna-agent -n "$NAMESPACE" --timeout=120s 2>/dev/null || log_warn "Agent rollout status timeout"
log_ok "Workloads ready"
echo ""

# ---- Phase 7: E2E + capability report ----
if [ "$SKIP_E2E" = false ] && [ -x "$SCRIPTS/e2e/run-e2e-with-capability-report.sh" ]; then
  log_info "Phase 7: Running E2E + capability tests and writing report..."
  "$SCRIPTS/e2e/run-e2e-with-capability-report.sh"
  log_ok "E2E report done"
elif [ "$SKIP_E2E" = false ] && [ -x "$SCRIPTS/e2e/run-e2e-full.sh" ]; then
  log_info "Phase 7: Running E2E full report..."
  "$SCRIPTS/e2e/run-e2e-full.sh"
  log_ok "E2E report done"
else
  log_info "Phase 7: E2E skipped (--skip-e2e or script not found)."
fi

echo ""
echo "=========================================="
log_ok "Full rebuild / sync / deploy / E2E finished."
echo "=========================================="
echo "  Image tag: $TAG"
echo "  Verify: kubectl get pods -n $NAMESPACE"
echo "  Report: docs/test-results/E2E-WITH-CAPABILITY-*.md or E2E-FULL-*.md"
echo "  Agent sync: if 500, ensure Core ran migrations (clusters.kubeconfig); see full-clean-database-rebuild-deploy.sh header."
echo ""
