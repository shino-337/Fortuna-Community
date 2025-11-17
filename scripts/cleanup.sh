#!/bin/bash

# Cleanup script for KSAM

set -e

echo "🧹 Cleaning up KSAM resources..."

# Delete KSAM namespace (includes Core, Dashboard, PostgreSQL)
kubectl delete namespace ksam --ignore-not-found=true

# Delete Agent resources
kubectl delete daemonset ksam-agent -n kube-system --ignore-not-found=true
kubectl delete serviceaccount ksam-agent -n kube-system --ignore-not-found=true
kubectl delete clusterrole ksam-agent-reader --ignore-not-found=true
kubectl delete clusterrolebinding ksam-agent-reader --ignore-not-found=true

# Delete test data
kubectl delete namespace test-ksam --ignore-not-found=true

# Clean up Docker images (optional)
read -p "Do you want to remove Docker images? (y/N) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    docker rmi ksam/agent:latest ksam/core:latest ksam/dashboard:latest 2>/dev/null || true
fi

echo "✅ Cleanup complete!"

