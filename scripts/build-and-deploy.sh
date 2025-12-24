#!/bin/bash

# ============================================================================
# Build and Deploy to Minikube
# ============================================================================
# Builds Docker images for Core and Agent, loads them into minikube,
# and deploys the services
# ============================================================================

set -euo pipefail

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
IMAGE_TAG="${IMAGE_TAG:-latest}"
CORE_IMAGE="fortuna-core:${IMAGE_TAG}"
AGENT_IMAGE="fortuna-agent:${IMAGE_TAG}"
MINIKUBE_REGISTRY="minikube"

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

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    local missing=0
    
    if ! command -v docker >/dev/null 2>&1; then
        log_error "docker is not installed"
        missing=1
    else
        log_success "docker is installed"
    fi
    
    if ! command -v minikube >/dev/null 2>&1; then
        log_error "minikube is not installed"
        missing=1
    else
        log_success "minikube is installed"
    fi
    
    if ! command -v kubectl >/dev/null 2>&1; then
        log_error "kubectl is not installed"
        missing=1
    else
        log_success "kubectl is installed"
    fi
    
    if [ $missing -eq 1 ]; then
        log_error "Please install missing prerequisites"
        exit 1
    fi
}

# Check minikube status
check_minikube() {
    log_info "Checking minikube status..."
    
    if ! minikube status >/dev/null 2>&1; then
        log_warning "Minikube is not running. Starting minikube..."
        minikube start
    else
        log_success "Minikube is running"
    fi
    
    # Set docker environment for minikube
    eval $(minikube docker-env)
    log_success "Docker environment configured for minikube"
}

# Build Core image
build_core() {
    log_info "Building Core Docker image..."
    
    cd "$(dirname "$0")/../core"
    
    if docker build -t "${CORE_IMAGE}" .; then
        log_success "Core image built: ${CORE_IMAGE}"
    else
        log_error "Failed to build Core image"
        exit 1
    fi
    
    cd - >/dev/null
}

# Build Agent image
build_agent() {
    log_info "Building Agent Docker image..."
    
    cd "$(dirname "$0")/../agent"
    
    if docker build -t "${AGENT_IMAGE}" .; then
        log_success "Agent image built: ${AGENT_IMAGE}"
    else
        log_error "Failed to build Agent image"
        exit 1
    fi
    
    cd - >/dev/null
}

# Verify images in minikube
verify_images() {
    log_info "Verifying images in minikube..."
    
    if docker images | grep -q "fortuna-core"; then
        log_success "Core image found in minikube"
    else
        log_error "Core image not found in minikube"
        exit 1
    fi
    
    if docker images | grep -q "fortuna-agent"; then
        log_success "Agent image found in minikube"
    else
        log_error "Agent image not found in minikube"
        exit 1
    fi
}

# Deploy to Kubernetes
deploy() {
    log_info "Deploying to Kubernetes..."
    
    local deploy_dir="$(dirname "$0")/../deploy"
    
    # Apply RBAC first
    if [ -f "${deploy_dir}/fortuna-rbac.yaml" ]; then
        log_info "Applying RBAC..."
        kubectl apply -f "${deploy_dir}/fortuna-rbac.yaml" || log_warning "RBAC may already exist"
    fi
    
    # Apply Core deployment
    if [ -f "${deploy_dir}/fortuna-core-deployment.yaml" ]; then
        log_info "Applying Core deployment..."
        kubectl apply -f "${deploy_dir}/fortuna-core-deployment.yaml"
        log_success "Core deployment applied"
    else
        log_warning "Core deployment file not found"
    fi
    
    # Apply Agent DaemonSet
    if [ -f "${deploy_dir}/fortuna-agent-daemonset.yaml" ]; then
        log_info "Applying Agent DaemonSet..."
        kubectl apply -f "${deploy_dir}/fortuna-agent-daemonset.yaml"
        log_success "Agent DaemonSet applied"
    else
        log_warning "Agent DaemonSet file not found"
    fi
    
    # Wait for deployments
    log_info "Waiting for deployments to be ready..."
    kubectl wait --for=condition=available --timeout=300s deployment/fortuna-core 2>/dev/null || log_warning "Core deployment not ready yet"
    kubectl wait --for=condition=ready --timeout=300s daemonset/fortuna-agent 2>/dev/null || log_warning "Agent DaemonSet not ready yet"
}

# Show status
show_status() {
    log_info "Current deployment status:"
    echo ""
    
    echo "=== Pods ==="
    kubectl get pods -l app=fortuna-core 2>/dev/null || echo "No Core pods found"
    kubectl get pods -l app=fortuna-agent 2>/dev/null || echo "No Agent pods found"
    echo ""
    
    echo "=== Deployments ==="
    kubectl get deployments -l app=fortuna-core 2>/dev/null || echo "No Core deployment found"
    echo ""
    
    echo "=== DaemonSets ==="
    kubectl get daemonsets -l app=fortuna-agent 2>/dev/null || echo "No Agent DaemonSet found"
    echo ""
    
    echo "=== Services ==="
    kubectl get services -l app=fortuna-core 2>/dev/null || echo "No Core service found"
    echo ""
    
    echo "=== Images in Minikube ==="
    docker images | grep -E "fortuna|REPOSITORY" || echo "No images found"
}

# Main execution
main() {
    echo ""
    echo "=========================================="
    echo "KSAM Build and Deploy to Minikube"
    echo "=========================================="
    echo ""
    
    check_prerequisites
    check_minikube
    build_core
    build_agent
    verify_images
    deploy
    show_status
    
    echo ""
    log_success "Build and deploy completed!"
    echo ""
    log_info "Next steps:"
    echo "  1. Check pod status: kubectl get pods"
    echo "  2. Check logs: kubectl logs -l app=fortuna-core"
    echo "  3. Check Agent logs: kubectl logs -l app=fortuna-agent"
    echo ""
}

# Run main
main "$@"
