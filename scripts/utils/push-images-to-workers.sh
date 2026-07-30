#!/bin/bash

# Push Images to Worker Nodes Script
# Exports Fortuna Core/Agent images from local containerd and imports them on
# all nodes (master + workers) so registryless deployments can schedule runtime
# workloads where they are needed.
#
# Build tip: tag must match docker.io/library/fortuna-agent:latest in the same namespace
# this script exports from (typically default nerdctl namespace). For kubelet-visible images,
# build with: nerdctl -n k8s.io build -t docker.io/library/fortuna-agent:latest -f agent/Dockerfile .
# (from repo root). See agent/README.md "Build".
#
# Dashboard is not pushed by default. Local registryless deployments keep the
# dashboard on the control-plane/master where the image is built or loaded. Use
# --include-dashboard only when intentionally scheduling dashboard elsewhere.
#
# Config file: Set PUSH_CONFIG_FILE to path of a file with per-node credentials, or place
#   push-images.config in this directory (see push-images.config.example). Format:
#   MASTER_NODE=192.168.56.100
#   MASTER_SSH_USER=root
#   MASTER_SSH_PASS=
#   WORKER_NODES=192.168.56.101
#   WORKER_SSH_USER=k8s
#   WORKER_SSH_PASS=
# If no config file, uses env SSH_USER, SSH_PASS, WORKER_NODES (single credential for all nodes).
#
# REMOTE_TEMP_DIR: default /var/tmp/fortuna-images (avoids /tmp Permission denied).
#
# Options:
#   --clean-remote   On each node, remove existing fortuna* images from ctr -n k8s.io before pushing.
#   --clean-only     Only clean fortuna* images on all nodes (no export/push). Use after cleaning local and before rebuild+push.
#   --agent-only     Export/import only fortuna-agent. Use for remote Agent-only clusters.
#   --include-dashboard  Also export/import fortuna-dashboard (off by default).
#   --no-dashboard       Only export/import core + agent (default).
#   --build-if-missing   If Core/Agent (and dashboard when included) are missing in local k8s.io, run
#                        scripts/build/build-and-load-containerd.sh once before export. Env: AUTO_BUILD_IF_MISSING=1.
#
# Called automatically by: full-clean-database-rebuild-deploy.sh (Phase 2b), deploy-fortuna-robust.sh (Step 5b when multi-node).

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Load config file if present (per-node master/worker user and pass)
PUSH_CONFIG_FILE="${PUSH_CONFIG_FILE:-$SCRIPT_DIR/push-images.config}"
if [ -f "$PUSH_CONFIG_FILE" ]; then
    set -a
    # shellcheck source=/dev/null
    source "$PUSH_CONFIG_FILE" 2>/dev/null || true
    set +a
    # Strip Windows CRLF so passwords and IPs match (common when editing on Windows).
    _push_strip_cr() { printf '%s' "${1:-}" | tr -d '\r'; }
    MASTER_NODE=$(_push_strip_cr "${MASTER_NODE:-}")
    MASTER_SSH_USER=$(_push_strip_cr "${MASTER_SSH_USER:-}")
    MASTER_SSH_PASS=$(_push_strip_cr "${MASTER_SSH_PASS:-}")
    WORKER_SSH_USER=$(_push_strip_cr "${WORKER_SSH_USER:-}")
    WORKER_SSH_PASS=$(_push_strip_cr "${WORKER_SSH_PASS:-}")
    WORKER_NODES=$(_push_strip_cr "${WORKER_NODES:-}")
    SSH_USER=$(_push_strip_cr "${SSH_USER:-}")
    SSH_PASS=$(_push_strip_cr "${SSH_PASS:-}")
    # Build full node list: master + workers (so we push to all nodes with their own creds)
    if [ -n "${MASTER_NODE:-}" ] && [ -n "${WORKER_NODES:-}" ]; then
        case " $WORKER_NODES " in *" $MASTER_NODE "*) ;; *) WORKER_NODES="$MASTER_NODE $WORKER_NODES"; esac
    fi
