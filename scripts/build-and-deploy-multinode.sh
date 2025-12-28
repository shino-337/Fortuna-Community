#!/bin/bash

# ============================================================================
# Build and Deploy Fortuna to Multi-Node Kubernetes Cluster
# ============================================================================
# Builds Docker images and deploys to production K8s cluster
# Supports containerd runtime on master-worker nodes
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
REGISTRY="${REGISTRY:-}"  # Set to your registry, e.g., docker.io/username or registry.example.com
CORE_IMAGE="${REGISTRY:+${REGISTRY}/}fortuna-core:${IMAGE_TAG}"
AGENT_IMAGE="${REGISTRY:+${REGISTRY}/}fortuna-agent:${IMAGE_TAG}"
NAMESPACE="${NAMESPACE:-fortuna}"

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
    
    if ! command -v docker >/dev/null 2>&1; then
        log_error "docker is not installed"
        missing=1
    else
        log_success "docker is installed"
    fi
    
    if ! command -v kubectl >/dev/null 2>&1; then
        log_error "kubectl is not installed"
        missing=1
    else
        log_success "kubectl is installed"
    fi
    
    if ! kubectl cluster-info &>/dev/null 2>&1; then
        log_error "Kubernetes cluster is not accessible"
        log_info "Make sure kubectl is configured correctly"
        missing=1
    else
        log_success "Kubernetes cluster is accessible"
        kubectl cluster-info | head -1
    fi
    
    if [ $missing -eq 1 ]; then
        log_error "Please install missing prerequisites"
        exit 1
    fi
}

# Check cluster nodes
check_cluster() {
    log_step "Checking cluster status..."
    
    local nodes=$(kubectl get nodes --no-headers 2>/dev/null | wc -l)
    if [ "$nodes" -lt 1 ]; then
        log_error "No nodes found in cluster"
        exit 1
    fi
    
    log_success "Cluster has $nodes node(s)"
    kubectl get nodes
    
    # Check containerd
    log_info "Checking container runtime..."
    local runtime=$(kubectl get node -o jsonpath='{.items[0].status.nodeInfo.containerRuntimeVersion}' 2>/dev/null || echo "unknown")
    log_info "Container runtime: $runtime"
    
    if echo "$runtime" | grep -q "containerd"; then
        log_success "containerd detected"
    else
        log_warning "containerd not detected, but continuing..."
    fi
}

# Build Core image
build_core() {
    log_step "Building Core Docker image..."
    
    cd "$PROJECT_ROOT/core"
    
    local build_args=(
        --build-arg FORTUNA_BUILD_VERSION="${IMAGE_TAG}"
        --build-arg FORTUNA_BUILD_COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo 'dev')"
        --build-arg FORTUNA_BUILD_TIME="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
    )
    
    if docker build "${build_args[@]}" -t "${CORE_IMAGE}" .; then
        log_success "Core image built: ${CORE_IMAGE}"
        
        # Show image size
        local size=$(docker images "${CORE_IMAGE}" --format "{{.Size}}" | head -1)
        log_info "Image size: $size"
    else
        log_error "Failed to build Core image"
        exit 1
    fi
    
    cd - >/dev/null
}

# Build Agent image
build_agent() {
    log_step "Building Agent Docker image..."
    
    cd "$PROJECT_ROOT/agent"
    
    local build_args=(
        --build-arg FORTUNA_BUILD_VERSION="${IMAGE_TAG}"
        --build-arg FORTUNA_BUILD_COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo 'dev')"
        --build-arg FORTUNA_BUILD_TIME="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
    )
    
    if docker build "${build_args[@]}" -t "${AGENT_IMAGE}" .; then
        log_success "Agent image built: ${AGENT_IMAGE}"
        
        # Show image size
        local size=$(docker images "${AGENT_IMAGE}" --format "{{.Size}}" | head -1)
        log_info "Image size: $size"
    else
        log_error "Failed to build Agent image"
        exit 1
    fi
    
    cd - >/dev/null
}

