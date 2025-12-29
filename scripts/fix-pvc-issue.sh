#!/bin/bash

# ============================================================================
# Fix PVC Unbound Issue
# ============================================================================
# This script fixes the "pod has unbound immediate PersistentVolumeClaims" error
# by installing local-path-provisioner or creating manual PVs
# ============================================================================

set -euo pipefail

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

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

# Check if kubectl is available
if ! command -v kubectl >/dev/null 2>&1; then
    log_error "kubectl is not installed or not in PATH"
    exit 1
fi

# Check if cluster is accessible
if ! kubectl cluster-info &>/dev/null 2>&1; then
    log_error "Kubernetes cluster is not accessible"
    log_info "Make sure kubectl is configured correctly"
    exit 1
fi

log_success "Kubernetes cluster is accessible"

# Check existing StorageClasses
log_info "Checking existing StorageClasses..."
STORAGE_CLASSES=$(kubectl get storageclass --no-headers 2>/dev/null | wc -l)
DEFAULT_SC=$(kubectl get storageclass -o jsonpath='{.items[?(@.metadata.annotations.storageclass\.kubernetes\.io/is-default-class=="true")].metadata.name}' 2>/dev/null || echo "")

if [ -n "$DEFAULT_SC" ]; then
    log_success "Default StorageClass found: $DEFAULT_SC"
    log_info "PVCs should bind automatically. Checking PVC status..."
    
    # Check PVC status
    if kubectl get namespace fortuna &>/dev/null 2>&1; then
        PVC_STATUS=$(kubectl get pvc -n fortuna postgres-pvc -o jsonpath='{.status.phase}' 2>/dev/null || echo "NotFound")
        if [ "$PVC_STATUS" = "Bound" ]; then
            log_success "PostgreSQL PVC is already Bound"
            exit 0
        elif [ "$PVC_STATUS" = "Pending" ]; then
            log_warning "PostgreSQL PVC is Pending. This might resolve automatically."
            log_info "Waiting 30 seconds for PVC to bind..."
            sleep 30
            PVC_STATUS=$(kubectl get pvc -n fortuna postgres-pvc -o jsonpath='{.status.phase}' 2>/dev/null || echo "NotFound")
            if [ "$PVC_STATUS" = "Bound" ]; then
                log_success "PostgreSQL PVC is now Bound"
                exit 0
            fi
        fi
    fi
else
    log_warning "No default StorageClass found"
fi

# Ask user which solution to use
echo ""
log_info "Choose a solution to fix PVC issue:"
echo "  1) Install local-path-provisioner (recommended for development)"
echo "  2) Create manual PersistentVolume with hostPath"
echo "  3) Check current PVC/PV status only"
echo ""
read -p "Enter choice [1-3]: " choice

case $choice in
    1)
        log_info "Installing local-path-provisioner..."
        
        # Install local-path-provisioner
        if kubectl apply -f https://raw.githubusercontent.com/rancher/local-path-provisioner/v0.0.24/deploy/local-path-storage.yaml; then
            log_success "local-path-provisioner installed"
        else
            log_error "Failed to install local-path-provisioner"
            exit 1
        fi
        
        # Wait for pods to be ready
        log_info "Waiting for local-path-provisioner to be ready..."
        kubectl wait --for=condition=ready pod -l app=local-path-provisioner -n local-path-storage --timeout=120s 2>/dev/null || log_warning "Timeout waiting for pods"
        
        # Set as default
        log_info "Setting local-path as default StorageClass..."
        if kubectl patch storageclass local-path -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'; then
            log_success "local-path set as default StorageClass"
        else
            log_warning "Failed to set as default (may already be default)"
        fi
        
        # Delete and recreate PVC if exists
        if kubectl get pvc -n fortuna postgres-pvc &>/dev/null 2>&1; then
            log_info "Deleting existing PVC to recreate with new StorageClass..."
            kubectl delete pvc -n fortuna postgres-pvc
            sleep 2
        fi
        
        # Apply PostgreSQL with updated PVC
        if [ -f "deploy/infrastructure/postgresql-with-age.yaml" ]; then
            log_info "Applying PostgreSQL deployment..."
            kubectl apply -f deploy/infrastructure/postgresql-with-age.yaml
            log_success "PostgreSQL deployment applied"
        else
            log_error "PostgreSQL deployment file not found: deploy/infrastructure/postgresql-with-age.yaml"
            exit 1
        fi
        
        log_info "Waiting for PVC to bind..."
        sleep 5
        ;;
        
    2)
        log_info "Creating manual PersistentVolume with hostPath..."
        
        # Get node name
        NODE_NAME=$(kubectl get nodes -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
        if [ -z "$NODE_NAME" ]; then
            log_error "No nodes found in cluster"
            exit 1
        fi
        
        log_info "Using node: $NODE_NAME"
        log_warning "You need to create directory on node: /mnt/postgres-data"
        log_info "Run this command on node $NODE_NAME:"
        echo "  sudo mkdir -p /mnt/postgres-data && sudo chmod 777 /mnt/postgres-data"
        echo ""
        read -p "Have you created the directory? (y/n): " confirm
        
        if [ "$confirm" != "y" ] && [ "$confirm" != "Y" ]; then
            log_error "Please create the directory first"
            exit 1
        fi
        
        # Apply local PV manifest
        if [ -f "deploy/infrastructure/postgresql-local-pv.yaml" ]; then
            log_info "Applying PostgreSQL with local PV..."
            kubectl apply -f deploy/infrastructure/postgresql-local-pv.yaml
            log_success "PostgreSQL with local PV applied"
        else
            log_error "Local PV manifest not found: deploy/infrastructure/postgresql-local-pv.yaml"
            exit 1
        fi
        
        log_info "Waiting for PV and PVC to bind..."
        sleep 5
        ;;
        
    3)
        log_info "Checking current status..."
        ;;
        
    *)
        log_error "Invalid choice"
        exit 1
        ;;
esac

# Check final status
echo ""
log_info "Current status:"
echo ""

echo "=== StorageClasses ==="
kubectl get storageclass
echo ""

echo "=== PersistentVolumes ==="
kubectl get pv
echo ""

if kubectl get namespace fortuna &>/dev/null 2>&1; then
    echo "=== PersistentVolumeClaims (fortuna namespace) ==="
    kubectl get pvc -n fortuna
    echo ""
    
    echo "=== PostgreSQL Pods ==="
    kubectl get pods -n fortuna -l app=postgres
    echo ""
    
    # Check PVC status
    PVC_STATUS=$(kubectl get pvc -n fortuna postgres-pvc -o jsonpath='{.status.phase}' 2>/dev/null || echo "NotFound")
    if [ "$PVC_STATUS" = "Bound" ]; then
        log_success "PostgreSQL PVC is Bound ✅"
        log_info "PostgreSQL should start soon"
    elif [ "$PVC_STATUS" = "Pending" ]; then
        log_warning "PostgreSQL PVC is still Pending"
        log_info "This may take a few minutes. Check with: kubectl get pvc -n fortuna -w"
    else
        log_warning "PostgreSQL PVC status: $PVC_STATUS"
    fi
else
    log_warning "Namespace 'fortuna' does not exist"
    log_info "Create it with: kubectl create namespace fortuna"
fi

echo ""
log_info "Troubleshooting:"
echo "  - Check PVC: kubectl get pvc -n fortuna"
echo "  - Check PV: kubectl get pv"
echo "  - Check pods: kubectl get pods -n fortuna -l app=postgres"
echo "  - Describe PVC: kubectl describe pvc -n fortuna postgres-pvc"
echo ""

