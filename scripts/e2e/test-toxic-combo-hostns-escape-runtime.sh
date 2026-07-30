#!/usr/bin/env bash
# E2E testcase: toxic combo "host namespaces + escape runtime signals"
# Goal: create a real pod, inject escape-class runtime events, then observe:
# - pod_risk_profiles (static/runtime/total risk)
# - pod_attack_steps
# - risk report insights (toxic combo rule)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./common.sh
source "${SCRIPT_DIR}/common.sh"

TEST_NS="${TEST_NS:-fortuna-toxic-e2e}"
POD_NAME="${POD_NAME:-toxic-hostns-escape-$(date +%s)}"
WAIT_DB_TIMEOUT="${WAIT_DB_TIMEOUT:-240}"
KEEP_TEST_POD="${KEEP_TEST_POD:-false}"

echo "=========================================="
echo "Toxic Combo E2E: hostns + runtime escape"
echo "=========================================="
echo "Namespace: $TEST_NS"
echo "Pod name:  $POD_NAME"
echo ""

kubectl create namespace "$TEST_NS" --dry-run=client -o yaml | kubectl apply -f - >/dev/null

cat <<POD_EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: $POD_NAME
  namespace: $TEST_NS
  labels:
    app: toxic-combo-e2e
    fortuna-testcase: toxic-hostns-escape-runtime
spec:
  hostNetwork: true
  hostPID: true
  hostIPC: true
  restartPolicy: Never
  tolerations:
  - key: "node-role.kubernetes.io/control-plane"
    operator: "Exists"
    effect: "NoSchedule"
  containers:
  - name: test
    image: busybox:latest
    command: ["sh", "-c", "sleep 3600"]
    securityContext:
      privileged: true
POD_EOF

kubectl wait --for=condition=Ready "pod/$POD_NAME" -n "$TEST_NS" --timeout=120s >/dev/null
POD_UID="$(kubectl get pod "$POD_NAME" -n "$TEST_NS" -o jsonpath='{.metadata.uid}')"
NODE_NAME="$(kubectl get pod "$POD_NAME" -n "$TEST_NS" -o jsonpath='{.spec.nodeName}')"
echo "✅ Test pod running: uid=$POD_UID node=$NODE_NAME"

CORE_POD="$(require_core_pod)"
PG_POD="$(require_postgres_pod)"
TOKEN="$(require_jwt_token "$CORE_POD")"

echo "⏳ Waiting pod to be persisted in DB..."
if WAITED="$(wait_for_pod_in_db "$POD_UID" "$WAIT_DB_TIMEOUT" "$PG_POD")"; then
  echo "✅ Pod visible in DB after ${WAITED}s"
else
  red "❌ Pod not found in DB after ${WAIT_DB_TIMEOUT}s"
  exit 1
fi

TS="$(date +%s)"
RUNTIME_EVENTS_PAYLOAD=$(cat <<JSON
[
  {
    "event_type": "escape_attempt",
    "mitre_technique": "T1611",
    "signal": "PROC_ROOT_PIVOT",
    "severity": "critical",
    "pod": { "name": "$POD_NAME", "namespace": "$TEST_NS", "uid": "$POD_UID" },
    "node_name": "$NODE_NAME",
    "runtime": "containerd",
    "syscall": "openat",
    "target": "/proc/1/root",
    "timestamp": $TS
  },
  {
    "event_type": "namespace_escape",
    "mitre_technique": "T1611",
    "signal": "NAMESPACE_ESCAPE",
    "severity": "high",
    "pod": { "name": "$POD_NAME", "namespace": "$TEST_NS", "uid": "$POD_UID" },
    "node_name": "$NODE_NAME",
    "runtime": "containerd",
    "syscall": "setns",
    "target": "/proc/1/ns/mnt",
    "timestamp": $TS
  },
  {
    "event_type": "capability_misuse",
    "mitre_technique": "T1611",
    "signal": "CAPABILITY_MISUSE",
    "severity": "high",
    "pod": { "name": "$POD_NAME", "namespace": "$TEST_NS", "uid": "$POD_UID" },
    "node_name": "$NODE_NAME",
    "runtime": "containerd",
    "syscall": "ptrace",
    "target": "kernel",
    "timestamp": $TS
  }
]
JSON
)

echo ""
echo "--- Inject runtime escape signals ---"
POST_RESP="$(core_api_post_json "runtime/events" "$TOKEN" "$CORE_POD" "$RUNTIME_EVENTS_PAYLOAD" || true)"
echo "$POST_RESP"

