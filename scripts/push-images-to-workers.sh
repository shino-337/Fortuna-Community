#!/bin/bash

# Push Images to Worker Nodes Script
# Automatically exports and pushes Fortuna images to all worker nodes

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
CORE_IMAGE="${CORE_IMAGE:-fortuna-core:latest}"
AGENT_IMAGE="${AGENT_IMAGE:-fortuna/agent:latest}"
WORKER_NODES="${WORKER_NODES:-k8s-worker01}"
SSH_USER="${SSH_USER:-k8s}"
SSH_PASS="${SSH_PASS:-k8s}"
TEMP_DIR="${TEMP_DIR:-/tmp/fortuna-images}"

# Functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    # Check if images exist
    if ! ctr -n k8s.io images list | grep -q "$CORE_IMAGE"; then
        log_error "Core image not found: $CORE_IMAGE"
        log_info "Please build images first: bash scripts/build-and-load-containerd.sh"
        return 1
    fi
    
    if ! ctr -n k8s.io images list | grep -q "$AGENT_IMAGE"; then
        log_error "Agent image not found: $AGENT_IMAGE"
        log_info "Please build images first: bash scripts/build-and-load-containerd.sh"
        return 1
    fi
    
    # Check if sshpass is available (for password-based SSH)
    if ! command -v sshpass &> /dev/null; then
        log_warning "sshpass not found. Install with: sudo apt-get install sshpass (or use SSH keys)"
        log_info "Will attempt to use SSH keys instead"
    fi
    
    log_success "Prerequisites check passed"
    return 0
}

# Export image to tar file
export_image() {
    local image_name=$1
    local output_file=$2
    
    log_info "Exporting image: $image_name -> $output_file"
    
    # Create temp directory
    mkdir -p "$TEMP_DIR"
    
    # Export using nerdctl
    if nerdctl --namespace k8s.io save -o "$output_file" "$image_name" 2>/dev/null; then
        log_success "Image exported: $output_file"
        return 0
    else
        log_error "Failed to export image: $image_name"
        return 1
    fi
}

# Copy file to worker node
copy_to_worker() {
    local file=$1
    local worker=$2
    local remote_path="/tmp/$(basename $file)"
    
    log_info "Copying $file to $worker:$remote_path"
    
    # Try with sshpass if available, otherwise use SSH keys
    if command -v sshpass &> /dev/null && [ -n "$SSH_PASS" ]; then
        if sshpass -p "$SSH_PASS" scp -o StrictHostKeyChecking=no "$file" "$SSH_USER@$worker:$remote_path" 2>/dev/null; then
            log_success "File copied to $worker"
            echo "$remote_path"
            return 0
        fi
    fi
    
    # Fallback to SSH keys
    if scp -o StrictHostKeyChecking=no "$file" "$SSH_USER@$worker:$remote_path" 2>/dev/null; then
        log_success "File copied to $worker"
        echo "$remote_path"
        return 0
    else
        log_error "Failed to copy file to $worker"
        return 1
    fi
}

# Import image on worker node
import_on_worker() {
    local worker=$1
    local remote_file=$2
    local image_name=$3
    
    log_info "Importing image on $worker: $image_name"
    
    # Try with sshpass if available, otherwise use SSH keys
    if command -v sshpass &> /dev/null && [ -n "$SSH_PASS" ]; then
        if sshpass -p "$SSH_PASS" ssh -o StrictHostKeyChecking=no "$SSH_USER@$worker" "sudo ctr -n k8s.io images import $remote_file && sudo ctr -n k8s.io images tag $image_name $image_name" 2>/dev/null; then
            log_success "Image imported on $worker"
            return 0
        fi
    fi
    
    # Fallback to SSH keys
    if ssh -o StrictHostKeyChecking=no "$SSH_USER@$worker" "sudo ctr -n k8s.io images import $remote_file && sudo ctr -n k8s.io images tag $image_name $image_name" 2>/dev/null; then
        log_success "Image imported on $worker"
        return 0
    else
        log_error "Failed to import image on $worker"
        return 1
    fi
}

# Cleanup remote file
cleanup_remote() {
    local worker=$1
    local remote_file=$2
    
    log_info "Cleaning up $remote_file on $worker"
    
    if command -v sshpass &> /dev/null && [ -n "$SSH_PASS" ]; then
        sshpass -p "$SSH_PASS" ssh -o StrictHostKeyChecking=no "$SSH_USER@$worker" "rm -f $remote_file" 2>/dev/null || true
    else
        ssh -o StrictHostKeyChecking=no "$SSH_USER@$worker" "rm -f $remote_file" 2>/dev/null || true
    fi
}

# Process single worker node
process_worker() {
    local worker=$1
    
    log_info "Processing worker node: $worker"
    
    # Export Core image
    local core_tar="$TEMP_DIR/fortuna-core.tar"
    if ! export_image "$CORE_IMAGE" "$core_tar"; then
        return 1
    fi
    
    # Export Agent image
    local agent_tar="$TEMP_DIR/fortuna-agent.tar"
    if ! export_image "$AGENT_IMAGE" "$agent_tar"; then
        return 1
    fi
    
    # Copy Core image to worker
    local core_remote=$(copy_to_worker "$core_tar" "$worker")
    if [ -z "$core_remote" ]; then
        return 1
    fi
    
    # Copy Agent image to worker
    local agent_remote=$(copy_to_worker "$agent_tar" "$worker")
    if [ -z "$agent_remote" ]; then
        cleanup_remote "$worker" "$core_remote"
        return 1
    fi
    
    # Import Core image on worker
    if ! import_on_worker "$worker" "$core_remote" "$CORE_IMAGE"; then
        cleanup_remote "$worker" "$core_remote"
        cleanup_remote "$worker" "$agent_remote"
        return 1
    fi
    
    # Import Agent image on worker
    if ! import_on_worker "$worker" "$agent_remote" "$AGENT_IMAGE"; then
        cleanup_remote "$worker" "$core_remote"
        cleanup_remote "$worker" "$agent_remote"
        return 1
    fi
    
    # Cleanup remote files
    cleanup_remote "$worker" "$core_remote"
    cleanup_remote "$worker" "$agent_remote"
    
    log_success "Worker $worker processed successfully"
    return 0
}

# Main
main() {
    echo ""
    echo "=========================================="
    echo "Push Images to Worker Nodes"
    echo "=========================================="
    echo ""
    
    # Check prerequisites
    if ! check_prerequisites; then
        exit 1
    fi
    echo ""
    
    # Process each worker node
    local success_count=0
    local total_count=0
    
    for worker in $WORKER_NODES; do
        total_count=$((total_count + 1))
        if process_worker "$worker"; then
            success_count=$((success_count + 1))
        else
            log_error "Failed to process worker: $worker"
        fi
        echo ""
    done
    
    # Cleanup local temp files
    log_info "Cleaning up local temp files..."
    rm -rf "$TEMP_DIR"/*.tar 2>/dev/null || true
    
    # Summary
    echo ""
    echo "=========================================="
    echo "Summary"
    echo "=========================================="
    echo "Workers processed: $success_count/$total_count"
    
    if [ $success_count -eq $total_count ]; then
        log_success "All worker nodes processed successfully!"
        echo ""
        echo "Images are now available on all worker nodes:"
        for worker in $WORKER_NODES; do
            echo "  - $worker: $CORE_IMAGE, $AGENT_IMAGE"
        done
        return 0
    else
        log_error "Some worker nodes failed to process"
        return 1
    fi
}

main "$@"

