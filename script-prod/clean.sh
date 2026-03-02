#!/usr/bin/env bash
# =============================================================================
# Fortuna Production – Clean resources (namespace workloads, optional images/DB)
# Prompts for confirmation unless -y/--yes. Use with care.
# Usage:
#   ./script-prod/clean.sh              # delete Core, Agent, Dashboard only
#   ./script-prod/clean.sh --images    # also remove fortuna-* images
#   ./script-prod/clean.sh --db         # also clear DB data (DELETE, keep schema)
#   ./script-prod/clean.sh --db-reset   # also full DB reset (DROP tables)
#   ./script-prod/clean.sh -y           # skip confirmation
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$SCRIPT_DIR/lib.sh"

CLEAN_IMAGES=false
CLEAN_DB=false
DB_RESET=false
CONFIRM_YES=false
for arg in "$@"; do
  case "$arg" in
    --images)   CLEAN_IMAGES=true ;;
    --db)       CLEAN_DB=true ;;
    --db-reset) DB_RESET=true ;;
    -y|--yes)   CONFIRM_YES=true ;;
  esac
done

ensure_log_dir
LOG_FILE="$LOG_DIR/clean.log"
exec >> "$LOG_FILE" 2>&1

log_info "Clean started at $(date -Iseconds)"
log_info "NAMESPACE=$NAMESPACE CLEAN_IMAGES=$CLEAN_IMAGES CLEAN_DB=$CLEAN_DB DB_RESET=$DB_RESET"

# Confirmation
if [ "$CONFIRM_YES" != true ]; then
  echo "This will delete Core, Agent, and Dashboard workloads in namespace '$NAMESPACE'."
  [ "$CLEAN_IMAGES" = true ] && echo "  + Remove fortuna-* images from containerd/docker."
  [ "$CLEAN_DB" = true ]    && echo "  + Clear DB data (DELETE, schema kept)."
  [ "$DB_RESET" = true ]    && echo "  + Full DB reset (DROP tables)."
  echo "Continue? [y/N]"
  read -r ans
  if [ "$ans" != "y" ] && [ "$ans" != "Y" ]; then
    log_info "Aborted by user"
    exit 0
  fi
fi

require_kubectl

# 1. Delete workloads (keep namespace and infra: postgres, nats)
log_info "Deleting Core, Agent, Dashboard..."
kubectl delete deployment -n "$NAMESPACE" fortuna-core fortuna-dashboard --ignore-not-found=true --timeout=60s
kubectl delete daemonset -n "$NAMESPACE" fortuna-agent --ignore-not-found=true --timeout=60s
kubectl delete service -n "$NAMESPACE" fortuna-core fortuna-dashboard --ignore-not-found=true
kubectl delete pods -n "$NAMESPACE" -l app.kubernetes.io/component=core --ignore-not-found=true --timeout=30s
kubectl delete pods -n "$NAMESPACE" -l app.kubernetes.io/component=agent --ignore-not-found=true --timeout=30s
log_ok "Workloads deleted"
sleep 3

# 2. Optional: DB
if [ "$CLEAN_DB" = true ] || [ "$DB_RESET" = true ]; then
  PG_POD=$(kubectl get pods -n "$NAMESPACE" -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
  if [ -n "$PG_POD" ]; then
    if [ "$DB_RESET" = true ]; then
      SQL="$PROJECT_ROOT/deploy/e2e/reset_database_full.sql"
      if [ -f "$SQL" ]; then
        kubectl cp "$SQL" "$NAMESPACE/$PG_POD:/tmp/reset_db.sql" 2>/dev/null || true
        kubectl exec -n "$NAMESPACE" "$PG_POD" -- psql -U postgres -d fortuna -f /tmp/reset_db.sql 2>/dev/null || true
        log_ok "DB full reset (DROP tables) done"
      else
        log_warn "File not found: $SQL"
      fi
    else
      SQL="$PROJECT_ROOT/deploy/e2e/clear_all_cluster_data.sql"
      if [ -f "$SQL" ]; then
        kubectl cp "$SQL" "$NAMESPACE/$PG_POD:/tmp/clear_db.sql" 2>/dev/null || true
        kubectl exec -n "$NAMESPACE" "$PG_POD" -- psql -U postgres -d fortuna -f /tmp/clear_db.sql 2>/dev/null || true
        log_ok "DB data cleared (DELETE, schema kept)"
      else
        log_warn "File not found: $SQL"
      fi
    fi
  else
    log_warn "Postgres pod not found; skip DB clean"
  fi
fi

# 3. Optional: images
if [ "$CLEAN_IMAGES" = true ]; then
  log_info "Removing fortuna images..."
  if command -v nerdctl &>/dev/null; then
    nerdctl --namespace "$CONTAINERD_NS" images 2>/dev/null | grep fortuna | awk '{print $3}' | xargs -r nerdctl --namespace "$CONTAINERD_NS" rmi --force 2>/dev/null || true
    nerdctl --namespace "$CONTAINERD_NS" system prune -f 2>/dev/null || true
  elif command -v docker &>/dev/null; then
    docker images --format '{{.Repository}}:{{.Tag}}' | grep fortuna | xargs -r docker rmi -f 2>/dev/null || true
  fi
  log_ok "Images removed"
fi

log_ok "Clean complete. Log: $LOG_FILE"
