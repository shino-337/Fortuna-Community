# Pod Detail – Implementation Reference

**Phiên bản:** 1.1  
**Ngày:** 2026-03-09  
**Phạm vi:** API Core, Dashboard, Agent reporter; đồng bộ luồng; không seed/mock.

---

## 0. Luồng dữ liệu và chính sách không seed/mock

- **Nguồn dữ liệu duy nhất:** Pod Detail (runtime-metrics, processes, network-connections, events) chỉ đến từ **Agent** gửi lên Core qua POST. Core **không** có migration/script seed dữ liệu vào các bảng `pod_runtime_metrics`, `pod_processes`, `pod_network_connections`, `k8s_events`.
- **Agent:** Chỉ báo cáo **pod thực** trên node (list qua K8s API với `spec.nodeName=`). Metrics lấy từ `pod.Status.ContainerStatuses` (state, restart count). Process list lấy **thực** bằng exec vào từng container (`ps -eo pid,ppid,user,%cpu,%mem,comm`); không dùng dữ liệu giả. Nếu container không có `ps` (ví dụ distroless), container đó bỏ qua (log và tiếp tục).
- **Core:** Chỉ ghi DB khi nhận POST từ Agent; validate `podUid` (reject rỗng / `"0"`). GET trả đúng dữ liệu trong DB.
- **Dashboard:** Chỉ hiển thị dữ liệu từ API (GET). Không fallback mock khi `items` rỗng.
- **Đồng bộ:** Pod phải đã được sync vào bảng `pods` (từ sync payload của Agent) thì mới có `id`/`uid` để Dashboard gọi GET. Agent Pod Detail reporter chạy độc lập với sync; `podUid` gửi lên Core phải trùng với `pods.uid` (cùng cluster) để Dashboard tìm được khi mở Pod Detail theo pod đó.

---

## 1. API Pod Detail (Core)

### 1.1 GET – Dashboard gọi theo `id` hoặc `uid`

- **Base path:** `/api/v1`
- **Cách gọi:** Dùng **id** (số, primary key pod) hoặc **uid** (chuỗi). Dashboard dùng `pod.id ?? pod.uid` làm `idOrUid`.

| Endpoint | Mô tả |
|----------|--------|
| `GET /pods/:id/runtime-metrics` | Metrics theo id (số). |
| `GET /pods/:podUid/runtime-metrics` | Metrics theo uid. |
| `GET /pods/:id/processes` | Process list theo id. |
| `GET /pods/:podUid/processes` | Process list theo uid. |
| `GET /pods/:id/network-connections` | Network connections theo id. |
| `GET /pods/:podUid/network-connections` | Network connections theo uid. |
| `GET /pods/:id/events` | K8s events (involved_uid = pod) theo id. |
| `GET /pods/:podUid/events` | K8s events theo uid. |

### 1.2 Response shape (GET)

- **runtime-metrics:** `{ "podUid": "<uid>", "items": [ { "id", "podUid", "clusterId", "namespace", "containerName", "cpuUsageMillicore", "memoryUsageBytes", "memoryLimitBytes", "restartCount", "state", "lastObservedAt", ... } ] }`
- **processes:** `{ "podUid": "<uid>", "items": [ { "id", "podUid", "containerName", "pid", "ppid", "userName", "cpuPercent", "memoryPercent", "command", "binaryPath", "observedAt", ... } ] }`
- **network-connections:** `{ "podUid": "<uid>", "items": [ { "id", "podUid", "containerName", "sourceIp", "sourcePort", "destIp", "destPort", "protocol", "state", ... } ] }`
- **events:** `{ "podUid": "<uid>", "items": [ { "id", "clusterId", "eventUid", "namespace", "involvedKind", "involvedUid", "involvedName", "reason", "message", "eventType", "count", "lastTimestamp", ... } ] }`

Dashboard luôn dùng trường `items` (mảng). Nếu không có bản ghi, Core trả `items: []`.

### 1.3 POST Ingest (Agent → Core)

