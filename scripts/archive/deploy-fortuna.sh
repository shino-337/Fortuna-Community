#!/bin/bash

# ============================================================================
# Deploy Fortuna to Existing Kubernetes Cluster
# ============================================================================
# For environments with:
#   - Kubernetes cluster already set up
#   - containerd runtime
#   - Go 1.24+ installed
# ============================================================================
# Builds Docker images and deploys Fortuna components
# ============================================================================

set -euo pipefail

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# Configuration
IMAGE_TAG="${IMAGE_TAG:-latest}"
REGISTRY="${REGISTRY:-}"  # Optional: docker.io/username or registry.example.com
CORE_IMAGE="${REGISTRY:+${REGISTRY}/}fortuna-core:${IMAGE_TAG}"
AGENT_IMAGE="${REGISTRY:+${REGISTRY}/}fortuna-agent:${IMAGE_TAG}"
NAMESPACE="${NAMESPACE:-fortuna}"
SKIP_BUILD="${SKIP_BUILD:-false}"
SKIP_INFRA="${SKIP_INFRA:-false}"
SKIP_DEPLOY="${SKIP_DEPLOY:-false}"

# Logging
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} ✅ $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} ❌ $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} ⚠️  $1"
}

log_step() {
    echo -e "${CYAN}[STEP]${NC} $1"
}

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

# Check prerequisites
check_prerequisites() {
    log_step "Checking prerequisites..."
    
    local missing=0
    
    # Check Go
    if ! command -v go >/dev/null 2>&1; then
        log_error "Go is not installed"
        missing=1
    else
        local go_version=$(go version | awk '{print $3}' | sed 's/go//')
        log_success "Go is installed: $go_version"
        
        # Check Go version (should be 1.24+)
        local major=$(echo "$go_version" | cut -d. -f1)
        local minor=$(echo "$go_version" | cut -d. -f2)
        if [ "$major" -lt 1 ] || ([ "$major" -eq 1 ] && [ "$minor" -lt 24 ]); then
            log_warning "Go version $go_version may not be compatible (recommended: 1.24+)"
        fi
    fi
    
    # Check Docker (for building images)
    if ! command -v docker >/dev/null 2>&1; then
        log_error "docker is not installed (required for building images)"
        missing=1
    else
        log_success "docker is installed"
    fi
    
    # Check kubectl
    if ! command -v kubectl >/dev/null 2>&1; then
        log_error "kubectl is not installed"
        missing=1
    else
        log_success "kubectl is installed"
    fi
    
    # Check Kubernetes cluster
    if ! kubectl cluster-info &>/dev/null 2>&1; then
        log_error "Kubernetes cluster is not accessible"
        log_info "Make sure kubectl is configured correctly (check ~/.kube/config)"
        missing=1
    else
        log_success "Kubernetes cluster is accessible"
        local cluster_info=$(kubectl cluster-info | head -1)
        log_info "  $cluster_info"
    fi
    
    # Check containerd (optional check, just for info)
    if command -v crictl >/dev/null 2>&1; then
        log_info "crictl is available (containerd runtime detected)"
    fi
    
    if [ $missing -eq 1 ]; then
        log_error "Please install missing prerequisites"
        exit 1
    fi
}

# Check cluster status
check_cluster() {
    log_step "Checking cluster status..."
    
    local nodes=$(kubectl get nodes --no-headers 2>/dev/null | wc -l)
    if [ "$nodes" -lt 1 ]; then
        log_error "No nodes found in cluster"
        exit 1
    fi
    
    log_success "Cluster has $nodes node(s)"
    echo ""
    kubectl get nodes -o wide
    echo ""
    
    # Check container runtime
    log_info "Container runtime info:"
    local runtime=$(kubectl get node -o jsonpath='{.items[0].status.nodeInfo.containerRuntimeVersion}' 2>/dev/null || echo "unknown")
    log_info "  Runtime: $runtime"
    
    if echo "$runtime" | grep -qi "containerd"; then
        log_success "containerd runtime detected"
    else
        log_warning "containerd not detected, but continuing..."
    fi
    
    # Check if namespace exists
    if kubectl get namespace "$NAMESPACE" &>/dev/null 2>&1; then
        log_info "Namespace '$NAMESPACE' already exists"
    else
        log_info "Namespace '$NAMESPACE' will be created"
    fi
}