fi

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
detect_dashboard_image() {
    # Dashboard deploy is not a daemonset; detect from the dashboard Deployment manifest.
    if [ -f "$PROJECT_ROOT/deploy/dashboard-deployment.yaml" ]; then
        grep -E "^\s+image:\s+fortuna-dashboard:" "$PROJECT_ROOT/deploy/dashboard-deployment.yaml" | sed -E "s/.*image:\s+//" | tr -d " \r" | head -1
    fi
}
detect_all_node_ips() {
    kubectl get nodes -o jsonpath='{.items[*].status.addresses[?(@.type=="InternalIP")].address}' 2>/dev/null | tr ' ' '\n' | sort -u
}

# Configuration (defaults match deploy YAML and all nodes so Core on master gets image).
# Default SSH_USER is 'k8s' (non-root); sudo is used remotely for ctr commands.
_detected_core=$(detect_core_image)
_detected_agent=$(detect_agent_image)
_detected_dashboard=$(detect_dashboard_image)
_detected_nodes=$(detect_all_node_ips)
CORE_IMAGE="${CORE_IMAGE:-${_detected_core:-fortuna-core:latest}}"
AGENT_IMAGE="${AGENT_IMAGE:-${_detected_agent:-fortuna-agent:latest}}"
# Optional dashboard image (only exported/imported when INCLUDE_DASHBOARD=true)
DASHBOARD_IMAGE="${DASHBOARD_IMAGE:-${_detected_dashboard:-fortuna-dashboard:latest}}"
# Include all nodes (master + workers); Core runs on control-plane and needs the image there
WORKER_NODES="${WORKER_NODES:-${_detected_nodes:-192.168.56.100 192.168.56.101}}"
# SSH_USER: set to k8s, root, or leave empty to try SSH_TRY_USERS. SSH_PASS for sshpass (optional).
SSH_USER="${SSH_USER:-k8s}"
SSH_PASS="${SSH_PASS:-}"
# Many configs set only WORKER_SSH_USER / WORKER_SSH_PASS for all nodes (no MASTER_* block).
# Without this, defaults stay SSH_USER=k8s and SSH_PASS empty and ssh/scp prompt for a password.
if [ -z "${MASTER_SSH_USER:-}" ]; then
    if [ -n "${WORKER_SSH_USER:-}" ]; then
        SSH_USER="$WORKER_SSH_USER"
    fi
    if [ -n "${WORKER_SSH_PASS:-}" ]; then
        SSH_PASS="$WORKER_SSH_PASS"
    fi
fi
# Try these users in order if SSH_USER is empty and connection fails
SSH_TRY_USERS="${SSH_TRY_USERS:-k8s $USER root}"
TEMP_DIR="${TEMP_DIR:-/tmp/fortuna-images}"
# Remote node: where to put tar files (default /var/tmp to avoid /tmp Permission denied on some nodes)
REMOTE_TEMP_DIR="${REMOTE_TEMP_DIR:-/var/tmp/fortuna-images}"
VERIFY_REMOTE_DIGEST="${VERIFY_REMOTE_DIGEST:-true}"

# Parse flags (before main)
CLEAN_REMOTE_IMAGES="${CLEAN_REMOTE_IMAGES:-false}"
CLEAN_ONLY="${CLEAN_ONLY:-false}"
INCLUDE_DASHBOARD="${INCLUDE_DASHBOARD:-false}"
PUSH_CORE="${PUSH_CORE:-true}"
PUSH_AGENT="${PUSH_AGENT:-true}"
BUILD_IF_MISSING=false
for arg in "$@"; do
    case "$arg" in
        --clean-remote) CLEAN_REMOTE_IMAGES=true ;;
        --clean-only)   CLEAN_ONLY=true ;;
        --agent-only)   PUSH_CORE=false; PUSH_AGENT=true; INCLUDE_DASHBOARD=false ;;
        --include-dashboard) INCLUDE_DASHBOARD=true ;;
        --no-dashboard) INCLUDE_DASHBOARD=false ;;
        --build-if-missing) BUILD_IF_MISSING=true ;;
    esac