- **Base path:** `/api/v1/agent` (không bắt buộc auth).
- **Contract:**
  - **pod-runtime-metrics:** Body `{ "podUid", "clusterId", "namespace", "metrics": [ { "containerName", "cpuUsageMillicore", "memoryUsageBytes", "memoryLimitBytes", "restartCount", "state" } ] }`. `podUid` bắt buộc, không được rỗng hoặc `"0"`.
  - **pod-processes:** Body `{ "podUid", "clusterId", "namespace", "processes": [ { "containerName", "pid", "ppid", "userName", "cpuPercent", "memoryPercent", "command", "binaryPath" } ] }`. `podUid` bắt buộc, không được rỗng hoặc `"0"`.
  - **pod-network-connections:** Body `{ "podUid", "clusterId", "namespace", "connections": [ ... ] }`. Reject nếu `podUid` rỗng hoặc `"0"`.
  - **pod-events:** Body `{ "clusterId", "events": [ ... ] }` (K8s events; `involvedUid` có thể là pod UID).

Nếu `podUid` rỗng hoặc `"0"`, Core trả 400 với message `"podUid required and must not be 0 or empty"`.

---

## 2. Dashboard

- **Trang:** `dashboard/pages/PodDetail.tsx`.
- **API client:** `dashboard/lib/api.ts`: `getPodRuntimeMetrics(idOrUid)`, `getPodProcesses(idOrUid)`, `getPodNetworkConnections(idOrUid)`, `getPodEvents(idOrUid)`.
- **idOrUid:** Luôn dùng `pod.id ?? pod.uid` khi gọi 4 API trên (mở bằng UID vẫn có đủ dữ liệu).
- **Tab:** Overview, SBOM, Related Risks, **Processes**, **Network**, **Events**. Tab Process/Network/Events hiển thị bảng; nếu `items` rỗng thì hiển thị empty state.

---

## 3. Agent

- **Reporter:** `agent/internal/poddetail/reporter.go`; **process collector:** `agent/internal/poddetail/process_collector.go`.
- **Chu kỳ:** Mặc định 2 phút; env `POD_DETAIL_REPORT_INTERVAL` (e.g. `1m`, `5m`).
- **Luồng:** List pods trên node (K8s API, field selector `spec.nodeName=`) → với mỗi pod (bỏ qua `uid == ""` hoặc `"0"`): (1) gửi runtime metrics từ `pod.Status.ContainerStatuses` (state, restart count; CPU/memory 0 nếu chưa lấy từ metrics-server); (2) thu thập process list **thực** bằng exec vào từng container (`ps -eo pid,ppid,user,%cpu,%mem,comm`), parse output và gửi lên Core. Nếu exec thất bại (ví dụ container không có `ps`), container đó bỏ qua, không gửi mock.
- **Core URL:** Cùng `CORE_HTTP_ENDPOINT` với syncer; POST tới `/api/v1/agent/pod-runtime-metrics`, `/api/v1/agent/pod-processes`.
- **rest.Config:** Reporter nhận `*rest.Config` (từ `k8sClient.Config`) để thực hiện exec; nếu nil thì process/network payload gửi rỗng.
- **Network:** Thu thập thực bằng exec trong từng container (`ss -tunap` hoặc `netstat -tunap`), parse và POST `/api/v1/agent/pod-network-connections`. File: `agent/internal/poddetail/network_collector.go`.
- **K8s Events:** `EventsCollector` dùng SharedInformer cho `core/v1.Event`, batch mỗi 30s và POST `/api/v1/agent/pod-events`. File: `agent/internal/poddetail/events_collector.go`. Khởi chạy trong `cmd/main.go`.
- **Retry (Phase 2.2):** Mọi POST từ reporter dùng `postWithRetry` (tối đa 3 lần, exponential backoff; retry khi 429 hoặc 5xx).
- **Core ingest retry (Phase 3.3):** Mọi DB write trong ingest (metrics, processes, network, events) dùng `dbIngestWithRetry` (3 lần, backoff 1s/2s/4s). File: `pod_detail_ingest_retry.go`.
- **Encryption at-rest (Phase 4.2):** Trường nhạy cảm `command` và `binary_path` trong `pod_processes` được encrypt AES-256-GCM trước khi lưu khi cấu hình key. Key lấy từ **config** `PodDetailEncryptionKey` (trong `core/internal/config/config.go`), load từ env `POD_DETAIL_ENCRYPTION_KEY`; user có thể đổi giá trị trước build/deploy (env hoặc K8s Secret). GET giải mã trước khi trả về. File: `pod_detail_encrypt.go`; khởi tạo từ `api.InitPodDetailEncryptionKey(cfg.PodDetailEncryptionKey)` trong `routes.go`.
- **WebSocket (Phase 5.1):** Core endpoint `GET /api/v1/ws/pod/:uid` — client subscribe theo UID; khi ingest (metrics, processes, network, events) thành công, Core gửi message `{"type":"metrics"|"processes"|"network"|"events"}` tới tất cả client đăng ký UID đó. Dashboard PodDetail mở WS với `api.getPodDetailWsUrl(uid)`, khi nhận message refetch 4 API (runtime-metrics, processes, network-connections, events). Auth: Bearer header hoặc query `?token=`. File: `pod_detail_ws_hub.go`, routes.go; ingest handlers gọi `BroadcastPodDetailUpdate(podUid, type)`.

