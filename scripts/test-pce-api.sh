#!/bin/bash

# API Testing Script for PCE Phase 2.2
# Tests all new API endpoints

set -e

NAMESPACE="${NAMESPACE:-fortuna}"

echo "=========================================="
echo "PCE API Testing (Phase 2.2)"
echo "=========================================="
echo ""

# Get Core pod
CORE_POD=$(kubectl -n ${NAMESPACE} get pods -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}')
if [ -z "$CORE_POD" ]; then
  echo "❌ Core pod not found"
  exit 1
fi

echo "Core pod: $CORE_POD"
echo ""

# Test 1: GET /api/v1/capability-metadata
echo "Test 1: GET /api/v1/capability-metadata"
echo "----------------------------------------"
RESPONSE=$(kubectl -n ${NAMESPACE} exec ${CORE_POD} -- curl -s -w "\nHTTP_CODE:%{http_code}" http://localhost:8080/api/v1/capability-metadata 2>&1)
HTTP_CODE=$(echo "$RESPONSE" | grep "HTTP_CODE" | cut -d: -f2)
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
RESPONSE=$(kubectl -n ${NAMESPACE} exec ${CORE_POD} -- curl -s -w "\nHTTP_CODE:%{http_code}" http://localhost:8080/api/v1/capability-metadata/ESC_PRIV_POD 2>&1)
HTTP_CODE=$(echo "$RESPONSE" | grep "HTTP_CODE" | cut -d: -f2)
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

# Test 3: GET /api/v1/attack-steps/summary
echo "Test 3: GET /api/v1/attack-steps/summary"
echo "----------------------------------------"
RESPONSE=$(kubectl -n ${NAMESPACE} exec ${CORE_POD} -- curl -s -w "\nHTTP_CODE:%{http_code}" http://localhost:8080/api/v1/attack-steps/summary 2>&1)
HTTP_CODE=$(echo "$RESPONSE" | grep "HTTP_CODE" | cut -d: -f2)
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
DB_POD=$(kubectl -n ${NAMESPACE} get pods -l app=postgres -o jsonpath='{.items[0].metadata.name}')
POD_UID=$(kubectl -n ${NAMESPACE} exec ${DB_POD} -- psql -U postgres -d fortuna -t -c "SELECT pod_uid FROM pod_capabilities LIMIT 1;" 2>&1 | tr -d ' ' | head -1)

if [ -n "$POD_UID" ] && [ "$POD_UID" != "" ]; then
  echo "Found pod UID: $POD_UID"
  echo ""
  echo "Test 4: GET /api/v1/attack-steps/pods/${POD_UID}"
  echo "----------------------------------------"
  RESPONSE=$(kubectl -n ${NAMESPACE} exec ${CORE_POD} -- curl -s -w "\nHTTP_CODE:%{http_code}" "http://localhost:8080/api/v1/attack-steps/pods/${POD_UID}" 2>&1)
  HTTP_CODE=$(echo "$RESPONSE" | grep "HTTP_CODE" | cut -d: -f2)
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
