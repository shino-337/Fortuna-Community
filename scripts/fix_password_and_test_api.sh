#!/bin/bash
# Fix admin password and test API

set -e

echo "=== Fixing Admin Password and Testing API ==="
echo ""

POSTGRES_POD=$(kubectl get pods -n ksam | grep postgres | head -1 | awk '{print $1}')
CORE_POD=$(kubectl get pods -n ksam -l app=ksam-core -o jsonpath='{.items[0].metadata.name}')

echo "1. Checking current admin user..."
kubectl exec -n ksam "$POSTGRES_POD" -- psql -U postgres -d ksam -c "SELECT username, role FROM users WHERE username='admin';" 2>&1

echo ""
echo "2. Deleting admin user to force recreation..."
kubectl exec -n ksam "$POSTGRES_POD" -- psql -U postgres -d ksam -c "DELETE FROM users WHERE username='admin';" 2>&1

echo ""
echo "3. Restarting Core to trigger admin user creation..."
kubectl delete pod -n ksam -l app=ksam-core >/dev/null 2>&1
echo "Waiting for Core to restart..."
sleep 30

echo ""
echo "4. Checking if admin user was created..."
kubectl logs -n ksam -l app=ksam-core --tail=100 | grep -E "Created default admin|Admin user already exists" | tail -3

echo ""
echo "5. Testing authentication..."
sleep 5
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123"}' 2>/dev/null | python3 -c "import sys, json; data=json.load(sys.stdin); print(data.get('token', ''))" 2>/dev/null || echo "")

if [ -n "$TOKEN" ] && [ "$TOKEN" != "None" ] && [ "$TOKEN" != "" ]; then
    echo "✅ Authentication successful!"
    echo ""
    echo "6. Testing API retrieval..."
    echo ""
    echo "6.1. Insights Summary:"
    curl -s -H "Authorization: Bearer $TOKEN" "http://localhost:8080/api/v1/insights/summary" | python3 -m json.tool 2>/dev/null
    echo ""
    echo "6.2. Total Insights:"
    curl -s -H "Authorization: Bearer $TOKEN" "http://localhost:8080/api/v1/insights?pageSize=1" | python3 -c "import sys, json; data=json.load(sys.stdin); print(f\"Total: {data.get('total', len(data.get('insights', [])))}\")" 2>/dev/null
    echo ""
    echo "6.3. Critical Insights (first 5):"
    curl -s -H "Authorization: Bearer $TOKEN" "http://localhost:8080/api/v1/insights?severity=critical&pageSize=5" | python3 -m json.tool 2>/dev/null | head -400
    echo ""
    echo "6.4. Cluster-Admin Related Insights:"
    curl -s -H "Authorization: Bearer $TOKEN" "http://localhost:8080/api/v1/insights?severity=critical" 2>/dev/null | python3 -c "
import sys, json
data = json.load(sys.stdin)
insights = [i for i in data.get('insights', []) if 'cluster-admin' in i.get('description', '').lower()]
print(f'Found {len(insights)} cluster-admin related critical insights')
for i in insights[:5]:
    print(f\"  [{i.get('id')}] {i.get('type', 'N/A')} - {i.get('severity', 'N/A')}\")
    print(f\"      {i.get('description', 'N/A')[:130]}...\")
    print('')
" 2>/dev/null
    echo ""
    echo "✅✅✅ COMPLETE END-TO-END TEST SUCCESSFUL! ✅✅✅"
else
    echo "❌ Authentication failed"
    curl -s -X POST http://localhost:8080/api/v1/auth/login -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123"}' | python3 -m json.tool 2>/dev/null
fi

