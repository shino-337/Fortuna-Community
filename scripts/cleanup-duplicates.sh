#!/bin/bash

# Script to cleanup duplicate ServiceAccounts with test-uid-* UIDs
# These are old test entries that should be removed

set -e

echo "=========================================="
echo "Cleaning up duplicate ServiceAccounts"
echo "=========================================="
echo ""

# Get Postgres pod
POSTGRES_POD=$(kubectl get pods -n ksam -l app=postgres -o jsonpath='{.items[0].metadata.name}')

if [ -z "$POSTGRES_POD" ]; then
  echo "ERROR: Postgres pod not found"
  exit 1
fi

echo "Postgres pod: $POSTGRES_POD"
echo ""

# Connect to database and delete duplicates with test-uid-* UIDs
echo "Deleting ServiceAccounts with test-uid-* UIDs..."
kubectl exec -n ksam "$POSTGRES_POD" -- psql -U postgres -d ksam -c "
DELETE FROM service_accounts 
WHERE uid LIKE 'test-uid-%'
AND cluster_id = 'minikube';
" 2>/dev/null

echo "✓ Cleanup complete"
echo ""

# Verify
echo "Remaining ServiceAccounts in default namespace:"
kubectl exec -n ksam "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "
SELECT name, namespace, uid 
FROM service_accounts 
WHERE cluster_id = 'minikube' 
AND namespace = 'default'
ORDER BY name;
" 2>/dev/null | head -10

echo ""
echo "Remaining ServiceAccounts in test-ksam namespace:"
kubectl exec -n ksam "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "
SELECT name, namespace, uid 
FROM service_accounts 
WHERE cluster_id = 'minikube' 
AND namespace = 'test-ksam'
ORDER BY name;
" 2>/dev/null | head -10

echo ""
echo "=========================================="
echo "Cleanup complete!"
echo "=========================================="

