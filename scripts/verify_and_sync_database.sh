#!/bin/bash

# Verify and Sync Database Schema
# This script verifies database schema matches the system design and clears if needed

set -e

NAMESPACE="ksam"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
REPORT_DIR="test_results/db_verify_${TIMESTAMP}"
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

get_postgres_pod() {
    kubectl get pods -n "$NAMESPACE" -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null
}

exec_psql() {
    local pod=$1
    shift
    kubectl exec -n "$NAMESPACE" "$pod" -- psql -U postgres -d ksam "$@" 2>&1
}

# Step 1: Check Postgres
check_postgres() {
    log_info "Checking PostgreSQL..."
    
    POSTGRES_POD=$(get_postgres_pod)
    if [ -z "$POSTGRES_POD" ]; then
        log_error "PostgreSQL pod not found"
        exit 1
    fi
    
    STATUS=$(kubectl get pod -n "$NAMESPACE" "$POSTGRES_POD" -o jsonpath='{.status.phase}' 2>/dev/null)
    if [ "$STATUS" != "Running" ]; then
        log_error "PostgreSQL pod is not running (status: $STATUS)"
        exit 1
    fi
    
    log_success "PostgreSQL pod: $POSTGRES_POD (status: $STATUS)"
}

# Step 2: Get current schema
get_current_schema() {
    log_info "Getting current database schema..."
    
    POSTGRES_POD=$(get_postgres_pod)
    
    # Get all tables
    exec_psql "$POSTGRES_POD" -c "\dt" > "$REPORT_DIR/tables.txt" 2>&1
    
    # Get table structures
    exec_psql "$POSTGRES_POD" -c "
        SELECT 
            table_name,
            column_name,
            data_type,
            is_nullable,
            column_default
        FROM information_schema.columns
        WHERE table_schema = 'public'
        ORDER BY table_name, ordinal_position;
    " > "$REPORT_DIR/schema.txt" 2>&1
    
    # Get indexes
    exec_psql "$POSTGRES_POD" -c "
        SELECT 
            tablename,
            indexname,
            indexdef
        FROM pg_indexes
        WHERE schemaname = 'public'
        ORDER BY tablename, indexname;
    " > "$REPORT_DIR/indexes.txt" 2>&1
    
    log_success "Schema information saved to $REPORT_DIR/"
}

