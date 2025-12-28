#!/bin/bash

# Complete Deployment Script - All Steps
# This script performs complete deployment from scratch including CVE database loading

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
NAMESPACE="${NAMESPACE:-fortuna}"
PROJECT_ROOT="/Users/tuatnh/Desktop/Learn/K8s Service Account Management Platform/KSAM"

# Functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Step 1: Cleanup
step1_cleanup() {
    log_info "Step 1: Cleaning up existing deployment..."
    kubectl delete deployment -n $NAMESPACE fortuna-core 2>/dev/null || true
    kubectl delete daemonset -n $NAMESPACE fortuna-agent 2>/dev/null || true
    kubectl delete service -n $NAMESPACE fortuna-core fortuna-agent 2>/dev/null || true
    kubectl delete pod -n $NAMESPACE -l app=test-pod-e2e 2>/dev/null || true
    kubectl delete job -n $NAMESPACE cve-loader 2>/dev/null || true
    log_success "Cleanup completed"
}

# Step 2: Setup Minikube
step2_setup_minikube() {
    log_info "Step 2: Setting up Minikube..."
    minikube status || minikube start
    eval $(minikube docker-env)
    kubectl create namespace $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -
    log_success "Minikube ready"
}

# Step 3: Deploy Infrastructure
step3_deploy_infrastructure() {
    log_info "Step 3: Deploying infrastructure..."
    kubectl apply -f $PROJECT_ROOT/deploy/infrastructure/postgres.yaml
    kubectl apply -f $PROJECT_ROOT/deploy/infrastructure/nats.yaml
    
    log_info "Waiting for PostgreSQL..."
    kubectl wait --for=condition=ready pod -n $NAMESPACE -l app=postgres --timeout=300s || {
        log_error "PostgreSQL failed to start"
        exit 1
    }
    
    log_info "Waiting for NATS..."
    kubectl wait --for=condition=ready pod -n $NAMESPACE -l app=nats --timeout=300s || {
        log_error "NATS failed to start"
        exit 1
    }
    
    log_success "Infrastructure deployed"
}

# Step 4: Build Images
step4_build_images() {
    log_info "Step 4: Building Docker images..."
    cd "$PROJECT_ROOT"
    eval $(minikube docker-env)
    
    log_info "Building Core image..."
    docker build -f core/Dockerfile -t fortuna-core:latest . || {
        log_error "Failed to build Core image"
        exit 1
    }
    
    log_info "Building Agent image..."
    docker build -f agent/Dockerfile -t fortuna-agent:latest . || {
        log_error "Failed to build Agent image"
        exit 1
    }
    
    log_success "Images built"
}

# Step 5: Deploy Core
step5_deploy_core() {
    log_info "Step 5: Deploying Core..."
    kubectl apply -f $PROJECT_ROOT/deploy/fortuna-core-deployment.yaml
    
    log_info "Waiting for Core..."
    kubectl wait --for=condition=ready pod -n $NAMESPACE -l 'app.kubernetes.io/name=fortuna,app.kubernetes.io/component=core' --timeout=300s || {
        log_error "Core failed to start"
        kubectl logs -n $NAMESPACE -l 'app.kubernetes.io/component=core' --tail=50
        exit 1
    }
    
    log_success "Core deployed"
}

# Step 6: Deploy Agent
step6_deploy_agent() {
    log_info "Step 6: Deploying Agent..."
    kubectl apply -f $PROJECT_ROOT/deploy/fortuna-agent-daemonset.yaml
    
    log_info "Waiting for Agent..."
    kubectl wait --for=condition=ready pod -n $NAMESPACE -l 'app.kubernetes.io/name=fortuna,app.kubernetes.io/component=agent' --timeout=300s || {
        log_error "Agent failed to start"
        kubectl logs -n $NAMESPACE -l 'app.kubernetes.io/component=agent' --tail=50
        exit 1
    }
    
    log_success "Agent deployed"
}

# Step 7: Load CVE Database
step7_load_cve_database() {
    log_info "Step 7: Loading CVE database..."
    
    # Check CVE data
    CVE_FILE_COUNT=$(find "$PROJECT_ROOT/cve-data/all" -name "*.json" 2>/dev/null | wc -l | tr -d ' ')
    if [ "$CVE_FILE_COUNT" -eq 0 ]; then
        log_error "No CVE data files found in $PROJECT_ROOT/cve-data/all/"
        exit 1
    fi
    log_info "Found $CVE_FILE_COUNT CVE files"
    
    CORE_POD=$(kubectl get pods -n $NAMESPACE -l 'app.kubernetes.io/component=core' -o jsonpath='{.items[0].metadata.name}')
    
    # Check if cve-loader exists
    if ! kubectl exec -n $NAMESPACE $CORE_POD -- ls /app/cve-loader >/dev/null 2>&1; then
        log_error "CVE loader binary not found in Core image"
        log_info "Rebuilding Core image with CVE loader..."
        step4_build_images
        step5_deploy_core
        CORE_POD=$(kubectl get pods -n $NAMESPACE -l 'app.kubernetes.io/component=core' -o jsonpath='{.items[0].metadata.name}')
    fi
    
    # Copy CVE data
    log_info "Copying CVE data to Core pod (this may take several minutes)..."
    kubectl cp "$PROJECT_ROOT/cve-data/all" $NAMESPACE/$CORE_POD:/cve-data/all || {
        log_warning "kubectl cp failed, trying alternative method with Job..."
        step7_load_cve_via_job
        return
    }
    
    # Run CVE loader
    log_info "Running CVE loader (this will take 10-30 minutes)..."
    kubectl exec -n $NAMESPACE $CORE_POD -- /app/cve-loader \
        --source /cve-data/all \
        --workers 20 \
        --batch-size 100 \
        --skip-errors true || {
        log_error "CVE loader failed"
        kubectl logs -n $NAMESPACE $CORE_POD --tail=100 | grep -i "cve\|loader" || true
        exit 1
    }
    
    log_success "CVE database loaded"
}

