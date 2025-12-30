#!/bin/bash

# ============================================================================
# Copy Containerd Images to All K8s Nodes
# ============================================================================
# Exports images from current node and imports them on all cluster nodes
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Configuration
IMAGE_PREFIX="${IMAGE_PREFIX:-fortuna}"
VERSION="${VERSION:-latest}"
NAMESPACE="${CONTAINERD_NAMESPACE:-k8s.io}"
TEMP_DIR="${TEMP_DIR:-/tmp/fortuna-images}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo "=========================================="
echo "Copy Containerd Images to All Nodes"
echo "=========================================="
echo ""

# Check prerequisites
if ! command -v kubectl >/dev/null 2>&1; then
    echo -e "${RED}❌ kubectl not found${NC}"
    exit 1
fi

if ! command -v nerdctl >/dev/null 2>&1; then
    echo -e "${RED}❌ nerdctl not found${NC}"
    exit 1
fi

# Get all nodes
echo -e "${BLUE}Getting cluster nodes...${NC}"
NODES=$(kubectl get nodes -o jsonpath='{.items[*].metadata.name}')
if [ -z "$NODES" ]; then
    echo -e "${RED}❌ No nodes found${NC}"
    exit 1
fi

NODE_COUNT=$(echo "$NODES" | wc -w)
echo -e "${GREEN}✅${NC} Found $NODE_COUNT node(s): $NODES"
echo ""

# Step 1: Export images on current node
echo -e "${BLUE}Step 1: Exporting images from current node...${NC}"
mkdir -p "$TEMP_DIR"

CORE_IMAGE="${IMAGE_PREFIX}-core:${VERSION}"
AGENT_IMAGE="${IMAGE_PREFIX}-agent:${VERSION}"

# Export Core
CORE_EXPORT="${TEMP_DIR}/${IMAGE_PREFIX}-core-${VERSION}.tar"
if nerdctl --namespace "${NAMESPACE}" images ls | grep -q "${CORE_IMAGE}"; then
    echo "Exporting ${CORE_IMAGE}..."
    nerdctl --namespace "${NAMESPACE}" save -o "$CORE_EXPORT" "${CORE_IMAGE}"
    if [ -f "$CORE_EXPORT" ]; then
        CORE_SIZE=$(du -h "$CORE_EXPORT" | cut -f1)
        echo -e "${GREEN}✅${NC} Core exported: $CORE_EXPORT ($CORE_SIZE)"
    else
        echo -e "${RED}❌${NC} Failed to export Core image"
        exit 1
    fi
else
    echo -e "${RED}❌${NC} Core image not found: ${CORE_IMAGE}"
    exit 1
fi

# Export Agent
AGENT_EXPORT="${TEMP_DIR}/${IMAGE_PREFIX}-agent-${VERSION}.tar"
if nerdctl --namespace "${NAMESPACE}" images ls | grep -q "${AGENT_IMAGE}"; then
    echo "Exporting ${AGENT_IMAGE}..."
    nerdctl --namespace "${NAMESPACE}" save -o "$AGENT_EXPORT" "${AGENT_IMAGE}"
    if [ -f "$AGENT_EXPORT" ]; then
        AGENT_SIZE=$(du -h "$AGENT_EXPORT" | cut -f1)
        echo -e "${GREEN}✅${NC} Agent exported: $AGENT_EXPORT ($AGENT_SIZE)"
    else
        echo -e "${RED}❌${NC} Failed to export Agent image"
        exit 1
    fi
else
    echo -e "${RED}❌${NC} Agent image not found: ${AGENT_IMAGE}"
    exit 1
fi

echo ""

# Step 2: Copy and import to each node
echo -e "${BLUE}Step 2: Copying and importing images to nodes...${NC}"

for node in $NODES; do
    echo "----------------------------------------"
    echo "Processing node: $node"
    
    # Check if node is current node
    CURRENT_NODE=$(hostname 2>/dev/null || echo "")
    if [ "$node" = "$CURRENT_NODE" ]; then
        echo -e "${YELLOW}⚠️${NC}  Skipping current node"
        continue
    fi
    
    # Copy files to node
    echo "Copying image files to $node..."
    if kubectl cp "$CORE_EXPORT" "${node}:${CORE_EXPORT}" 2>/dev/null; then
        echo -e "${GREEN}✅${NC} Core image copied"
    else
        echo -e "${RED}❌${NC} Failed to copy Core image"
        continue
    fi
    
    if kubectl cp "$AGENT_EXPORT" "${node}:${AGENT_EXPORT}" 2>/dev/null; then
        echo -e "${GREEN}✅${NC} Agent image copied"
    else
        echo -e "${RED}❌${NC} Failed to copy Agent image"
        continue
    fi
    
    # Import images on node
    echo "Importing images on $node..."
    
    # Import Core
    if kubectl exec "$node" -- ctr -n "${NAMESPACE}" images import "$CORE_EXPORT" 2>/dev/null; then
        echo -e "${GREEN}✅${NC} Core image imported on $node"
    else
        echo -e "${RED}❌${NC} Failed to import Core image on $node"
    fi
    
    # Import Agent
    if kubectl exec "$node" -- ctr -n "${NAMESPACE}" images import "$AGENT_EXPORT" 2>/dev/null; then
        echo -e "${GREEN}✅${NC} Agent image imported on $node"
    else
        echo -e "${RED}❌${NC} Failed to import Agent image on $node"
    fi
    
    # Cleanup on remote node
    echo "Cleaning up on $node..."
    kubectl exec "$node" -- rm -f "$CORE_EXPORT" "$AGENT_EXPORT" 2>/dev/null || true
    
    echo ""
done

# Step 3: Verify images on all nodes
echo -e "${BLUE}Step 3: Verifying images on all nodes...${NC}"
echo ""

for node in $NODES; do
    echo "Node: $node"
    if kubectl exec "$node" -- ctr -n "${NAMESPACE}" images ls 2>/dev/null | grep -q "${IMAGE_PREFIX}"; then
        echo -e "${GREEN}✅${NC} Images found:"
        kubectl exec "$node" -- ctr -n "${NAMESPACE}" images ls 2>/dev/null | grep "${IMAGE_PREFIX}" || true
    else
        echo -e "${RED}❌${NC} No images found"
    fi
    echo ""
done

# Cleanup local temp files
echo -e "${BLUE}Cleaning up local temp files...${NC}"
rm -f "$CORE_EXPORT" "$AGENT_EXPORT"
rmdir "$TEMP_DIR" 2>/dev/null || true

echo ""
echo "=========================================="
echo -e "${GREEN}✅ Image distribution complete!${NC}"
echo "=========================================="
echo ""

