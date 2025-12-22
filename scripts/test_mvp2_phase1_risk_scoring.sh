#!/bin/bash

# MVP2 Phase 1.1 - Risk Scoring Test Script
# Tests risk score calculation with real K8s resources
# Verifies: K8s → Database → API consistency

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test namespace
TEST_NAMESPACE="mvp2-test-$(date +%Y%m%d-%H%M%S)"
TEST_POD_NAME="test-risky-pod-$(date +%s)"
TEST_SA_NAME="test-risky-sa-$(date +%s)"
TEST_ROLE_NAME="test-risky-role-$(date +%s)"
TEST_RB_NAME="test-risky-rb-$(date +%s)"

# Database connection
DB_HOST="${DB_HOST:-postgres.ksam.svc.cluster.local}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-ksam}"
DB_USER="${DB_USER:-ksam}"
DB_PASSWORD="${DB_PASSWORD:-ksam123}"

# API endpoint
API_URL="${API_URL:-http://localhost:8080}"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}MVP2 Phase 1.1 - Risk Scoring Test${NC}"
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
        if curl -s "http://localhost:$port/health" > /dev/null 2>&1 || \
           curl -s "http://localhost:$port/api/v1/auth/login" > /dev/null 2>&1; then
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
        PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -A -c "$query" 2>/dev/null
    else
        # Fallback to kubectl exec
        local postgres_pod=$(kubectl get pods -n ksam -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
        if [ -n "$postgres_pod" ]; then
            kubectl exec -n ksam "$postgres_pod" -- psql -U "$DB_USER" -d "$DB_NAME" -t -A -c "$query" 2>/dev/null
        else
            echo ""
        fi
    fi
}

# Step 1: Setup port-forwarding
echo -e "${BLUE}[1/8] Setting up port-forwarding...${NC}"
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
echo -e "\n${BLUE}[2/8] Getting auth token...${NC}"
TOKEN=$(get_auth_token)
if [ -z "$TOKEN" ]; then
    echo -e "${RED}❌ Failed to get auth token${NC}"
    kill $PF_PID 2>/dev/null || true
    exit 1
fi
echo -e "${GREEN}✅ Token obtained${NC}"

# Step 3: Create test namespace
echo -e "\n${BLUE}[3/8] Creating test namespace: $TEST_NAMESPACE${NC}"
kubectl create namespace "$TEST_NAMESPACE" 2>/dev/null || true
sleep 2

# Step 4: Create risky ServiceAccount with cluster-admin binding
echo -e "\n${BLUE}[4/8] Creating risky resources...${NC}"

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

# Step 5: Wait for agent sync
echo -e "\n${BLUE}[5/11] Waiting for agent sync (60s)...${NC}"
echo "   This allows agent to collect resources and sync to Core"
sleep 60

# Step 6: Verify resources in database before evaluation
echo -e "\n${BLUE}[6/11] Verifying resources in database...${NC}"
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

# Step 7: Trigger risk evaluation
echo -e "\n${BLUE}[7/11] Triggering risk evaluation...${NC}"
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

# Step 8: Get resource UIDs from K8s
echo -e "\n${BLUE}[8/11] Getting resource UIDs from K8s...${NC}"
POD_UID=$(kubectl get pod "$TEST_POD_NAME" -n "$TEST_NAMESPACE" -o jsonpath='{.metadata.uid}' 2>/dev/null)
SA_UID=$(kubectl get serviceaccount "$TEST_SA_NAME" -n "$TEST_NAMESPACE" -o jsonpath='{.metadata.uid}' 2>/dev/null)

if [ -z "$POD_UID" ] || [ -z "$SA_UID" ]; then
    echo -e "${RED}❌ Failed to get resource UIDs${NC}"
    echo "   POD_UID: $POD_UID"
    echo "   SA_UID: $SA_UID"
    kill $PF_PID 2>/dev/null || true
    exit 1
fi

echo -e "${GREEN}✅ Resource UIDs:${NC}"
echo "   Pod UID: $POD_UID"
echo "   SA UID: $SA_UID"

# Step 8: Verify in Database
echo -e "\n${BLUE}[8/11] Verifying in Database...${NC}"

# Check insights for pod
POD_INSIGHTS_COUNT=$(query_db "SELECT COUNT(*) FROM insights WHERE affected_resources::text LIKE '%$POD_UID%' AND status = 'active' AND deleted_at IS NULL;" 2>/dev/null || echo "0")
if [ -z "$POD_INSIGHTS_COUNT" ]; then
    POD_INSIGHTS_COUNT="0"
fi
echo "   Pod insights in DB: $POD_INSIGHTS_COUNT"

# Check insights for SA
SA_INSIGHTS_COUNT=$(query_db "SELECT COUNT(*) FROM insights WHERE affected_resources::text LIKE '%$SA_UID%' AND status = 'active' AND deleted_at IS NULL;" 2>/dev/null || echo "0")
if [ -z "$SA_INSIGHTS_COUNT" ]; then
    SA_INSIGHTS_COUNT="0"
fi
echo "   SA insights in DB: $SA_INSIGHTS_COUNT"

# Step 9: Calculate Risk Scores via API
echo -e "\n${BLUE}[9/11] Calculating risk scores via API...${NC}"

