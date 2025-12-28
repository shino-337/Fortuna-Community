#!/bin/bash

# E2E Test Script: Pod with CVE - Complete Verification
# Tests SBOM extraction, CVE matching, insights, and policy engine

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
NAMESPACE="${NAMESPACE:-fortuna}"
PROJECT_ROOT="/Users/tuatnh/Desktop/Learn/K8s Service Account Management Platform/KSAM"

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

# Step 1: Create Test Pod
create_test_pod() {
    log_info "Step 1: Creating test pod with vulnerable image..."
    
    TEST_POD_NAME="test-pod-cve-$(date +%s)"
    cat > /tmp/test-pod-cve.yaml << EOF
apiVersion: v1
kind: Pod
metadata:
  name: $TEST_POD_NAME
  namespace: $NAMESPACE
  labels:
    app: test-pod-cve
    test: e2e-cve
spec:
  nodeSelector:
    kubernetes.io/hostname: minikube
  containers:
  - name: test-container
    image: nginx:1.25-alpine
    ports:
    - containerPort: 80
  restartPolicy: Never
EOF
    
    kubectl apply -f /tmp/test-pod-cve.yaml
    log_info "Waiting for pod to be ready..."
    kubectl wait --for=condition=Ready pod/$TEST_POD_NAME -n $NAMESPACE --timeout=60s
    
    POD_UID=$(kubectl get pod -n $NAMESPACE $TEST_POD_NAME -o jsonpath='{.metadata.uid}')
    log_success "Test pod created: $TEST_POD_NAME (UID: $POD_UID)"
    
    export TEST_POD_NAME
    export POD_UID
}

# Step 2: Wait for Processing
wait_for_processing() {
    log_info "Step 2: Waiting for SBOM processing (2 minutes)..."
    sleep 120
    log_success "Wait completed"
}

# Step 3: Check SBOM
check_sbom() {
    log_info "Step 3: Checking SBOM in database..."
    
    POSTGRES_POD=$(kubectl get pods -n $NAMESPACE -l app=postgres -o jsonpath='{.items[0].metadata.name}')
    
    SBOM_ID=$(kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -t -A -c \
        "SELECT id FROM sboms WHERE pod_uid = '$POD_UID' OR pod_name = '$TEST_POD_NAME' ORDER BY created_at DESC LIMIT 1;" 2>/dev/null | tr -d ' ')
    
    if [ -z "$SBOM_ID" ] || [ "$SBOM_ID" = "" ]; then
        log_warning "SBOM not found yet"
        return 1
    fi
    
    COMPONENT_COUNT=$(kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -t -A -c \
        "SELECT COUNT(*) FROM sbom_components WHERE sbom_id = $SBOM_ID;" 2>/dev/null | tr -d ' ')
    
    log_success "SBOM found: ID=$SBOM_ID, Components=$COMPONENT_COUNT"
    
    export SBOM_ID
    return 0
}

# Step 4: Check CVE Matches
check_cve_matches() {
    log_info "Step 4: Checking CVE matches..."
    
    if [ -z "$SBOM_ID" ]; then
        log_warning "SBOM ID not available, skipping CVE match check"
        return 1
    fi
    
    POSTGRES_POD=$(kubectl get pods -n $NAMESPACE -l app=postgres -o jsonpath='{.items[0].metadata.name}')
    
    CVE_MATCH_COUNT=$(kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -t -A -c \
        "SELECT COUNT(*) FROM cve_matches WHERE sbom_id = $SBOM_ID;" 2>/dev/null | tr -d ' ')
    
    if [ "$CVE_MATCH_COUNT" -gt 0 ]; then
        log_success "CVE matches found: $CVE_MATCH_COUNT"
        
        # Show sample matches
        log_info "Sample CVE matches:"
        kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -c \
            "SELECT cve_id, package_name, severity, cvss_score FROM cve_matches WHERE sbom_id = $SBOM_ID ORDER BY severity DESC, cvss_score DESC LIMIT 5;" 2>&1 | head -10
    else
        log_warning "No CVE matches found"
    fi
    
    export CVE_MATCH_COUNT
}

