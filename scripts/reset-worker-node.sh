#!/bin/bash

# Worker Node Reset Script
# Run this script on the worker node to reset and prepare for joining new cluster

set -e

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║     WORKER NODE RESET SCRIPT                                  ║"
echo "╚══════════════════════════════════════════════════════════════╝"
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

# Check if running as root
if [ "$EUID" -ne 0 ]; then 
    log_error "Please run as root"
    exit 1
fi

log_info "Step 1: Resetting kubeadm..."
kubeadm reset --force

log_info "Step 2: Cleaning up CNI configuration..."
rm -rf /etc/cni/net.d/*
rm -rf /var/lib/cni/*

log_info "Step 3: Stopping kubelet..."
systemctl stop kubelet || true

log_info "Step 4: Cleaning up kubelet data..."
rm -rf /var/lib/kubelet/*

log_info "Step 5: Cleaning up iptables rules (optional)..."
iptables -F && iptables -t nat -F && iptables -t mangle -F && iptables -X || true

log_info "Step 6: Removing old kubeconfig..."
rm -rf $HOME/.kube/config || true

log_info "✅ Worker node reset complete!"
echo ""
log_info "Now you can join the cluster with:"
echo ""
echo "kubeadm join 192.168.56.100:6443 --token ij4i4w.fwhtxwvy4pfg0bqg \\"
echo "	--discovery-token-ca-cert-hash sha256:03c3ec188f565348fde0e3b5c0c504ff4fff606f69098e274034c7ea1c4df204"
echo ""
