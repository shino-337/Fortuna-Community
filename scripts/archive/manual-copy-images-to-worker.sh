#!/bin/bash

# ============================================================================
# Manual Copy Images to Worker Node
# ============================================================================
# Simple script to copy images from master to worker node
# Usage: ./scripts/manual-copy-images-to-worker.sh <worker-node-ip-or-hostname>
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

# Check arguments
if [ $# -eq 0 ]; then
    log_error "Usage: $0 <worker-node-ip-or-hostname>"
    log_info "Example: $0 k8s-worker01"
    log_info "Or: $0 192.168.1.101"
    exit 1
fi

WORKER_NODE="$1"

# Check prerequisites
if ! command -v ctr >/dev/null 2>&1; then
    log_error "ctr is not installed"
    exit 1
fi

log_info "Copying images to worker node: $WORKER_NODE"

# Check if images exist locally
CORE_EXISTS=$(ctr -n k8s.io images ls 2>/dev/null | grep -c "fortuna-core:latest" || echo "0")
AGENT_EXISTS=$(ctr -n k8s.io images ls 2>/dev/null | grep -c "fortuna-agent:latest" || echo "0")

if [ "$CORE_EXISTS" -eq 0 ] && [ "$AGENT_EXISTS" -eq 0 ]; then
    log_error "Images not found locally in k8s.io namespace"
    log_info "Please ensure images are in k8s.io namespace first:"
    log_info "  ./scripts/copy-images-to-k8s-namespace.sh"
    exit 1
fi

# Create temp directory
TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

# Export images
log_info "Exporting images..."
if [ "$CORE_EXISTS" -gt 0 ]; then
    ctr -n k8s.io images export "$TEMP_DIR/fortuna-core.tar" "docker.io/library/fortuna-core:latest" || {
        log_error "Failed to export Core image"
        exit 1
    }
    log_success "Core image exported"
fi

if [ "$AGENT_EXISTS" -gt 0 ]; then
    ctr -n k8s.io images export "$TEMP_DIR/fortuna-agent.tar" "docker.io/library/fortuna-agent:latest" || {
        log_error "Failed to export Agent image"
        exit 1
    }
    log_success "Agent image exported"
fi

# Copy to worker node
log_info "Copying files to $WORKER_NODE..."

if command -v scp >/dev/null 2>&1; then
    # Try scp
    if scp "$TEMP_DIR/fortuna-core.tar" "$TEMP_DIR/fortuna-agent.tar" "root@$WORKER_NODE:/tmp/" 2>/dev/null; then
        log_success "Files copied to $WORKER_NODE"
    else
        log_warning "scp failed. Please copy manually:"
        log_info "  scp $TEMP_DIR/fortuna-*.tar root@$WORKER_NODE:/tmp/"
        exit 1
    fi
else
    log_warning "scp not available. Please copy manually:"
    log_info "  scp $TEMP_DIR/fortuna-*.tar root@$WORKER_NODE:/tmp/"
    exit 1
fi

# Import on worker node
log_info "Importing images on $WORKER_NODE..."

if command -v ssh >/dev/null 2>&1; then
    ssh "root@$WORKER_NODE" <<EOF
        echo "Importing Core image..."
        ctr -n k8s.io images import /tmp/fortuna-core.tar
        echo "Importing Agent image..."
        ctr -n k8s.io images import /tmp/fortuna-agent.tar
        echo "Cleaning up..."
        rm -f /tmp/fortuna-*.tar
        echo "Verifying..."
        ctr -n k8s.io images ls | grep fortuna
EOF
    
    if [ $? -eq 0 ]; then
        log_success "Images imported successfully on $WORKER_NODE"
    else
        log_error "Failed to import images via SSH"
        log_info "Please SSH to $WORKER_NODE and run manually:"
        echo "  ctr -n k8s.io images import /tmp/fortuna-core.tar"
        echo "  ctr -n k8s.io images import /tmp/fortuna-agent.tar"
        exit 1
    fi
else
    log_warning "ssh not available. Please import manually on $WORKER_NODE:"
    log_info "  ssh root@$WORKER_NODE"
    log_info "  ctr -n k8s.io images import /tmp/fortuna-core.tar"
    log_info "  ctr -n k8s.io images import /tmp/fortuna-agent.tar"
    exit 1
fi

# Verify
log_info "Verifying images on $WORKER_NODE..."
REMOTE_IMAGES=$(ssh "root@$WORKER_NODE" "ctr -n k8s.io images ls 2>/dev/null | grep fortuna" || echo "")
if [ -n "$REMOTE_IMAGES" ]; then
    log_success "Images verified on $WORKER_NODE:"
    echo "$REMOTE_IMAGES"
else
    log_warning "Could not verify images (but import may have succeeded)"
fi

echo ""
log_success "Image copy completed!"
log_info "Pods on $WORKER_NODE should now be able to start"
echo ""

