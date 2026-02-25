#!/bin/bash

# Push Images to Worker Nodes Script
# Exports fortuna-core and fortuna-agent from local containerd and imports on WORKER_NODES
# so that all nodes (including workers) have the image (Agent DaemonSet, Core on master).
#
# SSH: Set SSH_USER (e.g. root, k8s) and optionally SSH_PASS for sshpass.
#      If SSH_USER is empty, tries SSH_TRY_USERS (default: $USER root k8s).
#      WORKER_NODES defaults to all node IPs from kubectl; set to limit (e.g. "192.168.56.101").
#
# Called automatically by: full-clean-database-rebuild-deploy.sh (Phase 2b), deploy-fortuna-robust.sh (Step 5b when multi-node).

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Default image tags from deploy YAML so they match imagePullPolicy
detect_core_image() {
    if [ -f "$PROJECT_ROOT/deploy/fortuna-core-deployment.yaml" ]; then
        grep -E '^\s+image:\s+fortuna-core:' "$PROJECT_ROOT/deploy/fortuna-core-deployment.yaml" | sed -E 's/.*image:\s+//' | tr -d ' \r' | head -1
    fi
}
detect_agent_image() {
    if [ -f "$PROJECT_ROOT/deploy/fortuna-agent-daemonset.yaml" ]; then
        grep -E '^\s+image:\s+fortuna-agent:' "$PROJECT_ROOT/deploy/fortuna-agent-daemonset.yaml" | sed -E 's/.*image:\s+//' | tr -d ' \r' | head -1
    fi
}
detect_all_node_ips() {
    kubectl get nodes -o jsonpath='{.items[*].status.addresses[?(@.type=="InternalIP")].address}' 2>/dev/null | tr ' ' '\n' | sort -u
}

# Configuration (defaults match deploy YAML and all nodes so Core on master gets image)
_detected_core=$(detect_core_image)
_detected_agent=$(detect_agent_image)
_detected_nodes=$(detect_all_node_ips)
CORE_IMAGE="${CORE_IMAGE:-${_detected_core:-fortuna-core:latest}}"
AGENT_IMAGE="${AGENT_IMAGE:-${_detected_agent:-fortuna-agent:latest}}"
# Include all nodes (master + workers); Core runs on control-plane and needs the image there
WORKER_NODES="${WORKER_NODES:-${_detected_nodes:-192.168.56.100 192.168.56.101}}"
# SSH_USER: set to root, k8s, or leave empty to try current user ($USER). SSH_PASS for sshpass (optional).
SSH_USER="${SSH_USER:-}"
SSH_PASS="${SSH_PASS:-}"
# Try these users in order if SSH_USER is empty and connection fails
SSH_TRY_USERS="${SSH_TRY_USERS:-$USER root k8s}"
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

# Check if image exists in local containerd (ctr or nerdctl namespace k8s.io)
image_exists() {
    local img="$1"
    (ctr -n k8s.io images list 2>/dev/null; nerdctl --namespace k8s.io images list 2>/dev/null) | grep -q "$img"
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    if ! image_exists "$CORE_IMAGE"; then
        log_error "Core image not found: $CORE_IMAGE"
        log_info "Build first: ./scripts/build/build-and-load-containerd.sh (from $PROJECT_ROOT)"
        return 1
    fi
    
    if ! image_exists "$AGENT_IMAGE"; then
        log_error "Agent image not found: $AGENT_IMAGE"
        log_info "Build first: ./scripts/build/build-and-load-containerd.sh (from $PROJECT_ROOT)"
        return 1
    fi
    
    if [ -z "$WORKER_NODES" ]; then
        log_error "WORKER_NODES is empty (could not get from kubectl get nodes)"
        return 1
    fi
    
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
    
    # Export using ctr (containerd)
    if ctr -n k8s.io images export "$output_file" "$image_name" 2>/dev/null; then
        log_success "Image exported: $output_file"
        return 0
    # Fallback to nerdctl
    elif nerdctl --namespace k8s.io save -o "$output_file" "$image_name" 2>/dev/null; then
        log_success "Image exported: $output_file"
        return 0
    else
        log_error "Failed to export image: $image_name"
        return 1
    fi
}

# Build SSH target string (user@worker or worker)
_ssh_target() {
    local worker=$1
    local user=${2:-}
    if [ -n "$user" ]; then
        echo "$user@$worker"
    else
        echo "$worker"
    fi
}

