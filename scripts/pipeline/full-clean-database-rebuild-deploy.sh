#!/usr/bin/env bash
# ============================================================================
# Full Clean (images + optional DB) → Rebuild (nerdctl/containerd) → Deploy
# ============================================================================
# Ensures all code changes are applied: clean removes ALL fortuna images + build
# cache; rebuild uses NO_CACHE when clean was run; deploy rollout restarts core,
# dashboard, agent so pods use the new images. See docs/FULL_CLEAN_REBUILD_DEPLOY_VERIFICATION.md
#
# 1. Clean: port-forwards, E2E namespaces, fortuna images by tag and by ID, system/builder prune.
# 2. Optional DB: run clear_all_cluster_data.sql (--db) or reset_database_full.sql (--db-reset).
#    Core runs all migrations on startup; --db-reset (DROP tables) ensures fresh schema
#    (e.g. migration 062: clusters.region/endpoint/kubeconfig — fixes agent sync 500 if missing).
# 3. Rebuild: core, agent, dashboard via build-and-load-containerd.sh (nerdctl → containerd k8s.io).
# 4. Deploy: addons, Flannel, StorageClass, deploy-fortuna-robust.sh; Phase 3b rollout restart (Core, Dashboard, Agent).
#
# Usage:
#   ./scripts/pipeline/full-clean-database-rebuild-deploy.sh              # interactive menu (no args)
#   ./scripts/pipeline/full-clean-database-rebuild-deploy.sh --menu       # force interactive menu
#   ./scripts/pipeline/full-clean-database-rebuild-deploy.sh --full      # clean + rebuild + deploy
#   ./scripts/pipeline/full-clean-database-rebuild-deploy.sh --db         # + clear DB data (DELETE, keep schema)
#   ./scripts/pipeline/full-clean-database-rebuild-deploy.sh --db-reset   # + full DB reset (DROP tables)
#   ./scripts/pipeline/full-clean-database-rebuild-deploy.sh --skip-rebuild   # clean + deploy only
#   ./scripts/pipeline/full-clean-database-rebuild-deploy.sh --skip-deploy    # clean + rebuild only
#   ./scripts/pipeline/full-clean-database-rebuild-deploy.sh --only-db-reset  # DB full reset only (no clean/rebuild/deploy)
#   RUN_ASYNC=1 ./scripts/pipeline/full-clean-database-rebuild-deploy.sh  # run in background
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
SCRIPTS="$PROJECT_ROOT/scripts"
NAMESPACE="${NAMESPACE:-fortuna}"
CONTAINERD_NS="${CONTAINERD_NAMESPACE:-k8s.io}"

# Run in background to avoid timeout (build takes 10–20+ min). Log: PIPELINE_LOG_FILE or /tmp/clean-rebuild-deploy.log
if [ "${RUN_ASYNC:-0}" = "1" ] || [ "${RUN_IN_BACKGROUND:-0}" = "1" ]; then
  LOG_FILE="${PIPELINE_LOG_FILE:-/tmp/clean-rebuild-deploy.log}"
  echo "Pipeline starting in background (avoids timeout). Log: $LOG_FILE"
  echo "  Monitor: tail -f $LOG_FILE"
  RUN_ASYNC=0 RUN_IN_BACKGROUND=0 nohup "$0" "$@" >> "$LOG_FILE" 2>&1 &
  PID=$!
  echo "  PID: $PID"
  exit 0
fi

SKIP_CLEAN=false
SKIP_REBUILD=false
SKIP_DEPLOY=false
CLEAN_DB=false
DB_RESET=false
SHOW_MENU=false

for arg in "$@"; do
  case "$arg" in
    --skip-clean)    SKIP_CLEAN=true ;;
    --skip-rebuild)  SKIP_REBUILD=true ;;
    --skip-deploy)   SKIP_DEPLOY=true ;;
    --db)            CLEAN_DB=true ;;
    --db-reset)       DB_RESET=true ;;
    --only-db-reset)  SKIP_CLEAN=true; SKIP_REBUILD=true; SKIP_DEPLOY=true; DB_RESET=true ;;
    --menu|-i)        SHOW_MENU=true ;;
    --full)           ;;  # no-extra flags = full pipeline
  esac
