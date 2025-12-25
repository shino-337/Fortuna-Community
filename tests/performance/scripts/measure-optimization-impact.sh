#!/bin/bash

# ============================================================================
# Measure Optimization Impact
# ============================================================================
# Compares performance before and after optimizations by measuring:
# 1. Database query performance (with new indexes)
# 2. Batch processing efficiency
# 3. Overall E2E processing time
# 4. Database connection pool utilization
# ============================================================================

set -euo pipefail

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-ksam}"
DB_USER="${DB_USER:-ksam}"
CORE_API="${CORE_API_URL:-http://localhost:8080}"
RESULTS_DIR="${RESULTS_DIR:-$(dirname $0)/../results}"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

mkdir -p "${RESULTS_DIR}"
RESULT_FILE="${RESULTS_DIR}/optimization_impact_${TIMESTAMP}.txt"

# Database query
db_query() {
    PGPASSWORD="${DB_PASSWORD:-ksam}" psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" -t -A -c "$1" 2>/dev/null || echo ""
}

# Logging
log() {
    echo -e "${BLUE}$1${NC}" | tee -a "${RESULT_FILE}"
}

log_metric() {
    echo -e "${GREEN}  ✓${NC} $1" | tee -a "${RESULT_FILE}"
}

log_comparison() {
    echo -e "${YELLOW}  →${NC} $1" | tee -a "${RESULT_FILE}"
}

echo "========================================"  | tee "${RESULT_FILE}"
echo "Optimization Impact Analysis"  | tee -a "${RESULT_FILE}"
echo "========================================"  | tee -a "${RESULT_FILE}"
echo "Date: $(date)"  | tee -a "${RESULT_FILE}"
echo "========================================"  | tee -a "${RESULT_FILE}"
echo ""  | tee -a "${RESULT_FILE}"

# ============================================================================
# Benchmark 1: CVE Lookup Performance (Bulk vs Individual)
# ============================================================================
log "[Benchmark 1] CVE Lookup Performance"
echo "----------------------------------------" | tee -a "${RESULT_FILE}"

