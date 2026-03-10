# Kế hoạch thực hiện Pod Detail – Theo Technical Spec & Design

**Nguồn:** Pod_Details_Processing_Flow_Technical_Specifications.md (11 Findings), POD_DETAIL_SERVICES_DESIGN.md, Pod_Detail_Session_Plan_vs_Actual.md.  
**Mục tiêu:** Triển khai lại toàn bộ đề xuất theo thứ tự ưu tiên (nền tảng → High → Medium → Low).

---

## Phase 1 – Nền tảng (đã có + bổ sung thiếu)

| # | Hạng mục | Trạng thái | Công việc |
|---|----------|------------|-----------|
| 1.1 | DB & models | ✅ Có | Migrations 068, 071–073; models pod_runtime_metrics, pod_processes, pod_network_connections, k8s_event. |
| 1.2 | Core API GET Pod Detail | ❌ Thiếu | Tạo `pod_detail_services_handlers.go`: GET runtime-metrics, processes, network-connections, events (theo :id và by-uid/:uid). |
| 1.3 | Core API POST Ingest | ❌ Thiếu | POST /api/v1/agent/pod-runtime-metrics, pod-processes, pod-network-connections, pod-events; validate podUid (reject "" và "0"). |
| 1.4 | Đăng ký route | ❌ Thiếu | Thêm route GET/POST vào `routes.go` (route chi tiết trước /pods/:id). |
| 1.5 | Dashboard API + UI | ❌ Thiếu | api.ts: getPodRuntimeMetrics, getPodProcesses, getPodNetworkConnections, getPodEvents. PodDetail.tsx: tab Process, Network, Events; refetch dùng pod.id ?? pod.uid. |
| 1.6 | Agent reporter | ❌ Thiếu | `agent/internal/poddetail/reporter.go`: thu thập metrics + processes (và sau đó network/events); POST lên Core; bỏ qua khi uid rỗng/"0". |

**Deliverable Phase 1:** Dashboard mở Pod Detail có đủ 4 tab dữ liệu; Agent gửi được metrics/processes; Core lưu và trả về qua GET.

**Trạng thái (2026-03-09):** Phase 1 đã triển khai. Bổ sung: **dữ liệu thực, không seed/mock** — Agent thu thập process **thực** bằng exec vào container (`poddetail/process_collector.go`); metrics từ `pod.Status`; Core chỉ ghi khi nhận POST; Dashboard chỉ hiển thị từ API. Luồng đồng bộ và chính sách không seed ghi trong `Pod_Detail_Implementation_Reference.md` §0.

---

## Phase 2 – High (Finding #1, #2, #6)

| # | Finding | Công việc | Effort | Trạng thái |
|---|---------|-----------|--------|------------|
| 2.1 | #1 Network & K8s Events | Agent: collector network (ss/netstat), K8s Events (SharedInformer); POST lên Core. | Medium | ✅ Đã làm: `network_collector.go` (exec ss/netstat), `events_collector.go` (informer + batch POST), reporter gọi sendNetworkConnections. |
| 2.2 | #2 Event channel đầy – backpressure/retry | Agent: retry với backoff khi POST fail. | Low | ✅ Đã làm: `postWithRetry` (3 lần, exponential backoff, 429/5xx retry). |
| 2.3 | #6 SBOM drop khi channel đầy | Agent: ưu tiên SBOM trong queue hoặc kênh riêng; retry khi queue đầy. | Low | Chưa (SBOM path riêng, không thuộc Pod Detail reporter). |

---

## Backlog – Phụ thuộc Redis / Prometheus (chưa có hạ tầng)

Các hạng mục sau **không triển khai** cho đến khi có Redis và/hoặc Prometheus trong hạ tầng:

| # | Finding | Công việc | Lý do backlog |
|---|---------|-----------|----------------|
| 3.2 | #4 Dedup | Redis dedup (dedup:{message_id}, TTL) | Cần Redis. |
| 3.4 | #7 Cache API | Redis cache GET (pod:{uid}:metrics, TTL 120s) | Cần Redis. |
| 3.5 | #8 Prometheus | Histogram latency per type (metrics, processes, network, events) | Cần Prometheus. |
| 3.1 | #3 Async NATS | Recv → publish NATS → worker batch insert | Tùy chọn khi có NATS; có thể làm sau. |
| 3.3 DLQ | #5 DLQ | Publish failed ingest lên DLQ NATS | Tùy chọn khi có NATS. |

