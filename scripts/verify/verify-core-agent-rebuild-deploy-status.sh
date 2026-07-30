#!/usr/bin/env bash
# =============================================================================
# Kiểm tra Core và Agent đã được rebuild và deploy theo cập nhật trong
# docs/02-architecture/Architecture_Finding_Remediation_Plan.md hay chưa.
#
# Cách kiểm tra:
# 1. So sánh image ID: pod đang chạy vs image local (nerdctl) fortuna-core:latest, fortuna-agent:latest.
# 2. Nếu trùng → pod đang dùng image build từ máy này (đã load sau build).
# 3. Lấy dòng [Build] từ log Core/Agent: version= commit= time= (nếu build script truyền build-arg thì commit khớp với git hiện tại).
#
# Chạy: ./scripts/verify/verify-core-agent-rebuild-deploy-status.sh
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
NAMESPACE="${NAMESPACE:-fortuna}"
CONTAINERD_NS="${CONTAINERD_NAMESPACE:-k8s.io}"
# Timeout cho kubectl (tránh treo khi API chậm/mất kết nối). Bỏ qua nếu không có lệnh timeout.
KUBECTL_TIMEOUT="${KUBECTL_TIMEOUT:-15}"
kube() { if command -v timeout &>/dev/null; then timeout "$KUBECTL_TIMEOUT" kubectl "$@"; else kubectl "$@"; fi; }

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'
ok()   { echo -e "${GREEN}✅${NC} $1"; }
warn() { echo -e "${YELLOW}⚠️${NC}  $1"; }
fail() { echo -e "${RED}❌${NC} $1"; }
info() { echo -e "${BLUE}[INFO]${NC} $1"; }

echo "=========================================="
echo "Kiểm tra Core/Agent – Rebuild & Deploy"
echo "=========================================="
echo "So với: docs/02-architecture/Architecture_Finding_Remediation_Plan.md"
echo ""

FAIL=0

# --- 1. Image tag trong deploy YAML (dùng tag này cho nerdctl, không hardcode latest) ---
echo "=== 1. Image trong deploy YAML ==="
CORE_YAML="${DEPLOY_CORE_YAML:-$PROJECT_ROOT/deploy/fortuna-core-deployment.yaml}"
AGENT_YAML="${DEPLOY_AGENT_YAML:-$PROJECT_ROOT/deploy/fortuna-agent-daemonset.yaml}"
# Extract tag from deploy YAML (image: fortuna-core:TAG); fallback latest nếu file không tồn tại hoặc không parse được
get_image_tag() {
  local file="$1"
  local name="$2"
  [ -f "$file" ] || return 0
  grep -E "image:[[:space:]]*${name}:" "$file" 2>/dev/null | sed -E 's/.*'"${name}"':([^[:space:]#]+).*/\1/' | tr -d '"' | head -1
}
CORE_TAG=$(get_image_tag "$CORE_YAML" "fortuna-core")
AGENT_TAG=$(get_image_tag "$AGENT_YAML" "fortuna-agent")
CORE_TAG="${CORE_TAG:-latest}"
AGENT_TAG="${AGENT_TAG:-latest}"
info "fortuna-core:${CORE_TAG}  (from ${CORE_YAML##*/})"
info "fortuna-agent:${AGENT_TAG}  (from ${AGENT_YAML##*/})"
echo ""

# --- 2. Cluster và pod ---
echo "=== 2. Pod Core / Agent trên cluster ==="
if ! kube cluster-info &>/dev/null; then
  fail "Không kết nối được cluster (kubectl cluster-info)"
  FAIL=1
else
  ok "Cluster accessible"
fi

CORE_POD=$(kube get pods -n "$NAMESPACE" -l app.kubernetes.io/name=fortuna,app.kubernetes.io/component=core --no-headers 2>/dev/null | head -1 | awk '{print $1}')
AGENT_POD=$(kube get pods -n "$NAMESPACE" -l app.kubernetes.io/name=fortuna,app.kubernetes.io/component=agent --no-headers 2>/dev/null | head -1 | awk '{print $1}')

if [ -z "$CORE_POD" ]; then
  warn "Không tìm thấy pod Core trong namespace $NAMESPACE"
  CORE_IMAGE_ID=""
else
  ok "Core pod: $CORE_POD"
  CORE_IMAGE_ID=$(kube get pod -n "$NAMESPACE" "$CORE_POD" -o jsonpath='{.status.containerStatuses[0].imageID}' 2>/dev/null || echo "")
