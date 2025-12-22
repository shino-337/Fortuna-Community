#!/bin/bash

# Script to start Minikube with appropriate resources for KSAM
# Recommended: 4GB RAM, 2 CPUs, 20GB disk

set -e

echo "=========================================="
echo "Starting Minikube for KSAM"
echo "=========================================="
echo ""

# Configuration
MEMORY="${MINIKUBE_MEMORY:-4096}"      # 4GB RAM (default)
CPUS="${MINIKUBE_CPUS:-2}"             # 2 CPUs (default)
DISK_SIZE="${MINIKUBE_DISK_SIZE:-20g}" # 20GB disk (default)
DRIVER="${MINIKUBE_DRIVER:-docker}"     # docker driver (default)

echo "Configuration:"
echo "  Memory: ${MEMORY}MB"
echo "  CPUs: ${CPUS}"
echo "  Disk Size: ${DISK_SIZE}"
echo "  Driver: ${DRIVER}"
echo ""

# Check if Minikube is already running
if minikube status > /dev/null 2>&1; then
    echo "⚠️  Minikube is already running"
    echo ""
    echo "Current status:"
    minikube status
    echo ""
    read -p "Do you want to stop and restart with new configuration? (y/N): " -n 1 -r
    echo ""
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "Stopping Minikube..."
        minikube stop
        echo "Deleting Minikube cluster..."
        minikube delete
    else
        echo "Keeping current Minikube configuration"
        exit 0
    fi
fi

# Check if driver is available
echo "Checking driver availability..."
if ! minikube config set driver "$DRIVER" 2>/dev/null; then
    echo "⚠️  Driver '$DRIVER' not available, trying default..."
    DRIVER=$(minikube config get driver 2>/dev/null || echo "docker")
fi
echo "Using driver: $DRIVER"
echo ""

# Start Minikube with configuration
echo "Starting Minikube..."
echo "This may take a few minutes..."
echo ""

minikube start \
    --memory="$MEMORY" \
    --cpus="$CPUS" \
    --disk-size="$DISK_SIZE" \
    --driver="$DRIVER" \
    --addons=ingress \
    --addons=metrics-server

if [ $? -eq 0 ]; then
    echo ""
    echo "✅ Minikube started successfully!"
    echo ""
    
    # Show status
    echo "Minikube status:"
    minikube status
    echo ""
    
    # Show resources
    echo "Allocated resources:"
    minikube config get memory
    minikube config get cpus
    minikube config get disk-size
    echo ""
    
    # Set kubectl context
    echo "Setting kubectl context..."
    kubectl config use-context minikube
    echo ""
    
    echo "=========================================="
    echo "Minikube is ready!"
    echo "=========================================="
    echo ""
    echo "Next steps:"
    echo "  1. Create namespace: kubectl create namespace ksam"
    echo "  2. Deploy infrastructure: kubectl apply -f deploy/infrastructure/"
    echo "  3. Deploy Core: kubectl apply -f deploy/core-deployment.yaml"
    echo "  4. Deploy Agent: kubectl apply -f deploy/agent-daemonset.yaml"
    echo ""
else
    echo ""
    echo "❌ Failed to start Minikube"
    echo ""
    echo "Troubleshooting:"
    echo "  1. Check Docker Desktop is running (if using docker driver)"
    echo "  2. Try with different driver: minikube start --driver=virtualbox"
    echo "  3. Increase system resources if available"
    exit 1
fi


