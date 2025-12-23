#!/bin/bash

# Script to clear/reset the database in Kubernetes
# Usage: ./scripts/clear_database_k8s.sh [namespace]

set -e

NAMESPACE="${1:-ksam}"
POSTGRES_POD=$(kubectl get pods -n $NAMESPACE -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

if [ -z "$POSTGRES_POD" ]; then
    echo "❌ PostgreSQL pod not found in namespace $NAMESPACE"
    exit 1
fi

echo "⚠️  WARNING: This will DROP ALL TABLES in the database!"
echo "PostgreSQL pod: $POSTGRES_POD"
echo "Namespace: $NAMESPACE"
read -p "Are you sure you want to continue? (yes/no): " confirm

if [ "$confirm" != "yes" ]; then
    echo "Aborted."
    exit 1
fi

echo "Clearing database..."

# Drop all tables via kubectl exec
kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam <<EOF
-- Drop all tables in public schema
DO \$\$ 
DECLARE 
    r RECORD;
BEGIN
    FOR r IN (SELECT tablename FROM pg_tables WHERE schemaname = 'public') 
    LOOP
        EXECUTE 'DROP TABLE IF EXISTS ' || quote_ident(r.tablename) || ' CASCADE';
    END LOOP;
END \$\$;

-- Drop all sequences
DO \$\$ 
DECLARE 
    r RECORD;
BEGIN
    FOR r IN (SELECT sequence_name FROM information_schema.sequences WHERE sequence_schema = 'public') 
    LOOP
        EXECUTE 'DROP SEQUENCE IF EXISTS ' || quote_ident(r.sequence_name) || ' CASCADE';
    END LOOP;
END \$\$;
EOF

echo "✅ Database cleared successfully!"
echo "Restart core pod to run migrations: kubectl delete pod -n $NAMESPACE -l app=ksam-core"


