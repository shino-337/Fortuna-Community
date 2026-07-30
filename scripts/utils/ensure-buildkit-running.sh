#!/usr/bin/env bash
# Ensure root buildkitd is running (systemd) so nerdctl/buildctl can build Fortuna images.
# Safe no-op if buildkit already active or systemd unavailable. Used by ops before pipeline.
set -euo pipefail
command -v systemctl >/dev/null 2>&1 || { echo "No systemctl; skip."; exit 0; }
if systemctl is-active --quiet buildkit 2>/dev/null; then
  echo "buildkit already active"
  exit 0
fi
if [ "$(id -u)" = 0 ]; then
  systemctl start buildkit 2>/dev/null || true
elif command -v sudo >/dev/null 2>&1 && sudo -n true 2>/dev/null; then
  sudo -n systemctl start buildkit 2>/dev/null || true
else
  echo "WARN: cannot start buildkit (need root or passwordless sudo). Run: sudo systemctl start buildkit" >&2
  exit 0
fi
sleep 1
if systemctl is-active --quiet buildkit 2>/dev/null; then
  echo "buildkit started"
else
  echo "WARN: buildkit still not active after start attempt" >&2
fi
