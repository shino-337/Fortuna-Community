#!/usr/bin/env bash
# =============================================================================
# Network Activity — kiểm thử hiển thị / nhất quán dữ liệu (namespace fortuna)
# =============================================================================
# So khớp API talkers vs connections (cùng cluster, namespace, since, pageSize).
#
# Giới hạn sản phẩm:
#   - Talkers = TOP pod theo observation_count (pageSize mặc định 200).
#   - Connections = 200 dòng gần nhất (theo bucket_5m / observed_at), không gom theo pod.
#   → Pod ít traffic (dashboard, agent) có thể có dòng trong DB/connections nhưng
#     không vào top-200 talkers → topology có thể không hiện node đó; vẫn thấy cạnh
#     từ pod khác tới cùng đích.
#
# Chạy:
#   CORE_URL=http://127.0.0.1:8080 ./scripts/e2e/test-network-activity-fortuna.sh
#   CLUSTER_ID=<id> NAMESPACE=fortuna STRICT=1 ./scripts/e2e/test-network-activity-fortuna.sh
#
# So DB (tuỳ chọn):
#   USE_DB=1 ./scripts/e2e/test-network-activity-fortuna.sh
#
set -euo pipefail

CORE_URL="${CORE_URL:-http://127.0.0.1:8080}"
API="${CORE_URL}/api/v1"
NAMESPACE="${NAMESPACE:-fortuna}"
PAGE_SIZE="${PAGE_SIZE:-200}"
SINCE_MINUTES="${SINCE_MINUTES:-1440}"
STRICT="${STRICT:-0}"
USE_DB="${USE_DB:-0}"
POSTGRES_NS="${POSTGRES_NS:-fortuna}"
POSTGRES_DEPLOY="${POSTGRES_DEPLOY:-postgres}"
DB_NAME="${DB_NAME:-fortuna}"

echo "=========================================="
echo "Network Activity testcase — ns=$NAMESPACE"
echo "API=$API  sinceMinutes=$SINCE_MINUTES pageSize=$PAGE_SIZE"
echo "=========================================="