# Build Core image
build_core() {
    if [ "$SKIP_BUILD" = "true" ]; then
        log_step "Skipping Core build (--skip-build)"
        return
    fi
    
    log_step "Building Core Docker image..."
    
    # Check if Dockerfile exists
    if [ ! -f "$PROJECT_ROOT/core/Dockerfile" ]; then
        log_error "Dockerfile not found in core/"
        exit 1
    fi
    
    # Check if api module exists (required for build)
    if [ ! -d "$PROJECT_ROOT/api" ]; then
        log_warning "api/ directory not found, build may fail"
    fi
    
    # Check if go.work exists (for workspace support)
    if [ ! -f "$PROJECT_ROOT/go.work" ]; then
        log_warning "go.work not found, workspace build may not work correctly"
    fi
    
    local build_args=(
        --build-arg FORTUNA_BUILD_VERSION="${IMAGE_TAG}"
        --build-arg FORTUNA_BUILD_COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo 'dev')"
        --build-arg FORTUNA_BUILD_TIME="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
    )
    
    log_info "Building from repo root with context: $PROJECT_ROOT"
    log_info "Dockerfile: core/Dockerfile"
    log_info "Build args: ${build_args[*]}"
    
    # Build from repo root with Dockerfile path
    cd "$PROJECT_ROOT"
    if docker build "${build_args[@]}" -f core/Dockerfile -t "${CORE_IMAGE}" .; then
        log_success "Core image built: ${CORE_IMAGE}"
        
        # Show image size
        local size=$(docker images "${CORE_IMAGE}" --format "{{.Size}}" | head -1)
        log_info "  Image size: $size"
    else
        log_error "Failed to build Core image"
        exit 1
    fi
}

# Build Agent image
build_agent() {
    if [ "$SKIP_BUILD" = "true" ]; then
        log_step "Skipping Agent build (--skip-build)"
        return
    fi
    
    log_step "Building Agent Docker image..."
    
    # Check if Dockerfile exists
    if [ ! -f "$PROJECT_ROOT/agent/Dockerfile" ]; then
        log_error "Dockerfile not found in agent/"
        exit 1
    fi
    
    # Check if api module exists (required for build)
    if [ ! -d "$PROJECT_ROOT/api" ]; then
        log_warning "api/ directory not found, build may fail"
    fi
    
    # Check if go.work exists (for workspace support)
    if [ ! -f "$PROJECT_ROOT/go.work" ]; then
        log_warning "go.work not found, workspace build may not work correctly"
    fi
    
    local build_args=(
        --build-arg FORTUNA_BUILD_VERSION="${IMAGE_TAG}"
        --build-arg FORTUNA_BUILD_COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo 'dev')"
        --build-arg FORTUNA_BUILD_TIME="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
    )
    
    log_info "Building from repo root with context: $PROJECT_ROOT"
    log_info "Dockerfile: agent/Dockerfile"
    log_info "Build args: ${build_args[*]}"
    
    # Build from repo root with Dockerfile path
    cd "$PROJECT_ROOT"
    if docker build "${build_args[@]}" -f agent/Dockerfile -t "${AGENT_IMAGE}" .; then
        log_success "Agent image built: ${AGENT_IMAGE}"
        
        # Show image size
        local size=$(docker images "${AGENT_IMAGE}" --format "{{.Size}}" | head -1)
        log_info "  Image size: $size"
    else
        log_error "Failed to build Agent image"
        exit 1
    fi
}

# Push images to registry
push_images() {
    if [ "$SKIP_BUILD" = "true" ]; then
        return
    fi
    
    if [ -z "$REGISTRY" ]; then
        log_info "No registry specified, images available locally only"
        log_info "To use on remote nodes:"
        log_info "  1. Set REGISTRY environment variable and push"
        log_info "  2. Save images: docker save ${CORE_IMAGE} -o fortuna-core.tar"
        log_info "  3. Load on nodes: docker load -i fortuna-core.tar"
        return
    fi
    
    log_step "Pushing images to registry..."
    
    log_info "Pushing Core image..."
    if docker push "${CORE_IMAGE}"; then
        log_success "Core image pushed: ${CORE_IMAGE}"
    else
        log_error "Failed to push Core image"
        exit 1
    fi
    
    log_info "Pushing Agent image..."
    if docker push "${AGENT_IMAGE}"; then
        log_success "Agent image pushed: ${AGENT_IMAGE}"
    else
        log_error "Failed to push Agent image"
        exit 1
    fi
}

