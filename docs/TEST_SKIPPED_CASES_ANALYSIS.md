# Phân tích các Testcase đã Skip và Điều kiện Cần thiết

**Date**: $(date +"%Y-%m-%d %H:%M:%S")  
**Purpose**: Phân tích chi tiết các testcase đã bị skip và điều kiện để chạy chúng

---

## Tổng quan

Có **5 testcase đã bị skip** do thiếu điều kiện runtime environment. Tài liệu này phân tích từng testcase và đưa ra hướng dẫn setup đầy đủ.

---

## Issue #1: CEL Engine & Hot-Reload

### Test 4: YAML Rule Loading

**Status**: ⏭️ SKIPPED  
**Lý do**: Requires database connection

#### Điều kiện cần thiết:

1. **PostgreSQL Database đang chạy**
   ```bash
   # Kiểm tra PostgreSQL pod
   kubectl get pods -n ksam | grep postgres
   
   # Kết quả mong đợi:
   # postgres-747fc6cdfb-7bcx5   1/1   Running
   ```

2. **Database connection string**
   ```bash
   # Environment variables cần thiết:
   DB_HOST=postgres.ksam.svc.cluster.local
   DB_PORT=5432
   DB_USER=ksam_user
   DB_PASSWORD=<password>
   DB_NAME=ksam
   ```

3. **Database schema đã được migrate**
   ```bash
   # Chạy migrations
   kubectl exec -n ksam <core-pod> -- /app/core migrate
   ```

4. **Rules directory có YAML files**
   ```bash
   # Kiểm tra rules directory
   ls -la core/rules/
   
   # Hoặc trong pod:
   kubectl exec -n ksam <core-pod> -- ls -la /etc/ksam/rules/
   ```

#### Cách test:

```bash
# 1. Setup database connection
export DB_HOST=postgres.ksam.svc.cluster.local
export DB_PORT=5432
export DB_USER=ksam_user
export DB_PASSWORD=$(kubectl get secret -n ksam postgres -o jsonpath='{.data.password}' | base64 -d)
export DB_NAME=ksam

# 2. Set rules directory
export KSAM_RULES_DIR=/etc/ksam/rules

# 3. Run test
cd core
go test -v ./pkg/riskengine -run TestYAMLRuleLoading
```

#### Test script mẫu:

```go
func TestYAMLRuleLoading(t *testing.T) {
    // Setup database connection
    db, err := setupTestDB()
    require.NoError(t, err)
    
    // Create YAMLEngine
    rulesDir := os.Getenv("KSAM_RULES_DIR")
    if rulesDir == "" {
        rulesDir = "../../rules" // Default
    }
    
    engine, err := riskengine.NewYAMLEngine(db, rulesDir)
    require.NoError(t, err)
    
    // Verify rules loaded
    rules := engine.GetRules()
    assert.Greater(t, len(rules), 0, "Should load at least one rule")
    
    // Verify CEL expressions compiled
    for _, rule := range rules {
        for _, cond := range rule.Conditions {
            if cond.Type == riskengine.CondTypeExpression {
                assert.NotEmpty(t, cond.Expression, "Expression should not be empty")
            }
        }
    }
}
```

---

### Test 5: Hot-Reload File Watcher

**Status**: ⏭️ SKIPPED  
**Lý do**: Requires running service

#### Điều kiện cần thiết:

1. **Core service đang chạy**
   ```bash
   # Kiểm tra Core pod
   kubectl get pods -n ksam | grep core
   
   # Kết quả mong đợi:
   # ksam-core-7f94cfcf8-772m9   1/1   Running
   ```

2. **Rules directory được mount vào pod**
   ```yaml
   # Trong deployment.yaml
   volumeMounts:
     - name: rules
       mountPath: /etc/ksam/rules
   volumes:
     - name: rules
       configMap:
         name: ksam-rules
   ```

3. **Environment variable KSAM_RULES_DIR được set**
   ```bash
   # Trong pod
   kubectl exec -n ksam <core-pod> -- env | grep KSAM_RULES_DIR
   # Kết quả: KSAM_RULES_DIR=/etc/ksam/rules
   ```

