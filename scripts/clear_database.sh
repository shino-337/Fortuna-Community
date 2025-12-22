#!/bin/bash
# Clear Database for Fresh Test

set -e

POSTGRES_POD=$(kubectl get pods -n ksam | grep postgres | head -1 | awk '{print $1}')

if [ -z "$POSTGRES_POD" ]; then
    echo "❌ PostgreSQL pod not found"
    exit 1
fi

echo "=========================================="
echo "Clearing Database"
echo "=========================================="
echo ""
echo "PostgreSQL Pod: $POSTGRES_POD"
echo ""

# Clear tables in order (respecting foreign keys)
echo "Clearing tables..."

kubectl exec -n ksam "$POSTGRES_POD" -- psql -U postgres -d ksam <<EOF
-- Disable foreign key checks temporarily
SET session_replication_role = 'replica';

-- Clear tables in order
TRUNCATE TABLE insights CASCADE;
TRUNCATE TABLE service_accounts CASCADE;
TRUNCATE TABLE roles CASCADE;
TRUNCATE TABLE cluster_roles CASCADE;
TRUNCATE TABLE role_bindings CASCADE;
TRUNCATE TABLE cluster_role_bindings CASCADE;
TRUNCATE TABLE pods CASCADE;
TRUNCATE TABLE deployments CASCADE;
TRUNCATE TABLE replica_sets CASCADE;
TRUNCATE TABLE clusters CASCADE;

-- Re-enable foreign key checks
SET session_replication_role = 'origin';

-- Show counts
SELECT 
    'insights' as table_name, COUNT(*) as count FROM insights
UNION ALL
SELECT 'service_accounts', COUNT(*) FROM service_accounts
UNION ALL
SELECT 'pods', COUNT(*) FROM pods
UNION ALL
SELECT 'clusters', COUNT(*) FROM clusters;
EOF

echo ""
echo "✅ Database cleared"
echo ""
