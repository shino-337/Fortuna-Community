# WebSocket Flows – Core / Agent / Dashboard Sync

Tài liệu này mô tả toàn bộ luồng WebSocket và đảm bảo Core, Agent và Dashboard đồng bộ cách xử lý WS.

## Tổng quan

| Luồng | Core | Agent | Dashboard |
|-------|------|-------|-----------|
| **Risk Center** | Cung cấp `/api/v1/ws/risks`, broadcast khi insights thay đổi (NATS → subscriber → `BroadcastRisksUpdate`) | Không dùng WS; gửi dữ liệu qua gRPC/HTTP → Core | Kết nối `getRisksWsUrl()`, nhận `type === 'insights_updated'` → refetch risks/summary |
| **Pod Detail** | Cung cấp `/api/v1/ws/pod/:uid`, broadcast khi ingest metrics/processes/network/events thành công | Gửi ingest qua HTTP POST → Core; không mở WS | Kết nối `getPodDetailWsUrl(uid)`, nhận `{ type }` → refetch theo type (metrics/processes/network/events) |

**Agent không mở WebSocket.** Agent đẩy dữ liệu lên Core (ingest API); Core ghi DB và broadcast qua WS tới Dashboard.

---

## 1. Risk Center WebSocket

### Core

- **Route:** `GET /api/v1/ws/risks` (trong group `v1`, có AuthMiddleware khi bật auth).
- **Handler:** `api.RisksWS()` (`core/internal/api/risks_ws_hub.go`).
- **Hub:** `defaultRisksHub` (init trong `init()`); mỗi client được add vào hub, nhận message qua channel `send`.
- **Kích hoạt broadcast:**
  - CVE Matcher Worker / Risk Worker sau khi `BatchCreateOrUpdateInsights` thành công → publish NATS `fortuna.insights.updated`.
  - `core/cmd/main.go`: subscriber JetStream `worker.SubjectInsightsUpdated` nhận message → gọi `api.BroadcastRisksUpdate()` → `msg.Ack()`.
- **Nội dung gửi:** JSON `{"type":"insights_updated"}`.

### Dashboard

- **URL:** `api.getRisksWsUrl()` → `wss://host/api/v1/ws/risks?token=...` (hoặc `ws://` nếu HTTP).
- **Trang:** Risk Center (Insights) – `dashboard/pages/Insights.tsx`.
- **Xử lý:** `ws.onmessage` parse JSON, nếu `d?.type === 'insights_updated'` thì gọi `fetchDataRef.current()` (refetch risks + summary). Vẫn giữ polling làm fallback.

### Đồng bộ

- Core gửi đúng một type: `insights_updated`.
- Dashboard chỉ refetch khi nhận đúng type đó → đồng bộ.

---

## 2. Pod Detail WebSocket

### Core

- **Route:** `GET /api/v1/ws/pod/:uid` (trong group `v1`, có AuthMiddleware khi bật auth).
- **Handler:** `api.PodDetailWS()` (`core/internal/api/pod_detail_ws_hub.go`).
- **Hub:** `defaultPodDetailHub`; map theo `uid` → set connections; broadcast chỉ gửi tới connections đăng ký đúng UID.
- **Kích hoạt broadcast:** Ingest handlers sau khi ghi DB thành công:
  - `IngestPodRuntimeMetricsPayload` → `BroadcastPodDetailUpdate(req.PodUID, "metrics")`
  - `IngestPodProcessesPayload` → `BroadcastPodDetailUpdate(req.PodUID, "processes")`
  - `IngestPodNetworkConnectionsPayload` → `BroadcastPodDetailUpdate(req.PodUID, "network")`
  - `IngestPodEventsPayload` → với mỗi `InvolvedUID` → `BroadcastPodDetailUpdate(u, "events")`
- **Nội dung gửi:** JSON `{"type":"<dataType>"}` với `dataType` ∈ `metrics` | `processes` | `network` | `events`.

### Agent

- Agent **không** kết nối WebSocket. Agent gọi HTTP POST tới Core (ingest endpoints); Core sau khi lưu DB sẽ gọi `BroadcastPodDetailUpdate` → chỉ Core ↔ Dashboard qua WS.

### Dashboard

- **URL:** `api.getPodDetailWsUrl(uid)` → `wss://host/api/v1/ws/pod/<uid>?token=...`.
- **Trang:** Pod Detail – `dashboard/pages/PodDetail.tsx`.
- **Xử lý:** `ws.onmessage` parse JSON; nếu có `type` thì chỉ refetch đúng API tương ứng (metrics / processes / network / events); nếu không có type thì refetch cả bốn (tương thích ngược).

### Đồng bộ

- Core gửi `{"type":"metrics"|"processes"|"network"|"events"}`.
- Dashboard refetch theo đúng type → đồng bộ, giảm gọi API thừa.

---

## 3. Auth

- Cả hai WS đều nằm trong `v1` group → khi `cfg.AuthEnabled` thì dùng `middleware.AuthMiddleware(db, cfg.JWTSecret)`.
- Middleware đọc token từ header `Authorization: Bearer <token>` hoặc query `?token=<token>` (phù hợp WebSocket không gửi header tùy ý).
- Dashboard: `getRisksWsUrl()` và `getPodDetailWsUrl(uid)` đều append `?token=...` khi có `getToken()`.

---

## 4. Tóm tắt file liên quan

| Thành phần | File |
|------------|------|
| Core – Risk WS | `core/internal/api/risks_ws_hub.go` (RisksWS, BroadcastRisksUpdate) |
| Core – Pod Detail WS | `core/internal/api/pod_detail_ws_hub.go` (PodDetailWS, BroadcastPodDetailUpdate) |
| Core – Ingest → broadcast | `core/internal/api/pod_detail_services_handlers.go` (gọi BroadcastPodDetailUpdate sau khi lưu) |
| Core – NATS → Risk broadcast | `core/cmd/main.go` (subscribe `worker.SubjectInsightsUpdated` → api.BroadcastRisksUpdate) |
| Core – Routes | `core/internal/api/routes.go` (v1.GET `/ws/pod/:uid`, `/ws/risks`) |
| Core – Auth | `core/internal/middleware/auth.go` (token từ header hoặc query) |
| Dashboard – API URLs | `dashboard/lib/api.ts` (getRisksWsUrl, getPodDetailWsUrl) |
| Dashboard – Risk Center WS | `dashboard/pages/Insights.tsx` (connect, onmessage → insights_updated → refetch) |
| Dashboard – Pod Detail WS | `dashboard/pages/PodDetail.tsx` (connect, onmessage → refetch theo type) |
| Agent | Không file WS; gửi dữ liệu qua HTTP ingest tới Core |

---

Cập nhật lần cuối: sau khi nối NATS cho Risk Center và chuẩn hóa Pod Detail refetch theo `type`.
