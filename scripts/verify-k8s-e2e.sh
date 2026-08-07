#!/usr/bin/env bash
# Fortuna live-cluster E2E verification (attack paths + chains + risk V3).
# Prerequisites: kubectl, curl, jq; workloads from scenarios/ applied; Fortuna synced inventory.
set -euo pipefail

FORTUNA_API_URL="${FORTUNA_API_URL:-}"
FORTUNA_CLUSTER_ID="${FORTUNA_CLUSTER_ID:-}"
FORTUNA_JWT="${FORTUNA_JWT:-}"
FORTUNA_NAMESPACE="${FORTUNA_NAMESPACE:-fortuna-test}"
CURL_INSECURE="${CURL_INSECURE:-0}"

fail() { echo "FAIL: $*" >&2; exit 1; }
warn() { echo "WARN: $*" >&2; }
pass() { echo "PASS: $*"; }
log() { echo "[verify-k8s-e2e] $*"; }
need_cmd() { command -v "$1" >/dev/null 2>&1 || fail "missing command: $1"; }

need_cmd kubectl
need_cmd curl
need_cmd jq
need_cmd awk

[[ -n "$FORTUNA_API_URL" ]] || fail "set FORTUNA_API_URL"
[[ -n "$FORTUNA_CLUSTER_ID" ]] || fail "set FORTUNA_CLUSTER_ID"

API_BASE="${FORTUNA_API_URL%/}"
CURL_COMMON=(-fsS)
[[ "$CURL_INSECURE" == "1" ]] && CURL_COMMON+=(-k)
HDR=()
[[ -n "${FORTUNA_JWT:-}" ]] && HDR=(-H "Authorization: Bearer ${FORTUNA_JWT}")

escape_cluster() { printf %s "$1" | jq -sRr @uri; }

pod_uid() {
  kubectl get pod "$1" -n "$FORTUNA_NAMESPACE" -o jsonpath='{.metadata.uid}'
}

api_get() { curl "${CURL_COMMON[@]}" "${HDR[@]}" "$API_BASE$1"; }
api_post_empty() { curl "${CURL_COMMON[@]}" "${HDR[@]}" -X POST "$API_BASE$1" >/dev/null; }

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