---

## Phase 3 – Làm ngay (không phụ thuộc Redis/Prometheus)

| # | Finding | Công việc | Trạng thái |
|---|---------|-----------|------------|
| 3.3 | #5 Retry ingest | Core: retry 3x backoff khi DB write fail (trước khi trả 500). | Làm ngay. |

---

## Phase 4 – Medium (Finding #9, #10)

| # | Finding | Công việc | Trạng thái |
|---|---------|-----------|------------|
| 4.1 | #9 Process procfs / host inspection | Thu thập process/network từ node (host): /proc, cgroup, không exec. **Kế hoạch triển khai chi tiết:** `Host_Inspection_Implementation_Plan.md` (Phase 0→4, file mới, checklist). Kiến trúc: `docs/02-architecture/Runtime_Monitoring_Architecture.md`. | **Kế hoạch đã lên;** triển khai theo Host_Inspection_Implementation_Plan. |
| 4.2 | #10 Encryption at-rest | Core: encrypt command/binary_path; key từ config. | ✅ Đã làm: `pod_detail_encrypt.go`; key đọc từ **config** `PodDetailEncryptionKey` (config load từ env `POD_DETAIL_ENCRYPTION_KEY`). User có thể sửa giá trị trước build/deploy (env hoặc Secret `fortuna-secrets` key `pod-detail-encryption-key`). |

---

## Phase 5 – Low (Finding #11)

| # | Finding | Công việc | Trạng thái |
|---|---------|-----------|------------|
| 5.1 | #11 WebSocket | Core: /api/v1/ws/pod/:uid; push khi ingest; Dashboard subscribe + refetch. | ✅ Đã làm: `pod_detail_ws_hub.go`, Broadcast sau ingest; Dashboard mở WS và refetch metrics/processes/network/events khi nhận message. Auth: token query (?token=) hoặc Bearer header. |

---

## Thứ tự thực hiện (đã làm / làm tiếp)

- **Đã xong:** Phase 1, Phase 2, Phase 3.3 (retry ingest), Phase 4.2 (encryption at-rest), Phase 5.1 (WebSocket).  
- **Backlog:** Redis (dedup, cache), Prometheus (histogram), NATS async/DLQ.  
- **Host inspection (#9):** Kế hoạch triệt để trong **`Host_Inspection_Implementation_Plan.md`** (Phase 0: DaemonSet hostPID + /proc → Phase 1: process từ host → Phase 2: network từ host → Phase 3: auto/fallback → Phase 4: UI Runtime Source). **UI:** Runtime Source indicator (Container Exec | Host Inspection) khi có host pipeline.
- **Ngắn hạn (đã làm):** Exec “tool not found” (distroless) chỉ log DEBUG khi `LOG_LEVEL=debug`; không log error.

---

## File tạo/sửa (Phase 1)

| File | Hành động |
|------|------------|
| `core/internal/api/pod_detail_services_handlers.go` | Tạo mới: GET/POST handlers. |
| `core/internal/api/routes.go` | Bổ sung: route GET /pods/:id/runtime-metrics, processes, network-connections, events; by-uid tương ứng; POST /api/v1/agent/pod-*. |
| `dashboard/lib/api.ts` | Thêm: getPodRuntimeMetrics, getPodProcesses, getPodNetworkConnections, getPodEvents. |
| `dashboard/types.ts` | Thêm: type cho metrics, process, network connection, k8s event (nếu chưa có). |
| `dashboard/pages/PodDetail.tsx` | Thêm: tab Process, Network, Events; gọi 4 API với pod.id ?? pod.uid. |
| `agent/internal/poddetail/reporter.go` | Tạo mới: collect metrics/processes; POST lên Core; skip uid ""/"0". |
| `agent/cmd/main.go` | Khởi chạy reporter (nếu cần). |
| `docs/03-components/podDetail/Pod_Detail_Implementation_Reference.md` | Tạo: mô tả API, response shape, xử lý empty. |
