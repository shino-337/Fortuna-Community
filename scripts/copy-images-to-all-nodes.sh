#!/bin/bash

# ============================================================================
# Copy Images to All Kubernetes Nodes
# ============================================================================
# Copies Fortuna images to all nodes in the cluster
# Required for multi-node clusters where images are only on master
# ============================================================================

set -euo pipefail

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
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

log_section() {
    echo ""
    echo -e "${CYAN}========================================${NC}"
    echo -e "${CYAN}$1${NC}"
    echo -e "${CYAN}========================================${NC}"
    echo ""
}

# Check prerequisites
if ! command -v kubectl >/dev/null 2>&1; then
    log_error "kubectl is not installed"
    exit 1
fi

if ! command -v ctr >/dev/null 2>&1; then
    log_error "ctr is not installed (required for containerd)"
    exit 1
fi

# Get all nodes
log_section "Getting cluster nodes"
NODES=$(kubectl get nodes -o jsonpath='{.items[*].metadata.name}')
if [ -z "$NODES" ]; then
    log_error "No nodes found in cluster"
    exit 1
fi

log_info "Found nodes:"
for node in $NODES; do
    echo "  - $node"
done

# Export images on master/current node
log_section "Step 1: Exporting images from current node"

TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

# Check if images exist in k8s.io namespace
CORE_EXISTS=$(ctr -n k8s.io images ls 2>/dev/null | grep -c "fortuna-core:latest" || echo "0")
AGENT_EXISTS=$(ctr -n k8s.io images ls 2>/dev/null | grep -c "fortuna-agent:latest" || echo "0")

if [ "$CORE_EXISTS" -eq 0 ] && [ "$AGENT_EXISTS" -eq 0 ]; then
    log_error "Images not found in k8s.io namespace"
    log_info "Please run: ./scripts/copy-images-to-k8s-namespace.sh first"
    exit 1
fi

# Export Core image
if [ "$CORE_EXISTS" -gt 0 ]; then
    log_info "Exporting fortuna-core:latest..."
    if ctr -n k8s.io images export "$TEMP_DIR/fortuna-core.tar" "docker.io/library/fortuna-core:latest" 2>/dev/null; then
        log_success "Core image exported"
    else
        log_error "Failed to export Core image"
        exit 1
    fi
fi

# Export Agent image
if [ "$AGENT_EXISTS" -gt 0 ]; then
    log_info "Exporting fortuna-agent:latest..."
    if ctr -n k8s.io images export "$TEMP_DIR/fortuna-agent.tar" "docker.io/library/fortuna-agent:latest" 2>/dev/null; then
        log_success "Agent image exported"
    else
        log_error "Failed to export Agent image"
        exit 1
    fi
fi

# Get current node name
CURRENT_NODE=$(hostname 2>/dev/null || echo "unknown")
log_info "Current node: $CURRENT_NODE"

# Copy to each node
log_section "Step 2: Copying images to all nodes"

for node in $NODES; do
    log_info "Processing node: $node"
    
    # Skip if current node (already has images)
    if [ "$node" = "$CURRENT_NODE" ]; then
        log_info "  Skipping current node (already has images)"
        continue
    fi
    
    # Copy tar files to node
    log_info "  Copying image files to $node..."
    
    # Use scp if available, otherwise suggest manual copy
    if command -v scp >/dev/null 2>&1; then
        # Try to copy via scp (may need SSH key setup)
        if scp "$TEMP_DIR/fortuna-core.tar" "$TEMP_DIR/fortuna-agent.tar" "root@$node:/tmp/" 2>/dev/null; then
            log_success "  Files copied to $node"
        else
            log_warning "  scp failed, trying alternative method..."
            # Alternative: use kubectl cp via pod
            log_info "  Using kubectl cp method..."
            # Create a temporary pod for file transfer
            # This is more complex, so we'll provide manual instructions
            log_warning "  Manual copy required for $node"
            log_info "  Run on $node:"
            echo "    # Copy files manually or use:"
            echo "    scp $TEMP_DIR/fortuna-*.tar root@$node:/tmp/"
            continue
        fi
    else
        log_warning "  scp not available, manual copy required"
        log_info "  Manual steps for $node:"
        echo "    1. Copy files: scp $TEMP_DIR/fortuna-*.tar root@$node:/tmp/"
        echo "    2. SSH to node: ssh root@$node"
        echo "    3. Import images:"
        echo "       ctr -n k8s.io images import /tmp/fortuna-core.tar"
        echo "       ctr -n k8s.io images import /tmp/fortuna-agent.tar"
        echo "    4. Verify: ctr -n k8s.io images ls | grep fortuna"
        continue
    fi
    
    # Import images on remote node
    log_info "  Importing images on $node..."
    ssh "root@$node" "ctr -n k8s.io images import /tmp/fortuna-core.tar && ctr -n k8s.io images import /tmp/fortuna-agent.tar && rm -f /tmp/fortuna-*.tar" 2>/dev/null && {
        log_success "  Images imported on $node"
    } || {
        log_warning "  Failed to import via SSH, manual import required"
        log_info "  SSH to $node and run:"
        echo "    ctr -n k8s.io images import /tmp/fortuna-core.tar"
        echo "    ctr -n k8s.io images import /tmp/fortuna-agent.tar"
    }
done

# Alternative method: Use DaemonSet with initContainer
log_section "Alternative: Using image registry or manual copy"

log_info "If SSH method doesn't work, use one of these:"
echo ""
echo "Option 1: Push to registry (recommended for production)"
echo "  docker tag fortuna-core:latest <registry>/fortuna-core:latest"
echo "  docker tag fortuna-agent:latest <registry>/fortuna-agent:latest"
echo "  docker push <registry>/fortuna-core:latest"
echo "  docker push <registry>/fortuna-agent:latest"
echo "  # Then update deployments to use registry images"
echo ""
echo "Option 2: Manual copy to each node"
echo "  # On master node:"
echo "  ctr -n k8s.io images export /tmp/fortuna-core.tar docker.io/library/fortuna-core:latest"
echo "  ctr -n k8s.io images export /tmp/fortuna-agent.tar docker.io/library/fortuna-agent:latest"
echo "  # Copy to each worker node:"
echo "  scp /tmp/fortuna-*.tar root@<worker-node>:/tmp/"
echo "  # On each worker node:"
echo "  ctr -n k8s.io images import /tmp/fortuna-core.tar"
echo "  ctr -n k8s.io images import /tmp/fortuna-agent.tar"
echo ""

# Final verification
log_section "Step 3: Verification"

log_info "Checking images on all nodes..."
for node in $NODES; do
    log_info "Node: $node"
    if [ "$node" = "$CURRENT_NODE" ]; then
        # Check locally
        IMAGES=$(ctr -n k8s.io images ls 2>/dev/null | grep fortuna || echo "")
        if [ -n "$IMAGES" ]; then
            log_success "  Images found locally"
        else
            log_error "  Images not found"
        fi
    else
        # Check remotely
        REMOTE_IMAGES=$(ssh "root@$node" "ctr -n k8s.io images ls 2>/dev/null | grep fortuna" || echo "")
        if [ -n "$REMOTE_IMAGES" ]; then
            log_success "  Images found on $node"
        else
            log_warning "  Images not found on $node (may need manual copy)"
        fi
    fi
done

echo ""
log_success "Image copy process completed!"
log_info "If some nodes still don't have images, use manual copy method above"
echo ""

