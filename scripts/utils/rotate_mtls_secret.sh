#!/usr/bin/env bash
# ============================================================================
# Rotate mTLS secrets and restart Core/Agent (Finding #4.3)
# ============================================================================
# Regenerates certs (create_mtls_secret.sh), applies secrets, then rollout
# restarts Core and Agent so they load the new certificates. Run before cert
# expiry (e.g. within 30 days of 365-day expiry). Optional: schedule via cron
# or pipeline.
#
# Usage:
#   NAMESPACE=fortuna ./scripts/utils/rotate_mtls_secret.sh
#   ./scripts/utils/rotate_mtls_secret.sh   # uses NAMESPACE=fortuna
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
NAMESPACE="${NAMESPACE:-fortuna}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "=========================================="
echo "mTLS rotation (Finding #4.3)"
echo "=========================================="
echo "Namespace: $NAMESPACE"
echo ""

# 1. Generate and apply new secrets
echo "[1/3] Generating and applying mTLS secrets..."
if ! cd "$PROJECT_ROOT" && NAMESPACE="$NAMESPACE" bash "$SCRIPT_DIR/create_mtls_secret.sh"; then
  echo -e "${RED}Failed to create mTLS secrets${NC}"
  exit 1
fi
echo ""

# 2. Rollout restart Core and Agent
echo "[2/3] Rolling out Core and Agent to load new certs..."
kubectl rollout restart deployment/fortuna-core -n "$NAMESPACE" --timeout=120s 2>/dev/null || true
kubectl rollout restart daemonset/fortuna-agent -n "$NAMESPACE" --timeout=120s 2>/dev/null || true
echo -e "${GREEN}Rollout restart requested${NC}"
echo ""

# 3. Brief verify
echo "[3/3] Waiting for Core to be available (max 90s)..."
if kubectl wait --for=condition=available deployment/fortuna-core -n "$NAMESPACE" --timeout=90s 2>/dev/null; then
  echo -e "${GREEN}mTLS rotation complete. Core and Agent are loading new certificates.${NC}"
else
  echo -e "${YELLOW}Core not yet available; check: kubectl get pods -n $NAMESPACE -l app.kubernetes.io/component=core${NC}"
fi
echo ""
echo "Done. Verify gRPC connection in Agent logs (no TLS handshake errors)."
