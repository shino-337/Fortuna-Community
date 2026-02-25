#!/bin/bash

# Remote Worker Node Reset Script
# Run this from master node to reset worker node remotely

set -e

WORKER_NODE="${1:-k8s-worker}"
WORKER_USER="${2:-root}"

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║     REMOTE WORKER NODE RESET                                 ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""
echo "Target: ${WORKER_USER}@${WORKER_NODE}"
echo ""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check SSH connectivity
log_info "Checking SSH connectivity to ${WORKER_NODE}..."
if ! ssh -o ConnectTimeout=5 -o StrictHostKeyChecking=no ${WORKER_USER}@${WORKER_NODE} "echo 'Connection OK'" 2>/dev/null; then
    log_error "Cannot connect to ${WORKER_NODE}"
    log_info "Please ensure:"
    log_info "  1. SSH key is set up for passwordless login"
    log_info "  2. Worker node is accessible"
    log_info "  3. Hostname/IP is correct"
    exit 1
fi

log_info "Step 1: Resetting kubeadm on ${WORKER_NODE}..."
ssh ${WORKER_USER}@${WORKER_NODE} "kubeadm reset --force"

log_info "Step 2: Cleaning up CNI configuration..."
ssh ${WORKER_USER}@${WORKER_NODE} "rm -rf /etc/cni/net.d/* /var/lib/cni/*"

log_info "Step 3: Stopping kubelet..."
ssh ${WORKER_USER}@${WORKER_NODE} "systemctl stop kubelet || true"

log_info "Step 4: Cleaning up kubelet data..."
ssh ${WORKER_USER}@${WORKER_NODE} "rm -rf /var/lib/kubelet/*"

log_info "Step 5: Cleaning up iptables..."
ssh ${WORKER_USER}@${WORKER_NODE} "iptables -F && iptables -t nat -F && iptables -t mangle -F && iptables -X || true"

log_info "Step 6: Removing old kubeconfig..."
ssh ${WORKER_USER}@${WORKER_NODE} "rm -rf \$HOME/.kube/config || true"

log_info "✅ Worker node reset complete!"
echo ""
log_info "Now join the worker node to the cluster:"
echo ""
echo "ssh ${WORKER_USER}@${WORKER_NODE}"
echo "kubeadm join 192.168.56.100:6443 --token ij4i4w.fwhtxwvy4pfg0bqg \\"
echo "	--discovery-token-ca-cert-hash sha256:03c3ec188f565348fde0e3b5c0c504ff4fff606f69098e274034c7ea1c4df204"
echo ""
