#!/usr/bin/env bash
# ============================================================================
# Run full clean/rebuild/deploy in background so it does not hit IDE or shell timeout.
# Build (Core + Agent + Dashboard) can take 10–20+ minutes.
# ============================================================================
# Usage:
#   ./scripts/pipeline/run-clean-rebuild-deploy-no-timeout.sh
#   ./scripts/pipeline/run-clean-rebuild-deploy-no-timeout.sh --db
# Then: tail -f /tmp/clean-rebuild-deploy.log
# ============================================================================

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"
export RUN_ASYNC=1
exec ./full-clean-database-rebuild-deploy.sh "$@"
