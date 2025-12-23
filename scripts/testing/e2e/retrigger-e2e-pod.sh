#!/bin/bash
set -e

# Quick script to re-trigger E2E pod processing
# This deletes and recreates the pod to generate fresh events

NAMESPACE="ksam-e2e"
POD_NAME="ksam-e2e-vuln-debian10"

echo "========================================"
echo "Re-triggering E2E Pod Processing"
echo "========================================"
echo ""

# Step 1: Delete existing pod
echo "Step 1: Deleting existing pod..."
kubectl delete pod -n "$NAMESPACE" "$POD_NAME" --ignore-not-found=true
echo "✅ Pod deleted"
echo ""

# Wait for deletion to complete
echo "Step 2: Waiting for deletion to complete..."
sleep 5
echo "✅ Ready to recreate"
echo ""

# Step 3: Recreate pod
echo "Step 3: Recreating pod..."
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: $POD_NAME
  namespace: $NAMESPACE
  labels:
    app: ksam-e2e-vuln
    test: e2e-cve-insight
spec:
  restartPolicy: Never
  containers:
    - name: app
      image: ksam/e2e-vuln:e2e-20251218141020
      command: ["sh", "-c", "echo 'KSAM E2E pod running' && sleep 3600"]
EOF

echo "✅ Pod recreated"
echo ""

# Step 4: Wait for pod to be ready
echo "Step 4: Waiting for pod to be Running..."
kubectl wait --for=condition=Ready pod/"$POD_NAME" -n "$NAMESPACE" --timeout=60s
echo "✅ Pod is Running"
echo ""

# Get pod UID
POD_UID=$(kubectl get pod -n "$NAMESPACE" "$POD_NAME" -o jsonpath='{.metadata.uid}')
echo "New Pod UID: $POD_UID"
echo ""

echo "========================================"
echo "✅ Pod re-triggered successfully!"
echo "========================================"
echo ""
echo "Next steps:"
echo "1. Wait 2-3 minutes for SBOM generation"
echo "2. Run: ./scripts/verify-e2e-pipeline.sh"
echo ""
echo "Or watch logs:"
echo "  kubectl logs -n ksam -l app=ksam-core -f | grep -E 'SBOMWorker|CVEMatcher|$POD_NAME'"
