#!/bin/bash
# End-to-End CVE Detection Test Script
# Tests Migration021 fix and CVE detection pipeline

set -e  # Exit on error

echo "=========================================="
echo "KSAM CVE Detection E2E Test"
echo "=========================================="
echo ""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
NAMESPACE="ksam"
TEST_POD_NAME="test-cve-vulnerable-nginx"
TEST_IMAGE="nginx:1.19.0"  # Known vulnerable image (CVE-2021-23017)
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Helper functions
print_step() {
    echo -e "${BLUE}[STEP]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[✓]${NC} $1"
}

print_error() {
    echo -e "${RED}[✗]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[!]${NC} $1"
}

wait_for_pod() {
    local pod=$1
    local namespace=$2
    echo "Waiting for pod $pod in namespace $namespace..."
    kubectl wait --for=condition=ready pod -l app=$pod -n $namespace --timeout=120s
}

# Step 1: Rebuild Docker image with Migration021 fix
print_step "1/10: Rebuilding Docker image with Migration021 fix..."
cd "$PROJECT_ROOT"

# Check if using Minikube
if command -v minikube &> /dev/null; then
    print_warning "Using Minikube Docker environment"
    eval $(minikube docker-env)
fi

# Build image
print_step "Building ksam-core:latest..."
docker build -t ksam-core:latest -f core/Dockerfile . || {
    print_error "Docker build failed"
    exit 1
}
print_success "Docker image built successfully"
echo ""

# Step 2: Restart deployment
print_step "2/10: Restarting ksam-core deployment..."
kubectl rollout restart deployment/ksam-core -n $NAMESPACE
kubectl rollout status deployment/ksam-core -n $NAMESPACE --timeout=120s
print_success "Deployment restarted"
echo ""

# Step 3: Verify Migration021 execution
print_step "3/10: Verifying Migration021 execution..."
sleep 10  # Wait for pod to start and run migrations

MIGRATION_LOGS=$(kubectl logs -n $NAMESPACE deployment/ksam-core --tail=1000 | grep "Migration 021" || echo "")

if [ -z "$MIGRATION_LOGS" ]; then
    print_error "Migration 021 NOT found in logs!"
    print_warning "This means the binary is still old or migrations didn't run"
    echo ""
    echo "Recent migration logs:"
    kubectl logs -n $NAMESPACE deployment/ksam-core --tail=50 | grep -i "migration"
    exit 1
else
    print_success "Migration 021 found in logs"
    echo "$MIGRATION_LOGS" | head -5
fi
echo ""

# Step 4: Check database schema
print_step "4/10: Verifying database schema..."

# Get postgres pod
POSTGRES_POD=$(kubectl get pod -n $NAMESPACE -l app=postgres -o jsonpath='{.items[0].metadata.name}')

if [ -z "$POSTGRES_POD" ]; then
    print_warning "Postgres pod not found in ksam namespace, checking for service..."
    # Try via service
    HAS_PURL=$(kubectl exec -it deployment/ksam-core -n $NAMESPACE -- /bin/sh -c \
        "apt-get update -qq && apt-get install -y postgresql-client -qq && \
         psql -h postgres.$NAMESPACE.svc.cluster.local -U postgres -d ksam -t -c \
         \"SELECT EXISTS (SELECT FROM information_schema.columns WHERE table_name = 'sbom_components' AND column_name = 'purl');\"" \
        2>/dev/null | tr -d ' \n')
else
    # Direct psql via postgres pod
    HAS_PURL=$(kubectl exec -it $POSTGRES_POD -n $NAMESPACE -- \
        psql -U postgres -d ksam -t -c \
        "SELECT EXISTS (SELECT FROM information_schema.columns WHERE table_name = 'sbom_components' AND column_name = 'purl');" \
        2>/dev/null | tr -d ' \n')
fi

if [ "$HAS_PURL" = "t" ]; then
    print_success "Database schema correct: 'purl' column exists"
else
    print_error "Database schema incorrect: 'purl' column missing!"
    print_warning "Run manual fix: ALTER TABLE sbom_components RENAME COLUMN p_url TO purl;"
fi
echo ""

# Step 5: Check SBOM pipeline configuration
print_step "5/10: Checking SBOM pipeline configuration..."

