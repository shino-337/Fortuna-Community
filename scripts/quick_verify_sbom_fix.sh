#!/bin/bash
#
# Quick SBOM Duplicate Key Fix Verification
# ==========================================
# Fast verification test (< 2 minutes)
#

set -e

echo "============================================================"
echo "SBOM Duplicate Key Fix - Quick Verification"
echo "============================================================"
echo ""

POSTGRES_POD=$(kubectl get pod -n ksam -l app=postgres -o jsonpath='{.items[0].metadata.name}')
echo "PostgreSQL Pod: $POSTGRES_POD"
echo ""

# ============================================================
# TEST 1: Database Schema
# ============================================================
echo "[TEST 1] Database Schema Verification"
echo "------------------------------------------------------------"

# Check sboms table
TABLE_COUNT=$(kubectl exec -n ksam $POSTGRES_POD -- psql -U postgres -d ksam -t -c \
  "SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'sboms';" 2>/dev/null | tr -d ' \n')

if [ "$TABLE_COUNT" = "1" ]; then
    echo "✅ sboms table exists"
else
    echo "❌ sboms table missing"
    exit 1
fi

# Check unique constraint
CONSTRAINT_COUNT=$(kubectl exec -n ksam $POSTGRES_POD -- psql -U postgres -d ksam -t -c \
  "SELECT COUNT(*) FROM pg_constraint WHERE conname = 'sboms_image_digest_key';" 2>/dev/null | tr -d ' \n')

if [ "$CONSTRAINT_COUNT" = "1" ]; then
    echo "✅ Unique constraint 'sboms_image_digest_key' exists"
else
    echo "❌ Unique constraint not found"
    exit 1
fi

# Show table structure
echo ""
echo "SBOM Table Structure:"
kubectl exec -n ksam $POSTGRES_POD -- psql -U postgres -d ksam -c "\\d sboms" | head -25

# ============================================================
# TEST 2: Current SBOM Data
# ============================================================
echo ""
echo "[TEST 2] Current SBOM Data"
echo "------------------------------------------------------------"

SBOM_COUNT=$(kubectl exec -n ksam $POSTGRES_POD -- psql -U postgres -d ksam -t -c \
  "SELECT COUNT(*) FROM sboms;" 2>/dev/null | tr -d ' \n')

echo "Total SBOMs in database: $SBOM_COUNT"
echo ""

# Show recent SBOMs
echo "Recent SBOMs (top 5 by use_count):"
kubectl exec -n ksam $POSTGRES_POD -- psql -U postgres -d ksam -c \
  "SELECT id, image_name, image_tag, LEFT(image_digest, 20) as digest, use_count, updated_at
   FROM sboms
   WHERE deleted_at IS NULL
   ORDER BY use_count DESC, updated_at DESC
   LIMIT 5;"

# ============================================================
# TEST 3: Check for Duplicate image_digest
# ============================================================
echo ""
echo "[TEST 3] Duplicate image_digest Check"
echo "------------------------------------------------------------"

DUPLICATES=$(kubectl exec -n ksam $POSTGRES_POD -- psql -U postgres -d ksam -t -c \
  "SELECT COUNT(*) FROM (
     SELECT image_digest
     FROM sboms
     WHERE deleted_at IS NULL AND image_digest != ''
     GROUP BY image_digest
     HAVING COUNT(*) > 1
   ) dup;" 2>/dev/null | tr -d ' \n')

if [ "$DUPLICATES" = "0" ]; then
    echo "✅ No duplicate image_digest entries found"
    echo "   (Unique constraint is working correctly)"
else
    echo "❌ Found $DUPLICATES duplicate image_digest entries:"
    kubectl exec -n ksam $POSTGRES_POD -- psql -U postgres -d ksam -c \
      "SELECT image_digest, COUNT(*) as count
       FROM sboms
       WHERE deleted_at IS NULL
       GROUP BY image_digest
       HAVING COUNT(*) > 1;"
fi

# ============================================================
# TEST 4: Recent Error Logs
# ============================================================
echo ""
echo "[TEST 4] Check for Duplicate Key Errors in Logs"
echo "------------------------------------------------------------"

# Check last 10 minutes of logs
ERRORS=$(kubectl logs -n ksam -l app=ksam-core --since=10m 2>/dev/null | \
  grep -c "duplicate key.*sboms_image_digest_key\|23505.*sbom\|failed to upsert SBOM" || echo "0")

if [ "$ERRORS" = "0" ]; then
    echo "✅ No duplicate key errors found in logs (last 10 minutes)"
