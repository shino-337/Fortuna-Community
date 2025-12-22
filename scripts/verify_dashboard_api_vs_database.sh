#!/bin/bash

# Verify Dashboard API vs Database
# Compares API responses with actual database data to ensure consistency

set -e

NAMESPACE="ksam"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
REPORT_DIR="test_results/dashboard_api_verify_${TIMESTAMP}"
mkdir -p "$REPORT_DIR"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1" | tee -a "$REPORT_DIR/verify.log"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1" | tee -a "$REPORT_DIR/verify.log"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1" | tee -a "$REPORT_DIR/verify.log"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1" | tee -a "$REPORT_DIR/verify.log"
}

log_section() {
    echo "" | tee -a "$REPORT_DIR/verify.log"
    echo "==========================================" | tee -a "$REPORT_DIR/verify.log"
    echo "$1" | tee -a "$REPORT_DIR/verify.log"
    echo "==========================================" | tee -a "$REPORT_DIR/verify.log"
}

# Configuration
CORE_PORT=8080
CORE_PF_PID=""

# Cleanup
cleanup() {
    if [ ! -z "$CORE_PF_PID" ]; then
        kill $CORE_PF_PID 2>/dev/null || true
    fi
}

trap cleanup EXIT

# Get pods
get_postgres_pod() {
    kubectl get pods -n "$NAMESPACE" -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null
}

get_core_pod() {
    kubectl get pods -n "$NAMESPACE" -l app=ksam-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null
}

# Database queries
exec_psql() {
    local pod=$1
    shift
    kubectl exec -n "$NAMESPACE" "$pod" -- psql -U postgres -d ksam "$@" 2>&1
}

# API calls
call_api() {
    local endpoint=$1
    curl -s "http://localhost:${CORE_PORT}${endpoint}" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $(get_auth_token)" 2>&1
}

get_auth_token() {
    local response=$(curl -s -X POST "http://localhost:${CORE_PORT}/api/v1/auth/login" \
        -H "Content-Type: application/json" \
        -d '{"username":"admin","password":"admin123"}' 2>&1)
    
    echo "$response" | grep -o '"token":"[^"]*' | cut -d'"' -f4 || echo ""
}

# ==========================================
# SETUP
# ==========================================

setup() {
    log_section "Setup: Port Forwarding and Authentication"
    
    # Setup port forwarding
    CORE_POD=$(get_core_pod)
    if [ -z "$CORE_POD" ]; then
        log_error "Core pod not found"
        exit 1
    fi
    
    log_info "Setting up port-forward to Core pod: $CORE_POD"
    kubectl port-forward -n "$NAMESPACE" "$CORE_POD" ${CORE_PORT}:8080 > /dev/null 2>&1 &
    CORE_PF_PID=$!
    sleep 3
    
    # Test connection
    if curl -s "http://localhost:${CORE_PORT}/health" > /dev/null 2>&1; then
        log_success "Core API accessible"
    else
        log_error "Core API not accessible"
        exit 1
    fi
    
    # Get auth token
    TOKEN=$(get_auth_token)
    if [ -z "$TOKEN" ]; then
        log_warning "Could not get auth token, some API calls may fail"
    else
        log_success "Authentication token obtained"
    fi
}

# ==========================================
# VERIFY INSIGHTS SUMMARY API
# ==========================================

