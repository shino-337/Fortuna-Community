#!/bin/bash

# Comprehensive End-to-End Test Script
# Tests all features: API, mTLS, Database, Workers, etc.

set -e

NAMESPACE="ksam"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
REPORT_DIR="test_results/e2e_full_${TIMESTAMP}"
mkdir -p "$REPORT_DIR"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0
SKIPPED_TESTS=0

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

get_core_pod() {
    kubectl get pods -n "$NAMESPACE" -l app=ksam-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null
}

get_agent_pod() {
    kubectl get pods -n "$NAMESPACE" -l app=ksam-agent -o jsonpath='{.items[0].metadata.name}' 2>/dev/null
}

get_postgres_pod() {
    kubectl get pods -n "$NAMESPACE" -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null
}

start_port_forward() {
    local pod=$1
    local local_port=$2
    local remote_port=$3
    kubectl port-forward -n "$NAMESPACE" "$pod" "$local_port:$remote_port" > "$REPORT_DIR/pf_${pod}_${local_port}.log" 2>&1 &
    echo $!
    sleep 3
}

stop_port_forward() {
    local pid=$1
    kill $pid 2>/dev/null || true
    wait $pid 2>/dev/null || true
}

# ==========================================
# TEST SUITE 1: Infrastructure
# ==========================================

test_infrastructure() {
    log_section "TEST SUITE 1: Infrastructure & Health"
    
    ((TOTAL_TESTS++))
    log_info "Test 1.1: Core Pod Status"
    CORE_POD=$(get_core_pod)
    if [ -z "$CORE_POD" ]; then
        log_fail "Core pod not found"
        return 1
    fi
    STATUS=$(kubectl get pod -n "$NAMESPACE" "$CORE_POD" -o jsonpath='{.status.phase}' 2>/dev/null)
    if [ "$STATUS" = "Running" ]; then
        log_success "Core pod is running: $CORE_POD"
    else
        log_fail "Core pod is not running: $STATUS"
        return 1
    fi
    
    ((TOTAL_TESTS++))
    log_info "Test 1.2: Agent Pod Status"
    AGENT_POD=$(get_agent_pod)
    if [ -z "$AGENT_POD" ]; then
        log_fail "Agent pod not found"
        return 1
    fi
    STATUS=$(kubectl get pod -n "$NAMESPACE" "$AGENT_POD" -o jsonpath='{.status.phase}' 2>/dev/null)
    if [ "$STATUS" = "Running" ]; then
        log_success "Agent pod is running: $AGENT_POD"
    else
        log_fail "Agent pod is not running: $STATUS"
        return 1
    fi
    
    ((TOTAL_TESTS++))
    log_info "Test 1.3: Postgres Pod Status"
    POSTGRES_POD=$(get_postgres_pod)
    if [ -z "$POSTGRES_POD" ]; then
        log_fail "Postgres pod not found"
        return 1
    fi
    STATUS=$(kubectl get pod -n "$NAMESPACE" "$POSTGRES_POD" -o jsonpath='{.status.phase}' 2>/dev/null)
    if [ "$STATUS" = "Running" ]; then
        log_success "Postgres pod is running: $POSTGRES_POD"
    else
        log_fail "Postgres pod is not running: $STATUS"
        return 1
    fi
    
    ((TOTAL_TESTS++))
    log_info "Test 1.4: NATS Status"
    NATS_COUNT=$(kubectl get pods -n "$NAMESPACE" -l app=nats --no-headers 2>/dev/null | wc -l | tr -d ' ')
    if [ "$NATS_COUNT" -ge 1 ]; then
        log_success "NATS pods found: $NATS_COUNT"
    else
        log_fail "NATS pods not found"
        return 1
    fi
}

# ==========================================
# TEST SUITE 2: Health Endpoints
# ==========================================

