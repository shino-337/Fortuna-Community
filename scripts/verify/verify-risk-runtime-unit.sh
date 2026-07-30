#!/usr/bin/env bash
# Unit tests cho risk/runtime GAP (không cần cluster). Chạy từ repo root:
#   ./scripts/verify/verify-risk-runtime-unit.sh
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT/core"
go test ./pkg/riskengine/... ./pkg/worker/... -count=1 "$@"
echo "core riskengine/worker: OK"
cd "$ROOT/agent"
go test ./internal/runtime/... -count=1 "$@"
echo "agent runtime: OK"
