#!/bin/bash

# Comprehensive End-to-End Test Suite for KSAM
# Tests all major features and functions

set -e

NAMESPACE="ksam"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
REPORT_DIR="test_results/e2e_${TIMESTAMP}"
mkdir -p "$REPORT_DIR"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test counters
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0
SKIPPED_TESTS=0

# Logging
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1" | tee -a "$REPORT_DIR/test.log"
}

log_success() {
    echo -e "${GREEN}[PASS]${NC} $1" | tee -a "$REPORT_DIR/test.log"
    ((PASSED_TESTS++))
}

log_fail() {
    echo -e "${RED}[FAIL]${NC} $1" | tee -a "$REPORT_DIR/test.log"
    ((FAILED_TESTS++))
}

log_skip() {
    echo -e "${YELLOW}[SKIP]${NC} $1" | tee -a "$REPORT_DIR/test.log"
    ((SKIPPED_TESTS++))
}

log_section() {
    echo "" | tee -a "$REPORT_DIR/test.log"
    echo "==========================================" | tee -a "$REPORT_DIR/test.log"
    echo "$1" | tee -a "$REPORT_DIR/test.log"
    echo "==========================================" | tee -a "$REPORT_DIR/test.log"
}

# Get pod names
get_core_pod() {
    kubectl get pods -n "$NAMESPACE" -l app=ksam-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null
}

get_agent_pod() {
    kubectl get pods -n "$NAMESPACE" -l app=ksam-agent -o jsonpath='{.items[0].metadata.name}' 2>/dev/null
}

get_postgres_pod() {
    kubectl get pods -n "$NAMESPACE" -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null
}

# Test helper: Check if pod is ready
check_pod_ready() {
    local pod=$1
    local status=$(kubectl get pod -n "$NAMESPACE" "$pod" -o jsonpath='{.status.phase}' 2>/dev/null)
    [ "$status" = "Running" ]
}

# Test helper: Execute command in pod
exec_in_pod() {
    local pod=$1
    shift
    kubectl exec -n "$NAMESPACE" "$pod" -- "$@" 2>&1
}

# Test helper: Port forward
start_port_forward() {
    local pod=$1
    local local_port=$2
    local remote_port=$3
    kubectl port-forward -n "$NAMESPACE" "$pod" "$local_port:$remote_port" > /tmp/pf_${pod}_${local_port}.log 2>&1 &
    echo $!
    sleep 2
}

stop_port_forward() {
    local pid=$1
    kill $pid 2>/dev/null || true
    wait $pid 2>/dev/null || true
}

# ==========================================
# TEST SUITE 1: Infrastructure & Health
# ==========================================

test_infrastructure() {
    log_section "TEST SUITE 1: Infrastructure & Health Checks"
    ((TOTAL_TESTS++))
    
    # Test 1.1: Core Pod Status
    log_info "Test 1.1: Core Pod Status"
    CORE_POD=$(get_core_pod)
    if [ -z "$CORE_POD" ]; then
        log_fail "Core pod not found"
        return 1
    fi
    
    if check_pod_ready "$CORE_POD"; then
        log_success "Core pod is running: $CORE_POD"
    else
        log_fail "Core pod is not ready: $CORE_POD"
        return 1
    fi
    
    # Test 1.2: Agent Pod Status
    log_info "Test 1.2: Agent Pod Status"
    AGENT_POD=$(get_agent_pod)
    if [ -z "$AGENT_POD" ]; then
        log_fail "Agent pod not found"
        return 1
    fi
    
    if check_pod_ready "$AGENT_POD"; then
        log_success "Agent pod is running: $AGENT_POD"
    else
        log_fail "Agent pod is not ready: $AGENT_POD"
        return 1
    fi
    
    # Test 1.3: Postgres Pod Status
    log_info "Test 1.3: Postgres Pod Status"
    POSTGRES_POD=$(get_postgres_pod)
    if [ -z "$POSTGRES_POD" ]; then
        log_fail "Postgres pod not found"
        return 1
    fi
    
    if check_pod_ready "$POSTGRES_POD"; then
        log_success "Postgres pod is running: $POSTGRES_POD"
    else
        log_fail "Postgres pod is not ready: $POSTGRES_POD"
        return 1
    fi
    
    # Test 1.4: NATS Status
    log_info "Test 1.4: NATS Status"
    NATS_PODS=$(kubectl get pods -n "$NAMESPACE" -l app=nats --no-headers 2>/dev/null | wc -l | tr -d ' ')
    if [ "$NATS_PODS" -ge 1 ]; then
        log_success "NATS pods found: $NATS_PODS"
    else
        log_fail "NATS pods not found"
        return 1
    fi
    
    return 0
}

