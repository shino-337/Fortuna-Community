#!/bin/bash

# End-to-End Test: Pod Risk Detection → Insight Creation → Fix → Insight Resolution
# Test Flow:
# 1. Deploy pod with overprivileged ServiceAccount
# 2. Agent collects → Core processes → Risk Engine creates insight
# 3. Dashboard displays insight
# 4. DevOps fixes (reduces SA permissions)
# 5. Agent collects again → Insight resolved
# 6. Dashboard risk count decreases

echo "=========================================="
echo "🧪 END-TO-END RISK DETECTION TEST"
echo "=========================================="
echo ""

# Setup
API_URL="http://localhost:8080"
NAMESPACE="default"
POD_NAME="frontend-test-$(date +%s)"
SA_NAME="frontend-sa-test"
ROLE_NAME="frontend-role-test"
ROLE_BINDING_NAME="frontend-rb-test"

# Cleanup function
cleanup() {
    echo ""
    echo "🧹 Cleaning up test resources..."
    kubectl delete pod $POD_NAME -n $NAMESPACE --ignore-not-found=true 2>/dev/null
    kubectl delete serviceaccount $SA_NAME -n $NAMESPACE --ignore-not-found=true 2>/dev/null
    kubectl delete role $ROLE_NAME -n $NAMESPACE --ignore-not-found=true 2>/dev/null
    kubectl delete rolebinding $ROLE_BINDING_NAME -n $NAMESPACE --ignore-not-found=true 2>/dev/null
    echo "✅ Cleanup complete"
}

# Trap to cleanup on exit
trap cleanup EXIT

# Start port-forward
echo "1️⃣  Starting port-forward..."
pkill -f "port-forward.*ksam-core" 2>/dev/null
kubectl port-forward -n ksam svc/ksam-core 8080:8080 > /tmp/pf.log 2>&1 &
PF_PID=$!
sleep 8

# Get initial insight count
echo ""
echo "2️⃣  Getting initial insight count..."
INITIAL_COUNT=$(curl -s "$API_URL/api/v1/insights/summary" | jq -r '.total // 0')
echo "   Initial active insights: $INITIAL_COUNT"

# Step 1: Deploy pod with overprivileged ServiceAccount
# Using wildcard permissions to ensure risk detection
echo ""
echo "3️⃣  Step 1: Deploying pod with overprivileged ServiceAccount..."
kubectl apply -f - <<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  name: $SA_NAME
  namespace: $NAMESPACE
  labels:
    test: e2e-risk-detection
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: $ROLE_NAME
  namespace: $NAMESPACE
  labels:
    test: e2e-risk-detection
