#!/usr/bin/env bash
# ============================================================================
# Full Clean (images + optional DB) → Rebuild → Deploy
# ============================================================================
# Ensures all code changes are applied: clean removes ALL fortuna images + build
# cache; rebuild uses NO_CACHE when clean was run; deploy rollout restarts core,
# dashboard, agent so pods use the new images.
#
# Build tool: auto-detects nerdctl, docker, or buildctl (override: BUILD_TOOL=docker).
# When using Docker, images are built with `docker build` then imported into
# containerd via `ctr -n k8s.io images import` so kubelet sees them.
#
# 1. Clean: port-forwards, E2E namespaces, fortuna images by tag and by ID, system/builder prune.
# 2. Optional DB: run clear_all_cluster_data.sql (--db) or reset_database_full.sql (--db-reset).
#    When deploy runs, DB clean happens in Phase 2d AFTER Flannel + StorageClass + apply Postgres (PVC must bind).
#    When --only-db-reset, DB clean runs in Phase 1b (Postgres must already exist).
#    Env: PG_RECREATE_PVC=1 scales postgres to 0, deletes PVC postgres-pvc, reapplies manifest (fixes corrupt
#    data dir: "invalid primary checkpoint" / CrashLoopBackOff). Destroys all DB files on that PVC.
#    Core runs all migrations on startup; --db-reset (DROP tables) ensures fresh schema
#    (e.g. migration 062: clusters.region/endpoint/kubeconfig — fixes agent sync 500 if missing).
#    DB reset drops all Fortuna tables in reset_database_full.sql (incl. attack_paths, exception_policies, sbom_processing_state; updated 2026-04-14).
# 3. Rebuild: core, agent, dashboard via build-and-load-containerd.sh (nerdctl/docker/buildctl → containerd k8s.io).
#    Phase 2 image wait: fast then slow polls (PHASE2_IMAGE_* env). No second full NO_CACHE rebuild when build exits 0
#    but image listing lags (avoids ~2× rebuild time). Retry build only after a non-zero build exit code.
# 4. Deploy: addons, Flannel, StorageClass, deploy-fortuna-robust.sh; Phase 3b rollout restart (Core, Dashboard, Agent).
# 5. Post-deploy: Core auto-syncs Aikido malware feeds (122k packages, every 6h). No manual seed needed.
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
#   ./scripts/pipeline/full-clean-database-rebuild-deploy.sh --only-core     # build + apply + rollout Core only (skip default clean)
#   ./scripts/pipeline/full-clean-database-rebuild-deploy.sh --only-agent    # Agent DaemonSet only
#   ./scripts/pipeline/full-clean-database-rebuild-deploy.sh --only-dashboard # Dashboard only
#   ./scripts/pipeline/full-clean-database-rebuild-deploy.sh --only-core --with-clean   # same as above + full Phase 1 clean
#   COMPONENT_ONLY=core ./scripts/pipeline/full-clean-database-rebuild-deploy.sh      # equivalent to --only-core
#   BUILD_TOOL=docker ./scripts/pipeline/full-clean-database-rebuild-deploy.sh --full  # force Docker build backend
#   ./scripts/pipeline/full-clean-database-rebuild-deploy.sh --full --with-falco     # full pipeline + deploy Falco
#   ./scripts/pipeline/full-clean-database-rebuild-deploy.sh --full --with-ebpf      # full pipeline + enable eBPF sensor
#   ./scripts/pipeline/full-clean-database-rebuild-deploy.sh --full --with-runtime   # full pipeline + Falco + eBPF (complete runtime coverage)
#   ./scripts/pipeline/full-clean-database-rebuild-deploy.sh --full --with-e2e        # full pipeline + run E2E tests + report
#   ./scripts/pipeline/full-clean-database-rebuild-deploy.sh --full --with-e2e --e2e-suite=full-report  # specific suite
#   ./scripts/pipeline/full-clean-database-rebuild-deploy.sh --full --db-reset --with-runtime --with-e2e  # full + runtime + E2E
#   ./scripts/pipeline/full-clean-database-rebuild-deploy.sh --full --with-e2e --e2e-cleanup  # E2E + cleanup test data after report
#   Component-only mode: skips Phase 2a/2a2/2c/2d/3a and deploy-fortuna-robust; ignores --db/--db-reset (warns if set).
#   PUSH_IMAGES_AFTER_REBUILD defaults to true only when the current cluster has more than one node.
#        Set true to force registryless SSH image import, or false to skip it.
#   ./scripts/pipeline/full-clean-database-rebuild-deploy.sh --only-e2e [--e2e-suite=full-report]   # E2E only (skip clean/rebuild/deploy; cluster must be up)
#   ./scripts/pipeline/full-clean-database-rebuild-deploy.sh --full --with-e2e --e2e-no-scenario   # Full pipeline + E2E without deploying scenarios/ (faster)
#   Env: E2E_WITH_SCENARIO=true|false overrides scenario block in full-report suite
#   RUN_ASYNC=1 ./scripts/pipeline/full-clean-database-rebuild-deploy.sh  # run in background
#   Env: Phase 2 sets DOCKER_BUILDKIT=1 for Docker builds (override with DOCKER_BUILDKIT=0).
#   Env: VERSION controls the image tag. Default is current git tag/commit without dirty suffix so deploy manifests
#        and rollout image refs use the same stable tag generated by the build phase.
#   Env: SYNC_DEPLOY_IMAGE_TAG=false skips updating deploy YAML image tags after rebuild.
#   Env: CLEAN_OLD_FORTUNA_IMAGES_AFTER_DEPLOY=false keeps older local Fortuna image tags after successful deploy.
#   Env: CLEAN_BUILD_CACHE_AFTER_DEPLOY=true prunes BuildKit cache after successful deploy.
#   Env: PG_RECREATE_PVC=1 recreates PVC postgres-pvc (fixes corrupt PG data / CrashLoop); destroys DB files on that volume.
#   Env: POSTGRES_AUTO_RECOVER_PVC_ON_CORRUPTION=1 lets post-deploy verification recreate a corrupt Postgres PVC.
#   Env: CVE_CATALOG_POST_DEPLOY_CHECK=auto|required|skip controls post-reset CVE catalog recovery verification.
#        auto runs when --db-reset, PG_RECREATE_PVC=1, or Postgres PVC recovery happened in Phase 5.
#        AUTO_LOAD_CVE_CATALOG=true lets the guard run scripts/utils/load-cve-data.sh when catalog rows are missing.
#   Env: FALCO_E2E_ENABLED=false skips Falco trigger verification when --with-falco is used.
#   Env: FALCO_E2E_REQUIRED=false lets pipeline continue if Falco trigger verification fails.
#   Env: FALCO_E2E_KEEP_POD=false makes the Falco trigger pod exit after generating events.
#   Env: FALCO_E2E_INVENTORY_WAIT_SECONDS controls the optional dashboard inventory wait for the trigger pod.
#   Env: PARALLEL_FORTUNA_BUILDS=1 (see build-and-load-containerd.sh) builds Core+Agent in parallel.
#   Env: REMOTE_KUBECONFIGS="cluster02=/path/to/kubeconfig" syncs Agent-only remote clusters after local deploy.
#        Set MANAGEMENT_NODE or CORE_HTTP_ENDPOINT/CORE_GRPC_ENDPOINT. REMOTE_IMAGE_MODE=registry|local|auto.
#        REMOTE_SYNC_REQUIRED=false lets the pipeline continue when remote sync fails.
#   Env: RELAX_INFRA_WAIT=1 lets deploy-fortuna-robust.sh continue if Postgres/NATS Ready wait fails.
# ============================================================================

set -euo pipefail

# Match build-and-load-containerd.sh: minimal environments often lack /usr/bin in PATH.
export PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:${PATH:-}"

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
PUSH_DASHBOARD="${PUSH_DASHBOARD:-false}"
DEPLOY_MINIMAL=false
COMPONENT_ONLY="${COMPONENT_ONLY:-}"
WITH_CLEAN_FLAG=false
FULL_PIPELINE=false
WITH_FALCO="${WITH_FALCO:-false}"
WITH_EBPF="${WITH_EBPF:-false}"
FALCO_E2E_ENABLED="${FALCO_E2E_ENABLED:-true}"
FALCO_E2E_REQUIRED="${FALCO_E2E_REQUIRED:-true}"
FALCO_E2E_KEEP_POD="${FALCO_E2E_KEEP_POD:-true}"
FALCO_E2E_CLEANUP="${FALCO_E2E_CLEANUP:-false}"
FALCO_E2E_NAMESPACE="${FALCO_E2E_NAMESPACE:-fortuna-falco-visible}"
FALCO_E2E_TRIGGER_POD="${FALCO_E2E_TRIGGER_POD:-falco-k8s-api-visible}"
FALCO_E2E_HOLD_SECONDS="${FALCO_E2E_HOLD_SECONDS:-3600}"
FALCO_E2E_INVENTORY_WAIT_SECONDS="${FALCO_E2E_INVENTORY_WAIT_SECONDS:-180}"
WITH_E2E="${WITH_E2E:-false}"
E2E_SUITE="${E2E_SUITE:-full-report}"
E2E_CLEANUP="${E2E_CLEANUP:-false}"
ONLY_E2E=false
E2E_WITH_SCENARIO="${E2E_WITH_SCENARIO:-true}"

for arg in "$@"; do
  case "$arg" in
    --skip-clean)    SKIP_CLEAN=true ;;
    --skip-rebuild)  SKIP_REBUILD=true ;;
    --skip-deploy)   SKIP_DEPLOY=true ;;
    --push-dashboard) PUSH_DASHBOARD=true ;;
    --db)            CLEAN_DB=true ;;
    --db-reset)       DB_RESET=true ;;
    --only-db-reset)  SKIP_CLEAN=true; SKIP_REBUILD=true; SKIP_DEPLOY=true; DB_RESET=true; COMPONENT_ONLY=""; DEPLOY_MINIMAL=false ;;
    --menu|-i)        SHOW_MENU=true ;;
    --full)           FULL_PIPELINE=true; COMPONENT_ONLY=""; DEPLOY_MINIMAL=false; SKIP_CLEAN=false ;;
    --only-core)      COMPONENT_ONLY=core; DEPLOY_MINIMAL=true; SKIP_CLEAN=true ;;
    --only-agent)     COMPONENT_ONLY=agent; DEPLOY_MINIMAL=true; SKIP_CLEAN=true ;;
    --only-dashboard) COMPONENT_ONLY=dashboard; DEPLOY_MINIMAL=true; SKIP_CLEAN=true ;;
    --with-clean)     WITH_CLEAN_FLAG=true; SKIP_CLEAN=false ;;
    --with-falco)     WITH_FALCO=true ;;
    --with-ebpf)      WITH_EBPF=true ;;
    --with-runtime)   WITH_FALCO=true; WITH_EBPF=true ;;
    --with-e2e)       WITH_E2E=true ;;
    --e2e-suite=*)    E2E_SUITE="${arg#--e2e-suite=}" ;;
    --e2e-cleanup)    E2E_CLEANUP=true ;;
    --only-e2e)       ONLY_E2E=true; WITH_E2E=true; SKIP_CLEAN=true; SKIP_REBUILD=true; SKIP_DEPLOY=true ;;
    --e2e-with-scenario)  E2E_WITH_SCENARIO=true ;;
    --e2e-no-scenario)    E2E_WITH_SCENARIO=false ;;
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
  DEPLOY_MINIMAL=false
  COMPONENT_ONLY=""
  WITH_CLEAN_FLAG=false
  FULL_PIPELINE=false
  WITH_FALCO=false
  WITH_EBPF=false
  WITH_E2E=false
  E2E_SUITE="full-report"
  E2E_CLEANUP=false
  ONLY_E2E=false
  E2E_WITH_SCENARIO=true
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
  echo "  8) Core only              Build + apply + rollout Core (minimal deploy)"
  echo "  9) Agent only             Build + apply + rollout Agent DaemonSet"
  echo " 10) Dashboard only         Build dashboard + apply + rollout Dashboard"
  echo " 11) Full + Falco           Full pipeline + deploy Falco (Helm) for runtime security events"
  echo " 12) Full + eBPF            Full pipeline + enable eBPF sensor on Agent"
  echo " 13) Full + Runtime (all)   Full pipeline + Falco + eBPF (complete runtime coverage)"
  echo " 14) Full + E2E            Full pipeline + run E2E tests + capability report"
  echo " 15) Full + Runtime + E2E  Full pipeline + Falco + eBPF + E2E tests + report"
  echo " 16) E2E only                 Skip clean/rebuild/deploy; E2E full-report + scenarios (cluster ready)"
  echo " 17) E2E only (no scenarios)  Same as 16 without deploying scenarios/ (faster)"
  echo "  0) Cancel"
  echo ""
  printf "  Select [1-17, 0]: "
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
    8)  COMPONENT_ONLY=core; DEPLOY_MINIMAL=true; SKIP_CLEAN=true ;;
    9)  COMPONENT_ONLY=agent; DEPLOY_MINIMAL=true; SKIP_CLEAN=true ;;
    10) COMPONENT_ONLY=dashboard; DEPLOY_MINIMAL=true; SKIP_CLEAN=true ;;
    11) WITH_FALCO=true ;;
    12) WITH_EBPF=true ;;
    13) WITH_FALCO=true; WITH_EBPF=true ;;
    14) WITH_E2E=true ;;
    15) WITH_FALCO=true; WITH_EBPF=true; WITH_E2E=true ;;
    16) ONLY_E2E=true; WITH_E2E=true; SKIP_CLEAN=true; SKIP_REBUILD=true; SKIP_DEPLOY=true ;;
    17) ONLY_E2E=true; WITH_E2E=true; SKIP_CLEAN=true; SKIP_REBUILD=true; SKIP_DEPLOY=true; E2E_WITH_SCENARIO=false ;;
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