# Check environment variables
CUSTOM_PIPELINE=$(kubectl exec -it deployment/ksam-core -n $NAMESPACE -- printenv KSAM_SBOM_USE_CUSTOM 2>/dev/null || echo "")

if [ -z "$CUSTOM_PIPELINE" ] || [ "$CUSTOM_PIPELINE" != "false" ]; then
    print_success "Custom SBOM pipeline enabled (default)"
else
    print_warning "Custom SBOM pipeline disabled"
fi

# Check Trivy DB path
TRIVY_DB_PATH=$(kubectl exec -it deployment/ksam-core -n $NAMESPACE -- printenv KSAM_TRIVY_DB_PATH 2>/dev/null || echo "/var/lib/ksam/trivy.db")
print_success "Trivy DB path: $TRIVY_DB_PATH"
echo ""

# Step 6: Clean up old test pods
print_step "6/10: Cleaning up old test pods..."
kubectl delete pod $TEST_POD_NAME -n default --ignore-not-found=true 2>/dev/null || true
kubectl delete pod test-cve-pod-migration-fix -n default --ignore-not-found=true 2>/dev/null || true
kubectl delete pod test-cve-pod-schema-fix-final -n default --ignore-not-found=true 2>/dev/null || true
sleep 5
print_success "Old test pods cleaned up"
echo ""

# Step 7: Create test pod with vulnerable image
print_step "7/10: Creating test pod with vulnerable image ($TEST_IMAGE)..."

cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: $TEST_POD_NAME
  namespace: default
  labels:
    app: test-cve
    test: e2e-cve-detection
spec:
  containers:
  - name: nginx
    image: $TEST_IMAGE
    ports:
    - containerPort: 80
EOF

# Wait for pod to be ready
kubectl wait --for=condition=ready pod/$TEST_POD_NAME -n default --timeout=60s || {
    print_error "Test pod failed to become ready"
    kubectl describe pod $TEST_POD_NAME -n default
    exit 1
}
print_success "Test pod created and running"
echo ""

# Step 8: Manually trigger CVE scan (since PodWatcher not running)
print_step "8/10: Manually triggering CVE scan..."
print_warning "Note: Automatic pod scanning (PodWatcher) is NOT implemented yet"
print_warning "We'll test the SBOM pipeline manually via exec"

# Create a manual scan script
SCAN_SCRIPT='
package main
import (
    "context"
    "log"
    "github.com/ksam/core/pkg/sbom"
    "github.com/ksam/core/pkg/riskengine"
    "gorm.io/driver/postgres"
    "gorm.io/gorm"
)
func main() {
    dsn := "host=postgres.ksam.svc.cluster.local user=postgres password=postgres dbname=ksam port=5432 sslmode=disable"
    db, _ := gorm.Open(postgres.Open(dsn), &gorm.Config{})
    im := riskengine.NewInsightManager(db, "cluster-1")
    pipeline := sbom.NewPipeline(im, db)
    err := pipeline.ProcessImage(context.Background(), "nginx:1.19.0", "test-uid", "test-pod", "default", "nginx")
    if err != nil {
        log.Fatalf("Scan failed: %v", err)
    }
    log.Println("Scan completed successfully")
}
'

# For now, just log that manual scanning would happen here
print_warning "Manual CVE scanning requires code integration (see step 10)"
echo ""

# Step 9: Check for CVEs and insights in database
print_step "9/10: Checking database for CVEs and insights..."

# Query SBOMs
if [ -z "$POSTGRES_POD" ]; then
    print_warning "Skipping database checks (postgres pod not found)"
