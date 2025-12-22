#!/bin/bash

# Full Deployment Script for KSAM
# Rebuilds and deploys all components

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

NAMESPACE="ksam"
MINIKUBE_REGISTRY="minikube"

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_section() {
    echo ""
    echo "=========================================="
    echo "$1"
    echo "=========================================="
}

# Check prerequisites
check_prerequisites() {
    log_section "Checking Prerequisites"
    
    # Check minikube
    if ! command -v minikube &> /dev/null; then
        log_error "minikube not found. Please install minikube."
        exit 1
    fi
    
    # Check kubectl
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl not found. Please install kubectl."
        exit 1
    fi
    
    # Check docker
    if ! command -v docker &> /dev/null; then
        log_error "docker not found. Please install docker."
        exit 1
    fi
    
    # Check minikube status
    if ! minikube status &> /dev/null; then
        log_warn "Minikube is not running. Starting minikube..."
        minikube start --memory=4096 --cpus=2
    else
        log_success "Minikube is running"
    fi
    
    # Set docker environment for minikube
    eval $(minikube docker-env)
    log_success "Docker environment set for minikube"
}

# Create namespace
create_namespace() {
    log_section "Creating Namespace"
    
    if kubectl get namespace "$NAMESPACE" &> /dev/null; then
        log_warn "Namespace $NAMESPACE already exists"
    else
        kubectl create namespace "$NAMESPACE"
        log_success "Namespace $NAMESPACE created"
    fi
}

# Build Core image
build_core() {
    log_section "Building Core Image"
    
    cd "$PROJECT_ROOT/core"
    
    log_info "Building ksam-core image..."
    docker build -t ksam-core:latest . 2>&1 | tail -20
    
    if [ $? -eq 0 ]; then
        log_success "Core image built successfully"
    else
        log_error "Core image build failed"
        exit 1
    fi
    
    cd "$PROJECT_ROOT"
}

# Build Agent image
build_agent() {
    log_section "Building Agent Image"
    
    cd "$PROJECT_ROOT/agent"
    
    log_info "Building ksam-agent image..."
    docker build -t ksam-agent:latest . 2>&1 | tail -20
    
    if [ $? -eq 0 ]; then
        log_success "Agent image built successfully"
    else
        log_error "Agent image build failed"
        exit 1
    fi
    
    cd "$PROJECT_ROOT"
}

# Deploy infrastructure
deploy_infrastructure() {
    log_section "Deploying Infrastructure"
    
    # Deploy Postgres
    log_info "Deploying PostgreSQL..."
    kubectl apply -f deploy/infrastructure/postgresql-with-age.yaml -n "$NAMESPACE"
    
    # Wait for Postgres to be ready
    log_info "Waiting for PostgreSQL to be ready..."
    kubectl wait --for=condition=ready pod -l app=postgres -n "$NAMESPACE" --timeout=120s || {
        log_warn "PostgreSQL not ready within timeout, continuing..."
    }
    
    # Deploy NATS
    if [ -f "deploy/infrastructure/nats.yaml" ]; then
        log_info "Deploying NATS..."
        kubectl apply -f deploy/infrastructure/nats.yaml -n "$NAMESPACE"
        
        log_info "Waiting for NATS to be ready..."
        kubectl wait --for=condition=ready pod -l app=nats -n "$NAMESPACE" --timeout=120s || {
            log_warn "NATS not ready within timeout, continuing..."
        }
    else
        log_warn "NATS deployment file not found, skipping..."
    fi
    
    log_success "Infrastructure deployed"
}

# Generate TLS certificates
generate_certificates() {
    log_section "Generating TLS Certificates"
    
    if [ -f "scripts/generate_certs.sh" ]; then
        log_info "Running certificate generation script..."
        bash scripts/generate_certs.sh
        log_success "Certificates generated"
    else
        log_warn "Certificate generation script not found, skipping..."
    fi
}

