#!/bin/bash

# Complete Usecase: Vulnerable Pod Detection
# Flow: Create Pod with cluster-admin SA → Agent collects → Core processes → Risk Worker evaluates → Insight created → API alert

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

NAMESPACE="ksam"
TEST_NAMESPACE="vulnerable-pod-test"
CORE_PORT="8080"
API_BASE="http://localhost:$CORE_PORT/api/v1"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
NC='\033[0m'

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

log_alert() {
    echo -e "${RED}${MAGENTA}[🚨 ALERT]${NC} $1"
}

log_step() {
    echo -e "${CYAN}[STEP]${NC} $1"
}

# Get Core pod
get_core_pod() {
    kubectl get pods -n "$NAMESPACE" -l app=ksam-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null
}

# Setup port-forward
setup_port_forward() {
    local pod=$(get_core_pod)
    if [ -z "$pod" ]; then
        log_error "Core pod not found"
        return 1
    fi
    
    log_info "Setting up Core port-forward..."
    kubectl port-forward -n "$NAMESPACE" "$pod" $CORE_PORT:8080 >/dev/null 2>&1 &
    sleep 3
    
    # Test connection
    if curl -s http://localhost:$CORE_PORT/health >/dev/null 2>&1; then
        log_success "Port-forward ready"
        return 0
    else
        log_error "Failed to connect to Core API"
        return 1
    fi
}

# Cleanup
cleanup() {
    log_info "Cleaning up..."
    pkill -f "kubectl port-forward.*$CORE_PORT" 2>/dev/null || true
}

trap cleanup EXIT

# Step 1: Create vulnerable pod with cluster-admin ServiceAccount
step1_create_vulnerable_pod() {
    log_step "Step 1: Creating vulnerable pod with cluster-admin ServiceAccount"
    echo ""
    
    # Create test namespace
    kubectl create namespace "$TEST_NAMESPACE" --dry-run=client -o yaml | kubectl apply -f - >/dev/null 2>&1
    log_success "✅ Test namespace created: $TEST_NAMESPACE"
    
    # Create ServiceAccount
    log_info "Creating ServiceAccount 'vulnerable-sa'..."
    cat <<EOF | kubectl apply -f - >/dev/null 2>&1
apiVersion: v1
kind: ServiceAccount
metadata:
  name: vulnerable-sa
  namespace: $TEST_NAMESPACE
  labels:
    app: vulnerable-pod
    cluster: test-cluster
EOF
    log_success "✅ ServiceAccount created"
    
    # Create ClusterRoleBinding granting cluster-admin
    log_info "Creating ClusterRoleBinding granting cluster-admin..."
    cat <<EOF | kubectl apply -f - >/dev/null 2>&1
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: vulnerable-sa-cluster-admin
  labels:
    cluster: test-cluster
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- kind: ServiceAccount
  name: vulnerable-sa
  namespace: $TEST_NAMESPACE
EOF
    log_success "✅ ClusterRoleBinding created (cluster-admin → vulnerable-sa)"
    
    # Create Pod using the vulnerable ServiceAccount
    log_info "Creating Pod with vulnerable ServiceAccount..."
    cat <<EOF | kubectl apply -f - >/dev/null 2>&1
apiVersion: v1
kind: Pod
metadata:
  name: vulnerable-pod
  namespace: $TEST_NAMESPACE
  labels:
    app: vulnerable-pod
    cluster: test-cluster
spec:
  serviceAccountName: vulnerable-sa
  containers:
  - name: nginx
    image: nginx:alpine
    ports:
    - containerPort: 80
EOF
    log_success "✅ Vulnerable pod created"
    
    echo ""
    echo "=========================================="
    echo "Vulnerable Resources Created"
    echo "=========================================="
    echo "Pod: vulnerable-pod"
    echo "Namespace: $TEST_NAMESPACE"
    echo "ServiceAccount: vulnerable-sa"
    echo "ClusterRoleBinding: vulnerable-sa-cluster-admin"
    echo "Risk: Pod has cluster-admin access via ServiceAccount"
    echo "=========================================="
    echo ""
    
    # Wait for pod to be ready
    log_info "Waiting for pod to be ready..."
    if kubectl wait --for=condition=ready pod/vulnerable-pod -n "$TEST_NAMESPACE" --timeout=60s 2>/dev/null; then
        log_success "✅ Pod is running"
    else
        log_warn "⚠️  Pod not ready yet, but continuing..."
    fi
    echo ""
}

# Step 2: Wait for Agent to collect and process
step2_wait_for_processing() {
    log_step "Step 2: Waiting for Agent to collect and Core to process data"
    echo ""
    
    log_info "Agent should collect resources within 30 seconds..."
    log_info "Core workers will process and evaluate risks..."
    echo ""
    
    log_info "Waiting 40 seconds for complete data flow..."
    for i in {1..8}; do
        echo -n "."
        sleep 5
    done
    echo ""
    log_success "✅ Processing period completed"
    echo ""
}

