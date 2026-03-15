#!/usr/bin/env bash
# build-control-plane-digest-map.sh – Build digest→version map for control-plane images
# for use in agent/pkg/sbom/signatures/distroless.json (digestMap per binary).
#
# Prereq: crane – go install github.com/google/go-containerregistry/cmd/crane@latest
# Usage:
#   ./scripts/docs/build-control-plane-digest-map.sh [stable|v1.29.0|v1.30.0 ...]
#   With no args, uses a default list of tags (stable + a few versions).
# Output: JSON object digestMap (or one per binary) to merge into distroless.json.

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Default: fetch stable and a few recent versions for k8s components
get_k8s_stable() {
  curl -sL "https://dl.k8s.io/release/stable.txt" 2>/dev/null || echo "v1.29.0"
}

# Image list: binary_name image_ref (tag or digest)
# registry.k8s.io images; coredns is under coredns/coredns
CONTROL_PLANE_IMAGES=(
  "kube-apiserver|registry.k8s.io/kube-apiserver"
  "kube-controller-manager|registry.k8s.io/kube-controller-manager"
  "kube-scheduler|registry.k8s.io/kube-scheduler"
  "kube-proxy|registry.k8s.io/kube-proxy"
  "etcd|registry.k8s.io/etcd"
  "coredns|registry.k8s.io/coredns/coredns"
  "pause|registry.k8s.io/pause"
)

# Versions to resolve (default: stable + a few). For etcd/coredns/pause tags may differ
# (e.g. coredns v1.11.1, pause 3.9); use same list for all or extend script to per-image tags.
VERSIONS=("$@")
if [ ${#VERSIONS[@]} -eq 0 ]; then
  STABLE=$(get_k8s_stable)
  VERSIONS=("$STABLE" "v1.30.0" "v1.29.0" "v1.28.0")
fi

if ! command -v crane &>/dev/null; then
  echo "crane not found. Install: go install github.com/google/go-containerregistry/cmd/crane@latest" >&2
  exit 1
fi

# Output per-binary digestMap (for merging into distroless.json)
# Format: binary_name -> { "sha256:...": "version", ... }
echo "{"
first_binary=1
for entry in "${CONTROL_PLANE_IMAGES[@]}"; do
  binary="${entry%%|*}"
  image="${entry#*|}"
  # coredns uses different tag scheme (v1.11.1); others use k8s version
  # For simplicity we use same VERSIONS for all; you can override per-image later
  digest_map=""
  for ver in "${VERSIONS[@]}"; do
    ref="${image}:${ver}"
    digest=""
    if digest=$(crane digest "$ref" 2>/dev/null); then
      # Normalize version for PURL (strip v for semver consistency if desired)
      ver_clean="$ver"
      [[ "$ver_clean" =~ ^v([0-9].*) ]] && ver_clean="${BASH_REMATCH[1]}"
      if [ -n "$digest_map" ]; then digest_map="$digest_map, "; fi
      digest_map="${digest_map}\"${digest}\": \"${ver_clean}\""
    fi
  done
  [ $first_binary -eq 0 ] && echo ","
  first_binary=0
  echo "  \"$binary\": { $digest_map }"
done
echo "}"
