#!/usr/bin/env bash
# ============================================================================
# Ensure Flannel CNI is installed so pod network works (subnet.env, etc.)
# ============================================================================
# Many kubeadm clusters expect Flannel but do not install it by default. Without
# Flannel (or another CNI), pods fail with "plugin type=flannel failed ...
# open /run/flannel/subnet.env: no such file or directory" and PVCs never bind
# because local-path-provisioner cannot start.
#
# This script:
#   - Treats Flannel as OK only when flannel pods are **Running** (not ConfigMap-only).
#   - If another CNI is present (Calico/Cilium/Weave), skips Flannel install.
#   - Otherwise applies the official Flannel manifest and waits for Ready.
#
# Called by: deploy-fortuna-robust.sh, full-clean-database-rebuild-deploy.sh,
#            check-prerequisites-core-agent.sh, pre-deployment-checks.sh (optional).
#
# Usage: ./scripts/deploy/ensure-flannel.sh
# Env:   SKIP_FLANNEL_INSTALL=1     do not install, only check (exit 0 / 1)
#        FLANNEL_MANIFEST_URL       override manifest URL
#        FLANNEL_WAIT_TIMEOUT=120   wait for pods (default 120s)
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

# Prefer pinned release; master may change without notice
FLANNEL_MANIFEST_URL="${FLANNEL_MANIFEST_URL:-https://raw.githubusercontent.com/flannel-io/flannel/v0.26.0/Documentation/kube-flannel.yml}"
WAIT_TIMEOUT="${FLANNEL_WAIT_TIMEOUT:-120}"

# True when at least one Flannel pod is Running (subnet.env gets written on nodes)
flannel_healthy() {
  local n
  n=$(kubectl get pods -n kube-flannel -l app=flannel --field-selector=status.phase=Running --no-headers 2>/dev/null | wc -l)
  [ "${n:-0}" -ge 1 ]
}

# Another CNI is running — do not install Flannel on top
has_other_cni() {
  kubectl get pods -n kube-system --no-headers 2>/dev/null | grep -qE 'calico-node|cilium|weave-net|kube-weave|canal' || \
  kubectl get pods -A --no-headers 2>/dev/null | grep -qE 'calico-node|cilium-agent|weave-net|kube-weave'
}

# Legacy/partial install: ConfigMap or DS exists but pods not Running — repair
try_repair_flannel() {
  if ! kubectl get ns kube-flannel &>/dev/null; then
    return 1
  fi
  local ds
  ds=$(kubectl get ds -n kube-flannel -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
  if [ -n "$ds" ]; then
    log_info "Attempting rollout restart of DaemonSet $ds (partial/broken Flannel)..."
    kubectl rollout restart "daemonset/$ds" -n kube-flannel 2>/dev/null || true
    sleep 10
    flannel_healthy && return 0
  fi
  return 1
}

log_info "Checking CNI (Flannel)..."

if flannel_healthy; then
  log_success "Flannel is running"
  kubectl get pods -n kube-flannel -o wide 2>/dev/null || true
  exit 0
fi

if has_other_cni; then
  log_warn "Another CNI appears to be running (Calico/Cilium/Weave/Canal). Skipping Flannel install."
  exit 0
fi

if [ "${SKIP_FLANNEL_INSTALL:-0}" = "1" ]; then
  log_warn "Flannel not healthy; SKIP_FLANNEL_INSTALL=1. Pod network may fail (subnet.env missing)."
  log_info "Install manually: kubectl apply -f $FLANNEL_MANIFEST_URL"
  exit 0
fi

# ConfigMap-only or broken install: do not exit early — re-apply or restart
if kubectl get configmap kube-flannel-cfg -n kube-flannel &>/dev/null && ! flannel_healthy; then
  log_warn "kube-flannel ConfigMap exists but no Running flannel pods — repairing..."
  try_repair_flannel || true
fi

if flannel_healthy; then
  log_success "Flannel is running after repair"
  kubectl get pods -n kube-flannel -o wide 2>/dev/null || true
  exit 0
fi

log_info "Flannel not healthy. Applying manifest: $FLANNEL_MANIFEST_URL"
if ! kubectl apply -f "$FLANNEL_MANIFEST_URL" 2>&1; then
  log_error "Failed to apply Flannel manifest"
  log_info "Try manually: kubectl apply -f $FLANNEL_MANIFEST_URL"
  exit 1
fi

log_info "Waiting for Flannel pods (timeout=${WAIT_TIMEOUT}s)..."
if kubectl wait --for=condition=ready pod -l app=flannel -n kube-flannel --timeout="${WAIT_TIMEOUT}s" 2>/dev/null; then
  log_success "Flannel pods are Ready"
else
  log_warn "Flannel pods may still be starting. Check: kubectl get pods -n kube-flannel -o wide"
fi

log_info "Waiting 15s for Flannel to write /run/flannel/subnet.env on nodes..."
sleep 15

if flannel_healthy; then
  log_success "Flannel is installed and running"
  kubectl get pods -n kube-flannel -o wide 2>/dev/null || true
  exit 0
fi

log_error "Flannel is not healthy; pod network may still fail (subnet.env)"
kubectl get pods -n kube-flannel -o wide 2>/dev/null || true
exit 1