# Create namespace
create_namespace() {
    log_step "Creating namespace..."
    
    if kubectl get namespace "$NAMESPACE" &>/dev/null 2>&1; then
        log_info "Namespace '$NAMESPACE' already exists"
    else
        kubectl create namespace "$NAMESPACE"
        log_success "Namespace '$NAMESPACE' created"
    fi
}

# Generate certificates
generate_certificates() {
    log_step "Generating mTLS certificates..."
    
    local cert_dir="$PROJECT_ROOT/.certs"
    mkdir -p "$cert_dir"
    
    if [ -f "$cert_dir/ca.crt" ] && [ -f "$cert_dir/server.crt" ] && [ -f "$cert_dir/client.crt" ]; then
        log_info "Certificates already exist, skipping generation"
    else
        log_info "Generating new certificates..."
        
        # Check if openssl is available
        if ! command -v openssl >/dev/null 2>&1; then
            log_error "openssl is not installed (required for certificate generation)"
            exit 1
        fi
        
        # Generate CA
        openssl genrsa -out "$cert_dir/ca.key" 4096 2>/dev/null
        openssl req -new -x509 -days 365 -key "$cert_dir/ca.key" -out "$cert_dir/ca.crt" \
            -subj "/CN=Fortuna CA" 2>/dev/null
        
        # Generate server certificate
        openssl genrsa -out "$cert_dir/server.key" 4096 2>/dev/null
        openssl req -new -key "$cert_dir/server.key" -out "$cert_dir/server.csr" \
            -subj "/CN=fortuna-core.${NAMESPACE}.svc.cluster.local" 2>/dev/null
        openssl x509 -req -days 365 -in "$cert_dir/server.csr" -CA "$cert_dir/ca.crt" \
            -CAkey "$cert_dir/ca.key" -CAcreateserial -out "$cert_dir/server.crt" 2>/dev/null
        
        # Generate client certificate
        openssl genrsa -out "$cert_dir/client.key" 4096 2>/dev/null
        openssl req -new -key "$cert_dir/client.key" -out "$cert_dir/client.csr" \
            -subj "/CN=fortuna-agent" 2>/dev/null
        openssl x509 -req -days 365 -in "$cert_dir/client.csr" -CA "$cert_dir/ca.crt" \
            -CAkey "$cert_dir/ca.key" -CAcreateserial -out "$cert_dir/client.crt" 2>/dev/null
        
        log_success "Certificates generated"
    fi
    
    # Create Kubernetes secrets
    log_info "Creating Kubernetes secrets..."
    kubectl create secret generic fortuna-core-server-tls \
        --from-file=tls.crt="$cert_dir/server.crt" \
        --from-file=tls.key="$cert_dir/server.key" \
        --from-file=ca.crt="$cert_dir/ca.crt" \
        -n "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f - 2>/dev/null || true
    
    kubectl create secret generic fortuna-agent-client-tls \
        --from-file=tls.crt="$cert_dir/client.crt" \
        --from-file=tls.key="$cert_dir/client.key" \
        --from-file=ca.crt="$cert_dir/ca.crt" \
        -n "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f - 2>/dev/null || true
    
    log_success "Secrets created/updated"
}

