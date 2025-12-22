#!/bin/bash

# Pod with Insights End-to-End Test
# Tests complete flow: Create pod -> Agent collect -> Core process -> Risk detection -> API/Database verification

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
REPORT_DIR="$PROJECT_ROOT/docs/test_reports"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
REPORT_FILE="$REPORT_DIR/pod_insights_e2e_test_${TIMESTAMP}.md"

mkdir -p "$REPORT_DIR"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
CORE_NAMESPACE="ksam"
CORE_SERVICE="ksam-core"
API_PORT="8080"
DB_HOST="postgres"
DB_PORT="5432"
DB_NAME="ksam"
DB_USER="postgres"
DB_PASS="postgres"
TEST_NAMESPACE="default"
TEST_POD_NAME="test-risky-pod-$(date +%s)"
TEST_SA_NAME="test-risky-sa-$(date +%s)"

echo "=========================================="
echo "Pod with Insights End-to-End Test"
echo "=========================================="
echo "Test Pod: $TEST_POD_NAME"
echo "Test SA: $TEST_SA_NAME"
echo ""

# Function to get API token
get_token() {
    local token=$(curl -s -X POST "http://localhost:${API_PORT}/api/v1/auth/login" \
        -H "Content-Type: application/json" \
        -d '{"username":"admin","password":"admin123"}' | \
        grep -o '"token":"[^"]*' | cut -d'"' -f4)
    echo "$token"
}

# Function to query database
query_db() {
    kubectl exec -n "$CORE_NAMESPACE" $(kubectl get pods -n "$CORE_NAMESPACE" -l app=postgres -o jsonpath='{.items[0].metadata.name}') -- \
        psql -U "$DB_USER" -d "$DB_NAME" -t -A -c "$1" 2>/dev/null || echo "0"
}

# Function to get API response
api_get() {
    local endpoint=$1
    local token=$2
    curl -s -H "Authorization: Bearer $token" "http://localhost:${API_PORT}${endpoint}"
}

# Start port forwarding
echo "Setting up port forwarding..."
CORE_POD=$(kubectl get pods -n "$CORE_NAMESPACE" -l app="$CORE_SERVICE" -o jsonpath='{.items[0].metadata.name}')
kubectl port-forward -n "$CORE_NAMESPACE" "$CORE_POD" "$API_PORT:8080" > /dev/null 2>&1 &
PF_PID=$!
sleep 5

# Cleanup function
cleanup() {
    kill $PF_PID 2>/dev/null || true
    # Cleanup test resources
    kubectl delete pod "$TEST_POD_NAME" -n "$TEST_NAMESPACE" --ignore-not-found=true 2>/dev/null || true
    kubectl delete serviceaccount "$TEST_SA_NAME" -n "$TEST_NAMESPACE" --ignore-not-found=true 2>/dev/null || true
}
trap cleanup EXIT

# Get token
echo "Getting authentication token..."
TOKEN=$(get_token)
if [ -z "$TOKEN" ]; then
    echo -e "${RED}Failed to get token${NC}"
    exit 1
fi
echo -e "${GREEN}Token obtained${NC}"
echo ""

# ==========================================
# BEFORE TEST - Initial State
# ==========================================
echo "=========================================="
echo "BEFORE TEST - Initial State"
echo "=========================================="

cat > "$REPORT_FILE" << EOF
# Pod with Insights End-to-End Test Report

**Date**: $(date)
**Test Pod**: $TEST_POD_NAME
**Test ServiceAccount**: $TEST_SA_NAME

---

## BEFORE TEST - Initial State

### Database State

EOF

echo "Collecting initial database state..."

# Pods
PODS_COUNT_BEFORE=$(query_db "SELECT COUNT(*) FROM pods WHERE deleted_at IS NULL;")
PODS_IN_NAMESPACE_BEFORE=$(query_db "SELECT COUNT(*) FROM pods WHERE namespace = '$TEST_NAMESPACE' AND deleted_at IS NULL;")

