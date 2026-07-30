#!/usr/bin/env bash
set -euo pipefail

# Verify worker-node health for Kubernetes runtime/network stability.
# Run from control-plane host with kubectl access.

NAMESPACE_APP="${NAMESPACE_APP:-fortuna}"
MIN_READY_WINDOW_SEC="${MIN_READY_WINDOW_SEC:-120}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

PASS=0
WARN=0
FAIL=0

ok() {
  echo -e "${GREEN}[PASS]${NC} $*"
  PASS=$((PASS + 1))
}

warn() {
  echo -e "${YELLOW}[WARN]${NC} $*"
  WARN=$((WARN + 1))
}

fail() {
  echo -e "${RED}[FAIL]${NC} $*"
  FAIL=$((FAIL + 1))
}

title() {
  echo
  echo -e "${BLUE}==== $* ====${NC}"
}

require_tools() {
  title "Preflight"
  if ! command -v kubectl >/dev/null 2>&1; then
    echo "kubectl not found"
    exit 2
  fi
  ok "kubectl exists"
}

check_nodes() {
  title "Node Status"
  local out
  out="$(kubectl get nodes --no-headers 2>/dev/null || true)"
  if [[ -z "$out" ]]; then
    fail "Cannot list nodes"
    return
  fi

  while IFS= read -r line; do
    local name status roles version
    name="$(awk '{print $1}' <<<"$line")"
    status="$(awk '{print $2}' <<<"$line")"
    roles="$(awk '{print $3}' <<<"$line")"
    version="$(awk '{print $5}' <<<"$line")"

    if [[ "$roles" == "<none>" ]]; then
      if [[ "$status" == *"Ready"* && "$status" != *"SchedulingDisabled"* ]]; then
        ok "worker ${name} is Ready (k8s ${version})"
      elif [[ "$status" == *"SchedulingDisabled"* ]]; then
        warn "worker ${name} is cordoned (${status})"
      else
        fail "worker ${name} not healthy (${status})"
      fi
    fi
  done <<<"$out"
}

check_kube_system_daemonsets() {
  title "kube-system DaemonSets"
  local ds_list=("kube-proxy")
  if kubectl get ns kube-flannel >/dev/null 2>&1; then
    if kubectl -n kube-flannel get ds kube-flannel-ds >/dev/null 2>&1; then
      ds_list+=("kube-flannel/kube-flannel-ds")
    fi
  fi

  local item
  for item in "${ds_list[@]}"; do
    local ns name desired ready
    if [[ "$item" == *"/"* ]]; then
      ns="${item%%/*}"
      name="${item##*/}"
    else
      ns="kube-system"
      name="$item"
    fi
    if ! kubectl -n "$ns" get ds "$name" >/dev/null 2>&1; then
      warn "DaemonSet ${ns}/${name} not found"
      continue
    fi
    desired="$(kubectl -n "$ns" get ds "$name" -o jsonpath='{.status.desiredNumberScheduled}')"
    ready="$(kubectl -n "$ns" get ds "$name" -o jsonpath='{.status.numberReady}')"
    if [[ "$desired" == "$ready" ]]; then
      ok "DaemonSet ${ns}/${name} ready ${ready}/${desired}"
    else
      fail "DaemonSet ${ns}/${name} ready ${ready}/${desired}"
    fi
  done
}

check_recent_events() {
  title "Recent Unhealthy Events"
  local events
  events="$(kubectl get events -A --sort-by=.lastTimestamp --no-headers 2>/dev/null || true)"
  if [[ -z "$events" ]]; then
    warn "No events returned (or permission issue)"
    return
  fi

  local hit=0
  while IFS= read -r line; do
    # format: NS LASTSEEN TYPE REASON OBJECT MESSAGE...
    local ns reason object
    ns="$(awk '{print $1}' <<<"$line")"
    reason="$(awk '{print $4}' <<<"$line")"
    object="$(awk '{print $5}' <<<"$line")"
    if [[ "$reason" == "SandboxChanged" || "$reason" == "BackOff" || "$reason" == "Failed" || "$reason" == "Unhealthy" ]]; then
      echo "  ${ns} ${reason} ${object}"
      hit=1
    fi
  done <<<"$events"

  if [[ "$hit" -eq 1 ]]; then
    warn "Found unstable events (SandboxChanged/BackOff/Failed/Unhealthy)"
  else
    ok "No unstable events detected"
  fi
}

check_fortuna_workloads() {
  title "Fortuna Workloads"
  if ! kubectl get ns "$NAMESPACE_APP" >/dev/null 2>&1; then
    warn "Namespace ${NAMESPACE_APP} not found"
    return
  fi

  local bad
  bad="$(kubectl -n "$NAMESPACE_APP" get pods --no-headers 2>/dev/null | awk '{
    split($2, a, "/");
    if (a[1] != a[2] || ($3 != "Running" && $3 != "Completed")) print $0
  }')"
  if [[ -n "$bad" ]]; then
    warn "Some pods are not fully ready in ${NAMESPACE_APP}:"
    echo "$bad"
  else
    ok "All pods ready in ${NAMESPACE_APP}"
  fi

  # NATS-specific quick health
  if kubectl -n "$NAMESPACE_APP" get sts nats >/dev/null 2>&1; then
    local nrep nready
    nrep="$(kubectl -n "$NAMESPACE_APP" get sts nats -o jsonpath='{.spec.replicas}')"
    nready="$(kubectl -n "$NAMESPACE_APP" get sts nats -o jsonpath='{.status.readyReplicas}')"
    nready="${nready:-0}"
    if [[ "$nready" == "$nrep" ]]; then
      ok "NATS StatefulSet ready ${nready}/${nrep}"
    else
      fail "NATS StatefulSet ready ${nready}/${nrep}"
    fi
  fi
}

print_summary() {
  title "Summary"
  echo "PASS: ${PASS}"
  echo "WARN: ${WARN}"
  echo "FAIL: ${FAIL}"
  if [[ "$FAIL" -gt 0 ]]; then
    echo -e "${RED}Worker environment is NOT stable yet.${NC}"
    exit 1
  fi
  echo -e "${GREEN}Worker environment baseline looks healthy.${NC}"
}

main() {
  require_tools
  check_nodes
  check_kube_system_daemonsets
  check_recent_events
  check_fortuna_workloads
  print_summary
}

main "$@"