else
    echo "❌ Found $ERRORS duplicate key error(s) in logs:"
    echo ""
    kubectl logs -n ksam -l app=ksam-core --since=10m 2>/dev/null | \
      grep -A 3 "duplicate key\|23505\|failed to upsert" | head -20
fi

# ============================================================
# TEST 5: SBOM Processing Logs
# ============================================================
echo ""
echo "[TEST 5] SBOM Processing Activity"
echo "------------------------------------------------------------"

# Look for SBOM processing logs
SBOM_PROCESSING=$(kubectl logs -n ksam -l app=ksam-core --since=10m 2>/dev/null | \
  grep -c "Created new SBOM\|Updated existing SBOM\|ON CONFLICT" || echo "0")

echo "SBOM processing operations (last 10 minutes): $SBOM_PROCESSING"

if [ "$SBOM_PROCESSING" -gt 0 ]; then
    echo ""
    echo "Sample SBOM operation logs:"
    kubectl logs -n ksam -l app=ksam-core --since=10m 2>/dev/null | \
      grep "Created new SBOM\|Updated existing SBOM.*ON CONFLICT" | head -5
fi

# ============================================================
# TEST 6: Quick Concurrent Test
# ============================================================
echo ""
echo "[TEST 6] Quick Concurrent Pod Test (3 pods, same image)"
echo "------------------------------------------------------------"

TEST_IMAGE="nginx:alpine"
echo "Test image: $TEST_IMAGE"
echo ""

# Clean up any existing test pods
kubectl delete pods -l test=quick-verify --ignore-not-found=true 2>/dev/null
sleep 2

# Record before state
BEFORE_COUNT=$(kubectl exec -n ksam $POSTGRES_POD -- psql -U postgres -d ksam -t -c \
  "SELECT COUNT(*) FROM sboms;" 2>/dev/null | tr -d ' \n')

echo "Creating 3 pods concurrently with same image..."

# Create 3 pods at the same time
kubectl run test-quick-1 --image=$TEST_IMAGE --restart=Never --labels="test=quick-verify" --command -- sleep 120 &
kubectl run test-quick-2 --image=$TEST_IMAGE --restart=Never --labels="test=quick-verify" --command -- sleep 120 &
kubectl run test-quick-3 --image=$TEST_IMAGE --restart=Never --labels="test=quick-verify" --command -- sleep 120 &
wait

echo "✅ 3 pods created"
echo ""

echo "Waiting 30 seconds for SBOM processing..."
sleep 30

# Check for errors during this test
TEST_ERRORS=$(kubectl logs -n ksam -l app=ksam-core --since=1m 2>/dev/null | \
  grep -c "duplicate key.*sboms\|23505.*sbom" || echo "0")

if [ "$TEST_ERRORS" = "0" ]; then
    echo "✅ No duplicate key errors during concurrent pod creation"
else
    echo "❌ Found $TEST_ERRORS duplicate key error(s) during test"
fi

# Check final count
AFTER_COUNT=$(kubectl exec -n ksam $POSTGRES_POD -- psql -U postgres -d ksam -t -c \
  "SELECT COUNT(*) FROM sboms;" 2>/dev/null | tr -d ' \n')

echo "SBOM count: $BEFORE_COUNT → $AFTER_COUNT (change: $((AFTER_COUNT - BEFORE_COUNT)))"

# Cleanup test pods
echo "Cleaning up test pods..."
kubectl delete pods -l test=quick-verify --ignore-not-found=true 2>/dev/null

# ============================================================
# Summary
# ============================================================
echo ""
echo "============================================================"
echo "Verification Summary"
echo "============================================================"
echo ""

if [ "$ERRORS" = "0" ] && [ "$TEST_ERRORS" = "0" ] && [ "$DUPLICATES" = "0" ]; then
    echo "✅ ALL CHECKS PASSED"
    echo ""
    echo "Key Findings:"
    echo "  ✅ Database schema correct (unique constraint present)"
    echo "  ✅ No duplicate image_digest entries"
    echo "  ✅ No duplicate key errors in logs"
    echo "  ✅ Concurrent pod creation handled correctly"
    echo ""
    echo "The SBOM duplicate key fix is working correctly!"
    exit 0
else
    echo "⚠️  SOME ISSUES FOUND"
    echo ""
    echo "Issues:"
    [ "$DUPLICATES" != "0" ] && echo "  ❌ Found $DUPLICATES duplicate image_digest entries"
    [ "$ERRORS" != "0" ] && echo "  ❌ Found $ERRORS duplicate key errors in logs"
    [ "$TEST_ERRORS" != "0" ] && echo "  ❌ Found $TEST_ERRORS errors during concurrent test"
    echo ""
    echo "Please review the test results above."
    exit 1
fi