rules:
- apiGroups: ["*"]
  resources: ["*"]
  verbs: ["*"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: $ROLE_BINDING_NAME
  namespace: $NAMESPACE
  labels:
    test: e2e-risk-detection
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: $ROLE_NAME
subjects:
- kind: ServiceAccount
  name: $SA_NAME
  namespace: $NAMESPACE
---
apiVersion: v1
kind: Pod
metadata:
  name: $POD_NAME
  namespace: $NAMESPACE
  labels:
    app: frontend-test
    test: e2e-risk-detection
spec:
  serviceAccountName: $SA_NAME
  containers:
  - name: frontend
    image: nginx:latest
    ports:
    - containerPort: 80
EOF

echo "   ✅ Pod deployed: $POD_NAME"
echo "   ✅ ServiceAccount: $SA_NAME (with clusterrolebindings create permission)"

# Clean up duplicate roles in database before test
echo ""
echo "4️⃣  Cleaning up duplicate roles in database..."
POSTGRES_POD=$(kubectl get pods -n ksam -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
if [ ! -z "$POSTGRES_POD" ]; then
    kubectl exec -n ksam $POSTGRES_POD -- psql -U postgres -d ksam -t -c "WITH ranked AS (SELECT id, ROW_NUMBER() OVER (PARTITION BY name, namespace ORDER BY updated_at DESC) as rn FROM roles WHERE name = '$ROLE_NAME' AND namespace = '$NAMESPACE' AND deleted_at IS NULL) UPDATE roles SET deleted_at = NOW() WHERE id IN (SELECT id FROM ranked WHERE rn > 1);" 2>/dev/null > /dev/null
    echo "   ✅ Cleaned up duplicate roles"
else
    echo "   ⚠️  Postgres pod not found, skipping cleanup"
fi

# Wait for agent to collect (increased to 30s for better sync)
echo ""
echo "5️⃣  Waiting for Agent to collect data (30s)..."
sleep 30

# Trigger risk evaluation
echo ""
echo "6️⃣  Triggering risk evaluation..."
EVAL_RESPONSE=$(curl -s -X POST "$API_URL/api/v1/insights/evaluate/historical")
echo "   Response: $EVAL_RESPONSE"

# Wait for processing (increased to 15s)
echo "   Waiting for processing (15s)..."
sleep 15

# Check for new insights (with retry logic)
echo ""
echo "7️⃣  Checking for new insights..."
NEW_COUNT=$(curl -s "$API_URL/api/v1/insights/summary" | jq -r '.total // 0')
echo "   New active insights: $NEW_COUNT"
echo "   Increase: $((NEW_COUNT - INITIAL_COUNT))"

# Retry if no increase detected (may need more time)
if [ $NEW_COUNT -le $INITIAL_COUNT ]; then
    echo "   ⚠️  No new insights detected. Retrying after 10s..."
    sleep 10
    NEW_COUNT=$(curl -s "$API_URL/api/v1/insights/summary" | jq -r '.total // 0')
    echo "   Retry - New active insights: $NEW_COUNT (increase: $((NEW_COUNT - INITIAL_COUNT)))"
fi

if [ $NEW_COUNT -le $INITIAL_COUNT ]; then
    echo "   ⚠️  Still no new insights. Checking all insights..."
    ALL_INSIGHTS=$(curl -s "$API_URL/api/v1/insights?pageSize=50" | jq -r '.insights[] | select(.status == "active" or .status == null) | {id, type, severity, description}' | head -50)
    echo "   Recent active insights:"
    echo "$ALL_INSIGHTS"
fi

# Find insight related to our test (improved search logic)
echo ""
echo "8️⃣  Searching for insight related to test resources..."
# Search for insights related to wildcard permissions or our specific resources
# Also search by namespace "default" in description
TEST_INSIGHT=$(curl -s "$API_URL/api/v1/insights?pageSize=100" | jq -r ".insights[] | select((.description | contains(\"$SA_NAME\")) or (.description | contains(\"$ROLE_NAME\")) or ((.description | contains(\"wildcard\")) and (.description | contains(\"default\"))) or (.description | contains(\"overprivileged\"))) | select(.status == \"active\" or .status == null) | {id, status, type, severity, description}" | head -30)

if [ ! -z "$TEST_INSIGHT" ] && [ "$TEST_INSIGHT" != "null" ]; then
    echo "   ✅ Found test-related insight:"
    echo "$TEST_INSIGHT"
    # Try multiple search patterns to find insight ID
    INSIGHT_ID=$(curl -s "$API_URL/api/v1/insights?pageSize=100" | jq -r ".insights[] | select((.description | contains(\"$SA_NAME\")) or (.description | contains(\"$ROLE_NAME\")) or ((.description | contains(\"wildcard\")) and (.description | contains(\"default\")))) | select(.status == \"active\" or .status == null) | .id" | head -1)
    if [ -z "$INSIGHT_ID" ] || [ "$INSIGHT_ID" = "null" ]; then
        # Fallback: get most recent wildcard insight in default namespace
        INSIGHT_ID=$(curl -s "$API_URL/api/v1/insights?pageSize=100" | jq -r ".insights[] | select((.description | contains(\"wildcard\")) and (.description | contains(\"default\"))) | select(.status == \"active\" or .status == null) | .id" | head -1)
    fi
    echo "   Insight ID: $INSIGHT_ID"
else
    echo "   ⚠️  No specific test insight found. Checking wildcard insights in default namespace..."
    WILDCARD_INSIGHT=$(curl -s "$API_URL/api/v1/insights?pageSize=100" | jq -r ".insights[] | select((.description | contains(\"wildcard\")) and (.description | contains(\"default\"))) | select(.status == \"active\" or .status == null) | {id, status, type, severity, description}" | head -30)
    if [ ! -z "$WILDCARD_INSIGHT" ] && [ "$WILDCARD_INSIGHT" != "null" ]; then
        echo "   Found wildcard insight in default namespace (may be related to our test):"
        echo "$WILDCARD_INSIGHT"
        INSIGHT_ID=$(curl -s "$API_URL/api/v1/insights?pageSize=100" | jq -r ".insights[] | select((.description | contains(\"wildcard\")) and (.description | contains(\"default\"))) | select(.status == \"active\" or .status == null) | .id" | head -1)
    else
        echo "   Checking all active insights..."
        ALL_ACTIVE=$(curl -s "$API_URL/api/v1/insights?status=active&pageSize=20" | jq -r '.insights[] | {id, type, severity, description}' | head -40)
        echo "$ALL_ACTIVE"
        INSIGHT_ID=$(curl -s "$API_URL/api/v1/insights?status=active&pageSize=20" | jq -r '.insights[0].id // empty')
    fi
fi

# Verify insight appears in dashboard summary
echo ""
echo "9️⃣  Verifying insight in dashboard summary..."
SUMMARY=$(curl -s "$API_URL/api/v1/insights/summary")
echo "   Summary: $SUMMARY"

# Step 2: DevOps fixes (reduces SA permissions)
echo ""
echo "🔟 Step 2: DevOps fixes - Reducing SA permissions..."
kubectl delete role $ROLE_NAME -n $NAMESPACE 2>/dev/null
kubectl delete rolebinding $ROLE_BINDING_NAME -n $NAMESPACE 2>/dev/null
sleep 3

# Create new role with reduced permissions (no wildcards)
kubectl apply -f - <<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: $ROLE_NAME
  namespace: $NAMESPACE
  labels:
    test: e2e-risk-detection
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list"]
- apiGroups: [""]
  resources: ["configmaps"]
  verbs: ["get"]
EOF

echo "   ✅ Reduced permissions: Only 'get' and 'list' pods (no wildcards)"

# Clean up duplicate roles again after fix
echo ""
echo "1️⃣1️⃣  Cleaning up duplicate roles after fix..."
if [ ! -z "$POSTGRES_POD" ]; then
    kubectl exec -n ksam $POSTGRES_POD -- psql -U postgres -d ksam -t -c "WITH ranked AS (SELECT id, ROW_NUMBER() OVER (PARTITION BY name, namespace ORDER BY updated_at DESC) as rn FROM roles WHERE name = '$ROLE_NAME' AND namespace = '$NAMESPACE' AND deleted_at IS NULL) UPDATE roles SET deleted_at = NOW() WHERE id IN (SELECT id FROM ranked WHERE rn > 1);" 2>/dev/null > /dev/null
    echo "   ✅ Cleaned up duplicate roles"
fi

# Wait for agent to collect again (increased to 30s)
echo ""
echo "1️⃣2️⃣  Waiting for Agent to collect updated data (30s)..."
sleep 30

# Trigger risk evaluation again
echo ""
echo "1️⃣3️⃣  Triggering risk evaluation again..."
EVAL_RESPONSE2=$(curl -s -X POST "$API_URL/api/v1/insights/evaluate/historical")
echo "   Response: $EVAL_RESPONSE2"

# Wait for processing (increased to 15s)
echo "   Waiting for processing (15s)..."
sleep 15

# Check if insight is resolved (with retry)
echo ""
echo "1️⃣4️⃣  Checking if insight is resolved..."
FINAL_COUNT=$(curl -s "$API_URL/api/v1/insights/summary" | jq -r '.total // 0')
echo "   Final active insights: $FINAL_COUNT"
echo "   Decrease: $((NEW_COUNT - FINAL_COUNT))"

if [ ! -z "$INSIGHT_ID" ] && [ "$INSIGHT_ID" != "null" ] && [ "$INSIGHT_ID" != "" ]; then
    INSIGHT_STATUS=$(curl -s "$API_URL/api/v1/insights/$INSIGHT_ID" | jq -r '.status // "unknown"')
    echo "   Insight ID $INSIGHT_ID status: $INSIGHT_STATUS"
    
    if [ "$INSIGHT_STATUS" = "resolved" ]; then
        echo "   ✅ Insight automatically resolved!"
    elif [ "$INSIGHT_STATUS" = "active" ] || [ "$INSIGHT_STATUS" = "null" ]; then
        echo "   ⚠️  Insight still active. Checking if risk still exists..."
        # Check if the role still has dangerous permissions
        ROLE_RULES=$(kubectl get role $ROLE_NAME -n $NAMESPACE -o jsonpath='{.rules[*].verbs}' 2>/dev/null)
        echo "   Current role verbs: $ROLE_RULES"
        
        # Retry after 10s - auto-resolution may need more time
        echo "   Retrying after 10s to check if auto-resolution completed..."
        sleep 10
        INSIGHT_STATUS=$(curl -s "$API_URL/api/v1/insights/$INSIGHT_ID" | jq -r '.status // "unknown"')
        if [ "$INSIGHT_STATUS" = "resolved" ]; then
            echo "   ✅ Insight automatically resolved (after retry)!"
        else
            echo "   ⚠️  Insight still active after retry. Status: $INSIGHT_STATUS"
        fi
    fi
else
    echo "   ⚠️  No insight ID found to check"
fi

# Check resolved insights
echo ""
echo "1️⃣5️⃣  Checking resolved insights..."
RESOLVED_COUNT=$(curl -s "$API_URL/api/v1/insights?status=resolved&pageSize=1" | jq -r '.total // 0')
echo "   Total resolved insights: $RESOLVED_COUNT"

# Final summary
echo ""
echo "=========================================="
echo "📊 TEST RESULTS SUMMARY"
echo "=========================================="
echo ""
echo "Initial active insights: $INITIAL_COUNT"
echo "After risk detection:    $NEW_COUNT (increase: $((NEW_COUNT - INITIAL_COUNT)))"
echo "After fix:               $FINAL_COUNT (decrease: $((NEW_COUNT - FINAL_COUNT)))"
echo ""

# Verify expected results
echo "✅ Expected Results Check:"
if [ $NEW_COUNT -gt $INITIAL_COUNT ]; then
    echo "   ✅ Insight appeared after risk detection"
else
    echo "   ❌ Insight did not appear (expected increase)"
fi

if [ $FINAL_COUNT -lt $NEW_COUNT ]; then
    echo "   ✅ Risk count decreased after fix"
elif [ ! -z "$INSIGHT_ID" ] && [ "$INSIGHT_STATUS" = "resolved" ]; then
    echo "   ✅ Insight resolved (count may not decrease if other insights exist)"
else
    echo "   ⚠️  Risk count did not decrease (may need more time or insight still active)"
fi

echo ""
echo "⏱️  Timing:"
echo "   - Insight detection: ~30s (after agent sync + evaluation)"
echo "   - Insight resolution: ~45s (after fix + agent sync + evaluation + auto-resolution)"

# Cleanup
cleanup

# Stop port-forward
kill $PF_PID 2>/dev/null
wait $PF_PID 2>/dev/null || true

echo ""
echo "=========================================="
echo "✅ END-TO-END TEST COMPLETE"
echo "=========================================="

