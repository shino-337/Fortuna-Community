#!/usr/bin/env bash
# =============================================================================
# Check that this environment can rebuild and deploy all Fortuna images.
# Run: ./scripts/verify/check-env-rebuild-deploy.sh
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'
ok()  { echo -e "${GREEN}✅${NC} $1"; }
warn() { echo -e "${YELLOW}⚠️${NC}  $1"; }
err()  { echo -e "${RED}❌${NC} $1"; }
info() { echo -e "${BLUE}[INFO]${NC} $1"; }

echo "=========================================="
echo "Environment check – Rebuild & Deploy"
echo "=========================================="
echo ""

FAIL=0

# --- Prerequisites (build) ---
echo "=== Build and runtime tools ==="
command -v kubectl >/dev/null 2>&1 && ok "kubectl" || { err "kubectl"; FAIL=1; }
command -v nerdctl >/dev/null 2>&1 && ok "nerdctl" || { err "nerdctl (needed to build into containerd)"; FAIL=1; }
command -v ctr >/dev/null 2>&1 && ok "ctr (containerd)" || { err "ctr"; FAIL=1; }
echo ""

# --- Kubernetes cluster ---
echo "=== Kubernetes cluster ==="
if kubectl cluster-info &>/dev/null; then
  ok "Cluster accessible"
  node_count="$(kubectl get nodes --no-headers 2>/dev/null | wc -l | tr -d ' ')"
  info "Nodes: $node_count"
  disk_pressure_nodes="$(kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.name}{"="}{range .status.conditions[?(@.type=="DiskPressure")]}{.status}{end}{" "}{end}' 2>/dev/null || true)"
  if [[ "$disk_pressure_nodes" == *"=True"* ]]; then
    err "Nodes under DiskPressure ($disk_pressure_nodes); free disk space and wait for False before rolling out"
    FAIL=1
  else
    ok "Không có node DiskPressure"
  fi
else
  err "Cannot reach the cluster (kubectl cluster-info failed)"
  warn "Start the cluster (kind/kubeadm/minikube) and rerun. Deploy only runs against a ready cluster."
  FAIL=1
fi
echo ""

# --- Repo: Dockerfiles ---
echo "=== Dockerfiles ==="
for f in core/Dockerfile agent/Dockerfile dashboard/Dockerfile; do
  [ -f "$PROJECT_ROOT/$f" ] && ok "$f" || { err "Missing $f"; FAIL=1; }
done
echo ""

# --- Repo: Build script ---
echo "=== Script build ==="
[ -x "$PROJECT_ROOT/scripts/build/build-and-load-containerd.sh" ] && ok "build-and-load-containerd.sh" || { err "Missing or not executable: scripts/build/build-and-load-containerd.sh"; FAIL=1; }
for f in scripts/utils/create_mtls_secret.sh scripts/utils/ensure-fortuna-secrets.sh scripts/deploy/deploy-fortuna-robust.sh; do
  [ -x "$PROJECT_ROOT/$f" ] && ok "$f" || { err "Missing or not executable: $f"; FAIL=1; }
done
echo ""

# --- Repo: Deploy YAMLs ---
echo "=== Deploy YAMLs ==="
for f in deploy/fortuna-core-deployment.yaml deploy/fortuna-agent-daemonset.yaml deploy/fortuna-rbac.yaml deploy/dashboard-deployment.yaml; do
  [ -f "$PROJECT_ROOT/$f" ] && ok "$f" || { err "Missing $f"; FAIL=1; }
done
echo ""

# --- Pipeline ---
echo "=== Pipeline ==="
[ -x "$PROJECT_ROOT/scripts/pipeline/full-clean-database-rebuild-deploy.sh" ] && ok "full-clean-database-rebuild-deploy.sh" || warn "Pipeline script không executable"
if [ ! -x "$PROJECT_ROOT/scripts/deploy/deploy-fortuna-robust.sh" ]; then
  warn "deploy-fortuna-robust.sh not found; the pipeline will apply deploy/*.yaml directly"
fi
echo ""

# --- Disk / RAM (advisory) ---
echo "=== Host resources (advisory) ==="
free -h | head -2
df -h / 2>/dev/null | tail -1
echo ""

echo "=========================================="
if [ $FAIL -eq 0 ]; then
  echo -e "${GREEN}Result: this environment can rebuild and deploy (once the cluster is running).${NC}"
  echo ""
  echo "Cluster already has data: use the image update procedure that keeps data:"
  echo "  docs/operations/DEPLOYMENT_CONTAINERD.md#update-an-existing-local-single-node-installation"
  echo ""
  echo "Use the clean/rebuild/deploy pipeline only when you intend a clean reset:"
  echo "  ./scripts/pipeline/full-clean-database-rebuild-deploy.sh --full"
  echo ""
  echo "Rebuild only (no deploy):"
  echo "  ./scripts/pipeline/full-clean-database-rebuild-deploy.sh --skip-deploy"
  echo ""
  echo "Run the pipeline in the background (avoids timeouts; log in /tmp/clean-rebuild-deploy.log):"
  echo "  RUN_ASYNC=1 ./scripts/pipeline/full-clean-database-rebuild-deploy.sh"
  exit 0
else
  echo -e "${RED}Some checks failed; fix the errors above and rerun.${NC}"
  exit 1
fi
