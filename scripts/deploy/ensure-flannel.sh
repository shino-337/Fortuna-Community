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

# Flannel is considered healthy only when DaemonSet has all desired pods Ready.
flannel_healthy() {
  local desired ready
  desired=$(kubectl get ds -n kube-flannel -l app=flannel -o jsonpath='{.items[0].status.desiredNumberScheduled}' 2>/dev/null || echo "0")
  ready=$(kubectl get ds -n kube-flannel -l app=flannel -o jsonpath='{.items[0].status.numberReady}' 2>/dev/null || echo "0")
  [ "${desired:-0}" -gt 0 ] && [ "${ready:-0}" -ge "${desired:-0}" ]
}

# Detect common node-side CNI binary issue: missing /opt/cni/bin/loopback.
has_loopback_cni_error() {
  kubectl get events -A --sort-by=.lastTimestamp 2>/dev/null | grep -q 'failed to find plugin "loopback" in path \[/opt/cni/bin\]'
}

# Best-effort repair when loopback plugin is missing on nodes.
# Uses scripts/utils/push-images.config if present, or SSH_USER/SSH_PASS from env.
try_repair_cni_binaries_via_ssh() {
  local cfg="$PROJECT_ROOT/scripts/utils/push-images.config"
  local node_ips user pass target rc=0

  # shellcheck disable=SC1090
  [ -f "$cfg" ] && source "$cfg" 2>/dev/null || true

  node_ips=$(kubectl get nodes -o jsonpath='{.items[*].status.addresses[?(@.type=="InternalIP")].address}' 2>/dev/null || true)
  [ -n "$node_ips" ] || return 1

  for ip in $node_ips; do
    if [ -n "${MASTER_NODE:-}" ] && [ "$ip" = "$MASTER_NODE" ]; then
      user="${MASTER_SSH_USER:-${SSH_USER:-}}"
      pass="${MASTER_SSH_PASS:-${SSH_PASS:-}}"
    else
      user="${WORKER_SSH_USER:-${SSH_USER:-}}"
      pass="${WORKER_SSH_PASS:-${SSH_PASS:-}}"
    fi
    [ -n "$user" ] || { log_warn "Skip CNI repair on $ip (missing SSH user)"; rc=1; continue; }
    target="$user@$ip"

    if command -v sshpass >/dev/null 2>&1 && [ -n "$pass" ]; then
      log_info "Repairing CNI binaries on $target (kubernetes-cni reinstall)..."
      if ! sshpass -p "$pass" ssh -o StrictHostKeyChecking=no -o ConnectTimeout=10 "$target"         "echo '$pass' | sudo -S apt-get install --reinstall -y kubernetes-cni >/dev/null && test -x /opt/cni/bin/loopback"; then
        log_warn "CNI repair failed on $target"
        rc=1
      fi
    else
      log_warn "Cannot auto-repair $target (sshpass/pass unavailable). Use SSH key or install sshpass."
      rc=1
    fi
  done

  return $rc
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

# Self-heal common CNI issue before checking Flannel health.
if has_loopback_cni_error; then
  log_warn "Detected missing loopback CNI plugin on one or more nodes (/opt/cni/bin/loopback)."
  if try_repair_cni_binaries_via_ssh; then
    log_success "CNI plugin repair done. Restarting flannel/kube-proxy pods..."
    kubectl delete pod -n kube-flannel -l app=flannel --wait=false >/dev/null 2>&1 || true
    kubectl delete pod -n kube-system -l k8s-app=kube-proxy --wait=false >/dev/null 2>&1 || true
    sleep 10
  else
    log_warn "Auto-repair incomplete. Manual fix on each node: sudo apt-get install --reinstall -y kubernetes-cni"
  fi
fi

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