# ---- Component-only: COMPONENT_ONLY from env or menu 8–10 (CLI --only-* already set) ----
if [ -n "${COMPONENT_ONLY:-}" ]; then
  case "$COMPONENT_ONLY" in
    core|agent|dashboard)
      DEPLOY_MINIMAL=true
      if [ "$WITH_CLEAN_FLAG" = false ]; then
        SKIP_CLEAN=true
      fi
      ;;
    *)
      echo "[ERR] Invalid COMPONENT_ONLY=$COMPONENT_ONLY (use core, agent, or dashboard)" >&2
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

VERSION="${VERSION:-$(cd "$PROJECT_ROOT" && git describe --tags --always 2>/dev/null || git rev-parse --short HEAD 2>/dev/null || echo latest)}"
COMMIT="${COMMIT:-$(cd "$PROJECT_ROOT" && git rev-parse --short HEAD 2>/dev/null || echo unknown)}"
export VERSION COMMIT
SYNC_DEPLOY_IMAGE_TAG="${SYNC_DEPLOY_IMAGE_TAG:-true}"
CLEAN_OLD_FORTUNA_IMAGES_AFTER_DEPLOY="${CLEAN_OLD_FORTUNA_IMAGES_AFTER_DEPLOY:-true}"
CLEAN_BUILD_CACHE_AFTER_DEPLOY="${CLEAN_BUILD_CACHE_AFTER_DEPLOY:-false}"

ensure_local_database_env() {
  if [ -n "${FORTUNA_DATABASE_URL:-}" ]; then
    return 0
  fi

  local pg_password="${FORTUNA_POSTGRES_PASSWORD:-}"
  if [ -z "$pg_password" ] && kubectl get secret postgres-credentials -n "$NAMESPACE" >/dev/null 2>&1; then
    pg_password="$(kubectl get secret postgres-credentials -n "$NAMESPACE" -o jsonpath='{.data.POSTGRES_PASSWORD}' 2>/dev/null | base64 -d 2>/dev/null || true)"
  fi

  if [ -z "$pg_password" ]; then
    if command -v openssl >/dev/null 2>&1; then
      pg_password="$(openssl rand -base64 24 | tr -d '=+/ ' | cut -c1-24)"
    else
      pg_password="Fortuna_Postgres_Local_ChangeMe_123"
    fi
    export FORTUNA_POSTGRES_PASSWORD="$pg_password"
    log_warn "FORTUNA_DATABASE_URL not set; generated local bundled PostgreSQL password for this deploy."
  else
    export FORTUNA_POSTGRES_PASSWORD="$pg_password"
    log_warn "FORTUNA_DATABASE_URL not set; using bundled PostgreSQL password from env/secret."
  fi

  export FORTUNA_DATABASE_URL="postgres://postgres:${pg_password}@postgres.${NAMESPACE}.svc.cluster.local:5432/fortuna?sslmode=disable"
}

ensure_agent_schedules_on_nodes() {
  if ! kubectl get daemonset/fortuna-agent -n "$NAMESPACE" >/dev/null 2>&1; then
    return 0
  fi

  local disabled_selector
  disabled_selector=$(kubectl get daemonset/fortuna-agent -n "$NAMESPACE" -o jsonpath='{.spec.template.spec.nodeSelector.fortuna\.dev/disabled}' 2>/dev/null || true)
  if [ -n "$disabled_selector" ]; then
    log_warn "Removing stale fortuna-agent nodeSelector fortuna.dev/disabled=$disabled_selector so DaemonSet can schedule."
    kubectl patch daemonset/fortuna-agent -n "$NAMESPACE" --type=json -p='[{"op":"remove","path":"/spec/template/spec/nodeSelector"}]' >/dev/null 2>&1 || \
      log_warn "Could not remove stale fortuna-agent nodeSelector; inspect: kubectl describe ds fortuna-agent -n $NAMESPACE"
  fi
}

ensure_falco_schedules_on_nodes() {
  if ! kubectl get daemonset/falco -n "$NAMESPACE" >/dev/null 2>&1; then
    return 0
  fi

  local disabled_selector
  disabled_selector=$(kubectl get daemonset/falco -n "$NAMESPACE" -o jsonpath='{.spec.template.spec.nodeSelector.fortuna\.dev/disabled}' 2>/dev/null || true)
  if [ -n "$disabled_selector" ]; then
    log_warn "Removing stale falco nodeSelector fortuna.dev/disabled=$disabled_selector so DaemonSet can schedule."
    kubectl patch daemonset/falco -n "$NAMESPACE" --type=json -p='[{"op":"remove","path":"/spec/template/spec/nodeSelector"}]' >/dev/null 2>&1 || \
      log_warn "Could not remove stale falco nodeSelector; inspect: kubectl describe ds falco -n $NAMESPACE"
  fi
}

ensure_core_agent_secrets() {
  kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f - 2>/dev/null || true

  ensure_local_database_env
  log_info "Ensuring fortuna-secrets/postgres-credentials..."
  "$SCRIPTS/utils/ensure-fortuna-secrets.sh" "$NAMESPACE"

  log_info "Ensuring mTLS secrets for Core/Agent..."
  if [ ! -x "$SCRIPTS/utils/create_mtls_secret.sh" ]; then
    log_error "Missing executable script: $SCRIPTS/utils/create_mtls_secret.sh"
    exit 1
  fi
  NAMESPACE="$NAMESPACE" "$SCRIPTS/utils/create_mtls_secret.sh"

  for secret in fortuna-core-tls fortuna-agent-tls fortuna-ca-cert fortuna-webhook-tls; do
    if ! kubectl get secret "$secret" -n "$NAMESPACE" >/dev/null 2>&1; then
      log_error "Required mTLS secret not found after creation: $secret"
      exit 1
    fi
  done
  log_success "Core/Agent secrets ready"
}

# --- Postgres PVC recovery (unclean shutdown / corrupt checkpoint → CrashLoopBackOff; SQL reset cannot help) ---
_fortuna_postgres_manifest_yaml() {
  local pwa="$PROJECT_ROOT/deploy/infrastructure/postgresql-with-age.yaml"
  local pw="$PROJECT_ROOT/deploy/infrastructure/postgresql.yaml"
  if [ -f "$pwa" ]; then
    echo "$pwa"
  elif [ -f "$pw" ]; then
    echo "$pw"
  else
    echo ""
  fi
}

_fortuna_pg_logs_suggest_corrupt_data() {
  local ns="$1" pod="$2" cname="${3:-postgres}" logtxt
  logtxt=$(kubectl logs -n "$ns" "$pod" -c "$cname" --tail=200 2>/dev/null || true)
  if echo "$logtxt" | grep -qE 'could not locate a valid checkpoint|invalid primary checkpoint record'; then
    return 0
  fi
  logtxt=$(kubectl logs -n "$ns" "$pod" -c "$cname" --previous --tail=200 2>/dev/null || true)
  if echo "$logtxt" | grep -qE 'could not locate a valid checkpoint|invalid primary checkpoint record'; then
    return 0
  fi
  return 1
}

_fortuna_truthy_env() {
  case "${1:-}" in
    1|true|TRUE|yes|YES|on|ON) return 0 ;;
    *) return 1 ;;
  esac
}

_fortuna_pg_recreate_pvc() {
  local ns="$1" yaml i cnt
  yaml="$(_fortuna_postgres_manifest_yaml)"
  if [ -z "$yaml" ]; then
    log_error "No postgresql manifest under deploy/infrastructure/ (postgresql-with-age.yaml or postgresql.yaml)."
    return 1
  fi
  log_warn "PG_RECREATE_PVC: scale Deployment/postgres to 0, delete PVC postgres-pvc, apply $(basename "$yaml"), scale to 1 (all data on that PVC is destroyed)."
  kubectl scale deployment/postgres -n "$ns" --replicas=0 2>/dev/null || true
  for i in $(seq 1 90); do
    cnt=$(kubectl get pods -n "$ns" -l app=postgres --no-headers 2>/dev/null | wc -l | tr -d ' ' || echo 1)
    [ "${cnt:-1}" -eq 0 ] && break
    sleep 2
  done
  kubectl delete pvc postgres-pvc -n "$ns" --wait=true 2>/dev/null || true
  kubectl apply -f "$yaml"
  kubectl scale deployment/postgres -n "$ns" --replicas=1 2>/dev/null || true
  FORTUNA_PG_RECOVERED=1
  if _fortuna_pg_wait_ready "$ns" "${PG_RECREATE_WAIT_SECONDS:-300}" false >/dev/null; then
    log_success "Postgres PVC recreated and database is accepting connections."
    return 0
  fi
  log_error "Postgres PVC was recreated, but database did not become usable in time."
  return 1
}

_fortuna_pg_auto_recover_enabled() {
  _fortuna_truthy_env "${POSTGRES_AUTO_RECOVER_PVC_ON_CORRUPTION:-}" || _fortuna_truthy_env "${PG_RECREATE_PVC:-}"
}

