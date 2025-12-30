#!/bin/bash

# ============================================================================
# Import Images to Containerd
# ============================================================================
# Imports Docker/OCI images into containerd for Kubernetes
# Supports both tar files and direct image names
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Configuration
NAMESPACE="${CONTAINERD_NAMESPACE:-k8s.io}"  # Default containerd namespace for K8s
IMPORT_METHOD="${IMPORT_METHOD:-ctr}"  # ctr or nerdctl

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

usage() {
    cat <<EOF
Usage: $0 [OPTIONS] <image-file-or-name>

Import images into containerd for Kubernetes.

Options:
    -n, --namespace NAMESPACE    Containerd namespace (default: k8s.io)
    -m, --method METHOD          Import method: ctr or nerdctl (default: ctr)
    -h, --help                   Show this help

Examples:
    # Import from tar file using ctr
    $0 /path/to/fortuna-core-v1.0.0.tar

    # Import from tar file using nerdctl
    $0 -m nerdctl /path/to/fortuna-core-v1.0.0.tar

    # Import from Docker registry (using nerdctl)
    $0 -m nerdctl docker.io/fortuna/core:v1.0.0

    # Import multiple files
    for file in *.tar; do $0 "\$file"; done
EOF
}

# Parse arguments
IMPORT_FILES=()
while [[ $# -gt 0 ]]; do
    case $1 in
        -n|--namespace)
            NAMESPACE="$2"
            shift 2
            ;;
        -m|--method)
            IMPORT_METHOD="$2"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        -*)
            echo -e "${RED}Unknown option: $1${NC}"
            usage
            exit 1
            ;;
        *)
            IMPORT_FILES+=("$1")
            shift
            ;;
    esac
done

if [ ${#IMPORT_FILES[@]} -eq 0 ]; then
    echo -e "${RED}Error: No image file or name specified${NC}"
    usage
    exit 1
fi

# Check prerequisites
check_prerequisites() {
    echo -e "${BLUE}Checking prerequisites...${NC}"
    
    if [ "$IMPORT_METHOD" = "ctr" ]; then
        if ! command -v ctr >/dev/null 2>&1; then
            echo -e "${RED}❌ ctr not found${NC}"
            exit 1
        fi
        echo -e "${GREEN}✅${NC} ctr found"
    elif [ "$IMPORT_METHOD" = "nerdctl" ]; then
        if ! command -v nerdctl >/dev/null 2>&1; then
            echo -e "${RED}❌ nerdctl not found${NC}"
            exit 1
        fi
        echo -e "${GREEN}✅${NC} nerdctl found"
    else
        echo -e "${RED}❌ Invalid import method: ${IMPORT_METHOD}${NC}"
        echo "Use 'ctr' or 'nerdctl'"
        exit 1
    fi
    
    # Check containerd socket
    if [ -S /run/containerd/containerd.sock ]; then
        echo -e "${GREEN}✅${NC} containerd.sock found"
    elif [ -S /var/run/containerd/containerd.sock ]; then
        export CONTAINERD_ADDRESS=/var/run/containerd/containerd.sock
        echo -e "${GREEN}✅${NC} containerd.sock found at /var/run/containerd/containerd.sock"
    else
        echo -e "${RED}❌${NC} Cannot find containerd socket"
        exit 1
    fi
    
    echo ""
}

# Import image using ctr
import_with_ctr() {
    local file=$1
    
    echo -e "${BLUE}Importing ${file} using ctr...${NC}"
    
    if [ ! -f "$file" ]; then
        echo -e "${RED}❌ File not found: ${file}${NC}"
        return 1
    fi
    
    # Import image
    if ctr -n "${NAMESPACE}" images import "$file"; then
        echo -e "${GREEN}✅${NC} Successfully imported ${file}"
        
        # List imported images
        echo "Imported images:"
        ctr -n "${NAMESPACE}" images ls | grep -E "fortuna|ksam" || true
        echo ""
        return 0
    else
        echo -e "${RED}❌${NC} Failed to import ${file}"
        return 1
    fi
}

# Import image using nerdctl
import_with_nerdctl() {
    local file=$1
    
    echo -e "${BLUE}Importing ${file} using nerdctl...${NC}"
    
    # Check if it's a file or image name
    if [ -f "$file" ]; then
        # It's a tar file
        if nerdctl --namespace "${NAMESPACE}" load -i "$file"; then
            echo -e "${GREEN}✅${NC} Successfully imported ${file}"
            
            # List imported images
            echo "Imported images:"
            nerdctl --namespace "${NAMESPACE}" images ls | grep -E "fortuna|ksam" || true
            echo ""
            return 0
        else
            echo -e "${RED}❌${NC} Failed to import ${file}"
            return 1
        fi
    else
        # It's an image name, pull it
        echo -e "${BLUE}Pulling image ${file}...${NC}"
        if nerdctl --namespace "${NAMESPACE}" pull "$file"; then
            echo -e "${GREEN}✅${NC} Successfully pulled ${file}"
            
            # List imported images
            echo "Imported images:"
            nerdctl --namespace "${NAMESPACE}" images ls | grep -E "fortuna|ksam" || true
            echo ""
            return 0
        else
            echo -e "${RED}❌${NC} Failed to pull ${file}"
            return 1
        fi
    fi
}

# Main import process
main() {
    check_prerequisites
    
    echo "=========================================="
    echo "Import Images to Containerd"
    echo "=========================================="
    echo ""
    echo "Configuration:"
    echo "  Namespace:  ${NAMESPACE}"
    echo "  Method:     ${IMPORT_METHOD}"
    echo "  Files:      ${#IMPORT_FILES[@]}"
    echo ""
    
    SUCCESS=0
    FAILED=0
    
    for file in "${IMPORT_FILES[@]}"; do
        echo "----------------------------------------"
        if [ "$IMPORT_METHOD" = "ctr" ]; then
            if import_with_ctr "$file"; then
                ((SUCCESS++))
            else
                ((FAILED++))
            fi
        else
            if import_with_nerdctl "$file"; then
                ((SUCCESS++))
            else
                ((FAILED++))
            fi
        fi
    done
    
    echo "=========================================="
    echo "Import Summary"
    echo "=========================================="
    echo -e "${GREEN}✅ Successful: ${SUCCESS}${NC}"
    if [ $FAILED -gt 0 ]; then
        echo -e "${RED}❌ Failed: ${FAILED}${NC}"
    fi
    echo ""
    
    if [ $FAILED -eq 0 ]; then
        echo "List all images:"
        if [ "$IMPORT_METHOD" = "ctr" ]; then
            echo "  ctr -n ${NAMESPACE} images ls | grep fortuna"
        else
            echo "  nerdctl --namespace ${NAMESPACE} images ls | grep fortuna"
        fi
        echo ""
        exit 0
    else
        exit 1
    fi
}

# Run main
main

