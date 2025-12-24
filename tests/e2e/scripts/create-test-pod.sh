#!/bin/bash

# ============================================================================
# Create Test Pod Script
# ============================================================================
# Utility script to create a test pod for E2E testing
# ============================================================================

set -euo pipefail

NAMESPACE="${TEST_NAMESPACE:-ksam-test}"
POD_NAME="${POD_NAME:-test-pod-$(date +%s)}"
IMAGE="${TEST_IMAGE:-nginx:latest}"

echo "Creating test pod: ${POD_NAME}"
echo "Namespace: ${NAMESPACE}"
echo "Image: ${IMAGE}"

kubectl run "${POD_NAME}" \
    --image="${IMAGE}" \
    --namespace="${NAMESPACE}" \
    --restart=Never \
    --labels="test=ksam-e2e,app=test-pod"

echo "Waiting for pod to be ready..."
kubectl wait --for=condition=ready pod/"${POD_NAME}" -n "${NAMESPACE}" --timeout=120s

POD_UID=$(kubectl get pod "${POD_NAME}" -n "${NAMESPACE}" -o jsonpath='{.metadata.uid}')

echo "✅ Pod created successfully"
echo "Pod Name: ${POD_NAME}"
echo "Pod UID: ${POD_UID}"
echo "Namespace: ${NAMESPACE}"

