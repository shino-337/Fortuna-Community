#!/usr/bin/env bash
# ============================================================================
# Deploy-full: single entrypoint for Fortuna deploy (Finding #7.1)
# ============================================================================
# Runs the full sequence: CNI/storage → infra (postgres, nats) → mTLS →
# RBAC → control-plane label → prerequisites check (secrets + postgres + nats)
# → Core → Agent → Dashboard. Exits on first failure with clear message.
#
# Usage:
#   ./scripts/deploy/deploy-full.sh
#   NAMESPACE=myns ./scripts/deploy/deploy-full.sh
#
# See also: deploy/README.md (Deploy checklist), deploy-fortuna-robust.sh.
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
SCRIPTS="$PROJECT_ROOT/scripts"
NAMESPACE="${NAMESPACE:-fortuna}"

echo "=========================================="
echo "Fortuna deploy-full (single entrypoint)"
echo "=========================================="
echo "Namespace: $NAMESPACE"
echo ""

if [ ! -x "$SCRIPTS/deploy/deploy-fortuna-robust.sh" ]; then
  echo "ERROR: deploy-fortuna-robust.sh not found or not executable: $SCRIPTS/deploy/deploy-fortuna-robust.sh"
  exit 1
fi

exec bash "$SCRIPTS/deploy/deploy-fortuna-robust.sh"
