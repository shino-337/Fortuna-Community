#!/usr/bin/env bash
# ============================================================================
# Ensure StorageClass (local-path) exists for Fortuna PVCs
# ============================================================================
# Checks for StorageClass "local-path"; if missing, installs Rancher
# local-path-provisioner so PostgreSQL/NATS PVCs can bind (avoid Pending).
# Called by: deploy-fortuna-robust.sh (Step 3b), full-clean-database-rebuild-deploy.sh (Phase 2c).
#
# Usage: ./scripts/deploy/ensure-storage-class.sh
# Env:   SKIP_STORAGE_CLASS_INSTALL=1  to only check, never install
#        STORAGE_CLASS_WAIT_TIMEOUT=120  wait for provisioner pods (default 120s)
# ============================================================================

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'
log_info()    { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[OK]${NC} $1"; }
log_warn()    { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error()   { echo -e "${RED}[ERR]${NC} $1"; }

LOCAL_PATH_PROVISIONER_URL="${LOCAL_PATH_PROVISIONER_URL:-https://raw.githubusercontent.com/rancher/local-path-provisioner/v0.0.24/deploy/local-path-storage.yaml}"
WAIT_READY_TIMEOUT="${STORAGE_CLASS_WAIT_TIMEOUT:-120}"

# Check if StorageClass "local-path" exists (required by deploy/infrastructure/*.yaml)
has_storage_class() {
  kubectl get storageclass local-path 2>/dev/null | grep -q local-path
}

wait_for_provisioner() {
  if ! kubectl get namespace local-path-storage >/dev/null 2>&1; then
    log_warn "local-path StorageClass exists, but namespace local-path-storage is missing."
    return 1
  fi

  kubectl delete pod -n local-path-storage -l app=local-path-provisioner \
    --field-selector=status.phase=Failed --ignore-not-found=true --wait=false >/dev/null 2>&1 || true

  log_info "Waiting for local-path-provisioner pods (timeout=${WAIT_READY_TIMEOUT}s)..."
  kubectl wait --for=condition=ready pod -l app=local-path-provisioner \
    -n local-path-storage --timeout="${WAIT_READY_TIMEOUT}s" 2>/dev/null
}

log_info "Checking StorageClass (local-path or default)..."
if has_storage_class; then
  log_success "StorageClass already available"
  kubectl get storageclass 2>/dev/null || true
  if wait_for_provisioner; then
    log_success "local-path-provisioner is ready"
  else
    log_warn "local-path-provisioner is not Ready; PVCs may stay Pending. Check: kubectl get pods -n local-path-storage"
  fi
  exit 0
fi

if [ "${SKIP_STORAGE_CLASS_INSTALL:-0}" = "1" ]; then
  log_warn "No StorageClass found; SKIP_STORAGE_CLASS_INSTALL=1, skipping install. PVCs may stay Pending."
  log_info "To install manually: kubectl apply -f $LOCAL_PATH_PROVISIONER_URL"
  exit 0
fi

log_info "No StorageClass found. Installing local-path-provisioner..."
if ! kubectl apply -f "$LOCAL_PATH_PROVISIONER_URL" 2>&1; then
  log_error "Failed to apply local-path-provisioner manifest"
  log_info "Install manually: kubectl apply -f $LOCAL_PATH_PROVISIONER_URL"
  exit 1
fi

if wait_for_provisioner; then
  log_success "local-path-provisioner is ready"
else
  log_warn "Pods may still be starting. Check: kubectl get pods -n local-path-storage"
fi

# Retry until StorageClass appears (provisioner may create it shortly after pods are ready)
for i in 1 2 3 4 5 6 7 8 9 10; do
  if has_storage_class; then
    log_success "StorageClass is now available"
    kubectl get storageclass 2>/dev/null || true
    exit 0
  fi
  log_info "Waiting for StorageClass local-path to appear (attempt $i/10)..."
  sleep 5
done

# Provisioner might set default class without name "local-path"
if kubectl get storageclass 2>/dev/null | grep -q .; then
  log_success "StorageClass(s) present (check name matches deploy YAMLs: local-path)"
  kubectl get storageclass 2>/dev/null || true
  exit 0
fi

log_error "StorageClass still not found after install. PVCs will stay Pending."
log_info "Manual fix: kubectl get pods -n local-path-storage && kubectl get storageclass"
exit 1