# Push images to registry
push_images() {
    if [ -z "$REGISTRY" ]; then
        log_warning "No registry specified, skipping push"
        log_info "Images are available locally only"
        log_info "To use on remote nodes, either:"
        log_info "  1. Set REGISTRY environment variable and push"
        log_info "  2. Save and load images manually on each node"
        return
    fi
    
    log_step "Pushing images to registry..."
    
    if docker push "${CORE_IMAGE}"; then
        log_success "Core image pushed: ${CORE_IMAGE}"
    else
        log_error "Failed to push Core image"
        exit 1
    fi
    
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
    
    if kubectl get namespace "$NAMESPACE" &>/dev/null; then
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
    
    log_success "Secrets created"
}

# Update deployment files with image references
update_deployments() {
    log_step "Updating deployment files with image references..."
    
    local deploy_dir="$PROJECT_ROOT/deploy"
    
    # Update Core deployment
    if [ -f "$deploy_dir/fortuna-core-deployment.yaml" ]; then
        if [ -n "$REGISTRY" ]; then
            # Update image reference
            sed -i.bak "s|image: fortuna-core:.*|image: ${CORE_IMAGE}|g" "$deploy_dir/fortuna-core-deployment.yaml"
            sed -i.bak "s|imagePullPolicy: Never|imagePullPolicy: IfNotPresent|g" "$deploy_dir/fortuna-core-deployment.yaml"
            log_info "Updated Core deployment with registry image"
        else
            # Use local image
            sed -i.bak "s|image: fortuna-core:.*|image: ${CORE_IMAGE}|g" "$deploy_dir/fortuna-core-deployment.yaml"
            sed -i.bak "s|imagePullPolicy:.*|imagePullPolicy: IfNotPresent|g" "$deploy_dir/fortuna-core-deployment.yaml"
            log_info "Updated Core deployment with local image"
        fi
    fi
    
    # Update Agent deployment
    if [ -f "$deploy_dir/fortuna-agent-daemonset.yaml" ]; then
        if [ -n "$REGISTRY" ]; then
            # Update image reference
            sed -i.bak "s|image: fortuna-agent:.*|image: ${AGENT_IMAGE}|g" "$deploy_dir/fortuna-agent-daemonset.yaml"
            sed -i.bak "s|imagePullPolicy: Never|imagePullPolicy: IfNotPresent|g" "$deploy_dir/fortuna-agent-daemonset.yaml"
            log_info "Updated Agent deployment with registry image"
        else
            # Use local image
            sed -i.bak "s|image: fortuna-agent:.*|image: ${AGENT_IMAGE}|g" "$deploy_dir/fortuna-agent-daemonset.yaml"
            sed -i.bak "s|imagePullPolicy:.*|imagePullPolicy: IfNotPresent|g" "$deploy_dir/fortuna-agent-daemonset.yaml"
            log_info "Updated Agent deployment with local image"
        fi
    fi
    
    log_success "Deployment files updated"
}

# Deploy infrastructure
deploy_infrastructure() {
    log_step "Deploying infrastructure..."
    
    local deploy_dir="$PROJECT_ROOT/deploy"
    
    # Deploy PostgreSQL
    if [ -f "$deploy_dir/infrastructure/postgresql-with-age.yaml" ]; then
        log_info "Deploying PostgreSQL..."
        kubectl apply -f "$deploy_dir/infrastructure/postgresql-with-age.yaml"
        
        log_info "Waiting for PostgreSQL to be ready..."
        if kubectl wait --for=condition=ready pod -l app=postgres -n "$NAMESPACE" --timeout=300s 2>/dev/null; then
            log_success "PostgreSQL is ready"
        else
            log_warning "PostgreSQL not ready yet (may take a few minutes)"
        fi
    else
        log_warning "PostgreSQL deployment file not found"
    fi
    
    # Deploy NATS
    if [ -f "$deploy_dir/infrastructure/nats.yaml" ]; then
        log_info "Deploying NATS..."
        kubectl apply -f "$deploy_dir/infrastructure/nats.yaml"
        
        log_info "Waiting for NATS to be ready..."
        if kubectl wait --for=condition=ready pod -l app=nats -n "$NAMESPACE" --timeout=300s 2>/dev/null; then
            log_success "NATS is ready"
        else
            log_warning "NATS not ready yet (may take a few minutes)"
        fi
    else
        log_warning "NATS deployment file not found"
    fi
}

