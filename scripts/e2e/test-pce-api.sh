#!/usr/bin/env bash

# API Testing Script for PCE Phase 2.2
# Tests all new API endpoints

set -euo pipefail

NAMESPACE="${NAMESPACE:-fortuna}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./common.sh
source "${SCRIPT_DIR}/common.sh"

echo "=========================================="
echo "PCE API Testing (Phase 2.2)"
echo "=========================================="
echo ""

CORE_POD="$(require_core_pod)"
TOKEN="$(require_jwt_token "$CORE_POD")"

echo "Core pod: $CORE_POD"
echo ""

# Test 1: GET /api/v1/capability-metadata
echo "Test 1: GET /api/v1/capability-metadata"
echo "----------------------------------------"
RESPONSE=$(kubectl -n "${NAMESPACE}" exec "${CORE_POD}" -- curl -s -w "\nHTTP_CODE:%{http_code}" \
  -H "Authorization: Bearer ${TOKEN}" \
  http://localhost:8080/api/v1/capability-metadata 2>&1)
HTTP_CODE=$(echo "$RESPONSE" | awk -F: '/HTTP_CODE/{print $2}')
BODY=$(echo "$RESPONSE" | sed '/HTTP_CODE/d')

if [ "$HTTP_CODE" = "200" ]; then
  COUNT=$(echo "$BODY" | grep -o '"count":[0-9]*' | cut -d: -f2)
  echo "✅ Status: $HTTP_CODE"
  echo "✅ Count: $COUNT"
  if [ "$COUNT" -ge 11 ]; then
    echo "✅ Expected at least 11 metadata records"
  else
    echo "⚠️  Expected 11 records, got $COUNT"
  fi
else
  echo "❌ Status: $HTTP_CODE"
  echo "Response: $BODY"
fi
echo ""

# Test 2: GET /api/v1/capability-metadata/:capabilityId
echo "Test 2: GET /api/v1/capability-metadata/ESC_PRIV_POD"
echo "----------------------------------------"
RESPONSE=$(kubectl -n "${NAMESPACE}" exec "${CORE_POD}" -- curl -s -w "\nHTTP_CODE:%{http_code}" \
  -H "Authorization: Bearer ${TOKEN}" \
  http://localhost:8080/api/v1/capability-metadata/ESC_PRIV_POD 2>&1)
HTTP_CODE=$(echo "$RESPONSE" | awk -F: '/HTTP_CODE/{print $2}')
BODY=$(echo "$RESPONSE" | sed '/HTTP_CODE/d')

if [ "$HTTP_CODE" = "200" ]; then
  CAP_ID=$(echo "$BODY" | grep -o '"capabilityId":"[^"]*"' | cut -d: -f2 | tr -d '"')
  echo "✅ Status: $HTTP_CODE"
  echo "✅ Capability ID: $CAP_ID"
  if [ "$CAP_ID" = "ESC_PRIV_POD" ]; then
    echo "✅ Correct capability ID returned"
  fi
else
  echo "❌ Status: $HTTP_CODE"
  echo "Response: $BODY"
fi
echo ""

# Test 3: GET /api/v1/risk/attack-steps/summary
echo "Test 3: GET /api/v1/risk/attack-steps/summary"
echo "----------------------------------------"
RESPONSE=$(kubectl -n "${NAMESPACE}" exec "${CORE_POD}" -- curl -s -w "\nHTTP_CODE:%{http_code}" \
  -H "Authorization: Bearer ${TOKEN}" \
  http://localhost:8080/api/v1/risk/attack-steps/summary 2>&1)
HTTP_CODE=$(echo "$RESPONSE" | awk -F: '/HTTP_CODE/{print $2}')
BODY=$(echo "$RESPONSE" | sed '/HTTP_CODE/d')

if [ "$HTTP_CODE" = "200" ]; then
  COUNT=$(echo "$BODY" | grep -o '"count":[0-9]*' | cut -d: -f2)
  echo "✅ Status: $HTTP_CODE"
  echo "✅ Summary count: $COUNT"
else
  echo "❌ Status: $HTTP_CODE"
  echo "Response: $BODY"
fi
echo ""

# Test 4: Get a pod UID for attack steps test
echo "Test 4: Finding pod with capabilities..."
DB_POD="$(require_postgres_pod)"
POD_UID=$(kubectl -n "${NAMESPACE}" exec "${DB_POD}" -- psql -U postgres -d fortuna -t -A -c \
  "SELECT pod_uid FROM pod_capabilities ORDER BY updated_at DESC NULLS LAST, created_at DESC LIMIT 1;" \
  | tr -d ' ' | head -1)

if [ -n "$POD_UID" ] && [ "$POD_UID" != "" ]; then
  echo "Found pod UID: $POD_UID"
  echo ""
  echo "Test 4: GET /api/v1/risk/pods/${POD_UID}/attack-steps"
  echo "----------------------------------------"
  RESPONSE=$(kubectl -n "${NAMESPACE}" exec "${CORE_POD}" -- curl -s -w "\nHTTP_CODE:%{http_code}" \
    -H "Authorization: Bearer ${TOKEN}" \
    "http://localhost:8080/api/v1/risk/pods/${POD_UID}/attack-steps" 2>&1)
  HTTP_CODE=$(echo "$RESPONSE" | awk -F: '/HTTP_CODE/{print $2}')
  BODY=$(echo "$RESPONSE" | sed '/HTTP_CODE/d')

  if [ "$HTTP_CODE" = "200" ]; then
    COUNT=$(echo "$BODY" | grep -o '"count":[0-9]*' | cut -d: -f2)
    echo "✅ Status: $HTTP_CODE"
    echo "✅ Attack steps count: $COUNT"
  else
    echo "❌ Status: $HTTP_CODE"
    echo "Response: $BODY"
  fi
else
  echo "⚠️  No pods found with capabilities, skipping attack steps test"
fi
echo ""

echo "=========================================="
echo "API Testing Complete"
echo "=========================================="
