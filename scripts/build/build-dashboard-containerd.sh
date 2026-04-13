#!/usr/bin/env bash
# =============================================================================
# Build Fortuna Dashboard image only and load into containerd
# =============================================================================
# Wrapper around build-and-load-containerd.sh that builds only the Dashboard.
# Supports three build backends (auto-detected): nerdctl, docker, buildctl.
# Override: BUILD_TOOL=docker | BUILD_TOOL=nerdctl | BUILD_TOOL=buildctl
#
# Usage:
#   ./scripts/build/build-dashboard-containerd.sh
#   BUILD_TOOL=docker ./scripts/build/build-dashboard-containerd.sh
#   NO_CACHE=true ./scripts/build/build-dashboard-containerd.sh
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Build only dashboard by setting BUILD_DASHBOARD_ONLY=true
export BUILD_DASHBOARD_ONLY=true
exec "$SCRIPT_DIR/build-and-load-containerd.sh" "$@"
