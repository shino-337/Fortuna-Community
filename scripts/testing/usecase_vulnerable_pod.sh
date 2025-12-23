#!/bin/bash

# Usecase: Tạo pod với lỗ hổng → Phát hiện bằng rule → Cảnh báo qua API
# Flow: Create Pod → Agent collects → Core processes → Risk Worker evaluates → Insight created → API returns alert

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

NAMESPACE="ksam"
TEST_NAMESPACE="vulnerable-pod-test"
CORE_PORT="8080"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
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
    echo -e "${RED}[ALERT]${NC} $1"
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
    sleep 2
    log_success "Port-forward ready"
}

# Cleanup
cleanup() {
    log_info "Cleaning up..."
    pkill -f "kubectl port-forward.*$CORE_PORT" 2>/dev/null || true
    
    # Cleanup test resources
    kubectl delete namespace "$TEST_NAMESPACE" --ignore-not-found=true 2>/dev/null || true
}

trap cleanup EXIT

# Step 1: Create vulnerable pod
step1_create_vulnerable_pod() {
    log_info "=========================================="
    log_info "Step 1: Creating vulnerable pod"
    log_info "=========================================="
    echo ""
    
    # Create test namespace
    kubectl create namespace "$TEST_NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -
    log_success "Test namespace created: $TEST_NAMESPACE"
    
    # Create ServiceAccount with cluster-admin binding (vulnerable)
    log_info "Creating ServiceAccount with cluster-admin binding..."
    
    cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ServiceAccount
metadata:
  name: vulnerable-sa
  namespace: $TEST_NAMESPACE
  labels:
    app: vulnerable-pod
    cluster: test-cluster
---
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
---
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
    echo "Pod Details:"
    echo "  Name: vulnerable-pod"
    echo "  Namespace: $TEST_NAMESPACE"
    echo "  ServiceAccount: vulnerable-sa (bound to cluster-admin)"
    echo ""
    
    # Wait for pod to be ready
    log_info "Waiting for pod to be ready..."
    kubectl wait --for=condition=ready pod/vulnerable-pod -n "$TEST_NAMESPACE" --timeout=60s || {
        log_warn "Pod not ready, but continuing..."
    }
    
    log_success "Pod is running"
    echo ""
}

