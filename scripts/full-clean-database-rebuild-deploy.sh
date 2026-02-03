#!/usr/bin/env bash
# ============================================================================
# Full Clean (images + optional DB) → Rebuild (nerdctl/containerd) → Deploy
# ============================================================================
# 1. Clean: port-forwards, E2E namespaces, fortuna images (nerdctl), prune.
# 2. Optional DB: run clear_all_cluster_data.sql (--db) or reset_database_full.sql (--db-reset).
# 3. Rebuild: core, agent, dashboard via build-and-load-containerd.sh (nerdctl → containerd k8s.io).
# 4. Deploy: Phase 3a CNI (Flannel) check/fix for multi-node (short wait);
#    deploy-fortuna-robust.sh (infra, RBAC, core, agent, dashboard, CNI, rollout restart);
#    Phase 3b rollout restart Core/Dashboard/Agent to use new images.
#
# Usage:
#   ./scripts/full-clean-database-rebuild-deploy.sh              # clean images + rebuild + deploy
#   ./scripts/full-clean-database-rebuild-deploy.sh --db         # + clear DB data (DELETE, keep schema)
#   ./scripts/full-clean-database-rebuild-deploy.sh --db-reset    # + full DB reset (DROP tables; Core will re-run migrations)
#   ./scripts/full-clean-database-rebuild-deploy.sh --skip-rebuild   # clean + deploy only (use existing images)
#   ./scripts/full-clean-database-rebuild-deploy.sh --skip-deploy    # clean + rebuild only
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
DB_RESET=false
for arg in "$@"; do
  case "$arg" in
    --skip-clean)    SKIP_CLEAN=true ;;
    --skip-rebuild) SKIP_REBUILD=true ;;
    --skip-deploy)  SKIP_DEPLOY=true ;;
    --db)           CLEAN_DB=true ;;
    --db-reset)     DB_RESET=true ;;
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
echo "Full Clean (images + optional DB) → Rebuild → Deploy"
echo "=========================================="
echo "  Skip clean: $SKIP_CLEAN | Skip rebuild: $SKIP_REBUILD | Skip deploy: $SKIP_DEPLOY"
echo "  Clean DB (delete data): $CLEAN_DB | DB full reset (drop tables): $DB_RESET"
echo ""

# ---- Phase 1: Clean ----
if [ "$SKIP_CLEAN" = false ]; then
  log_info "Phase 1: Clean (port-forwards, E2E ns, fortuna images, prune)..."
  pkill -f "kubectl.*port-forward" 2>/dev/null || true
  for ns in fortuna-e2e fortuna-e2e-2025; do
    kubectl get namespace "$ns" 2>/dev/null && kubectl delete namespace "$ns" --timeout=60s 2>/dev/null || true
  done
  log_info "Removing ALL fortuna images from containerd (namespace=$CONTAINERD_NS)..."
  nerdctl --namespace "$CONTAINERD_NS" images 2>/dev/null | grep fortuna | awk '{print $3}' | xargs -r nerdctl --namespace "$CONTAINERD_NS" rmi --force 2>/dev/null || true
  nerdctl --namespace "$CONTAINERD_NS" system prune -f 2>/dev/null || true
  nerdctl builder prune --namespace "$CONTAINERD_NS" -a -f 2>/dev/null || true
  log_success "Clean complete"
else
  log_info "Phase 1: Clean (skipped)"
fi
echo ""