fi
if [ -z "$AGENT_POD" ]; then
  warn "Không tìm thấy pod Agent trong namespace $NAMESPACE"
  AGENT_IMAGE_ID=""
else
  ok "Agent pod: $AGENT_POD"
  AGENT_IMAGE_ID=$(kube get pod -n "$NAMESPACE" "$AGENT_POD" -o jsonpath='{.status.containerStatuses[0].imageID}' 2>/dev/null || echo "")
fi
echo ""

# --- 3. Image local (nerdctl) ---
echo "=== 3. Image local (nerdctl, namespace=$CONTAINERD_NS) ==="
NERDCTL_BIN="${NERDCTL_BIN:-nerdctl}"
# Lấy Id từ image inspect (cùng format với Kubernetes imageID = config digest); fallback images -q nếu inspect không có.
get_local_image_id() {
  local img="$1"
  local id
  id=$("$NERDCTL_BIN" --namespace "$CONTAINERD_NS" image inspect "$img" --format '{{.Id}}' 2>/dev/null)
  [ -n "$id" ] && echo "$id" && return 0
  id=$("$NERDCTL_BIN" --namespace "$CONTAINERD_NS" images -q "$img" 2>/dev/null | head -1)
  [ -n "$id" ] && echo "$id"
}
if ! command -v "$NERDCTL_BIN" &>/dev/null; then
  warn "nerdctl không tìm thấy; bỏ qua so sánh image ID với local"
  LOCAL_CORE_ID=""
  LOCAL_AGENT_ID=""
else
  LOCAL_CORE_ID=$(get_local_image_id "fortuna-core:${CORE_TAG}")
  LOCAL_AGENT_ID=$(get_local_image_id "fortuna-agent:${AGENT_TAG}")
  if [ -z "$LOCAL_CORE_ID" ]; then
    warn "Không có image local fortuna-core:${CORE_TAG} (chưa build hoặc đã xóa; build script tag :latest và :\$VERSION)"
  else
    info "fortuna-core:${CORE_TAG} local image ID: ${LOCAL_CORE_ID:0:19}..."
  fi
  if [ -z "$LOCAL_AGENT_ID" ]; then
    warn "Không có image local fortuna-agent:${AGENT_TAG} (chưa build hoặc đã xóa)"
  else
    info "fortuna-agent:${AGENT_TAG} local image ID: ${LOCAL_AGENT_ID:0:19}..."
  fi
fi
echo ""

# --- 4. So sánh image ID (pod vs local) ---
echo "=== 4. So sánh image (pod đang chạy vs image local) ==="
# Kubernetes imageID = config digest (sha256:...). Local lấy từ nerdctl image inspect {{.Id}} (cùng format).
# So sánh 12 ký tự đầu của digest hoặc full string.
normalize_id() {
  local v="$1"
  if [[ "$v" == *sha256:* ]]; then
    echo "$v" | sed 's/.*sha256:\([a-f0-9]*\).*/\1/' | cut -c1-12
  else
    echo "$v" | tr -d '\n' | cut -c1-12
  fi
}

CORE_MATCH=false
AGENT_MATCH=false
if [ -n "$CORE_IMAGE_ID" ] && [ -n "$LOCAL_CORE_ID" ]; then
  POD_CORE_SHA=$(normalize_id "$CORE_IMAGE_ID")
  LOC_CORE_SHA=$(normalize_id "$LOCAL_CORE_ID")
  if [ "$POD_CORE_SHA" = "$LOC_CORE_SHA" ] || [ "$LOCAL_CORE_ID" = "$CORE_IMAGE_ID" ]; then
    ok "Core: pod đang dùng cùng image với local (đã rebuild & deploy)"
    CORE_MATCH=true
  else
    warn "Core: pod image khác image local → cần rollout restart sau khi rebuild (hoặc chưa rebuild)"
    info "  pod:  $CORE_IMAGE_ID"
    info "  local: $LOCAL_CORE_ID"
  fi
elif [ -z "$CORE_POD" ]; then
  warn "Core: không có pod để so sánh"
elif [ -z "$LOCAL_CORE_ID" ]; then
  warn "Core: không có image local để so sánh → chạy build-and-load-containerd.sh"
fi