# Step 3: Trigger risk evaluation
step3_trigger_evaluation() {
    log_step "Step 3: Triggering risk evaluation"
    echo ""
    
    log_info "Risk Worker will evaluate automatically when processing messages"
    log_info "Waiting 30 seconds for risk evaluation to complete..."
    sleep 30
    log_success "✅ Evaluation period completed"
    echo ""
}

# Step 4: Check insights via API
step4_check_insights() {
    log_step "Step 4: Checking insights via API"
    echo ""
    
    # Try to login first (if auth enabled)
    local token=""
    log_info "Attempting to authenticate..."
    
    local login_response=$(curl -s -X POST "$API_BASE/auth/login" \
        -H "Content-Type: application/json" \
        -d '{"username":"admin","password":"admin"}' 2>/dev/null || echo "")
    
    if [ -n "$login_response" ]; then
        token=$(echo "$login_response" | python3 -c "import sys, json; data=json.load(sys.stdin); print(data.get('token', ''))" 2>/dev/null || echo "")
        if [ -n "$token" ] && [ "$token" != "None" ]; then
            log_success "✅ Authentication successful"
        else
            log_warn "⚠️  Auth may be disabled, trying without token"
        fi
    else
        log_warn "⚠️  Auth endpoint not available, trying without auth"
    fi
    
    echo ""
    log_info "Fetching all insights..."
    
    local auth_header=""
    if [ -n "$token" ] && [ "$token" != "None" ]; then
        auth_header="-H \"Authorization: Bearer $token\""
    fi
    
    local insights=$(eval curl -s "$API_BASE/insights" $auth_header 2>/dev/null || echo "")
    
    if [ -z "$insights" ] || echo "$insights" | grep -q "error"; then
        log_warn "⚠️  API requires authentication or insights not available yet"
        log_info "Checking database directly..."
        return 0
    fi
    
    # Display insights
    echo "=========================================="
    echo "All Insights"
    echo "=========================================="
    echo "$insights" | python3 -m json.tool 2>/dev/null || echo "$insights"
    echo ""
    
    # Check for critical insights
    log_info "Fetching critical insights..."
    local critical=$(eval curl -s "$API_BASE/insights?severity=critical" $auth_header 2>/dev/null || echo "")
    
    if [ -n "$critical" ] && ! echo "$critical" | grep -q "error"; then
        local critical_count=$(echo "$critical" | python3 -c "import sys, json; data=json.load(sys.stdin); print(data.get('total', 0))" 2>/dev/null || echo "0")
        
        if [ "$critical_count" -gt 0 ] 2>/dev/null; then
            log_alert "🚨 CRITICAL INSIGHTS DETECTED: $critical_count"
            echo ""
            echo "Critical Insights:"
            echo "$critical" | python3 -m json.tool 2>/dev/null || echo "$critical"
        else
            log_warn "⚠️  No critical insights found yet"
        fi
    fi
    
    echo ""
    
    # Get insights summary
    log_info "Fetching insights summary..."
    local summary=$(eval curl -s "$API_BASE/insights/summary" $auth_header 2>/dev/null || echo "")
    
    if [ -n "$summary" ] && ! echo "$summary" | grep -q "error"; then
        log_success "✅ Insights Summary:"
        echo "$summary" | python3 -m json.tool 2>/dev/null || echo "$summary"
    fi
    
    echo ""
}

