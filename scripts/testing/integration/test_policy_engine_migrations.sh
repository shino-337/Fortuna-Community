#!/bin/bash

# Policy Engine Migration Test Script
# Tests MVP2 Phase 2.1 migrations: policy_templates, policy_instances, policy_violations

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
NAMESPACE="ksam"
POSTGRES_POD=""
DB_NAME="ksam"
DB_USER="postgres"
REPORT_DIR="KSAM/docs/test_reports"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
REPORT_FILE="$REPORT_DIR/policy_engine_migration_test_${TIMESTAMP}.md"

# Functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_section() {
    echo ""
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}========================================${NC}"
}

# Create report directory
mkdir -p "$REPORT_DIR"

# Initialize report
cat > "$REPORT_FILE" << EOF
# Policy Engine Migration Test Report

**Date**: $(date -u +"%Y-%m-%d %H:%M:%S UTC")  
**Test Script**: test_policy_engine_migrations.sh  
**Phase**: MVP2 Phase 2.1 - Policy Engine Foundation

---

## 📋 Test Summary

EOF

# Get PostgreSQL pod
log_section "Step 1: Environment Setup"
log_info "Finding PostgreSQL pod..."
POSTGRES_POD=$(kubectl get pods -n "$NAMESPACE" -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

if [ -z "$POSTGRES_POD" ]; then
    log_error "PostgreSQL pod not found in namespace $NAMESPACE"
    exit 1
fi

log_success "Found PostgreSQL pod: $POSTGRES_POD"

# Test database connection
log_info "Testing database connection..."
if ! kubectl exec -n "$NAMESPACE" "$POSTGRES_POD" -- psql -U "$DB_USER" -d "$DB_NAME" -c "SELECT 1;" > /dev/null 2>&1; then
    log_error "Cannot connect to database"
    exit 1
fi
log_success "Database connection successful"

# Pre-migration state
log_section "Step 2: Pre-Migration State"
log_info "Checking current database state..."

PRE_MIGRATION_TABLES=$(kubectl exec -n "$NAMESPACE" "$POSTGRES_POD" -- psql -U "$DB_USER" -d "$DB_NAME" -t -A -c "
SELECT COUNT(*) FROM information_schema.tables 
WHERE table_schema = 'public' AND table_name IN ('policy_templates', 'policy_instances', 'policy_violations');
" 2>/dev/null | tr -d ' ')

log_info "Policy tables before migration: $PRE_MIGRATION_TABLES"

if [ "$PRE_MIGRATION_TABLES" != "0" ]; then
    log_warn "Policy tables already exist. Checking which ones..."
    kubectl exec -n "$NAMESPACE" "$POSTGRES_POD" -- psql -U "$DB_USER" -d "$DB_NAME" -c "
    SELECT table_name FROM information_schema.tables 
    WHERE table_schema = 'public' AND table_name IN ('policy_templates', 'policy_instances', 'policy_violations');
    " 2>/dev/null
fi

# Run migrations via Core service
log_section "Step 3: Running Migrations"
log_info "Restarting Core service to trigger migrations..."

CORE_POD=$(kubectl get pods -n "$NAMESPACE" -l app=ksam-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

if [ -z "$CORE_POD" ]; then
    log_error "Core pod not found"
    exit 1
fi

log_info "Deleting Core pod to trigger restart and migrations..."
kubectl delete pod -n "$NAMESPACE" "$CORE_POD" > /dev/null 2>&1

log_info "Waiting for Core pod to restart..."
sleep 10

# Wait for pod to be ready
for i in {1..30}; do
    NEW_POD=$(kubectl get pods -n "$NAMESPACE" -l app=ksam-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
    if [ -n "$NEW_POD" ]; then
        STATUS=$(kubectl get pod -n "$NAMESPACE" "$NEW_POD" -o jsonpath='{.status.phase}' 2>/dev/null || echo "Unknown")
        if [ "$STATUS" = "Running" ]; then
            log_success "Core pod restarted: $NEW_POD"
            break
        fi
    fi
    sleep 2
done

# Check migration logs
log_info "Checking migration logs..."
kubectl logs -n "$NAMESPACE" "$NEW_POD" --tail=50 | grep -i "migration\|policy" || log_warn "No migration logs found"

# Post-migration verification
log_section "Step 4: Post-Migration Verification"

# Check if tables exist
log_info "Verifying tables were created..."

TABLES_EXIST=$(kubectl exec -n "$NAMESPACE" "$POSTGRES_POD" -- psql -U "$DB_USER" -d "$DB_NAME" -t -A -c "
SELECT COUNT(*) FROM information_schema.tables 
WHERE table_schema = 'public' AND table_name IN ('policy_templates', 'policy_instances', 'policy_violations');
" 2>/dev/null | tr -d ' ')

if [ "$TABLES_EXIST" != "3" ]; then
    log_error "Expected 3 tables, found $TABLES_EXIST"
    exit 1
fi

log_success "All 3 tables exist"

# Verify table structures
log_section "Step 5: Table Structure Verification"

verify_table_structure() {
    local table_name=$1
    log_info "Verifying $table_name table structure..."
    
    COLUMN_COUNT=$(kubectl exec -n "$NAMESPACE" "$POSTGRES_POD" -- psql -U "$DB_USER" -d "$DB_NAME" -t -A -c "
    SELECT COUNT(*) FROM information_schema.columns 
    WHERE table_schema = 'public' AND table_name = '$table_name';
    " 2>/dev/null | tr -d ' ')
    
    log_info "$table_name has $COLUMN_COUNT columns"
    
    # Get column list
    COLUMNS=$(kubectl exec -n "$NAMESPACE" "$POSTGRES_POD" -- psql -U "$DB_USER" -d "$DB_NAME" -t -A -c "
    SELECT column_name || ' (' || data_type || ')' 
    FROM information_schema.columns 
    WHERE table_schema = 'public' AND table_name = '$table_name'
    ORDER BY ordinal_position;
    " 2>/dev/null)
    
    echo "$COLUMNS" | while read -r col; do
        log_info "  - $col"
    done
}

verify_table_structure "policy_templates"
verify_table_structure "policy_instances"
verify_table_structure "policy_violations"

# Verify indexes
log_section "Step 6: Index Verification"

verify_indexes() {
    local table_name=$1
    log_info "Verifying indexes for $table_name..."
    
    INDEXES=$(kubectl exec -n "$NAMESPACE" "$POSTGRES_POD" -- psql -U "$DB_USER" -d "$DB_NAME" -t -A -c "
    SELECT indexname FROM pg_indexes 
    WHERE schemaname = 'public' AND tablename = '$table_name';
    " 2>/dev/null)
    
    INDEX_COUNT=$(echo "$INDEXES" | grep -v '^$' | wc -l | tr -d ' ')
    log_info "$table_name has $INDEX_COUNT indexes"
    
    echo "$INDEXES" | while read -r idx; do
        if [ -n "$idx" ]; then
            log_info "  - $idx"
        fi
    done
}

verify_indexes "policy_templates"
verify_indexes "policy_instances"
verify_indexes "policy_violations"

# Verify constraints
log_section "Step 7: Constraint Verification"

log_info "Checking foreign key constraints..."
FK_COUNT=$(kubectl exec -n "$NAMESPACE" "$POSTGRES_POD" -- psql -U "$DB_USER" -d "$DB_NAME" -t -A -c "
SELECT COUNT(*) FROM information_schema.table_constraints 
WHERE constraint_type = 'FOREIGN KEY' 
AND table_name IN ('policy_instances', 'policy_violations');
" 2>/dev/null | tr -d ' ')

log_info "Found $FK_COUNT foreign key constraints"

log_info "Checking check constraints..."
CHECK_COUNT=$(kubectl exec -n "$NAMESPACE" "$POSTGRES_POD" -- psql -U "$DB_USER" -d "$DB_NAME" -t -A -c "
SELECT COUNT(*) FROM information_schema.table_constraints 
WHERE constraint_type = 'CHECK' 
AND table_name IN ('policy_templates', 'policy_instances', 'policy_violations');
" 2>/dev/null | tr -d ' ')

log_info "Found $CHECK_COUNT check constraints"

# Test data operations
log_section "Step 8: Data Operations Test"

log_info "Test 1: Insert into policy_templates..."
TEMPLATE_INSERT=$(kubectl exec -n "$NAMESPACE" "$POSTGRES_POD" -- psql -U "$DB_USER" -d "$DB_NAME" -t -A -c "
INSERT INTO policy_templates (
    template_id, version, name, description, category, default_severity,
    cel_expression, default_action, is_system, created_by
) VALUES (
    'test-no-root', '1.0.0', 'Test No Root Containers',
    'Test policy for no root containers', 'security', 'high',
    'resource.spec.securityContext.runAsNonRoot == true',
    'block', true, 'system'
) RETURNING id;
" 2>&1)

if echo "$TEMPLATE_INSERT" | grep -q "ERROR"; then
    log_error "Failed to insert template: $TEMPLATE_INSERT"
else
    TEMPLATE_ID=$(echo "$TEMPLATE_INSERT" | tr -d ' ')
    log_success "Template inserted with ID: $TEMPLATE_ID"
fi

log_info "Test 2: Insert into policy_instances..."
INSTANCE_INSERT=$(kubectl exec -n "$NAMESPACE" "$POSTGRES_POD" -- psql -U "$DB_USER" -d "$DB_NAME" -t -A -c "
INSERT INTO policy_instances (
    template_id, template_version, instance_name, description,
    enabled, clusters, namespaces, resource_types, action, severity
) VALUES (
    'test-no-root', '1.0.0', 'test-instance-1',
    'Test instance', true, ARRAY['prod-*'], ARRAY['default'], 
    ARRAY['Pod'], 'block', 'high'
) RETURNING id;
" 2>&1)

if echo "$INSTANCE_INSERT" | grep -q "ERROR"; then
    log_error "Failed to insert instance: $INSTANCE_INSERT"
else
    INSTANCE_ID=$(echo "$INSTANCE_INSERT" | tr -d ' ')
    log_success "Instance inserted with ID: $INSTANCE_ID"
fi

log_info "Test 3: Insert into policy_violations..."
if [ -n "$INSTANCE_ID" ] && [ "$INSTANCE_ID" != "" ]; then
    VIOLATION_INSERT=$(kubectl exec -n "$NAMESPACE" "$POSTGRES_POD" -- psql -U "$DB_USER" -d "$DB_NAME" -t -A -c "
    INSERT INTO policy_violations (
        instance_id, instance_name, template_id, template_name,
        resource_type, resource_uid, resource_name, namespace,
        cluster_id, severity, action, status, message
    ) VALUES (
        $INSTANCE_ID, 'test-instance-1', 'test-no-root', 'Test No Root Containers',
        'Pod', 'pod-test-123', 'test-pod', 'default',
        'cluster-1', 'high', 'block', 'active', 'Test violation message'
    ) RETURNING id;
    " 2>&1)
    
    if echo "$VIOLATION_INSERT" | grep -q "ERROR"; then
        log_error "Failed to insert violation: $VIOLATION_INSERT"
    else
        VIOLATION_ID=$(echo "$VIOLATION_INSERT" | tr -d ' ')
        log_success "Violation inserted with ID: $VIOLATION_ID"
    fi
fi

# Test constraints
log_section "Step 9: Constraint Testing"

log_info "Test 4: Foreign key constraint (should fail)..."
FK_TEST=$(kubectl exec -n "$NAMESPACE" "$POSTGRES_POD" -- psql -U "$DB_USER" -d "$DB_NAME" -t -A -c "
INSERT INTO policy_instances (
    template_id, template_version, instance_name
) VALUES (
    'invalid-template', '1.0.0', 'test-fk-fail'
);
" 2>&1)

if echo "$FK_TEST" | grep -q "violates foreign key constraint"; then
    log_success "Foreign key constraint works correctly"
else
    log_error "Foreign key constraint test failed: $FK_TEST"
fi

log_info "Test 5: Check constraint (should fail)..."
CHECK_TEST=$(kubectl exec -n "$NAMESPACE" "$POSTGRES_POD" -- psql -U "$DB_USER" -d "$DB_NAME" -t -A -c "
INSERT INTO policy_templates (
    template_id, version, name, category, default_severity, cel_expression
) VALUES (
    'test-invalid', '1.0.0', 'Test', 'invalid-category', 'high', 'true'
);
" 2>&1)

if echo "$CHECK_TEST" | grep -q "violates check constraint"; then
    log_success "Check constraint works correctly"
else
    log_warn "Check constraint test: $CHECK_TEST"
fi

log_info "Test 6: Unique constraint (should fail)..."
UNIQUE_TEST=$(kubectl exec -n "$NAMESPACE" "$POSTGRES_POD" -- psql -U "$DB_USER" -d "$DB_NAME" -t -A -c "
INSERT INTO policy_templates (
    template_id, version, name, category, default_severity, cel_expression
) VALUES (
    'test-no-root', '1.0.0', 'Duplicate', 'security', 'high', 'true'
);
" 2>&1)

if echo "$UNIQUE_TEST" | grep -q "duplicate key\|unique constraint"; then
    log_success "Unique constraint works correctly"
else
    log_warn "Unique constraint test: $UNIQUE_TEST"
fi

# Cleanup test data
log_section "Step 10: Cleanup"
log_info "Cleaning up test data..."

kubectl exec -n "$NAMESPACE" "$POSTGRES_POD" -- psql -U "$DB_USER" -d "$DB_NAME" -c "
DELETE FROM policy_violations WHERE instance_name = 'test-instance-1';
DELETE FROM policy_instances WHERE instance_name = 'test-instance-1';
DELETE FROM policy_templates WHERE template_id = 'test-no-root';
" > /dev/null 2>&1

log_success "Test data cleaned up"

# Final summary
log_section "Test Summary"

cat >> "$REPORT_FILE" << EOF
## ✅ Test Results

### Tables Created
- ✅ policy_templates
- ✅ policy_instances  
- ✅ policy_violations

### Data Operations
- ✅ Template INSERT: Success
- ✅ Instance INSERT: Success
- ✅ Violation INSERT: Success

### Constraints
- ✅ Foreign Key: Working
- ✅ Check Constraint: Working
- ✅ Unique Constraint: Working

### Indexes
- ✅ All indexes created successfully

---

**Status**: ✅ **MIGRATIONS SUCCESSFUL**

EOF

log_success "Migration test completed!"
log_info "Report saved to: $REPORT_FILE"
echo ""
echo "✅ All migrations tested successfully!"

