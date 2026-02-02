#!/bin/bash
# Full pipeline: Clean -> Rebuild (core, agent) -> Deploy
# Usage: ./scripts/full-clean-rebuild-deploy.sh [--skip-clean] [--skip-rebuild] [--skip-deploy] [--db to clean E2E data from DB]
set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
SKIP_CLEAN=false
SKIP_REBUILD=false
SKIP_DEPLOY=false
EXTRA_CLEAN=
for arg in "$@"; do
  case "$arg" in
    --skip-clean)   SKIP_CLEAN=true ;;
    --skip-rebuild) SKIP_REBUILD=true ;;
    --skip-deploy)  SKIP_DEPLOY=true ;;
    --db)          EXTRA_CLEAN="--db" ;;
  esac
done

echo "=========================================="
echo "Full pipeline: Clean -> Rebuild -> Deploy"
echo "=========================================="

if [ "$SKIP_CLEAN" = false ] && [ -x "$SCRIPT_DIR/cleanup-environment.sh" ]; then
  echo "[1/3] Clean..."
  "$SCRIPT_DIR/cleanup-environment.sh" $EXTRA_CLEAN
else
  echo "[1/3] Clean (skipped)"
fi

if [ "$SKIP_REBUILD" = false ] && [ -x "$SCRIPT_DIR/build-and-load-containerd.sh" ]; then
  echo "[2/3] Rebuild (no cache)..."
  NO_CACHE=true "$SCRIPT_DIR/build-and-load-containerd.sh"
else
  echo "[2/3] Rebuild (skipped)"
fi

if [ "$SKIP_DEPLOY" = false ]; then
  echo "[3/3] Deploy..."
  if [ -x "$SCRIPT_DIR/deploy-fortuna-robust.sh" ]; then
    "$SCRIPT_DIR/deploy-fortuna-robust.sh"
  else
    kubectl create namespace fortuna --dry-run=client -o yaml | kubectl apply -f -
    for f in deploy/infrastructure/postgresql-with-age.yaml deploy/infrastructure/nats.yaml deploy/fortuna-rbac.yaml deploy/fortuna-core-deployment.yaml deploy/fortuna-agent-daemonset.yaml deploy/dashboard-deployment.yaml; do
      [ -f "$PROJECT_ROOT/$f" ] && kubectl apply -f "$PROJECT_ROOT/$f" || true
    done
  fi
  echo "[3b] Forcing rollout restart so pods use new images..."
  kubectl rollout restart deployment/fortuna-core -n fortuna --timeout=60s 2>/dev/null || true
  kubectl rollout restart daemonset/fortuna-agent -n fortuna --timeout=60s 2>/dev/null || true
  kubectl rollout restart deployment/fortuna-dashboard -n fortuna --timeout=60s 2>/dev/null || true
  echo "Waiting for rollouts..."
  kubectl rollout status deployment/fortuna-core -n fortuna --timeout=120s 2>/dev/null || true
  kubectl rollout status daemonset/fortuna-agent -n fortuna --timeout=120s 2>/dev/null || true
  kubectl rollout status deployment/fortuna-dashboard -n fortuna --timeout=120s 2>/dev/null || true
  echo "[3c] Verifying running images..."
  kubectl get pods -n fortuna -o wide 2>/dev/null || true
  kubectl get pods -n fortuna -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.containers[0].image}{"\t"}{.status.containerStatuses[0].imageID}{"\n"}{end}' 2>/dev/null || true
else
  echo "[3/3] Deploy (skipped)"
fi

echo "Pipeline complete."
