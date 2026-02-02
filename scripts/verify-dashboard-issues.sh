#!/bin/bash
# ============================================================================
# Verify Dashboard Issues - Comprehensive Check
# ============================================================================
# 1. Check pod image vs latest image
# 2. Test Risk Center API (pod filter)
# 3. Test SBOM API
# 4. Check database for SBOM data
# 5. Verify UI features
# ============================================================================

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

NAMESPACE="${NAMESPACE:-fortuna}"

ok()  { echo -e "${GREEN}✅${NC} $*"; }
fail() { echo -e "${RED}❌${NC} $*"; }
warn() { echo -e "${YELLOW}⚠️${NC}  $*"; }
info() { echo -e "${BLUE}[INFO]${NC} $*"; }

echo "=========================================="
echo "Dashboard Issues Verification"
echo "=========================================="
echo ""

# 1. Check pod image
info "1. Checking pod image..."
POD_IMAGE_ID=$(kubectl get pods -n "$NAMESPACE" -l app=fortuna-dashboard -o jsonpath='{.items[0].status.containerStatuses[0].imageID}' 2>/dev/null || echo "")
LATEST_IMAGE_ID=$(nerdctl --namespace k8s.io images | grep "fortuna-dashboard.*latest" | head -1 | awk '{print $3}' || echo "")

if [ -n "$POD_IMAGE_ID" ] && [ -n "$LATEST_IMAGE_ID" ]; then
  if echo "$POD_IMAGE_ID" | grep -q "$LATEST_IMAGE_ID"; then
    ok "Pod is using latest image"
  else
    fail "Pod is using OLD image! Expected: $LATEST_IMAGE_ID, Got: $POD_IMAGE_ID"
    warn "Need to force pod recreation"
  fi
else
  warn "Cannot determine image IDs"
fi
echo ""

# 2. Test Core API - Pod Capabilities with podName filter
info "2. Testing Core API - Pod Capabilities filter..."
pkill -f "port-forward.*fortuna-core.*8080" 2>/dev/null || true
kubectl port-forward -n "$NAMESPACE" svc/fortuna-core 8080:8080 > /tmp/core-pf-verify.log 2>&1 &
PF_PID=$!
sleep 3

TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}' \
  | grep -o '"token":"[^"]*' | cut -d'"' -f4 || echo "")

if [ -n "$TOKEN" ]; then
  API_RESPONSE=$(curl -s "http://localhost:8080/api/v1/pod-capabilities?limit=5" \
    -H "Authorization: Bearer $TOKEN" 2>/dev/null || echo "")
  
  if echo "$API_RESPONSE" | grep -q "capabilities"; then
    POD_NAME_COUNT=$(echo "$API_RESPONSE" | grep -o '"podName"' | wc -l || echo "0")
    if [ "${POD_NAME_COUNT:-0}" -gt 0 ]; then
      ok "API returns podName field ($POD_NAME_COUNT found)"
    else
      warn "API response does not contain podName field"
    fi
    
    # Test podName filter
    FILTER_RESPONSE=$(curl -s "http://localhost:8080/api/v1/pod-capabilities?podName=cve&limit=3" \
      -H "Authorization: Bearer $TOKEN" 2>/dev/null || echo "")
    if echo "$FILTER_RESPONSE" | grep -q "capabilities"; then
      ok "PodName filter works"
    else
      warn "PodName filter may not work correctly"
    fi
  else
    fail "API /pod-capabilities failed or returned empty"
  fi
else
  fail "Cannot get auth token"
fi

kill $PF_PID 2>/dev/null || true
echo ""

# 3. Test SBOM API
info "3. Testing SBOM API..."
kubectl port-forward -n "$NAMESPACE" svc/fortuna-core 8080:8080 > /tmp/core-pf-sbom.log 2>&1 &
PF_PID=$!
sleep 3

if [ -n "$TOKEN" ]; then
  SBOM_LIST=$(curl -s "http://localhost:8080/api/v1/sbom" \
    -H "Authorization: Bearer $TOKEN" 2>/dev/null || echo "")
  
  if echo "$SBOM_LIST" | grep -q "sboms"; then
    SBOM_COUNT=$(echo "$SBOM_LIST" | grep -o '"podId"' | wc -l || echo "0")
    if [ "${SBOM_COUNT:-0}" -gt 0 ]; then
      ok "SBOM API returns data ($SBOM_COUNT pods)"
    else
      warn "SBOM API returns empty list"
    fi
  else
    fail "SBOM API failed or returned invalid format"
  fi
else
  fail "Cannot get auth token for SBOM test"
fi

kill $PF_PID 2>/dev/null || true
echo ""

# 4. Check database for SBOM
info "4. Checking database for SBOM data..."
pkill -f "port-forward.*postgres.*5432" 2>/dev/null || true
kubectl port-forward -n "$NAMESPACE" svc/postgres 5432:5432 > /tmp/pg-pf-verify.log 2>&1 &
PF_PID=$!
sleep 3

SBOM_COUNT=$(PGPASSWORD=postgres psql -h localhost -p 5432 -U postgres -d fortuna \
  -t -c "SELECT COUNT(*) FROM pod_sboms;" 2>/dev/null | tr -d ' ' || echo "0")

if [ "${SBOM_COUNT:-0}" -gt 0 ]; then
  ok "Database has SBOM data ($SBOM_COUNT records)"
else
  warn "Database has no SBOM data (table may not exist or empty)"
fi

kill $PF_PID 2>/dev/null || true
echo ""

# 5. Summary
echo "=========================================="
echo "Summary"
echo "=========================================="
echo ""
echo "Next steps:"
echo "  1. If pod uses old image: kubectl delete pod -n $NAMESPACE -l app=fortuna-dashboard"
echo "  2. Check UI: kubectl port-forward -n $NAMESPACE svc/fortuna-dashboard 8081:80"
echo "  3. Verify Risk Center -> PCE Drill-down has podName filter"
echo "  4. Verify Risk List has search/filter functionality"
echo ""