4. **File watcher có quyền đọc rules directory**
   ```bash
   # Kiểm tra permissions
   kubectl exec -n ksam <core-pod> -- ls -la /etc/ksam/rules/
   ```

#### Cách test:

```bash
# 1. Setup port-forward để xem logs
kubectl port-forward -n ksam <core-pod> 8080:8080 &
PF_PID=$!

# 2. Monitor logs trong một terminal
kubectl logs -n ksam <core-pod> -f | grep RuleWatcher &

# 3. Trong terminal khác, modify một YAML file
kubectl exec -n ksam <core-pod> -- sh -c '
  echo "# Test comment" >> /etc/ksam/rules/test-rule.yaml
'

# 4. Kiểm tra logs cho reload message
# Expected log:
# [RuleWatcher] File changed: /etc/ksam/rules/test-rule.yaml
# [RuleWatcher] Reloading rules...
# [RuleWatcher] ✅ Rules reloaded successfully

# 5. Cleanup
kill $PF_PID
```

#### Test script mẫu:

```bash
#!/bin/bash
# test_hot_reload.sh

CORE_POD=$(kubectl get pods -n ksam -l app=ksam-core -o jsonpath='{.items[0].metadata.name}')

# 1. Get initial rule count
INITIAL_COUNT=$(kubectl exec -n ksam $CORE_POD -- sh -c '
  ls -1 /etc/ksam/rules/*.yaml | wc -l
')

# 2. Create a test rule file
kubectl exec -n ksam $CORE_POD -- sh -c 'cat > /etc/ksam/rules/test-hot-reload.yaml << EOF
id: test-hot-reload
name: Test Hot Reload Rule
severity: low
enabled: true
base_score: 1.0
conditions:
  - type: expression
    expression: "object.name == '\''test'\''"
aggregation: all
EOF'

# 3. Wait for reload (should happen within 1-2 seconds)
sleep 3

# 4. Check logs for reload message
RELOADED=$(kubectl logs -n ksam $CORE_POD --tail=50 | grep -c "Rules reloaded successfully")

if [ "$RELOADED" -gt 0 ]; then
    echo "✅ Hot-reload test PASSED"
else
    echo "❌ Hot-reload test FAILED - no reload detected"
fi

# 5. Cleanup
kubectl exec -n ksam $CORE_POD -- rm -f /etc/ksam/rules/test-hot-reload.yaml
```

---

## Issue #5: mTLS Advanced Features

### Test 1: CertManager Creation

**Status**: ⏭️ SKIPPED  
**Lý do**: Requires crypto setup

#### Điều kiện cần thiết:

1. **Certificates đã được generate**
   ```bash
   # Kiểm tra certificates
   ls -la scripts/certs/
   
   # Hoặc trong Kubernetes secrets:
   kubectl get secrets -n ksam | grep tls
   # ksam-core-tls
   # ksam-ca-cert
   ```

2. **Certificate paths hợp lệ**
   ```bash
   # Trong Core pod
   kubectl exec -n ksam <core-pod> -- ls -la /etc/ksam/certs/
   # Kết quả mong đợi:
   # tls.crt
   # tls.key
   # ca.crt
   ```

3. **Environment variables được set**
   ```bash
   kubectl exec -n ksam <core-pod> -- env | grep TLS
   # TLS_ENABLED=true
   # TLS_CERT_PATH=/etc/ksam/certs/tls.crt
   # TLS_KEY_PATH=/etc/ksam/certs/tls.key
   # TLS_CA_CERT_PATH=/etc/ksam/ca-cert/ca.crt
   ```

#### Cách test:

```bash
# 1. Generate test certificates (nếu chưa có)
cd scripts
bash generate_certs.sh

# 2. Create temporary test directory
mkdir -p /tmp/ksam-cert-test
cp scripts/certs/ca.crt /tmp/ksam-cert-test/
cp scripts/certs/core.crt /tmp/ksam-cert-test/tls.crt
cp scripts/certs/core.key /tmp/ksam-cert-test/tls.key

# 3. Run test
cd core
go test -v ./pkg/security -run TestCertManager
```

