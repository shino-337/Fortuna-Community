#!/bin/bash

# Full runtime test script with proper pod detection

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "=========================================="
echo "Full Runtime Environment Tests"
echo "=========================================="
echo ""

# Find pods
CORE_POD=$(kubectl get pods -n ksam -o name 2>/dev/null | grep -E "(core|ksam-core)" | head -1 | sed 's|pod/||' || echo "")
POSTGRES_POD=$(kubectl get pods -n ksam -o name 2>/dev/null | grep -E "postgres" | head -1 | sed 's|pod/||' || echo "")

echo "Detected pods:"
echo "  Core: $CORE_POD"
echo "  Postgres: $POSTGRES_POD"
echo ""

# Test 1: Core Service Health
echo "[TEST 1] Core Service Health Check..."
echo "-------------------------------------------"
if [ -n "$CORE_POD" ]; then
    kubectl port-forward -n ksam "$CORE_POD" 8080:8080 > /tmp/pf_core.log 2>&1 &
    PF_PID=$!
    sleep 3
    
    if health=$(curl -s http://localhost:8080/health 2>/dev/null); then
        echo "  ✅ Core service is healthy"
        echo "  Response: $health"
    else
        echo "  ⚠️  Core service health check failed"
    fi
    
    kill $PF_PID 2>/dev/null || true
    sleep 1
else
    echo "  ❌ Core pod not found"
fi
echo ""

# Test 2: Certificate API Endpoints
echo "[TEST 2] Certificate API Endpoints..."
echo "-------------------------------------------"
if [ -n "$CORE_POD" ]; then
    kubectl port-forward -n ksam "$CORE_POD" 8080:8080 > /tmp/pf_core.log 2>&1 &
    PF_PID=$!
    sleep 3
    
    # Test certificate info endpoint
    echo "  Testing GET /api/v1/certificates/info..."
    cert_info=$(curl -s http://localhost:8080/api/v1/certificates/info 2>/dev/null || echo "")
    if echo "$cert_info" | grep -q "subject\|error" 2>/dev/null; then
        echo "  ✅ Certificate info endpoint responded"
        echo "$cert_info" | jq '.' 2>/dev/null || echo "$cert_info" | head -5
    else
        echo "  ⚠️  Certificate info endpoint not accessible (may require auth or TLS not enabled)"
    fi
    
    kill $PF_PID 2>/dev/null || true
    sleep 1
else
    echo "  ❌ Core pod not found"
fi
echo ""

# Test 3: Prometheus Metrics
echo "[TEST 3] Prometheus Metrics..."
echo "-------------------------------------------"
if [ -n "$CORE_POD" ]; then
    kubectl port-forward -n ksam "$CORE_POD" 8080:8080 > /tmp/pf_core.log 2>&1 &
    PF_PID=$!
    sleep 3
    
    metrics=$(curl -s http://localhost:8080/metrics 2>/dev/null || echo "")
    
    if [ -n "$metrics" ]; then
        cert_metrics=(
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
        
        found=0
        missing=0
        
        for metric in "${cert_metrics[@]}"; do
            if echo "$metrics" | grep -q "^$metric"; then
                echo "  ✅ Found: $metric"
                value=$(echo "$metrics" | grep "^$metric" | head -1 | awk '{print $2}')
                echo "      Value: $value"
                found=$((found + 1))
            else
                echo "  ❌ Missing: $metric"
                missing=$((missing + 1))
            fi
        done
        
        echo ""
        echo "  Results: $found/9 metrics found"
        
        if [ $found -gt 0 ]; then
            echo "  ✅ Certificate metrics are exposed"
        else
            echo "  ⚠️  No certificate metrics found (TLS may not be enabled)"
        fi
    else
        echo "  ⚠️  Metrics endpoint not accessible"
    fi
    
    kill $PF_PID 2>/dev/null || true
    sleep 1
else
    echo "  ❌ Core pod not found"
fi
echo ""

# Test 4: Database Connection
echo "[TEST 4] Database Connection..."
echo "-------------------------------------------"
if [ -n "$POSTGRES_POD" ]; then
    if kubectl exec -n ksam "$POSTGRES_POD" -- psql -U ksam_user -d ksam -c "SELECT 1;" > /dev/null 2>&1; then
        echo "  ✅ Database connection successful"
        
        # Check for tables
        table_count=$(kubectl exec -n ksam "$POSTGRES_POD" -- psql -U ksam_user -d ksam -t -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public';" 2>/dev/null | tr -d ' ' || echo "0")
        echo "  ℹ️  Found $table_count tables in database"
        
        DB_AVAILABLE=true
    else
        echo "  ⚠️  Database connection failed"
        DB_AVAILABLE=false
    fi
else
    echo "  ❌ PostgreSQL pod not found"
    DB_AVAILABLE=false
fi
echo ""

# Test 5: Core Logs - TLS/CertManager
echo "[TEST 5] Core Logs - TLS/CertManager Check..."
echo "-------------------------------------------"
if [ -n "$CORE_POD" ]; then
    echo "  Checking Core logs for TLS/CertManager messages..."
    tls_logs=$(kubectl logs -n ksam "$CORE_POD" --tail=100 2>/dev/null | grep -E "(TLS|CertManager|certificate)" | head -10 || echo "")
    
    if [ -n "$tls_logs" ]; then
        echo "  ✅ Found TLS/CertManager related logs:"
        echo "$tls_logs" | while IFS= read -r line; do
            echo "    $line"
        done
    else
        echo "  ⚠️  No TLS/CertManager logs found (may be normal if TLS not enabled)"
    fi
else
    echo "  ❌ Core pod not found"
fi
echo ""

# Test 6: YAML Rules Directory
echo "[TEST 6] YAML Rules Directory Check..."
echo "-------------------------------------------"
RULES_DIRS=(
    "$PROJECT_ROOT/core/rules"
    "/etc/ksam/rules"
)

found_rules=false
for dir in "${RULES_DIRS[@]}"; do
    if [ -d "$dir" ]; then
        rule_count=$(find "$dir" -name "*.yaml" -o -name "*.yml" 2>/dev/null | wc -l | tr -d ' ')
        if [ "$rule_count" -gt 0 ]; then
            echo "  ✅ Found rules directory: $dir"
            echo "  ℹ️  Found $rule_count YAML rule files"
            found_rules=true
            break
        fi
    fi
done

if [ "$found_rules" = false ]; then
    echo "  ⚠️  No rules directory found"
fi
echo ""

echo "=========================================="
echo "Full Runtime Test Summary"
echo "=========================================="
if [ -n "$CORE_POD" ]; then
    echo "✅ Core Pod: $CORE_POD"
else
    echo "❌ Core Pod: Not found"
fi

if [ -n "$POSTGRES_POD" ]; then
    echo "✅ Postgres Pod: $POSTGRES_POD"
else
    echo "❌ Postgres Pod: Not found"
fi

if [ "$DB_AVAILABLE" = true ]; then
    echo "✅ Database: Available"
else
    echo "❌ Database: Not available"
fi

echo ""
echo "All runtime tests completed."