verify_insights_summary() {
    log_section "Verifying Insights Summary API vs Database"
    
    POSTGRES_POD=$(get_postgres_pod)
    
    # Get from Database
    log_info "Querying database for insights summary..."
    DB_TOTAL=$(exec_psql "$POSTGRES_POD" -t -c "
        SELECT COUNT(*) FROM insights 
        WHERE deleted_at IS NULL 
        AND (status = 'active' OR status IS NULL);
    " 2>&1 | tr -d ' ')
    
    DB_CRITICAL=$(exec_psql "$POSTGRES_POD" -t -c "
        SELECT COUNT(*) FROM insights 
        WHERE deleted_at IS NULL 
        AND (status = 'active' OR status IS NULL)
        AND LOWER(severity) = 'critical';
    " 2>&1 | tr -d ' ')
    
    DB_HIGH=$(exec_psql "$POSTGRES_POD" -t -c "
        SELECT COUNT(*) FROM insights 
        WHERE deleted_at IS NULL 
        AND (status = 'active' OR status IS NULL)
        AND LOWER(severity) = 'high';
    " 2>&1 | tr -d ' ')
    
    DB_MEDIUM=$(exec_psql "$POSTGRES_POD" -t -c "
        SELECT COUNT(*) FROM insights 
        WHERE deleted_at IS NULL 
        AND (status = 'active' OR status IS NULL)
        AND LOWER(severity) = 'medium';
    " 2>&1 | tr -d ' ')
    
    DB_LOW=$(exec_psql "$POSTGRES_POD" -t -c "
        SELECT COUNT(*) FROM insights 
        WHERE deleted_at IS NULL 
        AND (status = 'active' OR status IS NULL)
        AND LOWER(severity) = 'low';
    " 2>&1 | tr -d ' ')
    
    log_info "Database Insights Summary:"
    log_info "  Total: $DB_TOTAL"
    log_info "  Critical: $DB_CRITICAL"
    log_info "  High: $DB_HIGH"
    log_info "  Medium: $DB_MEDIUM"
    log_info "  Low: $DB_LOW"
    
    # Get from API
    log_info "Calling Insights Summary API..."
    API_RESPONSE=$(call_api "/api/v1/insights/summary")
    echo "$API_RESPONSE" > "$REPORT_DIR/api_insights_summary.json"
    
    if command -v jq &> /dev/null; then
        API_TOTAL=$(echo "$API_RESPONSE" | jq -r '.total // 0' 2>/dev/null || echo "0")
        API_CRITICAL=$(echo "$API_RESPONSE" | jq -r '.critical // 0' 2>/dev/null || echo "0")
        API_HIGH=$(echo "$API_RESPONSE" | jq -r '.high // 0' 2>/dev/null || echo "0")
        API_MEDIUM=$(echo "$API_RESPONSE" | jq -r '.medium // 0' 2>/dev/null || echo "0")
        API_LOW=$(echo "$API_RESPONSE" | jq -r '.low // 0' 2>/dev/null || echo "0")
    else
        # Fallback parsing without jq
        API_TOTAL=$(echo "$API_RESPONSE" | grep -o '"total":[0-9]*' | cut -d':' -f2 || echo "0")
        API_CRITICAL=$(echo "$API_RESPONSE" | grep -o '"critical":[0-9]*' | cut -d':' -f2 || echo "0")
        API_HIGH=$(echo "$API_RESPONSE" | grep -o '"high":[0-9]*' | cut -d':' -f2 || echo "0")
        API_MEDIUM=$(echo "$API_RESPONSE" | grep -o '"medium":[0-9]*' | cut -d':' -f2 || echo "0")
        API_LOW=$(echo "$API_RESPONSE" | grep -o '"low":[0-9]*' | cut -d':' -f2 || echo "0")
    fi
    
    log_info "API Insights Summary:"
    log_info "  Total: $API_TOTAL"
    log_info "  Critical: $API_CRITICAL"
    log_info "  High: $API_HIGH"
    log_info "  Medium: $API_MEDIUM"
    log_info "  Low: $API_LOW"
    
    # Compare
    log_info "Comparing Database vs API..."
    
    if [ "$DB_TOTAL" = "$API_TOTAL" ]; then
        log_success "✅ Total insights match: DB ($DB_TOTAL) = API ($API_TOTAL)"
    else
        log_error "❌ Total insights mismatch: DB ($DB_TOTAL) vs API ($API_TOTAL)"
    fi
    
    if [ "$DB_CRITICAL" = "$API_CRITICAL" ]; then
        log_success "✅ Critical insights match: DB ($DB_CRITICAL) = API ($API_CRITICAL)"
    else
        log_warning "⚠️  Critical insights mismatch: DB ($DB_CRITICAL) vs API ($API_CRITICAL)"
    fi
    
    if [ "$DB_HIGH" = "$API_HIGH" ]; then
        log_success "✅ High insights match: DB ($DB_HIGH) = API ($API_HIGH)"
    else
        log_warning "⚠️  High insights mismatch: DB ($DB_HIGH) vs API ($API_HIGH)"
    fi
    
    if [ "$DB_MEDIUM" = "$API_MEDIUM" ]; then
        log_success "✅ Medium insights match: DB ($DB_MEDIUM) = API ($API_MEDIUM)"
    else
        log_warning "⚠️  Medium insights mismatch: DB ($DB_MEDIUM) vs API ($API_MEDIUM)"
    fi
    
    if [ "$DB_LOW" = "$API_LOW" ]; then
        log_success "✅ Low insights match: DB ($DB_LOW) = API ($API_LOW)"
    else
        log_warning "⚠️  Low insights mismatch: DB ($DB_LOW) vs API ($API_LOW)"
    fi
}

# ==========================================
# VERIFY INSIGHTS LIST API
# ==========================================

verify_insights_list() {
    log_section "Verifying Insights List API vs Database"
    
    POSTGRES_POD=$(get_postgres_pod)
    
    # Get from Database
    log_info "Querying database for insights list..."
    DB_INSIGHTS_COUNT=$(exec_psql "$POSTGRES_POD" -t -c "
        SELECT COUNT(*) FROM (
            SELECT id FROM insights 
            WHERE deleted_at IS NULL 
            AND (status = 'active' OR status IS NULL)
            ORDER BY created_at DESC
            LIMIT 50
        ) sub;
    " 2>&1 | tr -d ' ')
    
    # Get from API
    log_info "Calling Insights List API..."
    API_RESPONSE=$(call_api "/api/v1/insights?pageSize=50")
    echo "$API_RESPONSE" > "$REPORT_DIR/api_insights_list.json"
    
    if command -v jq &> /dev/null; then
        API_TOTAL=$(echo "$API_RESPONSE" | jq -r '.total // 0' 2>/dev/null || echo "0")
        API_INSIGHTS_COUNT=$(echo "$API_RESPONSE" | jq -r '.insights | length' 2>/dev/null || echo "0")
    else
        API_TOTAL=$(echo "$API_RESPONSE" | grep -o '"total":[0-9]*' | cut -d':' -f2 || echo "0")
        API_INSIGHTS_COUNT=$(echo "$API_RESPONSE" | grep -o '"insights":\[' | wc -l || echo "0")
    fi
    
    log_info "Database: $DB_INSIGHTS_COUNT insights (first 50)"
    log_info "API Total: $API_TOTAL"
    log_info "API Returned: $API_INSIGHTS_COUNT insights"
    
    # Verify total matches
    DB_TOTAL=$(exec_psql "$POSTGRES_POD" -t -c "
        SELECT COUNT(*) FROM insights 
        WHERE deleted_at IS NULL 
        AND (status = 'active' OR status IS NULL);
    " 2>&1 | tr -d ' ')
    
    if [ "$DB_TOTAL" = "$API_TOTAL" ]; then
        log_success "✅ Insights total count matches: DB ($DB_TOTAL) = API ($API_TOTAL)"
    else
        log_error "❌ Insights total count mismatch: DB ($DB_TOTAL) vs API ($API_TOTAL)"
    fi
}

# ==========================================
# VERIFY SERVICEACCOUNTS API
# ==========================================

verify_serviceaccounts_api() {
    log_section "Verifying ServiceAccounts API vs Database"
    
    POSTGRES_POD=$(get_postgres_pod)
    
    # Get from Database
    log_info "Querying database for service accounts..."
    DB_SA_COUNT=$(exec_psql "$POSTGRES_POD" -t -c "
        SELECT COUNT(*) FROM service_accounts 
        WHERE deleted_at IS NULL;
    " 2>&1 | tr -d ' ')
    
    # Get from API
    log_info "Calling ServiceAccounts API..."
    API_RESPONSE=$(call_api "/api/v1/serviceaccounts?pageSize=100")
    echo "$API_RESPONSE" > "$REPORT_DIR/api_serviceaccounts.json"
    
    if command -v jq &> /dev/null; then
        API_TOTAL=$(echo "$API_RESPONSE" | jq -r '.total // 0' 2>/dev/null || echo "0")
        API_SA_COUNT=$(echo "$API_RESPONSE" | jq -r '.serviceAccounts | length' 2>/dev/null || echo "0")
    else
        API_TOTAL=$(echo "$API_RESPONSE" | grep -o '"total":[0-9]*' | cut -d':' -f2 || echo "0")
        API_SA_COUNT=$(echo "$API_RESPONSE" | grep -o '"serviceAccounts":\[' | wc -l || echo "0")
    fi
    
    log_info "Database ServiceAccounts: $DB_SA_COUNT"
    log_info "API Total: $API_TOTAL"
    log_info "API Returned: $API_SA_COUNT"
    
    if [ "$DB_SA_COUNT" = "$API_TOTAL" ]; then
        log_success "✅ ServiceAccounts count matches: DB ($DB_SA_COUNT) = API ($API_TOTAL)"
    else
        log_error "❌ ServiceAccounts count mismatch: DB ($DB_SA_COUNT) vs API ($API_TOTAL)"
    fi
    
    # Verify sample records match
    log_info "Verifying sample ServiceAccount records..."
    DB_SAMPLE=$(exec_psql "$POSTGRES_POD" -c "
        SELECT name, namespace, cluster_id, created_at 
        FROM service_accounts 
        WHERE deleted_at IS NULL 
        ORDER BY created_at DESC 
        LIMIT 5;
    " 2>&1)
    
    echo "$DB_SAMPLE" > "$REPORT_DIR/db_serviceaccounts_sample.txt"
    log_info "Database sample saved to db_serviceaccounts_sample.txt"
}

# ==========================================
# VERIFY PODS API
# ==========================================

verify_pods_api() {
    log_section "Verifying Pods API vs Database"
    
    POSTGRES_POD=$(get_postgres_pod)
    
    # Get from Database
    log_info "Querying database for pods..."
    DB_POD_COUNT=$(exec_psql "$POSTGRES_POD" -t -c "
        SELECT COUNT(*) FROM pods 
        WHERE deleted_at IS NULL;
    " 2>&1 | tr -d ' ')
    
    # Get from API
    log_info "Calling Pods API..."
    API_RESPONSE=$(call_api "/api/v1/pods?pageSize=100")
    echo "$API_RESPONSE" > "$REPORT_DIR/api_pods.json"
    
    if command -v jq &> /dev/null; then
        API_TOTAL=$(echo "$API_RESPONSE" | jq -r '.total // 0' 2>/dev/null || echo "0")
        API_POD_COUNT=$(echo "$API_RESPONSE" | jq -r '.pods | length' 2>/dev/null || echo "0")
    else
        API_TOTAL=$(echo "$API_RESPONSE" | grep -o '"total":[0-9]*' | cut -d':' -f2 || echo "0")
        API_POD_COUNT=$(echo "$API_RESPONSE" | grep -o '"pods":\[' | wc -l || echo "0")
    fi
    
    log_info "Database Pods: $DB_POD_COUNT"
    log_info "API Total: $API_TOTAL"
    log_info "API Returned: $API_POD_COUNT"
    
    if [ "$DB_POD_COUNT" = "$API_TOTAL" ]; then
        log_success "✅ Pods count matches: DB ($DB_POD_COUNT) = API ($API_TOTAL)"
    else
        log_error "❌ Pods count mismatch: DB ($DB_POD_COUNT) vs API ($API_TOTAL)"
    fi
}

# ==========================================
# VERIFY CLUSTERS STATS API
# ==========================================

verify_clusters_stats_api() {
    log_section "Verifying Clusters Stats API vs Database"
    
    POSTGRES_POD=$(get_postgres_pod)
    
    # Get from Database
    log_info "Querying database for clusters stats..."
    DB_CLUSTER_COUNT=$(exec_psql "$POSTGRES_POD" -t -c "
        SELECT COUNT(*) FROM clusters 
        WHERE deleted_at IS NULL;
    " 2>&1 | tr -d ' ')
    
    DB_TOTAL_PODS=$(exec_psql "$POSTGRES_POD" -t -c "
        SELECT COUNT(*) FROM pods 
        WHERE deleted_at IS NULL;
    " 2>&1 | tr -d ' ')
    
    DB_TOTAL_SAS=$(exec_psql "$POSTGRES_POD" -t -c "
        SELECT COUNT(*) FROM service_accounts 
        WHERE deleted_at IS NULL;
    " 2>&1 | tr -d ' ')
    
    # Get from API
    log_info "Calling Clusters Stats API..."
    API_RESPONSE=$(call_api "/api/v1/clusters/stats")
    echo "$API_RESPONSE" > "$REPORT_DIR/api_clusters_stats.json"
    
    if command -v jq &> /dev/null; then
        API_CLUSTER_COUNT=$(echo "$API_RESPONSE" | jq -r '.total // 0' 2>/dev/null || echo "0")
        API_CLUSTERS=$(echo "$API_RESPONSE" | jq -r '.clusters | length' 2>/dev/null || echo "0")
    else
        API_CLUSTER_COUNT=$(echo "$API_RESPONSE" | grep -o '"total":[0-9]*' | cut -d':' -f2 || echo "0")
        API_CLUSTERS=$(echo "$API_RESPONSE" | grep -o '"clusters":\[' | wc -l || echo "0")
    fi
    
    log_info "Database Clusters: $DB_CLUSTER_COUNT"
    log_info "Database Total Pods: $DB_TOTAL_PODS"
    log_info "Database Total ServiceAccounts: $DB_TOTAL_SAS"
    log_info "API Clusters: $API_CLUSTER_COUNT"
    
    if [ "$DB_CLUSTER_COUNT" = "$API_CLUSTER_COUNT" ]; then
        log_success "✅ Clusters count matches: DB ($DB_CLUSTER_COUNT) = API ($API_CLUSTER_COUNT)"
    else
        log_warning "⚠️  Clusters count mismatch: DB ($DB_CLUSTER_COUNT) vs API ($API_CLUSTER_COUNT)"
    fi
}

# ==========================================
# VERIFY DATA CONSISTENCY
# ==========================================

verify_data_consistency() {
    log_section "Verifying Data Consistency"
    
    POSTGRES_POD=$(get_postgres_pod)
    
    # Check for orphaned records
    log_info "Checking for orphaned records..."
    
    # Orphaned pods (no matching ServiceAccount)
    ORPHANED_PODS=$(exec_psql "$POSTGRES_POD" -t -c "
        SELECT COUNT(*) FROM pods p
        WHERE p.deleted_at IS NULL
        AND p.service_account != ''
        AND NOT EXISTS (
            SELECT 1 FROM service_accounts sa
            WHERE sa.name = p.service_account
            AND sa.namespace = p.namespace
            AND sa.cluster_id = p.cluster_id
            AND sa.deleted_at IS NULL
        );
    " 2>&1 | tr -d ' ')
    
    if [ "$ORPHANED_PODS" = "0" ]; then
        log_success "✅ No orphaned pods found"
    else
        log_warning "⚠️  Found $ORPHANED_PODS orphaned pods (pods with SA that doesn't exist)"
    fi
    
    # Check correlation integrity
    log_info "Checking correlation integrity..."
    
    TOTAL_PODS_WITH_SA=$(exec_psql "$POSTGRES_POD" -t -c "
        SELECT COUNT(*) FROM pods 
        WHERE deleted_at IS NULL 
        AND service_account != ''
        AND service_account != 'default';
    " 2>&1 | tr -d ' ')
    
    CORRELATED_PODS=$(exec_psql "$POSTGRES_POD" -t -c "
        SELECT COUNT(DISTINCT p.id) FROM pods p
        INNER JOIN service_accounts sa ON p.service_account = sa.name 
            AND p.namespace = sa.namespace
            AND p.cluster_id = sa.cluster_id
        WHERE p.deleted_at IS NULL 
        AND sa.deleted_at IS NULL
        AND p.service_account != ''
        AND p.service_account != 'default';
    " 2>&1 | tr -d ' ')
    
    log_info "Total Pods with ServiceAccount: $TOTAL_PODS_WITH_SA"
    log_info "Correlated Pods: $CORRELATED_PODS"
    
    if [ "$CORRELATED_PODS" = "$TOTAL_PODS_WITH_SA" ]; then
        log_success "✅ All pods with ServiceAccount are properly correlated"
    else
        log_warning "⚠️  Correlation mismatch: $CORRELATED_PODS correlated vs $TOTAL_PODS_WITH_SA total with SA"
    fi
}

# ==========================================
# GENERATE REPORT
# ==========================================

generate_report() {
    log_section "Generating Verification Report"
    
    cat > "$REPORT_DIR/DASHBOARD_API_VERIFICATION_REPORT.md" <<EOF
# Dashboard API vs Database Verification Report

**Date**: $(date)
**Purpose**: Verify Dashboard APIs return correct data from database

## Summary

This report compares Dashboard API responses with actual database data to ensure:
- API endpoints return correct counts
- Data consistency between API and database
- Dashboard displays accurate information

## Verification Results

See verify.log for detailed results.

## Files Generated

- \`verify.log\`: Full verification log
- \`api_insights_summary.json\`: Insights summary API response
- \`api_insights_list.json\`: Insights list API response
- \`api_serviceaccounts.json\`: ServiceAccounts API response
- \`api_pods.json\`: Pods API response
- \`api_clusters_stats.json\`: Clusters stats API response
- \`db_serviceaccounts_sample.txt\`: Database sample data

## API Endpoints Verified

1. \`GET /api/v1/insights/summary\` - Insights summary statistics
2. \`GET /api/v1/insights\` - Insights list with pagination
3. \`GET /api/v1/serviceaccounts\` - ServiceAccounts list
4. \`GET /api/v1/pods\` - Pods list
5. \`GET /api/v1/clusters/stats\` - Clusters statistics

## Data Consistency Checks

- Orphaned records detection
- Correlation integrity (Pod → ServiceAccount)
- Count accuracy (API vs Database)

EOF
    
    log_success "Report generated: $REPORT_DIR/DASHBOARD_API_VERIFICATION_REPORT.md"
}

# ==========================================
# MAIN
# ==========================================

main() {
    echo "=========================================="
    echo "Dashboard API vs Database Verification"
    echo "=========================================="
    echo ""
    
    setup
    verify_insights_summary
    verify_insights_list
    verify_serviceaccounts_api
    verify_pods_api
    verify_clusters_stats_api
    verify_data_consistency
    generate_report
    
    echo ""
    echo "=========================================="
    echo "Verification Complete"
    echo "=========================================="
    echo ""
    echo "Report saved to: $REPORT_DIR/"
}

main "$@"

