#!/usr/bin/env bash
# =============================================================================
# Fortuna local images: purge old refs / rollout workloads (stale :latest)
# =============================================================================
# Deploy uses imagePullPolicy: IfNotPresent + fortuna-*:latest. Kubelet keeps
# running the old rootfs until pods are recreated; if containerd still holds
# an old "latest" ref, you must remove refs before the new build is the only
# one left, then restart pods.
#
# Recommended single-node sequence:
#   ./scripts/utils/force-fortuna-image-refresh.sh purge
#   NO_CACHE=true ./scripts/build/build-and-load-containerd.sh
#   ./scripts/utils/force-fortuna-image-refresh.sh restart
#
# One-liner if you already rebuilt and only need new pods (image already updated):
#   ./scripts/utils/force-fortuna-image-refresh.sh restart
#
#   force-fortuna-image-refresh.sh verify   — fail if fortuna-*:latest missing in containerd (run before restart).
#
# Do NOT run nerdctl system prune here: it can evict busybox:1.36 used by Core initContainers → Init:ErrImagePull
# on air-gapped nodes. Use targeted rmi only (this script).
#
# Env: NAMESPACE (default fortuna), CONTAINERD_NAMESPACE (default k8s.io)
#      SKIP_VERIFY=1 with restart — skip local image check (e.g. you use a registry only)
# =============================================================================

set -euo pipefail

export PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:${PATH:-}"

NAMESPACE="${NAMESPACE:-fortuna}"
CONTAINERD_NS="${CONTAINERD_NAMESPACE:-k8s.io}"
CMD="${1:-}"

log() { echo "[force-fortuna-image-refresh] $*"; }

usage() {
  cat <<'EOF'
Usage:
  force-fortuna-image-refresh.sh purge    Remove fortuna-*:latest refs (targeted only; no system prune).
  force-fortuna-image-refresh.sh verify   Check fortuna-core/agent/dashboard :latest exist in containerd.
  force-fortuna-image-refresh.sh restart  Verify images (unless SKIP_VERIFY=1), then rollout restart workloads.

Typical: purge → build-and-load-containerd.sh → restart
One-shot: ./scripts/utils/rebuild-fortuna-workloads-safe.sh
EOF
}

purge_images() {
  log "Removing fortuna-* refs (nerdctl / ctr / docker), namespace=$CONTAINERD_NS"
  for t in latest; do
    if command -v nerdctl &>/dev/null; then
      nerdctl --namespace "$CONTAINERD_NS" rmi -f "fortuna-core:$t" 2>/dev/null || true
      nerdctl --namespace "$CONTAINERD_NS" rmi -f "fortuna-agent:$t" 2>/dev/null || true
      nerdctl --namespace "$CONTAINERD_NS" rmi -f "fortuna-dashboard:$t" 2>/dev/null || true
    fi
  done
  if command -v ctr &>/dev/null; then
    local refs
    refs=$(ctr -n "$CONTAINERD_NS" images list -q 2>/dev/null | grep -E 'fortuna-(core|agent|dashboard)' || true)
    if [[ -n "$refs" ]]; then
      while IFS= read -r ref; do
        [[ -z "$ref" ]] && continue
        log "ctr rm $ref"
        ctr -n "$CONTAINERD_NS" images rm "$ref" 2>/dev/null || true
      done <<< "$refs"
    fi
  fi
  if command -v docker &>/dev/null; then
    docker rmi -f fortuna-core:latest fortuna-agent:latest fortuna-dashboard:latest 2>/dev/null || true
  fi
  log "Purge done."
}

# Combined listing from nerdctl or ctr (plain text for grep).
_images_text() {
  if command -v nerdctl &>/dev/null; then
    nerdctl --namespace "$CONTAINERD_NS" images 2>/dev/null || true
    return 0
  fi
  if command -v ctr &>/dev/null; then
    ctr -n "$CONTAINERD_NS" images ls 2>/dev/null || true
    return 0
  fi
  echo ""
}

# True if listing contains this repo with tag latest (nerdctl table or ctr ref).
_image_has_repo_latest() {
  local txt="$1" repo="$2"
  echo "$txt" | grep -- "$repo" | grep -qE '(:latest|[[:space:]]latest[[:space:]])'
}

verify_images() {
  local txt
  txt="$(_images_text)"
  if [[ -z "$(echo "$txt" | tr -d '[:space:]')" ]]; then
    log "WARN: Could not list images (install nerdctl or ctr). Set SKIP_VERIFY=1 for restart, or install ctr/nerdctl."
    return 1
  fi
  local ok=true
  for repo in fortuna-core fortuna-agent fortuna-dashboard; do
    if ! _image_has_repo_latest "$txt" "$repo"; then
      log "ERR: missing ${repo}:latest in containerd namespace $CONTAINERD_NS"
      ok=false
    fi
  done
  if [[ "$ok" != true ]]; then
    log "Run: ./scripts/build/build-and-load-containerd.sh (on this host for single-node dev)."
    log "Core Deployment uses nodeSelector control-plane — images must exist on the node that schedules fortuna-core."
    return 1
  fi
  log "OK: fortuna-core/agent/dashboard :latest present in containerd listing."
  return 0
}

rollout_restart() {
  if ! command -v kubectl &>/dev/null; then
    log "kubectl not in PATH; cannot rollout."
    exit 1
  fi
  if [[ "${SKIP_VERIFY:-0}" != "1" ]]; then
    if ! verify_images; then
      log "Aborting restart (images missing). Build/load images, or set SKIP_VERIFY=1 if using remote registry."
      exit 2
    fi
  fi
  log "kubectl rollout restart in namespace=$NAMESPACE"
  kubectl rollout restart deployment/fortuna-core -n "$NAMESPACE" 2>/dev/null || log "WARN: fortuna-core"
  kubectl rollout restart deployment/fortuna-dashboard -n "$NAMESPACE" 2>/dev/null || log "WARN: fortuna-dashboard"
  kubectl rollout restart daemonset/fortuna-agent -n "$NAMESPACE" 2>/dev/null || log "WARN: fortuna-agent"
  kubectl rollout status deployment/fortuna-core -n "$NAMESPACE" --timeout=180s 2>/dev/null || true
  kubectl rollout status deployment/fortuna-dashboard -n "$NAMESPACE" --timeout=120s 2>/dev/null || true
  kubectl rollout status daemonset/fortuna-agent -n "$NAMESPACE" --timeout=180s 2>/dev/null || true
  log "Rollout done. Check: kubectl get pods -n $NAMESPACE -o wide"
}

case "$CMD" in
  purge)   purge_images ;;
  verify)  verify_images || exit 1 ;;
  restart) rollout_restart ;;
  -h|--help|help|"")
    usage
    [[ -n "$CMD" && "$CMD" != "-h" && "$CMD" != "--help" && "$CMD" != help ]] && exit 1
    exit 0
    ;;
  *)
    log "Unknown command: $CMD"
    usage
    exit 1
    ;;
esac
