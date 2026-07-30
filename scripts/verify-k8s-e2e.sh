#!/usr/bin/env bash
# Fortuna live-cluster E2E verification (attack paths + chains + risk V3).
# Prerequisites: kubectl, curl, jq; workloads from scenarios/ applied; Fortuna synced inventory.
set -euo pipefail

FORTUNA_API_URL="${FORTUNA_API_URL:-}"
FORTUNA_CLUSTER_ID="${FORTUNA_CLUSTER_ID:-}"
FORTUNA_JWT="${FORTUNA_JWT:-}"
FORTUNA_NAMESPACE="${FORTUNA_NAMESPACE:-fortuna-test}"
CURL_INSECURE="${CURL_INSECURE:-0}"

die() { echo "ERROR: $*" >&2; exit 1; }
log() { echo "[verify-k8s-e2e] $*"; }
need_cmd() { command -v "$1" >/dev/null 2>&1 || die "missing command: $1"; }

need_cmd kubectl
need_cmd curl
need_cmd jq

[[ -n "$FORTUNA_API_URL" ]] || die "set FORTUNA_API_URL"
[[ -n "$FORTUNA_CLUSTER_ID" ]] || die "set FORTUNA_CLUSTER_ID"

API_BASE="${FORTUNA_API_URL%/}"
CURL_COMMON=(-sS)
[[ "$CURL_INSECURE" == "1" ]] && CURL_COMMON+=(-k)
HDR=()
[[ -n "${FORTUNA_JWT:-}" ]] && HDR=(-H "Authorization: Bearer ${FORTUNA_JWT}")

escape_cluster() { printf %s "$1" | jq -sRr @uri; }

pod_uid() {
  kubectl get pod "$1" -n "$FORTUNA_NAMESPACE" -o jsonpath='{.metadata.uid}'
}

api_get() { curl "${CURL_COMMON[@]}" "${HDR[@]}" "${API_BASE}$1"; }
api_post_empty() { curl "${CURL_COMMON[@]}" "${HDR[@]}" -X POST "${API_BASE}$1" >/dev/null || true; }

risk_recalc() { api_post_empty "/api/v1/risk/scores/$1/calculate?mode=v3"; }

risk_json() {
  local uid=$1
  api_get "/api/v1/risk/scores/${uid}?cluster=$(escape_cluster "$FORTUNA_CLUSTER_ID")"
}

paths_for_pod() { api_get "/api/v1/graph/attack-paths/$1?max_depth=10"; }

bundle_json() {
  api_get "/api/v1/graph/attack-paths/bundle?cluster_id=$(escape_cluster "$FORTUNA_CLUSTER_ID")"
}

