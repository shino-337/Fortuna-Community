#!/bin/bash
# ============================================================================
# Manage Port-Forwards - Centralized Control
# ============================================================================
# Usage:
#   ./scripts/utils/manage-port-forwards.sh status    # Show all port-forwards
#   ./scripts/utils/manage-port-forwards.sh stop     # Stop all port-forwards
#   ./scripts/utils/manage-port-forwards.sh start    # Start common port-forwards
#   ./scripts/utils/manage-port-forwards.sh clean    # Clean all and verify
#   PORT_FORWARD_ADDRESS=0.0.0.0 ./scripts/utils/manage-port-forwards.sh start
# ============================================================================

set -euo pipefail

export PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:${PATH:-}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

NAMESPACE="${NAMESPACE:-fortuna}"
PORT_FORWARD_ADDRESS="${PORT_FORWARD_ADDRESS:-127.0.0.1}"

ok()  { echo -e "${GREEN}✅${NC} $*"; }
fail() { echo -e "${RED}❌${NC} $*"; }
warn() { echo -e "${YELLOW}⚠️${NC}  $*"; }
info() { echo -e "${BLUE}[INFO]${NC} $*"; }

port_listening() {
  local port="$1"
  ss -ltn 2>/dev/null | awk '{print $4}' | grep -Eq "(^|:)${port}$"
}

wait_for_port() {
  local port="$1"
  local attempts="${2:-20}"
  local i
  for ((i = 1; i <= attempts; i++)); do
    if port_listening "$port"; then
      return 0
    fi
    sleep 0.25
  done
  return 1
}

start_port_forward() {
  local log_file="$1"
  shift
  : > "$log_file"
  setsid -f kubectl port-forward --address "$PORT_FORWARD_ADDRESS" -n "$NAMESPACE" "$@" > "$log_file" 2>&1 < /dev/null
}

local_url() {
  local port="$1"
  if [ "$PORT_FORWARD_ADDRESS" = "127.0.0.1" ] || [ "$PORT_FORWARD_ADDRESS" = "localhost" ] || [ "$PORT_FORWARD_ADDRESS" = "0.0.0.0" ] || [ "$PORT_FORWARD_ADDRESS" = "::" ]; then
    echo "http://localhost:${port}"
    return 0
  fi
  echo "http://${PORT_FORWARD_ADDRESS}:${port}"
}

wait_for_http() {
  local url="$1"
  local attempts="${2:-20}"
  local i
  command -v curl >/dev/null 2>&1 || return 0
  for ((i = 1; i <= attempts; i++)); do
    if curl --noproxy '*' -fsS "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.25
  done
  return 1
}

show_status() {
  echo "=========================================="
  echo "Port-Forward Status"
  echo "=========================================="
  echo ""
  
  local count=0
  while IFS= read -r line; do
    [ -n "$line" ] && echo "  $line" && count=$((count + 1))
  done < <(pgrep -af "kubectl.*port-forward" 2>/dev/null | grep -v "manage-port-forwards" || true)
  
  if [ "${count:-0}" -eq 0 ]; then
    ok "No port-forward processes running"
  else
    warn "$count port-forward process(es) running"
  fi
  echo ""
  
  info "Ports in use:"
  local ports=$(lsof -i -P -n 2>/dev/null | grep LISTEN | grep -E "8080|8081|5432|9090|18080|28080|28081|29090" || echo "")
  if [ -n "$ports" ]; then
    echo "$ports" | while read -r line; do
      echo "  $line"
    done
  else
    ok "No common port-forward ports in use"
  fi
  echo ""
  info "Configured bind address: ${PORT_FORWARD_ADDRESS}"
  echo ""
}

