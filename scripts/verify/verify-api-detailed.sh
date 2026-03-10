#!/usr/bin/env bash
# =============================================================================
# Chi tiết API: đăng nhập admin/admin123, gọi từng endpoint và ghi kết quả (status + body).
# Chạy từ host; cần kubectl và Core pod trong namespace fortuna.
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
NAMESPACE="${NAMESPACE:-fortuna}"
CORE_POD=$(kubectl -n "$NAMESPACE" get pods -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
if [ -z "$CORE_POD" ]; then
  echo "Core pod not found in $NAMESPACE"
  exit 1
fi

BASE="http://localhost:8080"
REPORT_DIR="$PROJECT_ROOT/docs/test-results"
mkdir -p "$REPORT_DIR"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
REPORT="${REPORT_DIR}/verify-api-detailed-${TIMESTAMP}.md"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "# Chi tiết kiểm tra API (admin/admin123)" > "$REPORT"
echo "**Thời gian:** $(date -Iseconds)" >> "$REPORT"
echo "**Core pod:** $CORE_POD" >> "$REPORT"
echo "" >> "$REPORT"

# Login
echo "Đang đăng nhập admin/admin123..."
TOKEN=$(kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -X POST "$BASE/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | python3 -c "import sys,json; print(json.load(sys.stdin).get('token','') or '')")

if [ -z "$TOKEN" ]; then
  echo "Lỗi: Không lấy được token. Kiểm tra Core và user admin."
  echo "Lỗi: Login thất bại" >> "$REPORT"
  exit 1
fi
echo "Login OK, token length: ${#TOKEN}"
echo "" >> "$REPORT"

# Helper: gọi GET và trả về status + summary body
# Usage: api_get "path" "description"
api_get() {
  local path="$1"
  local desc="$2"
  local code body
  body=$(kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -w "\n%{http_code}" -H "Authorization: Bearer $TOKEN" "$BASE$path" 2>/dev/null || echo "")
  code=$(echo "$body" | tail -1)
  body=$(echo "$body" | sed '$d')
  local summary=""
  if [ "$code" = "200" ]; then
    summary=$(echo "$body" | python3 -c "
import sys, json
try:
    d = json.load(sys.stdin)
    if isinstance(d, dict):
        if 'pods' in d: summary = 'pods=%s total=%s' % (len(d.get('pods',[])), d.get('total',''))
        elif 'insights' in d: summary = 'insights=%s' % len(d.get('insights',[]))
        elif 'sboms' in d: summary = 'sboms=%s' % len(d.get('sboms',[]))
        elif 'metrics' in d: summary = 'metrics=%s' % len(d.get('metrics',[]))
        elif 'processes' in d: summary = 'processes=%s' % len(d.get('processes',[]))
        elif 'events' in d: summary = 'events=%s' % len(d.get('events',[]))
        elif 'user' in d: summary = 'user=%s' % (d.get('user') or {}).get('username','')
        elif 'totalClusters' in d: summary = 'clusters=%s pods=%s risks=%s' % (d.get('totalClusters'), d.get('runningPods'), d.get('totalRisks'))
        elif 'clusters' in d: summary = 'clusters=%s' % len(d.get('clusters',[]))
        elif 'error' in d: summary = 'error=%s' % (d.get('error','')[:80])
        else: summary = 'keys=' + ','.join(list(d.keys())[:8])
    else:
        summary = str(d)[:100]
except Exception as e:
    summary = 'parse_err'
print(summary)
" 2>/dev/null || echo "?")
  else
    summary=$(echo "$body" | head -c 120)
  fi
  echo "| $path | $code | $summary |" >> "$REPORT"
  if [ "$code" = "200" ]; then
    echo -e "${GREEN}[OK]${NC} $path -> $code ($summary)"
  else
    echo -e "${RED}[FAIL]${NC} $path -> $code ($summary)"
  fi
  # Don't echo code/summary to stdout (only the [OK]/[FAIL] line above)
}

echo "## Kết quả từng endpoint" >> "$REPORT"
echo "| Endpoint | HTTP | Mô tả |" >> "$REPORT"
echo "|----------|------|-------|" >> "$REPORT"

# Lấy một pod id và uid để test pod detail
PODS_JSON=$(kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -H "Authorization: Bearer $TOKEN" "$BASE/api/v1/pods?limit=1" 2>/dev/null || echo "{}")
POD_ID=$(echo "$PODS_JSON" | python3 -c "import sys,json; d=json.load(sys.stdin); p=d.get('pods',[]); print(str(p[0].get('id',''))) if p else print('')" 2>/dev/null || echo "")
POD_UID=$(echo "$PODS_JSON" | python3 -c "import sys,json; d=json.load(sys.stdin); p=d.get('pods',[]); print(p[0].get('uid','') or '') if p else print('')" 2>/dev/null || echo "")

api_get "/api/v1/me" "Current user"
api_get "/api/v1/clusters" "Clusters"
api_get "/api/v1/clusters/stats" "Cluster stats"
api_get "/api/v1/pods?limit=5" "Pods list"
api_get "/api/v1/dashboard/stats" "Dashboard stats"
api_get "/api/v1/insights?limit=5" "Insights"
api_get "/api/v1/sbom?limit=5" "SBOM list"
api_get "/api/v1/health/dashboard-data-integrity" "Health"
api_get "/api/v1/agents/status" "Agents status"
api_get "/api/v1/risks?limit=5" "Risks"
api_get "/api/v1/deployments?limit=5" "Deployments"
api_get "/api/v1/resources" "Resources"
api_get "/api/v1/runtime-signals?limit=5" "Runtime signals"
api_get "/api/v1/pod-capabilities/summary" "Pod capabilities summary"

if [ -n "$POD_ID" ]; then
  api_get "/api/v1/pods/$POD_ID" "Pod detail by id"
  api_get "/api/v1/pods/$POD_ID/capabilities" "Pod capabilities"
  api_get "/api/v1/pods/$POD_ID/runtime-metrics" "Runtime metrics"
  api_get "/api/v1/pods/$POD_ID/processes" "Processes"
  api_get "/api/v1/pods/$POD_ID/network-connections" "Network connections"
  api_get "/api/v1/pods/$POD_ID/events" "K8s events"
fi
if [ -n "$POD_UID" ]; then
  api_get "/api/v1/pods/by-uid/$POD_UID" "Pod by UID"
  api_get "/api/v1/pods/by-uid/$POD_UID/runtime-metrics" "Runtime metrics by UID"
  api_get "/api/v1/pods/by-uid/$POD_UID/processes" "Processes by UID"
  api_get "/api/v1/pods/by-uid/$POD_UID/events" "Events by UID"
fi

echo ""
echo "Báo cáo: $REPORT"
echo "" >> "$REPORT"
echo "---" >> "$REPORT"
echo "" >> "$REPORT"
echo "## Lưu ý" >> "$REPORT"
echo "" >> "$REPORT"
echo "- **Dashboard:** Cần đăng nhập (admin/admin123) thì mọi API mới hoạt động. Token lưu trong store và gửi qua header \`Authorization: Bearer <token>\`. Nếu chưa đăng nhập, API trả **401**." >> "$REPORT"
echo "- **Pod detail phụ (runtime-metrics, processes, events):** Trả 200 nhưng mảng rỗng cho đến khi Agent gửi dữ liệu lên Core (các endpoint POST tương ứng)." >> "$REPORT"
echo "- **Kiểm tra từ máy ngoài:** \`kubectl port-forward -n fortuna svc/fortuna-core 8080:8080\` rồi \`curl -X POST http://localhost:8080/api/v1/auth/login -H 'Content-Type: application/json' -d '{\"username\":\"admin\",\"password\":\"admin123\"}'\` để lấy token, sau đó gọi \`curl -H \"Authorization: Bearer <token>\" http://localhost:8080/api/v1/pods\`." >> "$REPORT"
