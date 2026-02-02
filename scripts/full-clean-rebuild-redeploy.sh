#!/usr/bin/env bash
# ============================================================================
# Full Clean, Clear Cache, Rebuild All, Redeploy All
# ============================================================================
# 1. Clean: stop port-forwards, delete E2E namespaces, completed pods,
#    remove ALL fortuna images (core, agent, dashboard), system prune, build cache prune.
# 2. Rebuild: core, agent, dashboard (nerdctl build and load into containerd).
# 3. Redeploy: infrastructure (postgres, nats), RBAC, core, agent, dashboard.
# Usage: ./scripts/full-clean-rebuild-redeploy.sh [--skip-clean] [--skip-rebuild] [--skip-deploy] [--db]
#   --db: also clean E2E test data from database (optional).
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
NAMESPACE="${NAMESPACE:-fortuna}"
CONTAINERD_NS="${CONTAINERD_NAMESPACE:-k8s.io}"

SKIP_CLEAN=false
SKIP_REBUILD=false
SKIP_DEPLOY=false
CLEAN_DB=false
for arg in "$@"; do
  case "$arg" in
    --skip-clean)   SKIP_CLEAN=true ;;
    --skip-rebuild) SKIP_REBUILD=true ;;
    --skip-deploy)  SKIP_DEPLOY=true ;;
    --db)           CLEAN_DB=true ;;
  esac
done

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'
log_info()    { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[OK]${NC} $1"; }
log_warn()    { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error()   { echo -e "${RED}[ERR]${NC} $1"; }

echo "=========================================="
echo "Full Clean → Rebuild → Redeploy"
echo "=========================================="
echo "Skip clean: $SKIP_CLEAN | Skip rebuild: $SKIP_REBUILD | Skip deploy: $SKIP_DEPLOY | Clean DB: $CLEAN_DB"
echo ""

# ---- Phase 1: Clean ----
if [ "$SKIP_CLEAN" = false ]; then
  log_info "Phase 1/3: Clean (all images, cache, port-forwards)..."
  pkill -f "kubectl.*port-forward" 2>/dev/null || true
  log_info "Stopped port-forwards"
  for ns in fortuna-e2e fortuna-e2e-2025; do
    kubectl get namespace "$ns" 2>/dev/null && kubectl delete namespace "$ns" --timeout=60s 2>/dev/null || true
  done
  if command -v jq >/dev/null 2>&1; then
    kubectl get pods -A -o json 2>/dev/null | jq -r '.items[] | select(.status.phase=="Succeeded" or .status.phase=="Failed" or .status.phase=="Evicted") | "\(.metadata.namespace) \(.metadata.name)"' 2>/dev/null | while read -r ns name; do
      [ -n "$ns" ] && [ -n "$name" ] && kubectl delete pod -n "$ns" "$name" --ignore-not-found 2>/dev/null || true
    done
  fi
  log_info "Removing ALL fortuna images..."
  nerdctl --namespace "$CONTAINERD_NS" images 2>/dev/null | grep fortuna | awk '{print $3}' | xargs -r nerdctl --namespace "$CONTAINERD_NS" rmi --force 2>/dev/null || true
  log_info "System prune and build cache prune..."
  nerdctl --namespace "$CONTAINERD_NS" system prune -f 2>/dev/null || true
  nerdctl builder prune --namespace "$CONTAINERD_NS" -a -f 2>/dev/null || true
  if [ "$CLEAN_DB" = true ]; then
    POD=$(kubectl get pods -n "$NAMESPACE" -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
    if [ -n "$POD" ] && [ -f "$PROJECT_ROOT/deploy/e2e/clear_e2e_test_data.sql" ]; then
      kubectl cp "$PROJECT_ROOT/deploy/e2e/clear_e2e_test_data.sql" "$NAMESPACE/$POD:/tmp/clear_e2e.sql" 2>/dev/null || true
      kubectl exec -n "$NAMESPACE" "$POD" -- psql -U postgres -d fortuna -f /tmp/clear_e2e.sql 2>/dev/null || true
      log_info "Database E2E data cleaned"
    fi
  fi
  log_success "Clean complete"
else
  log_info "Phase 1/3: Clean (skipped)"
fi
echo ""

# ---- Phase 2: Rebuild ----
if [ "$SKIP_REBUILD" = false ]; then
  log_info "Phase 2/3: Rebuild (core, agent, dashboard)..."
  cd "$PROJECT_ROOT"
  if [ -x "$SCRIPT_DIR/build-and-load-containerd.sh" ]; then
    SKIP_DASHBOARD=false "$SCRIPT_DIR/build-and-load-containerd.sh"
  else
    log_error "build-and-load-containerd.sh not found or not executable"
    exit 1
  fi
  log_success "Rebuild complete"
else
  log_info "Phase 2/3: Rebuild (skipped)"
fi
echo ""

# ---- Phase 3: Redeploy ----
if [ "$SKIP_DEPLOY" = false ]; then
  log_info "Phase 3/3: Redeploy (infra, core, agent, dashboard)..."
  if [ -x "$SCRIPT_DIR/deploy-fortuna-robust.sh" ]; then
    "$SCRIPT_DIR/deploy-fortuna-robust.sh"
  else
    kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -
    [ -f "$PROJECT_ROOT/deploy/infrastructure/postgresql-with-age.yaml" ] && kubectl apply -f "$PROJECT_ROOT/deploy/infrastructure/postgresql-with-age.yaml" || true
    [ -f "$PROJECT_ROOT/deploy/infrastructure/nats.yaml" ] && kubectl apply -f "$PROJECT_ROOT/deploy/infrastructure/nats.yaml" || true
    [ -f "$PROJECT_ROOT/deploy/fortuna-rbac.yaml" ] && kubectl apply -f "$PROJECT_ROOT/deploy/fortuna-rbac.yaml" || true
    [ -f "$PROJECT_ROOT/deploy/fortuna-core-deployment.yaml" ] && kubectl apply -f "$PROJECT_ROOT/deploy/fortuna-core-deployment.yaml" || true
    [ -f "$PROJECT_ROOT/deploy/fortuna-agent-daemonset.yaml" ] && kubectl apply -f "$PROJECT_ROOT/deploy/fortuna-agent-daemonset.yaml" || true
  fi
  if [ -f "$PROJECT_ROOT/deploy/dashboard-deployment.yaml" ]; then
    kubectl apply -f "$PROJECT_ROOT/deploy/dashboard-deployment.yaml"
    kubectl rollout restart deployment/fortuna-dashboard -n "$NAMESPACE" --timeout=120s 2>/dev/null || true
    log_success "Dashboard deployed and rollout restarted"
  else
    log_warn "dashboard-deployment.yaml not found, skipping dashboard deploy"
  fi
  log_success "Redeploy complete"
else
  log_info "Phase 3/3: Redeploy (skipped)"
fi

echo ""
echo "=========================================="
log_success "Full clean / rebuild / redeploy finished."
echo "=========================================="
echo "Verify: kubectl get pods -n $NAMESPACE"
echo "Core API: kubectl port-forward -n $NAMESPACE svc/fortuna-core 8080:8080"
echo "Dashboard: kubectl port-forward -n $NAMESPACE svc/fortuna-dashboard 8081:80"
echo ""