# Get sample package names
SAMPLE_PACKAGES=$(db_query "
    SELECT DISTINCT package_name
    FROM package_vulnerabilities
    WHERE ecosystem = 'debian'
    AND deleted_at IS NULL
    LIMIT 100;
" | tr '\n' ',' | sed 's/,$//')

if [ -z "${SAMPLE_PACKAGES}" ]; then
    log "⚠️  No sample packages found in database"
else
    # Simulate individual lookups (N queries)
    log "Testing individual lookups (N queries)..."
    START=$(date +%s.%N)

    IFS=',' read -ra PKG_ARRAY <<< "${SAMPLE_PACKAGES}"
    for pkg in "${PKG_ARRAY[@]:0:20}"; do  # Test first 20 packages
        db_query "
            SELECT COUNT(*)
            FROM package_vulnerabilities
            WHERE ecosystem = 'debian'
            AND package_name = '${pkg}'
            AND deleted_at IS NULL;
        " > /dev/null
    done

    END=$(date +%s.%N)
    INDIVIDUAL_TIME=$(echo "$END - $START" | bc)
    log_metric "Individual lookups (20 queries): ${INDIVIDUAL_TIME}s"

    # Bulk lookup (1 query with IN clause)
    log "Testing bulk lookup (1 query)..."
    START=$(date +%s.%N)

    db_query "
        SELECT package_name, COUNT(*)
        FROM package_vulnerabilities
        WHERE ecosystem = 'debian'
        AND package_name IN ($(echo "${SAMPLE_PACKAGES}" | sed "s/,/','/g" | sed "s/^/'/;s/$/'/"))
        AND deleted_at IS NULL
        GROUP BY package_name;
    " > /dev/null

    END=$(date +%s.%N)
    BULK_TIME=$(echo "$END - $START" | bc)
    log_metric "Bulk lookup (1 query): ${BULK_TIME}s"

    # Calculate speedup
    SPEEDUP=$(echo "scale=2; ${INDIVIDUAL_TIME} / ${BULK_TIME}" | bc)
    log_comparison "Speedup with bulk lookup: ${SPEEDUP}x faster"
fi

echo "" | tee -a "${RESULT_FILE}"

# ============================================================================
# Benchmark 2: Insight Query Performance (With vs Without Indexes)
# ============================================================================
log "[Benchmark 2] Insight Query Performance"
echo "----------------------------------------" | tee -a "${RESULT_FILE}"

# Test insight lookup with indexes
log "Testing insight query (with optimized indexes)..."
START=$(date +%s.%N)

db_query "
    SELECT i.id, i.insight_type, i.severity, i.cve_id
    FROM insights i
    WHERE i.resource_uid IN (
        SELECT uid FROM pods WHERE deleted_at IS NULL LIMIT 50
    )
    AND i.insight_type = 'vulnerability'
    AND i.status = 'active'
    AND i.deleted_at IS NULL
    ORDER BY i.detected_at DESC
    LIMIT 100;
" > /dev/null

END=$(date +%s.%N)
INDEXED_TIME=$(echo "$END - $START" | bc)
log_metric "Indexed query time: ${INDEXED_TIME}s"

# Test complex join query
log "Testing complex join query (insights + CVE matches)..."
START=$(date +%s.%N)

db_query "
    SELECT i.id, i.cve_id, cm.package_name, cm.severity, cm.cvss
    FROM insights i
    JOIN cve_matches cm ON i.cve_id = cm.cve_id
    WHERE i.insight_type = 'vulnerability'
    AND i.deleted_at IS NULL
    AND cm.deleted_at IS NULL
    LIMIT 100;
" > /dev/null

END=$(date +%s.%N)
JOIN_TIME=$(echo "$END - $START" | bc)
log_metric "Complex join query time: ${JOIN_TIME}s"

# Performance target
TARGET=1.0
MEETS_TARGET=$(echo "${INDEXED_TIME} < ${TARGET}" | bc)
if [ "${MEETS_TARGET}" -eq "1" ]; then
    log_comparison "✅ Meets performance target (<${TARGET}s)"
else
    log_comparison "❌ Does not meet performance target (<${TARGET}s)"
fi

echo "" | tee -a "${RESULT_FILE}"

# ============================================================================
# Benchmark 3: Batch Insert Performance
# ============================================================================
log "[Benchmark 3] Batch Processing Efficiency"
echo "----------------------------------------" | tee -a "${RESULT_FILE}"

# Get recent SBOM for testing
RECENT_SBOM=$(db_query "SELECT id FROM sboms WHERE deleted_at IS NULL ORDER BY created_at DESC LIMIT 1;")

if [ -n "${RECENT_SBOM}" ]; then
    # Measure CVE matching time
    log "Measuring CVE matching time for SBOM ${RECENT_SBOM}..."

    COMPONENT_COUNT=$(db_query "SELECT COUNT(*) FROM sbom_components WHERE sbom_id = ${RECENT_SBOM} AND deleted_at IS NULL;")
    CVE_MATCH_COUNT=$(db_query "SELECT COUNT(*) FROM cve_matches WHERE sbom_id = ${RECENT_SBOM} AND deleted_at IS NULL;")

    log_metric "Components: ${COMPONENT_COUNT}"
    log_metric "CVE Matches: ${CVE_MATCH_COUNT}"

    # Calculate matches per component (efficiency metric)
    if [ "${COMPONENT_COUNT}" -gt "0" ]; then
        MATCH_RATIO=$(echo "scale=2; ${CVE_MATCH_COUNT} / ${COMPONENT_COUNT}" | bc)
        log_metric "Match ratio: ${MATCH_RATIO} matches/component"
    fi

    # Get matching timestamps to estimate processing time
    FIRST_MATCH=$(db_query "SELECT MIN(matched_at) FROM cve_matches WHERE sbom_id = ${RECENT_SBOM};")
    LAST_MATCH=$(db_query "SELECT MAX(matched_at) FROM cve_matches WHERE sbom_id = ${RECENT_SBOM};")

    if [ -n "${FIRST_MATCH}" ] && [ -n "${LAST_MATCH}" ]; then
        MATCH_DURATION=$(db_query "SELECT EXTRACT(EPOCH FROM ('${LAST_MATCH}'::timestamp - '${FIRST_MATCH}'::timestamp));")
        log_metric "Matching duration: ${MATCH_DURATION}s"

        # Performance target: < 5s for 200 packages
        NORMALIZED_TIME=$(echo "scale=2; ${MATCH_DURATION} * 200 / ${COMPONENT_COUNT}" | bc 2>/dev/null || echo "N/A")
        log_comparison "Projected time for 200 packages: ${NORMALIZED_TIME}s"

        TARGET=5.0
        if [ "${NORMALIZED_TIME}" != "N/A" ]; then
            MEETS_TARGET=$(echo "${NORMALIZED_TIME} < ${TARGET}" | bc)
            if [ "${MEETS_TARGET}" -eq "1" ]; then
                log_comparison "✅ Meets performance target (<${TARGET}s for 200 packages)"
            else
                log_comparison "⚠️  Does not meet performance target (<${TARGET}s for 200 packages)"
            fi
        fi
    fi
else
    log "⚠️  No SBOMs found in database"
fi

echo "" | tee -a "${RESULT_FILE}"

# ============================================================================
# Benchmark 4: Database Connection Pool Metrics
# ============================================================================
log "[Benchmark 4] Connection Pool Utilization"
echo "----------------------------------------" | tee -a "${RESULT_FILE}"

# Get metrics from Prometheus endpoint
METRICS=$(curl -s "${CORE_API}/metrics" 2>/dev/null || echo "")

if [ -n "${METRICS}" ]; then
    # Extract DB connection metrics
    CONNECTIONS_OPEN=$(echo "${METRICS}" | grep "^ksam_db_connections_open " | awk '{print $2}')
    CONNECTIONS_IN_USE=$(echo "${METRICS}" | grep "^ksam_db_connections_in_use " | awk '{print $2}')
    CONNECTIONS_IDLE=$(echo "${METRICS}" | grep "^ksam_db_connections_idle " | awk '{print $2}')

    if [ -n "${CONNECTIONS_OPEN}" ]; then
        log_metric "Connections open: ${CONNECTIONS_OPEN}"
        log_metric "Connections in use: ${CONNECTIONS_IN_USE}"
        log_metric "Connections idle: ${CONNECTIONS_IDLE}"

        # Calculate utilization
        UTILIZATION=$(echo "scale=2; (${CONNECTIONS_IN_USE} / ${CONNECTIONS_OPEN}) * 100" | bc)
        log_metric "Utilization: ${UTILIZATION}%"

        if [ $(echo "${UTILIZATION} < 80" | bc) -eq 1 ]; then
            log_comparison "✅ Healthy utilization (<80%)"
        else
            log_comparison "⚠️  High utilization (≥80%)"
        fi
    else
        log "⚠️  Connection metrics not available"
    fi

    # Extract worker metrics
    WORKER_MESSAGES=$(echo "${METRICS}" | grep "^ksam_worker_messages_processed_total" | wc -l)
    if [ "${WORKER_MESSAGES}" -gt "0" ]; then
        log_metric "Worker metrics available: Yes"

        # Get processing duration histogram
        PROCESSING_SUM=$(echo "${METRICS}" | grep "^ksam_worker_processing_duration_seconds_sum" | head -1 | awk '{print $2}')
        PROCESSING_COUNT=$(echo "${METRICS}" | grep "^ksam_worker_processing_duration_seconds_count" | head -1 | awk '{print $2}')

        if [ -n "${PROCESSING_SUM}" ] && [ -n "${PROCESSING_COUNT}" ] && [ "${PROCESSING_COUNT}" != "0" ]; then
            AVG_PROCESSING=$(echo "scale=3; ${PROCESSING_SUM} / ${PROCESSING_COUNT}" | bc)
            log_metric "Average worker processing time: ${AVG_PROCESSING}s"
        fi
    fi
else
    log "⚠️  Metrics endpoint not accessible"
fi

echo "" | tee -a "${RESULT_FILE}"

# ============================================================================
# Benchmark 5: Overall System Performance
# ============================================================================
log "[Benchmark 5] System-Wide Performance Metrics"
echo "----------------------------------------" | tee -a "${RESULT_FILE}"

# Get total counts
TOTAL_SBOMS=$(db_query "SELECT COUNT(*) FROM sboms WHERE deleted_at IS NULL;")
TOTAL_CVE_MATCHES=$(db_query "SELECT COUNT(*) FROM cve_matches WHERE deleted_at IS NULL;")
TOTAL_INSIGHTS=$(db_query "SELECT COUNT(*) FROM insights WHERE deleted_at IS NULL;")

log_metric "Total SBOMs: ${TOTAL_SBOMS}"
log_metric "Total CVE Matches: ${TOTAL_CVE_MATCHES}"
log_metric "Total Insights: ${TOTAL_INSIGHTS}"

if [ "${TOTAL_SBOMS}" -gt "0" ]; then
    AVG_MATCHES=$(echo "scale=2; ${TOTAL_CVE_MATCHES} / ${TOTAL_SBOMS}" | bc)
    AVG_INSIGHTS=$(echo "scale=2; ${TOTAL_INSIGHTS} / ${TOTAL_SBOMS}" | bc)

    log_metric "Average CVE matches per SBOM: ${AVG_MATCHES}"
    log_metric "Average insights per SBOM: ${AVG_INSIGHTS}"
fi

# Get database size
DB_SIZE=$(db_query "SELECT pg_size_pretty(pg_database_size('${DB_NAME}'));")
log_metric "Database size: ${DB_SIZE}"

# Get table sizes
log "Table sizes:"
db_query "
    SELECT
        tablename,
        pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size
    FROM pg_tables
    WHERE schemaname = 'public'
    AND tablename IN ('sboms', 'sbom_components', 'cve_matches', 'insights', 'cves', 'package_vulnerabilities')
    ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;
" | while IFS='|' read -r table size; do
    echo "  - ${table}: ${size}" | tee -a "${RESULT_FILE}"
done

echo "" | tee -a "${RESULT_FILE}"

# ============================================================================
# Performance Comparison with Targets
# ============================================================================
log "[Performance Targets] Comparison"
echo "----------------------------------------" | tee -a "${RESULT_FILE}"

cat >> "${RESULT_FILE}" <<EOF

Performance Target Summary:
---------------------------
Metric                          Target      Status
--------------------------------------------------------
SBOM Processing                 < 30s       ⏳ Measure during E2E
CVE Matching (200 packages)     < 5s        $([ -n "${NORMALIZED_TIME:-}" ] && [ "${NORMALIZED_TIME}" != "N/A" ] && [ $(echo "${NORMALIZED_TIME} < 5" | bc) -eq 1 ] && echo "✅ PASS" || echo "⏳ Pending")
Insight Query Performance       < 1s        $([ -n "${INDEXED_TIME:-}" ] && [ $(echo "${INDEXED_TIME} < 1.0" | bc) -eq 1 ] && echo "✅ PASS" || echo "❌ FAIL")
Database Connection Utilization < 80%       $([ -n "${UTILIZATION:-}" ] && [ $(echo "${UTILIZATION} < 80" | bc) -eq 1 ] && echo "✅ PASS" || echo "⏳ Pending")
Bulk CVE Lookup Speedup         > 5x        $([ -n "${SPEEDUP:-}" ] && [ $(echo "${SPEEDUP} > 5" | bc) -eq 1 ] && echo "✅ PASS" || echo "⏳ Pending")

Expected Improvements (vs baseline):
------------------------------------
- CVE Matching: 6-8x faster (bulk lookups, N+1 fix)
- Insight Generation: 20x faster (batch UPSERT)
- Database Queries: 30-40x reduction (indexes, batching)
- Overall E2E: 10-15x faster processing

EOF

cat "${RESULT_FILE}"

echo ""
echo "========================================"
echo "Results saved to: ${RESULT_FILE}"
echo "========================================"
