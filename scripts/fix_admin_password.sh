#!/bin/bash
# Fix admin password hash using Go

set -e

echo "=== Fixing Admin Password Hash ==="
echo ""

# Create temporary Go program to generate bcrypt hash
cat > /tmp/hash_password.go << 'EOF'
package main

import (
	"fmt"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	password := "admin123"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		panic(err)
	}
	fmt.Print(string(hash))
}
EOF

echo "1. Generating bcrypt hash for password 'admin123'..."
HASH=$(cd /tmp && go run hash_password.go 2>/dev/null || echo "")

if [ -z "$HASH" ]; then
    echo "❌ Failed to generate hash - trying alternative method"
    # Use Python bcrypt if Go not available
    HASH=$(python3 -c "import bcrypt; print(bcrypt.hashpw(b'admin123', bcrypt.gensalt(rounds=12)).decode())" 2>/dev/null || echo "")
fi

if [ -z "$HASH" ]; then
    echo "❌ Cannot generate hash - please install Go or Python bcrypt"
    exit 1
fi

echo "✅ Hash generated: ${HASH:0:30}..."

echo ""
echo "2. Updating admin user password in database..."
POSTGRES_POD=$(kubectl get pods -n ksam | grep postgres | head -1 | awk '{print $1}')

# Escape single quotes in hash for SQL
ESCAPED_HASH=$(echo "$HASH" | sed "s/'/''/g")

kubectl exec -n ksam "$POSTGRES_POD" -- psql -U postgres -d ksam -c "UPDATE users SET password='$ESCAPED_HASH' WHERE username='admin';" 2>&1

echo ""
echo "3. Verifying update..."
kubectl exec -n ksam "$POSTGRES_POD" -- psql -U postgres -d ksam -c "SELECT username, LEFT(password, 30) as password_hash FROM users WHERE username='admin';" 2>&1

echo ""
echo "4. Testing authentication..."
sleep 2
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123"}' 2>/dev/null | python3 -c "import sys, json; data=json.load(sys.stdin); print(data.get('token', ''))" 2>/dev/null || echo "")

if [ -n "$TOKEN" ] && [ "$TOKEN" != "None" ] && [ "$TOKEN" != "" ]; then
    echo "✅ Authentication successful!"
    echo ""
    echo "5. Fetching insights summary..."
    curl -s -H "Authorization: Bearer $TOKEN" "http://localhost:8080/api/v1/insights/summary" | python3 -m json.tool 2>/dev/null
    echo ""
    echo "6. Fetching critical insights (first 3)..."
    curl -s -H "Authorization: Bearer $TOKEN" "http://localhost:8080/api/v1/insights?severity=critical&pageSize=3" | python3 -m json.tool 2>/dev/null
else
    echo "❌ Authentication still failing"
    curl -s -X POST http://localhost:8080/api/v1/auth/login -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123"}' | python3 -m json.tool 2>/dev/null
fi

# Cleanup
rm -f /tmp/hash_password.go