#### Test code mẫu:

```go
func TestCertManager(t *testing.T) {
    // Setup test certificates
    certPath := "/tmp/ksam-cert-test/tls.crt"
    keyPath := "/tmp/ksam-cert-test/tls.key"
    caCertPath := "/tmp/ksam-cert-test/ca.crt"
    
    // Create CertManager
    cm, err := security.NewCertManager(certPath, keyPath, caCertPath)
    require.NoError(t, err)
    defer cm.Stop()
    
    // Test GetCertificateInfo
    info, err := cm.GetCertificateInfo()
    require.NoError(t, err)
    assert.NotEmpty(t, info.Subject)
    assert.Greater(t, info.DaysUntilExpiry, 0)
    
    // Test GetTLSCertificate
    cert, err := cm.GetTLSCertificate(nil)
    require.NoError(t, err)
    assert.NotNil(t, cert)
    
    // Test rotation
    err = cm.RotateCertificate()
    assert.NoError(t, err)
}
```

---

### Test 2: Certificate API Endpoints

**Status**: ⏭️ SKIPPED  
**Lý do**: Service not running

#### Điều kiện cần thiết:

1. **Core service đang chạy và healthy**
   ```bash
   # Kiểm tra pod status
   kubectl get pods -n ksam -l app=ksam-core
   
   # Kiểm tra health endpoint
   kubectl port-forward -n ksam <core-pod> 8080:8080 &
   curl http://localhost:8080/health
   ```

2. **Port-forward hoặc Service accessible**
   ```bash
   # Option 1: Port-forward
   kubectl port-forward -n ksam <core-pod> 8080:8080
   
   # Option 2: Service (nếu có LoadBalancer hoặc NodePort)
   kubectl get svc -n ksam ksam-core
   ```

3. **TLS enabled trong Core**
   ```bash
   # Kiểm tra logs
   kubectl logs -n ksam <core-pod> | grep "TLS_ENABLED"
   # Kết quả: TLS_ENABLED=true
   ```

4. **CertManager được khởi tạo**
   ```bash
   # Kiểm tra logs
   kubectl logs -n ksam <core-pod> | grep CertManager
   # Kết quả: [CertManager] Certificate loaded successfully
   ```

#### Cách test:

```bash
#!/bin/bash
# test_certificate_api.sh

CORE_POD=$(kubectl get pods -n ksam -l app=ksam-core -o jsonpath='{.items[0].metadata.name}')

# Setup port-forward
kubectl port-forward -n ksam $CORE_POD 8080:8080 > /dev/null 2>&1 &
PF_PID=$!
sleep 3

# Test 1: GET /api/v1/certificates/info
echo "Testing GET /api/v1/certificates/info..."
RESPONSE=$(curl -s http://localhost:8080/api/v1/certificates/info)

if echo "$RESPONSE" | jq -e '.subject' > /dev/null 2>&1; then
    echo "✅ Certificate info endpoint PASSED"
    echo "$RESPONSE" | jq '.'
else
    echo "❌ Certificate info endpoint FAILED"
    echo "Response: $RESPONSE"
fi

# Test 2: POST /api/v1/certificates/rotate
echo ""
echo "Testing POST /api/v1/certificates/rotate..."
ROTATE_RESPONSE=$(curl -s -X POST http://localhost:8080/api/v1/certificates/rotate)

if echo "$ROTATE_RESPONSE" | grep -q "successfully"; then
    echo "✅ Certificate rotation endpoint PASSED"
    echo "$ROTATE_RESPONSE" | jq '.'
else
    echo "⚠️  Certificate rotation endpoint (may require auth)"
    echo "Response: $ROTATE_RESPONSE"
fi

# Cleanup
kill $PF_PID
```

---

### Test 3: Prometheus Metrics

**Status**: ⏭️ SKIPPED  
**Lý do**: Service not running

#### Điều kiện cần thiết:

1. **Core service đang chạy**
   ```bash
   kubectl get pods -n ksam -l app=ksam-core
   ```

