#!/bin/bash

# Script to clear old images (when pods are not running)
# Usage: ./scripts/clear_images.sh [namespace] [--force]

set -e

NAMESPACE="${1:-ksam}"
FORCE="${2:-}"

echo "🧹 Clearing Old Images"
echo "Namespace: $NAMESPACE"
echo ""

# Check if pods are running
CORE_PODS=$(kubectl get pods -n $NAMESPACE -l app=ksam-core --no-headers 2>/dev/null | wc -l || echo "0")
AGENT_PODS=$(kubectl get pods -n $NAMESPACE -l app=ksam-agent --no-headers 2>/dev/null | wc -l || echo "0")

if [ "$CORE_PODS" -gt 0 ] || [ "$AGENT_PODS" -gt 0 ]; then
    if [ "$FORCE" != "--force" ]; then
        echo "⚠️  Warning: Pods are running. Images cannot be deleted while in use."
        echo "   To force clear, stop pods first or use --force flag"
        echo ""
        echo "   To stop pods:"
        echo "   kubectl scale deployment ksam-core -n $NAMESPACE --replicas=0"
        echo "   kubectl scale daemonset ksam-agent -n $NAMESPACE --replicas=0"
        exit 1
    else
        echo "⚠️  Force mode: Will attempt to clear images (may fail if pods are running)"
    fi
fi

# Clear minikube images
echo "Step 1: Clearing minikube images..."
minikube ssh -- docker images | grep ksam | awk '{print $3}' | xargs -r minikube ssh -- docker rmi -f 2>&1 || echo "  Some images may be in use"
minikube ssh -- docker system prune -f > /dev/null 2>&1 || true
echo "  ✅ Minikube cleanup completed"

# Clear docker images
echo ""
echo "Step 2: Clearing docker images..."
docker images | grep ksam | awk '{print $3}' | xargs -r docker rmi -f 2>&1 || echo "  Some images may be in use"
docker system prune -f > /dev/null 2>&1 || true
echo "  ✅ Docker cleanup completed"

# Verify
echo ""
echo "Step 3: Verification..."
MINIKUBE_IMAGES=$(minikube ssh -- docker images | grep ksam | wc -l || echo "0")
DOCKER_IMAGES=$(docker images | grep ksam | wc -l || echo "0")

if [ "$MINIKUBE_IMAGES" -eq 0 ] && [ "$DOCKER_IMAGES" -eq 0 ]; then
    echo "  ✅ All ksam images cleared"
else
    echo "  ⚠️  Some images may still exist (in use by running containers)"
    echo "     Minikube: $MINIKUBE_IMAGES images"
    echo "     Docker: $DOCKER_IMAGES images"
fi

echo ""
echo "✅ Image cleanup completed"


