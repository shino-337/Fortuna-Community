#!/usr/bin/env bash
# Renders the plain manifests in deploy/ from the Helm chart in deploy/helm/fortuna.
#
#   scripts/build/render-manifests.sh          rewrite deploy/*.yaml
#   scripts/build/render-manifests.sh --check  fail if deploy/*.yaml differs from the chart (CI)
#
# The chart is the source of truth. Edit it, then run this script.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CHART="$ROOT/deploy/helm/fortuna"
HELM="${HELM:-helm}"
# Only used to satisfy the chart's kubeVersion constraint during rendering.
KUBE_VERSION="${KUBE_VERSION:-1.31.0}"

mode="write"
case "${1:-}" in
  --check) mode="check" ;;
  "") ;;
  *) echo "usage: $0 [--check]" >&2; exit 2 ;;
esac

command -v "$HELM" >/dev/null 2>&1 || { echo "helm not found (set HELM=/path/to/helm)" >&2; exit 1; }

out_dir="$(mktemp -d)"
trap 'rm -rf "$out_dir"' EXIT

"$HELM" template fortuna "$CHART" \
  --namespace fortuna \
  --kube-version "$KUBE_VERSION" \
  -f "$CHART/ci/raw-manifests-values.yaml" \
  | python3 "$ROOT/scripts/build/split_rendered_manifests.py" "$out_dir"

status=0
while IFS= read -r -d '' file; do
  rel="${file#"$out_dir"/}"
  target="$ROOT/deploy/$rel"
  if [ "$mode" = "check" ]; then
    if ! diff -u "$target" "$file" >/dev/null 2>&1; then
      echo "deploy/$rel is out of date with deploy/helm/fortuna" >&2
      diff -u "$target" "$file" >&2 || true
      status=1
    fi
  else
    mkdir -p "$(dirname "$target")"
    cp "$file" "$target"
    echo "wrote deploy/$rel"
  fi
done < <(find "$out_dir" -type f -name '*.yaml' -print0 | sort -z)

if [ "$status" -ne 0 ]; then
  echo "Run scripts/build/render-manifests.sh and commit the result." >&2
fi
exit "$status"
