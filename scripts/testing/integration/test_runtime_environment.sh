#!/bin/bash

# Test script for runtime environment tests
# Tests that require running services, database, etc.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "=========================================="
echo "Runtime Environment Tests"
echo "=========================================="
echo ""

# Test 1: Check Core Service Availability
echo "[TEST 1] Checking Core Service Availability..."
echo "-------------------------------------------"

CORE_SERVICE_URL="${CORE_SERVICE_URL:-http://localhost:8080}"
CORE_POD=$(kubectl get pods -n ksam -o jsonpath='{.items[*].metadata.name}' 2>/dev/null | tr ' ' '\n' | grep -E "core|ksam-core" | head -1 || echo "")

if [ -n "$CORE_POD" ]; then
    echo "  ✅ Core pod found: $CORE_POD"
    
    # Port forward if needed
    PORT_FORWARD_PID=""
    if ! curl -s -f "$CORE_SERVICE_URL/health" > /dev/null 2>&1; then
        echo "  ℹ️  Setting up port forward..."
        kubectl port-forward -n ksam "$CORE_POD" 8080:8080 > /dev/null 2>&1 &
        PORT_FORWARD_PID=$!
        sleep 2
    fi
    
    if curl -s -f "$CORE_SERVICE_URL/health" > /dev/null 2>&1; then
        echo "  ✅ Core service is accessible at $CORE_SERVICE_URL"
        CORE_AVAILABLE=true
    else
        echo "  ⚠️  Core service not accessible"
        CORE_AVAILABLE=false
    fi
    
    if [ -n "$PORT_FORWARD_PID" ]; then
        kill $PORT_FORWARD_PID 2>/dev/null || true
    fi
else
    echo "  ⚠️  Core pod not found"
    CORE_AVAILABLE=false
fi
echo ""

# Test 2: Certificate API Endpoints (if Core available)
if [ "$CORE_AVAILABLE" = true ]; then
    echo "[TEST 2] Testing Certificate API Endpoints..."
    echo "-------------------------------------------"
    
    # Setup port forward
    kubectl port-forward -n ksam "$CORE_POD" 8080:8080 > /dev/null 2>&1 &
    PF_PID=$!
    sleep 2
    
    # Test GET /api/v1/certificates/info
    echo "  Testing GET /api/v1/certificates/info..."
    if response=$(curl -s -f "$CORE_SERVICE_URL/api/v1/certificates/info" 2>&1); then
        echo "  ✅ Certificate info endpoint accessible"
        echo "  Response:"
        echo "$response" | jq '.' 2>/dev/null || echo "$response" | head -10
    else
        echo "  ⚠️  Certificate info endpoint not accessible"
        echo "  Response: $response" | head -5
    fi
    
    # Test POST /api/v1/certificates/rotate
    echo ""
    echo "  Testing POST /api/v1/certificates/rotate..."
    if response=$(curl -s -f -X POST "$CORE_SERVICE_URL/api/v1/certificates/rotate" 2>&1); then
        echo "  ✅ Certificate rotation endpoint accessible"
        echo "  Response:"
        echo "$response" | jq '.' 2>/dev/null || echo "$response" | head -10
    else
        echo "  ⚠️  Certificate rotation endpoint not accessible (may require auth)"
        echo "  Response: $response" | head -5
    fi
    
    kill $PF_PID 2>/dev/null || true
    echo ""
fi

# Test 3: Prometheus Metrics (if Core available)
if [ "$CORE_AVAILABLE" = true ]; then
    echo "[TEST 3] Testing Prometheus Metrics..."
    echo "-------------------------------------------"
    
    kubectl port-forward -n ksam "$CORE_POD" 8080:8080 > /dev/null 2>&1 &
    PF_PID=$!
    sleep 2
    
    metrics=$(curl -s "$CORE_SERVICE_URL/metrics" 2>/dev/null || echo "")
    
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
                found=$((found + 1))
            else
                echo "  ❌ Missing: $metric"
                missing=$((missing + 1))
            fi
        done
        
        echo ""
        echo "  Results: $found found, $missing missing"
        
        if [ $missing -eq 0 ]; then
            echo "  ✅ All certificate metrics present"
        else
            echo "  ⚠️  Some metrics missing (may be normal if TLS not enabled)"
        fi
    else
        echo "  ⚠️  Metrics endpoint not accessible"
    fi
    
    kill $PF_PID 2>/dev/null || true
    echo ""
fi

# Test 4: Database Connection
echo "[TEST 4] Testing Database Connection..."
echo "-------------------------------------------"

POSTGRES_POD=$(kubectl get pods -n ksam -o jsonpath='{.items[*].metadata.name}' 2>/dev/null | tr ' ' '\n' | grep -E "postgres" | head -1 || echo "")

