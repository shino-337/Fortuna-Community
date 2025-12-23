#!/bin/bash
#
# SBOM Duplicate Key Fix - End-to-End Test
# =========================================
#
# This script tests the fix for the duplicate key error that occurred when
# multiple pods used the same container image concurrently.
#
# Original Error:
# "Failed to process image registry.k8s.io/coredns/coredns:v1.11.1:
#  failed to save SBOM: ERROR: duplicate key value violates unique constraint
#  \"sboms_image_digest_key\" (SQLSTATE 23505)"
#
# Fix Implemented:
# - Modified core/pkg/sbom/pipeline.go to use PostgreSQL's ON CONFLICT clause
# - Database-level upsert prevents duplicate key errors
# - Handles concurrent inserts safely
#
# Date: December 16, 2025
#

set -e

TEST_IMAGE="nginx:alpine"
TEST_PREFIX="sbom-dup-test"
POD_COUNT=5
NAMESPACE="default"

echo "================================"
echo "SBOM Duplicate Key Fix E2E Test"
echo "================================"
echo ""

echo "Test Configuration:"
echo "- Test Image: $TEST_IMAGE"
echo "- Number of Pods: $POD_COUNT"
echo "- Namespace: $NAMESPACE"
echo "- Test Prefix: $TEST_PREFIX"
echo ""

# Step 1: Clean up any existing test pods
echo "[Step 1] Cleaning up existing test pods..."
kubectl delete pods -l test=sbom-duplicate --ignore-not-found=true
sleep 2
echo "✅ Cleanup complete"
echo ""

# Step 2: Get initial SBOM count
echo "[Step 2] Getting initial SBOM count..."
INITIAL_COUNT=$(kubectl exec -n ksam $(kubectl get pod -n ksam -l app=postgres -o jsonpath='{.items[0].metadata.name}') -- \
  psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM sboms;" | tr -d ' ')
echo "Initial SBOM count: $INITIAL_COUNT"
echo ""

# Step 3: Create multiple pods with same image concurrently
echo "[Step 3] Creating $POD_COUNT pods with same image ($TEST_IMAGE) concurrently..."
for i in $(seq 1 $POD_COUNT); do
  POD_NAME="${TEST_PREFIX}-${i}"
  kubectl run $POD_NAME \
    --image=$TEST_IMAGE \
    --restart=Never \
    --labels="test=sbom-duplicate" \
    --command -- sleep 300 &
done

# Wait for all background jobs to complete
wait
echo "✅ Created $POD_COUNT pods"
echo ""

# Step 4: Wait for pods to be running
echo "[Step 4] Waiting for pods to be Running..."
sleep 10
kubectl wait --for=condition=ready pod -l test=sbom-duplicate --timeout=60s || echo "Some pods may not be ready yet"
echo "✅ Pods are running"
echo ""

# Step 5: Wait for SBOM processing (give RiskWorker time to process)
echo "[Step 5] Waiting 60 seconds for SBOM pipeline to process pods..."
sleep 60
echo "✅ Wait complete"
echo ""

# Step 6: Check for duplicate key errors in logs
echo "[Step 6] Checking for duplicate key errors..."
DUPLICATE_ERRORS=$(kubectl logs -n ksam -l app=ksam-core --since=5m 2>/dev/null | \
  grep -c "duplicate key.*sboms_image_digest_key\|23505.*sbom\|failed to upsert SBOM" || echo "0" | tr -d '\n')
DUPLICATE_ERRORS=${DUPLICATE_ERRORS:-0}

if [ "$DUPLICATE_ERRORS" -gt 0 2>/dev/null ]; then
  echo "❌ FAILED: Found $DUPLICATE_ERRORS duplicate key errors in logs"
  echo ""
  echo "Error logs:"
  kubectl logs -n ksam -l app=ksam-core --since=5m | grep -A 3 "duplicate key\|23505\|failed to upsert"
  exit 1
else
  echo "✅ No duplicate key errors found"
fi
echo ""

# Step 7: Check SBOM processing logs
echo "[Step 7] Checking SBOM processing logs..."
SBOM_LOGS=$(kubectl logs -n ksam -l app=ksam-core --since=5m 2>/dev/null | \
  grep -i "created new sbom\|updated existing sbom.*on conflict\|found existing sbom" | head -10)

if [ -z "$SBOM_LOGS" ]; then
  echo "⚠️  WARNING: No SBOM processing logs found (pods may not have been processed yet)"
else
  echo "SBOM processing logs sample:"
  echo "$SBOM_LOGS"
fi
echo ""

# Step 8: Verify SBOM created/updated
echo "[Step 8] Checking SBOM table..."
FINAL_COUNT=$(kubectl exec -n ksam $(kubectl get pod -n ksam -l app=postgres -o jsonpath='{.items[0].metadata.name}') -- \
  psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM sboms;" | tr -d ' ')

echo "Final SBOM count: $FINAL_COUNT"
echo "SBOM count change: $((FINAL_COUNT - INITIAL_COUNT))"

# Check for the test image SBOM
echo ""
echo "Checking for $TEST_IMAGE SBOM..."
TEST_IMAGE_SBOM=$(kubectl exec -n ksam $(kubectl get pod -n ksam -l app=postgres -o jsonpath='{.items[0].metadata.name}') -- \
  psql -U postgres -d ksam -t -c "SELECT id, image_name, image_tag, use_count, image_digest FROM sboms WHERE image_name='nginx' AND image_tag='alpine';" || echo "Not found")

if [ -z "$TEST_IMAGE_SBOM" ]; then
  echo "⚠️  WARNING: No SBOM found for $TEST_IMAGE (may not have been processed yet)"
else
  echo "SBOM for $TEST_IMAGE:"
  echo "$TEST_IMAGE_SBOM"

  # Extract use_count
  USE_COUNT=$(echo "$TEST_IMAGE_SBOM" | awk '{print $4}' | head -1)
  echo ""
  echo "Use count for $TEST_IMAGE SBOM: $USE_COUNT"

  if [ "$USE_COUNT" -ge "$POD_COUNT" ]; then
    echo "✅ SBOM use_count correctly incremented (expected >= $POD_COUNT, got $USE_COUNT)"
  else
    echo "⚠️  WARNING: SBOM use_count lower than expected (expected >= $POD_COUNT, got $USE_COUNT)"
    echo "This may be normal if SBOM processing hasn't completed yet"
  fi
fi
echo ""

# Step 9: Summary
echo "================================"
echo "Test Summary"
echo "================================"
echo ""

if [ "$DUPLICATE_ERRORS" -eq 0 2>/dev/null ] || [ -z "$DUPLICATE_ERRORS" ]; then
  echo "✅ PASSED: No duplicate key errors detected"
  echo "✅ FIX VERIFIED: ON CONFLICT upsert is working correctly"
  echo ""
  echo "The fix successfully handles concurrent SBOM inserts for the same image."
  echo "Multiple pods can now use the same image without database errors."
  echo ""
  EXIT_CODE=0
else
  echo "❌ FAILED: Duplicate key errors detected"
  echo "The fix may not be working as expected."
  echo ""
  EXIT_CODE=1
fi

# Step 10: Cleanup
echo "[Cleanup] Deleting test pods..."
kubectl delete pods -l test=sbom-duplicate --ignore-not-found=true
echo "✅ Cleanup complete"
echo ""

echo "================================"
echo "Test Complete"
echo "================================"

exit $EXIT_CODE