# Deploy Fortuna
deploy_fortuna() {
    log_step "Deploying Fortuna components..."
    
    local deploy_dir="$PROJECT_ROOT/deploy"
    
    # Deploy RBAC
    if [ -f "$deploy_dir/fortuna-rbac.yaml" ]; then
        log_info "Deploying RBAC..."
        kubectl apply -f "$deploy_dir/fortuna-rbac.yaml"
        log_success "RBAC deployed"
    else
        log_warning "RBAC file not found"
    fi
    
    # Deploy Core
    if [ -f "$deploy_dir/fortuna-core-deployment.yaml" ]; then
        log_info "Deploying Core..."
        kubectl apply -f "$deploy_dir/fortuna-core-deployment.yaml"
        
        log_info "Waiting for Core to be ready..."
        if kubectl wait --for=condition=ready pod -l app.kubernetes.io/component=core -n "$NAMESPACE" --timeout=300s 2>/dev/null; then
            log_success "Core is ready"
        else
            log_warning "Core not ready yet (check logs)"
        fi
    else
        log_warning "Core deployment file not found"
    fi
    
    # Deploy Agent
    if [ -f "$deploy_dir/fortuna-agent-daemonset.yaml" ]; then
        log_info "Deploying Agent..."
        kubectl apply -f "$deploy_dir/fortuna-agent-daemonset.yaml"
        
        log_info "Waiting for Agent pods..."
        sleep 5
        local agent_pods=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=agent --no-headers 2>/dev/null | wc -l)
        if [ "$agent_pods" -gt 0 ]; then
            log_success "Agent deployed: $agent_pods pod(s)"
        else
            log_warning "No Agent pods found"
        fi
    else
        log_warning "Agent deployment file not found"
    fi
}

# Show status
show_status() {
    log_step "Deployment status:"
    echo ""
    
    echo "=== Nodes ==="
    kubectl get nodes
    echo ""
    
    echo "=== Pods ==="
    kubectl get pods -n "$NAMESPACE"
    echo ""
    
    echo "=== Services ==="
    kubectl get svc -n "$NAMESPACE"
    echo ""
    
    echo "=== Deployments ==="
    kubectl get deployments -n "$NAMESPACE" 2>/dev/null || echo "No deployments found"
    echo ""
    
    echo "=== DaemonSets ==="
    kubectl get daemonsets -n "$NAMESPACE" 2>/dev/null || echo "No daemonsets found"
    echo ""
}

# Main execution
main() {
    echo ""
    echo "=========================================="
    echo "Fortuna Build and Deploy to Multi-Node K8s"
    echo "=========================================="
    echo ""
    echo "Configuration:"
    echo "  Image Tag: ${IMAGE_TAG}"
    echo "  Registry: ${REGISTRY:-<not set - using local images>}"
    echo "  Namespace: ${NAMESPACE}"
    echo ""
    
    check_prerequisites
    check_cluster
    build_core
    build_agent
    
    if [ -n "$REGISTRY" ]; then
        push_images
    fi
    
    create_namespace
    generate_certificates
    update_deployments
    deploy_infrastructure
    deploy_fortuna
    show_status
    
    echo ""
    log_success "Build and deploy completed!"
    echo ""
    log_info "Next steps:"
    echo "  1. Check pod status: kubectl get pods -n ${NAMESPACE}"
    echo "  2. Check Core logs: kubectl logs -n ${NAMESPACE} -l app.kubernetes.io/component=core --tail=50"
    echo "  3. Check Agent logs: kubectl logs -n ${NAMESPACE} -l app.kubernetes.io/component=agent --tail=50"
    echo "  4. Test API: kubectl port-forward -n ${NAMESPACE} svc/fortuna-core 8080:8080 &"
    echo "     Then: curl http://localhost:8080/health"
    echo ""
    
    if [ -z "$REGISTRY" ]; then
        log_warning "Images are only available locally"
        log_info "To use on remote nodes, either:"
        log_info "  1. Set REGISTRY and push images"
        log_info "  2. Save images: docker save ${CORE_IMAGE} -o fortuna-core.tar"
        log_info "  3. Load on each node: docker load -i fortuna-core.tar"
        echo ""
    fi
}

# Run main
main "$@"