chain_has_type() {
  local chains=$1
  local type=$2
  jq -e --arg t "$type" 'map(select(.type == $t)) | length > 0' <<<"$chains" >/dev/null
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
  echo "$bundle" | jq -e .data >/dev/null || fail "bundle response missing .data — check API URL / auth / cluster_id"

  local CH1 CH2 CH3 CH4 CH5
  CH1=$(chains_for_pod "$U1" "$bundle")
  CH2=$(chains_for_pod "$U2" "$bundle")
  CH3=$(chains_for_pod "$U3" "$bundle")
  CH4=$(chains_for_pod "$U4" "$bundle")
  CH5=$(chains_for_pod "$U5" "$bundle")

  # S1: the evidence must belong to escape-pod, not merely exist somewhere in the cluster bundle.
  local path_s1 path_s2 path_s5 p1c p2c p5c
  path_s1=$(paths_for_pod "$U1")
  path_s2=$(paths_for_pod "$U2")
  path_s5=$(paths_for_pod "$U5")

  p1c=$(jq '.count // (.paths | length) // 0' <<<"$path_s1")
  p2c=$(jq '.count // (.paths | length) // 0' <<<"$path_s2")
  p5c=$(jq '.count // (.paths | length) // 0' <<<"$path_s5")

  [[ "$p1c" =~ ^[0-9]+$ ]] || p1c=0
  [[ "$p2c" =~ ^[0-9]+$ ]] || p2c=0
  [[ "$p5c" =~ ^[0-9]+$ ]] || p5c=0

  [[ "$p1c" -gt 0 ]] || fail "S1: no attack paths for escape-pod (count=$p1c). Wait for inventory/path reconcile."
  pass "S1: escape-pod has $p1c attack path(s)"

  chain_has_type "$CH1" "ESCAPE_TO_PRIV_ESC" \
    && pass "S1: escape-pod has ESCAPE_TO_PRIV_ESC chain" \
    || fail "S1: escape-pod has no ESCAPE_TO_PRIV_ESC chain"

  echo "$CH1" | jq -e '[.[].steps[]?.technique_id // empty] | index("ESCAPE_HOSTPATH") != null' >/dev/null \
    && pass "S1: ESCAPE_HOSTPATH evidence present" \
    || fail "S1: ESCAPE_HOSTPATH evidence missing"

  # S2: RBAC escalation must not be mislabeled as a host/escape path.
  [[ "$p2c" -gt 0 ]] || fail "S2: no attack paths for rbac-pod (count=$p2c)"
  echo "$path_s2" | jq -e 'all(.paths[]?; (.explainability.class // "") != "ESCAPE_PATH")' >/dev/null \
    && pass "S2: RBAC-only path is not classified as ESCAPE_PATH" \
    || fail "S2: RBAC-only pod was classified as ESCAPE_PATH"

  # S5: intentionally broken chain must not produce an escape chain.
  if chain_has_type "$CH5" "ESCAPE_TO_PRIV_ESC"; then
    fail "S5: broken-chain produced ESCAPE_TO_PRIV_ESC"
  else
    pass "S5: broken-chain does not produce ESCAPE_TO_PRIV_ESC"
  fi

  # S3/S4 runtime assertions are optional because runtime ingestion may not be enabled.
  local n cpSum
  n=$(jq '[.[].mitre_summary.correlation_precision // empty] | length' <<<"$CH3")
  if [[ "${n:-0}" -gt 0 ]]; then
    cpSum=$(jq '[.[].mitre_summary.correlation_precision // empty] | add' <<<"$CH3")
    if awk -v s="$cpSum" -v c="$n" 'BEGIN{exit !((s/c) < 0.6)}'; then
      pass "S3: mean correlation_precision < 0.6"
    else
      warn "S3: mean correlation_precision is not < 0.6"
    fi
  else
    warn "S3: no correlation_precision evidence; runtime ingestion may be disabled"
  fi

  if echo "$CH4" | jq -e '[.[].steps[]?.technique_id // empty] | index("SA_TOKEN_REUSE") != null' >/dev/null; then
    pass "S4: SA_TOKEN_REUSE evidence present"
  else
    warn "S4: SA_TOKEN_REUSE evidence not found; runtime correlation may be disabled"
  fi

  # Risk checks are intentionally bounded and evidence-based.
  local u R1 R2 R5 SC1 SC2 SC5
  for u in "$U1" "$U2" "$U5"; do
    risk_recalc "$u"
  done
  sleep 1

  R1=$(risk_json "$U1")
  R2=$(risk_json "$U2")
  R5=$(risk_json "$U5")

  score() { jq '.totalScore // .total_score // 0' <<<"$1"; }
  SC1=$(score "$R1"); SC2=$(score "$R2"); SC5=$(score "$R5")

  awk -v x="$SC1" 'BEGIN{exit !(x>0)}' \
    && pass "S1: totalScore > 0 ($SC1)" \
    || fail "S1: totalScore must be > 0 (got $SC1)"

  awk -v x="$SC2" 'BEGIN{exit !(x>0)}' \
    && pass "S2: totalScore > 0 ($SC2)" \
    || warn "S2: totalScore is 0"

  awk -v a="$SC1" -v b="$SC2" 'BEGIN{exit !(a>b)}' \
    && pass "Risk ordering: S1 ($SC1) > S2 ($SC2)" \
    || warn "Risk ordering: expected S1 ($SC1) > S2 ($SC2)"

  awk -v a="$SC2" -v b="$SC5" 'BEGIN{exit !(a>b)}' \
    && pass "Risk ordering: S2 ($SC2) > S5 ($SC5)" \
    || warn "Risk ordering: expected S2 ($SC2) > S5 ($SC5)"

  log "Verification complete. PASS assertions are required evidence; WARN assertions require optional runtime prerequisites."
}

main "$@"