# Update deployment files with image references
update_deployments() {
    log_step "Updating deployment files with image references..."
    
    local deploy_dir="$PROJECT_ROOT/deploy"
    
    # Backup original files
    if [ -f "$deploy_dir/fortuna-core-deployment.yaml" ]; then
        cp "$deploy_dir/fortuna-core-deployment.yaml" "$deploy_dir/fortuna-core-deployment.yaml.bak" 2>/dev/null || true
    fi
    if [ -f "$deploy_dir/fortuna-agent-daemonset.yaml" ]; then
        cp "$deploy_dir/fortuna-agent-daemonset.yaml" "$deploy_dir/fortuna-agent-daemonset.yaml.bak" 2>/dev/null || true
    fi
    
    # Update Core deployment
    if [ -f "$deploy_dir/fortuna-core-deployment.yaml" ]; then
        if [ -n "$REGISTRY" ]; then
            # Update image reference with registry
            sed -i.tmp "s|image:.*fortuna-core:.*|image: ${CORE_IMAGE}|g" "$deploy_dir/fortuna-core-deployment.yaml"
            sed -i.tmp "s|imagePullPolicy:.*|imagePullPolicy: IfNotPresent|g" "$deploy_dir/fortuna-core-deployment.yaml"
            rm -f "$deploy_dir/fortuna-core-deployment.yaml.tmp" 2>/dev/null || true
            log_info "Updated Core deployment: image=${CORE_IMAGE}"
        else
            # Use local image
            sed -i.tmp "s|image:.*fortuna-core:.*|image: ${CORE_IMAGE}|g" "$deploy_dir/fortuna-core-deployment.yaml"
            sed -i.tmp "s|imagePullPolicy:.*|imagePullPolicy: IfNotPresent|g" "$deploy_dir/fortuna-core-deployment.yaml"
            rm -f "$deploy_dir/fortuna-core-deployment.yaml.tmp" 2>/dev/null || true
            log_info "Updated Core deployment: image=${CORE_IMAGE} (local)"
        fi
    fi
    
    # Update Agent deployment
    if [ -f "$deploy_dir/fortuna-agent-daemonset.yaml" ]; then
        if [ -n "$REGISTRY" ]; then
            # Update image reference with registry
            sed -i.tmp "s|image:.*fortuna-agent:.*|image: ${AGENT_IMAGE}|g" "$deploy_dir/fortuna-agent-daemonset.yaml"
            sed -i.tmp "s|imagePullPolicy:.*|imagePullPolicy: IfNotPresent|g" "$deploy_dir/fortuna-agent-daemonset.yaml"
            rm -f "$deploy_dir/fortuna-agent-daemonset.yaml.tmp" 2>/dev/null || true
            log_info "Updated Agent deployment: image=${AGENT_IMAGE}"
        else
            # Use local image
            sed -i.tmp "s|image:.*fortuna-agent:.*|image: ${AGENT_IMAGE}|g" "$deploy_dir/fortuna-agent-daemonset.yaml"
            sed -i.tmp "s|imagePullPolicy:.*|imagePullPolicy: IfNotPresent|g" "$deploy_dir/fortuna-agent-daemonset.yaml"
            rm -f "$deploy_dir/fortuna-agent-daemonset.yaml.tmp" 2>/dev/null || true
            log_info "Updated Agent deployment: image=${AGENT_IMAGE} (local)"
        fi
    fi
    
    log_success "Deployment files updated"
}

# Deploy infrastructure
deploy_infrastructure() {
    if [ "$SKIP_INFRA" = "true" ]; then
        log_step "Skipping infrastructure deployment (--skip-infra)"
        return
    fi
    
    log_step "Deploying infrastructure..."
    
    local deploy_dir="$PROJECT_ROOT/deploy"
    
    # Deploy PostgreSQL
    if [ -f "$deploy_dir/infrastructure/postgresql-with-age.yaml" ]; then
        log_info "Deploying PostgreSQL with Apache AGE..."
        kubectl apply -f "$deploy_dir/infrastructure/postgresql-with-age.yaml"
        
        log_info "Waiting for PostgreSQL to be ready (timeout: 5 minutes)..."
        if kubectl wait --for=condition=ready pod -l app=postgres -n "$NAMESPACE" --timeout=300s 2>/dev/null; then
            log_success "PostgreSQL is ready"
        else
            log_warning "PostgreSQL not ready yet (may take a few minutes)"
            log_info "Check status: kubectl get pods -n $NAMESPACE -l app=postgres"
        fi
    else
        log_warning "PostgreSQL deployment file not found: $deploy_dir/infrastructure/postgresql-with-age.yaml"
    fi
    
    # Deploy NATS
    if [ -f "$deploy_dir/infrastructure/nats.yaml" ]; then
        log_info "Deploying NATS JetStream..."
        kubectl apply -f "$deploy_dir/infrastructure/nats.yaml"
        
        log_info "Waiting for NATS to be ready (timeout: 5 minutes)..."
        if kubectl wait --for=condition=ready pod -l app=nats -n "$NAMESPACE" --timeout=300s 2>/dev/null; then
            log_success "NATS is ready"
        else
            log_warning "NATS not ready yet (may take a few minutes)"
            log_info "Check status: kubectl get pods -n $NAMESPACE -l app=nats"
        fi
    else
        log_warning "NATS deployment file not found: $deploy_dir/infrastructure/nats.yaml"
    fi
}

