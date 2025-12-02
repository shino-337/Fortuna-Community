#!/bin/bash

# Setup script for test environment
# Checks and prepares all conditions needed for skipped testcases

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "=========================================="
echo "Setting up Test Environment"
echo "=========================================="
echo ""

# 1. Check Minikube
echo "[1/7] Checking Minikube..."
if minikube status > /dev/null 2>&1; then
    echo "  ✅ Minikube is running"
    minikube status | head -3
else
    echo "  ❌ Minikube is not running"
    echo "  Run: minikube start"
    exit 1
fi
echo ""

# 2. Check namespace
echo "[2/7] Checking namespace..."
if kubectl get namespace ksam > /dev/null 2>&1; then
    echo "  ✅ Namespace 'ksam' exists"
else
    echo "  ⚠️  Namespace 'ksam' not found"
    echo "  Run: kubectl create namespace ksam"
fi
echo ""

# 3. Check Core pod
echo "[3/7] Checking Core pod..."
CORE_POD=$(kubectl get pods -n ksam -o name 2>/dev/null | grep -E "(core|ksam-core)" | head -1 | sed 's|pod/||' || echo "")
if [ -n "$CORE_POD" ]; then
    STATUS=$(kubectl get pod -n ksam "$CORE_POD" -o jsonpath='{.status.phase}' 2>/dev/null || echo "")
    if [ "$STATUS" = "Running" ]; then
        echo "  ✅ Core pod found: $CORE_POD (Running)"
    else
        echo "  ⚠️  Core pod found: $CORE_POD (Status: $STATUS)"
    fi
else
    echo "  ❌ Core pod not found"
    echo "  Run: kubectl apply -f deploy/core-deployment.yaml"
    CORE_AVAILABLE=false
fi
echo ""

# 4. Check Postgres pod
echo "[4/7] Checking Postgres pod..."
POSTGRES_POD=$(kubectl get pods -n ksam -o name 2>/dev/null | grep postgres | head -1 | sed 's|pod/||' || echo "")
if [ -n "$POSTGRES_POD" ]; then
    STATUS=$(kubectl get pod -n ksam "$POSTGRES_POD" -o jsonpath='{.status.phase}' 2>/dev/null || echo "")
    if [ "$STATUS" = "Running" ]; then
        echo "  ✅ Postgres pod found: $POSTGRES_POD (Running)"
        
        # Test database connection
        if kubectl exec -n ksam "$POSTGRES_POD" -- psql -U ksam_user -d ksam -c "SELECT 1;" > /dev/null 2>&1; then
            echo "  ✅ Database connection successful"
            DB_AVAILABLE=true
        else
            echo "  ⚠️  Database connection failed"
            DB_AVAILABLE=false
        fi
    else
        echo "  ⚠️  Postgres pod found: $POSTGRES_POD (Status: $STATUS)"
        DB_AVAILABLE=false
    fi
else
    echo "  ❌ Postgres pod not found"
    echo "  Run: kubectl apply -f deploy/infrastructure/postgresql.yaml"
    DB_AVAILABLE=false
fi
echo ""

# 5. Check TLS certificates
echo "[5/7] Checking TLS certificates..."
if [ -n "$CORE_POD" ]; then
    if kubectl exec -n ksam "$CORE_POD" -- test -f /etc/ksam/certs/tls.crt 2>/dev/null; then
        echo "  ✅ TLS certificates found in Core pod"
        TLS_AVAILABLE=true
    else
        echo "  ⚠️  TLS certificates not found in Core pod"
        TLS_AVAILABLE=false
    fi
    
    # Check TLS secrets
    if kubectl get secret -n ksam ksam-core-tls > /dev/null 2>&1; then
        echo "  ✅ TLS secret 'ksam-core-tls' exists"
    else
        echo "  ⚠️  TLS secret 'ksam-core-tls' not found"
        echo "  Run: scripts/generate_certs.sh && kubectl apply -f deploy/core-secrets.yaml"
    fi
else
    echo "  ⚠️  Cannot check TLS (Core pod not available)"
    TLS_AVAILABLE=false
fi
echo ""

# 6. Check rules directory
echo "[6/7] Checking rules directory..."
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
    echo "  ⚠️  No rules directory found with YAML files"
    echo "  Create rules directory: mkdir -p core/rules"
fi
echo ""

# 7. Check environment variables
echo "[7/7] Checking Core environment variables..."
if [ -n "$CORE_POD" ]; then
    ENV_VARS=$(kubectl exec -n ksam "$CORE_POD" -- env 2>/dev/null || echo "")
    
    if echo "$ENV_VARS" | grep -q "TLS_ENABLED=true"; then
        echo "  ✅ TLS_ENABLED=true"
    else
        echo "  ⚠️  TLS_ENABLED not set to true"
        echo "  Set: kubectl set env deployment/ksam-core -n ksam TLS_ENABLED=true"
    fi
    
    if echo "$ENV_VARS" | grep -q "KSAM_RULES_DIR"; then
        RULES_DIR_VALUE=$(echo "$ENV_VARS" | grep "KSAM_RULES_DIR" | cut -d'=' -f2)
        echo "  ✅ KSAM_RULES_DIR=$RULES_DIR_VALUE"
    else
        echo "  ⚠️  KSAM_RULES_DIR not set"
        echo "  Set: kubectl set env deployment/ksam-core -n ksam KSAM_RULES_DIR=/etc/ksam/rules"
    fi
    
    # Check database env vars
    if echo "$ENV_VARS" | grep -q "DB_HOST"; then
        echo "  ✅ Database environment variables set"
    else
        echo "  ⚠️  Database environment variables not set"
    fi
else
    echo "  ⚠️  Cannot check environment variables (Core pod not available)"
fi
echo ""

# Summary
echo "=========================================="
echo "Environment Setup Summary"
echo "=========================================="
echo ""

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

if [ "$TLS_AVAILABLE" = true ]; then
    echo "✅ TLS Certificates: Available"
else
    echo "❌ TLS Certificates: Not available"
fi

if [ "$found_rules" = true ]; then
    echo "✅ YAML Rules: Found"
else
    echo "❌ YAML Rules: Not found"
fi

echo ""
echo "=========================================="
echo "Test Readiness"
echo "=========================================="
echo ""

# Check which tests can run
CAN_RUN_TESTS=()

if [ "$DB_AVAILABLE" = true ] && [ "$found_rules" = true ]; then
    CAN_RUN_TESTS+=("YAML Rule Loading")
fi

if [ -n "$CORE_POD" ] && [ "$found_rules" = true ]; then
    CAN_RUN_TESTS+=("Hot-Reload File Watcher")
fi

if [ "$TLS_AVAILABLE" = true ]; then
    CAN_RUN_TESTS+=("CertManager Creation")
fi

if [ -n "$CORE_POD" ]; then
    CAN_RUN_TESTS+=("Certificate API Endpoints")
    CAN_RUN_TESTS+=("Prometheus Metrics")
fi

if [ ${#CAN_RUN_TESTS[@]} -gt 0 ]; then
    echo "✅ Can run the following tests:"
    for test in "${CAN_RUN_TESTS[@]}"; do
        echo "  - $test"
    done
else
    echo "⚠️  No tests can run with current setup"
fi

echo ""
echo "For detailed setup instructions, see:"
echo "  docs/TEST_SKIPPED_CASES_ANALYSIS.md"