---

## 4. API 200 nhưng không có dữ liệu (empty `items`)

### 4.1 Checklist

1. **Pod có trong bảng `pods` chưa?** Dashboard lấy pod bằng `getPod(id)` hoặc `getPodByUid(uid)`. Nếu pod chưa sync (agent chưa gửi hoặc Core chưa lưu) thì không có bản ghi Pod Detail.
2. **Dashboard dùng đúng id/uid chưa?** Phải gọi 4 API với `pod.id ?? pod.uid`. Nếu chỉ dùng `pod.id` mà mở bằng UID (id = 0) thì request sai.
3. **Agent có gửi Pod Detail không?** Kiểm tra Agent log `[PodDetail]`; đảm bảo `CORE_HTTP_ENDPOINT` đặt đúng; reporter đang chạy.
4. **Core có reject podUid rỗng/0 không?** Agent bỏ qua pod có `uid == ""` hoặc `"0"`; Core reject ingest với 400. Không có bản ghi nào với `pod_uid = ''` hoặc `'0'` nếu logic đúng.

### 4.2 Script kiểm tra

- **verify-pod-detail-empty-response.sh:** Đếm bảng Pod Detail, list pods, số metrics/processes/conns/events theo `pod_uid`, kiểm tra `pod_uid` trong detail có trong `pods` không, mẫu vài pod; kiểm tra bản ghi lỗi `pod_uid = ''` hoặc `'0'` và gợi ý DELETE.

### 4.3 SQL nhanh (trong Postgres pod)

```sql
SELECT COUNT(*) FROM pod_runtime_metrics;
SELECT COUNT(*) FROM pod_processes;
SELECT pod_uid, COUNT(*) FROM pod_runtime_metrics GROUP BY pod_uid LIMIT 5;
-- Bad rows (nên xóa):
DELETE FROM pod_runtime_metrics WHERE pod_uid = '' OR pod_uid = '0';
DELETE FROM pod_processes WHERE pod_uid = '' OR pod_uid = '0';
```

---

## 5. File tham chiếu

| Thành phần | File |
|------------|------|
| Core handlers | `core/internal/api/pod_detail_services_handlers.go` |
| Core routes | `core/internal/api/routes.go` (GET/POST Pod Detail) |
| Models | `core/pkg/models/pod_runtime_metrics.go`, `pod_process.go`, `pod_network_connection.go`, `k8s_event.go` |
| Agent reporter | `agent/internal/poddetail/reporter.go` |
| Dashboard page | `dashboard/pages/PodDetail.tsx` |
| Dashboard API | `dashboard/lib/api.ts` |
| Kế hoạch thực hiện | `docs/03-components/podDetail/Pod_Detail_Implementation_Plan.md` |
