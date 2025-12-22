#!/bin/bash
# Create admin user and test API

set -e

CORE_POD=$(kubectl get pods -n ksam -l app=ksam-core -o jsonpath='{.items[0].metadata.name}')
POSTGRES_POD=$(kubectl get pods -n ksam | grep postgres | head -1 | awk '{print $1}')

echo "=== Creating Admin User and Testing API ==="
echo ""

# Check if admin user exists
echo "1. Checking existing users..."
kubectl exec -n ksam "$POSTGRES_POD" -- psql -U postgres -d ksam_db -c "SELECT username, email, role, active FROM users;" 2>/dev/null || echo "Cannot query users"

echo ""
echo "2. Checking Core logs for admin user creation..."
kubectl logs -n ksam -l app=ksam-core 2>&1 | grep -E "Created default admin|Admin user already exists" | tail -3

echo ""
echo "3. Restarting Core to trigger admin user creation..."
kubectl rollout restart deployment/ksam-core -n ksam
echo "Waiting for Core to restart..."
sleep 25

echo ""
echo "4. Checking if admin user was created..."
kubectl logs -n ksam -l app=ksam-core --tail=50 | grep -E "Created default admin|Admin user already exists" | tail -3

echo ""
echo "5. Verifying users in database..."
kubectl exec -n ksam "$POSTGRES_POD" -- psql -U postgres -d ksam_db -c "SELECT username, email, role, active FROM users;" 2>/dev/null || echo "Cannot query users"

echo ""
echo "6. Testing API authentication..."
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123"}' 2>/dev/null | python3 -c "import sys, json; data=json.load(sys.stdin); print(data.get('token', ''))" 2>/dev/null || echo "")

if [ -n "$TOKEN" ] && [ "$TOKEN" != "None" ] && [ "$TOKEN" != "" ]; then
    echo "✅ Authentication successful!"
    echo ""
    echo "7. Fetching insights summary..."
    curl -s -H "Authorization: Bearer $TOKEN" "http://localhost:8080/api/v1/insights/summary" | python3 -m json.tool 2>/dev/null || echo "Failed to fetch summary"
    
    echo ""
    echo "8. Fetching all insights..."
    curl -s -H "Authorization: Bearer $TOKEN" "http://localhost:8080/api/v1/insights?pageSize=10" | python3 -m json.tool 2>/dev/null | head -100 || echo "Failed to fetch insights"
    
    echo ""
    echo "9. Fetching critical insights..."
    curl -s -H "Authorization: Bearer $TOKEN" "http://localhost:8080/api/v1/insights?severity=critical" | python3 -m json.tool 2>/dev/null || echo "Failed to fetch critical insights"
else
    echo "❌ Authentication failed"
    echo "Response:"
    curl -s -X POST http://localhost:8080/api/v1/auth/login -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123"}' | python3 -m json.tool 2>/dev/null
fi

echo ""
echo "10. Querying insights from database..."
kubectl exec -n ksam "$POSTGRES_POD" -- psql -U postgres -d ksam_db -c "SELECT COUNT(*) as total FROM insights;" 2>/dev/null || echo "Cannot query insights"
kubectl exec -n ksam "$POSTGRES_POD" -- psql -U postgres -d ksam_db -c "SELECT severity, COUNT(*) as count FROM insights GROUP BY severity;" 2>/dev/null || echo "Cannot query insights by severity"

