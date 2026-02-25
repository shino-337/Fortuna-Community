#!/bin/bash
# ============================================================================
# Clean environment (script chính – dọn môi trường)
# ============================================================================
# Port-forward, E2E/test namespaces, completed/failed/evicted pods, old images, build cache.
# Usage:
#   ./scripts/clean/cleanup-environment.sh              # clean mặc định (giữ 3 image mới nhất)
#   ./scripts/clean/cleanup-environment.sh --db        # + xóa E2E test data trong Postgres
#   ./scripts/clean/cleanup-environment.sh --aggressive # + xóa thêm test ns, image không latest
# ============================================================================
set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
REPO_ROOT="$PROJECT_ROOT"
NAMESPACE="${NAMESPACE:-fortuna}"
CONTAINERD_NS="${CONTAINERD_NAMESPACE:-k8s.io}"
CLEAN_DB=false
AGGRESSIVE=false
for arg in "$@"; do
  case "$arg" in
    --db) CLEAN_DB=true ;;
    --aggressive) AGGRESSIVE=true ;;
  esac
done

echo "=========================================="
echo "Environment Cleanup"
echo "=========================================="
echo "  --db: $CLEAN_DB | --aggressive: $AGGRESSIVE"
echo ""

# 1. Kill port-forward
echo "[1/6] Stopping port-forward processes..."
pkill -f "kubectl.*port-forward" 2>/dev/null || true

# 2. Delete E2E/test namespaces
echo "[2/6] Deleting E2E/test namespaces..."
for ns in fortuna-e2e fortuna-e2e-2025; do
  kubectl get namespace "$ns" 2>/dev/null && kubectl delete namespace "$ns" --timeout=60s 2>/dev/null || true
done
if [ "$AGGRESSIVE" = true ]; then
  for ns in fortuna-cve-test fortuna-pce-test fortuna-sbom-real; do
    kubectl get namespace "$ns" 2>/dev/null && kubectl delete namespace "$ns" --timeout=60s 2>/dev/null || true
  done
fi

# 3. Delete completed/failed/evicted pods (all ns)
echo "[3/6] Deleting completed/failed/evicted pods..."
if command -v jq >/dev/null 2>&1; then
  jq_filter='.items[] | select(.status.phase=="Succeeded" or .status.phase=="Failed")'
  [ "$AGGRESSIVE" = true ] && jq_filter='.items[] | select(.status.phase=="Succeeded" or .status.phase=="Failed" or .status.phase=="Evicted")'
  kubectl get pods -A -o json 2>/dev/null | jq -r "$jq_filter | \"\(.metadata.namespace) \(.metadata.name)\"" 2>/dev/null | while read -r ns name; do
    [ -n "$ns" ] && [ -n "$name" ] && kubectl delete pod -n "$ns" "$name" --ignore-not-found 2>/dev/null || true
  done
fi

# 4. Clean old fortuna images
echo "[4/6] Cleaning old fortuna images..."
if [ "$AGGRESSIVE" = true ]; then
  nerdctl --namespace "$CONTAINERD_NS" images 2>/dev/null | grep fortuna | grep -v latest | grep -v "REPOSITORY" | awk '{print $3}' | xargs -r nerdctl --namespace "$CONTAINERD_NS" rmi --force 2>/dev/null || true
else
  nerdctl --namespace "$CONTAINERD_NS" images 2>/dev/null | grep fortuna-core | tail -n +4 | awk '{print $3}' | xargs -r nerdctl --namespace "$CONTAINERD_NS" rmi --force 2>/dev/null || true
  nerdctl --namespace "$CONTAINERD_NS" images 2>/dev/null | grep fortuna-agent | tail -n +4 | awk '{print $3}' | xargs -r nerdctl --namespace "$CONTAINERD_NS" rmi --force 2>/dev/null || true
  nerdctl --namespace "$CONTAINERD_NS" images 2>/dev/null | grep fortuna-dashboard | tail -n +4 | awk '{print $3}' | xargs -r nerdctl --namespace "$CONTAINERD_NS" rmi --force 2>/dev/null || true
fi

# 5. Prune build cache
echo "[5/6] Pruning build cache..."
if [ "$AGGRESSIVE" = true ]; then
  nerdctl builder prune --namespace "$CONTAINERD_NS" -a -f 2>/dev/null || true
else
  nerdctl builder prune --namespace "$CONTAINERD_NS" -f 2>/dev/null || true
fi

# 6. DB (optional)
if [ "$CLEAN_DB" = true ]; then
  echo "[6/6] Removing E2E test data from Postgres..."
  POD=$(kubectl get pods -n "$NAMESPACE" -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
  if [ -n "$POD" ] && [ -f "$PROJECT_ROOT/deploy/e2e/clear_e2e_test_data.sql" ]; then
    kubectl cp "$PROJECT_ROOT/deploy/e2e/clear_e2e_test_data.sql" "$NAMESPACE/$POD:/tmp/clear_e2e.sql" 2>/dev/null || true
    kubectl exec -n "$NAMESPACE" "$POD" -- psql -U postgres -d fortuna -f /tmp/clear_e2e.sql 2>/dev/null || true
  fi
else
  echo "[6/6] Skipping DB (use --db to clear E2E test data)"
fi

echo ""
echo "Cleanup complete."
