#!/bin/bash

# Script to test migration
# Usage: ./scripts/test_migration.sh [namespace]

set -e

NAMESPACE="${1:-ksam}"

echo "🧪 Testing Migration..."
echo "Namespace: $NAMESPACE"

# Step 1: Clear database
echo ""
echo "Step 1: Clearing database..."
./scripts/clear_database_k8s.sh $NAMESPACE <<EOF
yes
EOF

# Step 2: Restart core pod to trigger migration
echo ""
echo "Step 2: Restarting core pod to trigger migration..."
CORE_POD=$(kubectl get pods -n $NAMESPACE -l app=ksam-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

if [ -z "$CORE_POD" ]; then
    echo "❌ Core pod not found"
    exit 1
fi

echo "Deleting core pod: $CORE_POD"
kubectl delete pod -n $NAMESPACE $CORE_POD

# Step 3: Wait for core pod to be ready
echo ""
echo "Step 3: Waiting for core pod to be ready..."
kubectl wait --for=condition=ready pod -n $NAMESPACE -l app=ksam-core --timeout=120s

# Step 4: Check migration status
echo ""
echo "Step 4: Checking migration status..."
NEW_CORE_POD=$(kubectl get pods -n $NAMESPACE -l app=ksam-core -o jsonpath='{.items[0].metadata.name}')

echo "Checking logs for migration messages..."
kubectl logs -n $NAMESPACE $NEW_CORE_POD | grep -i "migration\|migrate" | tail -10 || echo "No migration logs found"

# Step 5: Verify tables
echo ""
echo "Step 5: Verifying database tables..."
POSTGRES_POD=$(kubectl get pods -n $NAMESPACE -l app=postgres -o jsonpath='{.items[0].metadata.name}')

kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -c "\dt" | head -20

echo ""
echo "✅ Migration test completed!"


