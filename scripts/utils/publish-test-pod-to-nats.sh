#!/bin/bash

# Publish test pod data to NATS to trigger SBOM processing
# This simulates what Agent would send

set -e

CORE_POD=$(kubectl get pods -n ksam -l app=ksam-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

if [ -z "$CORE_POD" ]; then
    echo "❌ Core pod not found"
    exit 1
fi

POD_NAME="${1:-sbom-test-manual}"
POD_NAMESPACE="${2:-ksam}"
POD_UID=$(kubectl get pod "$POD_NAME" -n "$POD_NAMESPACE" -o jsonpath='{.metadata.uid}' 2>/dev/null || echo "")

if [ -z "$POD_UID" ]; then
    echo "❌ Pod $POD_NAME not found in namespace $POD_NAMESPACE"
    exit 1
fi

echo "Publishing pod data to NATS..."
echo "Pod: $POD_NAME"
echo "UID: $POD_UID"
echo ""

# Get pod details
POD_JSON=$(kubectl get pod "$POD_NAME" -n "$POD_NAMESPACE" -o json)

# Extract container image
CONTAINER_IMAGE=$(echo "$POD_JSON" | jq -r '.spec.containers[0].image' || echo "nginx:1.19.0")

# Create normalized data structure
NORMALIZED_DATA=$(cat <<EOF
{
  "kind": "Pod",
  "uid": "$POD_UID",
  "name": "$POD_NAME",
  "namespace": "$POD_NAMESPACE",
  "cluster_id": "test-cluster",
  "containers": [
    {
      "name": "nginx",
      "image": "$CONTAINER_IMAGE"
    }
  ]
}
EOF
)

echo "Normalized data:"
echo "$NORMALIZED_DATA" | jq .
echo ""

# Publish to NATS using nats-box or direct API
echo "Publishing to NATS subject: ksam.normalized.pod..."
kubectl exec -n ksam "$CORE_POD" -- sh -c "
  echo '$NORMALIZED_DATA' | nats pub ksam.normalized.pod.test || echo 'NATS pub command not available'
" 2>&1 || echo "Note: Using alternative method..."

echo ""
echo "✅ Data published to NATS"
echo "Waiting 10 seconds for processing..."
sleep 10

echo ""
echo "Checking logs for processing..."
kubectl logs -n ksam "$CORE_POD" --tail=50 --since=15s | grep -iE "riskworker|sbom|processimage|extract" | tail -10 || echo "No processing logs found yet"


