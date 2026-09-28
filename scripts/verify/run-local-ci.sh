#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$repo_root"

usage() {
  printf 'Usage: %s [list|all|hygiene|scripts|go-test|cluster-identity-postgres|dashboard]\n' "$0"
}

if (( $# > 1 )); then
  usage >&2
  exit 2
fi

selection=${1:-all}
case "$selection" in
  list|all|hygiene|scripts|go-test|cluster-identity-postgres|dashboard) ;;
  -h|--help) usage; exit 0 ;;
  *) usage >&2; exit 2 ;;
esac

if ! command -v act >/dev/null 2>&1; then
  printf 'act is required; see docs/05-operations/LOCAL_CI.md\n' >&2
  exit 1
fi
if ! docker info >/dev/null 2>&1; then
  printf 'Docker daemon is unavailable; act needs a working Docker socket.\n' >&2
  exit 1
fi

workflow=.github/workflows/ci.yml
if [[ $selection == list ]]; then
  exec act -l -W "$workflow"
fi

act --validate -W "$workflow"

run_job() {
  local job=$1
  local module
  if [[ $job == go-test ]]; then
    # act can start all matrix entries together even with one concurrent job.
    for module in core agent api; do
      printf '\n=== Local CI: go-test (%s) ===\n' "$module"
      act workflow_dispatch -W "$workflow" -j go-test --matrix "module:$module"
    done
  else
    printf '\n=== Local CI: %s ===\n' "$job"
    act workflow_dispatch -W "$workflow" -j "$job"
  fi
}

if [[ $selection == all ]]; then
  # Keep the PostgreSQL service and the Go matrix from competing for VM resources.
  for job in hygiene scripts go-test cluster-identity-postgres dashboard; do
    run_job "$job"
  done
else
  run_job "$selection"
fi
