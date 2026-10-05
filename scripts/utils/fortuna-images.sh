#!/usr/bin/env bash
# Image selection for the install scripts. Source it; do not run it.
#
# The manifests in deploy/ use ${FORTUNA_REGISTRY}/fortuna-*:latest. To install
# another build, set one of:
#   FORTUNA_VERSION=v1.0.0                 registry images with that tag
#   FORTUNA_REGISTRY=registry.example/x    registry prefix (with FORTUNA_VERSION)
#   FORTUNA_CORE_IMAGE / FORTUNA_AGENT_IMAGE / FORTUNA_DASHBOARD_IMAGE
#                                          full references, e.g. local builds
# fortuna_manifest copies a manifest to a temporary file with those images
# substituted, so tracked files in deploy/ are never edited.

FORTUNA_DEFAULT_REGISTRY="ghcr.io/shino-337/fortuna-community"

fortuna_resolve_images() {
    local registry="${FORTUNA_REGISTRY:-$FORTUNA_DEFAULT_REGISTRY}"
    registry="${registry%/}"
    if [ -n "${FORTUNA_VERSION:-}" ]; then
        FORTUNA_CORE_IMAGE="${FORTUNA_CORE_IMAGE:-${registry}/fortuna-core:${FORTUNA_VERSION}}"
        FORTUNA_AGENT_IMAGE="${FORTUNA_AGENT_IMAGE:-${registry}/fortuna-agent:${FORTUNA_VERSION}}"
        FORTUNA_DASHBOARD_IMAGE="${FORTUNA_DASHBOARD_IMAGE:-${registry}/fortuna-dashboard:${FORTUNA_VERSION}}"
    fi
    export FORTUNA_CORE_IMAGE FORTUNA_AGENT_IMAGE FORTUNA_DASHBOARD_IMAGE
    if [ -z "${FORTUNA_MANIFEST_DIR:-}" ]; then
        FORTUNA_MANIFEST_DIR="$(mktemp -d)"
        # shellcheck disable=SC2064
        trap "rm -rf '$FORTUNA_MANIFEST_DIR'" EXIT
    fi
}

# fortuna_manifest FILE (after fortuna_resolve_images): prints the path of a copy of FILE with image overrides applied.
fortuna_manifest() {
    local src="$1" out name ref
    [ -f "$src" ] || { echo "manifest not found: $src" >&2; return 1; }
    [ -n "${FORTUNA_MANIFEST_DIR:-}" ] || { echo "call fortuna_resolve_images first" >&2; return 1; }
    out="$FORTUNA_MANIFEST_DIR/$(basename "$src")"
    cp "$src" "$out"
    for name in fortuna-core fortuna-agent fortuna-dashboard; do
        case "$name" in
            fortuna-core) ref="${FORTUNA_CORE_IMAGE:-}" ;;
            fortuna-agent) ref="${FORTUNA_AGENT_IMAGE:-}" ;;
            fortuna-dashboard) ref="${FORTUNA_DASHBOARD_IMAGE:-}" ;;
        esac
        [ -n "$ref" ] || continue
        sed -i -E "s@(image:[[:space:]]*)\"?([^[:space:]\"#]*/)?${name}:[^[:space:]\"#]+\"?@\\1${ref}@" "$out"
    done
    echo "$out"
}

# fortuna_image_in FILE NAME: the image reference for NAME after overrides.
fortuna_image_in() {
    grep -E "image:[[:space:]]*([^[:space:]#]+/)?${2}:" "$(fortuna_manifest "$1")" \
        | sed -E 's/.*image:[[:space:]]*([^[:space:]#]+).*/\1/' | tr -d '"' | head -1
}
