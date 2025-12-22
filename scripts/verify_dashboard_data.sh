#!/bin/bash
# Verify Dashboard is loading real data from database

set -e

API_URL="http://localhost:8080"
NAMESPACE="ksam"
POSTGRES_POD=$(kubectl get pods -n ${NAMESPACE} -l app=postgres -o jsonpath='{.items[0].metadata.name}')

echo "=========================================="
echo "Dashboard Data Verification"
echo "=========================================="
echo ""

# Get auth token
echo "[1] Getting Auth Token"
echo "-----------------------------------"
LOGIN_RESPONSE=$(curl -s -X POST "${API_URL}/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}')

TOKEN=$(echo "${LOGIN_RESPONSE}" | jq -r '.token // empty')
if [ -n "$TOKEN" ] && [ "$TOKEN" != "null" ]; then
  echo "✅ Token obtained"
  AUTH_HEADER="Authorization: Bearer ${TOKEN}"
else
  echo "⚠️  Auth disabled or failed, testing without token"
  AUTH_HEADER=""
fi
echo ""

# Database counts
echo "[2] Database Counts (Source of Truth)"
echo "-----------------------------------"
DB_PODS=$(kubectl exec -n ${NAMESPACE} "${POSTGRES_POD}" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM pods;" | xargs)
DB_SAS=$(kubectl exec -n ${NAMESPACE} "${POSTGRES_POD}" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM service_accounts;" | xargs)
DB_CLUSTERS=$(kubectl exec -n ${NAMESPACE} "${POSTGRES_POD}" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM clusters;" | xargs)
DB_INSIGHTS=$(kubectl exec -n ${NAMESPACE} "${POSTGRES_POD}" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM insights;" | xargs)

echo "  Pods: ${DB_PODS}"
echo "  ServiceAccounts: ${DB_SAS}"
echo "  Clusters: ${DB_CLUSTERS}"
echo "  Insights: ${DB_INSIGHTS}"
echo ""

# API counts
echo "[3] API Counts (What Dashboard Receives)"
echo "-----------------------------------"
PODS_RESPONSE=$(curl -s -H "${AUTH_HEADER}" "${API_URL}/api/v1/pods?pageSize=1")
API_PODS=$(echo "${PODS_RESPONSE}" | jq -r '.total // 0')
echo "  Pods (from API): ${API_PODS}"

CLUSTERS_RESPONSE=$(curl -s -H "${AUTH_HEADER}" "${API_URL}/api/v1/clusters/stats")
API_CLUSTERS=$(echo "${CLUSTERS_RESPONSE}" | jq -r '.total // (.clusters | length) // 0')
echo "  Clusters (from API): ${API_CLUSTERS}"

INSIGHTS_RESPONSE=$(curl -s -H "${AUTH_HEADER}" "${API_URL}/api/v1/insights/summary")
API_INSIGHTS_TOTAL=$(echo "${INSIGHTS_RESPONSE}" | jq -r '.total // 0')
echo "  Insights Total (from API): ${API_INSIGHTS_TOTAL}"
echo ""

# Compare
echo "[4] Data Comparison"
echo "-----------------------------------"
if [ "$DB_PODS" == "$API_PODS" ]; then
  echo "  ✅ Pods: Database (${DB_PODS}) == API (${API_PODS})"
else
  echo "  ⚠️  Pods: Database (${DB_PODS}) != API (${API_PODS})"
fi

if [ "$DB_CLUSTERS" == "$API_CLUSTERS" ]; then
  echo "  ✅ Clusters: Database (${DB_CLUSTERS}) == API (${API_CLUSTERS})"
else
  echo "  ⚠️  Clusters: Database (${DB_CLUSTERS}) != API (${API_CLUSTERS})"
fi

# Insights comparison (approximate)
if [ "$DB_INSIGHTS" -gt 0 ] && [ "$API_INSIGHTS_TOTAL" -gt 0 ]; then
  DIFF=$((DB_INSIGHTS - API_INSIGHTS_TOTAL))
  if [ ${DIFF#-} -lt 100 ]; then
    echo "  ✅ Insights: Database (${DB_INSIGHTS}) ≈ API (${API_INSIGHTS_TOTAL})"
  else
    echo "  ⚠️  Insights: Database (${DB_INSIGHTS}) != API (${API_INSIGHTS_TOTAL})"
  fi
else
  echo "  ⚠️  Insights: Cannot compare (DB: ${DB_INSIGHTS}, API: ${API_INSIGHTS_TOTAL})"
fi
echo ""

# Dashboard expected display
echo "[5] Expected Dashboard Display"
echo "-----------------------------------"
echo "  Dashboard should show:"
echo "    - Clusters: ${API_CLUSTERS}"
echo "    - Pods: ${API_PODS}"
echo "    - Insights: ${API_INSIGHTS_TOTAL}"
echo ""

# Check for hardcoded values
echo "[6] Checking for Hardcoded Values"
echo "-----------------------------------"
if grep -r "59/100\|30%" KSAM/dashboard/src/pages/Dashboard.tsx 2>/dev/null; then
  echo "  ⚠️  Found potential hardcoded values in Dashboard.tsx"
else
  echo "  ✅ No hardcoded values found"
fi
echo ""

echo "=========================================="
echo "Verification Complete"
echo "=========================================="
echo ""
echo "📊 Summary:"
echo "  - Database Pods: ${DB_PODS}"
echo "  - API Pods: ${API_PODS}"
echo "  - Database Clusters: ${DB_CLUSTERS}"
echo "  - API Clusters: ${API_CLUSTERS}"
echo ""
echo "🌐 Dashboard should display:"
echo "  - Pods: ${API_PODS} (from API)"
echo "  - Clusters: ${API_CLUSTERS} (from API)"
echo "  - Insights: ${API_INSIGHTS_TOTAL} (from API)"
echo ""
