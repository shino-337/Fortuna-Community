#!/bin/bash

# ============================================================================
# Verify Insights Script
# ============================================================================
# Verifies insight generation and storage in database and API
# ============================================================================

set -euo pipefail

POD_UID="${1:-}"
CORE_API="${CORE_API_URL:-http://localhost:8080}"
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-ksam}"
DB_USER="${DB_USER:-ksam}"

if [ -z "${POD_UID}" ]; then
    echo "Usage: $0 <pod_uid>"
    exit 1
fi

db_query() {
    local query=$1
    PGPASSWORD="${DB_PASSWORD:-ksam}" psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" -t -A -c "${query}" 2>/dev/null || echo ""
}

echo "Verifying insights for pod_uid: ${POD_UID}"

# Check insights in database
INSIGHT_COUNT=$(db_query "SELECT COUNT(*) FROM insights WHERE resource_uid = '${POD_UID}' AND deleted_at IS NULL;")

if [ "${INSIGHT_COUNT}" -eq 0 ]; then
    echo "⚠️  No insights found (this may be normal if no vulnerabilities exist)"
    exit 0
fi

echo "✅ Insights found in database: ${INSIGHT_COUNT}"

# Get insights by type and severity
echo ""
echo "Insights by Type:"
db_query "SELECT insight_type, COUNT(*) FROM insights WHERE resource_uid = '${POD_UID}' AND deleted_at IS NULL GROUP BY insight_type;" | while IFS='|' read -r type count; do
    echo "  ${type}: ${count}"
done

echo ""
echo "Insights by Severity:"
db_query "SELECT severity, COUNT(*) FROM insights WHERE resource_uid = '${POD_UID}' AND deleted_at IS NULL GROUP BY severity ORDER BY severity;" | while IFS='|' read -r severity count; do
    echo "  ${severity}: ${count}"
done

# Display sample insights
echo ""
echo "Sample Insights (first 10):"
db_query "SELECT id, insight_type, cve_id, severity, status, detected_at FROM insights WHERE resource_uid = '${POD_UID}' AND deleted_at IS NULL ORDER BY severity DESC, detected_at DESC LIMIT 10;" | while IFS='|' read -r id type cve_id severity status detected_at; do
    echo "  - ID: ${id} | Type: ${type} | CVE: ${cve_id} | Severity: ${severity} | Status: ${status} | Detected: ${detected_at}"
done

# Verify API
echo ""
echo "Verifying API response..."
API_RESPONSE=$(curl -s "${CORE_API}/api/v1/insights?resource_uid=${POD_UID}" || echo "{}")
API_COUNT=$(echo "${API_RESPONSE}" | jq '.data | length' 2>/dev/null || echo "0")

echo "API returned: ${API_COUNT} insights"

if [ "${INSIGHT_COUNT}" -eq "${API_COUNT}" ]; then
    echo "✅ Insight counts match: DB=${INSIGHT_COUNT}, API=${API_COUNT}"
else
    echo "❌ Insight count mismatch: DB=${INSIGHT_COUNT}, API=${API_COUNT}"
fi

echo ""
echo "✅ Insight verification complete"

