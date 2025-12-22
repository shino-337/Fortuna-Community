#!/bin/bash

# Enhanced E2E Test with API Verification
# Tests MVP-1 features and verifies Dashboard APIs match database

set -e

NAMESPACE="ksam"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
REPORT_DIR="test_results/mvp1_e2e_api_${TIMESTAMP}"
mkdir -p "$REPORT_DIR"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1" | tee -a "$REPORT_DIR/test.log"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1" | tee -a "$REPORT_DIR/test.log"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1" | tee -a "$REPORT_DIR/test.log"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1" | tee -a "$REPORT_DIR/test.log"
}

log_section() {
    echo "" | tee -a "$REPORT_DIR/test.log"
    echo "==========================================" | tee -a "$REPORT_DIR/test.log"
    echo "$1" | tee -a "$REPORT_DIR/test.log"
    echo "==========================================" | tee -a "$REPORT_DIR/test.log"
}

# Test configuration (valid K8s names)
TIMESTAMP_CLEAN=$(date +%s)
TEST_NAMESPACE="e2e-test-${TIMESTAMP_CLEAN}"
TEST_POD_NAME="test-pod-${TIMESTAMP_CLEAN}"
TEST_SA_NAME="test-sa-${TIMESTAMP_CLEAN}"
TEST_ROLE_NAME="test-role-${TIMESTAMP_CLEAN}"
TEST_RB_NAME="test-rb-${TIMESTAMP_CLEAN}"

# Port forwarding
CORE_PORT=8080
CORE_PF_PID=""

# Cleanup
cleanup() {
    log_info "Cleaning up..."
    if [ ! -z "$CORE_PF_PID" ]; then
        kill $CORE_PF_PID 2>/dev/null || true
    fi
    kubectl delete namespace "$TEST_NAMESPACE" --ignore-not-found=true 2>/dev/null || true
}

trap cleanup EXIT

# Helper functions
get_postgres_pod() {
    kubectl get pods -n "$NAMESPACE" -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null
}

get_core_pod() {
    kubectl get pods -n "$NAMESPACE" -l app=ksam-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null
}

exec_psql() {
    local pod=$1
    shift
    kubectl exec -n "$NAMESPACE" "$pod" -- psql -U postgres -d ksam "$@" 2>&1
}

get_auth_token() {
    local response=$(curl -s -X POST "http://localhost:${CORE_PORT}/api/v1/auth/login" \
        -H "Content-Type: application/json" \
        -d '{"username":"admin","password":"admin123"}' 2>&1)
    echo "$response" | grep -o '"token":"[^"]*' | cut -d'"' -f4 || echo ""
}

call_api() {
    local endpoint=$1
    curl -s "http://localhost:${CORE_PORT}${endpoint}" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $(get_auth_token)" 2>&1
}

# ==========================================
# PHASE 1: PRE-TEST STATE
# ==========================================

capture_pre_test_state() {
    log_section "Phase 1: Pre-Test State Capture"
    
    POSTGRES_POD=$(get_postgres_pod)
    CORE_POD=$(get_core_pod)
    
    # Setup port-forward
    kubectl port-forward -n "$NAMESPACE" "$CORE_POD" ${CORE_PORT}:8080 > /dev/null 2>&1 &
    CORE_PF_PID=$!
    sleep 3
    
    # Capture Database state
    DB_PODS_BEFORE=$(exec_psql "$POSTGRES_POD" -t -c "SELECT COUNT(*) FROM pods WHERE deleted_at IS NULL;" 2>&1 | tr -d ' ')
    DB_SAS_BEFORE=$(exec_psql "$POSTGRES_POD" -t -c "SELECT COUNT(*) FROM service_accounts WHERE deleted_at IS NULL;" 2>&1 | tr -d ' ')
    DB_ROLES_BEFORE=$(exec_psql "$POSTGRES_POD" -t -c "SELECT COUNT(*) FROM roles WHERE deleted_at IS NULL;" 2>&1 | tr -d ' ')
    DB_INSIGHTS_BEFORE=$(exec_psql "$POSTGRES_POD" -t -c "SELECT COUNT(*) FROM insights WHERE deleted_at IS NULL AND (status = 'active' OR status IS NULL);" 2>&1 | tr -d ' ')
    
    log_info "Database Before: Pods=$DB_PODS_BEFORE, SAs=$DB_SAS_BEFORE, Roles=$DB_ROLES_BEFORE, Insights=$DB_INSIGHTS_BEFORE"
    
    # Capture API state
    TOKEN=$(get_auth_token)
    if [ ! -z "$TOKEN" ]; then
        API_INSIGHTS_BEFORE=$(call_api "/api/v1/insights/summary" | grep -o '"total":[0-9]*' | cut -d':' -f2 || echo "0")
        log_info "API Insights Before: $API_INSIGHTS_BEFORE"
    fi
    
    log_success "Pre-test state captured"
}

