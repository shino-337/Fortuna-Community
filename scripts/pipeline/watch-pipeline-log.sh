#!/usr/bin/env bash
# Tail a pipeline log (e.g. from RUN_ASYNC=1) and print elapsed time periodically.
# Usage:
#   PIPELINE_LOG_FILE=/tmp/pipeline.log ./scripts/pipeline/watch-pipeline-log.sh
#   ./scripts/pipeline/watch-pipeline-log.sh /path/to/logfile
set -euo pipefail
LOG="${1:-${PIPELINE_LOG_FILE:-/tmp/clean-rebuild-deploy.log}}"
INTERVAL_SEC="${WATCH_LOG_INTERVAL_SEC:-120}"
if [ ! -f "$LOG" ]; then
  echo "Log not found: $LOG" >&2
  echo "Start the pipeline with e.g. RUN_ASYNC=1 PIPELINE_LOG_FILE=$LOG ..." >&2
  exit 1
fi
echo "Following $LOG (every ${INTERVAL_SEC}s: elapsed time). Ctrl+C to stop."
start=$(date +%s)
(
  while true; do
    sleep "$INTERVAL_SEC"
    now=$(date +%s)
    printf '\n--- %s — elapsed %s min ---\n' "$(date -Is)" "$(( (now - start) / 60 ))"
  done
) &
tick=$!
trap 'kill "$tick" 2>/dev/null || true' EXIT INT TERM
tail -n 30 -f "$LOG"