_fortuna_pg_wait_ready() {
  local ns="$1" max_seconds="${2:-600}" allow_recover="${3:-false}"
  local attempts i pod ready cname wait_reason phase query_ok
  attempts=$((max_seconds / 5))
  [ "$attempts" -gt 0 ] || attempts=1

  for i in $(seq 1 "$attempts"); do
    pod=$(kubectl get pods -n "$ns" -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
    if [ -z "$pod" ]; then
      sleep 5
      continue
    fi

    cname=$(kubectl get pod -n "$ns" "$pod" -o jsonpath='{.spec.containers[0].name}' 2>/dev/null || true)
    [ -n "$cname" ] || cname="postgres"
    ready=$(kubectl get pod -n "$ns" "$pod" -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || echo "False")
    if [ "$ready" = "True" ]; then
      query_ok=$(kubectl exec -n "$ns" "$pod" -c "$cname" -- psql -U postgres -d fortuna -tAc "SELECT 1" 2>/dev/null | tr -d '[:space:]' || true)
      if [ "$query_ok" = "1" ]; then
        return 0
      fi
    fi

    wait_reason=$(kubectl get pod -n "$ns" "$pod" -o jsonpath='{.status.containerStatuses[0].state.waiting.reason}' 2>/dev/null || true)
    if [ "$wait_reason" = "CrashLoopBackOff" ] || [ "$wait_reason" = "RunContainerError" ]; then
      if _fortuna_pg_logs_suggest_corrupt_data "$ns" "$pod" "$cname"; then
        if [ "$allow_recover" = true ] && _fortuna_pg_auto_recover_enabled; then
          log_warn "Corrupt PostgreSQL PVC detected during verification; recreating because recovery env is enabled."
          _fortuna_pg_recreate_pvc "$ns" || return 1
          continue
        fi
        log_error "PostgreSQL PVC appears corrupt (checkpoint/WAL). SQL reset cannot fix this while Postgres is crash-looping."
        log_error "Recovery option: POSTGRES_AUTO_RECOVER_PVC_ON_CORRUPTION=1 PG_RECREATE_PVC=1 $0 --full --db-reset"
        kubectl logs -n "$ns" "$pod" -c "$cname" --tail=40 2>/dev/null || true
        return 1
      fi
    fi

    if [ $((i % 6)) -eq 0 ]; then
      phase=$(kubectl get pod -n "$ns" "$pod" -o jsonpath='{.status.phase}' 2>/dev/null || echo "")
      log_info "Postgres pod $pod Ready=$ready phase=$phase wait_reason=${wait_reason:-none} (~$((i * 5))s)..."
    fi
    sleep 5
  done

  log_error "Postgres did not become Ready and queryable within ${max_seconds}s."
  return 1
}

# Same visibility as push-images-to-workers.sh (kubelet uses namespace k8s.io).
# nerdctl: use "images" only — "images list" treats "list" as a repository filter (empty/wrong).
_fortuna_ctr_list_k8s_io() {
  local ns="${CONTAINERD_NS:-k8s.io}" out="" c n
  if command -v ctr &>/dev/null; then
    c=ctr
  elif [ -x /usr/bin/ctr ]; then
    c=/usr/bin/ctr
  elif [ -x /usr/local/bin/ctr ]; then
    c=/usr/local/bin/ctr
  elif [ -x /opt/containerd/bin/ctr ]; then
    c=/opt/containerd/bin/ctr
  else
    c=""
  fi
  if command -v nerdctl &>/dev/null; then
    n=nerdctl
  elif [ -x /usr/bin/nerdctl ]; then
    n=/usr/bin/nerdctl
  elif [ -x /usr/local/bin/nerdctl ]; then
    n=/usr/local/bin/nerdctl
  else
    n=""
  fi
  if [ -n "$c" ]; then
    out="$("$c" -n "$ns" images list 2>/dev/null || true)"
  fi
  if [ -n "$n" ]; then
    out="${out}
$("$n" --namespace "$ns" images 2>/dev/null || true)"
  fi
  printf '%s\n' "$out"
}

# After Phase 2: required images must exist or Phase 2b / deploy will fail with ErrImageNeverPull.
# Fast polls first (import metadata can appear in <1s), then slower. Env overrides:
#   PHASE2_IMAGE_FAST_ATTEMPTS (default 12), PHASE2_IMAGE_FAST_SLEEP (default 1)
#   PHASE2_IMAGE_SLOW_ATTEMPTS (default 45), PHASE2_IMAGE_SLOW_SLEEP (default 1)
#   PHASE2_IMAGE_LAST_ATTEMPTS (default 25), PHASE2_IMAGE_LAST_SLEEP (default 2)
_phase2_images_ok() {
  local list ok
  list=$(_fortuna_ctr_list_k8s_io)
  ok=false
  _has_ref() {
    local name="$1" tag="$2"
    echo "$list" | grep -qE "${name}:${tag}([[:space:]]|$)"
  }
  case "${COMPONENT_ONLY:-}" in
    dashboard)
      if _has_ref "fortuna-dashboard" "${VERSION:-latest}" && _has_ref "fortuna-dashboard" "latest"; then
        ok=true
      fi
      ;;
    core)
      if _has_ref "fortuna-core" "${VERSION:-latest}" && _has_ref "fortuna-core" "latest"; then
        ok=true
      fi
      ;;
    agent)
      if _has_ref "fortuna-agent" "${VERSION:-latest}" && _has_ref "fortuna-agent" "latest"; then
        ok=true
      fi
      ;;
    *)
      if _has_ref "fortuna-core" "${VERSION:-latest}" && _has_ref "fortuna-core" "latest" \
        && _has_ref "fortuna-agent" "${VERSION:-latest}" && _has_ref "fortuna-agent" "latest"; then
        if [ "${SKIP_DASHBOARD:-false}" = "true" ] || { _has_ref "fortuna-dashboard" "${VERSION:-latest}" && _has_ref "fortuna-dashboard" "latest"; }; then
          ok=true
        fi
      fi
      ;;
  esac
  [ "$ok" = true ]
}

_phase2_required_images_present() {
  local attempt fa fs sa ss la ls
  fa="${PHASE2_IMAGE_FAST_ATTEMPTS:-12}"
  fs="${PHASE2_IMAGE_FAST_SLEEP:-1}"
  sa="${PHASE2_IMAGE_SLOW_ATTEMPTS:-45}"
  ss="${PHASE2_IMAGE_SLOW_SLEEP:-1}"
  la="${PHASE2_IMAGE_LAST_ATTEMPTS:-25}"
  ls="${PHASE2_IMAGE_LAST_SLEEP:-2}"
  for attempt in $(seq 1 "$fa"); do
    if _phase2_images_ok; then return 0; fi
    sleep "$fs"
  done
  for attempt in $(seq 1 "$sa"); do
    if _phase2_images_ok; then return 0; fi
    sleep "$ss"
  done
  for attempt in $(seq 1 "$la"); do
    if _phase2_images_ok; then return 0; fi
    sleep "$ls"
  done
  return 1
}

_sync_deploy_image_tags() {
  if [ "$SYNC_DEPLOY_IMAGE_TAG" != "true" ]; then
    log_info "Deploy image tag sync skipped (SYNC_DEPLOY_IMAGE_TAG=false)"
    return 0
  fi
  if [ "$SKIP_REBUILD" = true ]; then
    log_info "Deploy image tag sync skipped because rebuild is skipped"
    return 0
  fi

  log_info "Syncing deploy image tags to VERSION=$VERSION..."
  case "${COMPONENT_ONLY:-}" in
    core)
      [ -f "$PROJECT_ROOT/deploy/fortuna-core-deployment.yaml" ] && sed -i "s@image: fortuna-core:[^[:space:]]*@image: fortuna-core:${VERSION}@g" "$PROJECT_ROOT/deploy/fortuna-core-deployment.yaml" && log_success "  fortuna-core-deployment.yaml -> fortuna-core:$VERSION"
      ;;
    agent)
      [ -f "$PROJECT_ROOT/deploy/fortuna-agent-daemonset.yaml" ] && sed -i "s@image: fortuna-agent:[^[:space:]]*@image: fortuna-agent:${VERSION}@g" "$PROJECT_ROOT/deploy/fortuna-agent-daemonset.yaml" && log_success "  fortuna-agent-daemonset.yaml -> fortuna-agent:$VERSION"
      ;;
    dashboard)
      [ -f "$PROJECT_ROOT/deploy/dashboard-deployment.yaml" ] && sed -i "s@image: fortuna-dashboard:[^[:space:]]*@image: fortuna-dashboard:${VERSION}@g" "$PROJECT_ROOT/deploy/dashboard-deployment.yaml" && log_success "  dashboard-deployment.yaml -> fortuna-dashboard:$VERSION"
      ;;
    *)
      [ -f "$PROJECT_ROOT/deploy/fortuna-core-deployment.yaml" ] && sed -i "s@image: fortuna-core:[^[:space:]]*@image: fortuna-core:${VERSION}@g" "$PROJECT_ROOT/deploy/fortuna-core-deployment.yaml" && log_success "  fortuna-core-deployment.yaml -> fortuna-core:$VERSION"
      [ -f "$PROJECT_ROOT/deploy/fortuna-agent-daemonset.yaml" ] && sed -i "s@image: fortuna-agent:[^[:space:]]*@image: fortuna-agent:${VERSION}@g" "$PROJECT_ROOT/deploy/fortuna-agent-daemonset.yaml" && log_success "  fortuna-agent-daemonset.yaml -> fortuna-agent:$VERSION"
      [ "${SKIP_DASHBOARD:-false}" = "true" ] || { [ -f "$PROJECT_ROOT/deploy/dashboard-deployment.yaml" ] && sed -i "s@image: fortuna-dashboard:[^[:space:]]*@image: fortuna-dashboard:${VERSION}@g" "$PROJECT_ROOT/deploy/dashboard-deployment.yaml" && log_success "  dashboard-deployment.yaml -> fortuna-dashboard:$VERSION"; }
      ;;
  esac
}

_set_workload_images_for_tag() {
  if [ "$SKIP_REBUILD" = true ]; then
    return 0
  fi
  case "${COMPONENT_ONLY:-}" in
    core)
      kubectl set image deployment/fortuna-core -n "$NAMESPACE" core="fortuna-core:${VERSION}" >/dev/null 2>&1 || true
      ;;
    agent)
      kubectl set image daemonset/fortuna-agent -n "$NAMESPACE" agent="fortuna-agent:${VERSION}" >/dev/null 2>&1 || true
      ;;
    dashboard)
      kubectl set image deployment/fortuna-dashboard -n "$NAMESPACE" dashboard="fortuna-dashboard:${VERSION}" >/dev/null 2>&1 || true
      ;;
    *)
      kubectl set image deployment/fortuna-core -n "$NAMESPACE" core="fortuna-core:${VERSION}" >/dev/null 2>&1 || true
      kubectl set image deployment/fortuna-dashboard -n "$NAMESPACE" dashboard="fortuna-dashboard:${VERSION}" >/dev/null 2>&1 || true
      kubectl set image daemonset/fortuna-agent -n "$NAMESPACE" agent="fortuna-agent:${VERSION}" >/dev/null 2>&1 || true
      ;;
  esac
}

_cleanup_old_fortuna_images() {
  if [ "$CLEAN_OLD_FORTUNA_IMAGES_AFTER_DEPLOY" != "true" ]; then
    log_info "Old Fortuna image cleanup skipped (CLEAN_OLD_FORTUNA_IMAGES_AFTER_DEPLOY=false)"
    return 0
  fi
  if ! command -v nerdctl >/dev/null 2>&1; then
    log_warn "nerdctl not found; skip old Fortuna image cleanup"
    return 0
  fi

  local used_images refs image short_image
  used_images="$(kubectl get deployment,daemonset -n "$NAMESPACE" -o jsonpath='{range .items[*]}{range .spec.template.spec.containers[*]}{.image}{"\n"}{end}{end}' 2>/dev/null || true)"
  refs="$(nerdctl --namespace "$CONTAINERD_NS" images --format '{{.Repository}}:{{.Tag}}' 2>/dev/null | grep -E '(^|/)fortuna-(core|agent|dashboard):' || true)"
  while IFS= read -r image; do
    [ -n "$image" ] || continue
    short_image="${image#docker.io/library/}"
    case "$image" in
      *":${VERSION}"|*:latest) continue ;;
    esac
    if printf '%s\n' "$used_images" | grep -qxF "$image" || printf '%s\n' "$used_images" | grep -qxF "$short_image"; then
      continue
    fi
    nerdctl --namespace "$CONTAINERD_NS" rmi --force "$image" >/dev/null 2>&1 || true
  done <<< "$refs"

  if [ "$CLEAN_BUILD_CACHE_AFTER_DEPLOY" = "true" ]; then
    nerdctl --namespace "$CONTAINERD_NS" builder prune >/dev/null 2>&1 || true
  fi
  log_success "Old Fortuna image cleanup complete (kept VERSION=$VERSION, latest, and images currently referenced by workloads)"
}

echo "=========================================="
echo "Full Clean (images + optional DB) → Rebuild → Deploy"
echo "=========================================="
echo "  Skip clean: $SKIP_CLEAN | Skip rebuild: $SKIP_REBUILD | Skip deploy: $SKIP_DEPLOY"
echo "  Clean DB (delete data): $CLEAN_DB | DB full reset (drop tables): $DB_RESET"
echo "  Image tag VERSION: $VERSION | Commit: $COMMIT | Sync deploy tag: $SYNC_DEPLOY_IMAGE_TAG"
echo "  Deploy Falco: $WITH_FALCO | Enable eBPF: $WITH_EBPF | Falco E2E: $FALCO_E2E_ENABLED | Run E2E: $WITH_E2E (suite=$E2E_SUITE, cleanup=$E2E_CLEANUP, only_e2e=$ONLY_E2E, scenarios=$E2E_WITH_SCENARIO)"
if [ "$DEPLOY_MINIMAL" = true ]; then
  echo "  Component-only: $COMPONENT_ONLY (minimal deploy; infra/DB phases skipped)"
fi
echo ""