LOGIN=$(curl -sS -X POST "$API/auth/login" -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' 2>/dev/null) || true
TOKEN=$(echo "$LOGIN" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('token','') or '')" 2>/dev/null) || true
if [ -z "$TOKEN" ]; then
  echo "FAIL: Không lấy được JWT (Core? admin/admin123?)"
  echo "$LOGIN" | head -c 300
  exit 1
fi
AUTH="Authorization: Bearer $TOKEN"

CLUSTER_ID="${CLUSTER_ID:-}"
if [ -z "$CLUSTER_ID" ]; then
  CL=$(curl -sS -H "$AUTH" "$API/inventory/clusters" 2>/dev/null) || true
  CLUSTER_ID=$(echo "$CL" | python3 -c "import sys,json; d=json.load(sys.stdin); cs=d.get('clusters') or []; print(cs[0].get('id','') if cs else '')" 2>/dev/null) || true
fi
if [ -z "$CLUSTER_ID" ]; then
  echo "FAIL: Cần CLUSTER_ID hoặc cluster từ GET /inventory/clusters"
  exit 1
fi
echo "Cluster ID: $CLUSTER_ID"

ENC_CL=$(python3 -c "import urllib.parse; print(urllib.parse.quote('$CLUSTER_ID'))")
ENC_NS=$(python3 -c "import urllib.parse; print(urllib.parse.quote('$NAMESPACE'))")
QS="cluster=${ENC_CL}&namespace=${ENC_NS}&page=1&pageSize=${PAGE_SIZE}&sinceMinutes=${SINCE_MINUTES}"

fetch_view() {
  local view="$1"
  curl -sS -H "$AUTH" "${API}/runtime/network-activity?${QS}&view=${view}" 2>/dev/null
}

echo ""
echo "[1] GET view=talkers"
TALKERS_JSON=$(fetch_view talkers)
echo "[2] GET view=connections"
CONN_JSON=$(fetch_view connections)

export TALKERS_JSON CONN_JSON NAMESPACE STRICT
EXIT_CODE=0
python3 << 'PY' || EXIT_CODE=$?
import json, os, sys

strict = os.environ.get("STRICT", "0") == "1"
ns = os.environ.get("NAMESPACE", "fortuna")

def load(env_key):
    raw = os.environ.get(env_key, "") or "{}"
    try:
        return json.loads(raw)
    except json.JSONDecodeError as e:
        print(f"FAIL: JSON {env_key}: {e}", file=sys.stderr)
        sys.exit(2)

t = load("TALKERS_JSON")
c = load("CONN_JSON")

for label, d in ("talkers", t), ("connections", c):
    if not isinstance(d, dict):
        print(f"FAIL: {label} không phải object")
        sys.exit(2)
    if d.get("error"):
        print(f"FAIL: {label} error: {d.get('error')}")
        sys.exit(2)
    if "items" not in d:
        print(f"FAIL: {label} thiếu items")
        sys.exit(2)

items_t = t.get("items") or []
items_c = c.get("items") or []
print(f"Talkers: total={t.get('total')} len(items)={len(items_t)}")
print(f"Connections: total={c.get('total')} len(items)={len(items_c)}")

talker_uids = {str(x.get("podUid") or "") for x in items_t if x.get("podUid")}
talker_names = sorted({str(x.get("podName") or "").strip() for x in items_t if x.get("podName")})

conn_uids = set()
for x in items_c:
    u = str(x.get("podUid") or "")
    if u:
        conn_uids.add(u)

print(f"Unique podUid (talkers p1): {len(talker_uids)}")
print(f"Unique podUid (connections p1): {len(conn_uids)}")

missing = sorted(conn_uids - talker_uids)
if missing:
    print("")
    print("WARN: pod_uid có trong 200 dòng connections nhưng KHÔNG trong 200 talkers (top observation_count):")
    for u in missing[:30]:
        print(f"  - {u}")
    if len(missing) > 30:
        print(f"  ... +{len(missing)-30} uid")
    print("")
    print("→ Đây là nguyên nhân hay gặp khi dashboard/agent không thấy trên graph")
    print("  dù vẫn có cạnh từ core/nats: chúng nằm trong top talkers; pod ít flow không.")
    if strict:
        sys.exit(3)
else:
    print("OK: mọi pod_uid trong connections p1 đều có trong talkers p1.")

uid_to_name = {}
for x in items_t:
    u = str(x.get("podUid") or "")
    n = (x.get("podName") or "").strip()
    if u and n:
        uid_to_name[u] = n
conn_names = {uid_to_name.get(u, u[:12] + "…") for u in conn_uids if u}
talker_only = talker_uids - conn_uids
if talker_only:
    print("")
    print("WARN (topology Dashboard): Các pod CÓ trong talkers nhưng KHÔNG có trong 200 dòng connections đầu tiên:")
    for u in sorted(talker_only)[:20]:
        print(f"  - {uid_to_name.get(u, u)}  ({u[:8]}…)")
    print("  → UI graph dựng từ view=connections: các pod này có thể KHÔNG có node dù API tổng hợp có tên.")
    if strict:
        sys.exit(4)

print("")
print("PodName (talkers p1), tối đa 50:")
for n in talker_names[:50]:
    print(f"  · {n}")
if len(talker_names) > 50:
    print(f"  ... tổng {len(talker_names)}")
PY

if [ "${EXIT_CODE:-0}" -eq 3 ] || [ "${EXIT_CODE:-0}" -eq 4 ]; then
  EXIT_CODE=1
fi

if [ "$USE_DB" = "1" ] && command -v kubectl >/dev/null 2>&1; then
  echo ""
  echo "[3] DB pod_network_connections (24h, namespace=$NAMESPACE)"
  SQL="SELECT pod_uid::text, COUNT(*)::bigint AS n FROM pod_network_connections WHERE cluster_id = '${CLUSTER_ID}' AND namespace = '${NAMESPACE}' AND bucket_5m >= (NOW() AT TIME ZONE 'UTC') - INTERVAL '24 hours' GROUP BY pod_uid ORDER BY n DESC LIMIT 250;"
  TMPDB=$(mktemp)
  kubectl exec -n "$POSTGRES_NS" "deploy/$POSTGRES_DEPLOY" -- \
    psql -U postgres -d "$DB_NAME" -t -A -F'|' -c "$SQL" 2>/dev/null > "$TMPDB" || true
  if [ -s "$TMPDB" ]; then
    export CLUSTER_ID
    python3 << PY
import json, os
cluster = os.environ["CLUSTER_ID"]
db_uids = {}
with open("$TMPDB") as f:
    for ln in f:
        ln = ln.strip()
        if not ln or "|" not in ln:
            continue
        u, n = ln.split("|", 1)
        db_uids[u.strip()] = int(n.strip())
t = json.loads(os.environ["TALKERS_JSON"])
api_uids = {str(x.get("podUid")) for x in (t.get("items") or []) if x.get("podUid")}
print(f"DB pods (top slice): {len(db_uids)}")
for uid, n in sorted(db_uids.items(), key=lambda x: -x[1])[:20]:
    tag = "trong talkers p1" if uid in api_uids else "KHÔNG trong talkers p1"
    print(f"  {uid[:14]}… rows={n}  ({tag})")
not_in_top = [u for u in db_uids if u not in api_uids]
if not_in_top:
    print("")
    print(f"⚠ {len(not_in_top)} pod có dữ liệu DB (24h) nhưng không trong trang 1 talkers.")
PY
  else
    echo "WARN: không đọc được DB"
  fi
  rm -f "$TMPDB"
fi


# So Pod đang chạy trong namespace với inventory API (không cần port-forward)
USE_INVENTORY="${USE_INVENTORY:-1}"

if [ "$USE_INVENTORY" = "1" ] && command -v kubectl >/dev/null 2>&1; then
  echo ""
  echo "[4] Inventory: pods trong namespace $NAMESPACE (kubectl)"
  K8S_JSON=$(kubectl get pods -n "$NAMESPACE" -o json 2>/dev/null) || K8S_JSON="{}"
  export K8S_JSON TALKERS_JSON NAMESPACE
  python3 << "INVPY"
import json, os
ns = os.environ.get("NAMESPACE", "fortuna")
try:
    k = json.loads(os.environ.get("K8S_JSON") or "{}")
except json.JSONDecodeError:
    k = {}
items = (k.get("items") or [])
t = json.loads(os.environ.get("TALKERS_JSON") or "{}")
api_names = set()
for x in t.get("items") or []:
    n = (x.get("podName") or "").strip()
    if n:
        api_names.add(n)
running = []
for it in items:
    st = (it.get("status") or {}).get("phase") or ""
    name = (it.get("metadata") or {}).get("name") or ""
    if not name:
        continue
    if st == "Running":
        running.append(name)
missing = [n for n in sorted(running) if n not in api_names]
print(f"Running pods (kubectl): {len(running)}")
print(f"Talkers có podName khớp: {len(api_names)}")
if missing:
    print("")
    print("INFO: Pod Running nhưng không có podName trong talkers API (thường: chưa có dòng pod_network_connections / agent chưa báo cáo):")
    for n in missing[:30]:
        print(f"  · {n}")
    if len(missing) > 30:
        print(f"  ... +{len(missing)-30}")
else:
    print("OK: mọi pod Running đều có tên xuất hiện trong talkers (theo podName).")
INVPY
fi

echo ""
echo "=========================================="
if [ "$EXIT_CODE" -eq 0 ]; then
  echo "Testcase xong (0). Đọc WARN để hiểu hạn chế UI."
else
  echo "Testcase thất bại (STRICT=1 hoặc lỗi API)."
fi
echo "=========================================="
exit "$EXIT_CODE"
