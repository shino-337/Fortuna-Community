# Development Scripts

**10 scripts** for development tools, debugging, and certificate generation.

---

## 📝 Scripts

| Script | Purpose |
|--------|---------|
| **generate_certs.sh** | Generate mTLS certificates |
| **generate-webhook-certs.sh** | Generate webhook certificates |
| **generate_hash.py** | Generate password hashes (Python) |
| **generate_password_hash.go** | Generate password hashes (Go) |
| **generate_test_traffic.sh** | Generate test traffic |
| **debug_routes.sh** | Debug API routes |
| **create_admin_and_test_api.sh** | Create admin user and test |
| **fix_admin_password.sh** | Fix admin password |
| **fix_password_and_test_api.sh** | Fix password and test API |
| **create_test_template.sh** | Create test policy template |

---

## 🚀 Quick Start

### Generate Certificates
```bash
# mTLS certificates
./generate_certs.sh

# Webhook certificates
./generate-webhook-certs.sh
```

### Debug API
```bash
./debug_routes.sh
```

### Generate Test Data
```bash
./generate_test_traffic.sh
```

---

## 📚 Detailed Usage

### generate_certs.sh
Generate mTLS certificates for Agent-Core communication:
```bash
./generate_certs.sh
```

Generates:
- CA certificate
- Server certificate
- Client certificate
- Keys

Output: `certs/` directory

---

### generate-webhook-certs.sh
Generate certificates for admission webhook:
```bash
./generate-webhook-certs.sh
```

Generates:
- Webhook TLS certificate
- CA bundle for K8s

Output: `webhook-certs/` directory

---

### generate_hash.py
Generate bcrypt password hash (Python):
```bash
# Interactive
./generate_hash.py

# With password
./generate_hash.py mypassword

# Output
$2b$10$abcdef123456...
```

Requires: Python 3, bcrypt module

---

### generate_password_hash.go
Generate bcrypt password hash (Go):
```bash
# Interactive
go run generate_password_hash.go

# With password
echo "mypassword" | go run generate_password_hash.go

# Output
$2a$10$abcdef123456...
```

Requires: Go 1.21+

---

### generate_test_traffic.sh
Generate test traffic to API:
```bash
# Generate traffic
./generate_test_traffic.sh

# Custom requests per second
RPS=10 ./generate_test_traffic.sh

# Custom duration
DURATION=60 ./generate_test_traffic.sh
```

Generates:
- API requests
- Webhook calls
- NATS messages

---

### debug_routes.sh
Debug API routes and endpoints:
```bash
./debug_routes.sh
```

Shows:
- Registered routes
- Middleware chain
- Handler functions
- Request/response examples

---

### create_admin_and_test_api.sh
Create admin user and test API:
```bash
./create_admin_and_test_api.sh
```

Actions:
1. Creates admin user
2. Gets JWT token
3. Tests API endpoints
4. Shows results

---

### fix_admin_password.sh
Fix admin user password:
```bash
# Interactive (prompts for password)
./fix_admin_password.sh

# With password
PASSWORD=newpassword ./fix_admin_password.sh
```

---

### fix_password_and_test_api.sh
Fix password and test API in one go:
```bash
./fix_password_and_test_api.sh
```

Actions:
1. Resets admin password
2. Gets JWT token
3. Tests all API endpoints

---

### create_test_template.sh
Create test policy template:
```bash
# Create default template
./create_test_template.sh

# Create custom template
TEMPLATE_NAME="my-policy" ./create_test_template.sh
```

Creates:
- Policy template YAML
- Test cases
- Documentation

---

## 🔧 Development Workflows

### Setup Development Environment
```bash
# 1. Generate certificates
./generate_certs.sh
./generate-webhook-certs.sh

# 2. Create admin user
./create_admin_and_test_api.sh

# 3. Generate test data
./generate_test_traffic.sh
```

### Debug API Issues
```bash
# 1. Check routes
./debug_routes.sh

# 2. Test authentication
./fix_password_and_test_api.sh

# 3. Generate test traffic
./generate_test_traffic.sh
```

### Password Management
```bash
# Python version
./generate_hash.py mypassword

# Go version
echo "mypassword" | go run generate_password_hash.go

# Update in database
./fix_admin_password.sh
```

---

## 🔐 Certificate Management

### mTLS Certificates
```bash
# Generate
./generate_certs.sh

# Files created
certs/
├── ca.crt          # CA certificate
├── ca.key          # CA key
├── server.crt      # Server certificate
├── server.key      # Server key
├── client.crt      # Client certificate
└── client.key      # Client key

# Use in deployment
kubectl create secret tls fortuna-mtls \
  --cert=certs/server.crt \
  --key=certs/server.key
```

### Webhook Certificates
```bash
# Generate
./generate-webhook-certs.sh

# Files created
webhook-certs/
├── tls.crt         # Webhook certificate
├── tls.key         # Webhook key
└── ca-bundle.pem   # CA bundle

# Use in deployment
kubectl create secret tls webhook-certs \
  --cert=webhook-certs/tls.crt \
  --key=webhook-certs/tls.key
```

---

## 🧪 Testing Development Changes

### After Code Changes
```bash
# 1. Rebuild
../deployment/rebuild_and_deploy.sh

# 2. Test API
./fix_password_and_test_api.sh

# 3. Generate traffic
./generate_test_traffic.sh

# 4. Monitor
../monitoring/monitor_fortuna.sh
```

### Debug Workflow
```bash
# 1. Check routes
./debug_routes.sh

# 2. Test authentication
./create_admin_and_test_api.sh

# 3. Check logs
kubectl logs -n fortuna -l app=fortuna-core --tail=100

# 4. Test specific endpoint
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/insights
```

---

## 🚨 Troubleshooting

### Certificate Issues
```bash
# Regenerate certificates
./generate_certs.sh

# Verify certificates
openssl x509 -in certs/server.crt -text -noout

# Check expiry
openssl x509 -in certs/server.crt -noout -enddate
```

### Authentication Issues
```bash
# Fix admin password
./fix_admin_password.sh

# Test API
./create_admin_and_test_api.sh

# Check database
kubectl exec -n fortuna postgres-xxx -- \
  psql -U postgres -d fortuna -c "SELECT * FROM users WHERE role='admin';"
```

### API Not Responding
```bash
# Check routes
./debug_routes.sh

# Check pod
kubectl get pods -n fortuna -l app=fortuna-core

# Check logs
kubectl logs -n fortuna -l app=fortuna-core --tail=50

# Restart
kubectl rollout restart deployment/fortuna-core -n fortuna
```

---

## 📖 Related Documentation

- [Development Guide](../../docs/04-development/README.md)
- [API Documentation](../../docs/04-development/API_VERIFICATION_RESULTS.md)
- [Security Guide](../../docs/06-reference/SECURITY.md)

---

*Back to [Scripts README](../README.md)*