if [ "$DEPLOY_MINIMAL" = true ] && { [ "$CLEAN_DB" = true ] || [ "$DB_RESET" = true ]; }; then
  log_warn "Ignoring --db / --db-reset in component-only mode ($COMPONENT_ONLY). Use full pipeline for DB changes."
  CLEAN_DB=false
  DB_RESET=false
fi

if [ "$ONLY_E2E" != true ]; then

# ---- Phase 1: Clean ----
if [ "$SKIP_CLEAN" = false ]; then
  log_info "Phase 1: Clean (port-forwards, E2E ns, fortuna images, prune)..."
  pkill -f "kubectl.*port-forward" 2>/dev/null || true
  for ns in fortuna-e2e fortuna-e2e-2025; do
    kubectl get namespace "$ns" 2>/dev/null && kubectl delete namespace "$ns" --timeout=60s 2>/dev/null || true
  done
  if [ "$SKIP_REBUILD" = true ] && ! _fortuna_truthy_env "${FORCE_IMAGE_CLEAN:-}"; then
    log_warn "Skipping local Fortuna image cleanup because --skip-rebuild is set. Set FORCE_IMAGE_CLEAN=1 to override."
    log_success "Clean complete"
  else
  log_info "Removing ALL fortuna images (containerd namespace=$CONTAINERD_NS)..."
  # Clean via nerdctl if available
  if command -v nerdctl &>/dev/null; then
    for img in fortuna-core:latest fortuna-agent:latest fortuna-dashboard:latest; do
      nerdctl --namespace "$CONTAINERD_NS" rmi --force "$img" 2>/dev/null || true
    done
    fortuna_ids=""
    fortuna_ids=$(nerdctl --namespace "$CONTAINERD_NS" images 2>/dev/null | grep -E 'fortuna-(core|agent|dashboard)' | awk '{print $3}' | sort -u) || true
    for id in $fortuna_ids; do
      [ -n "$id" ] && [ "$id" != "ID" ] && nerdctl --namespace "$CONTAINERD_NS" rmi --force "$id" 2>/dev/null || true
    done
    log_info "Pruning containerd system and build cache..."
    nerdctl --namespace "$CONTAINERD_NS" system prune -f 2>/dev/null || true
    nerdctl --namespace "$CONTAINERD_NS" builder prune 2>/dev/null || true
  fi
  # Clean via docker if available
  if command -v docker &>/dev/null && docker info &>/dev/null 2>&1; then
    log_info "Removing fortuna images from Docker..."
    for img in fortuna-core fortuna-agent fortuna-dashboard; do
      docker images --format '{{.Repository}}:{{.Tag}}' 2>/dev/null | grep "^${img}:" | xargs -r docker rmi --force 2>/dev/null || true
    done
    docker image prune -f 2>/dev/null || true
  fi
  # Clean via ctr if available (catches images not managed by nerdctl/docker)
  # pipefail: grep exits 1 when there are no matches — would kill the whole script after Docker prune.
  if command -v ctr &>/dev/null; then
    { ctr -n "$CONTAINERD_NS" images list 2>/dev/null || true; } | awk '/fortuna-(core|agent|dashboard)/ {print $1}' | while read -r ref; do
      [ -n "$ref" ] && ctr -n "$CONTAINERD_NS" images rm "$ref" 2>/dev/null || true
    done
  fi
  log_success "Clean complete"
  fi
else
  log_info "Phase 1: Clean (skipped)"
fi
echo ""