# Deploy Fortuna
deploy_fortuna() {
    if [ "$SKIP_DEPLOY" = "true" ]; then
        log_step "Skipping Fortuna deployment (--skip-deploy)"
        return
    fi
    
    log_step "Deploying Fortuna components..."
    
    local deploy_dir="$PROJECT_ROOT/deploy"
    
    # Deploy RBAC
    if [ -f "$deploy_dir/fortuna-rbac.yaml" ]; then
        log_info "Deploying RBAC..."
        kubectl apply -f "$deploy_dir/fortuna-rbac.yaml"
        log_success "RBAC deployed"
    else
        log_warning "RBAC file not found: $deploy_dir/fortuna-rbac.yaml"
    fi
    
    # Deploy Core
    if [ -f "$deploy_dir/fortuna-core-deployment.yaml" ]; then
        log_info "Deploying Core..."
        kubectl apply -f "$deploy_dir/fortuna-core-deployment.yaml"
        
        log_info "Waiting for Core to be ready (timeout: 5 minutes)..."
        if kubectl wait --for=condition=ready pod -l app.kubernetes.io/component=core -n "$NAMESPACE" --timeout=300s 2>/dev/null; then
            log_success "Core is ready"
        else
            log_warning "Core not ready yet (check logs)"
            log_info "Check status: kubectl get pods -n $NAMESPACE -l app.kubernetes.io/component=core"
            log_info "Check logs: kubectl logs -n $NAMESPACE -l app.kubernetes.io/component=core --tail=50"
        fi
    else
        log_warning "Core deployment file not found: $deploy_dir/fortuna-core-deployment.yaml"
    fi
    
    # Deploy Agent
    if [ -f "$deploy_dir/fortuna-agent-daemonset.yaml" ]; then
        log_info "Deploying Agent (DaemonSet)..."
        kubectl apply -f "$deploy_dir/fortuna-agent-daemonset.yaml"
        
        log_info "Waiting for Agent pods (one per worker node)..."
        sleep 5
        local agent_pods=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=agent --no-headers 2>/dev/null | wc -l)
        local ready_pods=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=agent --no-headers 2>/dev/null | grep -c "Running" || echo "0")
        
        if [ "$agent_pods" -gt 0 ]; then
            log_success "Agent deployed: $ready_pods/$agent_pods pod(s) running"
            if [ "$ready_pods" -lt "$agent_pods" ]; then
                log_info "Some Agent pods may still be starting..."
            fi
        else
            log_warning "No Agent pods found"
        fi
    else
        log_warning "Agent deployment file not found: $deploy_dir/fortuna-agent-daemonset.yaml"
    fi
}

# Show status
show_status() {
    log_step "Deployment status:"
    echo ""
    
    echo "=== Nodes ==="
    kubectl get nodes -o wide
    echo ""
    
    echo "=== Namespace: $NAMESPACE ==="
    kubectl get pods -n "$NAMESPACE" -o wide
    echo ""
    
    echo "=== Services ==="
    kubectl get svc -n "$NAMESPACE" 2>/dev/null || echo "No services found"
    echo ""
    
    echo "=== Deployments ==="
    kubectl get deployments -n "$NAMESPACE" 2>/dev/null || echo "No deployments found"
    echo ""
    
    echo "=== DaemonSets ==="
    kubectl get daemonsets -n "$NAMESPACE" 2>/dev/null || echo "No daemonsets found"
    echo ""
    
    echo "=== PersistentVolumeClaims ==="
    kubectl get pvc -n "$NAMESPACE" 2>/dev/null || echo "No PVCs found"
    echo ""
}