done
if [ "${AUTO_BUILD_IF_MISSING:-0}" = "1" ] || [ "${AUTO_BUILD_IF_MISSING:-}" = "true" ]; then
    BUILD_IF_MISSING=true
fi

# Per-node credentials when config file sets MASTER_NODE + MASTER_SSH_USER
get_ssh_user_for_node() {
    local node="$1"
    if [ -n "${MASTER_NODE:-}" ] && [ -n "${MASTER_SSH_USER:-}" ]; then
        if [ "$node" = "$MASTER_NODE" ]; then
            echo "${MASTER_SSH_USER}"
        else
            echo "${WORKER_SSH_USER:-$SSH_USER}"
        fi
    else
        echo "${SSH_USER}"
    fi
}
get_ssh_pass_for_node() {
    local node="$1"
    if [ -n "${MASTER_NODE:-}" ] && [ -n "${MASTER_SSH_USER:-}" ]; then
        if [ "$node" = "$MASTER_NODE" ]; then
            echo "${MASTER_SSH_PASS:-$SSH_PASS}"
        else
            echo "${WORKER_SSH_PASS:-$SSH_PASS}"
        fi
    else
        echo "${SSH_PASS}"
    fi
}

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
    (ctr -n k8s.io images list 2>/dev/null; nerdctl --namespace k8s.io images 2>/dev/null) | grep -q "$img"
}

# When --build-if-missing or AUTO_BUILD_IF_MISSING=1: build locally before export (pipeline / deploy call this).
_maybe_build_missing_images() {
    [ "$BUILD_IF_MISSING" = true ] || return 0
    local need_build=false
    [ "$PUSH_CORE" = true ] && { image_exists "$CORE_IMAGE" || need_build=true; }
    [ "$PUSH_AGENT" = true ] && { image_exists "$AGENT_IMAGE" || need_build=true; }
    if [ "$INCLUDE_DASHBOARD" = true ]; then
        if ! image_exists "$DASHBOARD_IMAGE" && ! image_exists "fortuna-dashboard:latest"; then
            need_build=true
        fi
    fi
    [ "$need_build" = true ] || return 0
    local bs="$PROJECT_ROOT/scripts/build/build-and-load-containerd.sh"
    if [ ! -x "$bs" ]; then
        log_error "Images missing and auto-build requested but $bs is missing or not executable"
        return 1
    fi
    log_info "Local images missing; running build-and-load-containerd.sh (build-if-missing)..."
    local build_env=()
    if [ "$PUSH_CORE" != true ] && [ "$PUSH_AGENT" = true ]; then
        build_env+=(BUILD_AGENT_ONLY=true)
    fi
    ( cd "$PROJECT_ROOT" && env "${build_env[@]}" "$bs" ) || {
        log_error "build-and-load-containerd.sh failed (--build-if-missing)"
        return 1
    }
    return 0
}

resolve_export_ref() {
    local img="$1"
    local canonical="docker.io/library/$img"
    if image_exists "$canonical"; then
        echo "$canonical"
    else
        echo "$img"
    fi
}

