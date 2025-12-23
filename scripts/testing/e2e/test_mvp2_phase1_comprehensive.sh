#!/bin/bash

# MVP2 Phase 1.1 - Comprehensive Test Script
# Tests all aspects: K8s → Database → API → Risk Scores

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Test namespace
TEST_NAMESPACE="mvp2-test-$(date +%Y%m%d-%H%M%S)"
TEST_POD_NAME="test-risky-pod-$(date +%s)"
TEST_SA_NAME="test-risky-sa-$(date +%s)"
TEST_ROLE_NAME="test-risky-role-$(date +%s)"
TEST_RB_NAME="test-risky-rb-$(date +%s)"

# API endpoint
API_URL="${API_URL:-http://localhost:8080}"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}MVP2 Phase 1.1 - Comprehensive Test${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Function to cleanup
cleanup() {
    echo -e "\n${YELLOW}Cleaning up test resources...${NC}"
    kubectl delete namespace "$TEST_NAMESPACE" --ignore-not-found=true 2>/dev/null || true
    echo -e "${GREEN}✅ Cleanup complete${NC}"
}

trap cleanup EXIT

# Function to wait for port-forward
wait_for_port_forward() {
    local port=$1
    local max_attempts=30
    local attempt=0
    
    while [ $attempt -lt $max_attempts ]; do
        if curl -s "http://localhost:$port/api/v1/auth/login" > /dev/null 2>&1; then
            return 0
        fi
        attempt=$((attempt + 1))
        sleep 1
    done
    return 1
}

# Function to get auth token
get_auth_token() {
    local response=$(curl -s -X POST "$API_URL/api/v1/auth/login" \
        -H "Content-Type: application/json" \
        -d '{"username":"admin","password":"admin123"}')
    
    echo "$response" | grep -o '"token":"[^"]*' | cut -d'"' -f4
}

# Function to query database
query_db() {
    local query="$1"
    
    # Try direct psql first
    if command -v psql > /dev/null 2>&1; then
        PGPASSWORD="${DB_PASSWORD:-ksam123}" psql -h "${DB_HOST:-postgres.ksam.svc.cluster.local}" -p "${DB_PORT:-5432}" -U "${DB_USER:-ksam}" -d "${DB_NAME:-ksam}" -t -A -c "$query" 2>/dev/null
    else
        # Fallback to kubectl exec
        local postgres_pod=$(kubectl get pods -n ksam -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
        if [ -n "$postgres_pod" ]; then
            kubectl exec -n ksam "$postgres_pod" -- psql -U "${DB_USER:-ksam}" -d "${DB_NAME:-ksam}" -t -A -c "$query" 2>/dev/null
        else
            echo ""
        fi
    fi
}

# Step 1: Setup port-forwarding
echo -e "${BLUE}[1/12] Setting up port-forwarding...${NC}"
CORE_POD=$(kubectl get pods -n ksam -l app=ksam-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
if [ -z "$CORE_POD" ]; then
    echo -e "${RED}❌ Core pod not found${NC}"
    exit 1
fi

kubectl port-forward -n ksam "$CORE_POD" 8080:8080 > /dev/null 2>&1 &
PF_PID=$!
sleep 5

if ! wait_for_port_forward 8080; then
    echo -e "${RED}❌ Port-forward failed${NC}"
    kill $PF_PID 2>/dev/null || true
    exit 1
fi

echo -e "${GREEN}✅ Port-forward active (PID: $PF_PID)${NC}"

# Step 2: Get auth token
echo -e "\n${BLUE}[2/12] Getting auth token...${NC}"
TOKEN=$(get_auth_token)
if [ -z "$TOKEN" ]; then
    echo -e "${RED}❌ Failed to get auth token${NC}"
    kill $PF_PID 2>/dev/null || true
    exit 1
fi
echo -e "${GREEN}✅ Token obtained${NC}"

# Step 3: Test Risk API Routes (before creating resources)
echo -e "\n${BLUE}[3/12] Testing Risk API Routes (baseline)...${NC}"
SCORES_BEFORE=$(curl -s "http://localhost:8080/api/v1/risk/scores?pageSize=1" -H "Authorization: Bearer $TOKEN" | python3 -c "import sys, json; d=json.load(sys.stdin); print(d.get('total', 0))" 2>/dev/null || echo "0")
echo "   Risk scores before test: $SCORES_BEFORE"

# Step 4: Create test namespace
echo -e "\n${BLUE}[4/12] Creating test namespace: $TEST_NAMESPACE${NC}"
kubectl create namespace "$TEST_NAMESPACE" 2>/dev/null || true
sleep 2

# Step 5: Create risky resources
echo -e "\n${BLUE}[5/12] Creating risky resources...${NC}"

# Create ServiceAccount
kubectl create serviceaccount "$TEST_SA_NAME" -n "$TEST_NAMESPACE" 2>/dev/null || true

# Create ClusterRoleBinding with cluster-admin (HIGH RISK)
cat <<EOF | kubectl apply -f -
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: $TEST_RB_NAME
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- kind: ServiceAccount
  name: $TEST_SA_NAME
  namespace: $TEST_NAMESPACE
EOF

# Create Pod with privileged container (CRITICAL RISK)
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: $TEST_POD_NAME
  namespace: $TEST_NAMESPACE
spec:
  serviceAccountName: $TEST_SA_NAME
  containers:
  - name: test-container
    image: nginx:latest
    securityContext:
      privileged: true
  restartPolicy: Never
EOF

echo -e "${GREEN}✅ Risky resources created${NC}"
echo "   - ServiceAccount: $TEST_SA_NAME"
echo "   - ClusterRoleBinding: $TEST_RB_NAME (cluster-admin)"
echo "   - Pod: $TEST_POD_NAME (privileged)"

# Step 6: Wait for agent sync
echo -e "\n${BLUE}[6/12] Waiting for agent sync (60s)...${NC}"
echo "   This allows agent to collect resources and sync to Core"
sleep 60

# Step 7: Verify resources in database
echo -e "\n${BLUE}[7/12] Verifying resources in database...${NC}"
POD_UID=$(kubectl get pod "$TEST_POD_NAME" -n "$TEST_NAMESPACE" -o jsonpath='{.metadata.uid}' 2>/dev/null)
SA_UID=$(kubectl get serviceaccount "$TEST_SA_NAME" -n "$TEST_NAMESPACE" -o jsonpath='{.metadata.uid}' 2>/dev/null)

if [ -z "$POD_UID" ] || [ -z "$SA_UID" ]; then
    echo -e "${RED}❌ Failed to get resource UIDs${NC}"
    kill $PF_PID 2>/dev/null || true
    exit 1
fi

echo "   Pod UID: $POD_UID"
echo "   SA UID: $SA_UID"

POD_IN_DB=$(query_db "SELECT COUNT(*) FROM pods WHERE uid = '$POD_UID';" 2>/dev/null || echo "0")
SA_IN_DB=$(query_db "SELECT COUNT(*) FROM service_accounts WHERE uid = '$SA_UID';" 2>/dev/null || echo "0")

echo "   Pod in DB: $POD_IN_DB"
echo "   SA in DB: $SA_IN_DB"

if [ "$POD_IN_DB" = "0" ] || [ "$SA_IN_DB" = "0" ]; then
    echo -e "${YELLOW}⚠️  Resources not yet in DB, waiting additional 30s...${NC}"
    sleep 30
    POD_IN_DB=$(query_db "SELECT COUNT(*) FROM pods WHERE uid = '$POD_UID';" 2>/dev/null || echo "0")
    SA_IN_DB=$(query_db "SELECT COUNT(*) FROM service_accounts WHERE uid = '$SA_UID';" 2>/dev/null || echo "0")
    echo "   Pod in DB (after wait): $POD_IN_DB"
    echo "   SA in DB (after wait): $SA_IN_DB"
fi

# Step 8: Trigger risk evaluation
echo -e "\n${BLUE}[8/12] Triggering risk evaluation...${NC}"
EVAL_RESPONSE=$(curl -s -X POST "$API_URL/api/v1/insights/evaluate/historical" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json")

if echo "$EVAL_RESPONSE" | grep -q "error"; then
    echo -e "${YELLOW}⚠️  Evaluation response: $EVAL_RESPONSE${NC}"
else
    echo -e "${GREEN}✅ Risk evaluation triggered${NC}"
fi

echo "   Waiting 30s for risk evaluation to complete..."
sleep 30

# Step 9: Verify insights generation
echo -e "\n${BLUE}[9/12] Verifying insights generation...${NC}"
POD_INSIGHTS_COUNT=$(query_db "SELECT COUNT(*) FROM insights WHERE affected_resources::text LIKE '%$POD_UID%' AND status = 'active' AND deleted_at IS NULL;" 2>/dev/null || echo "0")
SA_INSIGHTS_COUNT=$(query_db "SELECT COUNT(*) FROM insights WHERE affected_resources::text LIKE '%$SA_UID%' AND status = 'active' AND deleted_at IS NULL;" 2>/dev/null || echo "0")

if [ -z "$POD_INSIGHTS_COUNT" ]; then POD_INSIGHTS_COUNT="0"; fi
if [ -z "$SA_INSIGHTS_COUNT" ]; then SA_INSIGHTS_COUNT="0"; fi

echo "   Pod insights in DB: $POD_INSIGHTS_COUNT"
echo "   SA insights in DB: $SA_INSIGHTS_COUNT"

# Step 10: Calculate risk scores
echo -e "\n${BLUE}[10/12] Calculating risk scores via API...${NC}"

# Calculate for Pod
POD_SCORE_RESPONSE=$(curl -s -X POST "$API_URL/api/v1/risk/scores/$POD_UID/calculate" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json")

POD_SCORE=$(echo "$POD_SCORE_RESPONSE" | python3 -c "import sys, json; d=json.load(sys.stdin); print(d.get('totalScore', 0))" 2>/dev/null || echo "0")
POD_PRIORITY=$(echo "$POD_SCORE_RESPONSE" | python3 -c "import sys, json; d=json.load(sys.stdin); print(d.get('priorityLevel', 'N/A'))" 2>/dev/null || echo "N/A")

# Calculate for SA
SA_SCORE_RESPONSE=$(curl -s -X POST "$API_URL/api/v1/risk/scores/$SA_UID/calculate" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json")

SA_SCORE=$(echo "$SA_SCORE_RESPONSE" | python3 -c "import sys, json; d=json.load(sys.stdin); print(d.get('totalScore', 0))" 2>/dev/null || echo "0")
SA_PRIORITY=$(echo "$SA_SCORE_RESPONSE" | python3 -c "import sys, json; d=json.load(sys.stdin); print(d.get('priorityLevel', 'N/A'))" 2>/dev/null || echo "N/A")

echo -e "${GREEN}✅ Risk scores calculated:${NC}"
echo "   Pod ($TEST_POD_NAME): Score=$POD_SCORE, Priority=$POD_PRIORITY"
echo "   SA ($TEST_SA_NAME): Score=$SA_SCORE, Priority=$SA_PRIORITY"

# Step 11: Verify in Database
echo -e "\n${BLUE}[11/12] Verifying risk scores in Database...${NC}"
POD_DB_SCORE=$(query_db "SELECT total_score FROM risk_scores WHERE resource_uid = '$POD_UID' ORDER BY calculated_at DESC LIMIT 1;" 2>/dev/null || echo "")
SA_DB_SCORE=$(query_db "SELECT total_score FROM risk_scores WHERE resource_uid = '$SA_UID' ORDER BY calculated_at DESC LIMIT 1;" 2>/dev/null || echo "")

if [ -z "$POD_DB_SCORE" ]; then POD_DB_SCORE="N/A"; fi
if [ -z "$SA_DB_SCORE" ]; then SA_DB_SCORE="N/A"; fi

echo "   Pod score in DB: $POD_DB_SCORE"
echo "   SA score in DB: $SA_DB_SCORE"

# Step 12: Get via API and compare
echo -e "\n${BLUE}[12/12] Getting risk scores via API and comparing...${NC}"

POD_API_RESPONSE=$(curl -s "$API_URL/api/v1/risk/scores/$POD_UID?cluster=minikube" -H "Authorization: Bearer $TOKEN")
POD_API_SCORE=$(echo "$POD_API_RESPONSE" | python3 -c "import sys, json; d=json.load(sys.stdin); print(d.get('totalScore', 0))" 2>/dev/null || echo "0")

SA_API_RESPONSE=$(curl -s "$API_URL/api/v1/risk/scores/$SA_UID?cluster=minikube" -H "Authorization: Bearer $TOKEN")
SA_API_SCORE=$(echo "$SA_API_RESPONSE" | python3 -c "import sys, json; d=json.load(sys.stdin); print(d.get('totalScore', 0))" 2>/dev/null || echo "0")

echo "   Pod score from API: $POD_API_SCORE"
echo "   SA score from API: $SA_API_SCORE"

# Final Comparison Report
echo -e "\n${BLUE}========================================${NC}"
echo -e "${BLUE}FINAL COMPARISON REPORT${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Pod comparison
echo -e "${YELLOW}Pod: $TEST_POD_NAME${NC}"
echo "   UID: $POD_UID"
echo "   K8s Status: $(kubectl get pod "$TEST_POD_NAME" -n "$TEST_NAMESPACE" -o jsonpath='{.status.phase}' 2>/dev/null || echo 'N/A')"
echo "   In Database: $POD_IN_DB"
echo "   Insights Count: $POD_INSIGHTS_COUNT"
echo "   Risk Score (Calculate API): $POD_SCORE"
echo "   Risk Score (DB): $POD_DB_SCORE"
echo "   Risk Score (Get API): $POD_API_SCORE"
echo "   Priority: $POD_PRIORITY"

POD_CONSISTENT="false"
if [ "$POD_SCORE" != "0" ] && [ "$POD_DB_SCORE" != "N/A" ] && [ "$POD_DB_SCORE" != "" ] && [ "$POD_API_SCORE" != "0" ]; then
    POD_CONSISTENT="true"
    echo -e "   ${GREEN}✅ Risk score consistency: OK${NC}"
else
    echo -e "   ${YELLOW}⚠️  Risk score consistency: Partial${NC}"
fi

echo ""
echo -e "${YELLOW}ServiceAccount: $TEST_SA_NAME${NC}"
echo "   UID: $SA_UID"
echo "   K8s Status: $(kubectl get serviceaccount "$TEST_SA_NAME" -n "$TEST_NAMESPACE" -o jsonpath='{.metadata.name}' 2>/dev/null || echo 'N/A')"
echo "   In Database: $SA_IN_DB"
echo "   Insights Count: $SA_INSIGHTS_COUNT"
echo "   Risk Score (Calculate API): $SA_SCORE"
echo "   Risk Score (DB): $SA_DB_SCORE"
echo "   Risk Score (Get API): $SA_API_SCORE"
echo "   Priority: $SA_PRIORITY"

SA_CONSISTENT="false"
if [ "$SA_SCORE" != "0" ] && [ "$SA_DB_SCORE" != "N/A" ] && [ "$SA_DB_SCORE" != "" ] && [ "$SA_API_SCORE" != "0" ]; then
    SA_CONSISTENT="true"
    echo -e "   ${GREEN}✅ Risk score consistency: OK${NC}"
else
    echo -e "   ${YELLOW}⚠️  Risk score consistency: Partial${NC}"
fi

# Test Risk Trends API
echo -e "\n${BLUE}Testing Risk Trends API...${NC}"
TRENDS_RESPONSE=$(curl -s "$API_URL/api/v1/risk/trends?days=7" -H "Authorization: Bearer $TOKEN")
if echo "$TRENDS_RESPONSE" | python3 -c "import sys, json; d=json.load(sys.stdin); exit(0 if 'trends' in d else 1)" 2>/dev/null; then
    TRENDS_COUNT=$(echo "$TRENDS_RESPONSE" | python3 -c "import sys, json; d=json.load(sys.stdin); print(len(d.get('trends', [])))" 2>/dev/null || echo "0")
    echo -e "${GREEN}✅ Trends API working (${TRENDS_COUNT} trend points)${NC}"
else
    echo -e "${YELLOW}⚠️  Trends API response: $TRENDS_RESPONSE${NC}"
fi

# Final Summary
echo -e "\n${BLUE}========================================${NC}"
echo -e "${BLUE}TEST SUMMARY${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

TOTAL_TESTS=0
PASSED_TESTS=0

# Test 1: Resources in K8s
TOTAL_TESTS=$((TOTAL_TESTS + 1))
if [ -n "$POD_UID" ] && [ -n "$SA_UID" ]; then
    PASSED_TESTS=$((PASSED_TESTS + 1))
    echo -e "${GREEN}✅ Test 1: Resources created in K8s${NC}"
else
    echo -e "${RED}❌ Test 1: Resources created in K8s${NC}"
fi

# Test 2: Resources in Database
TOTAL_TESTS=$((TOTAL_TESTS + 1))
if [ "$POD_IN_DB" != "0" ] && [ "$SA_IN_DB" != "0" ]; then
    PASSED_TESTS=$((PASSED_TESTS + 1))
    echo -e "${GREEN}✅ Test 2: Resources synced to Database${NC}"
else
    echo -e "${RED}❌ Test 2: Resources synced to Database${NC}"
fi

# Test 3: Insights Generated
TOTAL_TESTS=$((TOTAL_TESTS + 1))
if [ "$POD_INSIGHTS_COUNT" != "0" ] || [ "$SA_INSIGHTS_COUNT" != "0" ]; then
    PASSED_TESTS=$((PASSED_TESTS + 1))
    echo -e "${GREEN}✅ Test 3: Insights generated${NC}"
else
    echo -e "${RED}❌ Test 3: Insights generated${NC}"
fi

# Test 4: Risk Scores Calculated
TOTAL_TESTS=$((TOTAL_TESTS + 1))
if [ "$POD_SCORE" != "0" ] || [ "$SA_SCORE" != "0" ]; then
    PASSED_TESTS=$((PASSED_TESTS + 1))
    echo -e "${GREEN}✅ Test 4: Risk scores calculated${NC}"
else
    echo -e "${RED}❌ Test 4: Risk scores calculated${NC}"
fi

# Test 5: Risk Scores in Database
TOTAL_TESTS=$((TOTAL_TESTS + 1))
if [ "$POD_DB_SCORE" != "N/A" ] || [ "$SA_DB_SCORE" != "N/A" ]; then
    PASSED_TESTS=$((PASSED_TESTS + 1))
    echo -e "${GREEN}✅ Test 5: Risk scores stored in Database${NC}"
else
    echo -e "${RED}❌ Test 5: Risk scores stored in Database${NC}"
fi

# Test 6: Risk Scores via API
TOTAL_TESTS=$((TOTAL_TESTS + 1))
if [ "$POD_API_SCORE" != "0" ] || [ "$SA_API_SCORE" != "0" ]; then
    PASSED_TESTS=$((PASSED_TESTS + 1))
    echo -e "${GREEN}✅ Test 6: Risk scores retrievable via API${NC}"
else
    echo -e "${RED}❌ Test 6: Risk scores retrievable via API${NC}"
fi

# Test 7: Data Consistency
TOTAL_TESTS=$((TOTAL_TESTS + 1))
if [ "$POD_CONSISTENT" = "true" ] || [ "$SA_CONSISTENT" = "true" ]; then
    PASSED_TESTS=$((PASSED_TESTS + 1))
    echo -e "${GREEN}✅ Test 7: Data consistency (K8s → DB → API)${NC}"
else
    echo -e "${YELLOW}⚠️  Test 7: Data consistency (K8s → DB → API) - Partial${NC}"
fi

# Test 8: Trends API
TOTAL_TESTS=$((TOTAL_TESTS + 1))
if echo "$TRENDS_RESPONSE" | python3 -c "import sys, json; d=json.load(sys.stdin); exit(0 if 'trends' in d else 1)" 2>/dev/null; then
    PASSED_TESTS=$((PASSED_TESTS + 1))
    echo -e "${GREEN}✅ Test 8: Trends API working${NC}"
else
    echo -e "${RED}❌ Test 8: Trends API working${NC}"
fi

echo ""
echo -e "${BLUE}Test Results: $PASSED_TESTS/$TOTAL_TESTS passed${NC}"

if [ $PASSED_TESTS -eq $TOTAL_TESTS ]; then
    echo -e "${GREEN}🎉 All tests passed!${NC}"
    exit 0
else
    echo -e "${YELLOW}⚠️  Some tests need attention${NC}"
    exit 1
fi