# ---- Phase 1b: Database clean (only when deploy is skipped — Postgres must already exist, e.g. --only-db-reset) ----
if [ "$SKIP_DEPLOY" = true ] && { [ "$CLEAN_DB" = true ] || [ "$DB_RESET" = true ]; }; then
  log_info "Phase 1b: Database clean (deploy skipped — Postgres must be Ready)..."
  if _fortuna_truthy_env "${PG_RECREATE_PVC:-}"; then
    log_info "PG_RECREATE_PVC set: recreating postgres PVC before DB clean..."
    _fortuna_pg_recreate_pvc "$NAMESPACE" || exit 1
  fi
  POD=""
  _pg_wait_i=0
  for _ in $(seq 1 120); do
    _pg_wait_i=$((_pg_wait_i + 1))
    POD=$(kubectl get pods -n "$NAMESPACE" -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
    [ -z "$POD" ] && sleep 5 && continue
    READY=$(kubectl get pod -n "$NAMESPACE" "$POD" -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || echo "False")
    if [ "$READY" = "True" ]; then
      break
    fi
    PG_C_TMP=$(kubectl get pod -n "$NAMESPACE" "$POD" -o jsonpath='{.spec.containers[0].name}' 2>/dev/null || true)
    [ -n "$PG_C_TMP" ] || PG_C_TMP="postgres"
    WAIT_REASON=$(kubectl get pod -n "$NAMESPACE" "$POD" -o jsonpath='{.status.containerStatuses[0].state.waiting.reason}' 2>/dev/null || true)
    if [ "$WAIT_REASON" = "CrashLoopBackOff" ] || [ "$WAIT_REASON" = "RunContainerError" ]; then
      if _fortuna_pg_logs_suggest_corrupt_data "$NAMESPACE" "$POD" "$PG_C_TMP"; then
        if _fortuna_truthy_env "${PG_RECREATE_PVC:-}"; then
          log_warn "Corrupt PostgreSQL volume detected; PG_RECREATE_PVC set — recreating PVC..."
          _fortuna_pg_recreate_pvc "$NAMESPACE" || exit 1
          POD=""
          continue
        fi
        log_error "PostgreSQL data on PVC looks corrupt (checkpoint/WAL). SQL reset cannot run while Postgres is crash-looping."
        log_error "Fix (wipes DB files on PVC): PG_RECREATE_PVC=1 $0 --only-db-reset"
        log_error "Or: kubectl scale deployment/postgres -n $NAMESPACE --replicas=0; kubectl delete pvc postgres-pvc -n $NAMESPACE; kubectl apply -f deploy/infrastructure/postgresql-with-age.yaml; kubectl scale deployment/postgres -n $NAMESPACE --replicas=1"
        kubectl logs -n "$NAMESPACE" "$POD" -c "$PG_C_TMP" --tail=35 2>/dev/null || true
        exit 1
      fi
    fi
    if [ $((_pg_wait_i % 6)) -ne 0 ]; then
      sleep 5
      continue
    fi
    PHASE=$(kubectl get pod -n "$NAMESPACE" "$POD" -o jsonpath='{.status.phase}' 2>/dev/null || echo "")
    log_info "Postgres pod $POD Ready=$READY phase=$PHASE wait_reason=${WAIT_REASON:-none} (~$((_pg_wait_i * 5))s)..."
    if [ "$PHASE" = "Pending" ]; then
      log_warn "PVC local-path pins data to one node; pod must schedule there. If events say Insufficient cpu, free CPU on that node or lower postgres requests. Recent events:"
      kubectl get events -n "$NAMESPACE" --field-selector "involvedObject.name=$POD" 2>/dev/null | tail -8 || true
    fi
    sleep 5
  done
  if [ -z "$POD" ]; then
    log_warn "Postgres pod not found in namespace $NAMESPACE; skip DB clean."
  else
    READY=$(kubectl get pod -n "$NAMESPACE" "$POD" -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || echo "False")
    PG_C_TMP=$(kubectl get pod -n "$NAMESPACE" "$POD" -o jsonpath='{.spec.containers[0].name}' 2>/dev/null || true)
    [ -n "$PG_C_TMP" ] || PG_C_TMP="postgres"
    if [ "$READY" != "True" ]; then
      if _fortuna_pg_logs_suggest_corrupt_data "$NAMESPACE" "$POD" "$PG_C_TMP"; then
        log_error "PostgreSQL data on PVC appears corrupt. Fix: PG_RECREATE_PVC=1 $0 --only-db-reset"
        kubectl logs -n "$NAMESPACE" "$POD" -c "$PG_C_TMP" --tail=40 2>/dev/null || true
      else
        log_error "Postgres pod $POD never became Ready. Check: kubectl describe pod -n $NAMESPACE $POD"
        kubectl describe pod -n "$NAMESPACE" "$POD" | tail -45 || true
      fi
      exit 1
    fi
    PG_CONTAINER="$PG_C_TMP"
    if [ "$DB_RESET" = true ]; then
      SQL_FILE="$PROJECT_ROOT/deploy/sql/reset_database_full.sql"
      if [ -f "$SQL_FILE" ]; then
        if ! kubectl cp "$SQL_FILE" "$POD:/tmp/reset_db.sql" -n "$NAMESPACE" -c "$PG_CONTAINER"; then
          log_error "kubectl cp reset_database_full.sql failed"
          exit 1
        fi
        if ! kubectl exec -n "$NAMESPACE" "$POD" -c "$PG_CONTAINER" -- psql -U postgres -d fortuna -f /tmp/reset_db.sql; then
          log_error "psql reset_database_full.sql failed"
          exit 1
        fi
        log_success "DB full reset (DROP tables) done. Core will re-run migrations on next start."
      else
        log_error "File not found: $SQL_FILE"
        exit 1
      fi
    else
      SQL_FILE="$PROJECT_ROOT/deploy/sql/clear_all_cluster_data.sql"
      if [ -f "$SQL_FILE" ]; then
        if ! kubectl cp "$SQL_FILE" "$POD:/tmp/clear_db.sql" -n "$NAMESPACE" -c "$PG_CONTAINER"; then
          log_error "kubectl cp clear_all_cluster_data.sql failed"
          exit 1
        fi
        if ! kubectl exec -n "$NAMESPACE" "$POD" -c "$PG_CONTAINER" -- psql -U postgres -d fortuna -f /tmp/clear_db.sql; then
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
  cd "$PROJECT_ROOT"
  if [ -x "$SCRIPTS/utils/ensure-buildkit-running.sh" ]; then
    bash "$SCRIPTS/utils/ensure-buildkit-running.sh" || true
  fi
  # Docker builds: prefer BuildKit (faster / better caching when not NO_CACHE). Inherited by build-and-load-containerd.sh.
  export DOCKER_BUILDKIT="${DOCKER_BUILDKIT:-1}"
  export NO_CACHE="${NO_CACHE:-false}"
  if [ "$SKIP_CLEAN" = false ]; then
    NO_CACHE=true
    log_info "NO_CACHE=true (clean was run; ensure fresh build)"
  fi
  if [ "$COMPONENT_ONLY" = "dashboard" ]; then
    log_info "Phase 2: Rebuild dashboard only..."
    if [ -x "$SCRIPTS/build/build-and-load-containerd.sh" ]; then
      CONTAINERD_NAMESPACE="$CONTAINERD_NS" BUILD_DASHBOARD_ONLY=true "$SCRIPTS/build/build-and-load-containerd.sh" || {
        log_error "Dashboard build failed"
        exit 1
      }
      if ! _phase2_required_images_present; then
        log_error "fortuna-dashboard not found in containerd ($CONTAINERD_NS) after dashboard build."
        log_info "Check: nerdctl --namespace $CONTAINERD_NS images | grep fortuna-dashboard"
        log_info "Check: ctr -n $CONTAINERD_NS images list | grep fortuna-dashboard"
        if command -v docker &>/dev/null && docker image inspect fortuna-dashboard:latest &>/dev/null; then
          log_warn "Image exists in Docker but not in containerd — put ctr in PATH, use nerdctl alongside Docker (build script falls back to nerdctl load), or BUILD_TOOL=nerdctl with buildkitd."
        fi
        exit 1
      fi
    else
      log_error "build-and-load-containerd.sh not found or not executable"
      exit 1
    fi
  else
    if [ "$COMPONENT_ONLY" = "core" ]; then
      log_info "Phase 2: Rebuild Core only..."
      export BUILD_CORE_ONLY=true BUILD_AGENT_ONLY=false SKIP_DASHBOARD=true
    elif [ "$COMPONENT_ONLY" = "agent" ]; then
      log_info "Phase 2: Rebuild Agent only..."
      export BUILD_CORE_ONLY=false BUILD_AGENT_ONLY=true SKIP_DASHBOARD=true
    else
      log_info "Phase 2: Rebuild (core, agent, dashboard)..."
      unset BUILD_CORE_ONLY BUILD_AGENT_ONLY 2>/dev/null || true
    fi
    if [ -x "$SCRIPTS/build/build-and-load-containerd.sh" ]; then
      BUILD_RC=0
      CONTAINERD_NAMESPACE="$CONTAINERD_NS" SKIP_DASHBOARD="${SKIP_DASHBOARD:-false}" NO_CACHE="$NO_CACHE" \
        BUILD_CORE_ONLY="${BUILD_CORE_ONLY:-false}" BUILD_AGENT_ONLY="${BUILD_AGENT_ONLY:-false}" \
        "$SCRIPTS/build/build-and-load-containerd.sh" || BUILD_RC=$?
      if [ "$BUILD_RC" -ne 0 ]; then
        log_warn "Build script exited with code $BUILD_RC (one or more image builds may have failed)."
      fi
      if _phase2_images_ok; then
        log_success "Required images visible in containerd after build"
      elif [ "$BUILD_RC" -eq 0 ]; then
        log_warn "Build reported success but image refs not listed yet; waiting for containerd (no second full build)..."
        if ! _phase2_required_images_present; then
          log_error "Required Fortuna image(s) still missing in containerd ($CONTAINERD_NS) after a successful build."
          log_info "Check: nerdctl --namespace $CONTAINERD_NS images | grep fortuna-  ;  ctr -n $CONTAINERD_NS images list | grep fortuna-"
          log_info "Manual import: CONTAINERD_NAMESPACE=$CONTAINERD_NS $SCRIPTS/build/build-and-load-containerd.sh"
          exit 1
        fi
      else
        log_warn "Required images missing after a failed build; running one automatic NO_CACHE retry build..."
        BUILD_RC2=0
        if [ -z "${COMPONENT_ONLY:-}" ]; then
          log_info "Phase 2 (retry): full rebuild (core, agent, dashboard), NO_CACHE=true..."
          unset BUILD_CORE_ONLY BUILD_AGENT_ONLY 2>/dev/null || true
          CONTAINERD_NAMESPACE="$CONTAINERD_NS" SKIP_DASHBOARD=false NO_CACHE=true \
            BUILD_CORE_ONLY=false BUILD_AGENT_ONLY=false \
            "$SCRIPTS/build/build-and-load-containerd.sh" || BUILD_RC2=$?
        else
          log_info "Phase 2 (retry): component=$COMPONENT_ONLY, NO_CACHE=true..."
          CONTAINERD_NAMESPACE="$CONTAINERD_NS" SKIP_DASHBOARD="${SKIP_DASHBOARD:-false}" NO_CACHE=true \
            BUILD_CORE_ONLY="${BUILD_CORE_ONLY:-false}" BUILD_AGENT_ONLY="${BUILD_AGENT_ONLY:-false}" \
            "$SCRIPTS/build/build-and-load-containerd.sh" || BUILD_RC2=$?
        fi
        if [ "$BUILD_RC2" -ne 0 ]; then
          log_warn "Retry build exited with code $BUILD_RC2."
        fi
        if ! _phase2_required_images_present; then
          log_error "Required Fortuna image(s) missing in containerd ($CONTAINERD_NS) after rebuild and retry. Fix build errors above, then retry."
          log_info "Manual build from repo root: CONTAINERD_NAMESPACE=$CONTAINERD_NS $SCRIPTS/build/build-and-load-containerd.sh"
          exit 1
        fi
      fi
    else
      log_error "build-and-load-containerd.sh not found or not executable"
      exit 1
    fi
  fi
  log_success "Rebuild phase done"
  if [ "$SKIP_DEPLOY" = false ]; then
    _sync_deploy_image_tags
  fi
else
  log_info "Phase 2: Rebuild (skipped)"
  if [ "$SKIP_DEPLOY" = false ]; then
    if _phase2_required_images_present; then
      log_success "Required images already visible in containerd"
    else
      log_error "--skip-rebuild was requested, but required Fortuna image(s) are missing in containerd namespace $CONTAINERD_NS."
      log_info "Run without --skip-rebuild, or build/load manually: CONTAINERD_NAMESPACE=$CONTAINERD_NS $SCRIPTS/build/build-and-load-containerd.sh"
      exit 1
    fi
  fi
fi
echo ""

# ---- Phase 2b: Push runtime images to nodes so Core/Agent find images with imagePullPolicy: IfNotPresent ----
if [ "$DEPLOY_MINIMAL" = true ]; then
  PUSH_IMAGES_AFTER_REBUILD="${PUSH_IMAGES_AFTER_REBUILD:-false}"
else
  if [ -z "${PUSH_IMAGES_AFTER_REBUILD+x}" ]; then
    NODE_COUNT_FOR_PUSH=$(kubectl get nodes --no-headers 2>/dev/null | wc -l | tr -d ' ' || echo "0")
    if [ "${NODE_COUNT_FOR_PUSH:-0}" -gt 1 ]; then
      PUSH_IMAGES_AFTER_REBUILD=true
    else
      PUSH_IMAGES_AFTER_REBUILD=false
    fi
  fi
fi
if [ "$SKIP_DEPLOY" = false ] && [ "$SKIP_REBUILD" = false ] && [ "$PUSH_IMAGES_AFTER_REBUILD" = true ]; then
  if [ -x "$SCRIPTS/utils/push-images-to-workers.sh" ]; then
    log_info "Phase 2b: Push Core/Agent images to cluster nodes so runtime pods can start..."
    push_dashboard_arg="--no-dashboard"
    if [ "${PUSH_DASHBOARD:-false}" = "true" ]; then
      push_dashboard_arg="--include-dashboard"
    fi
    if CORE_IMAGE="fortuna-core:${VERSION}" \
      AGENT_IMAGE="fortuna-agent:${VERSION}" \
      DASHBOARD_IMAGE="fortuna-dashboard:${VERSION}" \
      "$SCRIPTS/utils/push-images-to-workers.sh" --build-if-missing "$push_dashboard_arg" 2>&1; then
      log_success "Runtime images pushed to all nodes"
    else
      log_warn "Push to nodes failed (SSH or node list). Add scripts/utils/push-images.config (see push-images.config.example) or set SSH_USER/SSH_PASS; if Core/Agent shows ErrImageNeverPull run: ./scripts/utils/push-images-to-workers.sh"
    fi
  else
    log_warn "push-images-to-workers.sh not found; if Core/Agent shows ErrImageNeverPull, run it after build to copy runtime images to cluster nodes."
  fi
  echo ""
elif [ "$SKIP_DEPLOY" = false ] && [ "$SKIP_REBUILD" = false ]; then
  log_info "Phase 2b: Node image push skipped (single-node/default or PUSH_IMAGES_AFTER_REBUILD=false)"
fi

# ---- Phase 2a: Ensure cluster addons (kube-proxy, CoreDNS) so ClusterIP/CNI work ----
if [ "$SKIP_DEPLOY" = false ] && [ "$DEPLOY_MINIMAL" = false ]; then
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
if [ "$SKIP_DEPLOY" = false ] && [ "$DEPLOY_MINIMAL" = false ]; then
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
if [ "$SKIP_DEPLOY" = false ] && [ "$DEPLOY_MINIMAL" = false ]; then
  log_info "Phase 2c: Ensure StorageClass (local-path) for PostgreSQL/NATS PVCs..."
  if [ -x "$SCRIPTS/deploy/ensure-storage-class.sh" ]; then
    if "$SCRIPTS/deploy/ensure-storage-class.sh" 2>/dev/null; then
      log_success "StorageClass ready"
    else
      log_warn "ensure-storage-class.sh failed or StorageClass not ready; PVCs may stay Pending. Install manually: kubectl apply -f https://raw.githubusercontent.com/rancher/local-path-provisioner/v0.0.24/deploy/local-path-storage.yaml"
    fi
  else
    log_warn "ensure-storage-class.sh not found; if PVCs stay Pending, install local-path-provisioner (see docs/01-getting-started/ENVIRONMENT_REQUIREMENTS.md)"
  fi
  log_info "Sleep 10s so PVCs can bind when infra is deployed..."
  sleep 10
  echo ""
fi

# ---- Phase 2d: Apply Postgres + DB clean when full deploy runs (after Flannel + StorageClass so PVC can bind) ----
# --db / --db-reset used to run in Phase 1b before Postgres was applied → pod stayed Pending. Now: apply PG, wait, then SQL, then Phase 3 deploys the rest.
if [ "$SKIP_DEPLOY" = false ] && [ "$DEPLOY_MINIMAL" = false ] && { [ "$CLEAN_DB" = true ] || [ "$DB_RESET" = true ]; }; then
  log_info "Phase 2d: Ensuring PostgreSQL for DB clean (apply manifest + wait for Ready)..."
  kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f - 2>/dev/null || true
  ensure_local_database_env
  log_info "Ensuring fortuna-secrets/postgres-credentials before PostgreSQL..."
  "$SCRIPTS/utils/ensure-fortuna-secrets.sh" "$NAMESPACE"
  PG_YAML="$PROJECT_ROOT/deploy/infrastructure/postgresql-with-age.yaml"
  if [ ! -f "$PG_YAML" ]; then
    log_error "PostgreSQL manifest not found: $PG_YAML"
    exit 1
  fi
  if _fortuna_truthy_env "${PG_RECREATE_PVC:-}"; then
    log_info "PG_RECREATE_PVC set: recreating postgres PVC before apply + DB clean..."
    _fortuna_pg_recreate_pvc "$NAMESPACE" || exit 1
  fi
  kubectl apply -f "$PG_YAML"
  log_info "Waiting for Postgres pod Ready (max ~600s; ensure PVC binds via local-path + CNI)..."
  POD=""
  _pg_wait_i=0
  for _ in $(seq 1 120); do
    _pg_wait_i=$((_pg_wait_i + 1))
    POD=$(kubectl get pods -n "$NAMESPACE" -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
    [ -z "$POD" ] && sleep 5 && continue
    READY=$(kubectl get pod -n "$NAMESPACE" "$POD" -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || echo "False")
    if [ "$READY" = "True" ]; then
      break
    fi
    PG_C_TMP=$(kubectl get pod -n "$NAMESPACE" "$POD" -o jsonpath='{.spec.containers[0].name}' 2>/dev/null || true)
    [ -n "$PG_C_TMP" ] || PG_C_TMP="postgres"
    WAIT_REASON=$(kubectl get pod -n "$NAMESPACE" "$POD" -o jsonpath='{.status.containerStatuses[0].state.waiting.reason}' 2>/dev/null || true)
    if [ "$WAIT_REASON" = "CrashLoopBackOff" ] || [ "$WAIT_REASON" = "RunContainerError" ]; then
      if _fortuna_pg_logs_suggest_corrupt_data "$NAMESPACE" "$POD" "$PG_C_TMP"; then
        if _fortuna_truthy_env "${PG_RECREATE_PVC:-}"; then
          log_warn "Corrupt PostgreSQL volume detected; PG_RECREATE_PVC set — recreating PVC..."
          _fortuna_pg_recreate_pvc "$NAMESPACE" || exit 1
          POD=""
          continue
        fi
        log_error "PostgreSQL data on PVC looks corrupt (checkpoint/WAL). Set PG_RECREATE_PVC=1 with --full --db-reset to wipe the PVC."
        kubectl logs -n "$NAMESPACE" "$POD" -c "$PG_C_TMP" --tail=35 2>/dev/null || true
        exit 1
      fi
    fi
    if [ $((_pg_wait_i % 6)) -ne 0 ]; then
      sleep 5
      continue
    fi
    PHASE=$(kubectl get pod -n "$NAMESPACE" "$POD" -o jsonpath='{.status.phase}' 2>/dev/null || echo "")
    log_info "Postgres pod $POD Ready=$READY phase=$PHASE wait_reason=${WAIT_REASON:-none} (~$((_pg_wait_i * 5))s)..."
    if [ "$PHASE" = "Pending" ]; then
      log_warn "PVC local-path pins data to one node; pod must schedule there. If events say Insufficient cpu, free CPU on that node or lower postgres requests. Recent events:"
      kubectl get events -n "$NAMESPACE" --field-selector "involvedObject.name=$POD" 2>/dev/null | tail -8 || true
    fi
    sleep 5
  done
  if [ -z "$POD" ]; then
    log_error "Postgres pod not found after apply. Check namespace and StorageClass."
    exit 1
  fi
  READY=$(kubectl get pod -n "$NAMESPACE" "$POD" -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || echo "False")
  PG_C_TMP=$(kubectl get pod -n "$NAMESPACE" "$POD" -o jsonpath='{.spec.containers[0].name}' 2>/dev/null || true)
  [ -n "$PG_C_TMP" ] || PG_C_TMP="postgres"
  if [ "$READY" != "True" ]; then
    if _fortuna_pg_logs_suggest_corrupt_data "$NAMESPACE" "$POD" "$PG_C_TMP"; then
      log_error "PostgreSQL data on PVC appears corrupt. Re-run with PG_RECREATE_PVC=1 and --db / --db-reset."
      kubectl logs -n "$NAMESPACE" "$POD" -c "$PG_C_TMP" --tail=40 2>/dev/null || true
    else
      log_error "Postgres pod $POD never became Ready."
      kubectl describe pod -n "$NAMESPACE" "$POD" | tail -50 || true
    fi
    exit 1
  fi
  PG_CONTAINER="$PG_C_TMP"
  log_info "Waiting for Postgres SQL readiness before DB clean..."
  PG_SQL_READY=false
  for _ in $(seq 1 60); do
    if kubectl exec -n "$NAMESPACE" "$POD" -c "$PG_CONTAINER" -- psql -U postgres -d fortuna -tAc "SELECT 1" 2>/dev/null | grep -qx "1"; then
      PG_SQL_READY=true
      break
    fi
    sleep 2
  done
  if [ "$PG_SQL_READY" != true ]; then
    log_error "Postgres pod is Ready but SQL is not accepting connections."
    kubectl logs -n "$NAMESPACE" "$POD" -c "$PG_CONTAINER" --tail=40 2>/dev/null || true
    exit 1
  fi
  if [ "$DB_RESET" = true ]; then
    SQL_FILE="$PROJECT_ROOT/deploy/sql/reset_database_full.sql"
    if [ -f "$SQL_FILE" ]; then
      if ! kubectl cp "$SQL_FILE" "$POD:/tmp/reset_db.sql" -n "$NAMESPACE" -c "$PG_CONTAINER"; then
        log_error "kubectl cp reset_database_full.sql failed"
        exit 1
      fi
      if ! kubectl exec -n "$NAMESPACE" "$POD" -c "$PG_CONTAINER" -- psql -U postgres -d fortuna -f /tmp/reset_db.sql; then
        log_error "psql reset_database_full.sql failed"
        exit 1
      fi
      log_success "DB full reset (DROP tables) done before full deploy."
    else
      log_error "File not found: $SQL_FILE"
      exit 1
    fi
  else
    SQL_FILE="$PROJECT_ROOT/deploy/sql/clear_all_cluster_data.sql"
    if [ -f "$SQL_FILE" ]; then
      if ! kubectl cp "$SQL_FILE" "$POD:/tmp/clear_db.sql" -n "$NAMESPACE" -c "$PG_CONTAINER"; then
        log_error "kubectl cp clear_all_cluster_data.sql failed"
        exit 1
      fi
      if ! kubectl exec -n "$NAMESPACE" "$POD" -c "$PG_CONTAINER" -- psql -U postgres -d fortuna -f /tmp/clear_db.sql; then
        log_error "psql clear_all_cluster_data.sql failed"
        exit 1
      fi
      log_success "DB data cleared (DELETE) before full deploy."
    else
      log_error "File not found: $SQL_FILE"
      exit 1
    fi
  fi
  echo ""
fi


# ---- Phase 3a: CNI (Flannel) check/fix for multi-node – short wait so script does not hang ----
if [ "$SKIP_DEPLOY" = false ] && [ "$DEPLOY_MINIMAL" = false ]; then
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
  if [ "$DEPLOY_MINIMAL" = true ]; then
    log_info "Phase 3: Deploy (component-only: $COMPONENT_ONLY)..."
    kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f - 2>/dev/null || true
    case "$COMPONENT_ONLY" in
      core)
        [ -f "$PROJECT_ROOT/deploy/fortuna-rbac.yaml" ] && kubectl apply -f "$PROJECT_ROOT/deploy/fortuna-rbac.yaml" || true
        ensure_core_agent_secrets
        [ -f "$PROJECT_ROOT/deploy/fortuna-core-deployment.yaml" ] && kubectl apply -f "$PROJECT_ROOT/deploy/fortuna-core-deployment.yaml" || true
        _set_workload_images_for_tag
        log_info "Phase 3b: Rollout restart Core..."
        kubectl rollout restart deployment/fortuna-core -n "$NAMESPACE" 2>/dev/null || true
        log_info "Waiting for Core rollout (max 120s)..."
        kubectl rollout status deployment/fortuna-core -n "$NAMESPACE" --timeout=120s 2>/dev/null || log_warn "Core rollout status check failed or timed out"
        log_info "Phase 3c: Verify rollout (core)..."
        CORE_OK=false
        kubectl rollout status deployment/fortuna-core -n "$NAMESPACE" --timeout=5s 2>/dev/null && CORE_OK=true || true
        if [ "$CORE_OK" = true ]; then log_success "  Core: rolled out"; else log_warn "  Core: not rolled out or still updating"; fi
        ;;
      agent)
        [ -f "$PROJECT_ROOT/deploy/fortuna-rbac.yaml" ] && kubectl apply -f "$PROJECT_ROOT/deploy/fortuna-rbac.yaml" || true
        ensure_core_agent_secrets
        [ -f "$PROJECT_ROOT/deploy/fortuna-agent-daemonset.yaml" ] && kubectl apply -f "$PROJECT_ROOT/deploy/fortuna-agent-daemonset.yaml" || true
        ensure_agent_schedules_on_nodes
        _set_workload_images_for_tag
        log_info "Phase 3b: Rollout restart Agent DaemonSet..."
        kubectl rollout restart daemonset/fortuna-agent -n "$NAMESPACE" 2>/dev/null || true
        log_info "Waiting for Agent DaemonSet rollout (max 120s)..."
        kubectl rollout status daemonset/fortuna-agent -n "$NAMESPACE" --timeout=120s 2>/dev/null || log_warn "Agent DaemonSet rollout status check failed or timed out"
        log_info "Phase 3c: Verify rollout (agent)..."
        AGENT_OK=false
        kubectl rollout status daemonset/fortuna-agent -n "$NAMESPACE" --timeout=5s 2>/dev/null && AGENT_OK=true || true
        if [ "$AGENT_OK" = true ]; then log_success "  Agent: rolled out"; else log_warn "  Agent: not rolled out or still updating"; fi
        log_info "Sleep 10s for Agent sync to run..."
        sleep 10
        ;;
      dashboard)
        [ -f "$PROJECT_ROOT/deploy/dashboard-nginx-configmap.yaml" ] && kubectl apply -f "$PROJECT_ROOT/deploy/dashboard-nginx-configmap.yaml" || true
        [ -f "$PROJECT_ROOT/deploy/dashboard-deployment.yaml" ] && kubectl apply -f "$PROJECT_ROOT/deploy/dashboard-deployment.yaml" || true
        _set_workload_images_for_tag
        log_info "Phase 3b: Rollout restart Dashboard..."
        kubectl rollout restart deployment/fortuna-dashboard -n "$NAMESPACE" 2>/dev/null || true
        log_info "Waiting for Dashboard rollout (max 90s)..."
        kubectl rollout status deployment/fortuna-dashboard -n "$NAMESPACE" --timeout=90s 2>/dev/null || log_warn "Dashboard rollout status check failed or timed out"
        log_info "Phase 3c: Verify rollout (dashboard)..."
        DASH_OK=false
        kubectl rollout status deployment/fortuna-dashboard -n "$NAMESPACE" --timeout=5s 2>/dev/null && DASH_OK=true || true
        if [ "$DASH_OK" = true ]; then log_success "  Dashboard: rolled out"; else log_warn "  Dashboard: not rolled out or still updating"; fi
        ;;
    esac
    log_success "Deploy complete (component-only)"
  else
    log_info "Phase 3: Deploy (infra, RBAC, core, agent, dashboard)..."
    ensure_core_agent_secrets
    if [ -x "$SCRIPTS/deploy/deploy-fortuna-robust.sh" ]; then
      # Runtime add-ons are handled in Phase 4 below. Keep deploy-fortuna-robust
      # focused on base workloads here to avoid installing Falco twice in one run.
      WITH_FALCO=false WITH_EBPF=false "$SCRIPTS/deploy/deploy-fortuna-robust.sh"
    else
      kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -
      ensure_local_database_env
      log_info "Ensuring fortuna-secrets before infrastructure/Core..."
      "$SCRIPTS/utils/ensure-fortuna-secrets.sh" "$NAMESPACE"
      PG_YAML="$(_fortuna_postgres_manifest_yaml)"
      [ -n "$PG_YAML" ] && kubectl apply -f "$PG_YAML" || true
      [ -f "$PROJECT_ROOT/deploy/infrastructure/nats.yaml" ] && kubectl apply -f "$PROJECT_ROOT/deploy/infrastructure/nats.yaml" || true
      [ -f "$PROJECT_ROOT/deploy/fortuna-rbac.yaml" ] && kubectl apply -f "$PROJECT_ROOT/deploy/fortuna-rbac.yaml" || true
      [ -f "$PROJECT_ROOT/deploy/fortuna-core-deployment.yaml" ] && kubectl apply -f "$PROJECT_ROOT/deploy/fortuna-core-deployment.yaml" || true
      [ -f "$PROJECT_ROOT/deploy/fortuna-agent-daemonset.yaml" ] && kubectl apply -f "$PROJECT_ROOT/deploy/fortuna-agent-daemonset.yaml" || true
    fi
    ensure_agent_schedules_on_nodes
    [ -f "$PROJECT_ROOT/deploy/dashboard-nginx-configmap.yaml" ] && kubectl apply -f "$PROJECT_ROOT/deploy/dashboard-nginx-configmap.yaml" || true
    [ -f "$PROJECT_ROOT/deploy/dashboard-deployment.yaml" ] && kubectl apply -f "$PROJECT_ROOT/deploy/dashboard-deployment.yaml" || true
    _set_workload_images_for_tag
    # Rollout restart is done inside deploy-fortuna-robust.sh (Step 10); here as safety net if deploy was partial
    log_info "Phase 3b: Rollout restart (Core, Dashboard, Agent) to use new images..."
    kubectl rollout restart deployment/fortuna-core -n "$NAMESPACE" 2>/dev/null || true
    kubectl rollout restart deployment/fortuna-dashboard -n "$NAMESPACE" 2>/dev/null || true
    kubectl rollout restart daemonset/fortuna-agent -n "$NAMESPACE" 2>/dev/null || true
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
  fi