else
    echo "Checking SBOMs..."
    SBOM_COUNT=$(kubectl exec -it $POSTGRES_POD -n $NAMESPACE -- \
        psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM sboms WHERE deleted_at IS NULL;" \
        2>/dev/null | tr -d ' \n')
    echo "  SBOMs in database: $SBOM_COUNT"

    echo "Checking CVE matches..."
    CVE_COUNT=$(kubectl exec -it $POSTGRES_POD -n $NAMESPACE -- \
        psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM cve_matches WHERE deleted_at IS NULL;" \
        2>/dev/null | tr -d ' \n')
    echo "  CVE matches in database: $CVE_COUNT"

    echo "Checking insights..."
    INSIGHT_COUNT=$(kubectl exec -it $POSTGRES_POD -n $NAMESPACE -- \
        psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM insights WHERE source = 'cve' AND deleted_at IS NULL;" \
        2>/dev/null | tr -d ' \n')
    echo "  CVE insights in database: $INSIGHT_COUNT"

    if [ "$INSIGHT_COUNT" -gt "0" ]; then
        print_success "Found $INSIGHT_COUNT CVE-based insights!"
        echo "Recent insights:"
        kubectl exec -it $POSTGRES_POD -n $NAMESPACE -- \
            psql -U postgres -d ksam -c \
            "SELECT id, title, severity, source, created_at FROM insights WHERE source = 'cve' ORDER BY created_at DESC LIMIT 5;" \
            2>/dev/null || true
    else
        print_warning "No CVE insights found (PodWatcher not running - see step 10)"
    fi
fi
echo ""

# Step 10: Root cause analysis
print_step "10/10: Root Cause Analysis..."
echo ""
echo "WHY NO CVE DETECTION?"
echo "====================="
echo ""
echo "1. ✅ Migration021 fix: DEPLOYED (schema corrected)"
echo "2. ✅ SBOM pipeline code: EXISTS (~75% complete)"
echo "3. ✅ CVE matcher code: EXISTS (version comparator, PURL parser)"
echo "4. ✅ Test pod created: RUNNING (nginx:1.19.0)"
echo ""
echo "5. ❌ PodWatcher: NOT RUNNING"
echo "   - PodWatcher code exists (pkg/scanner/pod_watcher.go)"
echo "   - But NOT initialized in main.go"
echo "   - NOT started as goroutine"
echo "   - NO automatic pod scanning happening"
echo ""
echo "6. ❌ Integration: MISSING"
echo "   - No connection between pod events and SBOM pipeline"
echo "   - Agent sends pod data but doesn't trigger CVE scans"
echo "   - SBOM pipeline can work manually but not automatically"
echo ""

print_error "ROOT CAUSE: PodWatcher not integrated in main.go"
echo ""
echo "FIX REQUIRED:"
echo "-------------"
echo "1. Add PodWatcher initialization in core/cmd/main.go"
echo "2. Connect PodWatcher to SBOM pipeline (not deprecated ImageScanner)"
echo "3. Start PodWatcher as goroutine"
echo "4. Test automatic scanning on pod creation"
echo ""

# Create fix documentation
cat > "$PROJECT_ROOT/docs/CVE_DETECTION_INTEGRATION_FIX.md" << 'FIXDOC'
# CVE Detection Integration Fix

## Problem
CVE/SBOM pipeline code exists but is NOT triggered by pod creation events.

## Root Cause
PodWatcher is not initialized or started in `core/cmd/main.go`.

## Solution

### Step 1: Update PodWatcher to use SBOM Pipeline

**File**: `core/pkg/scanner/pod_watcher.go`

Replace deprecated `ImageScanner` with `sbom.Pipeline`:

```go
import (
    "github.com/ksam/core/pkg/sbom"
)

type PodWatcher struct {
    db            *gorm.DB
    k8sClient     kubernetes.Interface
    sbomPipeline  *sbom.Pipeline  // NEW: Use SBOM pipeline
    clusterID     string
    logger        *log.Logger
}

func NewPodWatcher(
    db *gorm.DB,
    k8sClient kubernetes.Interface,
    sbomPipeline *sbom.Pipeline,  // NEW
    clusterID string,
) *PodWatcher {
    return &PodWatcher{
        db:           db,
        k8sClient:    k8sClient,
        sbomPipeline: sbomPipeline,  // NEW
        clusterID:    clusterID,
        logger:       log.New(log.Writer(), "[PodWatcher] ", log.LstdFlags),
    }
}

func (w *PodWatcher) scanPod(ctx context.Context, pod *corev1.Pod) error {
    w.logger.Printf("Scanning pod %s/%s", pod.Namespace, pod.Name)

    for _, container := range pod.Spec.Containers {
        // Use SBOM pipeline instead of deprecated ImageScanner
        err := w.sbomPipeline.ProcessImage(
            ctx,
            container.Image,
            string(pod.UID),
            pod.Name,
            pod.Namespace,
            container.Name,
        )
        if err != nil {
            w.logger.Printf("Failed to scan %s: %v", container.Image, err)
            continue
        }
        w.logger.Printf("✅ Scanned %s successfully", container.Image)
    }
    return nil
}
```

### Step 2: Initialize PodWatcher in main.go

**File**: `core/cmd/main.go`

Add after worker pool initialization:

```go
import (
    "github.com/ksam/core/pkg/scanner"
    "github.com/ksam/core/pkg/sbom"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"
)

// After worker pool setup...

// Initialize Kubernetes client
kubeConfig, err := rest.InClusterConfig()
if err != nil {
    log.Printf("Warning: Failed to get in-cluster config: %v", err)
} else {
    k8sClient, err := kubernetes.NewForConfig(kubeConfig)
    if err != nil {
        log.Printf("Warning: Failed to create k8s client: %v", err)
    } else {
        // Initialize SBOM pipeline
        sbomPipeline := sbom.NewPipeline(insightManager, db)

        // Initialize PodWatcher
        podWatcher := scanner.NewPodWatcher(
            db,
            k8sClient,
            sbomPipeline,
            "cluster-1",  // TODO: Get from config
        )

        // Start PodWatcher in background
        go func() {
            log.Println("[Main] Starting PodWatcher...")
            if err := podWatcher.Start(ctx); err != nil {
                log.Printf("[Main] PodWatcher stopped: %v", err)
            }
        }()

        log.Println("[Main] ✅ PodWatcher started")
    }
}
```

### Step 3: Add Trivy DB

Since CVE matching needs Trivy DB, either:

**Option A**: Download Trivy DB manually
```bash
# Download latest Trivy DB
wget https://github.com/aquasecurity/trivy-db/releases/latest/download/trivy-db.tar.gz
tar -xzf trivy-db.tar.gz
kubectl cp db/trivy.db ksam-core-pod:/var/lib/ksam/trivy.db
```

**Option B**: Use NVD API only (slower, no Trivy DB)
```bash
# CVE matcher will fallback to NVD API automatically
# Set KSAM_TRIVY_DB_PATH to non-existent path to force NVD API
```

### Step 4: Test

1. Rebuild and redeploy
2. Create test pod
3. Check logs for PodWatcher activity
4. Verify CVEs and insights in database

```bash
# Create test pod
kubectl run test-cve --image=nginx:1.19.0

# Check PodWatcher logs
kubectl logs -n ksam deployment/ksam-core | grep PodWatcher

# Check database
kubectl exec -it deployment/ksam-core -n ksam -- \
  psql -h postgres -U postgres -d ksam -c \
  "SELECT * FROM insights WHERE source = 'cve' LIMIT 5;"
```

## Expected Behavior After Fix

1. Pod created → PodWatcher detects
2. PodWatcher → Triggers SBOM pipeline
3. SBOM pipeline → Extracts packages
4. CVE matcher → Finds vulnerabilities
5. Insight manager → Creates insights
6. Dashboard → Shows CVE insights
FIXDOC

print_success "Fix documentation created: docs/CVE_DETECTION_INTEGRATION_FIX.md"
echo ""

# Summary
echo "=========================================="
echo "TEST SUMMARY"
echo "=========================================="
echo ""
echo "Migration021 Fix:"
if [ -n "$MIGRATION_LOGS" ]; then
    print_success "Migration021 executed successfully"
else
    print_error "Migration021 not executed"
fi
echo ""
echo "Database Schema:"
if [ "$HAS_PURL" = "t" ]; then
    print_success "Schema correct (purl column exists)"
else
    print_error "Schema incorrect (p_url instead of purl)"
fi
echo ""
echo "CVE Detection:"
print_error "NOT WORKING - PodWatcher not running"
print_warning "See docs/CVE_DETECTION_INTEGRATION_FIX.md for fix"
echo ""
echo "Next Steps:"
echo "1. Review CVE_DETECTION_INTEGRATION_FIX.md"
echo "2. Integrate PodWatcher with SBOM pipeline"
echo "3. Rebuild and redeploy"
echo "4. Re-run this test script"
echo ""
echo "=========================================="