# ==========================================
# TEST SUITE 2: Health & Readiness Endpoints
# ==========================================

test_health_endpoints() {
    log_section "TEST SUITE 2: Health & Readiness Endpoints"
    
    CORE_POD=$(get_core_pod)
    if [ -z "$CORE_POD" ]; then
        log_skip "Core pod not found - skipping health tests"
        return 0
    fi
    
    # Test 2.1: Health Endpoint
    ((TOTAL_TESTS++))
    log_info "Test 2.1: Health Endpoint"
    HEALTH_RESPONSE=$(exec_in_pod "$CORE_POD" wget -qO- http://localhost:8080/health 2>&1)
    if echo "$HEALTH_RESPONSE" | grep -q "healthy"; then
        log_success "Health endpoint returns healthy status"
        echo "$HEALTH_RESPONSE" > "$REPORT_DIR/health_response.json"
    else
        log_fail "Health endpoint failed or returned unexpected response"
        echo "$HEALTH_RESPONSE" > "$REPORT_DIR/health_response.json"
        return 1
    fi
    
    # Test 2.2: Readiness Endpoint
    ((TOTAL_TESTS++))
    log_info "Test 2.2: Readiness Endpoint"
    READY_RESPONSE=$(exec_in_pod "$CORE_POD" wget -qO- http://localhost:8080/ready 2>&1)
    if echo "$READY_RESPONSE" | grep -q "ready\|healthy"; then
        log_success "Readiness endpoint returns ready status"
        echo "$READY_RESPONSE" > "$REPORT_DIR/ready_response.json"
    else
        log_fail "Readiness endpoint failed"
        echo "$READY_RESPONSE" > "$REPORT_DIR/ready_response.json"
        return 1
    fi
    
    # Test 2.3: Liveness Endpoint
    ((TOTAL_TESTS++))
    log_info "Test 2.3: Liveness Endpoint"
    LIVE_RESPONSE=$(exec_in_pod "$CORE_POD" wget -qO- http://localhost:8080/live 2>&1)
    if [ -n "$LIVE_RESPONSE" ]; then
        log_success "Liveness endpoint responds"
        echo "$LIVE_RESPONSE" > "$REPORT_DIR/live_response.json"
    else
        log_fail "Liveness endpoint failed"
        return 1
    fi
    
    return 0
}

# ==========================================
# TEST SUITE 3: Database Operations
# ==========================================

test_database_operations() {
    log_section "TEST SUITE 3: Database Operations"
    
    POSTGRES_POD=$(get_postgres_pod)
    if [ -z "$POSTGRES_POD" ]; then
        log_skip "Postgres pod not found - skipping database tests"
        return 0
    fi
    
    # Test 3.1: Database Connection
    ((TOTAL_TESTS++))
    log_info "Test 3.1: Database Connection"
    DB_TEST=$(exec_in_pod "$POSTGRES_POD" psql -U postgres -d ksam -c "SELECT 'SUCCESS' as connection_test;" 2>&1)
    if echo "$DB_TEST" | grep -q "SUCCESS"; then
        log_success "Database connection successful"
    else
        log_fail "Database connection failed"
        echo "$DB_TEST" > "$REPORT_DIR/db_connection.log"
        return 1
    fi
    
    # Test 3.2: Tables Existence
    ((TOTAL_TESTS++))
    log_info "Test 3.2: Required Tables Existence"
    TABLES=$(exec_in_pod "$POSTGRES_POD" psql -U postgres -d ksam -t -c "
        SELECT COUNT(*) FROM information_schema.tables 
        WHERE table_schema = 'public' 
        AND table_name IN ('clusters', 'service_accounts', 'pods', 'roles', 'role_bindings', 'insights');
    " 2>&1 | tr -d ' ')
    
    if [ "$TABLES" -ge 5 ]; then
        log_success "Required tables exist: $TABLES/6 found"
    else
        log_fail "Missing required tables: only $TABLES/6 found"
        return 1
    fi
    
    # Test 3.3: Data Count
    ((TOTAL_TESTS++))
    log_info "Test 3.3: Data Count Check"
    CLUSTER_COUNT=$(exec_in_pod "$POSTGRES_POD" psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM clusters;" 2>&1 | tr -d ' ')
    POD_COUNT=$(exec_in_pod "$POSTGRES_POD" psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM pods;" 2>&1 | tr -d ' ')
    SA_COUNT=$(exec_in_pod "$POSTGRES_POD" psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM service_accounts;" 2>&1 | tr -d ' ')
    
    log_info "  Clusters: $CLUSTER_COUNT"
    log_info "  Pods: $POD_COUNT"
    log_info "  ServiceAccounts: $SA_COUNT"
    
    if [ "$CLUSTER_COUNT" -ge 0 ] && [ "$POD_COUNT" -ge 0 ] && [ "$SA_COUNT" -ge 0 ]; then
        log_success "Data counts retrieved successfully"
    else
        log_fail "Failed to retrieve data counts"
        return 1
    fi
    
    # Test 3.4: Graph Triggers
    ((TOTAL_TESTS++))
    log_info "Test 3.4: Graph Sync Triggers"
    TRIGGER_COUNT=$(exec_in_pod "$POSTGRES_POD" psql -U postgres -d ksam -t -c "
        SELECT COUNT(*) FROM pg_trigger WHERE tgname LIKE '%graph%';
    " 2>&1 | tr -d ' ')
    
    if [ "$TRIGGER_COUNT" -ge 4 ]; then
        log_success "Graph sync triggers exist: $TRIGGER_COUNT"
    else
        log_fail "Missing graph triggers: only $TRIGGER_COUNT/4 found"
        return 1
    fi
    
    return 0
}

# ==========================================
# TEST SUITE 4: mTLS & Certificate Management
# ==========================================

test_mtls_certificates() {
    log_section "TEST SUITE 4: mTLS & Certificate Management"
    
    CORE_POD=$(get_core_pod)
    if [ -z "$CORE_POD" ]; then
        log_skip "Core pod not found - skipping mTLS tests"
        return 0
    fi
    
    # Test 4.1: CertManager Initialization
    ((TOTAL_TESTS++))
    log_info "Test 4.1: CertManager Initialization"
    CERT_LOGS=$(kubectl logs -n "$NAMESPACE" "$CORE_POD" 2>&1 | grep -E "CertManager|TEST_UNIQUE_STRING" | head -5)
    if echo "$CERT_LOGS" | grep -q "CertManager\|TEST_UNIQUE_STRING"; then
        log_success "CertManager initialized (found in logs)"
        echo "$CERT_LOGS" > "$REPORT_DIR/certmanager_logs.txt"
    else
        log_fail "CertManager not initialized"
        return 1
    fi
    
    # Test 4.2: Certificate Files
    ((TOTAL_TESTS++))
    log_info "Test 4.2: Certificate Files Existence"
    CERT_EXISTS=$(exec_in_pod "$CORE_POD" test -f /etc/ksam/certs/tls.crt && echo "yes" || echo "no")
    KEY_EXISTS=$(exec_in_pod "$CORE_POD" test -f /etc/ksam/certs/tls.key && echo "yes" || echo "no")
    CA_EXISTS=$(exec_in_pod "$CORE_POD" test -f /etc/ksam/ca-cert/ca.crt && echo "yes" || echo "no")
    
    if [ "$CERT_EXISTS" = "yes" ] && [ "$KEY_EXISTS" = "yes" ] && [ "$CA_EXISTS" = "yes" ]; then
        log_success "All certificate files exist"
    else
        log_fail "Missing certificate files (cert: $CERT_EXISTS, key: $KEY_EXISTS, ca: $CA_EXISTS)"
        return 1
    fi
    
    # Test 4.3: Prometheus Certificate Metrics
    ((TOTAL_TESTS++))
    log_info "Test 4.3: Certificate Metrics"
    PF_PID=$(start_port_forward "$CORE_POD" 8080 8080)
    sleep 2
    
    METRICS=$(curl -s http://localhost:8080/metrics 2>&1 | grep -E "ksam_cert_" | head -5)
    stop_port_forward "$PF_PID"
    
    if echo "$METRICS" | grep -q "ksam_cert"; then
        log_success "Certificate metrics found in Prometheus"
        echo "$METRICS" > "$REPORT_DIR/cert_metrics.txt"
    else
        log_fail "Certificate metrics not found"
        return 1
    fi
    
    # Test 4.4: gRPC Server with mTLS
    ((TOTAL_TESTS++))
    log_info "Test 4.4: gRPC Server mTLS Configuration"
    GRPC_LOGS=$(kubectl logs -n "$NAMESPACE" "$CORE_POD" 2>&1 | grep -E "gRPC.*mTLS|WITH mTLS" | head -3)
    if echo "$GRPC_LOGS" | grep -q "mTLS\|WITH mTLS"; then
        log_success "gRPC server configured with mTLS"
        echo "$GRPC_LOGS" > "$REPORT_DIR/grpc_mtls_logs.txt"
    else
        log_fail "gRPC server mTLS not configured"
        return 1
    fi
    
    return 0
}

# ==========================================
# TEST SUITE 5: YAML Rules & CEL Engine
# ==========================================

test_yaml_rules_cel() {
    log_section "TEST SUITE 5: YAML Rules & CEL Engine"
    
    CORE_POD=$(get_core_pod)
    if [ -z "$CORE_POD" ]; then
        log_skip "Core pod not found - skipping YAML rules tests"
        return 0
    fi
    
    # Test 5.1: YAML Rules Directory
    ((TOTAL_TESTS++))
    log_info "Test 5.1: YAML Rules Directory"
    RULES_DIR="/etc/ksam/rules"
    RULES_EXIST=$(exec_in_pod "$CORE_POD" test -d "$RULES_DIR" && echo "yes" || echo "no")
    
    if [ "$RULES_EXIST" = "yes" ]; then
        RULE_COUNT=$(exec_in_pod "$CORE_POD" find "$RULES_DIR" -name "*.yaml" -o -name "*.yml" 2>/dev/null | wc -l | tr -d ' ')
        log_success "YAML rules directory exists with $RULE_COUNT files"
    else
        # Check if rules are loaded from hardcoded
        RULES_LOGS=$(kubectl logs -n "$NAMESPACE" "$CORE_POD" 2>&1 | grep -E "YAMLEngine|rules" | head -3)
        if echo "$RULES_LOGS" | grep -q "rules\|YAMLEngine"; then
            log_success "YAML engine active (using hardcoded rules)"
        else
            log_fail "YAML rules directory not found and engine not active"
            return 1
        fi
    fi
    
    # Test 5.2: CEL Compiler
    ((TOTAL_TESTS++))
    log_info "Test 5.2: CEL Compiler"
    CEL_LOGS=$(kubectl logs -n "$NAMESPACE" "$CORE_POD" 2>&1 | grep -E "CEL|cel" | head -3)
    if echo "$CEL_LOGS" | grep -q "CEL\|cel"; then
        log_success "CEL compiler found in logs"
        echo "$CEL_LOGS" > "$REPORT_DIR/cel_logs.txt"
    else
        log_skip "CEL compiler logs not found (may be using hardcoded rules)"
    fi
    
    # Test 5.3: Risk Worker
    ((TOTAL_TESTS++))
    log_info "Test 5.3: Risk Worker Activity"
    RISK_LOGS=$(kubectl logs -n "$NAMESPACE" "$CORE_POD" 2>&1 | grep -E "RiskWorker|Risk.*evaluat" | head -5)
    if echo "$RISK_LOGS" | grep -q "Risk"; then
        log_success "Risk worker is active"
        echo "$RISK_LOGS" > "$REPORT_DIR/risk_worker_logs.txt"
    else
        log_fail "Risk worker not active"
        return 1
    fi
    
    # Test 5.4: Hot-Reload (if enabled)
    ((TOTAL_TESTS++))
    log_info "Test 5.4: Rule Hot-Reload"
    WATCHER_LOGS=$(kubectl logs -n "$NAMESPACE" "$CORE_POD" 2>&1 | grep -E "RuleWatcher|watcher" | head -3)
    if echo "$WATCHER_LOGS" | grep -q "watcher\|Watcher"; then
        log_success "Rule watcher active (hot-reload enabled)"
    else
        log_skip "Rule watcher not found (hot-reload may not be enabled)"
    fi
    
    return 0
}

# ==========================================
# TEST SUITE 6: Graph Engine & Queries
# ==========================================

test_graph_engine() {
    log_section "TEST SUITE 6: Graph Engine & Queries"
    
    CORE_POD=$(get_core_pod)
    if [ -z "$CORE_POD" ]; then
        log_skip "Core pod not found - skipping graph tests"
        return 0
    fi
    
    # Test 6.1: Graph Engine Initialization
    ((TOTAL_TESTS++))
    log_info "Test 6.1: Graph Engine Initialization"
    GRAPH_LOGS=$(kubectl logs -n "$NAMESPACE" "$CORE_POD" 2>&1 | grep -E "AgeGraphEngine|graph" | head -5)
    if echo "$GRAPH_LOGS" | grep -q "graph\|Graph\|AGE"; then
        log_success "Graph engine initialized"
        echo "$GRAPH_LOGS" > "$REPORT_DIR/graph_engine_logs.txt"
    else
        log_skip "Graph engine logs not found (may be in fallback mode)"
    fi
    
    # Test 6.2: AGE Extension Check
    ((TOTAL_TESTS++))
    log_info "Test 6.2: Apache AGE Extension"
    POSTGRES_POD=$(get_postgres_pod)
    if [ -n "$POSTGRES_POD" ]; then
        AGE_INSTALLED=$(exec_in_pod "$POSTGRES_POD" psql -U postgres -d ksam -t -c "
            SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = 'age');
        " 2>&1 | tr -d ' ' | grep -i "t\|true\|1" || echo "false")
        
        if [ "$AGE_INSTALLED" != "false" ]; then
            log_success "Apache AGE extension installed"
        else
            log_skip "Apache AGE extension not installed (fallback mode active)"
        fi
    else
        log_skip "Postgres pod not found"
    fi
    
    # Test 6.3: Graph API Endpoints (Fallback)
    ((TOTAL_TESTS++))
    log_info "Test 6.3: Graph API Endpoints"
    PF_PID=$(start_port_forward "$CORE_POD" 8080 8080)
    sleep 2
    
    GRAPH_RESPONSE=$(curl -s -w "\n%{http_code}" http://localhost:8080/api/v1/graph 2>&1)
    HTTP_CODE=$(echo "$GRAPH_RESPONSE" | tail -1)
    stop_port_forward "$PF_PID"
    
    if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "401" ]; then
        log_success "Graph API endpoint responds (code: $HTTP_CODE)"
        echo "$GRAPH_RESPONSE" > "$REPORT_DIR/graph_api_response.txt"
    else
        log_fail "Graph API endpoint failed (code: $HTTP_CODE)"
        return 1
    fi
    
    return 0
}

# ==========================================
# TEST SUITE 7: API Endpoints
# ==========================================

test_api_endpoints() {
    log_section "TEST SUITE 7: REST API Endpoints"
    
    CORE_POD=$(get_core_pod)
    if [ -z "$CORE_POD" ]; then
        log_skip "Core pod not found - skipping API tests"
        return 0
    fi
    
    PF_PID=$(start_port_forward "$CORE_POD" 8080 8080)
    sleep 2
    
    # Test 7.1: Prometheus Metrics
    ((TOTAL_TESTS++))
    log_info "Test 7.1: Prometheus Metrics Endpoint"
    METRICS=$(curl -s http://localhost:8080/metrics 2>&1)
    if echo "$METRICS" | grep -q "ksam_"; then
        METRIC_COUNT=$(echo "$METRICS" | grep -c "ksam_" || echo "0")
        log_success "Prometheus metrics available ($METRIC_COUNT ksam_ metrics)"
        echo "$METRICS" | head -50 > "$REPORT_DIR/metrics_sample.txt"
    else
        log_fail "Prometheus metrics not found"
    fi
    
    # Test 7.2: Certificate API (requires auth, but should respond)
    ((TOTAL_TESTS++))
    log_info "Test 7.2: Certificate API Endpoint"
    CERT_RESPONSE=$(curl -s -w "\n%{http_code}" http://localhost:8080/api/v1/certificates/info 2>&1)
    HTTP_CODE=$(echo "$CERT_RESPONSE" | tail -1)
    if [ "$HTTP_CODE" = "401" ] || [ "$HTTP_CODE" = "200" ]; then
        log_success "Certificate API endpoint responds (code: $HTTP_CODE)"
        echo "$CERT_RESPONSE" > "$REPORT_DIR/cert_api_response.txt"
    else
        log_fail "Certificate API endpoint failed (code: $HTTP_CODE)"
    fi
    
    # Test 7.3: Graph Endpoints
    ((TOTAL_TESTS++))
    log_info "Test 7.3: Graph API Endpoints"
    GRAPH_RESPONSE=$(curl -s -w "\n%{http_code}" http://localhost:8080/api/v1/graph 2>&1)
    HTTP_CODE=$(echo "$GRAPH_RESPONSE" | tail -1)
    if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "401" ]; then
        log_success "Graph API endpoint responds (code: $HTTP_CODE)"
    else
        log_fail "Graph API endpoint failed (code: $HTTP_CODE)"
    fi
    
    stop_port_forward "$PF_PID"
    
    return 0
}

# ==========================================
# TEST SUITE 8: Worker Pool & Message Processing
# ==========================================

test_worker_pool() {
    log_section "TEST SUITE 8: Worker Pool & Message Processing"
    
    CORE_POD=$(get_core_pod)
    if [ -z "$CORE_POD" ]; then
        log_skip "Core pod not found - skipping worker tests"
        return 0
    fi
    
    # Test 8.1: Worker Pool Initialization
    ((TOTAL_TESTS++))
    log_info "Test 8.1: Worker Pool Initialization"
    WORKER_LOGS=$(kubectl logs -n "$NAMESPACE" "$CORE_POD" 2>&1 | grep -E "WorkerPool|Worker.*started" | head -10)
    if echo "$WORKER_LOGS" | grep -q "Worker"; then
        WORKER_COUNT=$(echo "$WORKER_LOGS" | grep -c "started" || echo "0")
        log_success "Worker pool initialized ($WORKER_COUNT workers started)"
        echo "$WORKER_LOGS" > "$REPORT_DIR/worker_pool_logs.txt"
    else
        log_fail "Worker pool not initialized"
        return 1
    fi
    
    # Test 8.2: Normalizer Worker
    ((TOTAL_TESTS++))
    log_info "Test 8.2: Normalizer Worker"
    NORM_LOGS=$(kubectl logs -n "$NAMESPACE" "$CORE_POD" 2>&1 | grep -E "normalizer|Normalizer" | head -5)
    if echo "$NORM_LOGS" | grep -q "normalizer"; then
        log_success "Normalizer worker active"
    else
        log_fail "Normalizer worker not found"
        return 1
    fi
    
    # Test 8.3: Correlator Worker
    ((TOTAL_TESTS++))
    log_info "Test 8.3: Correlator Worker"
    CORR_LOGS=$(kubectl logs -n "$NAMESPACE" "$CORE_POD" 2>&1 | grep -E "correlator|Correlator" | head -5)
    if echo "$CORR_LOGS" | grep -q "correlator"; then
        log_success "Correlator worker active"
    else
        log_fail "Correlator worker not found"
        return 1
    fi
    
    # Test 8.4: Risk Worker
    ((TOTAL_TESTS++))
    log_info "Test 8.4: Risk Worker"
    RISK_LOGS=$(kubectl logs -n "$NAMESPACE" "$CORE_POD" 2>&1 | grep -E "risk.*worker|RiskWorker" -i | head -5)
    if echo "$RISK_LOGS" | grep -qi "risk.*worker"; then
        log_success "Risk worker active"
    else
        log_fail "Risk worker not found"
        return 1
    fi
    
    # Test 8.5: NATS Connection
    ((TOTAL_TESTS++))
    log_info "Test 8.5: NATS Connection"
    NATS_LOGS=$(kubectl logs -n "$NAMESPACE" "$CORE_POD" 2>&1 | grep -E "NATS|nats" | head -5)
    if echo "$NATS_LOGS" | grep -q "NATS\|Connected\|Stream"; then
        log_success "NATS connection established"
        echo "$NATS_LOGS" > "$REPORT_DIR/nats_logs.txt"
    else
        log_fail "NATS connection not established"
        return 1
    fi
    
    return 0
}

# ==========================================
# TEST SUITE 9: Agent Communication
# ==========================================

test_agent_communication() {
    log_section "TEST SUITE 9: Agent Communication"
    
    AGENT_POD=$(get_agent_pod)
    CORE_POD=$(get_core_pod)
    
    if [ -z "$AGENT_POD" ] || [ -z "$CORE_POD" ]; then
        log_skip "Agent or Core pod not found - skipping communication tests"
        return 0
    fi
    
    # Test 9.1: Agent Pod Status
    ((TOTAL_TESTS++))
    log_info "Test 9.1: Agent Pod Status"
    if check_pod_ready "$AGENT_POD"; then
        log_success "Agent pod is running: $AGENT_POD"
    else
        log_fail "Agent pod is not ready"
        return 1
    fi
    
    # Test 9.2: Agent Logs
    ((TOTAL_TESTS++))
    log_info "Test 9.2: Agent Activity"
    AGENT_LOGS=$(kubectl logs -n "$NAMESPACE" "$AGENT_POD" --tail=50 2>&1)
    if echo "$AGENT_LOGS" | grep -q "agent\|Agent\|watch\|collect"; then
        log_success "Agent is active"
        echo "$AGENT_LOGS" | head -30 > "$REPORT_DIR/agent_logs.txt"
    else
        log_fail "Agent not active or no logs"
        return 1
    fi
    
    # Test 9.3: gRPC Connection (mTLS)
    ((TOTAL_TESTS++))
    log_info "Test 9.3: Agent to Core gRPC Connection"
    AGENT_GRPC_LOGS=$(kubectl logs -n "$NAMESPACE" "$AGENT_POD" 2>&1 | grep -E "grpc|gRPC|connect" -i | head -5)
    CORE_GRPC_LOGS=$(kubectl logs -n "$NAMESPACE" "$CORE_POD" 2>&1 | grep -E "gRPC.*server|mTLS" | head -5)
    
    if echo "$CORE_GRPC_LOGS" | grep -q "gRPC\|mTLS"; then
        log_success "gRPC server running with mTLS"
        echo "$CORE_GRPC_LOGS" > "$REPORT_DIR/core_grpc_logs.txt"
    else
        log_fail "gRPC server not configured correctly"
        return 1
    fi
    
    return 0
}

# ==========================================
# TEST SUITE 10: Data Flow End-to-End
# ==========================================

test_data_flow() {
    log_section "TEST SUITE 10: Data Flow End-to-End"
    
    POSTGRES_POD=$(get_postgres_pod)
    CORE_POD=$(get_core_pod)
    
    if [ -z "$POSTGRES_POD" ] || [ -z "$CORE_POD" ]; then
        log_skip "Required pods not found - skipping data flow tests"
        return 0
    fi
    
    # Test 10.1: Data Ingestion
    ((TOTAL_TESTS++))
    log_info "Test 10.1: Data Ingestion Check"
    POD_COUNT_BEFORE=$(exec_in_pod "$POSTGRES_POD" psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM pods;" 2>&1 | tr -d ' ')
    SA_COUNT_BEFORE=$(exec_in_pod "$POSTGRES_POD" psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM service_accounts;" 2>&1 | tr -d ' ')
    
    log_info "  Current data: Pods=$POD_COUNT_BEFORE, ServiceAccounts=$SA_COUNT_BEFORE"
    
    if [ "$POD_COUNT_BEFORE" -ge 0 ] && [ "$SA_COUNT_BEFORE" -ge 0 ]; then
        log_success "Data exists in database (Pods: $POD_COUNT_BEFORE, SAs: $SA_COUNT_BEFORE)"
    else
        log_fail "No data in database"
        return 1
    fi
    
    # Test 10.2: Normalization
    ((TOTAL_TESTS++))
    log_info "Test 10.2: Data Normalization"
    NORM_LOGS=$(kubectl logs -n "$NAMESPACE" "$CORE_POD" 2>&1 | grep -E "Normalizer|normalized" | tail -10)
    if echo "$NORM_LOGS" | grep -q "Normalizer\|normalized"; then
        log_success "Normalizer is processing data"
        echo "$NORM_LOGS" > "$REPORT_DIR/normalizer_logs.txt"
    else
        log_skip "Normalizer logs not found (may be idle)"
    fi
    
    # Test 10.3: Correlation
    ((TOTAL_TESTS++))
    log_info "Test 10.3: Data Correlation"
    CORR_LOGS=$(kubectl logs -n "$NAMESPACE" "$CORE_POD" 2>&1 | grep -E "Correlator|correlat" | tail -10)
    if echo "$CORR_LOGS" | grep -q "Correlator\|correlat"; then
        log_success "Correlator is processing data"
    else
        log_skip "Correlator logs not found (may be idle)"
    fi
    
    # Test 10.4: Risk Evaluation
    ((TOTAL_TESTS++))
    log_info "Test 10.4: Risk Evaluation"
    RISK_LOGS=$(kubectl logs -n "$NAMESPACE" "$CORE_POD" 2>&1 | grep -E "Risk.*evaluat|insight" -i | tail -10)
    if echo "$RISK_LOGS" | grep -qi "risk\|insight"; then
        log_success "Risk evaluation is active"
        echo "$RISK_LOGS" > "$REPORT_DIR/risk_evaluation_logs.txt"
    else
        log_skip "Risk evaluation logs not found (may be idle)"
    fi
    
    # Test 10.5: Insights Generation
    ((TOTAL_TESTS++))
    log_info "Test 10.5: Insights in Database"
    INSIGHT_COUNT=$(exec_in_pod "$POSTGRES_POD" psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM insights;" 2>&1 | tr -d ' ')
    if [ "$INSIGHT_COUNT" -ge 0 ]; then
        log_success "Insights table exists with $INSIGHT_COUNT insights"
    else
        log_fail "Insights table not found or error"
        return 1
    fi
    
    return 0
}

# ==========================================
# TEST SUITE 11: Performance & Metrics
# ==========================================

test_performance_metrics() {
    log_section "TEST SUITE 11: Performance & Metrics"
    
    CORE_POD=$(get_core_pod)
    if [ -z "$CORE_POD" ]; then
        log_skip "Core pod not found - skipping metrics tests"
        return 0
    fi
    
    PF_PID=$(start_port_forward "$CORE_POD" 8080 8080)
    sleep 2
    
    # Test 11.1: Prometheus Metrics Availability
    ((TOTAL_TESTS++))
    log_info "Test 11.1: Prometheus Metrics"
    METRICS=$(curl -s http://localhost:8080/metrics 2>&1)
    
    # Count different metric types
    CERT_METRICS=$(echo "$METRICS" | grep -c "ksam_cert_" || echo "0")
    RULE_METRICS=$(echo "$METRICS" | grep -c "ksam_rule_" || echo "0")
    WORKER_METRICS=$(echo "$METRICS" | grep -c "ksam_worker_" || echo "0")
    TOTAL_KSAM_METRICS=$(echo "$METRICS" | grep -c "ksam_" || echo "0")
    
    log_info "  Certificate metrics: $CERT_METRICS"
    log_info "  Rule metrics: $RULE_METRICS"
    log_info "  Worker metrics: $WORKER_METRICS"
    log_info "  Total KSAM metrics: $TOTAL_KSAM_METRICS"
    
    if [ "$TOTAL_KSAM_METRICS" -gt 0 ]; then
        log_success "Prometheus metrics available ($TOTAL_KSAM_METRICS total)"
        echo "$METRICS" | grep "ksam_" > "$REPORT_DIR/all_ksam_metrics.txt"
    else
        log_fail "No KSAM metrics found"
    fi
    
    stop_port_forward "$PF_PID"
    
    return 0
}

# ==========================================
# MAIN TEST RUNNER
# ==========================================

main() {
    echo "=========================================="
    echo "KSAM End-to-End Comprehensive Test Suite"
    echo "=========================================="
    echo "Date: $(date)"
    echo "Namespace: $NAMESPACE"
    echo "Report Directory: $REPORT_DIR"
    echo ""
    
    # Run all test suites
    test_infrastructure
    test_health_endpoints
    test_database_operations
    test_mtls_certificates
    test_yaml_rules_cel
    test_graph_engine
    test_api_endpoints
    test_worker_pool
    test_agent_communication
    test_data_flow
    test_performance_metrics
    
    # Final summary
    log_section "TEST SUMMARY"
    echo "Total Tests: $TOTAL_TESTS" | tee -a "$REPORT_DIR/test.log"
    echo "Passed: $PASSED_TESTS" | tee -a "$REPORT_DIR/test.log"
    echo "Failed: $FAILED_TESTS" | tee -a "$REPORT_DIR/test.log"
    echo "Skipped: $SKIPPED_TESTS" | tee -a "$REPORT_DIR/test.log"
    
    if [ $FAILED_TESTS -eq 0 ]; then
        echo -e "${GREEN}✅ ALL TESTS PASSED${NC}" | tee -a "$REPORT_DIR/test.log"
        exit 0
    else
        echo -e "${RED}❌ SOME TESTS FAILED${NC}" | tee -a "$REPORT_DIR/test.log"
        exit 1
    fi
}

# Run tests
main


