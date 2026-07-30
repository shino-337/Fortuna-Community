#!/usr/bin/env bash
# Create a Fortuna GitHub release and ensure the matching git tag is pushed.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

usage() {
  cat <<'EOF'
Usage:
  scripts/utils/create-github-release.sh <vX.Y.Z>

Environment:
  REPO        GitHub repository, default: shino-337/Fortuna-Community
  NOTES_FILE  Release notes file, default: RELEASE_NOTES_<tag>.md if present

This script pushes the git tag first. The pushed v* tag triggers
.github/workflows/publish-images.yml, which publishes:
  ghcr.io/<repo-lower>/fortuna-core:<tag>
  ghcr.io/<repo-lower>/fortuna-agent:<tag>
  ghcr.io/<repo-lower>/fortuna-dashboard:<tag>
and sha-<12-char-commit> aliases.
EOF
}

TAG="${1:-${TAG:-}}"
REPO="${REPO:-shino-337/Fortuna-Community}"

if [ -z "$TAG" ]; then
  usage
  exit 2
fi

case "$TAG" in
  v*) ;;
  *)
    echo "Release tag must start with 'v' so the image publish workflow runs: $TAG" >&2
    exit 2
    ;;
esac

cd "$PROJECT_ROOT"

if ! command -v gh >/dev/null 2>&1; then
  echo "GitHub CLI is required. Install gh or create the release manually." >&2
  exit 1
fi

if ! gh auth status >/dev/null 2>&1; then
  echo "GitHub CLI is not authenticated. Run: gh auth login" >&2
  exit 1
fi

if ! git diff --quiet || ! git diff --cached --quiet; then
  echo "Worktree has uncommitted changes. Commit or stash them before tagging a release." >&2
  exit 1
fi

if git rev-parse -q --verify "refs/tags/$TAG" >/dev/null; then
  echo "Local tag exists: $TAG"
else
  git tag -a "$TAG" -m "Fortuna Platform $TAG"
  echo "Created local annotated tag: $TAG"
fi

if git ls-remote --exit-code --tags origin "refs/tags/$TAG" >/dev/null 2>&1; then
  echo "Remote tag already exists: $TAG"
else
  git push origin "$TAG"
  echo "Pushed tag: $TAG"
fi

NOTES_FILE="${NOTES_FILE:-RELEASE_NOTES_${TAG}.md}"
RELEASE_ARGS=(release create "$TAG" --repo "$REPO" --title "Fortuna Platform $TAG")
if [ -f "$NOTES_FILE" ]; then
  RELEASE_ARGS+=(--notes-file "$NOTES_FILE")
else
  RELEASE_ARGS+=(--generate-notes)
fi

if gh release view "$TAG" --repo "$REPO" >/dev/null 2>&1; then
  echo "GitHub Release already exists: https://github.com/$REPO/releases/tag/$TAG"
else
  gh "${RELEASE_ARGS[@]}"
  echo "Created GitHub Release: https://github.com/$REPO/releases/tag/$TAG"
fi

repo_lc="$(printf '%s' "$REPO" | tr '[:upper:]' '[:lower:]')"
short_sha="$(git rev-parse --short=12 "$TAG")"
cat <<EOF

Expected image tags after the workflow completes:
  ghcr.io/${repo_lc}/fortuna-core:${TAG}
  ghcr.io/${repo_lc}/fortuna-agent:${TAG}
  ghcr.io/${repo_lc}/fortuna-dashboard:${TAG}
  ghcr.io/${repo_lc}/fortuna-core:sha-${short_sha}
  ghcr.io/${repo_lc}/fortuna-agent:sha-${short_sha}
  ghcr.io/${repo_lc}/fortuna-dashboard:sha-${short_sha}
EOF
