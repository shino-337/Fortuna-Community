#!/usr/bin/env bash
# ============================================================================
# Ensure at least one node has label node-role.kubernetes.io/control-plane
# ============================================================================
# Core deployment uses nodeSelector: node-role.kubernetes.io/control-plane.
# If no node has this label (e.g. cluster installed without kubeadm default
# labels), Core pod stays Pending. This script labels one node so Core can schedule.
#
# Logic: If any node already has the label, exit 0. Else pick a node to label:
# - Node whose name contains "master" or "control" (case-insensitive), or
# - First node in kubectl get nodes.
# Called by deploy-fortuna-robust.sh before deploying Core.
#
# Usage: ./scripts/deploy/ensure-control-plane-label.sh
# Env:   CONTROL_PLANE_NODE_NAME=k8s-master  force this node name to label
# ============================================================================

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'
log_info()    { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[OK]${NC} $1"; }
log_warn()    { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error()   { echo -e "${RED}[ERR]${NC} $1"; }

LABEL_KEY="node-role.kubernetes.io/control-plane"
LABEL_VAL=""

log_info "Checking for node with label $LABEL_KEY..."
if kubectl get nodes -l "$LABEL_KEY" --no-headers 2>/dev/null | grep -q .; then
  log_success "At least one node already has label $LABEL_KEY"
  kubectl get nodes -l "$LABEL_KEY" --no-headers 2>/dev/null || true
  exit 0
fi

# Pick node to label
TARGET_NODE=""
if [ -n "${CONTROL_PLANE_NODE_NAME:-}" ]; then
  if kubectl get node "$CONTROL_PLANE_NODE_NAME" &>/dev/null; then
    TARGET_NODE="$CONTROL_PLANE_NODE_NAME"
  fi
fi
if [ -z "$TARGET_NODE" ]; then
  # Prefer node whose name contains master or control
  TARGET_NODE=$(kubectl get nodes -o jsonpath='{.items[*].metadata.name}' 2>/dev/null | tr ' ' '\n' | grep -iE 'master|control' | head -1)
fi
if [ -z "$TARGET_NODE" ]; then
  # Fallback: first node
  TARGET_NODE=$(kubectl get nodes -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
fi

if [ -z "$TARGET_NODE" ]; then
  log_error "No nodes found; cannot add control-plane label"
  exit 1
fi

log_info "Adding label $LABEL_KEY to node $TARGET_NODE..."
if kubectl label node "$TARGET_NODE" "$LABEL_KEY=$LABEL_VAL" --overwrite 2>/dev/null; then
  log_success "Label added; Core deployment can now schedule on $TARGET_NODE"
  exit 0
fi

log_error "Failed to label node $TARGET_NODE"
exit 1