# Step 5: Check Insights
check_insights() {
    log_info "Step 5: Checking insights in database..."
    
    POSTGRES_POD=$(kubectl get pods -n $NAMESPACE -l app=postgres -o jsonpath='{.items[0].metadata.name}')
    
    INSIGHT_COUNT=$(kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -t -A -c \
        "SELECT COUNT(*) FROM insights WHERE resource_uid = '$POD_UID';" 2>/dev/null | tr -d ' ')
    
    if [ "$INSIGHT_COUNT" -gt 0 ]; then
        log_success "Insights found: $INSIGHT_COUNT"
        
        # Show insight breakdown
        log_info "Insights by type and severity:"
        kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -c \
            "SELECT insight_type, severity, COUNT(*) as count FROM insights WHERE resource_uid = '$POD_UID' GROUP BY insight_type, severity ORDER BY severity DESC;" 2>&1 | head -10
    else
        log_warning "No insights found"
    fi
    
    export INSIGHT_COUNT
}

# Step 6: Check API
check_api() {
    log_info "Step 6: Checking API insights..."
    
    CORE_POD=$(kubectl get pods -n $NAMESPACE -l 'app.kubernetes.io/component=core' -o jsonpath='{.items[0].metadata.name}')
    
    kubectl port-forward -n $NAMESPACE $CORE_POD 8080:8080 > /tmp/port-forward-api.log 2>&1 &
    PORT_FORWARD_PID=$!
    sleep 3
    
    # Health check
    HEALTH=$(curl -s http://localhost:8080/health 2>/dev/null)
    if echo "$HEALTH" | grep -q "healthy"; then
        log_success "API health check passed"
    else
        log_warning "API health check failed"
    fi
    
    # Get insights
    API_INSIGHTS=$(curl -s "http://localhost:8080/api/v1/insights?resource_uid=$POD_UID" 2>/dev/null)
    API_COUNT=$(echo "$API_INSIGHTS" | python3 -c "import sys, json; d=json.load(sys.stdin); print(len(d.get('insights', [])))" 2>/dev/null || echo "0")
    
    if [ "$API_COUNT" -gt 0 ]; then
        log_success "API returned $API_COUNT insights"
    else
        log_warning "API returned 0 insights"
    fi
    
    kill $PORT_FORWARD_PID 2>/dev/null || true
}

# Step 7: Check Policy Engine
check_policy_engine() {
    log_info "Step 7: Checking policy engine..."
    
    POSTGRES_POD=$(kubectl get pods -n $NAMESPACE -l app=postgres -o jsonpath='{.items[0].metadata.name}')
    
    POLICY_COUNT=$(kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -t -A -c \
        "SELECT COUNT(*) FROM policy_instances WHERE enabled = true;" 2>/dev/null | tr -d ' ')
    
    VIOLATION_COUNT=$(kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -t -A -c \
        "SELECT COUNT(*) FROM policy_violations;" 2>/dev/null | tr -d ' ')
    
    log_info "Active policies: $POLICY_COUNT"
    log_info "Policy violations: $VIOLATION_COUNT"
    
    if [ "$POLICY_COUNT" -gt 0 ]; then
        log_success "Policy engine is configured"
    else
        log_warning "No active policies found"
    fi
}

# Main
main() {
    echo ""
    echo "=========================================="
    echo "E2E Test: Pod with CVE - Complete Test"
    echo "=========================================="
    echo ""
    
    create_test_pod
    wait_for_processing
    check_sbom
    check_cve_matches
    check_insights
    check_api
    check_policy_engine
    
    echo ""
    echo "=========================================="
    log_success "Test completed!"
    echo "=========================================="
    echo ""
    echo "Test Pod: $TEST_POD_NAME"
    echo "Pod UID: $POD_UID"
    if [ -n "$SBOM_ID" ]; then
        echo "SBOM ID: $SBOM_ID"
        echo "CVE Matches: ${CVE_MATCH_COUNT:-0}"
        echo "Insights: ${INSIGHT_COUNT:-0}"
    fi
    echo ""
}

main "$@"