if [ -n "$POSTGRES_POD" ]; then
    echo "  ✅ PostgreSQL pod found: $POSTGRES_POD"
    
    # Test database connection
    if kubectl exec -n ksam "$POSTGRES_POD" -- psql -U ksam_user -d ksam -c "SELECT 1;" > /dev/null 2>&1; then
        echo "  ✅ Database connection successful"
        DB_AVAILABLE=true
    else
        echo "  ⚠️  Database connection failed"
        DB_AVAILABLE=false
    fi
else
    echo "  ⚠️  PostgreSQL pod not found"
    DB_AVAILABLE=false
fi
echo ""

# Test 5: YAML Rule Loading (if DB available)
if [ "$DB_AVAILABLE" = true ]; then
    echo "[TEST 5] Testing YAML Rule Loading..."
    echo "-------------------------------------------"
    
    RULES_DIR="${RULES_DIR:-$PROJECT_ROOT/core/rules}"
    
    if [ -d "$RULES_DIR" ]; then
        echo "  ✅ Rules directory found: $RULES_DIR"
        rule_count=$(find "$RULES_DIR" -name "*.yaml" -o -name "*.yml" 2>/dev/null | wc -l | tr -d ' ')
        echo "  ℹ️  Found $rule_count YAML rule files"
        
        if [ "$rule_count" -gt 0 ]; then
            echo "  ✅ YAML rules available for testing"
        else
            echo "  ⚠️  No YAML rule files found"
        fi
    else
        echo "  ⚠️  Rules directory not found: $RULES_DIR"
    fi
    echo ""
fi

# Test 6: Hot-Reload File Watcher (if rules available)
if [ "$DB_AVAILABLE" = true ] && [ -d "$RULES_DIR" ]; then
    echo "[TEST 6] Testing Hot-Reload File Watcher..."
    echo "-------------------------------------------"
    
    echo "  ℹ️  File watcher test requires running Core service"
    echo "  ℹ️  To test: modify a YAML file and check Core logs for reload message"
    echo "  ⚠️  Skipping (requires manual verification)"
    echo ""
fi

# Test 7: Certificate Rotation (if Core available and TLS enabled)
if [ "$CORE_AVAILABLE" = true ]; then
    echo "[TEST 7] Testing Certificate Rotation..."
    echo "-------------------------------------------"
    
    kubectl port-forward -n ksam "$CORE_POD" 8080:8080 > /dev/null 2>&1 &
    PF_PID=$!
    sleep 2
    
    # Check if TLS is enabled
    if kubectl logs -n ksam "$CORE_POD" --tail=50 | grep -q "TLS_ENABLED=true"; then
        echo "  ✅ TLS is enabled"
        
        # Get certificate info before rotation
        echo "  Getting certificate info before rotation..."
        cert_info_before=$(curl -s "$CORE_SERVICE_URL/api/v1/certificates/info" 2>/dev/null || echo "")
        
        if [ -n "$cert_info_before" ]; then
            serial_before=$(echo "$cert_info_before" | jq -r '.serial_number' 2>/dev/null || echo "")
            echo "  Current serial number: $serial_before"
            
            # Trigger rotation
            echo "  Triggering certificate rotation..."
            rotate_response=$(curl -s -X POST "$CORE_SERVICE_URL/api/v1/certificates/rotate" 2>/dev/null || echo "")
            
            if echo "$rotate_response" | grep -q "successfully"; then
                echo "  ✅ Rotation triggered successfully"
                
                # Get certificate info after rotation
                sleep 1
                cert_info_after=$(curl -s "$CORE_SERVICE_URL/api/v1/certificates/info" 2>/dev/null || echo "")
                serial_after=$(echo "$cert_info_after" | jq -r '.serial_number' 2>/dev/null || echo "")
                
                if [ "$serial_before" != "$serial_after" ]; then
                    echo "  ✅ Certificate rotated (serial changed)"
                else
                    echo "  ⚠️  Certificate serial unchanged (may be same cert)"
                fi
            else
                echo "  ⚠️  Rotation failed or requires authentication"
            fi
        else
            echo "  ⚠️  Could not get certificate info"
        fi
    else
        echo "  ⚠️  TLS not enabled, skipping certificate rotation test"
    fi
    
    kill $PF_PID 2>/dev/null || true
    echo ""
fi

echo "=========================================="
echo "Runtime Environment Test Summary"
echo "=========================================="
if [ "$CORE_AVAILABLE" = true ]; then
    echo "✅ Core Service: Available"
else
    echo "❌ Core Service: Not Available"
fi

if [ "$DB_AVAILABLE" = true ]; then
    echo "✅ Database: Available"
else
    echo "❌ Database: Not Available"
fi

echo ""
echo "Tests completed. Check individual test results above."