# ---- Phase 1b: Database clean (optional) ----
if [ "$CLEAN_DB" = true ] || [ "$DB_RESET" = true ]; then
  log_info "Phase 1b: Database clean..."
  POD=$(kubectl get pods -n "$NAMESPACE" -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
  if [ -z "$POD" ]; then
    log_warn "Postgres pod not found in namespace $NAMESPACE; skip DB clean. Deploy infra first."
  else
    if [ "$DB_RESET" = true ]; then
      SQL_FILE="$PROJECT_ROOT/deploy/e2e/reset_database_full.sql"
      if [ -f "$SQL_FILE" ]; then
        kubectl cp "$SQL_FILE" "$NAMESPACE/$POD:/tmp/reset_db.sql" 2>/dev/null || true
        kubectl exec -n "$NAMESPACE" "$POD" -- psql -U postgres -d fortuna -f /tmp/reset_db.sql 2>/dev/null || true
        log_success "DB full reset (DROP tables) done. Core will re-run migrations on next start."
      else
        log_error "File not found: $SQL_FILE"
      fi
    else
      SQL_FILE="$PROJECT_ROOT/deploy/e2e/clear_all_cluster_data.sql"
      if [ -f "$SQL_FILE" ]; then
        kubectl cp "$SQL_FILE" "$NAMESPACE/$POD:/tmp/clear_db.sql" 2>/dev/null || true
        kubectl exec -n "$NAMESPACE" "$POD" -- psql -U postgres -d fortuna -f /tmp/clear_db.sql 2>/dev/null || true
        log_success "DB data cleared (DELETE, schema kept)"
      else
        log_error "File not found: $SQL_FILE"
      fi
    fi
  fi
  echo ""
fi

# ---- Phase 2: Rebuild (nerdctl → containerd) ----
if [ "$SKIP_REBUILD" = false ]; then
  log_info "Phase 2: Rebuild (core, agent, dashboard) with nerdctl..."
  cd "$PROJECT_ROOT"
  if [ -x "$SCRIPT_DIR/build-and-load-containerd.sh" ]; then
    SKIP_DASHBOARD="${SKIP_DASHBOARD:-false}" "$SCRIPT_DIR/build-and-load-containerd.sh" || {
      log_warn "Build script had errors (e.g. agent build may fail). Continuing deploy with existing images."
    }
  else
    log_error "build-and-load-containerd.sh not found or not executable"
    exit 1
  fi
  log_success "Rebuild phase done"
else
  log_info "Phase 2: Rebuild (skipped)"
fi
echo ""

# ---- Phase 3a: CNI (Flannel) check/fix for multi-node – short wait so script does not hang ----
if [ "$SKIP_DEPLOY" = false ]; then
  NODE_COUNT=$(kubectl get nodes --no-headers 2>/dev/null | wc -l || echo "0")
  if [ "${NODE_COUNT:-0}" -gt 1 ] && [ -x "$SCRIPT_DIR/fix-flannel-vxlan.sh" ]; then
    log_info "Phase 3a: CNI (Flannel) check/fix (multi-node, short wait)..."
    CNI_WAIT_SECONDS=10 "$SCRIPT_DIR/fix-flannel-vxlan.sh" 2>/dev/null || log_warn "CNI fix skipped or had warnings"
  fi
fi

# ---- Phase 3: Deploy ----
if [ "$SKIP_DEPLOY" = false ]; then
  log_info "Phase 3: Deploy (infra, RBAC, core, agent, dashboard)..."
  if [ -x "$SCRIPT_DIR/deploy-fortuna-robust.sh" ]; then
    "$SCRIPT_DIR/deploy-fortuna-robust.sh"
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
  # Rollout restart is done inside deploy-fortuna-robust.sh (Step 10); here as safety net if deploy was partial
  log_info "Phase 3b: Rollout restart (Core, Dashboard, Agent) to use new images..."
  kubectl rollout restart deployment/fortuna-core -n "$NAMESPACE" --timeout=60s 2>/dev/null || true
  kubectl rollout restart deployment/fortuna-dashboard -n "$NAMESPACE" --timeout=90s 2>/dev/null || true
  kubectl rollout restart daemonset/fortuna-agent -n "$NAMESPACE" --timeout=90s 2>/dev/null || true
  log_success "Deploy complete"
else
  log_info "Phase 3: Deploy (skipped)"
fi

echo ""
echo "=========================================="
log_success "Full clean / rebuild / deploy finished."
echo "=========================================="
echo "  Verify: kubectl get pods -n $NAMESPACE"
echo "  Core API: kubectl port-forward -n $NAMESPACE svc/fortuna-core 8080:8080"
echo "  Dashboard: kubectl port-forward -n $NAMESPACE svc/fortuna-dashboard 8081:80"
echo "  Agents on Dashboard: wait 1–2 min after deploy for RegisterAgent + Ping; then GET /api/v1/agents/status"
echo ""