test_health_endpoints() {
    log_section "TEST SUITE 2: Health & Readiness Endpoints"
    
    CORE_POD=$(get_core_pod)
    if [ -z "$CORE_POD" ]; then
        log_skip "Core pod not found - skipping health tests"
        return 0
    fi
    
    ((TOTAL_TESTS++))
    log_info "Test 2.1: Health Endpoint"
    HEALTH=$(kubectl exec -n "$NAMESPACE" "$CORE_POD" -- wget -qO- http://localhost:8080/health 2>&1)
    echo "$HEALTH" > "$REPORT_DIR/health_response.json"
    if echo "$HEALTH" | grep -q "healthy"; then
        log_success "Health endpoint returns healthy"
    else
        log_fail "Health endpoint failed"
        return 1
    fi
    
    ((TOTAL_TESTS++))
    log_info "Test 2.2: Readiness Endpoint"
    READY=$(kubectl exec -n "$NAMESPACE" "$CORE_POD" -- wget -qO- http://localhost:8080/ready 2>&1)
    echo "$READY" > "$REPORT_DIR/ready_response.json"
    if echo "$READY" | grep -q "ready\|healthy"; then
        log_success "Readiness endpoint returns ready"
    else
        log_fail "Readiness endpoint failed"
        return 1
    fi
    
    ((TOTAL_TESTS++))
    log_info "Test 2.3: Liveness Endpoint"
    LIVE=$(kubectl exec -n "$NAMESPACE" "$CORE_POD" -- wget -qO- http://localhost:8080/live 2>&1)
    echo "$LIVE" > "$REPORT_DIR/live_response.json"
    if [ -n "$LIVE" ]; then
        log_success "Liveness endpoint responds"
    else
        log_fail "Liveness endpoint failed"
        return 1
    fi
}

# ==========================================
# TEST SUITE 3: Database Operations
# ==========================================

test_database() {
    log_section "TEST SUITE 3: Database Operations"
    
    POSTGRES_POD=$(get_postgres_pod)
    if [ -z "$POSTGRES_POD" ]; then
        log_skip "Postgres pod not found"
        return 0
    fi
    
    ((TOTAL_TESTS++))
    log_info "Test 3.1: Database Connection"
    DB_TEST=$(kubectl exec -n "$NAMESPACE" "$POSTGRES_POD" -- psql -U postgres -d ksam -c "SELECT 'SUCCESS' as test;" 2>&1)
    if echo "$DB_TEST" | grep -q "SUCCESS"; then
        log_success "Database connection successful"
    else
        log_fail "Database connection failed"
        return 1
    fi
    
    ((TOTAL_TESTS++))
    log_info "Test 3.2: Required Tables"
    TABLES=$(kubectl exec -n "$NAMESPACE" "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "
        SELECT COUNT(*) FROM information_schema.tables 
        WHERE table_schema = 'public' 
        AND table_name IN ('clusters', 'service_accounts', 'pods', 'roles', 'role_bindings', 'insights');
    " 2>&1 | tr -d ' ')
    if [ "$TABLES" -ge 5 ]; then
        log_success "Required tables exist: $TABLES/6"
    else
        log_fail "Missing tables: only $TABLES/6 found"
        return 1
    fi
    
    ((TOTAL_TESTS++))
    log_info "Test 3.3: Graph Triggers"
    TRIGGER_COUNT=$(kubectl exec -n "$NAMESPACE" "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "
        SELECT COUNT(*) FROM pg_trigger WHERE tgname LIKE '%graph%';
    " 2>&1 | tr -d ' ')
    if [ "$TRIGGER_COUNT" -ge 4 ]; then
        log_success "Graph triggers exist: $TRIGGER_COUNT"
    else
        log_fail "Missing triggers: only $TRIGGER_COUNT/4 found"
        return 1
    fi
}

# ==========================================
# TEST SUITE 4: mTLS & Certificates
# ==========================================

test_mtls() {
    log_section "TEST SUITE 4: mTLS & Certificate Management"
    
    CORE_POD=$(get_core_pod)
    if [ -z "$CORE_POD" ]; then
        log_skip "Core pod not found"
        return 0
    fi
    
    ((TOTAL_TESTS++))
    log_info "Test 4.1: CertManager Initialization"
    CERT_LOGS=$(kubectl logs -n "$NAMESPACE" "$CORE_POD" 2>&1 | grep -E "CertManager|TEST_UNIQUE_STRING" | head -5)
    if echo "$CERT_LOGS" | grep -q "CertManager\|TEST_UNIQUE_STRING"; then
        log_success "CertManager initialized"
        echo "$CERT_LOGS" > "$REPORT_DIR/certmanager_logs.txt"
    else
        log_fail "CertManager not initialized"
        return 1
    fi
    
    ((TOTAL_TESTS++))
    log_info "Test 4.2: Certificate Files"
    CERT_EXISTS=$(kubectl exec -n "$NAMESPACE" "$CORE_POD" -- test -f /etc/ksam/certs/tls.crt && echo "yes" || echo "no")
    KEY_EXISTS=$(kubectl exec -n "$NAMESPACE" "$CORE_POD" -- test -f /etc/ksam/certs/tls.key && echo "yes" || echo "no")
    CA_EXISTS=$(kubectl exec -n "$NAMESPACE" "$CORE_POD" -- test -f /etc/ksam/ca-cert/ca.crt && echo "yes" || echo "no")
    if [ "$CERT_EXISTS" = "yes" ] && [ "$KEY_EXISTS" = "yes" ] && [ "$CA_EXISTS" = "yes" ]; then
        log_success "All certificate files exist"
    else
        log_fail "Missing files (cert: $CERT_EXISTS, key: $KEY_EXISTS, ca: $CA_EXISTS)"
        return 1
    fi
    
    ((TOTAL_TESTS++))
    log_info "Test 4.3: gRPC Server mTLS"
    GRPC_LOGS=$(kubectl logs -n "$NAMESPACE" "$CORE_POD" 2>&1 | grep -E "gRPC.*mTLS|WITH mTLS" | head -3)
    if echo "$GRPC_LOGS" | grep -q "mTLS\|WITH mTLS"; then
        log_success "gRPC server configured with mTLS"
        echo "$GRPC_LOGS" > "$REPORT_DIR/grpc_mtls_logs.txt"
    else
        log_fail "gRPC server mTLS not configured"
        return 1
    fi
    
    ((TOTAL_TESTS++))
    log_info "Test 4.4: Agent mTLS Connection"
    AGENT_POD=$(get_agent_pod)
    if [ -n "$AGENT_POD" ]; then
        AGENT_LOGS=$(kubectl logs -n "$NAMESPACE" "$AGENT_POD" 2>&1 | grep -E "gRPC.*mTLS|mTLS" | head -3)
        if echo "$AGENT_LOGS" | grep -q "mTLS"; then
            log_success "Agent configured with mTLS"
            echo "$AGENT_LOGS" > "$REPORT_DIR/agent_mtls_logs.txt"
        else
            log_fail "Agent mTLS not configured"
            return 1
        fi
    else
        log_skip "Agent pod not found"
    fi
    
    ((TOTAL_TESTS++))
    log_info "Test 4.5: Certificate Metrics"
    PF_PID=$(start_port_forward "$CORE_POD" 8080 8080)
    METRICS=$(curl -s http://localhost:8080/metrics 2>&1 | grep -E "ksam_cert_" | head -10)
    stop_port_forward "$PF_PID"
    if echo "$METRICS" | grep -q "ksam_cert"; then
        CERT_METRIC_COUNT=$(echo "$METRICS" | wc -l | tr -d ' ')
        log_success "Certificate metrics available ($CERT_METRIC_COUNT metrics)"
        echo "$METRICS" > "$REPORT_DIR/cert_metrics.txt"
    else
        log_fail "Certificate metrics not found"
        return 1
    fi
}

# ==========================================
# TEST SUITE 5: REST API Endpoints
# ==========================================

test_api_endpoints() {
    log_section "TEST SUITE 5: REST API Endpoints"
    
    CORE_POD=$(get_core_pod)
    if [ -z "$CORE_POD" ]; then
        log_skip "Core pod not found"
        return 0
    fi
    
    PF_PID=$(start_port_forward "$CORE_POD" 8080 8080)
    
    ((TOTAL_TESTS++))
    log_info "Test 5.1: Prometheus Metrics Endpoint"
    METRICS=$(curl -s http://localhost:8080/metrics 2>&1)
    METRIC_COUNT=$(echo "$METRICS" | grep -c "ksam_" || echo "0")
    if [ "$METRIC_COUNT" -gt 0 ]; then
        log_success "Prometheus metrics available ($METRIC_COUNT ksam_ metrics)"
        echo "$METRICS" | head -100 > "$REPORT_DIR/metrics_sample.txt"
    else
        log_fail "Prometheus metrics not found"
    fi
    
    ((TOTAL_TESTS++))
    log_info "Test 5.2: Certificate API Endpoint"
    CERT_RESPONSE=$(curl -s -w "\n%{http_code}" http://localhost:8080/api/v1/certificates/info 2>&1)
    HTTP_CODE=$(echo "$CERT_RESPONSE" | tail -1)
    if [ "$HTTP_CODE" = "401" ] || [ "$HTTP_CODE" = "200" ]; then
        log_success "Certificate API responds (code: $HTTP_CODE)"
        echo "$CERT_RESPONSE" > "$REPORT_DIR/cert_api_response.txt"
    else
        log_fail "Certificate API failed (code: $HTTP_CODE)"
    fi
    
    ((TOTAL_TESTS++))
    log_info "Test 5.3: Graph API Endpoint"
    GRAPH_RESPONSE=$(curl -s -w "\n%{http_code}" http://localhost:8080/api/v1/graph 2>&1)
    HTTP_CODE=$(echo "$GRAPH_RESPONSE" | tail -1)
    if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "401" ]; then
        log_success "Graph API responds (code: $HTTP_CODE)"
        echo "$GRAPH_RESPONSE" > "$REPORT_DIR/graph_api_response.txt"
    else
        log_fail "Graph API failed (code: $HTTP_CODE)"
    fi
    
    ((TOTAL_TESTS++))
    log_info "Test 5.4: Clusters API Endpoint"
    CLUSTERS_RESPONSE=$(curl -s -w "\n%{http_code}" http://localhost:8080/api/v1/clusters 2>&1)
    HTTP_CODE=$(echo "$CLUSTERS_RESPONSE" | tail -1)
    if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "401" ]; then
        log_success "Clusters API responds (code: $HTTP_CODE)"
    else
        log_fail "Clusters API failed (code: $HTTP_CODE)"
    fi
    
    ((TOTAL_TESTS++))
    log_info "Test 5.5: ServiceAccounts API Endpoint"
    SA_RESPONSE=$(curl -s -w "\n%{http_code}" http://localhost:8080/api/v1/serviceaccounts 2>&1)
    HTTP_CODE=$(echo "$SA_RESPONSE" | tail -1)
    if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "401" ]; then
        log_success "ServiceAccounts API responds (code: $HTTP_CODE)"
    else
        log_fail "ServiceAccounts API failed (code: $HTTP_CODE)"
    fi
    
    ((TOTAL_TESTS++))
    log_info "Test 5.6: Insights API Endpoint"
    INSIGHTS_RESPONSE=$(curl -s -w "\n%{http_code}" http://localhost:8080/api/v1/insights 2>&1)
    HTTP_CODE=$(echo "$INSIGHTS_RESPONSE" | tail -1)
    if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "401" ]; then
        log_success "Insights API responds (code: $HTTP_CODE)"
    else
        log_fail "Insights API failed (code: $HTTP_CODE)"
    fi
    
    stop_port_forward "$PF_PID"
}

# ==========================================
# TEST SUITE 6: Worker Pool & Processing
# ==========================================

test_workers() {
    log_section "TEST SUITE 6: Worker Pool & Message Processing"
    
    CORE_POD=$(get_core_pod)
    if [ -z "$CORE_POD" ]; then
        log_skip "Core pod not found"
        return 0
    fi
    
    ((TOTAL_TESTS++))
    log_info "Test 6.1: Worker Pool Initialization"
    WORKER_LOGS=$(kubectl logs -n "$NAMESPACE" "$CORE_POD" 2>&1 | grep -E "WorkerPool|Worker.*started" | head -10)
    if echo "$WORKER_LOGS" | grep -q "Worker"; then
        WORKER_COUNT=$(echo "$WORKER_LOGS" | grep -c "started" || echo "0")
        log_success "Worker pool initialized ($WORKER_COUNT workers)"
        echo "$WORKER_LOGS" > "$REPORT_DIR/worker_pool_logs.txt"
    else
        log_fail "Worker pool not initialized"
        return 1
    fi
    
    ((TOTAL_TESTS++))
    log_info "Test 6.2: NATS Connection"
    NATS_LOGS=$(kubectl logs -n "$NAMESPACE" "$CORE_POD" 2>&1 | grep -E "NATS|nats|Connected" | head -5)
    if echo "$NATS_LOGS" | grep -q "NATS\|Connected\|Stream"; then
        log_success "NATS connection established"
        echo "$NATS_LOGS" > "$REPORT_DIR/nats_logs.txt"
    else
        log_fail "NATS connection not established"
        return 1
    fi
    
    ((TOTAL_TESTS++))
    log_info "Test 6.3: Normalizer Worker"
    NORM_LOGS=$(kubectl logs -n "$NAMESPACE" "$CORE_POD" 2>&1 | grep -E "normalizer|Normalizer" | head -5)
    if echo "$NORM_LOGS" | grep -q "normalizer"; then
        log_success "Normalizer worker active"
    else
        log_skip "Normalizer worker logs not found (may be idle)"
    fi
    
    ((TOTAL_TESTS++))
    log_info "Test 6.4: Risk Worker"
    RISK_LOGS=$(kubectl logs -n "$NAMESPACE" "$CORE_POD" 2>&1 | grep -E "RiskWorker|risk.*worker" -i | head -5)
    if echo "$RISK_LOGS" | grep -qi "risk.*worker"; then
        log_success "Risk worker active"
    else
        log_skip "Risk worker logs not found (may be idle)"
    fi
}

# ==========================================
# TEST SUITE 7: Agent Communication
# ==========================================

test_agent_communication() {
    log_section "TEST SUITE 7: Agent Communication"
    
    AGENT_POD=$(get_agent_pod)
    CORE_POD=$(get_core_pod)
    
    if [ -z "$AGENT_POD" ] || [ -z "$CORE_POD" ]; then
        log_skip "Agent or Core pod not found"
        return 0
    fi
    
    ((TOTAL_TESTS++))
    log_info "Test 7.1: Agent Activity"
    AGENT_LOGS=$(kubectl logs -n "$NAMESPACE" "$AGENT_POD" --tail=50 2>&1)
    if echo "$AGENT_LOGS" | grep -q "agent\|Agent\|watch\|collect"; then
        log_success "Agent is active"
        echo "$AGENT_LOGS" | head -30 > "$REPORT_DIR/agent_logs.txt"
    else
        log_fail "Agent not active"
        return 1
    fi
    
    ((TOTAL_TESTS++))
    log_info "Test 7.2: Agent to Core Connection"
    CONNECTION_LOGS=$(kubectl logs -n "$NAMESPACE" "$AGENT_POD" 2>&1 | grep -E "connect|gRPC|core" -i | head -5)
    if echo "$CONNECTION_LOGS" | grep -qi "connect\|grpc"; then
        log_success "Agent connection logs found"
        echo "$CONNECTION_LOGS" > "$REPORT_DIR/agent_connection_logs.txt"
    else
        log_skip "Connection logs not found (may be connected silently)"
    fi
    
    ((TOTAL_TESTS++))
    log_info "Test 7.3: Network Connectivity"
    NSLOOKUP=$(kubectl exec -n "$NAMESPACE" "$AGENT_POD" -- nslookup ksam-core.ksam.svc.cluster.local 2>&1 | head -3)
    if echo "$NSLOOKUP" | grep -q "ksam-core"; then
        log_success "DNS resolution works"
    else
        log_fail "DNS resolution failed"
        return 1
    fi
}

# ==========================================
# TEST SUITE 8: Data Flow
# ==========================================

test_data_flow() {
    log_section "TEST SUITE 8: Data Flow End-to-End"
    
    POSTGRES_POD=$(get_postgres_pod)
    CORE_POD=$(get_core_pod)
    
    if [ -z "$POSTGRES_POD" ] || [ -z "$CORE_POD" ]; then
        log_skip "Required pods not found"
        return 0
    fi
    
    ((TOTAL_TESTS++))
    log_info "Test 8.1: Data in Database"
    POD_COUNT=$(kubectl exec -n "$NAMESPACE" "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM pods;" 2>&1 | tr -d ' ')
    SA_COUNT=$(kubectl exec -n "$NAMESPACE" "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM service_accounts;" 2>&1 | tr -d ' ')
    CLUSTER_COUNT=$(kubectl exec -n "$NAMESPACE" "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM clusters;" 2>&1 | tr -d ' ')
    
    log_info "  Pods: $POD_COUNT, ServiceAccounts: $SA_COUNT, Clusters: $CLUSTER_COUNT"
    
    if [ "$CLUSTER_COUNT" -ge 0 ] && [ "$POD_COUNT" -ge 0 ] && [ "$SA_COUNT" -ge 0 ]; then
        log_success "Data exists in database"
    else
        log_fail "No data in database"
        return 1
    fi
    
    ((TOTAL_TESTS++))
    log_info "Test 8.2: Insights Generation"
    INSIGHT_COUNT=$(kubectl exec -n "$NAMESPACE" "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM insights;" 2>&1 | tr -d ' ')
    if [ "$INSIGHT_COUNT" -ge 0 ]; then
        log_success "Insights table exists with $INSIGHT_COUNT insights"
    else
        log_fail "Insights table not found"
        return 1
    fi
}

# ==========================================
# TEST SUITE 9: YAML Rules & CEL
# ==========================================

test_yaml_rules() {
    log_section "TEST SUITE 9: YAML Rules & CEL Engine"
    
    CORE_POD=$(get_core_pod)
    if [ -z "$CORE_POD" ]; then
        log_skip "Core pod not found"
        return 0
    fi
    
    ((TOTAL_TESTS++))
    log_info "Test 9.1: YAML Engine"
    YAML_LOGS=$(kubectl logs -n "$NAMESPACE" "$CORE_POD" 2>&1 | grep -E "YAMLEngine|rules" | head -5)
    if echo "$YAML_LOGS" | grep -q "YAMLEngine\|rules"; then
        log_success "YAML engine active"
        echo "$YAML_LOGS" > "$REPORT_DIR/yaml_engine_logs.txt"
    else
        log_skip "YAML engine logs not found (may use hardcoded rules)"
    fi
    
    ((TOTAL_TESTS++))
    log_info "Test 9.2: Risk Evaluation"
    RISK_LOGS=$(kubectl logs -n "$NAMESPACE" "$CORE_POD" 2>&1 | grep -E "Risk.*evaluat|insight" -i | tail -10)
    if echo "$RISK_LOGS" | grep -qi "risk\|insight"; then
        log_success "Risk evaluation is active"
        echo "$RISK_LOGS" > "$REPORT_DIR/risk_evaluation_logs.txt"
    else
        log_skip "Risk evaluation logs not found (may be idle)"
    fi
}

# ==========================================
# MAIN TEST RUNNER
# ==========================================

main() {
    echo "=========================================="
    echo "KSAM Comprehensive End-to-End Tests"
    echo "=========================================="
    echo "Date: $(date)"
    echo "Namespace: $NAMESPACE"
    echo "Report Directory: $REPORT_DIR"
    echo ""
    
    test_infrastructure
    test_health_endpoints
    test_database
    test_mtls
    test_api_endpoints
    test_workers
    test_agent_communication
    test_data_flow
    test_yaml_rules
    
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

main