done

# ---- Interactive menu when no options given or --menu requested ----
if [ $# -eq 0 ] || [ "$SHOW_MENU" = true ]; then
  # Menu mode: choices override any flags (e.g. --menu --db → menu choice wins)
  SKIP_CLEAN=false
  SKIP_REBUILD=false
  SKIP_DEPLOY=false
  CLEAN_DB=false
  DB_RESET=false
  RED='\033[0;31m'
  GREEN='\033[0;32m'
  YELLOW='\033[1;33m'
  BLUE='\033[0;34m'
  CYAN='\033[0;36m'
  NC='\033[0m'
  echo ""
  echo -e "${CYAN}=========================================="
  echo "  Full Clean → Rebuild → Deploy (options)"
  echo "==========================================${NC}"
  echo ""
  echo "  1) Full pipeline          Clean images + Rebuild + Deploy (+ rollout restart)"
  echo "  2) Full + Clear DB        Same as 1 + clear DB data (DELETE, keep schema)"
  echo "  3) Full + Reset DB        Same as 1 + full DB reset (DROP tables, migrations on start)"
  echo "  4) Clean + Deploy only    Skip rebuild (use existing images, rollout restart)"
  echo "  5) Clean + Rebuild only   Skip deploy"
  echo "  6) Run in background      Same as 1, log to /tmp/clean-rebuild-deploy.log"
  echo "  7) Only reset DB          DB full reset only (DROP tables; no clean/rebuild/deploy)"
  echo "  0) Cancel"
  echo ""
  printf "  Select [1-7, 0]: "
  read -r choice
  choice="${choice:-0}"
  case "$choice" in
    1)  # full
        ;;
    2)  CLEAN_DB=true ;;
    3)  DB_RESET=true ;;
    4)  SKIP_REBUILD=true ;;
    5)  SKIP_DEPLOY=true ;;
    6)  RUN_ASYNC=1 "$0" --full
        exit 0
        ;;
    7)  SKIP_CLEAN=true; SKIP_REBUILD=true; SKIP_DEPLOY=true; DB_RESET=true ;;
    0|q|Q)
        echo "Cancelled."
        exit 0
        ;;
    *)
        echo "Invalid choice. Exiting."
        exit 1
        ;;
  esac