# Step 3: Verify required tables
verify_required_tables() {
    log_info "Verifying required tables..."
    
    POSTGRES_POD=$(get_postgres_pod)
    
    # Required tables based on models
    REQUIRED_TABLES=(
        "clusters"
        "service_accounts"
        "pods"
        "roles"
        "cluster_roles"
        "role_bindings"
        "cluster_role_bindings"
        "nodes"
        "policies"
        "insights"
        "events_index"
        "users"
        "audit_logs"
        "deployments"
        "replica_sets"
    )
    
    MISSING_TABLES=()
    EXISTING_TABLES=()
    
    for table in "${REQUIRED_TABLES[@]}"; do
        EXISTS=$(exec_psql "$POSTGRES_POD" -t -c "
            SELECT EXISTS (
                SELECT FROM information_schema.tables 
                WHERE table_schema = 'public' AND table_name = '$table'
            );
        " 2>&1 | tr -d ' ' | head -1)
        
        if [ "$EXISTS" = "t" ]; then
            log_success "Table '$table' exists"
            EXISTING_TABLES+=("$table")
        else
            log_error "Table '$table' is missing"
            MISSING_TABLES+=("$table")
        fi
    done
    
    echo "" | tee -a "$REPORT_DIR/verify.log"
    log_info "Summary: ${#EXISTING_TABLES[@]}/${#REQUIRED_TABLES[@]} tables exist"
    
    if [ ${#MISSING_TABLES[@]} -gt 0 ]; then
        log_warning "Missing tables: ${MISSING_TABLES[*]}"
        return 1
    fi
    
    return 0
}

# Step 4: Verify table structures
verify_table_structures() {
    log_info "Verifying table structures..."
    
    POSTGRES_POD=$(get_postgres_pod)
    
    # Check insights table has status and deleted_at
    INSIGHTS_STATUS=$(exec_psql "$POSTGRES_POD" -t -c "
        SELECT EXISTS (
            SELECT FROM information_schema.columns 
            WHERE table_schema = 'public' 
            AND table_name = 'insights' 
            AND column_name = 'status'
        );
    " 2>&1 | tr -d ' ' | head -1)
    
    INSIGHTS_DELETED_AT=$(exec_psql "$POSTGRES_POD" -t -c "
        SELECT EXISTS (
            SELECT FROM information_schema.columns 
            WHERE table_schema = 'public' 
            AND table_name = 'insights' 
            AND column_name = 'deleted_at'
        );
    " 2>&1 | tr -d ' ' | head -1)
    
    if [ "$INSIGHTS_STATUS" = "t" ]; then
        log_success "insights.status column exists"
    else
        log_error "insights.status column missing"
    fi
    
    if [ "$INSIGHTS_DELETED_AT" = "t" ]; then
        log_success "insights.deleted_at column exists"
    else
        log_error "insights.deleted_at column missing"
    fi
    
    # Check pods table has containers (jsonb)
    PODS_CONTAINERS=$(exec_psql "$POSTGRES_POD" -t -c "
        SELECT EXISTS (
            SELECT FROM information_schema.columns 
            WHERE table_schema = 'public' 
            AND table_name = 'pods' 
            AND column_name = 'containers'
        );
    " 2>&1 | tr -d ' ' | head -1)
    
    if [ "$PODS_CONTAINERS" = "t" ]; then
        log_success "pods.containers column exists"
    else
        log_error "pods.containers column missing"
    fi
    
    # Check service_accounts has linked_pods and last_used
    SA_LINKED_PODS=$(exec_psql "$POSTGRES_POD" -t -c "
        SELECT EXISTS (
            SELECT FROM information_schema.columns 
            WHERE table_schema = 'public' 
            AND table_name = 'service_accounts' 
            AND column_name = 'linked_pods'
        );
    " 2>&1 | tr -d ' ' | head -1)
    
    SA_LAST_USED=$(exec_psql "$POSTGRES_POD" -t -c "
        SELECT EXISTS (
            SELECT FROM information_schema.columns 
            WHERE table_schema = 'public' 
            AND table_name = 'service_accounts' 
            AND column_name = 'last_used'
        );
    " 2>&1 | tr -d ' ' | head -1)
    
    if [ "$SA_LINKED_PODS" = "t" ]; then
        log_success "service_accounts.linked_pods column exists"
    else
        log_error "service_accounts.linked_pods column missing"
    fi
    
    if [ "$SA_LAST_USED" = "t" ]; then
        log_success "service_accounts.last_used column exists"
    else
        log_error "service_accounts.last_used column missing"
    fi
}

# Step 5: Check indexes
verify_indexes() {
    log_info "Verifying indexes..."
    
    POSTGRES_POD=$(get_postgres_pod)
    
    # Check critical indexes
    CRITICAL_INDEXES=(
        "idx_pods_uid"
        "idx_insights_status"
        "idx_insights_deleted_at"
        "idx_service_accounts_cluster_id"
    )
    
    for idx in "${CRITICAL_INDEXES[@]}"; do
        EXISTS=$(exec_psql "$POSTGRES_POD" -t -c "
            SELECT EXISTS (
                SELECT FROM pg_indexes 
                WHERE schemaname = 'public' 
                AND indexname = '$idx'
            );
        " 2>&1 | tr -d ' ' | head -1)
        
        if [ "$EXISTS" = "t" ]; then
            log_success "Index '$idx' exists"
        else
            log_warning "Index '$idx' missing (may be created automatically)"
        fi
    done
}

# Step 6: Clear database (if requested)
clear_database() {
    if [ "$1" != "--clear" ]; then
        return 0
    fi
    
    log_warning "Clearing database (--clear flag provided)..."
    
    POSTGRES_POD=$(get_postgres_pod)
    
    exec_psql "$POSTGRES_POD" <<EOF
-- Disable foreign key checks temporarily
SET session_replication_role = 'replica';

-- Clear tables in order (respecting foreign keys)
TRUNCATE TABLE events_index CASCADE;
TRUNCATE TABLE insights CASCADE;
TRUNCATE TABLE policies CASCADE;
TRUNCATE TABLE pods CASCADE;
TRUNCATE TABLE role_bindings CASCADE;
TRUNCATE TABLE cluster_role_bindings CASCADE;
TRUNCATE TABLE roles CASCADE;
TRUNCATE TABLE cluster_roles CASCADE;
TRUNCATE TABLE service_accounts CASCADE;
TRUNCATE TABLE deployments CASCADE;
TRUNCATE TABLE replica_sets CASCADE;
TRUNCATE TABLE nodes CASCADE;
TRUNCATE TABLE namespaces CASCADE;
TRUNCATE TABLE clusters CASCADE;
TRUNCATE TABLE audit_logs CASCADE;
-- Keep users table (don't clear admin user)

-- Re-enable foreign key checks
SET session_replication_role = 'origin';

-- Show counts
SELECT 
    'clusters' as table_name, COUNT(*) as count FROM clusters
UNION ALL
SELECT 'service_accounts', COUNT(*) FROM service_accounts
UNION ALL
SELECT 'pods', COUNT(*) FROM pods
UNION ALL
SELECT 'roles', COUNT(*) FROM roles
UNION ALL
SELECT 'insights', COUNT(*) FROM insights
UNION ALL
SELECT 'users', COUNT(*) FROM users;
EOF
    
    log_success "Database cleared"
}

# Step 7: Run migrations (if needed)
run_migrations_if_needed() {
    if [ "$1" != "--migrate" ]; then
        return 0
    fi
    
    log_info "Running migrations (--migrate flag provided)..."
    
    # This would trigger Core to run migrations on restart
    # Or we can run them directly via Core pod
    CORE_POD=$(kubectl get pods -n "$NAMESPACE" -l app=ksam-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
    
    if [ -z "$CORE_POD" ]; then
        log_warning "Core pod not found, migrations will run on next Core start"
        return 0
    fi
    
    log_info "Core pod found: $CORE_POD"
    log_info "Migrations will run automatically when Core starts"
    log_info "To force migration, restart Core pod: kubectl delete pod -n ksam $CORE_POD"
}

# Step 8: Generate report
generate_report() {
    log_info "Generating verification report..."
    
    cat > "$REPORT_DIR/VERIFICATION_REPORT.md" <<EOF
# Database Verification Report

**Date**: $(date)
**Namespace**: $NAMESPACE

## Summary

### Tables Status
- Required tables: $(echo "${REQUIRED_TABLES[@]}" | wc -w)
- Existing tables: $(cat "$REPORT_DIR/tables.txt" | grep -c "public" || echo "0")

### Schema Files
- Tables: $REPORT_DIR/tables.txt
- Schema: $REPORT_DIR/schema.txt
- Indexes: $REPORT_DIR/indexes.txt

## Verification Results

See verify.log for detailed results.

## Next Steps

1. Review schema files in $REPORT_DIR/
2. Compare with models in KSAM/core/pkg/models/
3. Run migrations if needed: --migrate flag
4. Clear database if needed: --clear flag
EOF
    
    log_success "Report generated: $REPORT_DIR/VERIFICATION_REPORT.md"
}

# Main
main() {
    echo "=========================================="
    echo "Database Verification and Sync"
    echo "=========================================="
    echo ""
    
    check_postgres
    get_current_schema
    verify_required_tables
    verify_table_structures
    verify_indexes
    clear_database "$@"
    run_migrations_if_needed "$@"
    generate_report
    
    echo ""
    echo "=========================================="
    echo "Verification Complete"
    echo "=========================================="
    echo ""
    echo "Reports saved to: $REPORT_DIR/"
    echo ""
    echo "Usage:"
    echo "  $0              - Verify only"
    echo "  $0 --clear      - Verify and clear database"
    echo "  $0 --migrate    - Verify and run migrations"
    echo "  $0 --clear --migrate - Clear and migrate"
}

main "$@"