get_local_digest() {
    local ref="$1"
    ctr -n k8s.io images ls | awk -v r="$ref" '$1==r {print $3; exit}'
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."

    if ! _maybe_build_missing_images; then
        return 1
    fi

    if [ "$PUSH_CORE" = true ] && ! image_exists "$CORE_IMAGE"; then
        log_error "Core image not found: $CORE_IMAGE"
        log_info "Build first: ./scripts/build/build-and-load-containerd.sh (from $PROJECT_ROOT)"
        return 1
    fi
    
    if [ "$PUSH_AGENT" = true ] && ! image_exists "$AGENT_IMAGE"; then
        log_error "Agent image not found: $AGENT_IMAGE"
        log_info "Build first: ./scripts/build/build-and-load-containerd.sh (from $PROJECT_ROOT)"
        return 1
    fi
    
    if [ "$INCLUDE_DASHBOARD" = "true" ]; then
        if ! image_exists "$DASHBOARD_IMAGE"; then
            if image_exists "fortuna-dashboard:latest"; then
                log_warning "Dashboard image not found: $DASHBOARD_IMAGE — using fortuna-dashboard:latest (pin deploy/dashboard-deployment.yaml to :latest after rebuild)"
                DASHBOARD_IMAGE="fortuna-dashboard:latest"
            else
                log_error "Dashboard image not found: $DASHBOARD_IMAGE (and fortuna-dashboard:latest missing)"
                log_info "Build first: ./scripts/build/build-and-load-containerd.sh (from $PROJECT_ROOT)"
                return 1
            fi
        fi
    fi
    
    if [ -z "$WORKER_NODES" ]; then
        log_error "WORKER_NODES is empty (could not get from kubectl get nodes)"
        return 1
    fi
    
    if ! command -v sshpass &> /dev/null; then
        log_warning "sshpass not found. Install with: sudo apt-get install sshpass (or use SSH keys)"
        log_info "Will attempt to use SSH keys instead"
        if [ -n "${MASTER_SSH_PASS:-}${WORKER_SSH_PASS:-}${SSH_PASS:-}" ]; then
            log_error "Passwords are set in config/env but sshpass is missing; SSH/SCP will prompt interactively and may hang."
            log_info "Install: sudo apt-get install -y sshpass   (or remove *_SSH_PASS and use SSH keys)"
            return 1
        fi
    fi
    
    log_success "Prerequisites check passed"
    return 0
}