stop_all() {
  echo "=========================================="
  echo "Stopping All Port-Forwards"
  echo "=========================================="
  echo ""
  
  local pids=$(pgrep -f "kubectl.*port-forward" 2>/dev/null || echo "")
  local count=0
  if [ -n "$pids" ]; then
    count=$(echo "$pids" | wc -l | tr -d ' ')
  fi
  if [ "$count" -gt 0 ] 2>/dev/null; then
    info "Stopping $count port-forward process(es)..."
    pkill -f "kubectl.*port-forward" 2>/dev/null || true
    sleep 2
    
    local remaining_pids=$(pgrep -f "kubectl.*port-forward" 2>/dev/null || echo "")
    local remaining=0
    if [ -n "$remaining_pids" ]; then
      remaining=$(echo "$remaining_pids" | wc -l | tr -d ' ')
    fi
    if [ "$remaining" -eq 0 ] 2>/dev/null; then
      ok "All port-forwards stopped"
    else
      warn "$remaining process(es) still running (may need force kill)"
      pkill -9 -f "kubectl.*port-forward" 2>/dev/null || true
    fi
  else
    ok "No port-forward processes to stop"
  fi
  echo ""
}

start_common() {
  echo "=========================================="
  echo "Starting Common Port-Forwards"
  echo "=========================================="
  echo ""
  
  # Check if already running
  if pgrep -f "port-forward.*fortuna-core.*8080" > /dev/null; then
    warn "Core API port-forward already running on 8080"
  else
    info "Starting Core API port-forward (${PORT_FORWARD_ADDRESS}:8080)..."
    start_port_forward /tmp/core-pf.log svc/fortuna-core 8080:8080
    if wait_for_port 8080; then
      local core_url
      core_url="$(local_url 8080)"
      if wait_for_http "${core_url}/healthz"; then
        ok "Core API: ${core_url}"
      else
        warn "Core API port is listening, but ${core_url}/healthz did not return HTTP success yet"
      fi
    else
      fail "Failed to start Core API port-forward"
      sed -n '1,80p' /tmp/core-pf.log 2>/dev/null || true
    fi
  fi
  
  if pgrep -f "port-forward.*fortuna-dashboard.*8081" > /dev/null; then
    warn "Dashboard port-forward already running on 8081"
  else
    info "Starting Dashboard port-forward (${PORT_FORWARD_ADDRESS}:8081)..."
    start_port_forward /tmp/dashboard-pf.log svc/fortuna-dashboard 8081:80
    if wait_for_port 8081; then
      local dashboard_url
      dashboard_url="$(local_url 8081)"
      if wait_for_http "$dashboard_url"; then
        ok "Dashboard: ${dashboard_url}"
      else
        warn "Dashboard port is listening, but ${dashboard_url} did not return HTTP success yet"
      fi
    else
      fail "Failed to start Dashboard port-forward"
      sed -n '1,80p' /tmp/dashboard-pf.log 2>/dev/null || true
    fi
  fi
  
  echo ""
  info "Port-forwards started. Logs:"
  echo "  Core API: /tmp/core-pf.log"
  echo "  Dashboard: /tmp/dashboard-pf.log"
  echo ""
  echo "To stop: ./scripts/utils/manage-port-forwards.sh stop"
  echo ""
}

clean_all() {
  echo "=========================================="
  echo "Clean All Port-Forwards"
  echo "=========================================="
  echo ""
  
  stop_all
  
  info "Verifying cleanup..."
  local remaining_pids=$(pgrep -f "kubectl.*port-forward" 2>/dev/null || echo "")
  local remaining=0
  if [ -n "$remaining_pids" ]; then
    remaining=$(echo "$remaining_pids" | wc -l | tr -d ' ')
  fi
  if [ "$remaining" -eq 0 ] 2>/dev/null; then
    ok "All port-forwards cleaned"
  else
    fail "$remaining process(es) still running"
    echo ""
    echo "Remaining processes:"
    pgrep -af "kubectl.*port-forward" 2>/dev/null | grep -v "manage-port-forwards" || true
  fi
  echo ""
}

case "${1:-status}" in
  status)
    show_status
    ;;
  stop)
    stop_all
    ;;
  start)
    start_common
    ;;
  clean)
    clean_all
    ;;
  *)
    echo "Usage: $0 {status|stop|start|clean}"
    echo ""
    echo "Commands:"
    echo "  status  - Show all port-forward processes and ports"
    echo "  stop    - Stop all port-forward processes"
    echo "  start   - Start common port-forwards (Core:8080, Dashboard:8081)"
    echo "  clean   - Stop all and verify cleanup"
    echo ""
    echo "Environment:"
    echo "  NAMESPACE=fortuna"
    echo "  PORT_FORWARD_ADDRESS=127.0.0.1"
    exit 1
    ;;
esac
