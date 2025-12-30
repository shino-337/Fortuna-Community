#!/bin/bash

# ============================================================================
# Fix Containerd Images for Kubernetes
# ============================================================================
# Ensures images are in the correct containerd namespace (k8s.io)
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

# Check images in k8s.io namespace
CORE_IN_K8S=$(ctr -n k8s.io images ls | grep -c "fortuna-core:latest" || echo "0")
AGENT_IN_K8S=$(ctr -n k8s.io images ls | grep -c "fortuna-agent:latest" || echo "0")

log_info "Images in k8s.io namespace:"
ctr -n k8s.io images ls | grep fortuna || log_warning "No fortuna images in k8s.io namespace"

# Check images in default namespace
CORE_IN_DEFAULT=$(ctr images ls 2>/dev/null | grep -c "fortuna-core:latest" || echo "0")
AGENT_IN_DEFAULT=$(ctr images ls 2>/dev/null | grep -c "fortuna-agent:latest" || echo "0")

if [ "$CORE_IN_DEFAULT" -gt 0 ] || [ "$AGENT_IN_DEFAULT" -gt 0 ]; then
    log_warning "Images found in default namespace but not in k8s.io namespace"
    log_info "Copying images to k8s.io namespace..."
    
    if [ "$CORE_IN_DEFAULT" -gt 0 ] && [ "$CORE_IN_K8S" -eq 0 ]; then
        log_info "Copying fortuna-core:latest to k8s.io namespace..."
        # Get image digest from default namespace
        IMAGE_DIGEST=$(ctr images ls | grep "fortuna-core:latest" | awk '{print $3}' | head -1)
        if [ -n "$IMAGE_DIGEST" ]; then
            # Tag image in k8s.io namespace
            ctr -n k8s.io images tag "docker.io/library/fortuna-core:latest@${IMAGE_DIGEST}" "docker.io/library/fortuna-core:latest" || {
                log_warning "Failed to tag, trying export/import method..."
                # Export and import
                TEMP_FILE=$(mktemp)
                ctr images export "$TEMP_FILE" "docker.io/library/fortuna-core:latest" 2>/dev/null && \
                ctr -n k8s.io images import "$TEMP_FILE" && \
                rm -f "$TEMP_FILE" || log_error "Failed to copy Core image"
            }
        fi
    fi
    
    if [ "$AGENT_IN_DEFAULT" -gt 0 ] && [ "$AGENT_IN_K8S" -eq 0 ]; then
        log_info "Copying fortuna-agent:latest to k8s.io namespace..."
        IMAGE_DIGEST=$(ctr images ls | grep "fortuna-agent:latest" | awk '{print $3}' | head -1)
        if [ -n "$IMAGE_DIGEST" ]; then
            ctr -n k8s.io images tag "docker.io/library/fortuna-agent:latest@${IMAGE_DIGEST}" "docker.io/library/fortuna-agent:latest" || {
                log_warning "Failed to tag, trying export/import method..."
                TEMP_FILE=$(mktemp)
                ctr images export "$TEMP_FILE" "docker.io/library/fortuna-agent:latest" 2>/dev/null && \
                ctr -n k8s.io images import "$TEMP_FILE" && \
                rm -f "$TEMP_FILE" || log_error "Failed to copy Agent image"
            }
        fi
    fi
fi

# Final check
echo ""
log_info "Final status - Images in k8s.io namespace:"
ctr -n k8s.io images ls | grep fortuna || log_warning "No fortuna images found"

# Check if images are accessible
CORE_FINAL=$(ctr -n k8s.io images ls | grep -c "fortuna-core:latest" || echo "0")
AGENT_FINAL=$(ctr -n k8s.io images ls | grep -c "fortuna-agent:latest" || echo "0")

if [ "$CORE_FINAL" -gt 0 ] && [ "$AGENT_FINAL" -gt 0 ]; then
    log_success "Both images are available in k8s.io namespace"
    log_info "Kubernetes should now be able to use these images"
else
    log_warning "Some images are still missing"
    if [ "$CORE_FINAL" -eq 0 ]; then
        log_info "Core image missing. Try:"
        log_info "  ./scripts/load-images-to-containerd.sh"
    fi
    if [ "$AGENT_FINAL" -eq 0 ]; then
        log_info "Agent image missing. Try:"
        log_info "  ./scripts/load-images-to-containerd.sh"
    fi
fi

echo ""

