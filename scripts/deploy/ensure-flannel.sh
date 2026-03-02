#!/usr/bin/env bash
# ============================================================================
# Ensure Flannel CNI is installed so pod network works (subnet.env, etc.)
# ============================================================================
# Many kubeadm clusters expect Flannel but do not install it by default. Without
# Flannel (or another CNI), pods fail with "plugin type=flannel failed ...
# open /run/flannel/subnet.env: no such file or directory" and PVCs never bind
# because local-path-provisioner cannot start.
#
# This script: if Flannel is not installed (no kube-flannel namespace or no
# kube-flannel-cfg / kube-flannel-ds), applies the official Flannel manifest
# and waits for pods to be Running. If another CNI is already present (Calico,
# Cilium, etc.), skip install (optional: set SKIP_FLANNEL_INSTALL=1).
#
# Called by: deploy-fortuna-robust.sh (before StorageClass), full-clean-database-rebuild-deploy.sh (Phase 2b).
#
# Usage: ./scripts/deploy/ensure-flannel.sh
# Env:   SKIP_FLANNEL_INSTALL=1     do not install, only check
#        FLANNEL_MANIFEST_URL       override manifest URL
#        FLANNEL_WAIT_TIMEOUT=120  wait for pods (default 120s)
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

# Prefer versioned tag; fallback to master if tag not found
FLANNEL_MANIFEST_URL="${FLANNEL_MANIFEST_URL:-https://raw.githubusercontent.com/flannel-io/flannel/master/Documentation/kube-flannel.yml}"
WAIT_TIMEOUT="${FLANNEL_WAIT_TIMEOUT:-120}"

# Flannel is installed if we have the ConfigMap or DaemonSet or running pods in kube-flannel
has_flannel() {
  kubectl get configmap kube-flannel-cfg -n kube-flannel &>/dev/null && return 0
  kubectl get daemonset kube-flannel-ds -n kube-flannel &>/dev/null && return 0
  [ "$(kubectl get pods -n kube-flannel --no-headers 2>/dev/null | wc -l)" -ge 1 ] 2>/dev/null && return 0
  return 1
}

# Another CNI is running (we could skip Flannel install)
has_other_cni() {
  kubectl get pods -n kube-system --no-headers 2>/dev/null | grep -qE 'calico|cilium|weave|canal' || \
  kubectl get pods -A --no-headers 2>/dev/null | grep -qE 'calico|cilium|weave'
}

log_info "Checking CNI (Flannel)..."
if has_flannel; then
  log_success "Flannel already installed"
  kubectl get pods -n kube-flannel --no-headers 2>/dev/null || true
  exit 0
fi

if [ "${SKIP_FLANNEL_INSTALL:-0}" = "1" ]; then
  log_warn "Flannel not found; SKIP_FLANNEL_INSTALL=1. Pod network may fail (subnet.env missing)."
  log_info "Install manually: kubectl apply -f $FLANNEL_MANIFEST_URL"
  exit 0
fi

if has_other_cni; then
  log_warn "Another CNI appears to be running (Calico/Cilium/Weave). Skipping Flannel install."
  exit 0
fi

log_info "Flannel not installed. Applying manifest..."
if ! kubectl apply -f "$FLANNEL_MANIFEST_URL" 2>&1; then
  log_error "Failed to apply Flannel manifest"
  log_info "Try manually: kubectl apply -f $FLANNEL_MANIFEST_URL"
  exit 1
fi

log_info "Waiting for Flannel pods (timeout=${WAIT_TIMEOUT}s)..."
if kubectl wait --for=condition=ready pod -l app=flannel -n kube-flannel --timeout="${WAIT_TIMEOUT}s" 2>/dev/null; then
  log_success "Flannel pods are ready"
else
  log_warn "Flannel pods may still be starting. Check: kubectl get pods -n kube-flannel"
fi

# Give nodes time to write subnet.env so subsequent pods (e.g. local-path-provisioner) can start
log_info "Waiting 15s for Flannel to write subnet.env on nodes..."
sleep 15

if has_flannel; then
  log_success "Flannel is installed and running"
  kubectl get pods -n kube-flannel -o wide 2>/dev/null || true
  exit 0
fi

log_error "Flannel may not be ready; pod network could still fail"
exit 1