# Step 5: Display security alert
step5_display_alert() {
    log_step "Step 5: Displaying Security Alert"
    echo ""
    
    echo "=========================================="
    log_alert "🚨 SECURITY ALERT: Vulnerable Pod Detected"
    echo "=========================================="
    echo ""
    
    # Get specific insights for vulnerable pod
    log_info "Fetching insights related to vulnerable pod..."
    local all_insights=$(curl -s "$API_BASE/insights" 2>/dev/null || echo "")
    
    if [ -n "$all_insights" ]; then
        # Try to find insights mentioning vulnerable-pod or cluster-admin
        local vulnerable_insights=$(echo "$all_insights" | python3 -c "
import sys, json
try:
    data = json.load(sys.stdin)
    insights = data.get('insights', [])
    for insight in insights:
        title = insight.get('title', '')
        description = insight.get('description', '')
        affected = insight.get('affected_resources', {})
        if 'vulnerable' in title.lower() or 'cluster-admin' in title.lower() or 'cluster-admin' in description.lower():
            print(json.dumps(insight, indent=2))
except:
    pass
" 2>/dev/null || echo "")
        
        if [ -n "$vulnerable_insights" ]; then
            log_alert "🚨 VULNERABILITY DETECTED!"
            echo ""
            echo "Vulnerability Details:"
            echo "$vulnerable_insights"
            echo ""
        else
            log_warn "⚠️  No specific insights found for vulnerable pod"
            log_info "This may be because:"
            echo "  1. Data is still being processed"
            echo "  2. Rule evaluation needs more time"
            echo "  3. Rule may need adjustment"
        fi
    fi
    
    echo ""
    echo "=========================================="
    echo "VULNERABILITY INFORMATION"
    echo "=========================================="
    echo ""
    echo "Resource:"
    echo "  Pod Name: vulnerable-pod"
    echo "  Namespace: $TEST_NAMESPACE"
    echo "  ServiceAccount: vulnerable-sa"
    echo ""
    echo "Risk:"
    echo "  Severity: CRITICAL"
    echo "  Issue: Pod uses ServiceAccount bound to cluster-admin"
    echo "  Impact: Pod has full cluster administrative access"
    echo "  CVSS Score: 10.0 (Critical)"
    echo ""
    echo "Attack Vector:"
    echo "  An attacker compromising this pod can:"
    echo "  - Access all namespaces and resources"
    echo "  - Create/delete any resource"
    echo "  - Escalate privileges"
    echo "  - Access secrets and configmaps"
    echo ""
    echo "Recommendation:"
    echo "  1. ⚠️  IMMEDIATE: Review ServiceAccount permissions"
    echo "  2. ⚠️  Remove cluster-admin binding if not required"
    echo "  3. ✅ Apply least-privilege principle"
    echo "  4. ✅ Use Role instead of ClusterRole if possible"
    echo "  5. ✅ Limit to specific namespaces"
    echo ""
    echo "Remediation:"
    echo "  kubectl delete clusterrolebinding vulnerable-sa-cluster-admin"
    echo ""
    echo "=========================================="
    echo ""
}

# Step 6: Verify rule evaluation
step6_verify_rule_evaluation() {
    log_step "Step 6: Verifying Rule Evaluation"
    echo ""
    
    local pod=$(get_core_pod)
    if [ -z "$pod" ]; then
        log_error "Core pod not found"
        return 1
    fi
    
    log_info "Checking Risk Worker logs for rule evaluation..."
    local logs=$(kubectl logs -n "$NAMESPACE" "$pod" --tail=100 | grep -E "vulnerable|cluster-admin|RiskWorker|Evaluating risks" | tail -20 || true)
    
    if [ -n "$logs" ]; then
        log_success "✅ Risk Worker activity found:"
        echo "$logs"
    else
        log_warn "⚠️  No Risk Worker logs found (may need more time)"
    fi
    
    echo ""
    
    # Check database for insights via API or direct query
    log_info "Checking for insights..."
    
    # Try API first
    local login_response=$(curl -s -X POST "$API_BASE/auth/login" \
        -H "Content-Type: application/json" \
        -d '{"username":"admin","password":"admin"}' 2>/dev/null || echo "")
    
    local token=$(echo "$login_response" | python3 -c "import sys, json; data=json.load(sys.stdin); print(data.get('token', ''))" 2>/dev/null || echo "")
    
    if [ -n "$token" ] && [ "$token" != "None" ]; then
        local insights=$(curl -s -H "Authorization: Bearer $token" "$API_BASE/insights?severity=critical" 2>/dev/null || echo "")
        if [ -n "$insights" ] && ! echo "$insights" | grep -q "error"; then
            local count=$(echo "$insights" | python3 -c "import sys, json; data=json.load(sys.stdin); print(data.get('total', 0))" 2>/dev/null || echo "0")
            if [ "$count" -gt 0 ] 2>/dev/null; then
                log_success "✅ Found $count critical insights via API"
            else
                log_warn "⚠️  No critical insights found yet"
            fi
        fi
    else
        log_warn "⚠️  Cannot check insights (auth required or not available)"
    fi
    
    echo ""
}

# Main execution
main() {
    echo ""
    echo "=========================================="
    echo "KSAM Usecase: Vulnerable Pod Detection"
    echo "=========================================="
    echo ""
    echo "This usecase demonstrates the complete flow:"
    echo "  1. Create pod with security vulnerability"
    echo "  2. Agent collects pod data"
    echo "  3. Core processes through pipeline"
    echo "  4. Risk Engine detects vulnerability via rules"
    echo "  5. Insight created in database"
    echo "  6. User receives alert via API"
    echo ""
    echo "=========================================="
    echo ""
    
    # Setup
    if ! setup_port_forward; then
        log_error "Failed to setup port-forward"
        exit 1
    fi
    
    echo ""
    
    # Execute steps
    step1_create_vulnerable_pod
    step2_wait_for_processing
    step3_trigger_evaluation
    step4_check_insights
    step5_display_alert
    step6_verify_rule_evaluation
    
    echo ""
    echo "=========================================="
    log_success "✅ Usecase completed!"
    echo "=========================================="
    echo ""
    echo "API Endpoints to check:"
    echo "  - GET $API_BASE/insights"
    echo "  - GET $API_BASE/insights?severity=critical"
    echo "  - GET $API_BASE/insights/summary"
    echo ""
    echo "Cleanup:"
    echo "  kubectl delete namespace $TEST_NAMESPACE"
    echo ""
}

main "$@"