else
  log_info "Phase 3: Deploy (skipped)"
fi

# ---- Phase 3d: Remote Agent-only cluster sync (multi-cluster) ----
if [ "$SKIP_DEPLOY" = false ] && [ "$DEPLOY_MINIMAL" = false ] && [ -n "${REMOTE_KUBECONFIGS:-}" ]; then
  if [ -x "$SCRIPTS/deploy/sync-remote-agent.sh" ]; then
    log_info "Phase 3d: Sync remote Agent-only clusters..."
    if VERSION="$VERSION" "$SCRIPTS/deploy/sync-remote-agent.sh"; then
      log_success "Remote Agent sync complete"
    else
      if [ "${REMOTE_SYNC_REQUIRED:-true}" = "false" ]; then
        log_warn "Remote Agent sync failed. Check REMOTE_KUBECONFIGS, MANAGEMENT_NODE/Core endpoints, image registry or SSH image import config."
      else
        log_error "Remote Agent sync failed. Set REMOTE_SYNC_REQUIRED=false to continue despite remote sync errors."
        exit 1
      fi
    fi
  else
    log_warn "sync-remote-agent.sh not found; skipping remote cluster sync."
  fi
fi

# ---- Phase 4: Runtime security (Falco + eBPF) ----
if [ "$WITH_FALCO" = true ] && [ "$SKIP_DEPLOY" = false ]; then
  log_info "Phase 4a: Deploy Falco (Helm) for runtime security events..."
  if [ -x "$SCRIPTS/deploy/install-falco-fortuna.sh" ]; then
    if FORTUNA_NAMESPACE="$NAMESPACE" "$SCRIPTS/deploy/install-falco-fortuna.sh"; then
      log_success "Falco deployed via Helm"
      ensure_falco_schedules_on_nodes
      log_info "Waiting for Falco pods (max 180s)..."
      kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=falco -n "$NAMESPACE" --timeout=180s 2>/dev/null || log_warn "Falco pods not ready yet (may need more time)"
    else
      log_warn "Falco install failed; skipping"
    fi
  else
    log_warn "install-falco-fortuna.sh not found at $SCRIPTS/deploy/install-falco-fortuna.sh; skipping Falco"
  fi