if [ -n "$AGENT_IMAGE_ID" ] && [ -n "$LOCAL_AGENT_ID" ]; then
  POD_AGENT_SHA=$(normalize_id "$AGENT_IMAGE_ID")
  LOC_AGENT_SHA=$(normalize_id "$LOCAL_AGENT_ID")
  if [ "$POD_AGENT_SHA" = "$LOC_AGENT_SHA" ] || [ "$LOCAL_AGENT_ID" = "$AGENT_IMAGE_ID" ]; then
    ok "Agent: pod đang dùng cùng image với local (đã rebuild & deploy)"
    AGENT_MATCH=true
  else
    warn "Agent: pod image khác image local → cần rollout restart sau khi rebuild (hoặc chưa rebuild)"
    info "  pod:  $AGENT_IMAGE_ID"
    info "  local: $LOCAL_AGENT_ID"
  fi
elif [ -z "$AGENT_POD" ]; then
  warn "Agent: không có pod để so sánh"
elif [ -z "$LOCAL_AGENT_ID" ]; then
  warn "Agent: không có image local để so sánh → chạy build-and-load-containerd.sh"
fi
echo ""

# --- 5. Build info từ log (commit / time) ---
echo "=== 5. Build info trong log (version= commit= time=) ==="
CURRENT_COMMIT=$(git -C "$PROJECT_ROOT" rev-parse --short HEAD 2>/dev/null || echo "")

if [ -n "$CORE_POD" ]; then
  BUILD_LINE=$(kube logs -n "$NAMESPACE" "$CORE_POD" --tail=500 2>/dev/null | grep -m1 '\[Build\]' || true)
  if [ -n "$BUILD_LINE" ]; then
    info "Core: $BUILD_LINE"
    if [ -n "$CURRENT_COMMIT" ] && echo "$BUILD_LINE" | grep -q "commit=$CURRENT_COMMIT"; then
      ok "Core commit trong log khớp với git hiện tại ($CURRENT_COMMIT)"
    elif echo "$BUILD_LINE" | grep -q "commit=none"; then
      warn "Core build không có commit (build cũ hoặc chưa dùng build-arg); rebuild bằng scripts/build/build-and-load-containerd.sh"
    fi
  else
    warn "Core: không tìm thấy dòng [Build] trong log"
  fi
fi
if [ -n "$AGENT_POD" ]; then
  BUILD_LINE=$(kube logs -n "$NAMESPACE" "$AGENT_POD" --tail=500 2>/dev/null | grep -m1 '\[Build\]' || true)
  if [ -n "$BUILD_LINE" ]; then
    info "Agent: $BUILD_LINE"
    if [ -n "$CURRENT_COMMIT" ] && echo "$BUILD_LINE" | grep -q "commit=$CURRENT_COMMIT"; then
      ok "Agent commit trong log khớp với git hiện tại ($CURRENT_COMMIT)"
    elif echo "$BUILD_LINE" | grep -q "commit=none"; then
      warn "Agent build không có commit (build cũ); rebuild bằng scripts/build/build-and-load-containerd.sh"
    fi
  else
    warn "Agent: không tìm thấy dòng [Build] trong log"
  fi
fi
if [ -n "$CURRENT_COMMIT" ]; then
  info "Git commit hiện tại: $CURRENT_COMMIT"
fi
echo ""

# --- Kết luận ---
echo "=========================================="
if [ "$CORE_MATCH" = true ] && [ "$AGENT_MATCH" = true ]; then
  echo -e "${GREEN}Kết luận: Core và Agent đang chạy image trùng với local (đã rebuild và deploy).${NC}"
  echo "Nếu vừa cập nhật code theo Architecture plan, image local đã là bản mới và pod đã dùng đúng image."
  exit 0
fi

echo -e "${YELLOW}Kết luận: Có thể chưa rebuild/deploy theo Architecture plan hoặc pod chưa restart.${NC}"
echo ""
echo "Để đảm bảo Core và Agent chạy code mới (docs/02-architecture/Architecture_Finding_Remediation_Plan.md):"
echo "  1. Rebuild:  ./scripts/build/build-and-load-containerd.sh"
echo "     (hoặc:    NO_CACHE=true ./scripts/build/build-and-load-containerd.sh)"
echo "  2. Deploy:   ./scripts/deploy/deploy-fortuna-robust.sh"
echo "     hoặc:     ./scripts/pipeline/full-clean-database-rebuild-deploy.sh"
echo "  3. Restart:  kubectl rollout restart deployment/fortuna-core -n $NAMESPACE"
echo "               kubectl rollout restart daemonset/fortuna-agent -n $NAMESPACE"
echo "  4. Kiểm tra lại: ./scripts/verify/verify-core-agent-rebuild-deploy-status.sh"
echo ""
exit 0
