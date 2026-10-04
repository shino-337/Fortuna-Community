#!/usr/bin/env bash
# Delete the kind cluster created by scripts/demo/up.sh and its local state.
set -euo pipefail

CLUSTER_NAME="${FORTUNA_DEMO_CLUSTER:-fortuna-demo}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

if kind get clusters 2>/dev/null | grep -qx "$CLUSTER_NAME"; then
  kind delete cluster --name "$CLUSTER_NAME"
else
  echo "kind cluster '${CLUSTER_NAME}' not found; nothing to delete."
fi
rm -rf "${ROOT}/.fortuna-demo"