echo ""
echo "⏳ Waiting runtime signals / risk profile to converge..."
for _ in $(seq 1 24); do
  RS_CNT="$(kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -t -A -c \
    "SELECT COUNT(*) FROM runtime_signals WHERE pod_uid='$POD_UID';" 2>/dev/null | tr -d ' ')"
  RS_CNT="${RS_CNT:-0}"

  PROF_CNT="$(kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -t -A -c \
    "SELECT COUNT(*) FROM pod_risk_profiles WHERE pod_uid='$POD_UID';" 2>/dev/null | tr -d ' ')"
  PROF_CNT="${PROF_CNT:-0}"

  if [ "${RS_CNT:-0}" -ge 1 ] 2>/dev/null && [ "${PROF_CNT:-0}" -ge 1 ] 2>/dev/null; then
    echo "✅ runtime_signals=$RS_CNT, pod_risk_profiles=$PROF_CNT"
    break
  fi
  sleep 5
done

echo ""
echo "--- DB: pod_risk_profiles (risk score thực tế) ---"
kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -c \
  "SELECT pod_uid, namespace, static_risk, runtime_score, (static_risk + runtime_score) AS total_risk, capabilities, updated_at
   FROM pod_risk_profiles
   WHERE pod_uid = '$POD_UID';"

echo ""
echo "--- DB: pod_attack_steps (attack path thực tế) ---"
kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -c \
  "SELECT pod_uid, step_id, category, confidence, created_at
   FROM pod_attack_steps
   WHERE pod_uid = '$POD_UID'
   ORDER BY created_at DESC;"

echo ""
echo "--- DB: insights liên quan toxic combo ---"
kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -c \
  "SELECT id, insight_type, severity, title, detected_at
   FROM insights
   WHERE resource_uid = '$POD_UID'
   ORDER BY detected_at DESC
   LIMIT 15;"

TOXIC_INSIGHT_CNT="$(kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -t -A -c \
  "SELECT COUNT(*) FROM insights WHERE resource_uid = '$POD_UID' AND lower(title) LIKE '%toxic combo%';" 2>/dev/null | tr -d ' ')"
TOXIC_INSIGHT_CNT="${TOXIC_INSIGHT_CNT:-0}"

ATTACK_STEP_CNT="$(kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -t -A -c \
  "SELECT COUNT(*) FROM pod_attack_steps WHERE pod_uid = '$POD_UID';" 2>/dev/null | tr -d ' ')"
ATTACK_STEP_CNT="${ATTACK_STEP_CNT:-0}"

echo ""
echo "--- Quick summary ---"
echo "runtime_signals=$RS_CNT | attack_steps=$ATTACK_STEP_CNT | toxic_combo_insights=$TOXIC_INSIGHT_CNT"
if [ "${ATTACK_STEP_CNT:-0}" -eq 0 ] 2>/dev/null; then
  yellow "⚠️  Attack path chưa materialize (pod_attack_steps=0). Có thể cần đợi attack_path_reconcile_job chạy thêm."
fi
if [ "${TOXIC_INSIGHT_CNT:-0}" -eq 0 ] 2>/dev/null; then
  yellow "⚠️  Chưa thấy insight title chứa 'toxic combo'; hiện tại engine đang ghi nhận theo capability-level insights."
fi

echo ""
echo "--- API: /risk/pods/:uid/runtime ---"
core_api_get "risk/pods/$POD_UID/runtime" "$TOKEN" "$CORE_POD" | python3 -m json.tool || true

echo ""
echo "--- API: /risk/pods/:uid/attack-steps ---"
core_api_get "risk/pods/$POD_UID/attack-steps" "$TOKEN" "$CORE_POD" | python3 -m json.tool || true

echo ""
echo "--- API: /risk/pods/:uid/report (summary + insights) ---"
core_api_get "risk/pods/$POD_UID/report" "$TOKEN" "$CORE_POD" | python3 -m json.tool || true

echo ""
green "✅ Toxic combo testcase completed"
echo "Pod UID: $POD_UID"
echo "Cleanup: kubectl delete pod $POD_NAME -n $TEST_NS"

if [ "$KEEP_TEST_POD" != "true" ]; then
  kubectl delete pod "$POD_NAME" -n "$TEST_NS" --ignore-not-found >/dev/null 2>&1 || true
  echo "🧹 Pod cleaned up (set KEEP_TEST_POD=true to keep it)."
fi
