#!/usr/bin/env bash
# =============================================================================
# Safe post-purge recovery: purge fortuna refs → build/load → verify → restart
# =============================================================================
# Running only purge + restart without a rebuild leaves no fortuna-*:latest in
# containerd → ImagePullBackOff. This script enforces the full sequence.
#
# Usage (from repo root):
#   ./scripts/utils/rebuild-fortuna-workloads-safe.sh
#   NO_CACHE=true ./scripts/utils/rebuild-fortuna-workloads-safe.sh
#
# Extra args are passed to build-and-load-containerd.sh.
# Env: NAMESPACE, CONTAINERD_NAMESPACE, SKIP_VERIFY (for restart; default verify on)
# =============================================================================

set -euo pipefail

export PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:${PATH:-}"

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

"$REPO_ROOT/scripts/utils/force-fortuna-image-refresh.sh" purge
"$REPO_ROOT/scripts/build/build-and-load-containerd.sh" "$@"
"$REPO_ROOT/scripts/utils/force-fortuna-image-refresh.sh" restart