# ==========================================
# PHASE 2: CREATE TEST RESOURCES
# ==========================================

create_test_resources() {
    log_section "Phase 2: Creating Test Resources"
    
    kubectl create namespace "$TEST_NAMESPACE" 2>&1 | tee -a "$REPORT_DIR/test.log"
    sleep 2
    
    kubectl create serviceaccount "$TEST_SA_NAME" -n "$TEST_NAMESPACE" 2>&1 | tee -a "$REPORT_DIR/test.log"
    
    cat <<EOF | kubectl apply -f - 2>&1 | tee -a "$REPORT_DIR/test.log"
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: $TEST_ROLE_NAME
  namespace: $TEST_NAMESPACE
rules:
- apiGroups: ["*"]
  resources: ["*"]
  verbs: ["*"]
EOF
    
    cat <<EOF | kubectl apply -f - 2>&1 | tee -a "$REPORT_DIR/test.log"
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: $TEST_RB_NAME
  namespace: $TEST_NAMESPACE
subjects:
- kind: ServiceAccount
  name: $TEST_SA_NAME
  namespace: $TEST_NAMESPACE
roleRef:
  kind: Role
  name: $TEST_ROLE_NAME
  apiGroup: rbac.authorization.k8s.io/v1
EOF
    
    cat <<EOF | kubectl apply -f - 2>&1 | tee -a "$REPORT_DIR/test.log"
apiVersion: v1
kind: Pod
metadata:
  name: $TEST_POD_NAME
  namespace: $TEST_NAMESPACE
spec:
  serviceAccountName: $TEST_SA_NAME
  containers:
  - name: test-container
    image: nginx:alpine
    ports:
    - containerPort: 80
EOF
    
    log_info "Waiting for Agent to collect resources..."
    sleep 30
    
    log_success "Test resources created"
}

# ==========================================
# PHASE 3: VERIFY DATABASE STORAGE
# ==========================================

