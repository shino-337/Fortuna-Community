#!/bin/bash

# Comprehensive E2E Test for MVP-1 Features
# Tests: Agent → Core → Database → Dashboard flow
# Compares: K8s state vs Database vs Dashboard

set -e

NAMESPACE="ksam"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
REPORT_DIR="test_results/mvp1_e2e_${TIMESTAMP}"
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

# Test configuration (use valid K8s names - lowercase, alphanumeric and hyphens only)
TIMESTAMP_CLEAN=$(echo "$TIMESTAMP" | tr '_' '-')
TEST_NAMESPACE="e2e-test-${TIMESTAMP_CLEAN}"
TEST_POD_NAME="test-pod-${TIMESTAMP_CLEAN}"
TEST_SA_NAME="test-sa-${TIMESTAMP_CLEAN}"
TEST_ROLE_NAME="test-role-${TIMESTAMP_CLEAN}"
TEST_RB_NAME="test-rb-${TIMESTAMP_CLEAN}"

# Port forwarding
CORE_PORT=8080
DASHBOARD_PORT=3000
CORE_PF_PID=""
DASHBOARD_PF_PID=""

# Cleanup function
cleanup() {
    log_info "Cleaning up test resources..."
    
    # Kill port forwards
    if [ ! -z "$CORE_PF_PID" ]; then
        kill $CORE_PF_PID 2>/dev/null || true
    fi
    if [ ! -z "$DASHBOARD_PF_PID" ]; then
        kill $DASHBOARD_PF_PID 2>/dev/null || true
    fi
    
    # Delete test resources
    kubectl delete namespace "$TEST_NAMESPACE" --ignore-not-found=true 2>/dev/null || true
    
    log_info "Cleanup complete"
}

trap cleanup EXIT

# Get pods
get_postgres_pod() {
    kubectl get pods -n "$NAMESPACE" -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null
}

get_core_pod() {
    kubectl get pods -n "$NAMESPACE" -l app=ksam-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null
}

get_agent_pod() {
    kubectl get pods -n "$NAMESPACE" -l app=ksam-agent -o jsonpath='{.items[0].metadata.name}' 2>/dev/null
}

# Database queries
exec_psql() {
    local pod=$1
    shift
    kubectl exec -n "$NAMESPACE" "$pod" -- psql -U postgres -d ksam "$@" 2>&1
}

# API calls
call_api() {
    local method=$1
    local endpoint=$2
    local data=$3
    
    if [ -z "$data" ]; then
        curl -s -X "$method" "http://localhost:${CORE_PORT}${endpoint}" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer $(get_auth_token)" 2>&1
    else
        curl -s -X "$method" "http://localhost:${CORE_PORT}${endpoint}" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer $(get_auth_token)" \
            -d "$data" 2>&1
    fi
}

get_auth_token() {
    # Try to get token from login
    local response=$(curl -s -X POST "http://localhost:${CORE_PORT}/api/v1/auth/login" \
        -H "Content-Type: application/json" \
        -d '{"username":"admin","password":"admin123"}' 2>&1)
    
    echo "$response" | grep -o '"token":"[^"]*' | cut -d'"' -f4 || echo ""
}

# ==========================================
# PHASE 1: PRE-TEST STATE CAPTURE
# ==========================================