2. **Metrics endpoint accessible**
   ```bash
   # Port-forward
   kubectl port-forward -n ksam <core-pod> 8080:8080
   
   # Test metrics endpoint
   curl http://localhost:8080/metrics
   ```

3. **Prometheus metrics được register**
   ```bash
   # Kiểm tra metrics có được expose không
   curl -s http://localhost:8080/metrics | grep ksam_cert
   ```

4. **TLS enabled (để có certificate metrics)**
   ```bash
   # Nếu TLS không enabled, certificate metrics sẽ không có giá trị
   kubectl logs -n ksam <core-pod> | grep "TLS_ENABLED"
   ```

#### Cách test:

```bash
#!/bin/bash
# test_prometheus_metrics.sh

CORE_POD=$(kubectl get pods -n ksam -l app=ksam-core -o jsonpath='{.items[0].metadata.name}')

# Setup port-forward
kubectl port-forward -n ksam $CORE_POD 8080:8080 > /dev/null 2>&1 &
PF_PID=$!
sleep 3

# Get metrics
METRICS=$(curl -s http://localhost:8080/metrics)

# Expected certificate metrics
CERT_METRICS=(
    "ksam_cert_expiry_timestamp"
    "ksam_cert_days_until_expiry"
    "ksam_cert_expiry_warning_total"
    "ksam_cert_expiry_critical_total"
    "ksam_cert_expired_total"
    "ksam_cert_rotation_total"
    "ksam_cert_rotation_failure_total"
    "ksam_cert_rotation_duration_seconds"
    "ksam_last_cert_rotation_timestamp"
)

echo "Checking Prometheus metrics..."
FOUND=0
MISSING=0

for metric in "${CERT_METRICS[@]}"; do
    if echo "$METRICS" | grep -q "^$metric"; then
        VALUE=$(echo "$METRICS" | grep "^$metric" | head -1 | awk '{print $2}')
        echo "  ✅ $metric = $VALUE"
        FOUND=$((FOUND + 1))
    else
        echo "  ❌ $metric - NOT FOUND"
        MISSING=$((MISSING + 1))
    fi
done

echo ""
echo "Results: $FOUND/9 metrics found"

if [ $FOUND -eq 9 ]; then
    echo "✅ All certificate metrics present"
elif [ $FOUND -gt 0 ]; then
    echo "⚠️  Some metrics missing (may be normal if TLS not enabled)"
else
    echo "❌ No certificate metrics found"
fi

# Cleanup
kill $PF_PID
```

---

## Setup Script Tổng hợp

Tạo script để setup tất cả điều kiện cần thiết:

