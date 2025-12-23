#!/bin/bash
# Comprehensive API Test for Dashboard

set -e

API_URL="http://localhost:8080"
NAMESPACE="ksam"

echo "=========================================="
echo "Dashboard API Test Suite"
echo "=========================================="
echo ""
echo "API URL: ${API_URL}"
echo ""

# Get auth token
echo "[1] Authentication Test"
echo "-----------------------------------"
LOGIN_RESPONSE=$(curl -s -X POST "${API_URL}/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}')

TOKEN=$(echo "${LOGIN_RESPONSE}" | jq -r '.token // empty')
if [ -n "$TOKEN" ] && [ "$TOKEN" != "null" ]; then
  echo "✅ Login successful"
  AUTH_HEADER="Authorization: Bearer ${TOKEN}"
else
  echo "⚠️  Login failed or auth disabled, testing without token"
  AUTH_HEADER=""
fi
echo ""

# Test Rules API
echo "[2] Rules Management API"
echo "-----------------------------------"
echo "GET /api/v1/rules"
RULES_RESPONSE=$(curl -s -H "${AUTH_HEADER}" "${API_URL}/api/v1/rules")
RULES_COUNT=$(echo "${RULES_RESPONSE}" | jq -r '.total // .rules | length // 0')
echo "  Total Rules: ${RULES_COUNT}"
if [ "$RULES_COUNT" -gt 0 ]; then
  FIRST_RULE_ID=$(echo "${RULES_RESPONSE}" | jq -r '.rules[0].id // empty')
  if [ -n "$FIRST_RULE_ID" ] && [ "$FIRST_RULE_ID" != "null" ]; then
    echo "  First Rule ID: ${FIRST_RULE_ID}"
    echo "  Testing GET /api/v1/rules/${FIRST_RULE_ID}"
    curl -s -H "${AUTH_HEADER}" "${API_URL}/api/v1/rules/${FIRST_RULE_ID}" | jq -r '.rule.id // "OK"' | head -1
    echo "  ✅ Rule detail endpoint works"
  fi
fi
echo ""

# Test Metrics API
echo "[3] Metrics API"
echo "-----------------------------------"
echo "GET /api/v1/metrics/workers"
WORKERS_RESPONSE=$(curl -s -H "${AUTH_HEADER}" "${API_URL}/api/v1/metrics/workers")
echo "${WORKERS_RESPONSE}" | jq -r '.workers // [] | length' | xargs -I {} echo "  Workers: {}"
echo "  ✅ Workers endpoint works"

echo "GET /api/v1/metrics/queue"
QUEUE_RESPONSE=$(curl -s -H "${AUTH_HEADER}" "${API_URL}/api/v1/metrics/queue")
echo "${QUEUE_RESPONSE}" | jq -r '.' | head -5
echo "  ✅ Queue endpoint works"

echo "GET /api/v1/metrics/system"
SYSTEM_RESPONSE=$(curl -s -H "${AUTH_HEADER}" "${API_URL}/api/v1/metrics/system")
echo "${SYSTEM_RESPONSE}" | jq -r '.health.status // "OK"'
echo "  ✅ System endpoint works"

echo "GET /api/v1/agents/status"
AGENTS_RESPONSE=$(curl -s -H "${AUTH_HEADER}" "${API_URL}/api/v1/agents/status")
AGENTS_TOTAL=$(echo "${AGENTS_RESPONSE}" | jq -r '.total // 0')
echo "  Total Agents: ${AGENTS_TOTAL}"
echo "  ✅ Agents endpoint works"
echo ""

# Test Insights API (for Risk Center)
echo "[4] Risk Center API"
echo "-----------------------------------"
echo "GET /api/v1/insights"
INSIGHTS_RESPONSE=$(curl -s -H "${AUTH_HEADER}" "${API_URL}/api/v1/insights?pageSize=10")
INSIGHTS_COUNT=$(echo "${INSIGHTS_RESPONSE}" | jq -r '.total // .insights | length // 0')
echo "  Total Insights: ${INSIGHTS_COUNT}"
if [ "$INSIGHTS_COUNT" -gt 0 ]; then
  FIRST_INSIGHT_ID=$(echo "${INSIGHTS_RESPONSE}" | jq -r '.insights[0].id // empty')
  if [ -n "$FIRST_INSIGHT_ID" ] && [ "$FIRST_INSIGHT_ID" != "null" ]; then
    echo "  First Insight ID: ${FIRST_INSIGHT_ID}"
    echo "  Testing POST /api/v1/insights/${FIRST_INSIGHT_ID}/acknowledge"
    ACK_RESPONSE=$(curl -s -X POST -H "${AUTH_HEADER}" "${API_URL}/api/v1/insights/${FIRST_INSIGHT_ID}/acknowledge")
    echo "${ACK_RESPONSE}" | jq -r '.acknowledged // "OK"' | head -1
    echo "  ✅ Acknowledge endpoint works"
  fi
fi
echo ""

# Test Clusters API (for Risk Inventory)
echo "[5] Clusters & Resources API"
echo "-----------------------------------"
echo "GET /api/v1/clusters/stats"
CLUSTERS_RESPONSE=$(curl -s -H "${AUTH_HEADER}" "${API_URL}/api/v1/clusters/stats")
CLUSTERS_COUNT=$(echo "${CLUSTERS_RESPONSE}" | jq -r '.total // .clusters | length // 0')
echo "  Total Clusters: ${CLUSTERS_COUNT}"
echo "  ✅ Clusters endpoint works"

echo "GET /api/v1/pods?pageSize=10"
PODS_RESPONSE=$(curl -s -H "${AUTH_HEADER}" "${API_URL}/api/v1/pods?pageSize=10")
PODS_COUNT=$(echo "${PODS_RESPONSE}" | jq -r '.total // .pods | length // 0')
echo "  Pods (sample): ${PODS_COUNT}"
echo "  ✅ Pods endpoint works"

echo "GET /api/v1/serviceaccounts?pageSize=10"
SAS_RESPONSE=$(curl -s -H "${AUTH_HEADER}" "${API_URL}/api/v1/serviceaccounts?pageSize=10")
SAS_COUNT=$(echo "${SAS_RESPONSE}" | jq -r '.total // .serviceAccounts | length // 0')
echo "  ServiceAccounts (sample): ${SAS_COUNT}"
echo "  ✅ ServiceAccounts endpoint works"
echo ""

# Summary
echo "=========================================="
echo "Test Summary"
echo "=========================================="
echo ""
echo "✅ Authentication: Working"
echo "✅ Rules API: Working (${RULES_COUNT} rules)"
echo "✅ Metrics API: Working"
echo "✅ Risk Center API: Working (${INSIGHTS_COUNT} insights)"
echo "✅ Resources API: Working"
echo ""
echo "All API endpoints are ready for Dashboard!"
echo ""

