#!/bin/bash

# ============================================================================
# Measure Insight Generation Performance
# ============================================================================
# Measures time taken for insight generation after CVE matching
# ============================================================================

set -euo pipefail

POD_UID="${1:-}"
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-ksam}"
DB_USER="${DB_USER:-ksam}"
RESULTS_DIR="${RESULTS_DIR:-$(dirname $0)/../results}"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

if [ -z "${POD_UID}" ]; then
    echo "Usage: $0 <pod_uid>"
    exit 1
fi

mkdir -p "${RESULTS_DIR}"

db_query() {
    local query=$1
    PGPASSWORD="${DB_PASSWORD:-ksam}" psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" -t -A -c "${query}" 2>/dev/null || echo ""
}

wait_for_insights() {
    local pod_uid=$1
    local max_wait=${2:-300}
    local elapsed=0
    
    while [ $elapsed -lt $max_wait ]; do
        local insight_count=$(db_query "SELECT COUNT(*) FROM insights WHERE resource_uid = '${pod_uid}' AND deleted_at IS NULL;")
        
        if [ "${insight_count}" -gt 0 ]; then
            return 0
        fi
        
        sleep 1
        elapsed=$((elapsed + 1))
    done
    
    return 1
}

echo "Insight Generation Performance Test"
echo "===================================="
echo "Pod UID: ${POD_UID}"
echo ""

# Get CVE match count
CVE_MATCH_COUNT=$(db_query "SELECT COUNT(*) FROM cve_matches cm JOIN sboms s ON cm.sbom_id = s.id WHERE s.pod_uid = '${POD_UID}' AND cm.deleted_at IS NULL AND s.deleted_at IS NULL;")
echo "CVE Matches to process: ${CVE_MATCH_COUNT}"

# Measure insight generation time
echo "Waiting for insight generation..."
START_TIME=$(date +%s.%N)
wait_for_insights "${POD_UID}" 300
END_TIME=$(date +%s.%N)

DURATION=$(echo "$END_TIME - $START_TIME" | bc)

# Get insight results
INSIGHT_COUNT=$(db_query "SELECT COUNT(*) FROM insights WHERE resource_uid = '${POD_UID}' AND deleted_at IS NULL;")
INSIGHT_RATE=$(echo "scale=2; ${INSIGHT_COUNT} / ${DURATION}" | bc)

echo ""
echo "Performance Results"
echo "===================="
echo "Insight Generation Time: ${DURATION} seconds"
echo "CVE Matches Processed: ${CVE_MATCH_COUNT}"
echo "Insights Created: ${INSIGHT_COUNT}"
echo "Generation Rate: ${INSIGHT_RATE} insights/second"

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

# Save results
cat > "${RESULTS_DIR}/insight_generation_performance_${TIMESTAMP}.txt" <<EOF
Insight Generation Performance Test
===================================
Date: $(date)
Pod UID: ${POD_UID}

Results:
- Insight Generation Time: ${DURATION} seconds
- CVE Matches Processed: ${CVE_MATCH_COUNT}
- Insights Created: ${INSIGHT_COUNT}
- Generation Rate: ${INSIGHT_RATE} insights/second

Performance Target: < 2 seconds for 100 CVEs
Status: $([ $(echo "${DURATION} < 2" | bc) -eq 1 ] && echo "PASSED" || echo "FAILED")
EOF

echo ""
echo "Results saved to: ${RESULTS_DIR}/insight_generation_performance_${TIMESTAMP}.txt"

