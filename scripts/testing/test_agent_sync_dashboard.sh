#!/bin/bash
# Test Agent Sync to Dashboard

set -e

NAMESPACE="agent-sync-test"
API_URL="${API_URL:-http://localhost:8080}"
DASHBOARD_URL="${DASHBOARD_URL:-http://localhost:3000}"

echo "=========================================="
echo "Agent Sync to Dashboard Test"
echo "=========================================="
echo ""
echo "Namespace: $NAMESPACE"
echo "API URL: $API_URL"
echo "Dashboard URL: $DASHBOARD_URL"
echo ""

# Step 1: Clear database
echo "[STEP 1] Clearing database..."
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
bash "$SCRIPT_DIR/clear_database.sh"
echo ""

# Step 2: Create test namespace
echo "[STEP 2] Creating test namespace..."
kubectl create namespace "$NAMESPACE" 2>/dev/null || kubectl get namespace "$NAMESPACE"
echo ""

# Step 3: Create test resources
echo "[STEP 3] Creating test resources..."

# Create ServiceAccount
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ServiceAccount
metadata:
  name: test-sa
  namespace: $NAMESPACE
  labels:
    app: test-app
    test: agent-sync
EOF

# Create Role
cat <<EOF | kubectl apply -f -
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: test-role
  namespace: $NAMESPACE
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list"]
EOF

# Create RoleBinding
cat <<EOF | kubectl apply -f -
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: test-rolebinding
  namespace: $NAMESPACE
subjects:
- kind: ServiceAccount
  name: test-sa
  namespace: $NAMESPACE
roleRef:
  kind: Role
  name: test-role
  apiGroup: rbac.authorization.k8s.io
EOF

# Create Pod
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
  namespace: $NAMESPACE
  labels:
    app: test-app
    test: agent-sync
spec:
  serviceAccountName: test-sa
  containers:
  - name: test-container
    image: nginx:alpine
    ports:
    - containerPort: 80
  restartPolicy: Never
EOF

echo "✅ Test resources created"
echo ""

# Step 4: Wait for agent to sync
echo "[STEP 4] Waiting for agent to sync (30 seconds)..."
sleep 30
echo ""

# Step 5: Check database
echo "[STEP 5] Checking database..."
POSTGRES_POD=$(kubectl get pods -n ksam | grep postgres | head -1 | awk '{print $1}')

echo "ServiceAccounts:"
kubectl exec -n ksam "$POSTGRES_POD" -- psql -U postgres -d ksam -c "SELECT name, namespace, cluster_id FROM service_accounts WHERE namespace='$NAMESPACE';" 2>&1

echo ""
echo "Pods:"
kubectl exec -n ksam "$POSTGRES_POD" -- psql -U postgres -d ksam -c "SELECT name, namespace, cluster_id FROM pods WHERE namespace='$NAMESPACE';" 2>&1

echo ""
echo "Roles:"
kubectl exec -n ksam "$POSTGRES_POD" -- psql -U postgres -d ksam -c "SELECT name, namespace FROM roles WHERE namespace='$NAMESPACE';" 2>&1

echo ""
echo "RoleBindings:"
kubectl exec -n ksam "$POSTGRES_POD" -- psql -U postgres -d ksam -c "SELECT name, namespace FROM role_bindings WHERE namespace='$NAMESPACE';" 2>&1

echo ""

# Step 6: Check API
echo "[STEP 6] Checking API..."

# Get token
TOKEN=$(curl -s -X POST "$API_URL/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    -H "Origin: http://localhost:3000" \
    -d '{"username":"admin","password":"admin123"}' 2>/dev/null | \
    python3 -c "import sys, json; data=json.load(sys.stdin); print(data.get('token', ''))" 2>/dev/null || echo "")

if [ -n "$TOKEN" ]; then
    AUTH_HEADER="Authorization: Bearer $TOKEN"
else
    AUTH_HEADER=""
fi

echo "Clusters:"
curl -s -H "$AUTH_HEADER" -H "Origin: http://localhost:3000" "$API_URL/api/v1/clusters/stats" 2>/dev/null | python3 -m json.tool 2>/dev/null | head -30

echo ""
echo "ServiceAccounts:"
curl -s -H "$AUTH_HEADER" -H "Origin: http://localhost:3000" "$API_URL/api/v1/serviceaccounts?namespace=$NAMESPACE" 2>/dev/null | python3 -m json.tool 2>/dev/null | head -50

echo ""
echo "Insights Summary:"
curl -s -H "$AUTH_HEADER" -H "Origin: http://localhost:3000" "$API_URL/api/v1/insights/summary" 2>/dev/null | python3 -m json.tool 2>/dev/null

echo ""

# Step 7: Verify Dashboard data
echo "[STEP 7] Dashboard Verification"
echo ""
echo "✅ Test Resources Created:"
echo "  - Namespace: $NAMESPACE"
echo "  - ServiceAccount: test-sa"
echo "  - Role: test-role"
echo "  - RoleBinding: test-rolebinding"
echo "  - Pod: test-pod"
echo ""
echo "📊 Check Dashboard:"
echo "  1. Open: $DASHBOARD_URL"
echo "  2. Check Dashboard screen:"
echo "     - Clusters count should be > 0"
echo "     - Pods count should include test-pod"
echo "  3. Check Clusters screen:"
echo "     - Should show cluster with pods"
echo "  4. Check Insights screen:"
echo "     - Should show any insights generated"
echo ""
echo "🔍 Manual Verification:"
echo "  - Verify ServiceAccount appears in Resources"
echo "  - Verify Pod appears in Resources"
echo "  - Verify data matches database"
echo ""

# Step 8: Cleanup (optional)
read -p "Clean up test resources? (y/n) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "Cleaning up..."
    kubectl delete namespace "$NAMESPACE" 2>/dev/null || true
    echo "✅ Cleanup complete"
fi

echo ""
echo "=========================================="
echo "Test Complete"
echo "=========================================="