```bash
#!/bin/bash
# setup_test_environment.sh

set -e

echo "=========================================="
echo "Setting up Test Environment"
echo "=========================================="

# 1. Check Minikube
echo "[1/6] Checking Minikube..."
if minikube status > /dev/null 2>&1; then
    echo "  ✅ Minikube is running"
else
    echo "  ❌ Minikube is not running"
    exit 1
fi

# 2. Check Core pod
echo "[2/6] Checking Core pod..."
CORE_POD=$(kubectl get pods -n ksam -o name 2>/dev/null | grep -E "(core|ksam-core)" | head -1 | sed 's|pod/||' || echo "")
if [ -n "$CORE_POD" ]; then
    echo "  ✅ Core pod found: $CORE_POD"
else
    echo "  ❌ Core pod not found"
    echo "  Run: kubectl apply -f deploy/"
    exit 1
fi

# 3. Check Postgres pod
echo "[3/6] Checking Postgres pod..."
POSTGRES_POD=$(kubectl get pods -n ksam -o name 2>/dev/null | grep postgres | head -1 | sed 's|pod/||' || echo "")
if [ -n "$POSTGRES_POD" ]; then
    echo "  ✅ Postgres pod found: $POSTGRES_POD"
else
    echo "  ❌ Postgres pod not found"
    exit 1
fi

# 4. Check TLS certificates
echo "[4/6] Checking TLS certificates..."
if kubectl exec -n ksam $CORE_POD -- test -f /etc/ksam/certs/tls.crt 2>/dev/null; then
    echo "  ✅ TLS certificates found"
else
    echo "  ⚠️  TLS certificates not found"
    echo "  Run: scripts/generate_certs.sh && kubectl apply -f deploy/core-secrets.yaml"
fi

# 5. Check rules directory
echo "[5/6] Checking rules directory..."
if [ -d "core/rules" ] && [ "$(ls -A core/rules/*.yaml 2>/dev/null | wc -l)" -gt 0 ]; then
    echo "  ✅ Rules directory found with $(ls core/rules/*.yaml | wc -l) files"
else
    echo "  ⚠️  Rules directory not found or empty"
fi

# 6. Check environment variables
echo "[6/6] Checking Core environment variables..."
TLS_ENABLED=$(kubectl exec -n ksam $CORE_POD -- env 2>/dev/null | grep "TLS_ENABLED" || echo "")
RULES_DIR=$(kubectl exec -n ksam $CORE_POD -- env 2>/dev/null | grep "KSAM_RULES_DIR" || echo "")

if [ -n "$TLS_ENABLED" ]; then
    echo "  ✅ $TLS_ENABLED"
else
    echo "  ⚠️  TLS_ENABLED not set"
fi

if [ -n "$RULES_DIR" ]; then
    echo "  ✅ $RULES_DIR"
else
    echo "  ⚠️  KSAM_RULES_DIR not set"
fi

echo ""
echo "=========================================="
echo "Environment Setup Complete"
echo "=========================================="
echo ""
echo "Next steps:"
echo "  1. Run: bash scripts/test_runtime_full.sh"
echo "  2. Run: bash scripts/test_certificate_api.sh"
echo "  3. Run: bash scripts/test_prometheus_metrics.sh"
```

---

## Checklist để Test Tất cả Cases

### Pre-requisites

- [ ] Minikube đang chạy
- [ ] Core pod đang running (1/1 Ready)
- [ ] Postgres pod đang running (1/1 Ready)
- [ ] TLS certificates đã được generate và mount vào Core pod
- [ ] Rules directory có ít nhất 1 YAML file
- [ ] Environment variables được set trong Core pod:
  - [ ] `TLS_ENABLED=true`
  - [ ] `KSAM_RULES_DIR=/etc/ksam/rules`
  - [ ] Database connection variables

### Test Execution Order

1. **Setup Environment**
   ```bash
   bash scripts/setup_test_environment.sh
   ```

2. **Run Unit Tests** (không cần runtime)
   ```bash
   bash scripts/test_issue1_cel_hotreload.sh
   bash scripts/test_issue5_mtls_advanced.sh
   ```

3. **Run Runtime Tests**
   ```bash
   bash scripts/test_runtime_full.sh
   ```

4. **Run Integration Tests**
   ```bash
   bash scripts/test_certificate_api.sh
   bash scripts/test_prometheus_metrics.sh
   bash scripts/test_hot_reload.sh
   ```

---

## Troubleshooting

### Core pod không accessible

```bash
# Check pod status
kubectl describe pod -n ksam <core-pod>

# Check logs
kubectl logs -n ksam <core-pod>

# Restart pod
kubectl delete pod -n ksam <core-pod>
```

### Database connection failed

```bash
# Check Postgres pod
kubectl logs -n ksam <postgres-pod>

# Test connection manually
kubectl exec -n ksam <postgres-pod> -- psql -U ksam_user -d ksam -c "SELECT 1;"
```

### TLS certificates not found

```bash
# Generate certificates
cd scripts
bash generate_certs.sh

# Apply secrets
kubectl apply -f deploy/core-secrets.yaml

# Restart Core pod
kubectl delete pod -n ksam <core-pod>
```

### Metrics không có certificate metrics

```bash
# Check if TLS is enabled
kubectl logs -n ksam <core-pod> | grep TLS_ENABLED

# Check CertManager logs
kubectl logs -n ksam <core-pod> | grep CertManager

# If TLS not enabled, enable it:
kubectl set env deployment/ksam-core -n ksam TLS_ENABLED=true
kubectl rollout restart deployment/ksam-core -n ksam
```

---

**Document Generated**: $(date +"%Y-%m-%d %H:%M:%S")


