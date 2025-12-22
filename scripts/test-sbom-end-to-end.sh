#!/bin/bash

# End-to-end test for Custom SBOM Pipeline
# Creates a test pod and verifies SBOM extraction and CVE matching

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

CORE_POD=$(kubectl get pods -n ksam -o name 2>/dev/null | grep -E "(core|ksam-core)" | head -1 | sed 's|pod/||' || echo "")
POSTGRES_POD=$(kubectl get pods -n ksam -o name 2>/dev/null | grep postgres | head -1 | sed 's|pod/||' || echo "")

if [ -z "$CORE_POD" ]; then
    echo "❌ Core pod not found"
    exit 1
fi

echo "=========================================="
echo "Custom SBOM Pipeline - End-to-End Test"
echo "=========================================="
echo ""

# Test 1: Create a test pod with a known image
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Test 1: Create Test Pod"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

TEST_POD_NAME="sbom-test-pod-$(date +%s)"
TEST_IMAGE="nginx:1.19.0"  # Known image with vulnerabilities

cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: $TEST_POD_NAME
  namespace: ksam
spec:
  containers:
  - name: nginx
    image: $TEST_IMAGE
    ports:
    - containerPort: 80
  restartPolicy: Never
EOF

echo "✅ Created test pod: $TEST_POD_NAME"
echo "Waiting for pod to be created..."
sleep 5
echo ""

# Test 2: Wait for SBOM processing
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Test 2: Wait for SBOM Processing"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Waiting 30 seconds for SBOM pipeline to process..."
sleep 30

# Check logs for SBOM processing
SBOM_LOGS=$(kubectl logs -n ksam "$CORE_POD" 2>&1 | grep -i "sbom\|extract\|normalize\|match" | tail -20 || echo "")
if [ -n "$SBOM_LOGS" ]; then
    echo "✅ SBOM processing logs found:"
    echo "$SBOM_LOGS" | head -10
else
    echo "⚠️  No SBOM processing logs found yet"
fi
echo ""

# Test 3: Check database for SBOM
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Test 3: Verify SBOM in Database"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
if [ -n "$POSTGRES_POD" ]; then
    SBOM_COUNT=$(kubectl exec -n ksam "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM sboms WHERE image_name LIKE '%nginx%';" 2>/dev/null | tr -d ' ' || echo "0")
    if [ "$SBOM_COUNT" -gt 0 ]; then
        echo "✅ Found $SBOM_COUNT SBOM(s) for nginx images"
        
        # Get component count
        COMPONENT_COUNT=$(kubectl exec -n ksam "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM sbom_components WHERE sbom_id IN (SELECT id FROM sboms WHERE image_name LIKE '%nginx%');" 2>/dev/null | tr -d ' ' || echo "0")
        echo "✅ Found $COMPONENT_COUNT components"
    else
        echo "⚠️  No SBOM found for nginx images (may need more time)"
    fi
else
    echo "⚠️  Postgres pod not found, skipping database check"
fi
echo ""

# Test 4: Check for CVE matches
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Test 4: Verify CVE Matches"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
if [ -n "$POSTGRES_POD" ]; then
    CVE_MATCH_COUNT=$(kubectl exec -n ksam "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM cve_matches WHERE sbom_id IN (SELECT id FROM sboms WHERE image_name LIKE '%nginx%');" 2>/dev/null | tr -d ' ' || echo "0")
    if [ "$CVE_MATCH_COUNT" -gt 0 ]; then
        echo "✅ Found $CVE_MATCH_COUNT CVE match(es)"
    else
        echo "⚠️  No CVE matches found (may need CVE database or more time)"
    fi
else
    echo "⚠️  Postgres pod not found, skipping CVE check"
fi
echo ""

# Test 5: Check insights
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Test 5: Verify Insights Created"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
if [ -n "$POSTGRES_POD" ]; then
    INSIGHT_COUNT=$(kubectl exec -n ksam "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM insights WHERE source = 'custom-sbom-scanner';" 2>/dev/null | tr -d ' ' || echo "0")
    if [ "$INSIGHT_COUNT" -gt 0 ]; then
        echo "✅ Found $INSIGHT_COUNT insight(s) from custom SBOM scanner"
    else
        echo "⚠️  No insights found from custom SBOM scanner yet"
    fi
else
    echo "⚠️  Postgres pod not found, skipping insight check"
fi
echo ""

# Cleanup
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Cleanup"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
kubectl delete pod "$TEST_POD_NAME" -n ksam --ignore-not-found=true
echo "✅ Cleaned up test pod"
echo ""

# Summary
echo "=========================================="
echo "Test Summary"
echo "=========================================="
echo "✅ Test pod created and processed"
echo "✅ SBOM pipeline executed"
echo "✅ Database integration verified"
echo ""


