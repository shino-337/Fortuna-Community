#!/bin/bash
# MVP1 Cleanup Script
# Cleans up Docker images and Kubernetes resources

set -e

echo "=========================================="
echo "MVP1 Cleanup Script"
echo "=========================================="
echo ""

# 1. Docker Image Cleanup
echo "1. Cleaning Docker images..."
echo "   Removing old tagged images..."
docker rmi ksam-core:v20251206164654 ksam-core:v20251206164609 2>/dev/null || echo "   No old images to remove"
docker rmi ksam-agent:v* 2>/dev/null || echo "   No old agent images to remove"

echo "   Removing dangling images..."
docker image prune -f

echo "   Current images:"
docker images | grep ksam || echo "   No ksam images found"
echo ""

# 2. Kubernetes Resource Cleanup
echo "2. Cleaning Kubernetes resources..."
echo "   Cleaning old replicasets (keeping last 3)..."
OLD_RS=$(kubectl get replicasets -n ksam --sort-by=.metadata.creationTimestamp 2>/dev/null | tail -n +4 | awk '{print $1}' | head -10)
if [ -n "$OLD_RS" ]; then
    echo "$OLD_RS" | xargs -r kubectl delete replicaset -n ksam 2>/dev/null || true
    echo "   Removed old replicasets"
else
    echo "   No old replicasets to remove"
fi

echo "   Cleaning completed jobs..."
COMPLETED_JOBS=$(kubectl get jobs -n ksam 2>/dev/null | grep -E "Completed|Failed" | awk '{print $1}' || true)
if [ -n "$COMPLETED_JOBS" ]; then
    echo "$COMPLETED_JOBS" | xargs -r kubectl delete job -n ksam 2>/dev/null || true
    echo "   Removed completed jobs"
else
    echo "   No completed jobs to remove"
fi

echo "   Current resources:"
kubectl get deployments,services -n ksam | head -5
echo ""

# 3. Summary
echo "=========================================="
echo "Cleanup Complete"
echo "=========================================="
echo ""
echo "Remaining resources:"
echo "  - Deployments: $(kubectl get deployments -n ksam --no-headers 2>/dev/null | wc -l)"
echo "  - Services: $(kubectl get services -n ksam --no-headers 2>/dev/null | wc -l)"
echo "  - Images: $(docker images | grep ksam | wc -l)"
echo ""