# Create TLS secrets
create_tls_secrets() {
    log_section "Creating TLS Secrets"
    
    # Check if certs exist
    if [ ! -f "certs/ca.crt" ] || [ ! -f "certs/core.crt" ] || [ ! -f "certs/core.key" ]; then
        log_warn "TLS certificates not found. Generating..."
        generate_certificates
    fi
    
    # Create CA cert secret
    if kubectl get secret ksam-ca-cert -n "$NAMESPACE" &> /dev/null; then
        log_warn "CA cert secret already exists, deleting..."
        kubectl delete secret ksam-ca-cert -n "$NAMESPACE"
    fi
    kubectl create secret generic ksam-ca-cert \
        --from-file=ca.crt=certs/ca.crt \
        -n "$NAMESPACE"
    log_success "CA cert secret created"
    
    # Create Core TLS secret
    if kubectl get secret ksam-core-tls -n "$NAMESPACE" &> /dev/null; then
        log_warn "Core TLS secret already exists, deleting..."
        kubectl delete secret ksam-core-tls -n "$NAMESPACE"
    fi
    kubectl create secret tls ksam-core-tls \
        --cert=certs/core.crt \
        --key=certs/core.key \
        -n "$NAMESPACE"
    log_success "Core TLS secret created"
}

# Deploy Core
deploy_core() {
    log_section "Deploying Core Service"
    
    # Check if deployment file exists
    if [ ! -f "deploy/core-deployment.yaml" ]; then
        log_error "Core deployment file not found"
        exit 1
    fi
    
    log_info "Applying Core deployment..."
    kubectl apply -f deploy/core-deployment.yaml -n "$NAMESPACE"
    
    log_info "Waiting for Core to be ready..."
    kubectl wait --for=condition=ready pod -l app=ksam-core -n "$NAMESPACE" --timeout=120s || {
        log_warn "Core not ready within timeout, checking status..."
        kubectl get pods -n "$NAMESPACE" -l app=ksam-core
    }
    
    log_success "Core deployed"
}

# Deploy Agent
deploy_agent() {
    log_section "Deploying Agent Service"
    
    # Check if deployment file exists
    if [ ! -f "deploy/agent/daemonset.yaml" ]; then
        log_error "Agent deployment file not found"
        exit 1
    fi
    
    log_info "Applying Agent RBAC..."
    if [ -f "deploy/agent/rbac.yaml" ]; then
        kubectl apply -f deploy/agent/rbac.yaml -n "$NAMESPACE"
    fi
    
    log_info "Applying Agent DaemonSet..."
    kubectl apply -f deploy/agent/daemonset.yaml -n "$NAMESPACE"
    
    log_info "Waiting for Agent pods to be ready..."
    sleep 10
    kubectl get pods -n "$NAMESPACE" -l app=ksam-agent
    
    log_success "Agent deployed"
}

# Verify deployment
verify_deployment() {
    log_section "Verifying Deployment"
    
    echo ""
    echo "Pods Status:"
    kubectl get pods -n "$NAMESPACE"
    
    echo ""
    echo "Services Status:"
    kubectl get services -n "$NAMESPACE"
    
    echo ""
    echo "Core Pod Logs (last 10 lines):"
    CORE_POD=$(kubectl get pods -n "$NAMESPACE" -l app=ksam-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
    if [ -n "$CORE_POD" ]; then
        kubectl logs -n "$NAMESPACE" "$CORE_POD" --tail=10 2>&1 || true
    fi
    
    log_success "Deployment verification complete"
}

# Main deployment flow
main() {
    log_section "KSAM Full Deployment"
    echo "Starting deployment at $(date)"
    echo ""
    
    check_prerequisites
    create_namespace
    generate_certificates
    create_tls_secrets
    build_core
    build_agent
    deploy_infrastructure
    deploy_core
    deploy_agent
    
    # Wait a bit for everything to settle
    log_info "Waiting for services to stabilize..."
    sleep 15
    
    verify_deployment
    
    log_section "Deployment Complete"
    echo "Deployment finished at $(date)"
    echo ""
    echo "To check status:"
    echo "  kubectl get pods -n $NAMESPACE"
    echo "  kubectl get services -n $NAMESPACE"
    echo ""
    echo "To view logs:"
    echo "  kubectl logs -n $NAMESPACE -l app=ksam-core"
    echo "  kubectl logs -n $NAMESPACE -l app=ksam-agent"
}

# Run main
main