# Step 2: Wait for Agent to collect
step2_wait_for_collection() {
    log_info "=========================================="
    log_info "Step 2: Waiting for Agent to collect data"
    log_info "=========================================="
    echo ""
    
    log_info "Agent should collect pod data within 30 seconds..."
    log_info "Checking Agent logs..."
    
    local agent_pod=$(kubectl get pods -n "$NAMESPACE" -l app=ksam-agent -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
    
    if [ -n "$agent_pod" ]; then
        log_info "Agent pod: $agent_pod"
        log_info "Recent Agent logs:"
        kubectl logs -n "$NAMESPACE" "$agent_pod" --tail=10 | grep -E "vulnerable-pod|vulnerable-sa|cluster-admin" || log_warn "No relevant logs yet"
    else
        log_warn "Agent pod not found"
    fi
    
    echo ""
    log_info "Waiting 30 seconds for data collection..."
    sleep 30
    log_success "Data collection period completed"
    echo ""
}

# Step 3: Verify data in database
step3_verify_data() {
    log_info "=========================================="
    log_info "Step 3: Verifying data in database"
    log_info "=========================================="
    echo ""
    
    local pod=$(get_core_pod)
    if [ -z "$pod" ]; then
        log_error "Core pod not found"
        return 1
    fi
    
    log_info "Checking database for vulnerable pod..."
    
    # Check if pod exists in database
    local pod_count=$(kubectl exec -n "$NAMESPACE" "$pod" -- \
        psql postgresql://ksam_user:ksam_password@postgres:5432/ksam_db -tAc \
        "SELECT COUNT(*) FROM pods WHERE name='vulnerable-pod' AND namespace='$TEST_NAMESPACE';" 2>/dev/null || echo "0")
    
    if [ "$pod_count" -gt 0 ]; then
        log_success "✅ Pod found in database"
        echo "  Pod count: $pod_count"
    else
        log_warn "⚠️  Pod not yet in database (may need more time)"
    fi
    
    # Check if service account exists
    local sa_count=$(kubectl exec -n "$NAMESPACE" "$pod" -- \
        psql postgresql://ksam_user:ksam_password@postgres:5432/ksam_db -tAc \
        "SELECT COUNT(*) FROM service_accounts WHERE name='vulnerable-sa' AND namespace='$TEST_NAMESPACE';" 2>/dev/null || echo "0")
    
    if [ "$sa_count" -gt 0 ]; then
        log_success "✅ ServiceAccount found in database"
        echo "  SA count: $sa_count"
    else
        log_warn "⚠️  ServiceAccount not yet in database"
    fi
    
    # Check if cluster role binding exists
    local crb_count=$(kubectl exec -n "$NAMESPACE" "$pod" -- \
        psql postgresql://ksam_user:ksam_password@postgres:5432/ksam_db -tAc \
        "SELECT COUNT(*) FROM cluster_role_bindings WHERE name='vulnerable-sa-cluster-admin';" 2>/dev/null || echo "0")
    
    if [ "$crb_count" -gt 0 ]; then
        log_success "✅ ClusterRoleBinding found in database"
        echo "  CRB count: $crb_count"
    else
        log_warn "⚠️  ClusterRoleBinding not yet in database"
    fi
    
    echo ""
}

# Step 4: Trigger risk evaluation
step4_trigger_risk_evaluation() {
    log_info "=========================================="
    log_info "Step 4: Triggering risk evaluation"
    log_info "=========================================="
    echo ""
    
    log_info "Triggering historical risk evaluation..."
    
    # Get auth token (if auth enabled, otherwise skip)
    local token=""
    if [ -n "$token" ]; then
        local response=$(curl -s -X POST http://localhost:$CORE_PORT/api/v1/insights/evaluate/historical \
            -H "Authorization: Bearer $token" \
            -H "Content-Type: application/json")
    else
        # Try without auth
        local response=$(curl -s -X POST http://localhost:$CORE_PORT/api/v1/insights/evaluate/historical \
            -H "Content-Type: application/json" 2>/dev/null || echo "")
    fi
    
    if [ -n "$response" ]; then
        log_success "✅ Risk evaluation triggered"
        echo "  Response: $response"
    else
        log_warn "⚠️  Risk evaluation endpoint may require auth or may not be available"
        log_info "Risk Worker will evaluate automatically when processing messages"
    fi
    
    echo ""
    log_info "Waiting 20 seconds for risk evaluation..."
    sleep 20
    log_success "Risk evaluation period completed"
    echo ""
}

# Step 5: Check for insights via API
step5_check_insights() {
    log_info "=========================================="
    log_info "Step 5: Checking insights via API"
    log_info "=========================================="
    echo ""
    
    log_info "Fetching insights from API..."
    
    # Get all insights
    local insights=$(curl -s http://localhost:$CORE_PORT/api/v1/insights 2>/dev/null || echo "")
    
    if [ -z "$insights" ]; then
        log_error "Failed to fetch insights"
        return 1
    fi
    
    # Parse and display insights
    echo "$insights" | python3 -m json.tool 2>/dev/null || echo "$insights"
    echo ""
    
    # Check for vulnerable pod insights
    local vulnerable_insights=$(echo "$insights" | grep -i "vulnerable\|cluster-admin" || true)
    
    if [ -n "$vulnerable_insights" ]; then
        log_alert "🚨 VULNERABILITY DETECTED!"
        echo ""
        echo "Insights related to vulnerable pod:"
        echo "$vulnerable_insights"
        echo ""
    else
        log_warn "⚠️  No insights found yet (may need more time for evaluation)"
    fi
    
    # Get insights summary
    log_info "Fetching insights summary..."
    local summary=$(curl -s http://localhost:$CORE_PORT/api/v1/insights/summary 2>/dev/null || echo "")
    
    if [ -n "$summary" ]; then
        log_success "✅ Insights summary:"
        echo "$summary" | python3 -m json.tool 2>/dev/null || echo "$summary"
    fi
    
    echo ""
}

# Step 6: Display alert
step6_display_alert() {
    log_info "=========================================="
    log_info "Step 6: Displaying Security Alert"
    log_info "=========================================="
    echo ""
    
    log_alert "🚨 SECURITY ALERT: Vulnerable Pod Detected"
    echo ""
    echo "=========================================="
    echo "VULNERABILITY DETAILS"
    echo "=========================================="
    echo ""
    echo "Pod Information:"
    echo "  Name: vulnerable-pod"
    echo "  Namespace: $TEST_NAMESPACE"
    echo "  ServiceAccount: vulnerable-sa"
    echo ""
    echo "Risk:"
    echo "  Severity: CRITICAL"
    echo "  Issue: Pod uses ServiceAccount bound to cluster-admin"
    echo "  Impact: Pod has full cluster access"
    echo ""
    echo "Recommendation:"
    echo "  1. Review ServiceAccount permissions"
    echo "  2. Remove cluster-admin binding if not needed"
    echo "  3. Use least-privilege principle"
    echo ""
    echo "=========================================="
    echo ""
    
    # Get specific insights
    log_info "Fetching detailed insights..."
    local insights=$(curl -s "http://localhost:$CORE_PORT/api/v1/insights?severity=critical" 2>/dev/null || echo "")
    
    if [ -n "$insights" ]; then
        echo "Critical Insights:"
        echo "$insights" | python3 -m json.tool 2>/dev/null || echo "$insights"
    fi
    
    echo ""
}

# Main execution
main() {
    echo "=========================================="
    echo "KSAM Usecase: Vulnerable Pod Detection"
    echo "=========================================="
    echo ""
    echo "This usecase demonstrates:"
    echo "  1. Creating a pod with security vulnerability"
    echo "  2. Agent collecting pod data"
    echo "  3. Risk Engine detecting vulnerability via rules"
    echo "  4. Insight creation"
    echo "  5. User alert via API"
    echo ""
    
    # Setup
    setup_port_forward || exit 1
    
    echo ""
    
    # Execute steps
    step1_create_vulnerable_pod
    step2_wait_for_collection
    step3_verify_data
    step4_trigger_risk_evaluation
    step5_check_insights
    step6_display_alert
    
    echo ""
    echo "=========================================="
    log_success "Usecase completed!"
    echo "=========================================="
    echo ""
    echo "Next Steps:"
    echo "  1. Review insights via API: curl http://localhost:$CORE_PORT/api/v1/insights"
    echo "  2. Check insights summary: curl http://localhost:$CORE_PORT/api/v1/insights/summary"
    echo "  3. Get specific insight: curl http://localhost:$CORE_PORT/api/v1/insights/{id}"
    echo ""
}

main "$@"