# Chains whose path_ids overlap paths that include pod uid in nodes.
chains_for_pod() {
  local uid=$1
  local bundle=$2
  jq --arg u "$uid" '
    .data.paths as $paths
    | ($paths | map(select((.nodes // []) | map(.id) | index($u) != null)) | map(.path_id)) as $pids
    | if ($pids | length) == 0 then []
      else [.data.chains[] | select([.paths[] as $p | ($pids | index($p)) != null] | any)]
      end
  ' <<<"$bundle"
}

has_escape_technique() {
  jq -e '[.[] | .steps[]?.technique_id // empty] | map(test("^ESCAPE_")) | any' <<<"$1" >/dev/null
}

main() {
  log "API=${API_BASE} cluster_id=${FORTUNA_CLUSTER_ID} ns=${FORTUNA_NAMESPACE}"

  local U1 U2 U3 U4 U5
  U1=$(pod_uid escape-pod)
  U2=$(pod_uid rbac-pod)
  U3=$(pod_uid noisy-scanner)
  U4=$(pod_uid lateral-pod)
  U5=$(pod_uid broken-chain)
  log "pod UIDs: S1=$U1 S2=$U2 S3=$U3 S4=$U4 S5=$U5"

  log "GET attack-paths bundle..."
  local bundle
  bundle=$(bundle_json)
  echo "$bundle" | jq -e .data >/dev/null || die "bundle response missing .data — check API URL / auth / cluster_id"

  local CH1 CH2 CH3 CH4 CH5
  CH1=$(chains_for_pod "$U1" "$bundle")
  CH2=$(chains_for_pod "$U2" "$bundle")
  CH3=$(chains_for_pod "$U3" "$bundle")
  CH4=$(chains_for_pod "$U4" "$bundle")
  CH5=$(chains_for_pod "$U5" "$bundle")

  local p1c
  p1c=$(paths_for_pod "$U1" | jq '.count // (.paths | length)')
  [[ "${p1c:-0}" =~ ^[0-9]+$ ]] || p1c=0
  [[ "$p1c" -gt 0 ]] || die "S1: no attack paths for escape-pod (count=$p1c). Wait for inventory/path reconcile."

  echo "$bundle" | jq -e '.data.chains | map(select(.type == "ESCAPE_TO_PRIV_ESC")) | length > 0' >/dev/null \
    || die "S1: cluster bundle must include ESCAPE_TO_PRIV_ESC chain (attack-path detection)"

  echo "$bundle" | jq -e '.data.chains | map(select(.type == "ESCAPE_TO_PRIV_ESC")) | .[0].steps[]?.technique_id | select(. == "ESCAPE_HOSTPATH")' >/dev/null \
    || log "WARN S1: ESCAPE_HOSTPATH step missing on global ESCAPE_TO_PRIV_ESC chain."

  local path_rbac
  path_rbac=$(paths_for_pod "$U2")
  echo "$path_rbac" | jq -e 'all(.paths[]?; (.explainability.class // "") != "ESCAPE_PATH")' >/dev/null 2>&1 \
    || die "S2: rbac-only pod must not carry ESCAPE_PATH class"

  if has_escape_technique "$CH2"; then
    log "WARN S2: cluster-level chain list for this pod includes ESCAPE_* (bundle merges multi-pod paths; verify per-path classes above)."
  fi

  local gaps
  gaps=$(echo "$CH5" | jq '[.[].capability_validation.gaps[]? // empty] | length')
  [[ "${gaps:-0}" -eq 0 ]] && log "WARN S5: zero capability_validation gaps — may be OK if chain soft-validates."

  local i u
  for u in "$U1" "$U2" "$U3" "$U4" "$U5"; do
    risk_recalc "$u"
  done
  sleep 1

  local R1 R2 R3 R4 R5
  R1=$(risk_json "$U1"); R2=$(risk_json "$U2"); R3=$(risk_json "$U3")
  R4=$(risk_json "$U4"); R5=$(risk_json "$U5")

  score() { jq '.totalScore // .total_score // 0' <<<"$1"; }
  local SC1 SC2 SC3 SC4 SC5
  SC1=$(score "$R1"); SC2=$(score "$R2"); SC3=$(score "$R3"); SC4=$(score "$R4"); SC5=$(score "$R5")

  awk -v x="$SC1" 'BEGIN{exit !(x>0)}' || die "S1: totalScore must be > 0 (got $SC1)"
  awk -v x="$SC2" 'BEGIN{exit !(x>0)}' || log "WARN S2: totalScore is 0"
  awk -v a="$SC1" -v b="$SC2" 'BEGIN{exit !(a>b)}' || log "WARN ordering: expected S1 ($SC1) > S2 ($SC2)"
  awk -v a="$SC2" -v b="$SC5" 'BEGIN{exit !(a>b)}' || log "WARN ordering: expected S2 ($SC2) > S5 ($SC5)"

  # S3 noisy: mean correlation_precision
  local n cpSum
  n=$(echo "$CH3" | jq '[.[].mitre_summary.correlation_precision // empty] | length')
  if [[ "${n:-0}" -gt 0 ]]; then
    cpSum=$(echo "$CH3" | jq '[.[].mitre_summary.correlation_precision // empty] | add')
    awk -v s="$cpSum" -v c="$n" 'BEGIN{exit !((s/c) < 0.6)}' \
      && log "S3: mean correlation_precision < 0.6 OK" \
      || log "WARN S3: mean correlation_precision not < 0.6 (need runtime noise in DB)"
  else
    log "WARN S3: no mitre_summary.correlation_precision on chains (runtime_events likely empty)."
  fi

  echo "$CH4" | jq -e '[.[].steps[]?.technique_id] | index("SA_TOKEN_REUSE") != null' >/dev/null \
    && log "S4: SA_TOKEN_REUSE on chain OK" \
    || log "WARN S4: SA_TOKEN_REUSE not found — inspect graph path labels."

  local mb1 mb3
  mb1=$(echo "$R1" | jq -r '(if (.factors | type) == "string" then (.factors | fromjson) else (.factors // {}) end) | .mitre_runtime_attack_path_boost // 0' 2>/dev/null || echo 0)
  mb3=$(echo "$R3" | jq -r '(if (.factors | type) == "string" then (.factors | fromjson) else (.factors // {}) end) | .mitre_runtime_attack_path_boost // 0' 2>/dev/null || echo 0)
  awk -v a="$mb3" -v b="$mb1" 'BEGIN{exit !(a <= b+2.0)}' \
    && log "S3 MITRE boost ($mb3) vs S1 ($mb1): no wild inflation vs +2 guard" \
    || log "WARN S3: MITRE boost possibly inflated vs S1"

  log "DONE (review WARN lines — re-run after Fortuna full sync if needed)."
}

main "$@"