# Main execution
main() {
    echo ""
    echo "=========================================="
    echo "Fortuna Deployment Script"
    echo "=========================================="
    echo ""
    echo "Configuration:"
    echo "  Image Tag: ${IMAGE_TAG}"
    echo "  Registry: ${REGISTRY:-<not set - using local images>}"
    echo "  Namespace: ${NAMESPACE}"
    echo "  Skip Build: ${SKIP_BUILD}"
    echo "  Skip Infrastructure: ${SKIP_INFRA}"
    echo "  Skip Deploy: ${SKIP_DEPLOY}"
    echo ""
    
    check_prerequisites
    check_cluster
    
    if [ "$SKIP_BUILD" != "true" ]; then
        build_core
        build_agent
        push_images
    fi
    
    create_namespace
    generate_certificates
    update_deployments
    
    if [ "$SKIP_INFRA" != "true" ]; then
        deploy_infrastructure
    fi
    
    if [ "$SKIP_DEPLOY" != "true" ]; then
        deploy_fortuna
    fi
    
    show_status
    
    echo ""
    log_success "Deployment completed!"
    echo ""
    log_info "Next steps:"
    echo "  1. Check pod status:"
    echo "     kubectl get pods -n ${NAMESPACE}"
    echo ""
    echo "  2. Check Core logs:"
    echo "     kubectl logs -n ${NAMESPACE} -l app.kubernetes.io/component=core --tail=50 -f"
    echo ""
    echo "  3. Check Agent logs:"
    echo "     kubectl logs -n ${NAMESPACE} -l app.kubernetes.io/component=agent --tail=50 -f"
    echo ""
    echo "  4. Test API:"
    echo "     kubectl port-forward -n ${NAMESPACE} svc/fortuna-core 8080:8080 &"
    echo "     curl http://localhost:8080/health"
    echo ""
    echo "  5. Check database:"
    echo "     POSTGRES_POD=\$(kubectl get pods -n ${NAMESPACE} -l app=postgres -o jsonpath='{.items[0].metadata.name}')"
    echo "     kubectl exec -n ${NAMESPACE} \$POSTGRES_POD -- psql -U postgres -d ksam -c '\\dt'"
    echo ""
    
    if [ -z "$REGISTRY" ] && [ "$SKIP_BUILD" != "true" ]; then
        log_warning "Images are only available locally"
        log_info "To use on remote nodes:"
        log_info "  1. Set REGISTRY and push: REGISTRY=your-registry ./scripts/deploy-fortuna.sh"
        log_info "  2. Or save/load manually:"
        log_info "     docker save ${CORE_IMAGE} -o fortuna-core.tar"
        log_info "     docker save ${AGENT_IMAGE} -o fortuna-agent.tar"
        log_info "     # Then copy to each node and: docker load -i fortuna-core.tar"
        echo ""
    fi
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --skip-build)
            SKIP_BUILD="true"
            shift
            ;;
        --skip-infra)
            SKIP_INFRA="true"
            shift
            ;;
        --skip-deploy)
            SKIP_DEPLOY="true"
            shift
            ;;
        --registry=*)
            REGISTRY="${1#*=}"
            shift
            ;;
        --tag=*)
            IMAGE_TAG="${1#*=}"
            shift
            ;;
        --namespace=*)
            NAMESPACE="${1#*=}"
            shift
            ;;
        -h|--help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --skip-build          Skip building Docker images"
            echo "  --skip-infra          Skip deploying infrastructure (PostgreSQL, NATS)"
            echo "  --skip-deploy         Skip deploying Fortuna components"
            echo "  --registry=REGISTRY   Docker registry (e.g., docker.io/username)"
            echo "  --tag=TAG            Image tag (default: latest)"
            echo "  --namespace=NS       Kubernetes namespace (default: fortuna)"
            echo "  -h, --help           Show this help message"
            echo ""
            echo "Environment variables:"
            echo "  REGISTRY             Docker registry"
            echo "  IMAGE_TAG            Image tag (default: latest)"
            echo "  NAMESPACE            Kubernetes namespace (default: fortuna)"
            echo "  SKIP_BUILD           Skip build (true/false)"
            echo "  SKIP_INFRA           Skip infrastructure (true/false)"
            echo "  SKIP_DEPLOY          Skip deploy (true/false)"
            echo ""
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Run main
main "$@"


