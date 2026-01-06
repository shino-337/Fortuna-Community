#!/bin/bash

# ============================================================================
# Load Docker Images to Containerd
# ============================================================================
# Exports images from Docker and imports into containerd for Kubernetes
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

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Check prerequisites
if ! command -v docker >/dev/null 2>&1; then
    log_error "docker is not installed"
    exit 1
fi

if ! command -v ctr >/dev/null 2>&1 && ! command -v nerdctl >/dev/null 2>&1; then
    log_error "ctr or nerdctl is not installed (required for containerd)"
    exit 1
fi

# Check if images exist in Docker
log_info "Checking Docker images..."
CORE_EXISTS=$(docker images fortuna-core:latest --format "{{.Repository}}:{{.Tag}}" 2>/dev/null | grep -c "fortuna-core:latest" || echo "0")
AGENT_EXISTS=$(docker images fortuna-agent:latest --format "{{.Repository}}:{{.Tag}}" 2>/dev/null | grep -c "fortuna-agent:latest" || echo "0")

if [ "$CORE_EXISTS" -eq 0 ] && [ "$AGENT_EXISTS" -eq 0 ]; then
    log_error "Images not found in Docker. Please build them first:"
    log_info "  docker build -f core/Dockerfile -t fortuna-core:latest ."
    log_info "  docker build -f agent/Dockerfile -t fortuna-agent:latest ."
    exit 1
fi

# Export images
TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

log_info "Exporting images from Docker..."

if [ "$CORE_EXISTS" -gt 0 ]; then
    log_info "Exporting fortuna-core:latest..."
    docker save fortuna-core:latest -o "$TEMP_DIR/fortuna-core.tar" || {
        log_error "Failed to export Core image"
        exit 1
    }
    log_success "Core image exported"
fi

if [ "$AGENT_EXISTS" -gt 0 ]; then
    log_info "Exporting fortuna-agent:latest..."
    docker save fortuna-agent:latest -o "$TEMP_DIR/fortuna-agent.tar" || {
        log_error "Failed to export Agent image"
        exit 1
    }
    log_success "Agent image exported"
fi

# Import into containerd
log_info "Importing images into containerd..."

# Use nerdctl if available (easier), otherwise use ctr
if command -v nerdctl >/dev/null 2>&1; then
    log_info "Using nerdctl to import images..."
    
    if [ "$CORE_EXISTS" -gt 0 ]; then
        log_info "Importing fortuna-core:latest..."
        nerdctl load -i "$TEMP_DIR/fortuna-core.tar" || {
            log_error "Failed to import Core image"
            exit 1
        }
        log_success "Core image imported"
    fi
    
    if [ "$AGENT_EXISTS" -gt 0 ]; then
        log_info "Importing fortuna-agent:latest..."
        nerdctl load -i "$TEMP_DIR/fortuna-agent.tar" || {
            log_error "Failed to import Agent image"
            exit 1
        }
        log_success "Agent image imported"
    fi
else
    log_info "Using ctr to import images into k8s.io namespace..."
    
    if [ "$CORE_EXISTS" -gt 0 ]; then
        log_info "Importing fortuna-core:latest..."
        ctr -n k8s.io images import "$TEMP_DIR/fortuna-core.tar" || {
            log_error "Failed to import Core image"
            exit 1
        }
        log_success "Core image imported"
    fi
    
    if [ "$AGENT_EXISTS" -gt 0 ]; then
        log_info "Importing fortuna-agent:latest..."
        ctr -n k8s.io images import "$TEMP_DIR/fortuna-agent.tar" || {
            log_error "Failed to import Agent image"
            exit 1
        }
        log_success "Agent image imported"
    fi
fi

# Verify images in containerd
log_info "Verifying images in containerd..."

if command -v nerdctl >/dev/null 2>&1; then
    log_info "Images in containerd:"
    nerdctl images | grep fortuna || log_warning "No fortuna images found"
else
    log_info "Images in containerd (k8s.io namespace):"
    ctr -n k8s.io images ls | grep fortuna || log_warning "No fortuna images found"
fi

echo ""
log_success "Images loaded successfully!"
log_info "Images are now available for Kubernetes pods"
echo ""

