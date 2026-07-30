#!/usr/bin/env bash

# Shared helpers for E2E/verify scripts.
# shellcheck shell=bash

set -o pipefail

NAMESPACE="${NAMESPACE:-fortuna}"
E2E_ADMIN_USER="${E2E_ADMIN_USER:-admin}"
E2E_ADMIN_PASS="${E2E_ADMIN_PASS:-${FORTUNA_ADMIN_PASSWORD:-${FORTUNA_DEFAULT_ADMIN_PASSWORD:-Fortuna_ChangeMe_123!}}}"

blue() { echo -e "\033[0;34m$*\033[0m"; }
green() { echo -e "\033[0;32m$*\033[0m"; }
yellow() { echo -e "\033[1;33m$*\033[0m"; }
red() { echo -e "\033[0;31m$*\033[0m"; }

get_core_pod() {
  kubectl -n "$NAMESPACE" get pods -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null
}

get_agent_pod() {
  kubectl -n "$NAMESPACE" get pods -l app.kubernetes.io/component=agent -o jsonpath='{.items[0].metadata.name}' 2>/dev/null
}

get_postgres_pod() {
  kubectl -n "$NAMESPACE" get pods -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null
}

require_core_pod() {
  local pod
  pod="$(get_core_pod)"
  if [ -z "$pod" ]; then
    red "❌ Core pod not found in namespace $NAMESPACE"
    return 1
  fi
  echo "$pod"
}

require_postgres_pod() {
  local pod
  pod="$(get_postgres_pod)"
  if [ -z "$pod" ]; then
    red "❌ Postgres pod not found in namespace $NAMESPACE"
    return 1
  fi
  echo "$pod"
}

get_jwt_token() {
  local core_pod="${1:-}"
  [ -z "$core_pod" ] && core_pod="$(require_core_pod)" || true
  kubectl -n "$NAMESPACE" exec "$core_pod" -- curl -s -X POST \
    http://localhost:8080/api/v1/auth/login \
    -H "Content-Type: application/json" \
    -d "{\"username\":\"$E2E_ADMIN_USER\",\"password\":\"$E2E_ADMIN_PASS\"}" \
    | python3 -c 'import sys,json
try:
 d=json.load(sys.stdin)
 print(d.get("token",""))
except Exception:
 print("")
'
}

require_jwt_token() {
  local core_pod="${1:-}"
  local token
  token="$(get_jwt_token "$core_pod")"
  if [ -z "$token" ]; then
    red "❌ Failed to obtain JWT token with $E2E_ADMIN_USER/$E2E_ADMIN_PASS"
    return 1
  fi
  echo "$token"
}

core_api_get() {
  local path="$1"
  local token="$2"
  local core_pod="$3"
  kubectl -n "$NAMESPACE" exec "$core_pod" -- curl -s \
    -H "Authorization: Bearer $token" \
    "http://localhost:8080/api/v1/$path"
}

core_api_post_json() {
  local path="$1"
  local token="$2"
  local core_pod="$3"
  local payload="$4"
  kubectl -n "$NAMESPACE" exec "$core_pod" -- curl -s -X POST \
    -H "Authorization: Bearer $token" \
    -H "Content-Type: application/json" \
    -d "$payload" \
    "http://localhost:8080/api/v1/$path"
}

wait_for_pod_in_db() {
  local pod_uid="$1"
  local timeout="${2:-180}"
  local pg_pod="$3"
  local waited=0
  while [ "$waited" -lt "$timeout" ]; do
    local found
    found="$(kubectl -n "$NAMESPACE" exec "$pg_pod" -- psql -U postgres -d fortuna -t -A -c \
      "SELECT 1 FROM pods WHERE uid = '$pod_uid' AND deleted_at IS NULL LIMIT 1;" 2>/dev/null | tr -d ' ')"
    if [ "$found" = "1" ]; then
      echo "$waited"
      return 0
    fi
    sleep 5
    waited=$((waited + 5))
  done
  return 1
}

e2e_node_selector_yaml() {
  local indent="${1:-2}"
  local host="${E2E_NODE_SELECTOR_HOST:-}"
  [ -z "$host" ] && return 0
  printf '%*snodeSelector:\n' "$indent" ''
  printf '%*skubernetes.io/hostname: %s\n' "$((indent + 2))" '' "$host"
}
