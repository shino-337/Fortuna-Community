#!/bin/bash

# Database Schema Verification Script
# Verifies that all required database tables and columns exist

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
NAMESPACE="${NAMESPACE:-fortuna}"
DB_NAME="${DB_NAME:-fortuna}"
DB_USER="${DB_USER:-postgres}"

# Functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Get PostgreSQL pod
get_postgres_pod() {
    kubectl get pods -n "$NAMESPACE" -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null
}

# Check if table exists
check_table() {
    local table_name=$1
    local pod=$2
    
    kubectl exec -n "$NAMESPACE" "$pod" -- psql -U "$DB_USER" -d "$DB_NAME" -t -A -c \
        "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema = 'public' AND table_name = '$table_name');" 2>/dev/null | tr -d ' ' | grep -q "t"
}

# Check if column exists
check_column() {
    local table_name=$1
    local column_name=$2
    local pod=$3
    
    kubectl exec -n "$NAMESPACE" "$pod" -- psql -U "$DB_USER" -d "$DB_NAME" -t -A -c \
        "SELECT EXISTS (SELECT FROM information_schema.columns WHERE table_schema = 'public' AND table_name = '$table_name' AND column_name = '$column_name');" 2>/dev/null | tr -d ' ' | grep -q "t"
}

# Verify required tables
verify_tables() {
    local pod=$1
    local all_passed=true
    
    log_info "Verifying required tables..."
    
    # Core tables
    local core_tables=(
        "clusters"
        "service_accounts"
        "roles"
        "cluster_roles"
        "role_bindings"
        "cluster_role_bindings"
        "pods"
        "audit_logs"
        "users"
        "deployments"
        "replicasets"
        "insights"
        "nodes"
        "policies"
        "events_index"
    )
    
    # CVE and SBOM tables
    local cve_sbom_tables=(
        "cves"
        "package_vulnerabilities"
        "sboms"
        "sbom_components"
        "cve_matches"
    )
    
    # Verify core tables
    for table in "${core_tables[@]}"; do
        if check_table "$table" "$pod"; then
            log_success "Table '$table' exists"
        else
            log_error "Table '$table' is missing"
            all_passed=false
        fi
    done
    
    # Verify CVE/SBOM tables
    for table in "${cve_sbom_tables[@]}"; do
        if check_table "$table" "$pod"; then
            log_success "Table '$table' exists"
        else
            log_error "Table '$table' is missing"
            all_passed=false
        fi
    done
    
    if [ "$all_passed" = true ]; then
        log_success "All required tables exist"
        return 0
    else
        log_error "Some required tables are missing"
        return 1
    fi
}

# Verify critical columns
verify_columns() {
    local pod=$1
    local all_passed=true
    
    log_info "Verifying critical columns..."
    
    # Check sboms table columns
    local sbom_columns=(
        "sboms:image_digest"
        "sboms:image_name"
        "sboms:image_tag"
        "sboms:pod_name"
        "sboms:pod_uid"
        "sboms:namespace"
        "sboms:container_name"
        "sboms:package_count"
    )
    
    # Check sbom_components table columns
    local component_columns=(
        "sbom_components:component_name"
        "sbom_components:component_version"
        "sbom_components:component_type"
        "sbom_components:purl"
    )
    
    # Check cve_matches table columns
    local cve_match_columns=(
        "cve_matches:cve_id"
        "cve_matches:package_name"
        "cve_matches:package_version"
        "cve_matches:pod_uid"
        "cve_matches:container_name"
    )
    
    # Check insights table columns
    local insight_columns=(
        "insights:insight_type"
        "insights:resource_type"
        "insights:resource_uid"
        "insights:resource_name"
        "insights:title"
        "insights:cvss"
    )
    
    # Verify all columns
    local all_columns=("${sbom_columns[@]}" "${component_columns[@]}" "${cve_match_columns[@]}" "${insight_columns[@]}")
    
    for col_spec in "${all_columns[@]}"; do
        IFS=':' read -r table column <<< "$col_spec"
        if check_column "$table" "$column" "$pod"; then
            log_success "Column '$table.$column' exists"
        else
            log_error "Column '$table.$column' is missing"
            all_passed=false
        fi
    done
    
    if [ "$all_passed" = true ]; then
        log_success "All critical columns exist"
        return 0
    else
        log_error "Some critical columns are missing"
        return 1
    fi
}

# Main
main() {
    echo ""
    echo "=========================================="
    echo "Database Schema Verification"
    echo "=========================================="
    echo ""
    
    # Get PostgreSQL pod
    POSTGRES_POD=$(get_postgres_pod)
    if [ -z "$POSTGRES_POD" ]; then
        log_error "PostgreSQL pod not found in namespace '$NAMESPACE'"
        exit 1
    fi
    
    log_info "Using PostgreSQL pod: $POSTGRES_POD"
    echo ""
    
    # Verify database connection
    log_info "Verifying database connection..."
    if kubectl exec -n "$NAMESPACE" "$POSTGRES_POD" -- psql -U "$DB_USER" -d "$DB_NAME" -c "SELECT 1;" >/dev/null 2>&1; then
        log_success "Database connection successful"
    else
        log_error "Failed to connect to database"
        exit 1
    fi
    echo ""
    
    # Verify tables
    if ! verify_tables "$POSTGRES_POD"; then
        log_error "Schema verification failed: Missing tables"
        exit 1
    fi
    echo ""
    
    # Verify columns
    if ! verify_columns "$POSTGRES_POD"; then
        log_error "Schema verification failed: Missing columns"
        exit 1
    fi
    echo ""
    
    log_success "Database schema verification completed successfully!"
    echo ""
}

main "$@"