# Export image to tar file
export_image() {
    local image_name=$1
    local output_file=$2
    local export_ref
    export_ref=$(resolve_export_ref "$image_name")
    
    log_info "Exporting image: $export_ref -> $output_file"
    
    # Create temp directory
    mkdir -p "$TEMP_DIR"
    
    _valid_image_tar() {
        tar -tf "$output_file" >/dev/null 2>&1
    }

    # Export using ctr (containerd) first; then nerdctl (same namespace k8s.io).
    # Validate the tar because ctr can occasionally leave a truncated archive while
    # still returning success when the source image is being updated concurrently.
    if ctr -n k8s.io images export "$output_file" "$export_ref" 2>/dev/null; then
        if _valid_image_tar; then
            log_success "Image exported: $output_file"
            return 0
        fi
        log_warning "ctr export produced an invalid tar for $image_name; retrying with nerdctl save"
        rm -f "$output_file"
    fi
    if nerdctl --namespace k8s.io save -o "$output_file" "$export_ref" 2>/dev/null; then
        if _valid_image_tar; then
            log_success "Image exported: $output_file"
            return 0
        fi
        log_error "nerdctl save produced an invalid tar for $image_name"
        rm -f "$output_file"
        return 1
    fi
    log_error "Failed to export image: $image_name (tried ctr and nerdctl)"
    log_info "Run on this host: ctr -n k8s.io images ls | grep fortuna  (or nerdctl -n k8s.io images | grep fortuna)"
    return 1
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
# With config file: uses per-node user/pass (get_ssh_user_for_node, get_ssh_pass_for_node).
# Otherwise tries SSH_USER, then SSH_TRY_USERS. Ensures REMOTE_TEMP_DIR exists before SCP.
copy_to_worker() {
    local file=$1
    local worker=$2
    local remote_path="$REMOTE_TEMP_DIR/$(basename "$file")"
    local target err
    local node_user node_pass
    node_user=$(get_ssh_user_for_node "$worker")
    node_pass=$(get_ssh_pass_for_node "$worker")

    _do_scp() {
        local tgt=$1
        local p=${2:-$SSH_PASS}
        local opts=(-o StrictHostKeyChecking=no -o ConnectTimeout=10)
        if command -v sshpass &> /dev/null && [ -n "$p" ]; then
            sshpass -p "$p" scp "${opts[@]}" "$file" "$tgt:$remote_path" 2>/dev/null
        else
            scp "${opts[@]}" -o BatchMode=yes "$file" "$tgt:$remote_path" 2>/dev/null
        fi
    }

    # Ensure remote directory exists (avoids Permission denied when /tmp is restricted)
    _ssh_run "$worker" "mkdir -p $REMOTE_TEMP_DIR" 2>/dev/null || true

    # Per-node config: single attempt with node_user/node_pass
    if [ -n "${MASTER_NODE:-}" ] && [ -n "${MASTER_SSH_USER:-}" ]; then
        target=$(_ssh_target "$worker" "$node_user")
        log_info "Copying $file to $target:$remote_path" >&2
        if _do_scp "$target" "$node_pass"; then
            log_success "File copied to $worker" >&2
            echo "$remote_path"
            return 0
        fi
        err=$(command -v sshpass &>/dev/null && [ -n "$node_pass" ] && sshpass -p "$node_pass" scp -o StrictHostKeyChecking=no -o ConnectTimeout=5 "$file" "$target:$remote_path" 2>&1 || scp -o StrictHostKeyChecking=no -o ConnectTimeout=5 -o BatchMode=yes "$file" "$target:$remote_path" 2>&1)
        log_error "SCP failed for $target: ${err:-connection or permission denied}" >&2
        return 1
    fi

    if [ -n "$SSH_USER" ]; then
        target=$(_ssh_target "$worker" "$SSH_USER")
        log_info "Copying $file to $target:$remote_path" >&2
        if _do_scp "$target"; then
            log_success "File copied to $worker" >&2
            echo "$remote_path"
            return 0
        fi
        err=$(command -v sshpass &>/dev/null && [ -n "$SSH_PASS" ] && sshpass -p "$SSH_PASS" scp -o StrictHostKeyChecking=no -o ConnectTimeout=5 "$file" "$target:$remote_path" 2>&1 || scp -o StrictHostKeyChecking=no -o ConnectTimeout=5 -o BatchMode=yes "$file" "$target:$remote_path" 2>&1)
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
    err=$(scp -o StrictHostKeyChecking=no -o ConnectTimeout=5 -o BatchMode=yes "$file" "$worker:$remote_path" 2>&1)
    log_error "SCP failed for $worker (tried users: $SSH_TRY_USERS). Last error: ${err:-no success}" >&2
    return 1
}

# Run command on worker via SSH. With config file uses per-node user/pass; else SSH_USER then SSH_TRY_USERS.
# If pass is set and cmd contains "sudo", wrap as: echo pass | sudo -S ...
_ssh_run() {
    local worker=$1
    shift
    local cmd="$*"
    local target run_cmd
    local node_user node_pass
    node_user=$(get_ssh_user_for_node "$worker")
    node_pass=$(get_ssh_pass_for_node "$worker")
    if [ -n "$node_pass" ] && echo "$cmd" | grep -q "sudo"; then
        run_cmd="echo '$node_pass' | sudo -S $(echo "$cmd" | sed 's/^sudo //')"
    else
        run_cmd="$cmd"
    fi
    # Per-node config: single attempt
    if [ -n "${MASTER_NODE:-}" ] && [ -n "${MASTER_SSH_USER:-}" ]; then
        target=$(_ssh_target "$worker" "$node_user")
        if command -v sshpass &>/dev/null && [ -n "$node_pass" ]; then
            sshpass -p "$node_pass" ssh -o StrictHostKeyChecking=no -o ConnectTimeout=10 "$target" "$run_cmd" 2>/dev/null
        else
            ssh -o StrictHostKeyChecking=no -o ConnectTimeout=10 -o BatchMode=yes "$target" "$run_cmd" 2>/dev/null
        fi
        return $?
    fi
    if [ -n "$SSH_USER" ]; then
        target=$(_ssh_target "$worker" "$SSH_USER")
        if command -v sshpass &>/dev/null && [ -n "$SSH_PASS" ]; then
            sshpass -p "$SSH_PASS" ssh -o StrictHostKeyChecking=no -o ConnectTimeout=10 "$target" "$run_cmd" 2>/dev/null
        else
            ssh -o StrictHostKeyChecking=no -o ConnectTimeout=10 -o BatchMode=yes "$target" "$run_cmd" 2>/dev/null
        fi
        return $?
    fi
    for u in $SSH_TRY_USERS; do
        [ -z "$u" ] && continue
        target=$(_ssh_target "$worker" "$u")
        if command -v sshpass &>/dev/null && [ -n "$SSH_PASS" ]; then
            sshpass -p "$SSH_PASS" ssh -o StrictHostKeyChecking=no -o ConnectTimeout=10 "$target" "$run_cmd" 2>/dev/null
        else
            ssh -o StrictHostKeyChecking=no -o ConnectTimeout=10 -o BatchMode=yes "$target" "$run_cmd" 2>/dev/null
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
    local expected_digest=${4:-}
    
    remote_file=$(echo "$remote_file" | tr -d '\n' | xargs)
    
    log_info "Importing image on $worker: $image_name (file: $remote_file)"
    
    if ! _ssh_run "$worker" "sudo ctr -n k8s.io images import $remote_file"; then
        log_error "Failed to import image on $worker"
        return 1
    fi
    # Tag so Kubernetes finds it (ensure both refs exist: nerdctl uses docker.io/library/...)
    _ssh_run "$worker" "sudo ctr -n k8s.io images tag docker.io/library/$image_name $image_name" 2>/dev/null || true
    _ssh_run "$worker" "sudo ctr -n k8s.io images tag $image_name docker.io/library/$image_name" 2>/dev/null || true

    if [ "$VERIFY_REMOTE_DIGEST" = "true" ] && [ -n "$expected_digest" ]; then
        local got_digest
        got_digest=$(_ssh_run "$worker" "sudo ctr -n k8s.io images ls | grep -E '^(docker.io/library/$image_name|$image_name)[[:space:]]' | head -n1 | tr -s ' ' | cut -d ' ' -f3")
        got_digest=$(echo "$got_digest" | tr -d '\r\n ')
        if [ "$got_digest" != "$expected_digest" ]; then
            log_error "Digest mismatch on $worker for $image_name: expected=$expected_digest got=${got_digest:-<empty>}"
            return 1
        fi
    fi
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

# Remove fortuna* images from containerd k8s.io on a remote node (avoids old layers/tags).
clean_remote_fortuna_images() {
    local worker=$1
    log_info "Cleaning old fortuna images on $worker (ctr -n k8s.io)..."
    # List and remove; xargs -r (GNU) skips if no input; fallback to true so we don't fail when no images
    if _ssh_run "$worker" "sudo ctr -n k8s.io images ls -q 2>/dev/null | grep -E 'fortuna' | xargs -r -I {} sudo ctr -n k8s.io images rm {} 2>/dev/null; true"; then
        log_success "Cleaned fortuna images on $worker"
    else
        log_warning "Clean on $worker had errors (continuing)"
    fi
}

# Process single worker node (core_tar/agent_tar may be empty in component-only modes)
process_worker() {
    local worker=$1
    local core_tar="$2"
    local agent_tar="$3"
    local dashboard_tar="${4:-}"
    
    log_info "Processing worker node: $worker"
    
    local core_remote=""
    if [ -n "$core_tar" ]; then
        core_remote=$(copy_to_worker "$core_tar" "$worker")
        if [ -z "$core_remote" ]; then
            return 1
        fi
    fi
    
    local agent_remote=""
    if [ -n "$agent_tar" ]; then
        agent_remote=$(copy_to_worker "$agent_tar" "$worker")
        if [ -z "$agent_remote" ]; then
            [ -n "$core_remote" ] && cleanup_remote "$worker" "$core_remote"
            return 1
        fi
    fi
    
    if [ -n "$core_remote" ]; then
        if ! import_on_worker "$worker" "$core_remote" "$CORE_IMAGE" "$expected_core_digest"; then
            cleanup_remote "$worker" "$core_remote"
            [ -n "$agent_remote" ] && cleanup_remote "$worker" "$agent_remote"
            return 1
        fi
    fi
    
    if [ -n "$agent_remote" ]; then
        if ! import_on_worker "$worker" "$agent_remote" "$AGENT_IMAGE" "$expected_agent_digest"; then
            [ -n "$core_remote" ] && cleanup_remote "$worker" "$core_remote"
            cleanup_remote "$worker" "$agent_remote"
            return 1
        fi
    fi
    
    # Optional: Import Dashboard image on worker
    if [ -n "$dashboard_tar" ]; then
        local dashboard_remote
        dashboard_remote=$(copy_to_worker "$dashboard_tar" "$worker")
        if [ -z "$dashboard_remote" ]; then
            [ -n "$core_remote" ] && cleanup_remote "$worker" "$core_remote"
            [ -n "$agent_remote" ] && cleanup_remote "$worker" "$agent_remote"
            return 1
        fi
        
        if ! import_on_worker "$worker" "$dashboard_remote" "$DASHBOARD_IMAGE" "$expected_dashboard_digest"; then
            [ -n "$core_remote" ] && cleanup_remote "$worker" "$core_remote"
            [ -n "$agent_remote" ] && cleanup_remote "$worker" "$agent_remote"
            cleanup_remote "$worker" "$dashboard_remote"
            return 1
        fi

        cleanup_remote "$worker" "$dashboard_remote"
    fi
    
    # Cleanup remote files
    [ -n "$core_remote" ] && cleanup_remote "$worker" "$core_remote"
    [ -n "$agent_remote" ] && cleanup_remote "$worker" "$agent_remote"
    
    log_success "Worker $worker processed successfully"
    return 0
}

# Main
main() {
    echo ""
    echo "=========================================="
    if [ "$CLEAN_ONLY" = "true" ]; then
        echo "Clean Old Fortuna Images on All Nodes"
    else
        echo "Push Images to All Nodes (master + workers)"
    fi
    echo "=========================================="
    echo "  NODES:            $WORKER_NODES"
    [ "$CLEAN_ONLY" = "false" ] && [ "$PUSH_CORE" = true ] && echo "  CORE_IMAGE:       $CORE_IMAGE"
    [ "$CLEAN_ONLY" = "false" ] && [ "$PUSH_AGENT" = true ] && echo "  AGENT_IMAGE:      $AGENT_IMAGE"
    if [ "$CLEAN_ONLY" = "false" ] && [ "$INCLUDE_DASHBOARD" = "true" ]; then
        echo "  DASHBOARD_IMAGE:  $DASHBOARD_IMAGE"
    fi
    echo "  CLEAN_REMOTE:     $CLEAN_REMOTE_IMAGES"
    echo "  CLEAN_ONLY:       $CLEAN_ONLY"
    echo ""
    
    if [ "$CLEAN_ONLY" = "true" ]; then
        for worker in $WORKER_NODES; do
            clean_remote_fortuna_images "$worker"
        done
        log_success "Remote clean done. Rebuild and push: ./scripts/build/build-and-load-containerd.sh && $SCRIPT_DIR/push-images-to-workers.sh"
        return 0
    fi
    
    # Check prerequisites (images must exist locally to push)
    if ! check_prerequisites; then
        exit 1
    fi
    echo ""
    
    # Export images ONCE on local machine (before cleaning any node).
    # If we clean first and this host is in WORKER_NODES (e.g. master), we would delete local images and export would fail.
    mkdir -p "$TEMP_DIR"
    core_tar=""
    agent_tar=""
    dashboard_tar=""
    log_info "Exporting images once (local)..."
    if [ "$PUSH_CORE" = true ]; then
        core_tar="$TEMP_DIR/fortuna-core.tar"
        if ! export_image "$CORE_IMAGE" "$core_tar"; then
            log_error "Export failed. Build images on this host first: ./scripts/build/build-and-load-containerd.sh"
            exit 1
        fi
    fi
    if [ "$PUSH_AGENT" = true ]; then
        agent_tar="$TEMP_DIR/fortuna-agent.tar"
        if ! export_image "$AGENT_IMAGE" "$agent_tar"; then
            log_error "Export failed. Build images on this host first: ./scripts/build/build-and-load-containerd.sh"
            exit 1
        fi
    fi
    if [ "$INCLUDE_DASHBOARD" = "true" ]; then
        dashboard_tar="$TEMP_DIR/fortuna-dashboard.tar"
        if ! export_image "$DASHBOARD_IMAGE" "$dashboard_tar"; then
            log_error "Export failed for dashboard. Build images on this host first: ./scripts/build/build-and-load-containerd.sh"
            exit 1
        fi
    fi
    log_success "Images exported to $TEMP_DIR"
    echo ""
    
    local expected_core_digest expected_agent_digest expected_dashboard_digest
    expected_core_digest=""
    expected_agent_digest=""
    [ "$PUSH_CORE" = true ] && expected_core_digest=$(get_local_digest "$(resolve_export_ref "$CORE_IMAGE")")
    [ "$PUSH_AGENT" = true ] && expected_agent_digest=$(get_local_digest "$(resolve_export_ref "$AGENT_IMAGE")")
    expected_dashboard_digest=""
    if [ "$INCLUDE_DASHBOARD" = "true" ]; then
        expected_dashboard_digest=$(get_local_digest "$(resolve_export_ref "$DASHBOARD_IMAGE")")
    fi

    # Process each worker node (clean if requested, then copy + import)
    local success_count=0
    local total_count=0
    
    for worker in $WORKER_NODES; do
        total_count=$((total_count + 1))
        if [ "$CLEAN_REMOTE_IMAGES" = "true" ]; then
            clean_remote_fortuna_images "$worker"
        fi
        if [ "$INCLUDE_DASHBOARD" = "true" ]; then
            if process_worker "$worker" "$core_tar" "$agent_tar" "$dashboard_tar"; then
                success_count=$((success_count + 1))
            else
                log_error "Failed to process worker: $worker"
            fi
        else
            if process_worker "$worker" "$core_tar" "$agent_tar"; then
                success_count=$((success_count + 1))
            else
                log_error "Failed to process worker: $worker"
            fi
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
            refs=""
            [ "$PUSH_CORE" = true ] && refs="${refs:+$refs, }$CORE_IMAGE"
            [ "$PUSH_AGENT" = true ] && refs="${refs:+$refs, }$AGENT_IMAGE"
            if [ "$INCLUDE_DASHBOARD" = "true" ]; then
                refs="${refs:+$refs, }$DASHBOARD_IMAGE"
                echo "  - $worker: $refs"
            else
                echo "  - $worker: $refs"
            fi
        done
        return 0
    else
        log_error "Some worker nodes failed to process"
        echo ""
        log_info "Ensure SSH from this host to each node works. Example:"
        echo "  export SSH_USER=root   # or k8s, or leave unset to try \$USER, root, k8s"
        echo "  export SSH_PASS=<ssh-password>   # optional, for sshpass"
        echo "  export WORKER_NODES=\"192.168.56.101\"   # or omit to use all node IPs from kubectl"
        echo "  export REMOTE_TEMP_DIR=/var/tmp/fortuna-images   # if SCP fails with 'Permission denied' on /tmp"
        echo "  $SCRIPT_DIR/push-images-to-workers.sh"
        return 1
    fi
}

main "$@"
