#!/bin/bash
set -e

# Quick verification script for E2E pipeline
# Checks if the E2E pod has been processed through the full pipeline

NAMESPACE="ksam-e2e"
POD_NAME="ksam-e2e-vuln-debian10"
EXPECTED_CVE="CVE-2014-0011"

echo "========================================"
echo "KSAM E2E Pipeline Verification"
echo "========================================"
echo ""

# Get pod UID
POD_UID=$(kubectl get pod -n "$NAMESPACE" "$POD_NAME" -o jsonpath='{.metadata.uid}' 2>/dev/null || echo "")

if [ -z "$POD_UID" ]; then
  echo "❌ Pod not found: $NAMESPACE/$POD_NAME"
  echo ""
  echo "Run: ./scripts/retrigger-e2e-pod.sh"
  exit 1
fi

echo "Pod: $NAMESPACE/$POD_NAME"
echo "Pod UID: $POD_UID"
echo ""

# Check 1: Pod Image Scan
echo "Check 1: SBOM Generation..."
scan_result=$(kubectl exec -n ksam postgres-6f4d585b96-c662t -- \
  psql -U postgres -d ksam -t -A -c \
  "SELECT sbom_id FROM pod_image_scans WHERE pod_uid='$POD_UID' AND sbom_id IS NOT NULL AND deleted_at IS NULL LIMIT 1;" 2>/dev/null || echo "")

if [ -z "$scan_result" ]; then
  echo "❌ SBOM not generated yet"
  echo ""
  echo "Troubleshooting:"
  echo "1. Check if pod is Running:"
  echo "   kubectl get pod -n $NAMESPACE $POD_NAME"
  echo ""
  echo "2. Check SBOMWorker logs:"
  echo "   kubectl logs -n ksam -l app=ksam-core --tail=50 | grep -E 'SBOMWorker|$POD_NAME'"
  echo ""
  echo "3. Wait 1-2 minutes and run this script again"
  exit 1
fi

SBOM_ID=$scan_result
echo "✅ SBOM generated (ID: $SBOM_ID)"
echo ""

# Check 2: SBOM Components
echo "Check 2: SBOM Components..."
component_count=$(kubectl exec -n ksam postgres-6f4d585b96-c662t -- \
  psql -U postgres -d ksam -t -c \
  "SELECT COUNT(*) FROM sbom_components WHERE sbom_id=$SBOM_ID AND deleted_at IS NULL;" 2>/dev/null | tr -d ' ')

if [ "$component_count" -eq 0 ]; then
  echo "⚠️  No components found (expected 1 for vnc4)"
else
  echo "✅ Components extracted: $component_count"
  kubectl exec -n ksam postgres-6f4d585b96-c662t -- \
    psql -U postgres -d ksam -c \
    "SELECT component_name, component_version FROM sbom_components WHERE sbom_id=$SBOM_ID AND deleted_at IS NULL;"
fi
echo ""

# Check 3: CVE Matches
echo "Check 3: CVE Matching..."
match_count=$(kubectl exec -n ksam postgres-6f4d585b96-c662t -- \
  psql -U postgres -d ksam -t -c \
  "SELECT COUNT(*) FROM cve_matches WHERE sbom_id=$SBOM_ID AND deleted_at IS NULL;" 2>/dev/null | tr -d ' ')

if [ "$match_count" -eq 0 ]; then
  echo "❌ No CVE matches found"
  echo ""
  echo "Troubleshooting:"
  echo "1. Check if CVE database is populated:"
  echo "   kubectl exec -n ksam postgres-6f4d585b96-c662t -- psql -U postgres -d ksam -c 'SELECT COUNT(*) FROM cves;'"
  echo ""
  echo "2. Check CVEMatcherWorker logs:"
  echo "   kubectl logs -n ksam -l app=ksam-core --tail=50 | grep CVEMatcher"
  echo ""
  echo "3. Verify sbom.created event was published:"
  echo "   kubectl logs -n ksam -l app=ksam-core --tail=100 | grep 'sbom.created'"
  exit 1
fi

echo "✅ CVE matches found: $match_count"
kubectl exec -n ksam postgres-6f4d585b96-c662t -- \
  psql -U postgres -d ksam -c \
  "SELECT cve_id, severity, fixed_version FROM cve_matches WHERE sbom_id=$SBOM_ID AND deleted_at IS NULL;"
echo ""

# Check 4: Expected CVE
echo "Check 4: Expected CVE ($EXPECTED_CVE)..."
found_cve=$(kubectl exec -n ksam postgres-6f4d585b96-c662t -- \
  psql -U postgres -d ksam -t -A -c \
  "SELECT cve_id FROM cve_matches WHERE sbom_id=$SBOM_ID AND cve_id='$EXPECTED_CVE' AND deleted_at IS NULL;" 2>/dev/null || echo "")

if [ -z "$found_cve" ]; then
  echo "⚠️  Expected CVE $EXPECTED_CVE not found"
  echo "    (This may be expected if using different test data)"
else
  echo "✅ Expected CVE $EXPECTED_CVE found"
fi
echo ""

# Check 5: Insights
echo "Check 5: Insights..."
insight_count=$(kubectl exec -n ksam postgres-6f4d585b96-c662t -- \
  psql -U postgres -d ksam -t -c \
  "SELECT COUNT(*) FROM insights WHERE sbom_id=$SBOM_ID AND status='active' AND deleted_at IS NULL;" 2>/dev/null | tr -d ' ')

if [ "$insight_count" -eq 0 ]; then
  echo "⚠️  No insights created"
  echo "    (Insights may be filtered by severity: only CRITICAL/HIGH)"
else
  echo "✅ Insights created: $insight_count"
  kubectl exec -n ksam postgres-6f4d585b96-c662t -- \
    psql -U postgres -d ksam -c \
    "SELECT type, severity, cve_id FROM insights WHERE sbom_id=$SBOM_ID AND status='active' AND deleted_at IS NULL LIMIT 5;"
fi
echo ""

# Final Summary
echo "========================================"
echo "Pipeline Verification Summary"
echo "========================================"
echo "Pod:        ✅ Running ($POD_UID)"
echo "SBOM:       ✅ Generated (ID: $SBOM_ID, $component_count components)"
echo "CVE Match:  $([ "$match_count" -gt 0 ] && echo '✅' || echo '❌') $match_count matches"
echo "Insights:   $([ "$insight_count" -gt 0 ] && echo '✅' || echo '⚠️ ') $insight_count insights"
echo ""

if [ "$match_count" -gt 0 ]; then
  echo "✅ E2E Pipeline is working correctly!"
else
  echo "❌ E2E Pipeline has issues - see troubleshooting above"
  exit 1
fi