# Step 7 Alternative: Load CVE via Job
step7_load_cve_via_job() {
    log_info "Using Job method to load CVE data..."
    
    cat > /tmp/cve-loader-job.yaml << EOF
apiVersion: batch/v1
kind: Job
metadata:
  name: cve-loader
  namespace: $NAMESPACE
spec:
  template:
    spec:
      containers:
      - name: cve-loader
        image: fortuna-core:latest
        command: ["/app/cve-loader"]
        args:
        - "--source"
        - "/cve-data/all"
        - "--workers"
        - "20"
        - "--batch-size"
        - "100"
        - "--skip-errors"
        - "true"
        env:
        - name: DATABASE_URL
          value: "postgres://postgres:postgres@postgres.$NAMESPACE.svc.cluster.local:5432/ksam?sslmode=disable"
        volumeMounts:
        - name: cve-data
          mountPath: /cve-data
      volumes:
      - name: cve-data
        hostPath:
          path: $PROJECT_ROOT/cve-data
          type: Directory
      restartPolicy: Never
  backoffLimit: 3
EOF
    
    kubectl apply -f /tmp/cve-loader-job.yaml
    
    log_info "Waiting for CVE loader job to complete (this may take 10-30 minutes)..."
    kubectl wait --for=condition=complete job/cve-loader -n $NAMESPACE --timeout=3600s || {
        log_error "CVE loader job failed or timed out"
        kubectl logs -n $NAMESPACE -l job-name=cve-loader --tail=100
        exit 1
    }
    
    log_success "CVE database loaded via Job"
}

# Step 8: Verify CVE Database
step8_verify_cve_database() {
    log_info "Step 8: Verifying CVE database..."
    
    POSTGRES_POD=$(kubectl get pods -n $NAMESPACE -l app=postgres -o jsonpath='{.items[0].metadata.name}')
    
    CVE_COUNT=$(kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -t -A -c "SELECT COUNT(*) FROM cves;" 2>/dev/null | tr -d ' ')
    PKG_VULN_COUNT=$(kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -t -A -c "SELECT COUNT(*) FROM package_vulnerabilities;" 2>/dev/null | tr -d ' ')
    
    log_info "CVE count: $CVE_COUNT"
    log_info "Package vulnerabilities count: $PKG_VULN_COUNT"
    
    if [ "$CVE_COUNT" -gt 0 ] && [ "$PKG_VULN_COUNT" -gt 0 ]; then
        log_success "CVE database populated successfully"
    else
        log_warning "CVE database appears empty or incomplete"
    fi
}

# Step 9: Verify Deployment
step9_verify_deployment() {
    log_info "Step 9: Verifying deployment..."
    
    echo ""
    echo "=== Pod Status ==="
    kubectl get pods -n $NAMESPACE
    
    echo ""
    echo "=== Core Health ==="
    CORE_POD=$(kubectl get pods -n $NAMESPACE -l 'app.kubernetes.io/component=core' -o jsonpath='{.items[0].metadata.name}')
    kubectl port-forward -n $NAMESPACE $CORE_POD 8080:8080 > /tmp/port-forward-verify.log 2>&1 &
    PORT_FORWARD_PID=$!
    sleep 3
    
    HEALTH_RESPONSE=$(curl -s http://localhost:8080/health 2>/dev/null || echo "{}")
    if echo "$HEALTH_RESPONSE" | grep -q "healthy"; then
        log_success "Core health check passed"
    else
        log_warning "Core health check failed or incomplete"
    fi
    
    kill $PORT_FORWARD_PID 2>/dev/null || true
    
    log_success "Deployment verified"
}

# Main
main() {
    echo ""
    echo "=========================================="
    echo "KSAM Complete Deployment"
    echo "=========================================="
    echo ""
    
    step1_cleanup
    step2_setup_minikube
    step3_deploy_infrastructure
    step4_build_images
    step5_deploy_core
    step6_deploy_agent
    step7_load_cve_database
    step8_verify_cve_database
    step9_verify_deployment
    
    echo ""
    echo "=========================================="
    log_success "Deployment Complete!"
    echo "=========================================="
    echo ""
    echo "Next steps:"
    echo "1. Run E2E test: cd $PROJECT_ROOT/tests/e2e/scripts && ./verify-e2e-flow.sh"
    echo "2. Monitor logs: kubectl logs -n $NAMESPACE -l 'app.kubernetes.io/component=core' -f"
    echo "3. Check API: kubectl port-forward -n $NAMESPACE <core-pod> 8080:8080"
    echo ""
}

main "$@"