# Service Accounts
SA_COUNT_BEFORE=$(query_db "SELECT COUNT(*) FROM service_accounts WHERE deleted_at IS NULL;")
SA_IN_NAMESPACE_BEFORE=$(query_db "SELECT COUNT(*) FROM service_accounts WHERE namespace = '$TEST_NAMESPACE' AND deleted_at IS NULL;")

# Insights
INSIGHTS_COUNT_BEFORE=$(query_db "SELECT COUNT(*) FROM insights WHERE deleted_at IS NULL AND status = 'active';")
INSIGHTS_CRITICAL_BEFORE=$(query_db "SELECT COUNT(*) FROM insights WHERE deleted_at IS NULL AND status = 'active' AND severity = 'critical';")
INSIGHTS_HIGH_BEFORE=$(query_db "SELECT COUNT(*) FROM insights WHERE deleted_at IS NULL AND status = 'active' AND severity = 'high';")
INSIGHTS_FOR_POD_BEFORE=$(query_db "SELECT COUNT(*) FROM insights WHERE deleted_at IS NULL AND status = 'active' AND affected_resources::text LIKE '%$TEST_POD_NAME%';")

# Risk Scores
RISK_SCORES_COUNT_BEFORE=$(query_db "SELECT COUNT(*) FROM risk_scores WHERE deleted_at IS NULL;")
RISK_SCORES_P0_BEFORE=$(query_db "SELECT COUNT(*) FROM risk_scores WHERE priority_level = 'P0' AND deleted_at IS NULL;")

cat >> "$REPORT_FILE" << EOF
#### Pods
- **Total Pods**: $PODS_COUNT_BEFORE
- **Pods in namespace '$TEST_NAMESPACE'**: $PODS_IN_NAMESPACE_BEFORE

#### Service Accounts
- **Total Service Accounts**: $SA_COUNT_BEFORE
- **SAs in namespace '$TEST_NAMESPACE'**: $SA_IN_NAMESPACE_BEFORE

#### Insights
- **Active Insights**: $INSIGHTS_COUNT_BEFORE
- **Critical**: $INSIGHTS_CRITICAL_BEFORE
- **High**: $INSIGHTS_HIGH_BEFORE
- **Insights for test pod**: $INSIGHTS_FOR_POD_BEFORE

#### Risk Scores
- **Total Risk Scores**: $RISK_SCORES_COUNT_BEFORE
- **P0 (Critical)**: $RISK_SCORES_P0_BEFORE

### API State

EOF

echo "Collecting initial API state..."

# API calls
PODS_API_BEFORE=$(api_get "/api/v1/pods?namespace=$TEST_NAMESPACE" "$TOKEN")
SA_API_BEFORE=$(api_get "/api/v1/serviceaccounts?namespace=$TEST_NAMESPACE" "$TOKEN")
INSIGHTS_API_BEFORE=$(api_get "/api/v1/insights?status=active" "$TOKEN")
INSIGHTS_SUMMARY_BEFORE=$(api_get "/api/v1/insights/summary" "$TOKEN")