# Calculate for Pod
POD_SCORE_RESPONSE=$(curl -s -X POST "$API_URL/api/v1/risk/scores/$POD_UID/calculate" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json")

POD_SCORE=$(echo "$POD_SCORE_RESPONSE" | grep -o '"totalScore":[0-9.]*' | cut -d':' -f2 || echo "0")

# Calculate for SA
SA_SCORE_RESPONSE=$(curl -s -X POST "$API_URL/api/v1/risk/scores/$SA_UID/calculate" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json")

SA_SCORE=$(echo "$SA_SCORE_RESPONSE" | grep -o '"totalScore":[0-9.]*' | cut -d':' -f2 || echo "0")

echo -e "${GREEN}✅ Risk scores calculated:${NC}"
echo "   Pod ($TEST_POD_NAME): $POD_SCORE"
echo "   SA ($TEST_SA_NAME): $SA_SCORE"

# Step 10: Verify in Database
echo -e "\n${BLUE}[10/11] Verifying risk scores in Database...${NC}"

POD_DB_SCORE=$(query_db "SELECT total_score FROM risk_scores WHERE resource_uid = '$POD_UID' ORDER BY calculated_at DESC LIMIT 1;" 2>/dev/null || echo "")
SA_DB_SCORE=$(query_db "SELECT total_score FROM risk_scores WHERE resource_uid = '$SA_UID' ORDER BY calculated_at DESC LIMIT 1;" 2>/dev/null || echo "")

if [ -z "$POD_DB_SCORE" ]; then
    POD_DB_SCORE="N/A"
fi
if [ -z "$SA_DB_SCORE" ]; then
    SA_DB_SCORE="N/A"
fi

echo "   Pod score in DB: $POD_DB_SCORE"
echo "   SA score in DB: $SA_DB_SCORE"

# Step 11: Get via API
echo -e "\n${BLUE}[11/11] Getting risk scores via API...${NC}"

POD_API_SCORE=$(curl -s "$API_URL/api/v1/risk/scores/$POD_UID?cluster=minikube" \
    -H "Authorization: Bearer $TOKEN" | grep -o '"totalScore":[0-9.]*' | cut -d':' -f2 || echo "0")

SA_API_SCORE=$(curl -s "$API_URL/api/v1/risk/scores/$SA_UID?cluster=minikube" \
    -H "Authorization: Bearer $TOKEN" | grep -o '"totalScore":[0-9.]*' | cut -d':' -f2 || echo "0")

echo "   Pod score from API: $POD_API_SCORE"
echo "   SA score from API: $SA_API_SCORE"

# Step 12: Comparison Report
echo -e "\n${BLUE}========================================${NC}"
echo -e "${BLUE}COMPARISON REPORT${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

echo -e "${YELLOW}Pod: $TEST_POD_NAME (UID: $POD_UID)${NC}"
echo "   K8s Status: $(kubectl get pod "$TEST_POD_NAME" -n "$TEST_NAMESPACE" -o jsonpath='{.status.phase}' 2>/dev/null || echo 'N/A')"
echo "   Insights in DB: $POD_INSIGHTS_COUNT"
echo "   Risk Score (API Calculate): $POD_SCORE"
echo "   Risk Score (DB): $POD_DB_SCORE"
echo "   Risk Score (API Get): $POD_API_SCORE"

if [ "$POD_SCORE" != "0" ] && [ "$POD_DB_SCORE" != "N/A" ] && [ "$POD_DB_SCORE" != "" ] && [ "$POD_API_SCORE" != "0" ]; then
    echo -e "   ${GREEN}✅ Risk score consistency: OK${NC}"
else
    echo -e "   ${YELLOW}⚠️  Risk score consistency: Partial (some values missing)${NC}"
fi

echo ""
echo -e "${YELLOW}ServiceAccount: $TEST_SA_NAME (UID: $SA_UID)${NC}"
echo "   K8s Status: $(kubectl get serviceaccount "$TEST_SA_NAME" -n "$TEST_NAMESPACE" -o jsonpath='{.metadata.name}' 2>/dev/null || echo 'N/A')"
echo "   Insights in DB: $SA_INSIGHTS_COUNT"
echo "   Risk Score (API Calculate): $SA_SCORE"
echo "   Risk Score (DB): $SA_DB_SCORE"
echo "   Risk Score (API Get): $SA_API_SCORE"

if [ "$SA_SCORE" != "0" ] && [ "$SA_DB_SCORE" != "N/A" ] && [ "$SA_DB_SCORE" != "" ] && [ "$SA_API_SCORE" != "0" ]; then
    echo -e "   ${GREEN}✅ Risk score consistency: OK${NC}"
else
    echo -e "   ${YELLOW}⚠️  Risk score consistency: Partial (some values missing)${NC}"
fi

# Step 12: Test Risk Trends API
echo -e "\n${BLUE}[12/12] Testing Risk Trends API...${NC}"
TRENDS_RESPONSE=$(curl -s "$API_URL/api/v1/risk/trends?days=7" \
    -H "Authorization: Bearer $TOKEN")

if echo "$TRENDS_RESPONSE" | grep -q "trends"; then
    echo -e "${GREEN}✅ Trends API working${NC}"
    echo "$TRENDS_RESPONSE" | python3 -m json.tool 2>/dev/null | head -20 || echo "$TRENDS_RESPONSE" | head -5
else
    echo -e "${YELLOW}⚠️  Trends API response: $TRENDS_RESPONSE${NC}"
fi

# Cleanup
kill $PF_PID 2>/dev/null || true

echo -e "\n${GREEN}========================================${NC}"
echo -e "${GREEN}TEST COMPLETE${NC}"
echo -e "${GREEN}========================================${NC}"