verify_database_storage() {
    log_section "Phase 3: Verifying Database Storage"
    
    POSTGRES_POD=$(get_postgres_pod)
    
    # Check resources in database
    DB_POD=$(exec_psql "$POSTGRES_POD" -t -c "
        SELECT COUNT(*) FROM pods 
        WHERE name = '$TEST_POD_NAME' 
        AND namespace = '$TEST_NAMESPACE' 
        AND deleted_at IS NULL;
    " 2>&1 | tr -d ' ')
    
    DB_SA=$(exec_psql "$POSTGRES_POD" -t -c "
        SELECT COUNT(*) FROM service_accounts 
        WHERE name = '$TEST_SA_NAME' 
        AND namespace = '$TEST_NAMESPACE' 
        AND deleted_at IS NULL;
    " 2>&1 | tr -d ' ')
    
    DB_ROLE=$(exec_psql "$POSTGRES_POD" -t -c "
        SELECT COUNT(*) FROM roles 
        WHERE name = '$TEST_ROLE_NAME' 
        AND namespace = '$TEST_NAMESPACE' 
        AND deleted_at IS NULL;
    " 2>&1 | tr -d ' ')
    
    if [ "$DB_POD" = "1" ]; then
        log_success "✅ Pod in database"
    else
        log_error "❌ Pod not in database (count: $DB_POD)"
    fi
    
    if [ "$DB_SA" = "1" ]; then
        log_success "✅ ServiceAccount in database"
    else
        log_error "❌ ServiceAccount not in database (count: $DB_SA)"
    fi
    
    if [ "$DB_ROLE" = "1" ]; then
        log_success "✅ Role in database"
    else
        log_error "❌ Role not in database (count: $DB_ROLE)"
    fi
}

# ==========================================
# PHASE 4: VERIFY API ENDPOINTS
# ==========================================

verify_api_endpoints() {
    log_section "Phase 4: Verifying API Endpoints vs Database"
    
    POSTGRES_POD=$(get_postgres_pod)
    
    # Verify Insights Summary API
    log_info "Verifying Insights Summary API..."
    DB_INSIGHTS_TOTAL=$(exec_psql "$POSTGRES_POD" -t -c "
        SELECT COUNT(*) FROM insights 
        WHERE deleted_at IS NULL 
        AND (status = 'active' OR status IS NULL);
    " 2>&1 | tr -d ' ')
    
    API_RESPONSE=$(call_api "/api/v1/insights/summary")
    echo "$API_RESPONSE" > "$REPORT_DIR/api_insights_summary.json"
    
    if command -v jq &> /dev/null; then
        API_INSIGHTS_TOTAL=$(echo "$API_RESPONSE" | jq -r '.total // 0' 2>/dev/null || echo "0")
    else
        API_INSIGHTS_TOTAL=$(echo "$API_RESPONSE" | grep -o '"total":[0-9]*' | cut -d':' -f2 || echo "0")
    fi
    
    if [ "$DB_INSIGHTS_TOTAL" = "$API_INSIGHTS_TOTAL" ]; then
        log_success "✅ Insights Summary: DB ($DB_INSIGHTS_TOTAL) = API ($API_INSIGHTS_TOTAL)"
    else
        log_error "❌ Insights Summary mismatch: DB ($DB_INSIGHTS_TOTAL) vs API ($API_INSIGHTS_TOTAL)"
    fi
    
    # Verify Insights List API
    log_info "Verifying Insights List API..."
    API_RESPONSE=$(call_api "/api/v1/insights?pageSize=1")
    echo "$API_RESPONSE" > "$REPORT_DIR/api_insights_list.json"
    
    if command -v jq &> /dev/null; then
        API_INSIGHTS_LIST_TOTAL=$(echo "$API_RESPONSE" | jq -r '.total // 0' 2>/dev/null || echo "0")
    else
        API_INSIGHTS_LIST_TOTAL=$(echo "$API_RESPONSE" | grep -o '"total":[0-9]*' | cut -d':' -f2 || echo "0")
    fi
    
    if [ "$DB_INSIGHTS_TOTAL" = "$API_INSIGHTS_LIST_TOTAL" ]; then
        log_success "✅ Insights List: DB ($DB_INSIGHTS_TOTAL) = API ($API_INSIGHTS_LIST_TOTAL)"
    else
        log_error "❌ Insights List mismatch: DB ($DB_INSIGHTS_TOTAL) vs API ($API_INSIGHTS_LIST_TOTAL)"
    fi
    
    # Verify ServiceAccounts API
    log_info "Verifying ServiceAccounts API..."
    DB_SA_TOTAL=$(exec_psql "$POSTGRES_POD" -t -c "
        SELECT COUNT(*) FROM service_accounts 
        WHERE deleted_at IS NULL;
    " 2>&1 | tr -d ' ')
    
    API_RESPONSE=$(call_api "/api/v1/serviceaccounts?pageSize=1")
    echo "$API_RESPONSE" > "$REPORT_DIR/api_serviceaccounts.json"
    
    if command -v jq &> /dev/null; then
        API_SA_TOTAL=$(echo "$API_RESPONSE" | jq -r '.total // 0' 2>/dev/null || echo "0")
    else
        API_SA_TOTAL=$(echo "$API_RESPONSE" | grep -o '"total":[0-9]*' | cut -d':' -f2 || echo "0")
    fi
    
    if [ "$DB_SA_TOTAL" = "$API_SA_TOTAL" ]; then
        log_success "✅ ServiceAccounts: DB ($DB_SA_TOTAL) = API ($API_SA_TOTAL)"
    else
        log_error "❌ ServiceAccounts mismatch: DB ($DB_SA_TOTAL) vs API ($API_SA_TOTAL)"
    fi
    
    # Verify Pods API
    log_info "Verifying Pods API..."
    DB_POD_TOTAL=$(exec_psql "$POSTGRES_POD" -t -c "
        SELECT COUNT(*) FROM pods 
        WHERE deleted_at IS NULL;
    " 2>&1 | tr -d ' ')
    
    API_RESPONSE=$(call_api "/api/v1/pods?pageSize=1")
    echo "$API_RESPONSE" > "$REPORT_DIR/api_pods.json"
    
    if command -v jq &> /dev/null; then
        API_POD_TOTAL=$(echo "$API_RESPONSE" | jq -r '.total // 0' 2>/dev/null || echo "0")
    else
        API_POD_TOTAL=$(echo "$API_RESPONSE" | grep -o '"total":[0-9]*' | cut -d':' -f2 || echo "0")
    fi
    
    if [ "$DB_POD_TOTAL" = "$API_POD_TOTAL" ]; then
        log_success "✅ Pods: DB ($DB_POD_TOTAL) = API ($API_POD_TOTAL)"
    else
        log_error "❌ Pods mismatch: DB ($DB_POD_TOTAL) vs API ($API_POD_TOTAL)"
    fi
    
    # Verify test resources via API
    log_info "Verifying test resources via API..."
    API_RESPONSE=$(call_api "/api/v1/serviceaccounts?namespace=$TEST_NAMESPACE")
    if echo "$API_RESPONSE" | grep -q "$TEST_SA_NAME"; then
        log_success "✅ Test ServiceAccount found via API"
    else
        log_warning "⚠️  Test ServiceAccount not found via API"
    fi
    
    API_RESPONSE=$(call_api "/api/v1/pods?namespace=$TEST_NAMESPACE")
    if echo "$API_RESPONSE" | grep -q "$TEST_POD_NAME"; then
        log_success "✅ Test Pod found via API"
    else
        log_warning "⚠️  Test Pod not found via API"
    fi
}

# ==========================================
# PHASE 5: VERIFY RISK DETECTION
# ==========================================

verify_risk_detection() {
    log_section "Phase 5: Verifying Risk Detection"
    
    POSTGRES_POD=$(get_postgres_pod)
    
    # Trigger risk evaluation
    log_info "Triggering risk evaluation..."
    RESPONSE=$(call_api "/api/v1/insights/evaluate")
    echo "$RESPONSE" > "$REPORT_DIR/risk_evaluation_response.json"
    
    log_info "Waiting for risk evaluation..."
    sleep 15
    
    # Check for insights
    DB_INSIGHT=$(exec_psql "$POSTGRES_POD" -t -c "
        SELECT COUNT(*) FROM insights 
        WHERE deleted_at IS NULL 
        AND (status = 'active' OR status IS NULL)
        AND (
            description LIKE '%$TEST_ROLE_NAME%' 
            OR description LIKE '%wildcard%'
        );
    " 2>&1 | tr -d ' ')
    
    if [ "$DB_INSIGHT" -gt 0 ]; then
        log_success "✅ Risk detection working: Found $DB_INSIGHT insight(s)"
    else
        log_warning "⚠️  No insights found for test resources"
    fi
}

# ==========================================
# PHASE 6: FINAL VERIFICATION
# ==========================================

final_verification() {
    log_section "Phase 6: Final API vs Database Verification"
    
    POSTGRES_POD=$(get_postgres_pod)
    
    # Comprehensive API verification
    log_info "Running comprehensive API verification..."
    
    # Insights Summary
    DB_TOTAL=$(exec_psql "$POSTGRES_POD" -t -c "
        SELECT COUNT(*) FROM insights 
        WHERE deleted_at IS NULL 
        AND (status = 'active' OR status IS NULL);
    " 2>&1 | tr -d ' ')
    
    API_RESPONSE=$(call_api "/api/v1/insights/summary")
    if command -v jq &> /dev/null; then
        API_TOTAL=$(echo "$API_RESPONSE" | jq -r '.total // 0' 2>/dev/null || echo "0")
    else
        API_TOTAL=$(echo "$API_RESPONSE" | grep -o '"total":[0-9]*' | cut -d':' -f2 || echo "0")
    fi
    
    log_info "Final Verification:"
    log_info "  Database Insights: $DB_TOTAL"
    log_info "  API Insights Summary: $API_TOTAL"
    
    if [ "$DB_TOTAL" = "$API_TOTAL" ]; then
        log_success "✅ FINAL: Insights count matches perfectly"
    else
        log_error "❌ FINAL: Insights count mismatch"
    fi
    
    # ServiceAccounts
    DB_SA=$(exec_psql "$POSTGRES_POD" -t -c "
        SELECT COUNT(*) FROM service_accounts 
        WHERE deleted_at IS NULL;
    " 2>&1 | tr -d ' ')
    
    API_RESPONSE=$(call_api "/api/v1/serviceaccounts?pageSize=1")
    if command -v jq &> /dev/null; then
        API_SA=$(echo "$API_RESPONSE" | jq -r '.total // 0' 2>/dev/null || echo "0")
    else
        API_SA=$(echo "$API_RESPONSE" | grep -o '"total":[0-9]*' | cut -d':' -f2 || echo "0")
    fi
    
    if [ "$DB_SA" = "$API_SA" ]; then
        log_success "✅ FINAL: ServiceAccounts count matches"
    else
        log_error "❌ FINAL: ServiceAccounts count mismatch"
    fi
    
    # Pods
    DB_POD=$(exec_psql "$POSTGRES_POD" -t -c "
        SELECT COUNT(*) FROM pods 
        WHERE deleted_at IS NULL;
    " 2>&1 | tr -d ' ')
    
    API_RESPONSE=$(call_api "/api/v1/pods?pageSize=1")
    if command -v jq &> /dev/null; then
        API_POD=$(echo "$API_RESPONSE" | jq -r '.total // 0' 2>/dev/null || echo "0")
    else
        API_POD=$(echo "$API_RESPONSE" | grep -o '"total":[0-9]*' | cut -d':' -f2 || echo "0")
    fi
    
    if [ "$DB_POD" = "$API_POD" ]; then
        log_success "✅ FINAL: Pods count matches"
    else
        log_error "❌ FINAL: Pods count mismatch"
    fi
}

# ==========================================
# GENERATE REPORT
# ==========================================

generate_report() {
    log_section "Generating Final Report"
    
    cat > "$REPORT_DIR/MVP1_E2E_API_VERIFICATION_REPORT.md" <<EOF
# MVP-1 E2E Test with API Verification Report

**Date**: $(date)
**Test Namespace**: $TEST_NAMESPACE

## Summary

This test verifies:
1. Agent collection and forwarding
2. Core processing (Normalizer, Correlator, Risk Engine)
3. Database storage
4. API endpoints return correct data
5. Dashboard APIs match database

## Test Results

See test.log for detailed results.

## API Verification

All Dashboard APIs have been verified against database:
- Insights Summary API
- Insights List API
- ServiceAccounts API
- Pods API

## Files Generated

- \`test.log\`: Full test log
- \`api_*.json\`: API responses
- \`risk_evaluation_response.json\`: Risk evaluation response

EOF
    
    log_success "Report generated"
}

# ==========================================
# MAIN
# ==========================================

main() {
    log_section "MVP-1 E2E Test with API Verification"
    echo "Starting test at $(date)"
    echo ""
    
    capture_pre_test_state
    create_test_resources
    verify_database_storage
    verify_api_endpoints
    verify_risk_detection
    final_verification
    generate_report
    
    log_section "Test Complete"
    echo "Test finished at $(date)"
    echo ""
    echo "Report saved to: $REPORT_DIR/"
}

main "$@"

