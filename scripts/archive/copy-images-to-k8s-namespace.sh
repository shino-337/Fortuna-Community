#!/bin/bash

# ============================================================================
# Copy Images to k8s.io Namespace
# ============================================================================
# Copies images from default containerd namespace to k8s.io namespace
# This is required for Kubernetes to find the images
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

# Check if ctr is available
if ! command -v ctr >/dev/null 2>&1; then
    log_error "ctr is not installed (required for containerd)"
    exit 1
fi

log_info "Checking images in containerd..."

# Check images in default namespace
log_info "Images in default namespace:"
DEFAULT_IMAGES=$(ctr images ls 2>/dev/null | grep fortuna || echo "")
if [ -z "$DEFAULT_IMAGES" ]; then
    log_warning "No fortuna images found in default namespace"
    log_info "Checking k8s.io namespace..."
    K8S_IMAGES=$(ctr -n k8s.io images ls 2>/dev/null | grep fortuna || echo "")
    if [ -z "$K8S_IMAGES" ]; then
        log_error "No fortuna images found in any namespace"
        log_info "Please build images first or import them"
        exit 1
    else
        log_success "Images already in k8s.io namespace:"
        echo "$K8S_IMAGES"
        exit 0
    fi
else
    echo "$DEFAULT_IMAGES"
fi

# Check images in k8s.io namespace
log_info "Images in k8s.io namespace:"
K8S_IMAGES=$(ctr -n k8s.io images ls 2>/dev/null | grep fortuna || echo "")
if [ -n "$K8S_IMAGES" ]; then
    echo "$K8S_IMAGES"
    log_success "Images already exist in k8s.io namespace"
    exit 0
else
    log_warning "No images in k8s.io namespace"
fi

# Copy images to k8s.io namespace
log_info "Copying images to k8s.io namespace..."

# Get image references
CORE_REF=$(ctr images ls | grep "fortuna-core:latest" | awk '{print $1}' | head -1)
AGENT_REF=$(ctr images ls | grep "fortuna-agent:latest" | awk '{print $1}' | head -1)

if [ -z "$CORE_REF" ] && [ -z "$AGENT_REF" ]; then
    log_error "No fortuna images found to copy"
    exit 1
fi

# Create temp directory
TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

# Export and import Core image
if [ -n "$CORE_REF" ]; then
    log_info "Copying Core image: $CORE_REF"
    
    # Export from default namespace
    log_info "  Exporting from default namespace..."
    if ctr images export "$TEMP_DIR/fortuna-core.tar" "$CORE_REF" 2>/dev/null; then
        log_success "  Exported successfully"
    else
        log_error "  Failed to export Core image"
        exit 1
    fi
    
    # Import to k8s.io namespace
    log_info "  Importing to k8s.io namespace..."
    if ctr -n k8s.io images import "$TEMP_DIR/fortuna-core.tar" 2>/dev/null; then
        log_success "  Imported successfully"
    else
        log_error "  Failed to import Core image to k8s.io namespace"
        exit 1
    fi
fi

# Export and import Agent image
if [ -n "$AGENT_REF" ]; then
    log_info "Copying Agent image: $AGENT_REF"
    
    # Export from default namespace
    log_info "  Exporting from default namespace..."
    if ctr images export "$TEMP_DIR/fortuna-agent.tar" "$AGENT_REF" 2>/dev/null; then
        log_success "  Exported successfully"
    else
        log_error "  Failed to export Agent image"
        exit 1
    fi
    
    # Import to k8s.io namespace
    log_info "  Importing to k8s.io namespace..."
    if ctr -n k8s.io images import "$TEMP_DIR/fortuna-agent.tar" 2>/dev/null; then
        log_success "  Imported successfully"
    else
        log_error "  Failed to import Agent image to k8s.io namespace"
        exit 1
    fi
fi

# Verify
log_info "Verifying images in k8s.io namespace:"
FINAL_IMAGES=$(ctr -n k8s.io images ls 2>/dev/null | grep fortuna || echo "")
if [ -n "$FINAL_IMAGES" ]; then
    echo "$FINAL_IMAGES"
    log_success "Images are now available in k8s.io namespace!"
    log_info "Kubernetes should now be able to use these images"
else
    log_error "Images not found in k8s.io namespace after import"
    exit 1
fi

echo ""

