#!/bin/bash

# Trigger SBOM processing by publishing pod data to NATS
# This simulates what Agent/Normalizer would send

set -e

CORE_POD=$(kubectl get pods -n ksam -l app=ksam-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

if [ -z "$CORE_POD" ]; then
    echo "❌ Core pod not found"
    exit 1
fi

POD_NAME="${1:-sbom-test-manual}"
POD_NAMESPACE="${2:-ksam}"

# Get pod details
POD_UID=$(kubectl get pod "$POD_NAME" -n "$POD_NAMESPACE" -o jsonpath='{.metadata.uid}' 2>/dev/null || echo "")
if [ -z "$POD_UID" ]; then
    echo "❌ Pod $POD_NAME not found in namespace $POD_NAMESPACE"
    exit 1
fi

# Get container image
CONTAINER_IMAGE=$(kubectl get pod "$POD_NAME" -n "$POD_NAMESPACE" -o jsonpath='{.spec.containers[0].image}' 2>/dev/null || echo "nginx:1.19.0")
CONTAINER_NAME=$(kubectl get pod "$POD_NAME" -n "$POD_NAMESPACE" -o jsonpath='{.spec.containers[0].name}' 2>/dev/null || echo "nginx")

echo "╔════════════════════════════════════════════════════════════════╗"
echo "║     Trigger SBOM Processing for Pod                           ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""
echo "Pod: $POD_NAME"
echo "Namespace: $POD_NAMESPACE"
echo "UID: $POD_UID"
echo "Image: $CONTAINER_IMAGE"
echo ""

# Create normalized data JSON
NORMALIZED_JSON=$(cat <<EOF
{
  "kind": "Pod",
  "uid": "$POD_UID",
  "name": "$POD_NAME",
  "namespace": "$POD_NAMESPACE",
  "cluster_id": "test-cluster",
  "containers": [
    {
      "name": "$CONTAINER_NAME",
      "image": "$CONTAINER_IMAGE"
    }
  ],
  "event_type": "Added"
}
EOF
)

echo "Normalized data:"
echo "$NORMALIZED_JSON" | jq .
echo ""

# Check if nats CLI is available in pod
echo "Checking NATS connectivity..."
NATS_ENDPOINT=$(kubectl exec -n ksam "$CORE_POD" -- env | grep NATS_ENDPOINT | cut -d'=' -f2 || echo "nats://nats.ksam.svc.cluster.local:4222")
echo "NATS Endpoint: $NATS_ENDPOINT"
echo ""

# Try to publish using Go code or API
echo "Publishing to NATS subject: ksam.normalized.pod.test..."
echo ""

# Method 1: Use kubectl exec to call API endpoint (if available)
echo "Method 1: Trying API endpoint..."
API_RESPONSE=$(kubectl exec -n ksam "$CORE_POD" -- sh -c "
  curl -s -X POST http://localhost:8080/api/v1/test/publish -H 'Content-Type: application/json' -d '$NORMALIZED_JSON' 2>&1 || echo 'API endpoint not available'
" 2>&1 || echo "Failed")

if echo "$API_RESPONSE" | grep -q "success\|ok\|published"; then
    echo "✅ Published via API"
else
    echo "⚠️  API method failed, trying direct NATS..."
    
    # Method 2: Use nats-box if available
    NATS_BOX=$(kubectl get pods -n ksam -l app=nats-box -o name 2>/dev/null | head -1 | sed 's|pod/||' || echo "")
    if [ -n "$NATS_BOX" ]; then
        echo "Using nats-box pod: $NATS_BOX"
        kubectl exec -n ksam "$NATS_BOX" -- nats pub ksam.normalized.pod.test "$NORMALIZED_JSON" 2>&1 || echo "Failed to publish via nats-box"
    else
        echo "⚠️  nats-box not available"
        echo ""
        echo "💡 Alternative: The pod data should be automatically processed when:"
        echo "   1. Agent collects the pod"
        echo "   2. Normalizer processes it"
        echo "   3. RiskWorker receives normalized data"
        echo ""
        echo "   To trigger manually, you can:"
        echo "   - Restart Agent to re-sync"
        echo "   - Wait for Agent to collect new pods"
    fi
fi

echo ""
echo "Waiting 30 seconds for processing..."
sleep 30

echo ""
echo "Checking logs for SBOM processing..."
kubectl logs -n ksam "$CORE_POD" --tail=100 --since=35s 2>&1 | grep -iE "riskworker.*pod|sbom|extract|processimage|pipeline" | tail -20 || echo "No SBOM processing logs found yet"

echo ""
echo "✅ Trigger completed"