fi

RUNTIME_RESTART_NEEDED=false
if [ "$WITH_FALCO" = true ] && [ "$SKIP_DEPLOY" = false ]; then
  CURRENT_FALCO=$(kubectl get daemonset fortuna-agent -n "$NAMESPACE" -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="FALCO_EVENTS_ENABLED")].value}' 2>/dev/null || echo "")
  CURRENT_FALCO_PATH=$(kubectl get daemonset fortuna-agent -n "$NAMESPACE" -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="FALCO_EVENTS_PATH")].value}' 2>/dev/null || echo "")
  CURRENT_FALCO_POLL=$(kubectl get daemonset fortuna-agent -n "$NAMESPACE" -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="FALCO_EVENTS_POLL")].value}' 2>/dev/null || echo "")
  EXPECTED_FALCO_PATH="${FALCO_EVENTS_PATH:-/var/log/falco/events.jsonl}"
  EXPECTED_FALCO_POLL="${FALCO_EVENTS_POLL:-5s}"
  if [ "$CURRENT_FALCO" != "true" ] || [ "$CURRENT_FALCO_PATH" != "$EXPECTED_FALCO_PATH" ] || [ "$CURRENT_FALCO_POLL" != "$EXPECTED_FALCO_POLL" ]; then
    log_info "Setting Falco reader env on Agent DaemonSet..."
    kubectl set env daemonset/fortuna-agent -n "$NAMESPACE" \
      FALCO_EVENTS_ENABLED=true \
      FALCO_EVENTS_PATH="$EXPECTED_FALCO_PATH" \
      FALCO_EVENTS_POLL="$EXPECTED_FALCO_POLL" 2>/dev/null || true
    RUNTIME_RESTART_NEEDED=true
  else
    log_info "Agent already has Falco reader env configured"
  fi
fi

if [ "$WITH_EBPF" = true ] && [ "$SKIP_DEPLOY" = false ]; then
  log_info "Phase 4b: Enable eBPF sensor on Agent DaemonSet..."
  CURRENT_EBPF=$(kubectl get daemonset fortuna-agent -n "$NAMESPACE" -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="EBPF_ENABLED")].value}' 2>/dev/null || echo "")
  if [ "$CURRENT_EBPF" != "true" ]; then
    kubectl set env daemonset/fortuna-agent -n "$NAMESPACE" EBPF_ENABLED=true 2>/dev/null || true
    RUNTIME_RESTART_NEEDED=true
    log_success "eBPF sensor enabled on Agent"
  else
    log_info "Agent already has EBPF_ENABLED=true"
  fi
fi

if [ "$RUNTIME_RESTART_NEEDED" = true ]; then
  log_info "Rollout restart Agent for runtime config changes..."
  kubectl rollout restart daemonset/fortuna-agent -n "$NAMESPACE" 2>/dev/null || true
  kubectl rollout status daemonset/fortuna-agent -n "$NAMESPACE" --timeout=120s 2>/dev/null || log_warn "Agent rollout status check timed out"
  log_success "Agent restarted with runtime configuration"
fi

if [ "$WITH_FALCO" = true ] && [ "$SKIP_DEPLOY" = false ] && [ "$FALCO_E2E_ENABLED" = true ]; then
  log_info "Phase 4c: Trigger and verify Falco runtime event flow..."
  if [ -x "$SCRIPTS/e2e/test-falco-runtime-e2e.sh" ]; then
    if NAMESPACE="$NAMESPACE" \
      TEST_NAMESPACE="$FALCO_E2E_NAMESPACE" \
      TRIGGER_POD="$FALCO_E2E_TRIGGER_POD" \
      KEEP_TRIGGER_RUNNING="$FALCO_E2E_KEEP_POD" \
      TRIGGER_HOLD_SECONDS="$FALCO_E2E_HOLD_SECONDS" \
      INVENTORY_WAIT_SECONDS="$FALCO_E2E_INVENTORY_WAIT_SECONDS" \
      CLEANUP="$FALCO_E2E_CLEANUP" \
      "$SCRIPTS/e2e/test-falco-runtime-e2e.sh"; then
      log_success "Falco runtime event flow verified"
      if [ "$FALCO_E2E_KEEP_POD" = true ] && [ "$FALCO_E2E_CLEANUP" != true ]; then
        log_info "  Dashboard test pod kept: namespace=$FALCO_E2E_NAMESPACE pod=$FALCO_E2E_TRIGGER_POD"
      fi
    else
      log_warn "Falco runtime event flow verification failed"
      [ "$FALCO_E2E_REQUIRED" = true ] && exit 1
    fi
  else
    log_warn "test-falco-runtime-e2e.sh not found; skipping Falco runtime verification"
  fi
fi