fi

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
  # Remove by tag first (so :latest and any VERSION tag are dropped)
  for img in fortuna-core:latest fortuna-agent:latest fortuna-dashboard:latest; do
    nerdctl --namespace "$CONTAINERD_NS" rmi --force "$img" 2>/dev/null || true
  done
  # Remove any remaining fortuna images by image ID (handles old/dangling refs)
  # Use a list to avoid pipeline exit 1 when grep finds nothing (set -o pipefail would exit script)
  fortuna_ids=""
  fortuna_ids=$(nerdctl --namespace "$CONTAINERD_NS" images 2>/dev/null | grep -E 'fortuna-(core|agent|dashboard)' | awk '{print $3}' | sort -u) || true
  for id in $fortuna_ids; do
    [ -n "$id" ] && [ "$id" != "ID" ] && nerdctl --namespace "$CONTAINERD_NS" rmi --force "$id" 2>/dev/null || true
  done
  log_info "Pruning containerd system and build cache..."
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
  POD=""
  for _ in 1 2 3 4 5 6 7 8 9 10 11 12; do
    POD=$(kubectl get pods -n "$NAMESPACE" -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
    [ -z "$POD" ] && sleep 5 && continue
    PHASE=$(kubectl get pod -n "$NAMESPACE" "$POD" -o jsonpath='{.status.phase}' 2>/dev/null || echo "")
    if [ "$PHASE" = "Running" ]; then
      break
    fi
    log_info "Postgres pod $POD phase=$PHASE (waiting for Running)..."
    sleep 5
  done
  if [ -z "$POD" ]; then
    log_warn "Postgres pod not found in namespace $NAMESPACE; skip DB clean. Deploy infra first."
  elif [ "$(kubectl get pod -n "$NAMESPACE" "$POD" -o jsonpath='{.status.phase}' 2>/dev/null)" != "Running" ]; then
    log_error "Postgres pod $POD is not Running (no host assigned). Wait for cluster/node then re-run with --db-reset, or run DB reset after deploy."
    exit 1
  else
    if [ "$DB_RESET" = true ]; then
      SQL_FILE="$PROJECT_ROOT/deploy/e2e/reset_database_full.sql"
      if [ -f "$SQL_FILE" ]; then
        if ! kubectl cp "$SQL_FILE" "$NAMESPACE/$POD:/tmp/reset_db.sql"; then
          log_error "kubectl cp reset_database_full.sql failed"
          exit 1
        fi
        if ! kubectl exec -n "$NAMESPACE" "$POD" -- psql -U postgres -d fortuna -f /tmp/reset_db.sql; then
          log_error "psql reset_database_full.sql failed"
          exit 1
        fi
        log_success "DB full reset (DROP tables) done. Core will re-run migrations on next start."
      else
        log_error "File not found: $SQL_FILE"
        exit 1
      fi
    else
      SQL_FILE="$PROJECT_ROOT/deploy/e2e/clear_all_cluster_data.sql"
      if [ -f "$SQL_FILE" ]; then
        if ! kubectl cp "$SQL_FILE" "$NAMESPACE/$POD:/tmp/clear_db.sql"; then
          log_error "kubectl cp clear_all_cluster_data.sql failed"
          exit 1
        fi
        if ! kubectl exec -n "$NAMESPACE" "$POD" -- psql -U postgres -d fortuna -f /tmp/clear_db.sql; then
          log_error "psql clear_all_cluster_data.sql failed"
          exit 1
        fi
        log_success "DB data cleared (DELETE, schema kept)"
      else
        log_error "File not found: $SQL_FILE"
        exit 1
      fi
    fi
  fi
  echo ""
fi

# ---- Phase 2: Rebuild (nerdctl → containerd) ----
if [ "$SKIP_REBUILD" = false ]; then
  log_info "Phase 2: Rebuild (core, agent, dashboard) with nerdctl..."
  cd "$PROJECT_ROOT"
  # After full clean (images + builder prune), force no cache so all layers rebuild from current source
  export NO_CACHE="${NO_CACHE:-false}"
  if [ "$SKIP_CLEAN" = false ]; then
    NO_CACHE=true
    log_info "NO_CACHE=true (clean was run; ensure fresh build)"
  fi
  if [ -x "$SCRIPTS/build/build-and-load-containerd.sh" ]; then
    SKIP_DASHBOARD="${SKIP_DASHBOARD:-false}" NO_CACHE="$NO_CACHE" "$SCRIPTS/build/build-and-load-containerd.sh" || {
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

# ---- Phase 2b: Push images to all nodes (including master) so Core/Agent find image with imagePullPolicy: Never ----
PUSH_IMAGES_AFTER_REBUILD="${PUSH_IMAGES_AFTER_REBUILD:-true}"
if [ "$SKIP_DEPLOY" = false ] && [ "$SKIP_REBUILD" = false ] && [ "$PUSH_IMAGES_AFTER_REBUILD" = true ]; then
  if [ -x "$SCRIPTS/utils/push-images-to-workers.sh" ]; then
    log_info "Phase 2b: Push images to all nodes (master + workers) so Core pod can start..."
    if "$SCRIPTS/utils/push-images-to-workers.sh" 2>&1; then
      log_success "Images pushed to all nodes"
    else
      log_warn "Push to nodes failed (SSH or node list). Add scripts/utils/push-images.config (see push-images.config.example) or set SSH_USER/SSH_PASS; if Core shows ErrImageNeverPull run: ./scripts/utils/push-images-to-workers.sh"
    fi
  else
    log_warn "push-images-to-workers.sh not found; if Core shows ErrImageNeverPull, run it after build to copy images to the node that runs Core."
  fi
  echo ""
fi

# ---- Phase 2a: Ensure cluster addons (kube-proxy, CoreDNS) so ClusterIP/CNI work ----
if [ "$SKIP_DEPLOY" = false ]; then
  log_info "Phase 2a: Ensure cluster addons (kube-proxy, CoreDNS)..."
  if [ -x "$SCRIPTS/deploy/ensure-cluster-addons.sh" ]; then
    if "$SCRIPTS/deploy/ensure-cluster-addons.sh" 2>/dev/null; then
      log_success "Cluster addons ready"
    else
      log_warn "ensure-cluster-addons.sh had warnings; ClusterIP/DNS may fail if addons missing"
    fi
    log_info "Sleep 15s for addons to stabilize..."
    sleep 15
  else
    log_warn "ensure-cluster-addons.sh not found; if ClusterIP/DNS fail, install kube-proxy and CoreDNS (kubeadm phase addon)"
  fi
  echo ""
fi

# ---- Phase 2a2: Ensure Flannel CNI so pod network works (avoids subnet.env / local-path-provisioner stuck) ----
if [ "$SKIP_DEPLOY" = false ]; then
  log_info "Phase 2a2: Ensure Flannel CNI (install if missing)..."
  if [ -x "$SCRIPTS/deploy/ensure-flannel.sh" ]; then
    if "$SCRIPTS/deploy/ensure-flannel.sh" 2>/dev/null; then
      log_success "Flannel CNI ready"
    else
      log_warn "ensure-flannel.sh had warnings; if pods stay ContainerCreating (subnet.env), install Flannel: kubectl apply -f https://raw.githubusercontent.com/flannel-io/flannel/v0.26.0/Documentation/kube-flannel.yml"
    fi
    log_info "Sleep 10s for Flannel to stabilize..."
    sleep 10
  else
    log_warn "ensure-flannel.sh not found; if pod network fails, install Flannel before StorageClass"
  fi
  echo ""
fi

# ---- Phase 2c: Ensure StorageClass (local-path) for PVCs ----
if [ "$SKIP_DEPLOY" = false ]; then
  log_info "Phase 2c: Ensure StorageClass (local-path) for PostgreSQL/NATS PVCs..."
  if [ -x "$SCRIPTS/deploy/ensure-storage-class.sh" ]; then
    if "$SCRIPTS/deploy/ensure-storage-class.sh" 2>/dev/null; then
      log_success "StorageClass ready"
    else
      log_warn "ensure-storage-class.sh failed or StorageClass not ready; PVCs may stay Pending. Install manually: kubectl apply -f https://raw.githubusercontent.com/rancher/local-path-provisioner/v0.0.24/deploy/local-path-storage.yaml"
    fi
  else
    log_warn "ensure-storage-class.sh not found; if PVCs stay Pending, install local-path-provisioner (see docs/01-getting-started/ENVIRONMENT_PREPARATION.md)"
  fi
  log_info "Sleep 10s so PVCs can bind when infra is deployed..."
  sleep 10
  echo ""
fi

# ---- Phase 3a: CNI (Flannel) check/fix for multi-node – short wait so script does not hang ----
if [ "$SKIP_DEPLOY" = false ]; then
  NODE_COUNT=$(kubectl get nodes --no-headers 2>/dev/null | wc -l || echo "0")
  if [ "${NODE_COUNT:-0}" -gt 1 ] && [ -x "$SCRIPTS/deploy/fix-flannel-vxlan.sh" ]; then
    log_info "Phase 3a: CNI (Flannel) check/fix (multi-node, short wait)..."
    CNI_WAIT_SECONDS=15 "$SCRIPTS/deploy/fix-flannel-vxlan.sh" 2>/dev/null || log_warn "CNI fix skipped or had warnings"
    log_info "Sleep 10s after CNI fix..."
    sleep 10
  fi
fi

# ---- Phase 3: Deploy ----
if [ "$SKIP_DEPLOY" = false ]; then
  log_info "Phase 3: Deploy (infra, RBAC, core, agent, dashboard)..."
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
  # Rollout restart is done inside deploy-fortuna-robust.sh (Step 10); here as safety net if deploy was partial
  log_info "Phase 3b: Rollout restart (Core, Dashboard, Agent) to use new images..."
  kubectl rollout restart deployment/fortuna-core -n "$NAMESPACE" --timeout=60s 2>/dev/null || true
  kubectl rollout restart deployment/fortuna-dashboard -n "$NAMESPACE" --timeout=90s 2>/dev/null || true
  kubectl rollout restart daemonset/fortuna-agent -n "$NAMESPACE" --timeout=90s 2>/dev/null || true
  log_info "Waiting for Core rollout (max 120s)..."
  kubectl rollout status deployment/fortuna-core -n "$NAMESPACE" --timeout=120s 2>/dev/null || log_warn "Core rollout status check failed or timed out"
  log_info "Waiting for Dashboard rollout (max 90s)..."
  kubectl rollout status deployment/fortuna-dashboard -n "$NAMESPACE" --timeout=90s 2>/dev/null || log_warn "Dashboard rollout status check failed or timed out"
  log_info "Waiting for Agent DaemonSet rollout (max 120s)..."
  kubectl rollout status daemonset/fortuna-agent -n "$NAMESPACE" --timeout=120s 2>/dev/null || log_warn "Agent DaemonSet rollout status check failed or timed out"
  log_info "Phase 3c: Verify rollout (core, dashboard, agent)..."
  CORE_OK=false; DASH_OK=false; AGENT_OK=false
  # Short timeout: if already complete, returns 0 immediately; else wait up to 5s
  kubectl rollout status deployment/fortuna-core -n "$NAMESPACE" --timeout=5s 2>/dev/null && CORE_OK=true || true
  kubectl rollout status deployment/fortuna-dashboard -n "$NAMESPACE" --timeout=5s 2>/dev/null && DASH_OK=true || true
  kubectl rollout status daemonset/fortuna-agent -n "$NAMESPACE" --timeout=5s 2>/dev/null && AGENT_OK=true || true
  if [ "$CORE_OK" = true ]; then log_success "  Core: rolled out"; else log_warn "  Core: not rolled out or still updating"; fi
  if [ "$DASH_OK" = true ]; then log_success "  Dashboard: rolled out"; else log_warn "  Dashboard: not rolled out or still updating"; fi
  if [ "$AGENT_OK" = true ]; then log_success "  Agent: rolled out"; else log_warn "  Agent: not rolled out or still updating"; fi
  log_info "Sleep 10s for Agent sync to run..."
  sleep 10
  log_success "Deploy complete"
else
  log_info "Phase 3: Deploy (skipped)"
fi

echo ""
echo "=========================================="
log_success "Full clean / rebuild / deploy finished."
echo "=========================================="
echo "  Verify: kubectl get pods -n $NAMESPACE"
echo "  Rollout status: ./scripts/pipeline/verify-rollout.sh   (or: kubectl rollout status deployment/fortuna-core deployment/fortuna-dashboard daemonset/fortuna-agent -n $NAMESPACE)"
echo "  Rebuild/deploy status (image ID vs local): ./scripts/verify/verify-core-agent-rebuild-deploy-status.sh"
echo "  Core API: kubectl port-forward -n $NAMESPACE svc/fortuna-core 8080:8080"
echo "  Dashboard: kubectl port-forward -n $NAMESPACE svc/fortuna-dashboard 8081:80"
echo "  Agents: wait 1–2 min; GET /api/v1/agents/status"
echo "  If agent shows [Syncer] Sync failed: status=500, check Core logs for 'column kubeconfig does not exist';"
echo "    fix: run with --db-reset and redeploy, or add clusters.kubeconfig (migration 062) and restart Core."
echo "  If Core pod shows ErrImageNeverPull: image must be on the node that runs Core (control-plane)."
echo "    fix: ./scripts/utils/push-images-to-workers.sh (pushes to all nodes including master; set SSH_USER/SSH_PASS or use keys)."
echo "  If Agent CrashLoopBackOff (OOMKilled): daemonset has memory limit 2Gi; optional SBOM_WORKERS=1 and rebuild agent."
echo "  Monitor errors: ./scripts/monitor/monitor-agent-core-errors.sh (or --follow). Full troubleshooting: docs/AGENT_CORE_ERRORS_MONITOR.md"
echo ""
