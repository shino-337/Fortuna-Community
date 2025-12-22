#!/bin/bash
# Query insights from database via Core pod

set -e

CORE_POD=$(kubectl get pods -n ksam -l app=ksam-core -o jsonpath='{.items[0].metadata.name}')
POSTGRES_POD=$(kubectl get pods -n ksam | grep postgres | head -1 | awk '{print $1}')

echo "=== Querying Insights from Database ==="
echo ""

# Try via Core pod first (if it has psql)
echo "1. Total Insights:"
kubectl exec -n ksam "$CORE_POD" -- sh -c 'PGPASSWORD=ksam_password psql -h postgres -U ksam_user -d ksam_db -c "SELECT COUNT(*) as total FROM insights;"' 2>/dev/null || echo "Cannot query via Core pod"

echo ""
echo "2. Recent Insights:"
kubectl exec -n ksam "$CORE_POD" -- sh -c 'PGPASSWORD=ksam_password psql -h postgres -U ksam_user -d ksam_db -c "SELECT id, title, severity, type, created_at FROM insights ORDER BY created_at DESC LIMIT 10;"' 2>/dev/null || echo "Cannot query via Core pod"

echo ""
echo "3. Critical Insights:"
kubectl exec -n ksam "$CORE_POD" -- sh -c 'PGPASSWORD=ksam_password psql -h postgres -U ksam_user -d ksam_db -c "SELECT id, title, severity FROM insights WHERE severity='\''critical'\'' ORDER BY created_at DESC LIMIT 10;"' 2>/dev/null || echo "Cannot query via Core pod"

echo ""
echo "4. Insights by Severity:"
kubectl exec -n ksam "$CORE_POD" -- sh -c 'PGPASSWORD=ksam_password psql -h postgres -U ksam_user -d ksam_db -c "SELECT severity, COUNT(*) as count FROM insights GROUP BY severity;"' 2>/dev/null || echo "Cannot query via Core pod"

echo ""
echo "5. Users:"
kubectl exec -n ksam "$CORE_POD" -- sh -c 'PGPASSWORD=ksam_password psql -h postgres -U ksam_user -d ksam_db -c "SELECT username, email, role, active FROM users LIMIT 10;"' 2>/dev/null || echo "Cannot query via Core pod"