# ---- Phase 5: Post-deploy verification ----
if [ "$SKIP_DEPLOY" = false ]; then
  echo ""
  log_info "Phase 5: Post-deploy verification..."
  VERIFY_PASS=0
  VERIFY_FAIL=0
  FORTUNA_PG_RECOVERED=0
  _v_ok()   { VERIFY_PASS=$((VERIFY_PASS + 1)); log_success "  ✓ $1"; }
  _v_fail() { VERIFY_FAIL=$((VERIFY_FAIL + 1)); log_warn "  ✗ $1"; }

  # 5a: All pods Running
  log_info "  5a: Pod status (namespace=$NAMESPACE)..."
  NOT_RUNNING=$(kubectl get pods -n "$NAMESPACE" --no-headers 2>/dev/null | grep -v -E "Running|Completed" || true)
  if [ -z "$NOT_RUNNING" ]; then
    _v_ok "All pods Running"
  else
    _v_fail "Some pods not Running:"
    echo "$NOT_RUNNING" | while read -r line; do echo "       $line"; done
  fi

  # 5a2: Postgres must be Ready and queryable before Core/Agent checks are meaningful.
  log_info "  5a2: Postgres DB readiness..."
  if _fortuna_pg_wait_ready "$NAMESPACE" "${PHASE5_POSTGRES_WAIT_SECONDS:-120}" true; then
    _v_ok "Postgres DB query OK"
  else
    _v_fail "Postgres DB not usable"
  fi
  if [ "${FORTUNA_PG_RECOVERED:-0}" = "1" ]; then
    log_warn "Postgres PVC was recovered during Phase 5; restarting Core, Dashboard, and Agent so migrations and reporters reconnect cleanly."
    kubectl rollout restart deployment/fortuna-core deployment/fortuna-dashboard -n "$NAMESPACE" >/dev/null 2>&1 || true
    kubectl rollout status deployment/fortuna-core -n "$NAMESPACE" --timeout=240s || _v_fail "Core rollout after Postgres recovery did not finish"
    kubectl rollout status deployment/fortuna-dashboard -n "$NAMESPACE" --timeout=180s || _v_fail "Dashboard rollout after Postgres recovery did not finish"
    kubectl rollout restart daemonset/fortuna-agent -n "$NAMESPACE" >/dev/null 2>&1 || true
    kubectl rollout status daemonset/fortuna-agent -n "$NAMESPACE" --timeout=240s || _v_fail "Agent rollout after Postgres recovery did not finish"
  fi

  # 5b: Core endpoints
  CORE_EP=$(kubectl get endpoints -n "$NAMESPACE" fortuna-core -o jsonpath='{.subsets[0].addresses[0].ip}' 2>/dev/null || echo "")
  if [ -n "$CORE_EP" ]; then
    _v_ok "Core service has endpoints ($CORE_EP)"
  else
    _v_fail "Core service has no endpoints"
  fi

  # 5c: Core health (via kubectl exec to avoid port-forward; Core image has curl, not wget)
  CORE_POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
  if [ -n "$CORE_POD" ]; then
    HEALTH=$(kubectl exec -n "$NAMESPACE" "$CORE_POD" -- curl -fsS --max-time 5 http://localhost:8080/healthz 2>/dev/null || echo "")
    if echo "$HEALTH" | grep -qiE 'ok|healthy|alive'; then
      _v_ok "Core /healthz OK"
    else
      _v_fail "Core /healthz not responding (may still be starting)"
    fi
  fi

  # 5d: Core readiness. Avoid raw log grep here: Core persists Kubernetes event messages,
  # and old event text can contain "connection refused", causing false DB-error warnings.
  if [ -n "$CORE_POD" ]; then
    READY_HEALTH=$(kubectl exec -n "$NAMESPACE" "$CORE_POD" -- curl -fsS --max-time 5 http://localhost:8080/ready 2>/dev/null || echo "")
    if echo "$READY_HEALTH" | grep -qiE 'ok|ready|healthy|alive'; then
      _v_ok "Core /ready OK"
    else
      _v_fail "Core /ready not responding"
    fi
  fi

  # 5d2: CVE catalog recovery after DB/PVC reset. Core can be healthy while
  # reference tables are empty, which makes SBOM match runs misleading.
  CVE_CHECK_MODE="${CVE_CATALOG_POST_DEPLOY_CHECK:-auto}"
  RUN_CVE_CATALOG_CHECK=false
  case "$CVE_CHECK_MODE" in
    required|true|1|yes) RUN_CVE_CATALOG_CHECK=true ;;
    skip|false|0|no) RUN_CVE_CATALOG_CHECK=false ;;
    auto|*)
      if [ "$DB_RESET" = true ] || _fortuna_truthy_env "${PG_RECREATE_PVC:-}" || [ "${FORTUNA_PG_RECOVERED:-0}" = "1" ]; then
        RUN_CVE_CATALOG_CHECK=true
      fi
      ;;
  esac
  if [ "$RUN_CVE_CATALOG_CHECK" = true ]; then
    log_info "  5d2: Post-reset CVE catalog readiness..."
    if [ -x "$SCRIPTS/verify/ensure-cve-catalog-ready.sh" ]; then
      if NAMESPACE="$NAMESPACE" \
        PROJECT_ROOT="$PROJECT_ROOT" \
        AUTO_LOAD_CVE_CATALOG="${AUTO_LOAD_CVE_CATALOG:-true}" \
        REQUIRE_CVE_CATALOG="${REQUIRE_CVE_CATALOG:-true}" \
        CLEAN_LOCAL_SOURCE_AFTER_LOAD="${CLEAN_LOCAL_SOURCE_AFTER_LOAD:-false}" \
        "$SCRIPTS/verify/ensure-cve-catalog-ready.sh"; then
        _v_ok "CVE catalog ready after reset"
      else
        _v_fail "CVE catalog unavailable after reset"
      fi
    else
      _v_fail "CVE catalog guard script missing: $SCRIPTS/verify/ensure-cve-catalog-ready.sh"
    fi
  else
    log_info "  - CVE catalog post-reset check skipped (mode=$CVE_CHECK_MODE)"
  fi

  # 5e: Agent pods on all nodes
  NODE_COUNT=$(kubectl get nodes --no-headers 2>/dev/null | wc -l)
  AGENT_PODS=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=agent --no-headers 2>/dev/null | awk '$3 == "Running" {count++} END {print count + 0}')
  if [ "$AGENT_PODS" -ge "$NODE_COUNT" ]; then
    _v_ok "Agent running on all $NODE_COUNT node(s) ($AGENT_PODS pods)"
  else
    _v_fail "Agent pods=$AGENT_PODS but nodes=$NODE_COUNT (some nodes missing agent)"
  fi

  # 5f: Agent connectivity — check agent logs for Core connection success
  AGENT_POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=agent -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
  if [ -n "$AGENT_POD" ]; then
    AGENT_CONNECTED=$(kubectl logs -n "$NAMESPACE" "$AGENT_POD" --tail=300 2>/dev/null | grep -ciE 'connected|heartbeat.*ok|sync.*success|Full sync completed|registered|SBOM sent to Core|Sending SBOM' || true)
    AGENT_ERRS=$(kubectl logs -n "$NAMESPACE" "$AGENT_POD" --tail=300 2>/dev/null | grep -ciE 'connect.*failed|no such host|connection refused|POST failed|failed to store SBOM' || true)
    AGENT_DB_OK=0
    AGENT_DB_POD="${PG_POD:-$(kubectl get pods -n "$NAMESPACE" -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")}"
    if [ -n "$AGENT_DB_POD" ]; then
      AGENT_DB_OK=$(kubectl exec -n "$NAMESPACE" "$AGENT_DB_POD" -- psql -U postgres -d fortuna -tAc "SELECT count(*) FROM agents WHERE deleted_at IS NULL AND status IN ('ready','online','active','healthy','running') AND last_seen_at >= now() - interval '10 minutes'" 2>/dev/null | tr -d '[:space:]' || echo "0")
    fi
    if [ "${AGENT_CONNECTED:-0}" -gt 0 ] || [ "${AGENT_DB_OK:-0}" -gt 0 ]; then
      _v_ok "Agent connected to Core"
      if [ "${AGENT_ERRS:-0}" -gt 0 ]; then
        log_info "  - Agent had $AGENT_ERRS transient startup connection error(s), but later sent data to Core"
      fi
    elif [ "${AGENT_ERRS:-0}" -gt 0 ]; then
      _v_fail "Agent has connection errors (check: kubectl logs -n $NAMESPACE $AGENT_POD)"
    else
      _v_fail "Agent connection status unclear (still starting?)"
    fi
  fi

  # 5g: Falco verification (if deployed)
  if [ "$WITH_FALCO" = true ]; then
    FALCO_RUNNING=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/name=falco --no-headers 2>/dev/null | awk '$3 == "Running" {count++} END {print count + 0}')
    if [ "$FALCO_RUNNING" -gt 0 ]; then
      _v_ok "Falco: $FALCO_RUNNING pod(s) Running"
    else
      _v_fail "Falco: no Running pods"
    fi
    if [ -n "$AGENT_POD" ]; then
      FALCO_ENV=$(kubectl get daemonset/fortuna-agent -n "$NAMESPACE" -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="FALCO_EVENTS_ENABLED")].value}' 2>/dev/null || echo "")
      FALCO_PATH=$(kubectl get daemonset/fortuna-agent -n "$NAMESPACE" -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="FALCO_EVENTS_PATH")].value}' 2>/dev/null || echo "")
      FALCO_LOG_OK=$(kubectl logs -n "$NAMESPACE" "$AGENT_POD" --tail=120 2>/dev/null | grep -ciE 'Falco.*reader.*enabled|FalcoEvents|IngestQuality|send_ok' || true)
      if [ "$FALCO_ENV" = "true" ] && [ -n "$FALCO_PATH" ]; then
        _v_ok "Agent: Falco reader configured"
        if [ "${FALCO_LOG_OK:-0}" -eq 0 ]; then
          log_info "  - Falco reader log line not in recent tail; using DaemonSet env + runtime DB verification"
        fi
      else
        _v_fail "Agent: Falco reader env is not configured"
      fi
    fi
  fi

  # 5h: eBPF verification (if enabled)
  if [ "$WITH_EBPF" = true ] && [ -n "$AGENT_POD" ]; then
    EBPF_ENV=$(kubectl get daemonset/fortuna-agent -n "$NAMESPACE" -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="EBPF_ENABLED")].value}' 2>/dev/null || echo "")
    EBPF_OK=$(kubectl logs -n "$NAMESPACE" "$AGENT_POD" --tail=120 2>/dev/null | grep -ciE 'eBPF.*sensor.*enabled|eBPF.*started|\[eBPF\].*heartbeat|eBPF.*heartbeat' || true)
    if [ "${EBPF_OK:-0}" -gt 0 ]; then
      _v_ok "Agent: eBPF sensor active"
    elif [ "$EBPF_ENV" = "true" ]; then
      _v_ok "Agent: eBPF sensor configured"
      log_info "  - eBPF heartbeat not in recent tail yet; using DaemonSet env as readiness signal"
    else
      _v_fail "Agent: eBPF sensor not configured"
    fi
  fi

  # 5i: Runtime events check — wait briefly then query DB
  if { [ "$WITH_FALCO" = true ] || [ "$WITH_EBPF" = true ]; } && { [ "$DB_RESET" = true ] || [ "$CLEAN_DB" = true ]; }; then
    log_info "  5i: Waiting 30s for runtime events to accumulate..."
    sleep 30
  fi
  PG_POD=$(kubectl get pods -n "$NAMESPACE" -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
  if [ -n "$PG_POD" ]; then
    EVT_COUNT=$(kubectl exec -n "$NAMESPACE" "$PG_POD" -- psql -U postgres -d fortuna -tAc "SELECT count(*) FROM runtime_events" 2>/dev/null || echo "0")
    EVT_COUNT=$(echo "$EVT_COUNT" | tr -d '[:space:]')
    if [ "${EVT_COUNT:-0}" -gt 0 ]; then
      _v_ok "Runtime events in DB: $EVT_COUNT"
    else
      if [ "$WITH_FALCO" = true ] || [ "$WITH_EBPF" = true ]; then
        _v_fail "Runtime events in DB: 0 (may need more time; Falco needs kernel activity to generate events)"
      else
        log_info "  - Runtime events: 0 (runtime features not enabled)"
      fi
    fi
    POD_PROC_COUNT=$(kubectl exec -n "$NAMESPACE" "$PG_POD" -- psql -U postgres -d fortuna -tAc "SELECT count(DISTINCT pod_uid) FROM pod_processes" 2>/dev/null || echo "0")
    POD_PROC_COUNT=$(echo "$POD_PROC_COUNT" | tr -d '[:space:]')
    if [ "${POD_PROC_COUNT:-0}" -gt 0 ]; then
      _v_ok "Process snapshots: $POD_PROC_COUNT pods with data"
    else
      _v_fail "Process snapshots: 0 pods (agent PodDetail reporter may need more time)"
    fi
  fi

  # 5j: Scenarios namespace pods (fortuna-test)
  FT_PODS=$(kubectl get pods -n fortuna-test --no-headers 2>/dev/null | wc -l || echo "0")
  if [ "${FT_PODS:-0}" -gt 0 ]; then
    _v_ok "fortuna-test namespace: $FT_PODS pods"
  else
    log_info "  - fortuna-test namespace: no pods (deploy scenarios: kubectl apply -f scenarios/)"
  fi

  echo ""
  if [ "$VERIFY_FAIL" -eq 0 ]; then
    log_success "Phase 5: All $VERIFY_PASS checks passed"
    _cleanup_old_fortuna_images
  else
    log_warn "Phase 5: $VERIFY_PASS passed, $VERIFY_FAIL failed (see warnings above)"
    exit 1
  fi
fi

fi
# ---- ^ end skip Phases 1–5 when ONLY_E2E=true ----

# ---- Phase 6: E2E tests + report ----
if [ "$WITH_E2E" = true ] && { [ "$SKIP_DEPLOY" = false ] || [ "$ONLY_E2E" = true ]; }; then
  echo ""
  log_info "Phase 6: Running E2E tests (suite=$E2E_SUITE)..."
  log_info "  Waiting 30s for all components to stabilize before E2E..."
  sleep 30
  E2E_RC=0
  E2E_REPORT_FILE=""
  case "$E2E_SUITE" in
    full-report)
      if [ -x "$SCRIPTS/e2e/run-e2e-with-capability-report.sh" ]; then
        log_info "  Running run-e2e-with-capability-report.sh (full report with capabilities)..."
        E2E_CLEANUP_ARG=
        [ "$E2E_CLEANUP" = true ] && E2E_CLEANUP_ARG="--cleanup"
        NAMESPACE="$NAMESPACE" E2E_CLEANUP="$E2E_CLEANUP" E2E_WITH_SCENARIO="$E2E_WITH_SCENARIO" "$SCRIPTS/e2e/run-e2e-with-capability-report.sh" $E2E_CLEANUP_ARG || E2E_RC=$?
        E2E_REPORT_FILE=$(ls -t "$PROJECT_ROOT"/test-results/E2E-WITH-CAPABILITY-*.md 2>/dev/null | head -1 || true)
      else
        log_warn "  run-e2e-with-capability-report.sh not found; falling back to run-e2e.sh --suite=full"
        E2E_SUITE=full
        NAMESPACE="$NAMESPACE" SUITE="$E2E_SUITE" "$SCRIPTS/e2e/run-e2e.sh" --suite="$E2E_SUITE" || E2E_RC=$?
        E2E_REPORT_FILE=$(ls -t "$PROJECT_ROOT"/test-results/E2E-FULL-*.md 2>/dev/null | head -1 || true)
      fi
      ;;
    full)
      if [ -x "$SCRIPTS/e2e/run-e2e.sh" ]; then
        log_info "  Running run-e2e.sh --suite=full..."
        NAMESPACE="$NAMESPACE" "$SCRIPTS/e2e/run-e2e.sh" --suite=full || E2E_RC=$?
        E2E_REPORT_FILE=$(ls -t "$PROJECT_ROOT"/test-results/E2E-FULL-*.md 2>/dev/null | head -1 || true)
      else
        log_warn "  run-e2e.sh not found"
        E2E_RC=1
      fi
      ;;
    *)
      if [ -x "$SCRIPTS/e2e/run-e2e.sh" ]; then
        log_info "  Running run-e2e.sh --suite=$E2E_SUITE..."
        NAMESPACE="$NAMESPACE" "$SCRIPTS/e2e/run-e2e.sh" --suite="$E2E_SUITE" || E2E_RC=$?
      else
        log_warn "  run-e2e.sh not found"
        E2E_RC=1
      fi
      ;;
  esac
  echo ""
  if [ "$E2E_RC" -eq 0 ]; then
    log_success "Phase 6: E2E tests passed (suite=$E2E_SUITE)"
  else
    log_warn "Phase 6: E2E tests finished with exit code $E2E_RC (some tests may have failed)"
  fi
  if [ -n "${E2E_REPORT_FILE:-}" ] && [ -f "$E2E_REPORT_FILE" ]; then
    log_success "  Report: $E2E_REPORT_FILE ($(wc -l < "$E2E_REPORT_FILE") lines)"
  fi
fi

echo ""
echo "=========================================="
log_success "Full clean / rebuild / deploy finished."
echo "=========================================="
echo "  Verify: kubectl get pods -n $NAMESPACE"
echo "  Rollout status: kubectl rollout status deployment/fortuna-core -n $NAMESPACE && kubectl rollout status deployment/fortuna-dashboard -n $NAMESPACE && kubectl rollout status daemonset/fortuna-agent -n $NAMESPACE"
echo "  Rebuild/deploy status (image ID vs local): ./scripts/verify/verify-core-agent-rebuild-deploy-status.sh"
echo "  Core API: kubectl port-forward -n $NAMESPACE svc/fortuna-core 8080:8080"
echo "  Dashboard: kubectl port-forward -n $NAMESPACE svc/fortuna-dashboard 8081:80"
echo "  Agents: wait 1–2 min; GET /api/v1/agents/status"
echo "  If agent shows [Syncer] Sync failed: status=500, check Core logs for 'column kubeconfig does not exist';"
echo "    fix: run with --db-reset and redeploy, or add clusters.kubeconfig (migration 062) and restart Core."
echo "  If Core pod shows ErrImageNeverPull: image must be on the node that runs Core (control-plane)."
echo "    fix: ./scripts/utils/push-images-to-workers.sh (pushes Core/Agent to nodes including master; set SSH_USER/SSH_PASS or use keys)."
echo "  Dashboard is scheduled on control-plane/master for local registryless deploys; set PUSH_DASHBOARD=true only if you intentionally schedule it elsewhere."
echo "  If Agent CrashLoopBackOff (OOMKilled): daemonset has memory limit 2Gi; optional SBOM_WORKERS=1 and rebuild agent."
echo "  Monitor errors: ./scripts/monitor/monitor-agent-core-errors.sh (or --follow). Troubleshooting: docs/05-operations/DEPLOYMENT.md"
echo "  Malware DB: Core auto-syncs Aikido feeds on startup (~122k packages). Verify: curl localhost:8080/api/v1/malware/stats"
echo "    Manual upload: curl -X POST localhost:8080/api/v1/malware/db/upload -d @malware.json"
echo "  E2E tests: ./scripts/e2e/run-e2e.sh --suite=full-report  (or: --with-e2e flag in this pipeline)"
echo "    Suites: full | full-report | risk-center | priority1 | runtime | falco-runtime | pce | dashboard | sbom | sbom-full"
echo "    Reports: test-results/E2E-WITH-CAPABILITY-*.md"
echo ""
