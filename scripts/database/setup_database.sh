#!/bin/bash

# Database Setup Script for KSAM
# Runs all migrations and verifies database setup

set -e

NAMESPACE="ksam"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
REPORT_DIR="test_results/db_setup_${TIMESTAMP}"
mkdir -p "$REPORT_DIR"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1" | tee -a "$REPORT_DIR/setup.log"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1" | tee -a "$REPORT_DIR/setup.log"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1" | tee -a "$REPORT_DIR/setup.log"
}

log_section() {
    echo "" | tee -a "$REPORT_DIR/setup.log"
    echo "==========================================" | tee -a "$REPORT_DIR/setup.log"
    echo "$1" | tee -a "$REPORT_DIR/setup.log"
    echo "==========================================" | tee -a "$REPORT_DIR/setup.log"
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
    log_section "Step 1: Checking PostgreSQL"
    
    POSTGRES_POD=$(get_postgres_pod)
    if [ -z "$POSTGRES_POD" ]; then
        log_error "PostgreSQL pod not found"
        exit 1
    fi
    
    log_success "PostgreSQL pod found: $POSTGRES_POD"
    
    # Check if pod is ready
    STATUS=$(kubectl get pod -n "$NAMESPACE" "$POSTGRES_POD" -o jsonpath='{.status.phase}' 2>/dev/null)
    if [ "$STATUS" != "Running" ]; then
        log_error "PostgreSQL pod is not running (status: $STATUS)"
        exit 1
    fi
    
    log_success "PostgreSQL pod is running"
    
    # Test connection
    log_info "Testing database connection..."
    VERSION=$(exec_psql "$POSTGRES_POD" -c "SELECT version();" 2>&1 | grep -i "PostgreSQL" | head -1)
    if [ -n "$VERSION" ]; then
        log_success "Database connection successful"
        echo "$VERSION" | tee -a "$REPORT_DIR/setup.log"
    else
        log_error "Database connection failed"
        exit 1
    fi
}

# Step 2: Check database exists
check_database() {
    log_section "Step 2: Checking Database"
    
    POSTGRES_POD=$(get_postgres_pod)
    
    log_info "Checking if database 'ksam' exists..."
    DB_EXISTS=$(exec_psql "$POSTGRES_POD" -lqt 2>&1 | grep -c "ksam" || echo "0")
    
    if [ "$DB_EXISTS" -gt 0 ]; then
        log_success "Database 'ksam' exists"
    else
        log_info "Database 'ksam' not found, creating..."
        kubectl exec -n "$NAMESPACE" "$POSTGRES_POD" -- psql -U postgres -c "CREATE DATABASE ksam;" 2>&1 | tee -a "$REPORT_DIR/setup.log"
        log_success "Database 'ksam' created"
    fi
}

# Step 3: Run migrations
run_migrations() {
    log_section "Step 3: Running Migrations"
    
    POSTGRES_POD=$(get_postgres_pod)
    
    # Find all migration files
    MIGRATIONS=$(find core/migrations -name "*.sql" -type f | sort)
    
    if [ -z "$MIGRATIONS" ]; then
        log_error "No migration files found"
        exit 1
    fi
    
    log_info "Found $(echo "$MIGRATIONS" | wc -l | tr -d ' ') migration file(s)"
    
    for migration in $MIGRATIONS; do
        MIG_NAME=$(basename "$migration")
        log_info "Running migration: $MIG_NAME"
        
        # Copy migration to pod
        kubectl cp "$migration" "$NAMESPACE/$POSTGRES_POD:/tmp/$MIG_NAME" 2>&1 | tee -a "$REPORT_DIR/setup.log"
        
        # Run migration
        OUTPUT=$(exec_psql "$POSTGRES_POD" -f "/tmp/$MIG_NAME" 2>&1)
        echo "$OUTPUT" | tee -a "$REPORT_DIR/setup.log"
        
        # Check for errors
        if echo "$OUTPUT" | grep -qi "error"; then
            # Check if it's a "already exists" error (which is OK)
            if echo "$OUTPUT" | grep -qi "already exists\|duplicate"; then
                log_success "Migration $MIG_NAME completed (some objects may already exist)"
            else
                log_error "Migration $MIG_NAME failed"
                echo "$OUTPUT" | tee -a "$REPORT_DIR/errors.log"
            fi
        else
            log_success "Migration $MIG_NAME completed successfully"
        fi
        
        echo "" | tee -a "$REPORT_DIR/setup.log"
    done
}

# Step 4: Verify schema
verify_schema() {
    log_section "Step 4: Verifying Database Schema"
    
    POSTGRES_POD=$(get_postgres_pod)
    
    log_info "Listing all tables..."
    TABLES=$(exec_psql "$POSTGRES_POD" -c "\dt" 2>&1)
    echo "$TABLES" | tee -a "$REPORT_DIR/schema.txt"
    
    TABLE_COUNT=$(echo "$TABLES" | grep -c "public\|Schema" || echo "0")
    log_info "Found $TABLE_COUNT table(s)"
    
    log_info "Listing all extensions..."
    EXTENSIONS=$(exec_psql "$POSTGRES_POD" -c "SELECT extname, extversion FROM pg_extension;" 2>&1)
    echo "$EXTENSIONS" | tee -a "$REPORT_DIR/extensions.txt"
    
    log_info "Checking required tables..."
    REQUIRED_TABLES=("clusters" "service_accounts" "pods" "roles" "role_bindings" "insights")
    MISSING_TABLES=()
    
    for table in "${REQUIRED_TABLES[@]}"; do
        EXISTS=$(exec_psql "$POSTGRES_POD" -t -c "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema = 'public' AND table_name = '$table');" 2>&1 | tr -d ' ')
        if [ "$EXISTS" = "t" ]; then
            log_success "Table '$table' exists"
        else
            log_error "Table '$table' is missing"
            MISSING_TABLES+=("$table")
        fi
    done
    
    if [ ${#MISSING_TABLES[@]} -eq 0 ]; then
        log_success "All required tables exist"
    else
        log_error "Missing tables: ${MISSING_TABLES[*]}"
    fi
}

# Step 5: Check Core connection
check_core_connection() {
    log_section "Step 5: Checking Core Database Connection"
    
    CORE_POD=$(kubectl get pods -n "$NAMESPACE" -l app=ksam-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
    
    if [ -z "$CORE_POD" ]; then
        log_error "Core pod not found"
        return 1
    fi
    
    log_info "Core pod: $CORE_POD"
    
    log_info "Checking Core health endpoint..."
    HEALTH=$(kubectl exec -n "$NAMESPACE" "$CORE_POD" -- wget -qO- http://localhost:8080/health 2>&1)
    echo "$HEALTH" | tee -a "$REPORT_DIR/core_health.json"
    
    if echo "$HEALTH" | grep -q "healthy"; then
        log_success "Core health check passed"
        
        if echo "$HEALTH" | grep -q "\"database\":\"ok\""; then
            log_success "Core database connection verified"
        else
            log_error "Core database connection failed"
        fi
    else
        log_error "Core health check failed"
    fi
}

# Main
main() {
    log_section "KSAM Database Setup"
    echo "Starting database setup at $(date)"
    echo ""
    
    check_postgres
    check_database
    run_migrations
    verify_schema
    check_core_connection
    
    log_section "Database Setup Complete"
    echo "Setup finished at $(date)"
    echo ""
    echo "Reports saved to: $REPORT_DIR"
    echo "  - setup.log: Full setup log"
    echo "  - schema.txt: Database schema"
    echo "  - extensions.txt: Installed extensions"
    echo "  - core_health.json: Core health check"
}

main