# Copy file to worker node (logs to stderr so caller can capture only remote_path)
# Tries SSH_USER, then if empty tries SSH_TRY_USERS. Shows actual error on failure.
copy_to_worker() {
    local file=$1
    local worker=$2
    local remote_path="/tmp/$(basename $file)"
    local target err

    _do_scp() {
        local tgt=$1
        if command -v sshpass &> /dev/null && [ -n "$SSH_PASS" ]; then
            sshpass -p "$SSH_PASS" scp -o StrictHostKeyChecking=no -o ConnectTimeout=10 "$file" "$tgt:$remote_path" 2>/dev/null
        else
            scp -o StrictHostKeyChecking=no -o ConnectTimeout=10 "$file" "$tgt:$remote_path" 2>/dev/null
        fi
    }

    if [ -n "$SSH_USER" ]; then
        target=$(_ssh_target "$worker" "$SSH_USER")
        log_info "Copying $file to $target:$remote_path" >&2
        if _do_scp "$target"; then
            log_success "File copied to $worker" >&2
            echo "$remote_path"
            return 0
        fi
        err=$(command -v sshpass &>/dev/null && [ -n "$SSH_PASS" ] && sshpass -p "$SSH_PASS" scp -o StrictHostKeyChecking=no -o ConnectTimeout=5 "$file" "$target:$remote_path" 2>&1 || scp -o StrictHostKeyChecking=no -o ConnectTimeout=5 "$file" "$target:$remote_path" 2>&1)
        log_error "SCP failed for $target: ${err:-connection or permission denied}" >&2
        return 1
    fi

    for u in $SSH_TRY_USERS; do
        [ -z "$u" ] && continue
        target=$(_ssh_target "$worker" "$u")
        log_info "Copying $file to $target:$remote_path" >&2
        if _do_scp "$target"; then
            log_success "File copied to $worker" >&2
            echo "$remote_path"
            return 0
        fi
    done
    err=$(scp -o StrictHostKeyChecking=no -o ConnectTimeout=5 "$file" "$worker:$remote_path" 2>&1)
    log_error "SCP failed for $worker (tried users: $SSH_TRY_USERS). Last error: ${err:-no success}" >&2
    return 1
}

# Run command on worker via SSH (tries SSH_USER then SSH_TRY_USERS)
# If SSH_PASS is set and cmd contains "sudo", wrap as: echo SSH_PASS | sudo -S ...
_ssh_run() {
    local worker=$1
    shift
    local cmd="$*"
    local target run_cmd
    if [ -n "$SSH_PASS" ] && echo "$cmd" | grep -q "sudo"; then
        run_cmd="echo '$SSH_PASS' | sudo -S $(echo "$cmd" | sed 's/^sudo //')"
    else
        run_cmd="$cmd"
    fi
    if [ -n "$SSH_USER" ]; then
        target=$(_ssh_target "$worker" "$SSH_USER")
        if command -v sshpass &>/dev/null && [ -n "$SSH_PASS" ]; then
            sshpass -p "$SSH_PASS" ssh -o StrictHostKeyChecking=no -o ConnectTimeout=10 "$target" "$run_cmd" 2>/dev/null
        else
            ssh -o StrictHostKeyChecking=no -o ConnectTimeout=10 "$target" "$run_cmd" 2>/dev/null
        fi
        return $?
    fi
    for u in $SSH_TRY_USERS; do
        [ -z "$u" ] && continue
        target=$(_ssh_target "$worker" "$u")
        if command -v sshpass &>/dev/null && [ -n "$SSH_PASS" ]; then
            sshpass -p "$SSH_PASS" ssh -o StrictHostKeyChecking=no -o ConnectTimeout=10 "$target" "$run_cmd" 2>/dev/null
        else
            ssh -o StrictHostKeyChecking=no -o ConnectTimeout=10 "$target" "$run_cmd" 2>/dev/null
        fi
        [ $? -eq 0 ] && return 0
    done
    return 1
}

# Import image on worker node; after import, tag as fortuna-*:latest if needed (nerdctl exports as docker.io/library/...)
import_on_worker() {
    local worker=$1
    local remote_file=$2
    local image_name=$3
    
    remote_file=$(echo "$remote_file" | tr -d '\n' | xargs)
    
    log_info "Importing image on $worker: $image_name (file: $remote_file)"
    
    if ! _ssh_run "$worker" "sudo ctr -n k8s.io images import $remote_file"; then
        log_error "Failed to import image on $worker"
        return 1
    fi
    # Tag so Kubernetes finds it (ensure both refs exist: nerdctl uses docker.io/library/...)
    _ssh_run "$worker" "sudo ctr -n k8s.io images tag docker.io/library/$image_name $image_name" 2>/dev/null || true
    _ssh_run "$worker" "sudo ctr -n k8s.io images tag $image_name docker.io/library/$image_name" 2>/dev/null || true
    log_success "Image imported on $worker"
    return 0
}

# Cleanup remote file
cleanup_remote() {
    local worker=$1
    local remote_file=$2
    log_info "Cleaning up $remote_file on $worker"
    _ssh_run "$worker" "rm -f $remote_file" || true
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
    echo "Push Images to All Nodes (master + workers)"
    echo "=========================================="
    echo "  CORE_IMAGE:  $CORE_IMAGE"
    echo "  AGENT_IMAGE: $AGENT_IMAGE"
    echo "  NODES:       $WORKER_NODES"
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
        echo ""
        log_info "Ensure SSH from this host to each node works. Example:"
        echo "  export SSH_USER=root   # or k8s, or leave unset to try \$USER, root, k8s"
        echo "  export SSH_PASS=yourpassword   # optional, for sshpass"
        echo "  export WORKER_NODES=\"192.168.56.101\"   # or omit to use all node IPs from kubectl"
        echo "  $SCRIPT_DIR/push-images-to-workers.sh"
        return 1
    fi
}

main "$@"