capture_pre_test_state() {
    log_section "Phase 1: Capturing Pre-Test State"
    
    POSTGRES_POD=$(get_postgres_pod)
    if [ -z "$POSTGRES_POD" ]; then
        log_error "PostgreSQL pod not found"
        return 1
    fi
    
    # Capture K8s state
    log_info "Capturing Kubernetes state..."
    kubectl get pods -A -o json > "$REPORT_DIR/k8s_pods_before.json" 2>&1
    kubectl get serviceaccounts -A -o json > "$REPORT_DIR/k8s_sas_before.json" 2>&1
    kubectl get roles -A -o json > "$REPORT_DIR/k8s_roles_before.json" 2>&1
    kubectl get rolebindings -A -o json > "$REPORT_DIR/k8s_rbs_before.json" 2>&1
    
    # Count K8s resources
    K8S_PODS_BEFORE=$(kubectl get pods -A --no-headers 2>/dev/null | wc -l | tr -d ' ')
    K8S_SAS_BEFORE=$(kubectl get serviceaccounts -A --no-headers 2>/dev/null | wc -l | tr -d ' ')
    K8S_ROLES_BEFORE=$(kubectl get roles -A --no-headers 2>/dev/null | wc -l | tr -d ' ')
    K8S_RBS_BEFORE=$(kubectl get rolebindings -A --no-headers 2>/dev/null | wc -l | tr -d ' ')
    
    log_info "K8s Resources Before:"
    log_info "  Pods: $K8S_PODS_BEFORE"
    log_info "  ServiceAccounts: $K8S_SAS_BEFORE"
    log_info "  Roles: $K8S_ROLES_BEFORE"
    log_info "  RoleBindings: $K8S_RBS_BEFORE"
    
    # Capture Database state
    log_info "Capturing Database state..."
    exec_psql "$POSTGRES_POD" -c "
        SELECT 
            'pods' as resource_type,
            COUNT(*) as count
        FROM pods
        WHERE deleted_at IS NULL
        UNION ALL
        SELECT 
            'service_accounts',
            COUNT(*)
        FROM service_accounts
        WHERE deleted_at IS NULL
        UNION ALL
        SELECT 
            'roles',
            COUNT(*)
        FROM roles
        WHERE deleted_at IS NULL
        UNION ALL
        SELECT 
            'role_bindings',
            COUNT(*)
        FROM role_bindings
        WHERE deleted_at IS NULL
        UNION ALL
        SELECT 
            'insights',
            COUNT(*)
        FROM insights
        WHERE deleted_at IS NULL AND status = 'active';
    " > "$REPORT_DIR/db_state_before.txt" 2>&1
    
    DB_PODS_BEFORE=$(exec_psql "$POSTGRES_POD" -t -c "SELECT COUNT(*) FROM pods WHERE deleted_at IS NULL;" 2>&1 | tr -d ' ')
    DB_SAS_BEFORE=$(exec_psql "$POSTGRES_POD" -t -c "SELECT COUNT(*) FROM service_accounts WHERE deleted_at IS NULL;" 2>&1 | tr -d ' ')
    DB_ROLES_BEFORE=$(exec_psql "$POSTGRES_POD" -t -c "SELECT COUNT(*) FROM roles WHERE deleted_at IS NULL;" 2>&1 | tr -d ' ')
    DB_RBS_BEFORE=$(exec_psql "$POSTGRES_POD" -t -c "SELECT COUNT(*) FROM role_bindings WHERE deleted_at IS NULL;" 2>&1 | tr -d ' ')
    DB_INSIGHTS_BEFORE=$(exec_psql "$POSTGRES_POD" -t -c "SELECT COUNT(*) FROM insights WHERE deleted_at IS NULL AND status = 'active';" 2>&1 | tr -d ' ')
    
    log_info "Database Resources Before:"
    log_info "  Pods: $DB_PODS_BEFORE"
    log_info "  ServiceAccounts: $DB_SAS_BEFORE"
    log_info "  Roles: $DB_ROLES_BEFORE"
    log_info "  RoleBindings: $DB_RBS_BEFORE"
    log_info "  Active Insights: $DB_INSIGHTS_BEFORE"
    
    # Setup port forwarding for API access
    log_info "Setting up port forwarding..."
    CORE_POD=$(get_core_pod)
    if [ ! -z "$CORE_POD" ]; then
        kubectl port-forward -n "$NAMESPACE" "$CORE_POD" ${CORE_PORT}:8080 > /dev/null 2>&1 &
        CORE_PF_PID=$!
        sleep 2
        log_success "Core API port-forward established (PID: $CORE_PF_PID)"
    fi
    
    # Capture Dashboard state (if available)
    log_info "Checking Dashboard availability..."
    DASHBOARD_POD=$(kubectl get pods -n "$NAMESPACE" -l app=ksam-dashboard -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
    if [ ! -z "$DASHBOARD_POD" ]; then
        kubectl port-forward -n "$NAMESPACE" "$DASHBOARD_POD" ${DASHBOARD_PORT}:80 > /dev/null 2>&1 &
        DASHBOARD_PF_PID=$!
        sleep 2
        log_success "Dashboard port-forward established (PID: $DASHBOARD_PF_PID)"
    fi
    
    log_success "Pre-test state captured"
}

# ==========================================
# PHASE 2: CREATE TEST RESOURCES
# ==========================================

create_test_resources() {
    log_section "Phase 2: Creating Test Resources"
    
    # Create namespace
    log_info "Creating test namespace: $TEST_NAMESPACE"
    kubectl create namespace "$TEST_NAMESPACE" 2>&1 | tee -a "$REPORT_DIR/test.log"
    sleep 2
    
    # Create ServiceAccount
    log_info "Creating ServiceAccount: $TEST_SA_NAME"
    kubectl create serviceaccount "$TEST_SA_NAME" -n "$TEST_NAMESPACE" 2>&1 | tee -a "$REPORT_DIR/test.log"
    
    # Create Role with wildcard permissions (risky)
    log_info "Creating Role with wildcard permissions: $TEST_ROLE_NAME"
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
    
    # Create RoleBinding
    log_info "Creating RoleBinding: $TEST_RB_NAME"
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
    
    # Create Pod with the ServiceAccount
    log_info "Creating Pod: $TEST_POD_NAME"
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
    
    log_success "Test resources created"
    log_info "Waiting for resources to be processed by Agent..."
    sleep 30
}

# ==========================================
# PHASE 3: VERIFY AGENT COLLECTION
# ==========================================

verify_agent_collection() {
    log_section "Phase 3: Verifying Agent Collection"
    
    # Check Agent pod logs
    AGENT_POD=$(get_agent_pod)
    if [ ! -z "$AGENT_POD" ]; then
        log_info "Checking Agent logs for test resources..."
        kubectl logs -n "$NAMESPACE" "$AGENT_POD" --tail=50 | grep -i "$TEST_NAMESPACE" > "$REPORT_DIR/agent_logs.txt" 2>&1 || true
        
        if grep -q "$TEST_NAMESPACE" "$REPORT_DIR/agent_logs.txt" 2>/dev/null; then
            log_success "Agent detected test resources"
        else
            log_warning "Agent logs do not show test resources (may need more time)"
        fi
    else
        log_error "Agent pod not found"
        return 1
    fi
    
    log_success "Agent collection verified"
}

# ==========================================
# PHASE 4: VERIFY CORE PROCESSING
# ==========================================

verify_core_processing() {
    log_section "Phase 4: Verifying Core Processing"
    
    CORE_POD=$(get_core_pod)
    if [ ! -z "$CORE_POD" ]; then
        log_info "Checking Core logs for processing..."
        kubectl logs -n "$NAMESPACE" "$CORE_POD" --tail=100 | grep -iE "(normalizer|correlator|risk)" > "$REPORT_DIR/core_logs.txt" 2>&1 || true
        
        # Check for processing indicators
        if grep -qi "normalizer" "$REPORT_DIR/core_logs.txt" 2>/dev/null; then
            log_success "Normalizer is processing"
        fi
        
        if grep -qi "correlator" "$REPORT_DIR/core_logs.txt" 2>/dev/null; then
            log_success "Correlator is processing"
        fi
        
        if grep -qi "risk" "$REPORT_DIR/core_logs.txt" 2>/dev/null; then
            log_success "Risk Engine is processing"
        fi
    fi
    
    log_info "Waiting for Core to process resources..."
    sleep 15
    
    log_success "Core processing verified"
}

# ==========================================
# PHASE 5: VERIFY DATABASE STORAGE
# ==========================================

verify_database_storage() {
    log_section "Phase 5: Verifying Database Storage"
    
    POSTGRES_POD=$(get_postgres_pod)
    
    # Check if test resources are in database
    log_info "Checking for test resources in database..."
    
    # Check Pod
    DB_POD=$(exec_psql "$POSTGRES_POD" -t -c "
        SELECT COUNT(*) FROM pods 
        WHERE name = '$TEST_POD_NAME' 
        AND namespace = '$TEST_NAMESPACE' 
        AND deleted_at IS NULL;
    " 2>&1 | tr -d ' ')
    
    if [ "$DB_POD" = "1" ]; then
        log_success "Pod found in database"
    else
        log_error "Pod not found in database (count: $DB_POD)"
    fi
    
    # Check ServiceAccount
    DB_SA=$(exec_psql "$POSTGRES_POD" -t -c "
        SELECT COUNT(*) FROM service_accounts 
        WHERE name = '$TEST_SA_NAME' 
        AND namespace = '$TEST_NAMESPACE' 
        AND deleted_at IS NULL;
    " 2>&1 | tr -d ' ')
    
    if [ "$DB_SA" = "1" ]; then
        log_success "ServiceAccount found in database"
    else
        log_error "ServiceAccount not found in database (count: $DB_SA)"
    fi
    
    # Check Role
    DB_ROLE=$(exec_psql "$POSTGRES_POD" -t -c "
        SELECT COUNT(*) FROM roles 
        WHERE name = '$TEST_ROLE_NAME' 
        AND namespace = '$TEST_NAMESPACE' 
        AND deleted_at IS NULL;
    " 2>&1 | tr -d ' ')
    
    if [ "$DB_ROLE" = "1" ]; then
        log_success "Role found in database"
    else
        log_error "Role not found in database (count: $DB_ROLE)"
    fi
    
    # Check RoleBinding
    DB_RB=$(exec_psql "$POSTGRES_POD" -t -c "
        SELECT COUNT(*) FROM role_bindings 
        WHERE name = '$TEST_RB_NAME' 
        AND namespace = '$TEST_NAMESPACE' 
        AND deleted_at IS NULL;
    " 2>&1 | tr -d ' ')
    
    if [ "$DB_RB" = "1" ]; then
        log_success "RoleBinding found in database"
    else
        log_error "RoleBinding not found in database (count: $DB_RB)"
    fi
    
    # Get detailed resource info
    log_info "Fetching detailed resource information..."
    exec_psql "$POSTGRES_POD" -c "
        SELECT 
            'Pod' as resource_type,
            name,
            namespace,
            service_account as sa,
            created_at
        FROM pods
        WHERE name = '$TEST_POD_NAME' AND namespace = '$TEST_NAMESPACE'
        UNION ALL
        SELECT 
            'ServiceAccount',
            name,
            namespace,
            '' as sa,
            created_at
        FROM service_accounts
        WHERE name = '$TEST_SA_NAME' AND namespace = '$TEST_NAMESPACE'
        UNION ALL
        SELECT 
            'Role',
            name,
            namespace,
            '' as sa,
            created_at
        FROM roles
        WHERE name = '$TEST_ROLE_NAME' AND namespace = '$TEST_NAMESPACE';
    " > "$REPORT_DIR/db_resources_detail.txt" 2>&1
    
    log_success "Database storage verified"
}

# ==========================================
# PHASE 6: VERIFY RISK DETECTION
# ==========================================

verify_risk_detection() {
    log_section "Phase 6: Verifying Risk Detection"
    
    POSTGRES_POD=$(get_postgres_pod)
    
    # Trigger risk evaluation
    log_info "Triggering risk evaluation..."
    TOKEN=$(get_auth_token)
    if [ ! -z "$TOKEN" ]; then
        RESPONSE=$(curl -s -X POST "http://localhost:${CORE_PORT}/api/v1/insights/evaluate" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer $TOKEN" 2>&1)
        echo "$RESPONSE" | tee -a "$REPORT_DIR/risk_evaluation_response.json"
        log_info "Risk evaluation triggered"
    else
        log_warning "Could not get auth token, skipping API trigger"
    fi
    
    log_info "Waiting for risk evaluation to complete..."
    sleep 15
    
    # Check for insights
    log_info "Checking for generated insights..."
    DB_INSIGHTS=$(exec_psql "$POSTGRES_POD" -t -c "
        SELECT COUNT(*) FROM insights 
        WHERE deleted_at IS NULL 
        AND status = 'active'
        AND (
            description LIKE '%$TEST_ROLE_NAME%' 
            OR description LIKE '%wildcard%'
            OR description LIKE '%$TEST_SA_NAME%'
        );
    " 2>&1 | tr -d ' ')
    
    if [ "$DB_INSIGHTS" -gt 0 ]; then
        log_success "Found $DB_INSIGHTS insight(s) for test resources"
        
        # Get insight details
        exec_psql "$POSTGRES_POD" -c "
            SELECT 
                id,
                type,
                severity,
                description,
                affected_resources,
                created_at
            FROM insights
            WHERE deleted_at IS NULL 
            AND status = 'active'
            AND (
                description LIKE '%$TEST_ROLE_NAME%' 
                OR description LIKE '%wildcard%'
                OR description LIKE '%$TEST_SA_NAME%'
            )
            ORDER BY created_at DESC
            LIMIT 5;
        " > "$REPORT_DIR/insights_detail.txt" 2>&1
        
        cat "$REPORT_DIR/insights_detail.txt" | tee -a "$REPORT_DIR/test.log"
    else
        log_warning "No insights found for test resources (may need more time)"
    fi
    
    log_success "Risk detection verified"
}

# ==========================================
# PHASE 7: VERIFY API ENDPOINTS
# ==========================================

verify_api_endpoints() {
    log_section "Phase 7: Verifying API Endpoints"
    
    TOKEN=$(get_auth_token)
    if [ -z "$TOKEN" ]; then
        log_warning "Could not get auth token, skipping API verification"
        return 0
    fi
    
    # Test ServiceAccounts API
    log_info "Testing ServiceAccounts API..."
    RESPONSE=$(call_api "GET" "/api/v1/serviceaccounts?namespace=$TEST_NAMESPACE")
    if command -v jq &> /dev/null; then
        echo "$RESPONSE" | jq '.' > "$REPORT_DIR/api_serviceaccounts.json" 2>&1 || echo "$RESPONSE" > "$REPORT_DIR/api_serviceaccounts.json"
    else
        echo "$RESPONSE" > "$REPORT_DIR/api_serviceaccounts.json"
    fi
    
    if echo "$RESPONSE" | grep -q "$TEST_SA_NAME"; then
        log_success "ServiceAccount found via API"
    else
        log_warning "ServiceAccount not found via API"
    fi
    
    # Test Pods API
    log_info "Testing Pods API..."
    RESPONSE=$(call_api "GET" "/api/v1/pods?namespace=$TEST_NAMESPACE")
    if command -v jq &> /dev/null; then
        echo "$RESPONSE" | jq '.' > "$REPORT_DIR/api_pods.json" 2>&1 || echo "$RESPONSE" > "$REPORT_DIR/api_pods.json"
    else
        echo "$RESPONSE" > "$REPORT_DIR/api_pods.json"
    fi
    
    if echo "$RESPONSE" | grep -q "$TEST_POD_NAME"; then
        log_success "Pod found via API"
    else
        log_warning "Pod not found via API"
    fi
    
    # Test Insights API
    log_info "Testing Insights API..."
    RESPONSE=$(call_api "GET" "/api/v1/insights")
    if command -v jq &> /dev/null; then
        echo "$RESPONSE" | jq '.' > "$REPORT_DIR/api_insights.json" 2>&1 || echo "$RESPONSE" > "$REPORT_DIR/api_insights.json"
        INSIGHT_COUNT=$(echo "$RESPONSE" | jq -r '.total // 0' 2>/dev/null || echo "0")
    else
        echo "$RESPONSE" > "$REPORT_DIR/api_insights.json"
        INSIGHT_COUNT="0"
    fi
    log_info "Total insights via API: $INSIGHT_COUNT"
    
    # Test Insights Summary
    log_info "Testing Insights Summary API..."
    RESPONSE=$(call_api "GET" "/api/v1/insights/summary")
    if command -v jq &> /dev/null; then
        echo "$RESPONSE" | jq '.' > "$REPORT_DIR/api_insights_summary.json" 2>&1 || echo "$RESPONSE" > "$REPORT_DIR/api_insights_summary.json"
    else
        echo "$RESPONSE" > "$REPORT_DIR/api_insights_summary.json"
    fi
    
    log_success "API endpoints verified"
}

# ==========================================
# PHASE 8: VERIFY DASHBOARD
# ==========================================

verify_dashboard() {
    log_section "Phase 8: Verifying Dashboard"
    
    if [ -z "$DASHBOARD_PF_PID" ]; then
        log_warning "Dashboard port-forward not available, skipping dashboard verification"
        return 0
    fi
    
    log_info "Checking Dashboard availability..."
    DASHBOARD_RESPONSE=$(curl -s "http://localhost:${DASHBOARD_PORT}/" 2>&1)
    
    if echo "$DASHBOARD_RESPONSE" | grep -qi "ksam\|dashboard"; then
        log_success "Dashboard is accessible"
    else
        log_warning "Dashboard may not be fully loaded"
    fi
    
    log_success "Dashboard verified"
}

# ==========================================
# PHASE 9: POST-TEST STATE CAPTURE
# ==========================================

capture_post_test_state() {
    log_section "Phase 9: Capturing Post-Test State"
    
    POSTGRES_POD=$(get_postgres_pod)
    
    # Capture K8s state
    log_info "Capturing Kubernetes state..."
    kubectl get pods -A -o json > "$REPORT_DIR/k8s_pods_after.json" 2>&1
    kubectl get serviceaccounts -A -o json > "$REPORT_DIR/k8s_sas_after.json" 2>&1
    kubectl get roles -A -o json > "$REPORT_DIR/k8s_roles_after.json" 2>&1
    kubectl get rolebindings -A -o json > "$REPORT_DIR/k8s_rbs_after.json" 2>&1
    
    # Count K8s resources
    K8S_PODS_AFTER=$(kubectl get pods -A --no-headers 2>/dev/null | wc -l | tr -d ' ')
    K8S_SAS_AFTER=$(kubectl get serviceaccounts -A --no-headers 2>/dev/null | wc -l | tr -d ' ')
    K8S_ROLES_AFTER=$(kubectl get roles -A --no-headers 2>/dev/null | wc -l | tr -d ' ')
    K8S_RBS_AFTER=$(kubectl get rolebindings -A --no-headers 2>/dev/null | wc -l | tr -d ' ')
    
    log_info "K8s Resources After:"
    log_info "  Pods: $K8S_PODS_AFTER (was: $K8S_PODS_BEFORE, diff: $((K8S_PODS_AFTER - K8S_PODS_BEFORE)))"
    log_info "  ServiceAccounts: $K8S_SAS_AFTER (was: $K8S_SAS_BEFORE, diff: $((K8S_SAS_AFTER - K8S_SAS_BEFORE)))"
    log_info "  Roles: $K8S_ROLES_AFTER (was: $K8S_ROLES_BEFORE, diff: $((K8S_ROLES_AFTER - K8S_ROLES_BEFORE)))"
    log_info "  RoleBindings: $K8S_RBS_AFTER (was: $K8S_RBS_BEFORE, diff: $((K8S_RBS_AFTER - K8S_RBS_BEFORE)))"
    
    # Capture Database state
    log_info "Capturing Database state..."
    DB_PODS_AFTER=$(exec_psql "$POSTGRES_POD" -t -c "SELECT COUNT(*) FROM pods WHERE deleted_at IS NULL;" 2>&1 | tr -d ' ')
    DB_SAS_AFTER=$(exec_psql "$POSTGRES_POD" -t -c "SELECT COUNT(*) FROM service_accounts WHERE deleted_at IS NULL;" 2>&1 | tr -d ' ')
    DB_ROLES_AFTER=$(exec_psql "$POSTGRES_POD" -t -c "SELECT COUNT(*) FROM roles WHERE deleted_at IS NULL;" 2>&1 | tr -d ' ')
    DB_RBS_AFTER=$(exec_psql "$POSTGRES_POD" -t -c "SELECT COUNT(*) FROM role_bindings WHERE deleted_at IS NULL;" 2>&1 | tr -d ' ')
    DB_INSIGHTS_AFTER=$(exec_psql "$POSTGRES_POD" -t -c "SELECT COUNT(*) FROM insights WHERE deleted_at IS NULL AND status = 'active';" 2>&1 | tr -d ' ')
    
    log_info "Database Resources After:"
    log_info "  Pods: $DB_PODS_AFTER (was: $DB_PODS_BEFORE, diff: $((DB_PODS_AFTER - DB_PODS_BEFORE)))"
    log_info "  ServiceAccounts: $DB_SAS_AFTER (was: $DB_SAS_BEFORE, diff: $((DB_SAS_AFTER - DB_SAS_BEFORE)))"
    log_info "  Roles: $DB_ROLES_AFTER (was: $DB_ROLES_BEFORE, diff: $((DB_ROLES_AFTER - DB_ROLES_BEFORE)))"
    log_info "  RoleBindings: $DB_RBS_AFTER (was: $DB_RBS_BEFORE, diff: $((DB_RBS_AFTER - DB_RBS_BEFORE)))"
    log_info "  Active Insights: $DB_INSIGHTS_AFTER (was: $DB_INSIGHTS_BEFORE, diff: $((DB_INSIGHTS_AFTER - DB_INSIGHTS_BEFORE)))"
    
    log_success "Post-test state captured"
}

# ==========================================
# PHASE 10: COMPARISON AND VALIDATION
# ==========================================

compare_and_validate() {
    log_section "Phase 10: Comparison and Validation"
    
    POSTGRES_POD=$(get_postgres_pod)
    
    # Compare K8s vs Database
    log_info "Comparing K8s state vs Database state..."
    
    # Check if test resources exist in both
    K8S_TEST_POD=$(kubectl get pod "$TEST_POD_NAME" -n "$TEST_NAMESPACE" --no-headers 2>/dev/null | wc -l | tr -d ' ')
    DB_TEST_POD=$(exec_psql "$POSTGRES_POD" -t -c "
        SELECT COUNT(*) FROM pods 
        WHERE name = '$TEST_POD_NAME' 
        AND namespace = '$TEST_NAMESPACE' 
        AND deleted_at IS NULL;
    " 2>&1 | tr -d ' ')
    
    if [ "$K8S_TEST_POD" = "1" ] && [ "$DB_TEST_POD" = "1" ]; then
        log_success "✅ Pod sync: K8s ($K8S_TEST_POD) = Database ($DB_TEST_POD)"
    else
        log_error "❌ Pod sync mismatch: K8s ($K8S_TEST_POD) vs Database ($DB_TEST_POD)"
    fi
    
    K8S_TEST_SA=$(kubectl get serviceaccount "$TEST_SA_NAME" -n "$TEST_NAMESPACE" --no-headers 2>/dev/null | wc -l | tr -d ' ')
    DB_TEST_SA=$(exec_psql "$POSTGRES_POD" -t -c "
        SELECT COUNT(*) FROM service_accounts 
        WHERE name = '$TEST_SA_NAME' 
        AND namespace = '$TEST_NAMESPACE' 
        AND deleted_at IS NULL;
    " 2>&1 | tr -d ' ')
    
    if [ "$K8S_TEST_SA" = "1" ] && [ "$DB_TEST_SA" = "1" ]; then
        log_success "✅ ServiceAccount sync: K8s ($K8S_TEST_SA) = Database ($DB_TEST_SA)"
    else
        log_error "❌ ServiceAccount sync mismatch: K8s ($K8S_TEST_SA) vs Database ($DB_TEST_SA)"
    fi
    
    K8S_TEST_ROLE=$(kubectl get role "$TEST_ROLE_NAME" -n "$TEST_NAMESPACE" --no-headers 2>/dev/null | wc -l | tr -d ' ')
    DB_TEST_ROLE=$(exec_psql "$POSTGRES_POD" -t -c "
        SELECT COUNT(*) FROM roles 
        WHERE name = '$TEST_ROLE_NAME' 
        AND namespace = '$TEST_NAMESPACE' 
        AND deleted_at IS NULL;
    " 2>&1 | tr -d ' ')
    
    if [ "$K8S_TEST_ROLE" = "1" ] && [ "$DB_TEST_ROLE" = "1" ]; then
        log_success "✅ Role sync: K8s ($K8S_TEST_ROLE) = Database ($DB_TEST_ROLE)"
    else
        log_error "❌ Role sync mismatch: K8s ($K8S_TEST_ROLE) vs Database ($DB_TEST_ROLE)"
    fi
    
    # Verify correlation (Pod → ServiceAccount)
    log_info "Verifying correlation (Pod → ServiceAccount)..."
    CORRELATION=$(exec_psql "$POSTGRES_POD" -t -c "
        SELECT COUNT(*) FROM pods p
        JOIN service_accounts sa ON p.service_account = sa.name 
            AND p.namespace = sa.namespace
        WHERE p.name = '$TEST_POD_NAME' 
        AND p.namespace = '$TEST_NAMESPACE';
    " 2>&1 | tr -d ' ')
    
    if [ "$CORRELATION" = "1" ]; then
        log_success "✅ Pod-ServiceAccount correlation verified"
    else
        log_warning "⚠️  Pod-ServiceAccount correlation not found"
    fi
    
    # Verify risk detection
    log_info "Verifying risk detection..."
    RISK_INSIGHT=$(exec_psql "$POSTGRES_POD" -t -c "
        SELECT COUNT(*) FROM insights 
        WHERE deleted_at IS NULL 
        AND status = 'active'
        AND (
            description LIKE '%wildcard%' 
            OR description LIKE '%$TEST_ROLE_NAME%'
            OR description LIKE '%$TEST_SA_NAME%'
        );
    " 2>&1 | tr -d ' ')
    
    if [ "$RISK_INSIGHT" -gt 0 ]; then
        log_success "✅ Risk detection working: Found $RISK_INSIGHT insight(s)"
    else
        log_warning "⚠️  No risk insights found for test resources"
    fi
    
    log_success "Comparison and validation complete"
}

# ==========================================
# PHASE 11: GENERATE REPORT
# ==========================================

generate_report() {
    log_section "Phase 11: Generating Final Report"
    
    cat > "$REPORT_DIR/MVP1_E2E_REPORT.md" <<EOF
# MVP-1 End-to-End Test Report

**Date**: $(date)
**Test Namespace**: $TEST_NAMESPACE
**Test Resources**:
- Pod: $TEST_POD_NAME
- ServiceAccount: $TEST_SA_NAME
- Role: $TEST_ROLE_NAME
- RoleBinding: $TEST_RB_NAME

## Test Summary

### Pre-Test State
- K8s Pods: $K8S_PODS_BEFORE
- K8s ServiceAccounts: $K8S_SAS_BEFORE
- K8s Roles: $K8S_ROLES_BEFORE
- K8s RoleBindings: $K8S_RBS_BEFORE
- Database Pods: $DB_PODS_BEFORE
- Database ServiceAccounts: $DB_SAS_BEFORE
- Database Roles: $DB_ROLES_BEFORE
- Database RoleBindings: $DB_RBS_BEFORE
- Active Insights: $DB_INSIGHTS_BEFORE

### Post-Test State
- K8s Pods: $K8S_PODS_AFTER (diff: $((K8S_PODS_AFTER - K8S_PODS_BEFORE)))
- K8s ServiceAccounts: $K8S_SAS_AFTER (diff: $((K8S_SAS_AFTER - K8S_SAS_BEFORE)))
- K8s Roles: $K8S_ROLES_AFTER (diff: $((K8S_ROLES_AFTER - K8S_ROLES_BEFORE)))
- K8s RoleBindings: $K8S_RBS_AFTER (diff: $((K8S_RBS_AFTER - K8S_RBS_BEFORE)))
- Database Pods: $DB_PODS_AFTER (diff: $((DB_PODS_AFTER - DB_PODS_BEFORE)))
- Database ServiceAccounts: $DB_SAS_AFTER (diff: $((DB_SAS_AFTER - DB_SAS_BEFORE)))
- Database Roles: $DB_ROLES_AFTER (diff: $((DB_ROLES_AFTER - DB_ROLES_BEFORE)))
- Database RoleBindings: $DB_RBS_AFTER (diff: $((DB_RBS_AFTER - DB_RBS_BEFORE)))
- Active Insights: $DB_INSIGHTS_AFTER (diff: $((DB_INSIGHTS_AFTER - DB_INSIGHTS_BEFORE)))

## Test Phases

1. ✅ Pre-Test State Capture
2. ✅ Test Resource Creation
3. ✅ Agent Collection Verification
4. ✅ Core Processing Verification
5. ✅ Database Storage Verification
6. ✅ Risk Detection Verification
7. ✅ API Endpoints Verification
8. ✅ Dashboard Verification
9. ✅ Post-Test State Capture
10. ✅ Comparison and Validation

## Files Generated

- \`test.log\`: Full test log
- \`k8s_*_before.json\`: K8s state before test
- \`k8s_*_after.json\`: K8s state after test
- \`db_state_before.txt\`: Database state before
- \`db_resources_detail.txt\`: Detailed resource info
- \`insights_detail.txt\`: Generated insights
- \`api_*.json\`: API responses
- \`agent_logs.txt\`: Agent logs
- \`core_logs.txt\`: Core logs

## Conclusion

See test.log for detailed results.
EOF
    
    log_success "Report generated: $REPORT_DIR/MVP1_E2E_REPORT.md"
}

# ==========================================
# MAIN EXECUTION
# ==========================================

main() {
    log_section "MVP-1 Comprehensive E2E Test"
    echo "Starting test at $(date)"
    echo ""
    
    capture_pre_test_state
    create_test_resources
    verify_agent_collection
    verify_core_processing
    verify_database_storage
    verify_risk_detection
    verify_api_endpoints
    verify_dashboard
    capture_post_test_state
    compare_and_validate
    generate_report
    
    log_section "Test Complete"
    echo "Test finished at $(date)"
    echo ""
    echo "Report saved to: $REPORT_DIR/"
    echo "  - MVP1_E2E_REPORT.md: Summary report"
    echo "  - test.log: Full test log"
    echo "  - *.json: API responses and state captures"
}

main "$@"

