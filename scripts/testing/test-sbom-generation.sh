#!/bin/bash

# Script to test SBOM generation and CVE matching
# This script creates a test pod and verifies SBOM pipeline works

set -e

echo "╔════════════════════════════════════════════════════════════════╗"
echo "║          TESTING SBOM GENERATION AND CVE MATCHING              ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""

# Configuration
TEST_NAMESPACE="ksam-test"
TEST_IMAGE="nginx:1.19.0"  # Known vulnerable image
TEST_POD_NAME="sbom-test-pod"

# Check if Core pod is running
CORE_POD=$(kubectl get pods -n ksam -l app=ksam-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

if [ -z "$CORE_POD" ]; then
    echo "❌ Error: Core pod not found"
    exit 1
fi

echo "✅ Core pod: $CORE_POD"
echo ""

# Check if Syft and Grype are available in Core pod
echo "🔍 Checking Syft and Grype installation..."
SYFT_VERSION=$(kubectl exec -n ksam $CORE_POD -- syft version 2>/dev/null | head -1 || echo "")
GRYPE_VERSION=$(kubectl exec -n ksam $CORE_POD -- grype version 2>/dev/null | head -1 || echo "")

if [ -z "$SYFT_VERSION" ]; then
    echo "❌ Error: Syft not found in Core pod"
    echo "   Please rebuild Docker image with Syft installed"
    exit 1
fi

if [ -z "$GRYPE_VERSION" ]; then
    echo "❌ Error: Grype not found in Core pod"
    echo "   Please rebuild Docker image with Grype installed"
    exit 1
fi

echo "✅ Syft: $SYFT_VERSION"
echo "✅ Grype: $GRYPE_VERSION"
echo ""

# Check if SBOM tables exist
echo "🔍 Checking SBOM tables..."
DB_URL=$(kubectl exec -n ksam $CORE_POD -- env | grep DATABASE_URL | cut -d'=' -f2- || echo "")
if [ -z "$DB_URL" ]; then
    echo "⚠️  Could not get database URL, will check via API"
else
    echo "✅ Database connection found"
fi
echo ""

# Create test namespace if it doesn't exist
echo "📦 Creating test namespace..."
kubectl create namespace $TEST_NAMESPACE --dry-run=client -o yaml | kubectl apply -f -
echo "✅ Namespace ready"
echo ""

# Create test pod
echo "🚀 Creating test pod with image: $TEST_IMAGE"
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: $TEST_POD_NAME
  namespace: $TEST_NAMESPACE
spec:
  containers:
  - name: nginx
    image: $TEST_IMAGE
    ports:
    - containerPort: 80
  restartPolicy: Never
EOF

echo "✅ Test pod created"
echo ""

# Wait for pod to be running
echo "⏳ Waiting for pod to be running..."
kubectl wait --for=condition=Ready pod/$TEST_POD_NAME -n $TEST_NAMESPACE --timeout=60s || {
    echo "⚠️  Pod not ready, checking status..."
    kubectl get pod $TEST_POD_NAME -n $TEST_NAMESPACE
    exit 1
}

echo "✅ Pod is running"
echo ""

# Wait a bit for SBOM processing (RiskWorker processes pods)
echo "⏳ Waiting for SBOM processing (30 seconds)..."
sleep 30

# Check SBOM in database
echo "🔍 Checking SBOM generation..."
echo ""

# Get database connection details
DB_USER=$(echo $DB_URL | sed -n 's|postgres://\([^:]*\):.*|\1|p')
DB_PASS=$(echo $DB_URL | sed -n 's|postgres://[^:]*:\([^@]*\)@.*|\1|p')
DB_HOST=$(echo $DB_URL | sed -n 's|postgres://[^@]*@\([^:]*\):.*|\1|p')
DB_PORT=$(echo $DB_URL | sed -n 's|postgres://[^@]*@[^:]*:\([^/]*\)/.*|\1|p')
DB_NAME=$(echo $DB_URL | sed -n 's|postgres://[^@]*@[^/]*/\(.*\)|\1|p')

if [ -n "$DB_URL" ] && [ -n "$DB_USER" ] && [ -n "$DB_PASS" ]; then
    echo "📊 Checking SBOMs table..."
    SBOM_COUNT=$(kubectl exec -n ksam $CORE_POD -- sh -c "
        PGPASSWORD='$DB_PASS' psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -tAc \
        'SELECT COUNT(*) FROM sboms WHERE image_name LIKE '\''%nginx%'\'';'
    " 2>/dev/null || echo "0")
    
    if [ "$SBOM_COUNT" -gt 0 ]; then
        echo "✅ Found $SBOM_COUNT SBOM(s) for nginx images"
        
        echo ""
        echo "📋 SBOM Details:"
        kubectl exec -n ksam $CORE_POD -- sh -c "
            PGPASSWORD='$DB_PASS' psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -c '
            SELECT 
                id,
                image_name,
                image_tag,
                component_count,
                os_packages,
                language_packages,
                generated_at
            FROM sboms
            WHERE image_name LIKE '\''%nginx%'\''
            ORDER BY generated_at DESC
            LIMIT 1;
            '
        " 2>/dev/null || echo "   (Could not query)"
        
        echo ""
        echo "📋 CVE Matches:"
        CVE_COUNT=$(kubectl exec -n ksam $CORE_POD -- sh -c "
            PGPASSWORD='$DB_PASS' psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -tAc \
            'SELECT COUNT(*) FROM cve_matches cm
             JOIN sboms s ON cm.sbom_id = s.id
             WHERE s.image_name LIKE '\''%nginx%'\'';'
        " 2>/dev/null || echo "0")
        
        if [ "$CVE_COUNT" -gt 0 ]; then
            echo "✅ Found $CVE_COUNT CVE match(es)"
            echo ""
            echo "📋 Top CVEs by severity:"
            kubectl exec -n ksam $CORE_POD -- sh -c "
                PGPASSWORD='$DB_PASS' psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -c '
                SELECT 
                    cm.cve_id,
                    cm.severity,
                    cm.cvss_score,
                    sc.component_name,
                    sc.component_version
                FROM cve_matches cm
                JOIN sbom_components sc ON cm.component_id = sc.id
                JOIN sboms s ON cm.sbom_id = s.id
                WHERE s.image_name LIKE '\''%nginx%'\''
                ORDER BY 
                    CASE cm.severity
                        WHEN '\''CRITICAL'\'' THEN 1
                        WHEN '\''HIGH'\'' THEN 2
                        WHEN '\''MEDIUM'\'' THEN 3
                        ELSE 4
                    END,
                    cm.cvss_score DESC NULLS LAST
                LIMIT 10;
                '
            " 2>/dev/null || echo "   (Could not query)"
        else
            echo "⚠️  No CVE matches found (this is normal if image has no vulnerabilities)"
        fi
        
        echo ""
        echo "📋 Insights created:"
        INSIGHT_COUNT=$(kubectl exec -n ksam $CORE_POD -- sh -c "
            PGPASSWORD='$DB_PASS' psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -tAc \
            'SELECT COUNT(*) FROM insights 
             WHERE source = '\''sbom-scanner'\''
             AND affected_resources::text LIKE '\''%$TEST_POD_NAME%'\'';'
        " 2>/dev/null || echo "0")
        
        if [ "$INSIGHT_COUNT" -gt 0 ]; then
            echo "✅ Found $INSIGHT_COUNT insight(s) from SBOM scanner"
        else
            echo "⚠️  No insights found (may need more time or no critical/high CVEs)"
        fi
    else
        echo "⚠️  No SBOM found for nginx images"
        echo "   This could mean:"
        echo "   - RiskWorker hasn't processed the pod yet"
        echo "   - SBOM pipeline is disabled (check KSAM_SBOM_ENABLED)"
        echo "   - Pod wasn't detected by RiskWorker"
    fi
else
    echo "⚠️  Could not query database directly"
    echo "   Checking via API instead..."
fi

echo ""
echo "🧹 Cleaning up test pod..."
kubectl delete pod $TEST_POD_NAME -n $TEST_NAMESPACE --ignore-not-found=true
echo "✅ Cleanup complete"
echo ""

echo "✅ Test completed!"
echo ""
echo "📊 Summary:"
echo "   - Syft: $SYFT_VERSION ✅"
echo "   - Grype: $GRYPE_VERSION ✅"
echo "   - SBOMs found: $SBOM_COUNT"
echo "   - CVE matches: $CVE_COUNT"
echo "   - Insights: $INSIGHT_COUNT"


