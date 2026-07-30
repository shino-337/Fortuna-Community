#!/usr/bin/env bash
# ============================================================================
# Ensure cluster addons: kube-proxy, CoreDNS
# ============================================================================
# Without kube-proxy, ClusterIP (e.g. 10.96.0.1 for API server) does not work;
# Flannel and local-path-provisioner can then fail. Without CoreDNS, pod DNS
# fails. This script checks and, if possible, installs addons via kubeadm.
# Called by full-clean-database-rebuild-deploy.sh (Phase 2a) before StorageClass.
#
# Usage: ./scripts/deploy/ensure-cluster-addons.sh
# Env:   SKIP_CLUSTER_ADDONS_INSTALL=1  to only check, never install
#        KUBECONFIG or /etc/kubernetes/admin.conf for kubeadm
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

KUBECONFIG_PATH="${KUBECONFIG:-/etc/kubernetes/admin.conf}"
WAIT_READY_TIMEOUT="${CLUSTER_ADDONS_WAIT_TIMEOUT:-90}"

# Check if kube-proxy is running
has_kube_proxy() {
  local count
  count=$(kubectl get pods -n kube-system -l k8s-app=kube-proxy --no-headers 2>/dev/null | grep -c "Running" || echo "0")
  [ "${count:-0}" -ge 1 ]
}

# Check if CoreDNS is running
has_coredns() {
  local count
  count=$(kubectl get pods -n kube-system -l k8s-app=kube-dns --no-headers 2>/dev/null | grep -c "Running" || echo "0")
  [ "${count:-0}" -ge 1 ]
}

log_info "Checking cluster addons (kube-proxy, CoreDNS)..."

if has_kube_proxy && has_coredns; then
  log_success "kube-proxy and CoreDNS already running"
  kubectl get pods -n kube-system -l k8s-app=kube-proxy --no-headers 2>/dev/null || true
  kubectl get pods -n kube-system -l k8s-app=kube-dns --no-headers 2>/dev/null || true
  exit 0
fi

NEED_PROXY=false
NEED_DNS=false
has_kube_proxy || NEED_PROXY=true
has_coredns || NEED_DNS=true

if [ "${SKIP_CLUSTER_ADDONS_INSTALL:-0}" = "1" ]; then
  [ "$NEED_PROXY" = true ] && log_warn "kube-proxy not running; SKIP_CLUSTER_ADDONS_INSTALL=1, not installing. ClusterIP/Flannel may fail."
  [ "$NEED_DNS" = true ] && log_warn "CoreDNS not running; SKIP_CLUSTER_ADDONS_INSTALL=1, not installing. Pod DNS may fail."
  exit 0
fi

if ! command -v kubeadm &>/dev/null; then
  log_warn "kubeadm not found; cannot install addons. Ensure kube-proxy and CoreDNS are installed on the cluster."
  [ "$NEED_PROXY" = true ] && log_warn "Install kube-proxy: kubeadm init phase addon kube-proxy --kubeconfig <kubeconfig>"
  [ "$NEED_DNS" = true ] && log_warn "Install CoreDNS: kubeadm init phase addon coredns --kubeconfig <kubeconfig>"
  exit 0
fi

if [ ! -f "$KUBECONFIG_PATH" ]; then
  log_warn "Kubeconfig not found at $KUBECONFIG_PATH; cannot run kubeadm phase addon."
  exit 0
fi

if [ "$NEED_PROXY" = true ]; then
  log_info "Installing kube-proxy addon..."
  if kubeadm init phase addon kube-proxy --kubeconfig "$KUBECONFIG_PATH" 2>&1; then
    log_success "kube-proxy addon applied"
  else
    log_warn "kubeadm addon kube-proxy failed (may already exist or need manual install)"
  fi
fi

if [ "$NEED_DNS" = true ]; then
  log_info "Installing CoreDNS addon..."
  if kubeadm init phase addon coredns --kubeconfig "$KUBECONFIG_PATH" 2>&1; then
    log_success "CoreDNS addon applied"
  else
    log_warn "kubeadm addon coredns failed (may already exist or need manual install)"
  fi
fi

log_info "Waiting for addon pods to be ready (timeout=${WAIT_READY_TIMEOUT}s)..."
sleep 5

if [ "$NEED_PROXY" = true ]; then
  if kubectl wait --for=condition=ready pod -l k8s-app=kube-proxy -n kube-system --timeout="${WAIT_READY_TIMEOUT}s" 2>/dev/null; then
    log_success "kube-proxy is ready"
  else
    log_warn "kube-proxy may still be starting; check: kubectl get pods -n kube-system -l k8s-app=kube-proxy"
  fi
fi

if [ "$NEED_DNS" = true ]; then
  if kubectl wait --for=condition=ready pod -l k8s-app=kube-dns -n kube-system --timeout="${WAIT_READY_TIMEOUT}s" 2>/dev/null; then
    log_success "CoreDNS is ready"
  else
    log_warn "CoreDNS may still be starting; check: kubectl get pods -n kube-system -l k8s-app=kube-dns"
  fi
fi

log_info "Short wait for addons to stabilize..."
sleep 10

log_success "Cluster addons check/install complete"