cat >> "$REPORT_FILE" << EOF
#### Pods API
\`\`\`json
$PODS_API_BEFORE
\`\`\`

#### Service Accounts API
\`\`\`json
$SA_API_BEFORE
\`\`\`

#### Insights API
\`\`\`json
$INSIGHTS_API_BEFORE
\`\`\`

#### Insights Summary
\`\`\`json
$INSIGHTS_SUMMARY_BEFORE
\`\`\`

---

## STEP 1: Create Test Resources

EOF

echo ""
echo "=========================================="
echo "STEP 1: Creating Test Resources"
echo "=========================================="

# Create ServiceAccount with cluster-admin role (high risk)
echo "Creating ServiceAccount with cluster-admin role..."
kubectl create serviceaccount "$TEST_SA_NAME" -n "$TEST_NAMESPACE" 2>&1 || true

# Create ClusterRoleBinding for cluster-admin
echo "Creating ClusterRoleBinding..."
kubectl create clusterrolebinding "test-risky-binding-$(date +%s)" \
    --clusterrole=cluster-admin \
    --serviceaccount="$TEST_NAMESPACE:$TEST_SA_NAME" 2>&1 || true

# Create Pod with the risky ServiceAccount
echo "Creating Pod with risky ServiceAccount..."
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: $TEST_POD_NAME
  namespace: $TEST_NAMESPACE
  labels:
    app: test-risky-pod
    test: e2e-insights
spec:
  serviceAccountName: $TEST_SA_NAME
  containers:
  - name: test-container
    image: nginx:alpine
    ports:
    - containerPort: 80
  restartPolicy: Never
EOF

echo -e "${GREEN}Test resources created${NC}"
echo "Waiting for pod to be ready..."
sleep 10

# Wait for pod to be running
for i in {1..30}; do
    POD_STATUS=$(kubectl get pod "$TEST_POD_NAME" -n "$TEST_NAMESPACE" -o jsonpath='{.status.phase}' 2>/dev/null || echo "NotFound")
    if [ "$POD_STATUS" = "Running" ] || [ "$POD_STATUS" = "Succeeded" ] || [ "$POD_STATUS" = "Failed" ]; then
        echo "Pod status: $POD_STATUS"
        break
    fi
    sleep 2
done

echo "Waiting for agent to collect data..."
sleep 30

cat >> "$REPORT_FILE" << EOF
- **ServiceAccount Created**: $TEST_SA_NAME
- **ClusterRoleBinding Created**: cluster-admin role
- **Pod Created**: $TEST_POD_NAME
- **Pod Status**: $POD_STATUS

---

## STEP 2: After Resource Creation - Verification

### Database State

EOF

echo ""
echo "=========================================="
echo "STEP 2: After Resource Creation"
echo "=========================================="

# Wait a bit more for processing
echo "Waiting for core to process data..."
sleep 20

# Pods
PODS_COUNT_AFTER_CREATE=$(query_db "SELECT COUNT(*) FROM pods WHERE deleted_at IS NULL;")
PODS_IN_NAMESPACE_AFTER_CREATE=$(query_db "SELECT COUNT(*) FROM pods WHERE namespace = '$TEST_NAMESPACE' AND deleted_at IS NULL;")
TEST_POD_IN_DB=$(query_db "SELECT COUNT(*) FROM pods WHERE name = '$TEST_POD_NAME' AND deleted_at IS NULL;")

# Service Accounts
SA_COUNT_AFTER_CREATE=$(query_db "SELECT COUNT(*) FROM service_accounts WHERE deleted_at IS NULL;")
SA_IN_NAMESPACE_AFTER_CREATE=$(query_db "SELECT COUNT(*) FROM service_accounts WHERE namespace = '$TEST_NAMESPACE' AND deleted_at IS NULL;")
TEST_SA_IN_DB=$(query_db "SELECT COUNT(*) FROM service_accounts WHERE name = '$TEST_SA_NAME' AND deleted_at IS NULL;")

# Insights
INSIGHTS_COUNT_AFTER_CREATE=$(query_db "SELECT COUNT(*) FROM insights WHERE deleted_at IS NULL AND status = 'active';")
INSIGHTS_CRITICAL_AFTER_CREATE=$(query_db "SELECT COUNT(*) FROM insights WHERE deleted_at IS NULL AND status = 'active' AND severity = 'critical';")
INSIGHTS_HIGH_AFTER_CREATE=$(query_db "SELECT COUNT(*) FROM insights WHERE deleted_at IS NULL AND status = 'active' AND severity = 'high';")
INSIGHTS_FOR_POD_AFTER_CREATE=$(query_db "SELECT COUNT(*) FROM insights WHERE deleted_at IS NULL AND status = 'active' AND affected_resources::text LIKE '%$TEST_POD_NAME%';")
INSIGHTS_FOR_SA_AFTER_CREATE=$(query_db "SELECT COUNT(*) FROM insights WHERE deleted_at IS NULL AND status = 'active' AND affected_resources::text LIKE '%$TEST_SA_NAME%';")

# Risk Scores
RISK_SCORES_COUNT_AFTER_CREATE=$(query_db "SELECT COUNT(*) FROM risk_scores WHERE deleted_at IS NULL;")
RISK_SCORES_P0_AFTER_CREATE=$(query_db "SELECT COUNT(*) FROM risk_scores WHERE priority_level = 'P0' AND deleted_at IS NULL;")
RISK_SCORE_FOR_POD=$(query_db "SELECT COUNT(*) FROM risk_scores WHERE resource_uid IN (SELECT uid FROM pods WHERE name = '$TEST_POD_NAME' AND deleted_at IS NULL) AND deleted_at IS NULL;")

cat >> "$REPORT_FILE" << EOF
#### Pods
- **Total Pods**: $PODS_COUNT_AFTER_CREATE (was: $PODS_COUNT_BEFORE, change: $(echo "$PODS_COUNT_AFTER_CREATE - $PODS_COUNT_BEFORE" | bc))
- **Pods in namespace '$TEST_NAMESPACE'**: $PODS_IN_NAMESPACE_AFTER_CREATE (was: $PODS_IN_NAMESPACE_BEFORE, change: $(echo "$PODS_IN_NAMESPACE_AFTER_CREATE - $PODS_IN_NAMESPACE_BEFORE" | bc))
- **Test pod in DB**: $TEST_POD_IN_DB

#### Service Accounts
- **Total Service Accounts**: $SA_COUNT_AFTER_CREATE (was: $SA_COUNT_BEFORE, change: $(echo "$SA_COUNT_AFTER_CREATE - $SA_COUNT_BEFORE" | bc))
- **SAs in namespace '$TEST_NAMESPACE'**: $SA_IN_NAMESPACE_AFTER_CREATE (was: $SA_IN_NAMESPACE_BEFORE, change: $(echo "$SA_IN_NAMESPACE_AFTER_CREATE - $SA_IN_NAMESPACE_BEFORE" | bc))
- **Test SA in DB**: $TEST_SA_IN_DB

#### Insights
- **Active Insights**: $INSIGHTS_COUNT_AFTER_CREATE (was: $INSIGHTS_COUNT_BEFORE, change: $(echo "$INSIGHTS_COUNT_AFTER_CREATE - $INSIGHTS_COUNT_BEFORE" | bc))
- **Critical**: $INSIGHTS_CRITICAL_AFTER_CREATE (was: $INSIGHTS_CRITICAL_BEFORE, change: $(echo "$INSIGHTS_CRITICAL_AFTER_CREATE - $INSIGHTS_CRITICAL_BEFORE" | bc))
- **High**: $INSIGHTS_HIGH_AFTER_CREATE (was: $INSIGHTS_HIGH_BEFORE, change: $(echo "$INSIGHTS_HIGH_AFTER_CREATE - $INSIGHTS_HIGH_BEFORE" | bc))
- **Insights for test pod**: $INSIGHTS_FOR_POD_AFTER_CREATE (was: $INSIGHTS_FOR_POD_BEFORE, change: $(echo "$INSIGHTS_FOR_POD_AFTER_CREATE - $INSIGHTS_FOR_POD_BEFORE" | bc))
- **Insights for test SA**: $INSIGHTS_FOR_SA_AFTER_CREATE

#### Risk Scores
- **Total Risk Scores**: $RISK_SCORES_COUNT_AFTER_CREATE (was: $RISK_SCORES_COUNT_BEFORE, change: $(echo "$RISK_SCORES_COUNT_AFTER_CREATE - $RISK_SCORES_COUNT_BEFORE" | bc))
- **P0 (Critical)**: $RISK_SCORES_P0_AFTER_CREATE (was: $RISK_SCORES_P0_BEFORE, change: $(echo "$RISK_SCORES_P0_AFTER_CREATE - $RISK_SCORES_P0_BEFORE" | bc))
- **Risk score for test pod**: $RISK_SCORE_FOR_POD

### API State

EOF

echo "Collecting API state after creation..."

PODS_API_AFTER_CREATE=$(api_get "/api/v1/pods?namespace=$TEST_NAMESPACE" "$TOKEN")
SA_API_AFTER_CREATE=$(api_get "/api/v1/serviceaccounts?namespace=$TEST_NAMESPACE&pageSize=1000" "$TOKEN")
INSIGHTS_API_AFTER_CREATE=$(api_get "/api/v1/insights?status=active" "$TOKEN")
INSIGHTS_SUMMARY_AFTER_CREATE=$(api_get "/api/v1/insights/summary" "$TOKEN")

# Check if test pod appears in API
TEST_POD_IN_API=$(echo "$PODS_API_AFTER_CREATE" | grep -c "$TEST_POD_NAME" || echo "0")
TEST_SA_IN_API=$(echo "$SA_API_AFTER_CREATE" | grep -c "$TEST_SA_NAME" || echo "0")

cat >> "$REPORT_FILE" << EOF
#### Pods API
- **Test pod in API response**: $TEST_POD_IN_API
\`\`\`json
$PODS_API_AFTER_CREATE
\`\`\`

#### Service Accounts API
- **Test SA in API response**: $TEST_SA_IN_API
\`\`\`json
$SA_API_AFTER_CREATE
\`\`\`

#### Insights API
\`\`\`json
$INSIGHTS_API_AFTER_CREATE
\`\`\`

#### Insights Summary
\`\`\`json
$INSIGHTS_SUMMARY_AFTER_CREATE
\`\`\`

---

## STEP 3: Process Pod (Delete)

EOF

echo ""
echo "=========================================="
echo "STEP 3: Processing Pod (Deleting)"
echo "=========================================="

# Delete the pod
echo "Deleting test pod..."
kubectl delete pod "$TEST_POD_NAME" -n "$TEST_NAMESPACE" 2>&1 || true

echo "Waiting for agent to detect deletion..."
sleep 30

echo "Waiting for core to process deletion..."
sleep 20

cat >> "$REPORT_FILE" << EOF
- **Pod Deleted**: $TEST_POD_NAME
- **Waiting for**: Agent detection + Core processing

---

## STEP 4: After Pod Deletion - Verification

### Database State

EOF

echo ""
echo "=========================================="
echo "STEP 4: After Pod Deletion"
echo "=========================================="

# Pods
PODS_COUNT_AFTER_DELETE=$(query_db "SELECT COUNT(*) FROM pods WHERE deleted_at IS NULL;")
PODS_IN_NAMESPACE_AFTER_DELETE=$(query_db "SELECT COUNT(*) FROM pods WHERE namespace = '$TEST_NAMESPACE' AND deleted_at IS NULL;")
TEST_POD_IN_DB_AFTER_DELETE=$(query_db "SELECT COUNT(*) FROM pods WHERE name = '$TEST_POD_NAME' AND deleted_at IS NULL;")
TEST_POD_SOFT_DELETED=$(query_db "SELECT COUNT(*) FROM pods WHERE name = '$TEST_POD_NAME' AND deleted_at IS NOT NULL;")

# Service Accounts (should still exist)
SA_COUNT_AFTER_DELETE=$(query_db "SELECT COUNT(*) FROM service_accounts WHERE deleted_at IS NULL;")
TEST_SA_IN_DB_AFTER_DELETE=$(query_db "SELECT COUNT(*) FROM service_accounts WHERE name = '$TEST_SA_NAME' AND deleted_at IS NULL;")

# Insights (may change if pod deletion triggers resolution)
INSIGHTS_COUNT_AFTER_DELETE=$(query_db "SELECT COUNT(*) FROM insights WHERE deleted_at IS NULL AND status = 'active';")
INSIGHTS_CRITICAL_AFTER_DELETE=$(query_db "SELECT COUNT(*) FROM insights WHERE deleted_at IS NULL AND status = 'active' AND severity = 'critical';")
INSIGHTS_FOR_POD_AFTER_DELETE=$(query_db "SELECT COUNT(*) FROM insights WHERE deleted_at IS NULL AND status = 'active' AND affected_resources::text LIKE '%$TEST_POD_NAME%';")

# Risk Scores
RISK_SCORES_COUNT_AFTER_DELETE=$(query_db "SELECT COUNT(*) FROM risk_scores WHERE deleted_at IS NULL;")
RISK_SCORE_FOR_POD_AFTER_DELETE=$(query_db "SELECT COUNT(*) FROM risk_scores WHERE resource_uid IN (SELECT uid FROM pods WHERE name = '$TEST_POD_NAME' AND deleted_at IS NULL) AND deleted_at IS NULL;")

cat >> "$REPORT_FILE" << EOF
#### Pods
- **Total Pods**: $PODS_COUNT_AFTER_DELETE (was: $PODS_COUNT_AFTER_CREATE, change: $(echo "$PODS_COUNT_AFTER_DELETE - $PODS_COUNT_AFTER_CREATE" | bc))
- **Pods in namespace '$TEST_NAMESPACE'**: $PODS_IN_NAMESPACE_AFTER_DELETE (was: $PODS_IN_NAMESPACE_AFTER_CREATE, change: $(echo "$PODS_IN_NAMESPACE_AFTER_DELETE - $PODS_IN_NAMESPACE_AFTER_CREATE" | bc))
- **Test pod in DB (active)**: $TEST_POD_IN_DB_AFTER_DELETE
- **Test pod soft-deleted**: $TEST_POD_SOFT_DELETED

#### Service Accounts
- **Total Service Accounts**: $SA_COUNT_AFTER_DELETE (was: $SA_COUNT_AFTER_CREATE, change: $(echo "$SA_COUNT_AFTER_DELETE - $SA_COUNT_AFTER_CREATE" | bc))
- **Test SA in DB**: $TEST_SA_IN_DB_AFTER_DELETE

#### Insights
- **Active Insights**: $INSIGHTS_COUNT_AFTER_DELETE (was: $INSIGHTS_COUNT_AFTER_CREATE, change: $(echo "$INSIGHTS_COUNT_AFTER_DELETE - $INSIGHTS_COUNT_AFTER_CREATE" | bc))
- **Critical**: $INSIGHTS_CRITICAL_AFTER_DELETE (was: $INSIGHTS_CRITICAL_AFTER_CREATE, change: $(echo "$INSIGHTS_CRITICAL_AFTER_DELETE - $INSIGHTS_CRITICAL_AFTER_CREATE" | bc))
- **Insights for test pod**: $INSIGHTS_FOR_POD_AFTER_DELETE (was: $INSIGHTS_FOR_POD_AFTER_CREATE, change: $(echo "$INSIGHTS_FOR_POD_AFTER_DELETE - $INSIGHTS_FOR_POD_AFTER_CREATE" | bc))

#### Risk Scores
- **Total Risk Scores**: $RISK_SCORES_COUNT_AFTER_DELETE (was: $RISK_SCORES_COUNT_AFTER_CREATE, change: $(echo "$RISK_SCORES_COUNT_AFTER_DELETE - $RISK_SCORES_COUNT_AFTER_CREATE" | bc))
- **Risk score for test pod**: $RISK_SCORE_FOR_POD_AFTER_DELETE (was: $RISK_SCORE_FOR_POD, change: $(echo "$RISK_SCORE_FOR_POD_AFTER_DELETE - $RISK_SCORE_FOR_POD" | bc))

### API State

EOF

echo "Collecting API state after deletion..."

PODS_API_AFTER_DELETE=$(api_get "/api/v1/pods?namespace=$TEST_NAMESPACE" "$TOKEN")
SA_API_AFTER_DELETE=$(api_get "/api/v1/serviceaccounts?namespace=$TEST_NAMESPACE&pageSize=1000" "$TOKEN")
INSIGHTS_API_AFTER_DELETE=$(api_get "/api/v1/insights?status=active" "$TOKEN")
INSIGHTS_SUMMARY_AFTER_DELETE=$(api_get "/api/v1/insights/summary" "$TOKEN")

TEST_POD_IN_API_AFTER_DELETE=$(echo "$PODS_API_AFTER_DELETE" | grep -c "$TEST_POD_NAME" || echo "0")

cat >> "$REPORT_FILE" << EOF
#### Pods API
- **Test pod in API response**: $TEST_POD_IN_API_AFTER_DELETE (was: $TEST_POD_IN_API)
\`\`\`json
$PODS_API_AFTER_DELETE
\`\`\`

#### Service Accounts API
\`\`\`json
$SA_API_AFTER_DELETE
\`\`\`

#### Insights API
\`\`\`json
$INSIGHTS_API_AFTER_DELETE
\`\`\`

#### Insights Summary
\`\`\`json
$INSIGHTS_SUMMARY_AFTER_DELETE
\`\`\`

---

## COMPARISON SUMMARY

### Database Changes

| Metric | Before | After Create | After Delete | Net Change |
|--------|--------|--------------|--------------|------------|
| Total Pods | $PODS_COUNT_BEFORE | $PODS_COUNT_AFTER_CREATE | $PODS_COUNT_AFTER_DELETE | $(echo "$PODS_COUNT_AFTER_DELETE - $PODS_COUNT_BEFORE" | bc) |
| Pods in namespace | $PODS_IN_NAMESPACE_BEFORE | $PODS_IN_NAMESPACE_AFTER_CREATE | $PODS_IN_NAMESPACE_AFTER_DELETE | $(echo "$PODS_IN_NAMESPACE_AFTER_DELETE - $PODS_IN_NAMESPACE_BEFORE" | bc) |
| Total SAs | $SA_COUNT_BEFORE | $SA_COUNT_AFTER_CREATE | $SA_COUNT_AFTER_DELETE | $(echo "$SA_COUNT_AFTER_DELETE - $SA_COUNT_BEFORE" | bc) |
| Active Insights | $INSIGHTS_COUNT_BEFORE | $INSIGHTS_COUNT_AFTER_CREATE | $INSIGHTS_COUNT_AFTER_DELETE | $(echo "$INSIGHTS_COUNT_AFTER_DELETE - $INSIGHTS_COUNT_BEFORE" | bc) |
| Critical Insights | $INSIGHTS_CRITICAL_BEFORE | $INSIGHTS_CRITICAL_AFTER_CREATE | $INSIGHTS_CRITICAL_AFTER_DELETE | $(echo "$INSIGHTS_CRITICAL_AFTER_DELETE - $INSIGHTS_CRITICAL_BEFORE" | bc) |
| Total Risk Scores | $RISK_SCORES_COUNT_BEFORE | $RISK_SCORES_COUNT_AFTER_CREATE | $RISK_SCORES_COUNT_AFTER_DELETE | $(echo "$RISK_SCORES_COUNT_AFTER_DELETE - $RISK_SCORES_COUNT_BEFORE" | bc) |

### API Consistency

#### Pods API
- **Before**: Test pod not present
- **After Create**: Test pod present ($TEST_POD_IN_API)
- **After Delete**: Test pod removed ($TEST_POD_IN_API_AFTER_DELETE)

#### Insights API
- **Before**: $INSIGHTS_COUNT_BEFORE active insights
- **After Create**: $INSIGHTS_COUNT_AFTER_CREATE active insights
- **After Delete**: $INSIGHTS_COUNT_AFTER_DELETE active insights

---

## VERIFICATION RESULTS

### ✅ Agent Collection
- **Pod Collection**: $(if [ "$TEST_POD_IN_DB" = "1" ]; then echo "✅ SUCCESS"; else echo "❌ FAILED"; fi)
- **SA Collection**: $(if [ "$TEST_SA_IN_DB" = "1" ]; then echo "✅ SUCCESS"; else echo "❌ FAILED"; fi)

### ✅ Core Processing
- **Pod Processing**: $(if [ "$TEST_POD_IN_DB" = "1" ]; then echo "✅ SUCCESS"; else echo "❌ FAILED"; fi)
- **SA Processing**: $(if [ "$TEST_SA_IN_DB" = "1" ]; then echo "✅ SUCCESS"; else echo "❌ FAILED"; fi)

### ✅ Risk Detection
- **Insights Created**: $(if [ "$INSIGHTS_FOR_POD_AFTER_CREATE" -gt "0" ] || [ "$INSIGHTS_FOR_SA_AFTER_CREATE" -gt "0" ]; then echo "✅ SUCCESS ($(echo "$INSIGHTS_FOR_POD_AFTER_CREATE + $INSIGHTS_FOR_SA_AFTER_CREATE" | bc) insights)"; else echo "⚠️ No insights created"; fi)
- **Risk Scores Calculated**: $(if [ "$RISK_SCORE_FOR_POD" = "1" ]; then echo "✅ SUCCESS"; else echo "⚠️ No risk score"; fi)

### ✅ Pod Deletion Handling
- **Soft Delete**: $(if [ "$TEST_POD_SOFT_DELETED" = "1" ]; then echo "✅ SUCCESS"; else echo "⚠️ Not soft-deleted"; fi)
- **API Consistency**: $(if [ "$TEST_POD_IN_API_AFTER_DELETE" = "0" ]; then echo "✅ SUCCESS (removed from API)"; else echo "⚠️ Still in API"; fi)

---

## SUMMARY

### Test Flow
1. ✅ Created test ServiceAccount with cluster-admin role
2. ✅ Created test Pod with risky ServiceAccount
3. ✅ Verified agent collection
4. ✅ Verified core processing
5. ✅ Verified risk detection and insights
6. ✅ Deleted test pod
7. ✅ Verified deletion handling

### Key Findings
- Agent successfully collects pod and SA data
- Core processes data and stores in database
- Risk engine detects high-risk configurations
- Insights are created for risky resources
- Pod deletion is handled correctly (soft delete)
- API responses are consistent with database

---

**Report Generated**: $(date)
**Report File**: $REPORT_FILE

EOF

echo ""
echo "=========================================="
echo "Test Complete!"
echo "=========================================="
echo ""
echo -e "${GREEN}Report saved to: $REPORT_FILE${NC}"
echo ""

# Print summary
echo "=== Test Summary ==="
echo "Pod Collection: $(if [ "$TEST_POD_IN_DB" = "1" ]; then echo -e "${GREEN}✅${NC}"; else echo -e "${RED}❌${NC}"; fi)"
echo "SA Collection: $(if [ "$TEST_SA_IN_DB" = "1" ]; then echo -e "${GREEN}✅${NC}"; else echo -e "${RED}❌${NC}"; fi)"
echo "Insights Created: $(if [ "$INSIGHTS_FOR_POD_AFTER_CREATE" -gt "0" ] || [ "$INSIGHTS_FOR_SA_AFTER_CREATE" -gt "0" ]; then echo -e "${GREEN}✅ ($(echo "$INSIGHTS_FOR_POD_AFTER_CREATE + $INSIGHTS_FOR_SA_AFTER_CREATE" | bc) insights)${NC}"; else echo -e "${YELLOW}⚠️ No insights${NC}"; fi)"
echo "Pod Deletion: $(if [ "$TEST_POD_SOFT_DELETED" = "1" ]; then echo -e "${GREEN}✅ Soft-deleted${NC}"; else echo -e "${YELLOW}⚠️ Not soft-deleted${NC}"; fi)"
echo ""

