#!/bin/bash
# Quick start Minikube with recommended settings for KSAM

set -e

echo "=========================================="
echo "Quick Start Minikube for KSAM"
echo "=========================================="
echo ""

# Recommended configuration
MEMORY=6144      # 6GB RAM
CPUS=3           # 3 CPUs
DISK_SIZE=30g    # 30GB disk
DRIVER=docker    # docker driver

echo "Starting Minikube with:"
echo "  Memory: ${MEMORY}MB (6GB)"
echo "  CPUs: ${CPUS}"
echo "  Disk: ${DISK_SIZE}"
echo "  Driver: ${DRIVER}"
echo ""

# Check if already running
if minikube status > /dev/null 2>&1; then
    echo "Minikube is already running"
    minikube status
    exit 0
fi

# Start Minikube
minikube start \
    --memory="$MEMORY" \
    --cpus="$CPUS" \
    --disk-size="$DISK_SIZE" \
    --driver="$DRIVER" \
    --addons=ingress \
    --addons=metrics-server

echo ""
echo "✅ Minikube started!"
echo ""
echo "Next steps:"
echo "  kubectl create namespace ksam"
echo "  kubectl apply -f deploy/infrastructure/"
