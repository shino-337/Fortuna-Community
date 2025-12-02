#!/bin/bash

# Script to check Minikube resources and suggest optimal configuration

set -e

echo "=========================================="
echo "Minikube Resources Check"
echo "=========================================="
echo ""

# Check if Minikube is running
if ! minikube status > /dev/null 2>&1; then
    echo "⚠️  Minikube is not running"
    echo ""
    echo "To start Minikube with recommended resources:"
    echo "  bash scripts/start_minikube.sh"
    echo ""
    exit 0
fi

echo "Minikube Status:"
minikube status
echo ""

echo "Current Configuration:"
MEMORY=$(minikube config get memory 2>/dev/null || echo "not set")
CPUS=$(minikube config get cpus 2>/dev/null || echo "not set")
DISK_SIZE=$(minikube config get disk-size 2>/dev/null || echo "not set")

echo "  Memory: $MEMORY"
echo "  CPUs: $CPUS"
echo "  Disk Size: $DISK_SIZE"
echo ""

# Check actual usage
echo "Resource Usage:"
kubectl top nodes 2>/dev/null || echo "  ⚠️  Metrics server not available"
echo ""

# Check disk usage
echo "Disk Usage:"
minikube ssh -- df -h / 2>/dev/null | grep -v "^Filesystem" || echo "  ⚠️  Cannot check disk usage"
echo ""

# Recommendations
echo "=========================================="
echo "Recommendations for KSAM"
echo "=========================================="
echo ""
echo "Minimum Requirements:"
echo "  Memory: 4GB (4096MB)"
echo "  CPUs: 2"
echo "  Disk: 20GB"
echo ""
echo "Recommended:"
echo "  Memory: 6GB (6144MB) - for better performance"
echo "  CPUs: 3-4"
echo "  Disk: 30GB - for logs and images"
echo ""

# Check if current config meets minimum
MEMORY_MB=$(echo "$MEMORY" | sed 's/[^0-9]//g')
if [ -z "$MEMORY_MB" ] || [ "$MEMORY_MB" -lt 4096 ]; then
    echo "⚠️  Memory is below minimum (4GB)"
    echo "   Current: ${MEMORY_MB}MB"
    echo "   Recommended: 4096MB or higher"
fi

CPUS_INT=$(echo "$CPUS" | sed 's/[^0-9]//g')
if [ -z "$CPUS_INT" ] || [ "$CPUS_INT" -lt 2 ]; then
    echo "⚠️  CPUs is below minimum (2)"
    echo "   Current: ${CPUS_INT}"
    echo "   Recommended: 2 or higher"
fi

echo ""
echo "To update configuration:"
echo "  1. Stop Minikube: minikube stop"
echo "  2. Start with new config: bash scripts/start_minikube.sh"
echo ""

