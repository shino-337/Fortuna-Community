#!/bin/bash

# ============================================================================
# Compare API Response with Database
# ============================================================================
# Compares API response data with database records to verify consistency
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

echo "Comparing API response with database for pod_uid: ${POD_UID}"
echo "=========================================="

# Get database insights
echo "Fetching database insights..."
DB_INSIGHTS=$(db_query "SELECT id, insight_type, resource_uid, resource_name, resource_namespace, cve_id, severity, cvss, status, detected_at FROM insights WHERE resource_uid = '${POD_UID}' AND deleted_at IS NULL ORDER BY id;")

DB_COUNT=$(echo "${DB_INSIGHTS}" | grep -v "^$" | wc -l | tr -d ' ')
echo "Database insights: ${DB_COUNT}"

# Get API insights
echo "Fetching API insights..."
API_RESPONSE=$(curl -s "${CORE_API}/api/v1/insights?resource_uid=${POD_UID}" || echo "{}")
API_COUNT=$(echo "${API_RESPONSE}" | jq '.data | length' 2>/dev/null || echo "0")
echo "API insights: ${API_COUNT}"

# Compare counts
echo ""
if [ "${DB_COUNT}" -eq "${API_COUNT}" ]; then
    echo "✅ Count match: ${DB_COUNT} insights"
else
    echo "❌ Count mismatch: DB=${DB_COUNT}, API=${API_COUNT}"
fi

# Compare individual fields
echo ""
echo "Field-by-field comparison:"

# Create temporary files for comparison
DB_FILE=$(mktemp)
API_FILE=$(mktemp)

# Format database data
echo "${DB_INSIGHTS}" | while IFS='|' read -r id type uid name namespace cve_id severity cvss status detected_at; do
    echo "${id}|${type}|${uid}|${name}|${namespace}|${cve_id}|${severity}|${cvss}|${status}|${detected_at}" >> "${DB_FILE}"
done

# Format API data
echo "${API_RESPONSE}" | jq -r '.data[] | "\(.id)|\(.insight_type)|\(.resource_uid)|\(.resource_name)|\(.resource_namespace)|\(.cve_id)|\(.severity)|\(.cvss)|\(.status)|\(.detected_at)"' >> "${API_FILE}"

# Compare
DIFF_COUNT=0
while IFS='|' read -r db_id db_type db_uid db_name db_namespace db_cve db_severity db_cvss db_status db_detected; do
    API_MATCH=$(grep "^${db_id}|" "${API_FILE}" || echo "")
    
    if [ -z "${API_MATCH}" ]; then
        echo "❌ Insight ID ${db_id} not found in API"
        DIFF_COUNT=$((DIFF_COUNT + 1))
        continue
    fi
    
    IFS='|' read -r api_id api_type api_uid api_name api_namespace api_cve api_severity api_cvss api_status api_detected <<< "${API_MATCH}"
    
    MISMATCHES=()
    [ "${db_type}" != "${api_type}" ] && MISMATCHES+=("type: DB=${db_type} vs API=${api_type}")
    [ "${db_uid}" != "${api_uid}" ] && MISMATCHES+=("uid: DB=${db_uid} vs API=${api_uid}")
    [ "${db_cve}" != "${api_cve}" ] && MISMATCHES+=("cve_id: DB=${db_cve} vs API=${api_cve}")
    [ "${db_severity}" != "${api_severity}" ] && MISMATCHES+=("severity: DB=${db_severity} vs API=${api_severity}")
    [ "${db_status}" != "${api_status}" ] && MISMATCHES+=("status: DB=${db_status} vs API=${api_status}")
    
    if [ ${#MISMATCHES[@]} -gt 0 ]; then
        echo "❌ Insight ID ${db_id} has mismatches:"
        for mismatch in "${MISMATCHES[@]}"; do
            echo "   - ${mismatch}"
        done
        DIFF_COUNT=$((DIFF_COUNT + 1))
    fi
done < "${DB_FILE}"

# Cleanup
rm -f "${DB_FILE}" "${API_FILE}"

echo ""
if [ ${DIFF_COUNT} -eq 0 ]; then
    echo "✅ All insights match between API and database"
else
    echo "❌ Found ${DIFF_COUNT} insight(s) with mismatches"
    exit 1
fi

