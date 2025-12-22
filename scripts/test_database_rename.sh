#!/bin/bash
# Test Database Rename Script
# Purpose: Test KSAM → Fortuna database rename in safe environment
# Date: 2024-12-20

set -euo pipefail

# Colors for output
RED='\033[0,31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Functions
log_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

log_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

log_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

log_error() {
    echo -e "${RED}❌ $1${NC}"
}

# Configuration
NAMESPACE="${NAMESPACE:-ksam}"
OLD_DB_NAME="${OLD_DB_NAME:-ksam}"
NEW_DB_NAME="${NEW_DB_NAME:-fortuna}"
POSTGRES_POD=""

# Get PostgreSQL pod
get_postgres_pod() {
    POSTGRES_POD=$(kubectl -n "$NAMESPACE" get pods -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
    if [ -z "$POSTGRES_POD" ]; then
        log_error "PostgreSQL pod not found in namespace $NAMESPACE"
        exit 1
    fi
    log_info "Found PostgreSQL pod: $POSTGRES_POD"
}

# Execute SQL command
exec_sql() {
    local db=$1
    local sql=$2
    kubectl -n "$NAMESPACE" exec "$POSTGRES_POD" -- \
        psql -U postgres -d "$db" -t -c "$sql" 2>/dev/null
}

# Check if database exists
db_exists() {
    local db=$1
    local count
    count=$(exec_sql postgres "SELECT COUNT(*) FROM pg_database WHERE datname = '$db';")
    [ "$(echo "$count" | tr -d ' ')" = "1" ]
}

# Get table count
get_table_count() {
    local db=$1
    exec_sql "$db" "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public';" | tr -d ' '
}

# Get row count for a table
get_row_count() {
    local db=$1
    local table=$2
    exec_sql "$db" "SELECT COUNT(*) FROM $table;" | tr -d ' '
}

# Main test flow
main() {
    echo "═══════════════════════════════════════════════════════════════"
    echo "=== DATABASE RENAME TEST ==="
    echo "═══════════════════════════════════════════════════════════════"
    echo ""

    # Step 1: Prerequisites
    log_info "Step 1: Checking prerequisites..."
    get_postgres_pod

    if ! db_exists "$OLD_DB_NAME"; then
        log_error "Source database '$OLD_DB_NAME' does not exist"
        exit 1
    fi
    log_success "Source database '$OLD_DB_NAME' exists"

    if db_exists "$NEW_DB_NAME"; then
        log_warning "Target database '$NEW_DB_NAME' already exists!"
        read -p "Do you want to drop it and continue? (yes/no): " confirm
        if [ "$confirm" != "yes" ]; then
            log_info "Aborting test"
            exit 0
        fi
        log_info "Dropping existing '$NEW_DB_NAME' database..."
        exec_sql postgres "DROP DATABASE $NEW_DB_NAME;"
        log_success "Dropped existing database"
    fi

    # Step 2: Pre-rename verification
    log_info "Step 2: Pre-rename verification..."
    
    ORIGINAL_TABLE_COUNT=$(get_table_count "$OLD_DB_NAME")
    log_info "Tables in $OLD_DB_NAME: $ORIGINAL_TABLE_COUNT"

    ORIGINAL_CVE_COUNT=$(get_row_count "$OLD_DB_NAME" "cves")
    log_info "CVEs in $OLD_DB_NAME: $ORIGINAL_CVE_COUNT"

    ORIGINAL_INSIGHT_COUNT=$(get_row_count "$OLD_DB_NAME" "insights")
    log_info "Insights in $OLD_DB_NAME: $ORIGINAL_INSIGHT_COUNT"

    # Step 3: Stop applications (optional for test)
    log_info "Step 3: Checking active connections..."
    ACTIVE_CONNS=$(exec_sql "$OLD_DB_NAME" "SELECT COUNT(*) FROM pg_stat_activity WHERE datname = '$OLD_DB_NAME' AND pid <> pg_backend_pid();" | tr -d ' ')
    log_info "Active connections to $OLD_DB_NAME: $ACTIVE_CONNS"
    
    if [ "$ACTIVE_CONNS" -gt "0" ]; then
        log_warning "$ACTIVE_CONNS active connections found"
        log_warning "In production, you should:"
        log_warning "  1. Scale down applications"
        log_warning "  2. Terminate connections"
        log_warning "For this test, we'll continue anyway..."
    fi

    # Step 4: Create test database with copy
    log_info "Step 4: Creating test database with pg_dump/restore..."
    
    # Dump original database
    log_info "Dumping $OLD_DB_NAME..."
    kubectl -n "$NAMESPACE" exec "$POSTGRES_POD" -- \
        pg_dump -U postgres "$OLD_DB_NAME" > /tmp/${OLD_DB_NAME}_test_backup.sql
    log_success "Database dumped to /tmp/${OLD_DB_NAME}_test_backup.sql"

    # Create new database
    log_info "Creating $NEW_DB_NAME..."
    exec_sql postgres "CREATE DATABASE $NEW_DB_NAME;"
    log_success "Created database $NEW_DB_NAME"

    # Restore to new database
    log_info "Restoring data to $NEW_DB_NAME..."
    kubectl -n "$NAMESPACE" exec -i "$POSTGRES_POD" -- \
        psql -U postgres "$NEW_DB_NAME" < /tmp/${OLD_DB_NAME}_test_backup.sql > /dev/null 2>&1
    log_success "Data restored to $NEW_DB_NAME"

    # Step 5: Post-rename verification
    log_info "Step 5: Post-rename verification..."
    
    NEW_TABLE_COUNT=$(get_table_count "$NEW_DB_NAME")
    log_info "Tables in $NEW_DB_NAME: $NEW_TABLE_COUNT"

    NEW_CVE_COUNT=$(get_row_count "$NEW_DB_NAME" "cves")
    log_info "CVEs in $NEW_DB_NAME: $NEW_CVE_COUNT"

    NEW_INSIGHT_COUNT=$(get_row_count "$NEW_DB_NAME" "insights")
    log_info "Insights in $NEW_DB_NAME: $NEW_INSIGHT_COUNT"

    # Step 6: Compare results
    log_info "Step 6: Comparing results..."
    echo ""

    if [ "$ORIGINAL_TABLE_COUNT" = "$NEW_TABLE_COUNT" ]; then
        log_success "Table count matches: $ORIGINAL_TABLE_COUNT"
    else
        log_error "Table count mismatch! Original: $ORIGINAL_TABLE_COUNT, New: $NEW_TABLE_COUNT"
        exit 1
    fi

    if [ "$ORIGINAL_CVE_COUNT" = "$NEW_CVE_COUNT" ]; then
        log_success "CVE count matches: $ORIGINAL_CVE_COUNT"
    else
        log_error "CVE count mismatch! Original: $ORIGINAL_CVE_COUNT, New: $NEW_CVE_COUNT"
        exit 1
    fi

    if [ "$ORIGINAL_INSIGHT_COUNT" = "$NEW_INSIGHT_COUNT" ]; then
        log_success "Insight count matches: $ORIGINAL_INSIGHT_COUNT"
    else
        log_error "Insight count mismatch! Original: $ORIGINAL_INSIGHT_COUNT, New: $NEW_INSIGHT_COUNT"
        exit 1
    fi

    # Step 7: Test data integrity
    log_info "Step 7: Testing data integrity..."
    
    ORPHANED_PKGS=$(exec_sql "$NEW_DB_NAME" "SELECT COUNT(*) FROM package_vulnerabilities pv LEFT JOIN cves c ON pv.cve_id = c.cve_id WHERE c.cve_id IS NULL;" | tr -d ' ')
    if [ "$ORPHANED_PKGS" = "0" ]; then
        log_success "No orphaned package vulnerabilities"
    else
        log_error "Found $ORPHANED_PKGS orphaned package vulnerabilities"
        exit 1
    fi

    ORPHANED_COMPONENTS=$(exec_sql "$NEW_DB_NAME" "SELECT COUNT(*) FROM sbom_components sc LEFT JOIN sboms s ON sc.sbom_id = s.id WHERE s.id IS NULL;" | tr -d ' ')
    if [ "$ORPHANED_COMPONENTS" = "0" ]; then
        log_success "No orphaned SBOM components"
    else
        log_error "Found $ORPHANED_COMPONENTS orphaned SBOM components"
        exit 1
    fi

    # Step 8: Test connection with new name
    log_info "Step 8: Testing connection to new database..."
    TEST_QUERY=$(exec_sql "$NEW_DB_NAME" "SELECT 'Connection successful' as status;")
    if [[ "$TEST_QUERY" == *"Connection successful"* ]]; then
        log_success "Connection to $NEW_DB_NAME successful"
    else
        log_error "Failed to connect to $NEW_DB_NAME"
        exit 1
    fi

    # Step 9: Summary
    echo ""
    echo "═══════════════════════════════════════════════════════════════"
    echo "=== TEST SUMMARY ==="
    echo "═══════════════════════════════════════════════════════════════"
    echo ""
    log_success "✅ All tests passed!"
    echo ""
    echo "📊 Results:"
    echo "  - Tables: $NEW_TABLE_COUNT"
    echo "  - CVEs: $NEW_CVE_COUNT"
    echo "  - Insights: $NEW_INSIGHT_COUNT"
    echo "  - Data integrity: ✅ Perfect"
    echo ""
    echo "📝 Next steps:"
    echo "  1. Keep $NEW_DB_NAME for testing"
    echo "  2. Test application with new database name"
    echo "  3. Monitor for issues"
    echo "  4. When ready, drop old database"
    echo ""
    echo "🗑️  Cleanup (when ready):"
    echo "  kubectl -n $NAMESPACE exec $POSTGRES_POD -- psql -U postgres -c 'DROP DATABASE $OLD_DB_NAME;'"
    echo ""

    # Step 10: Cleanup option
    read -p "Do you want to clean up the test database $NEW_DB_NAME? (yes/no): " cleanup
    if [ "$cleanup" = "yes" ]; then
        log_info "Dropping test database $NEW_DB_NAME..."
        exec_sql postgres "DROP DATABASE $NEW_DB_NAME;"
        log_success "Test database dropped"
        rm -f /tmp/${OLD_DB_NAME}_test_backup.sql
        log_success "Backup file removed"
    else
        log_info "Test database $NEW_DB_NAME kept for further testing"
        log_info "Backup file: /tmp/${OLD_DB_NAME}_test_backup.sql"
    fi

    echo ""
    echo "═══════════════════════════════════════════════════════════════"
    log_success "Database rename test completed successfully!"
    echo "═══════════════════════════════════════════════════════════════"
}

# Run main function
main "$@"

