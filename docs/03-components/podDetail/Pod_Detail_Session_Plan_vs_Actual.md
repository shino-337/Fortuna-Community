# Pod Detail – Đối chiếu plan với thực tế và lý do thiếu file

**Ngày:** 2026-03-09  
**Mục đích:** Kiểm tra nội dung công việc đã lên plan, đã thực hiện theo đề xuất cải tiến Pod Detail trong session; giải thích vì sao một số tài liệu, mã nguồn, script có thể bị “mất” hoặc không thấy trong workspace.

---

## 1. Nguồn plan / đề xuất cải tiến

- **Pod_Details_Processing_Flow_Technical_Specifications.md** – 11 Findings (High/Medium/Low), mỗi Finding có Recommendation (Execution Flow, Information Processing, Implementation Notes).
- **POD_DETAIL_SERVICES_DESIGN.md** – Thiết kế 6 service (Pod, Runtime, Process, Security, Network, Event): DB, Agent, Core API, flow.
- **podDetail_QA.md** – Checklist kiểm tra luồng Pod Details (chưa phải plan thực hiện từng bước).

---

## 2. Đối chiếu 11 Findings (Technical Spec) với hiện trạng

| # | Finding | Risk | Trạng thái | Ghi chú |
|---|---------|------|------------|--------|
| 1 | Network connections & K8s events không thu thập từ Agent | High | **Chưa làm** | Core có bảng `pod_network_connections`, `k8s_events` (migration 071); Agent chưa có collector/stream kind tương ứng. |
| 2 | Drop data khi event channel đầy (256), không backpressure/retry | High | **Chưa làm** | Cần refactor channel + queue + retry ở Agent. |
| 3 | Stream handler Core xử lý đồng bộ – bottleneck | Medium-High | **Chưa làm** | Ingest vẫn sync; chưa offload NATS/worker. |
| 4 | Dedup in-memory per replica – duplicate khi multi-replica | Medium | **Chưa làm** | Chưa Redis dedup. |
| 5 | Không DLQ/retry cho failed ingest ở Core | Medium | **Chưa làm** | Ingest error → log & ACK. |
| 6 | SBOM drop khi sbomStreamCh đầy | High | **Chưa làm** | Tương tự #2. |
| 7 | Không cache cho Pod Detail API – DB load cao | Medium | **Chưa làm** | Chưa Redis cache cho GET. |
| 8 | Thiếu metrics Prometheus chi tiết cho pod flow | Medium | **Chưa làm** | Chưa histogram per-type. |
| 9 | Process collection qua exec ps – fragile | Medium | **Một phần** | Có `core/pkg/models/pod_process.go`, migration 071/072/073; Agent có `poddetail/procfs` (có thể hướng procfs) nhưng **không có `reporter.go`** trong workspace. |
| 10 | Không encryption at-rest cho sensitive pod data | Medium | **Chưa làm** | Chưa pgcrypto. |
| 11 | Dashboard polling – không real-time | Low | **Chưa làm** | Chưa WebSocket. |

**Kết luận ngắn:** Phần lớn đề xuất trong Technical Spec **chưa được triển khai**. Đã có: DB (migrations 068–073), models (pod_runtime_metrics, pod_processes, pod_network_connections, k8s_events), cleanup khi soft-delete pod trong `agent_service.go`, retention job cho `pod_processes`. Thiếu: handler HTTP Pod Detail ở Core, collector/reporter đầy đủ ở Agent, API GET cho metrics/processes/network/events, tab Process/Network/Events trên Dashboard.

---

## 3. Đối chiếu thiết kế (POD_DETAIL_SERVICES_DESIGN) với file thực tế

Tài liệu tham chiếu trong design (§11) ghi:

- **Handlers:** `core/internal/api/pod_detail_services_handlers.go`

### 3.1 File **có trong workspace** (đã thực hiện / đã có sẵn)

| Thành phần | File / vị trí | Ghi chú |
|------------|----------------|--------|
| DB migrations | `core/migrations/068_*`, `071_*`, `072_*`, `073_*`, `migrations.go` | Bảng pods (detail columns), pod_runtime_metrics, pod_processes, pod_network_connections, k8s_events. |
| Models | `core/pkg/models/pod_runtime_metrics.go`, `pod_process.go`, `pod_network_connection.go`, `k8s_event.go` | Đủ cho ingest/query. |
| Core cleanup | `core/internal/service/agent_service.go` | Xóa process/metrics/network theo pod_uid khi soft-delete pod. |
| Scheduler | `core/internal/scheduler/pod_process_retention_job.go` | Retention cho pod_processes. |
| Deploy | `deploy/fortuna-core-deployment.yaml`, `fortuna-agent-daemonset.yaml`, `fortuna-agent-service.yaml` | Core startupProbe; Agent port 81; Service agent. |
| Script deploy | `scripts/deploy/deploy-fortuna-robust.sh` | Apply agent service nếu có file. |
| Script verify | `scripts/verify/verify-pod-detail-api-and-db.sh`, `verify-pod-detail-empty-response.sh` | Kiểm tra schema, counts, bad pod_uid. |
| Dashboard | `dashboard/pages/PodDetail.tsx`, `lib/api.ts`, `types.ts` | Trang Pod Detail có Overview, SBOM, Risks; **chưa** có tab Process/Network/Events hay 4 API GET tương ứng. |

### 3.2 File **không có trong workspace** (bị “mất” / chưa từng tồn tại ở đây)

| File theo design / session | Lý do có thể “mất” (xem mục 4) |
|----------------------------|----------------------------------|
| `core/internal/api/pod_detail_services_handlers.go` | Được design doc và session nhắc là nơi chứa handler GET/POST cho runtime-metrics, processes, network-connections, events. **Trong repo hiện tại không tồn tại.** Routes tương ứng cũng chưa đăng ký trong `routes.go`. |
| `agent/internal/poddetail/reporter.go` | Technical Spec và design nhắc “reporter.go” (collect metrics/processes, gửi stream). **Trong workspace chỉ có thư mục `agent/internal/poddetail/` và `procfs/`**, không có file `reporter.go`. |
| `docs/03-components/podDetail/Pod_Detail_Implementation_Reference.md` | Session trước có mô tả §6 (API response cho Dashboard, xử lý API 200 nhưng không có dữ liệu). **File này không tồn tại trong thư mục podDetail.** |

Không có bằng chứng trong git history (trong phạm vi kiểm tra) cho thấy các file trên từng được commit rồi bị xóa; nhiều khả năng chúng **chưa từng được tạo trong nhánh/workspace hiện tại** hoặc chỉ tồn tại trong phiên làm việc (session) khác mà chưa được commit.

---

## 4. Vì sao tài liệu, mã nguồn, script có thể “bị xóa mất”?

Các lý do hợp lý (không loại trừ lẫn nhau):

1. **Chưa từng được tạo trong workspace này**  
   Plan và design doc **mô tả** file (ví dụ `pod_detail_services_handlers.go`, `reporter.go`), nhưng implementation có thể chưa được viết trong repo bạn đang mở. Session trước có thể đã **bàn và mô tả** thay đổi (refetchDetail dùng `pod.id ?? pod.uid`, thêm cột Container cho bảng Network, reject podUid rỗng/0) nhưng code thực tế được tạo trong một Cursor session / branch khác và chưa merge hoặc chưa pull về đây.

2. **Chỉ tồn tại trong session (bộ nhớ/context), chưa lưu đĩa / commit**  
   Cursor có thể đã tạo hoặc sửa file trong context của một conversation. Nếu:
   - không “Save” / không commit, hoặc  
   - session bị summarize/restart, workspace reload,  
   thì thay đổi có thể không còn trong working tree. Đặc biệt với file **mới** (chưa từng commit), chúng sẽ “mất” nếu chưa được ghi ra đĩa đúng lúc.

3. **Branch / workspace khác**  
   Công việc có thể đã làm trên branch khác (ví dụ `feature/pod-detail-handlers`). Ở branch hiện tại (ví dụ `main`) các file đó không có, nên tạo cảm giác “bị xóa”.

4. **Tài liệu tham chiếu đi trước implementation**  
   Design doc (§11) liệt kê `pod_detail_services_handlers.go` như **tài liệu tham chiếu** – tức là mô tả nơi **sẽ** đặt handler, chứ không nhất thiết file đã tồn tại. Vì vậy “thiếu file” có thể đơn giản là **implementation chưa làm**, chứ không phải file từng có rồi bị xóa.

5. **Backup session chỉ copy file tồn tại**  
   Khi backup “toàn bộ file đã thực hiện theo session”, chỉ những file **thực sự có trên đĩa** mới được copy. File chưa bao giờ tồn tại trong thư mục đó sẽ không có trong backup; MANIFEST đã ghi rõ các file “referred in session but not present in workspace at backup time”.

**Kết luận:** Phần lớn trường hợp “mất” file là do **chưa được tạo hoặc chưa được commit trong workspace/branch hiện tại**, hoặc do **làm ở session/branch khác** chưa được gộp lại, chứ không nhất thiết do ai đó xóa file đã commit.

---

## 5. Đề xuất bước tiếp theo (để khớp plan và design)

1. **Tạo `core/internal/api/pod_detail_services_handlers.go`**  
   - Implement handler GET (và nếu cần POST) cho:
     - `GET /api/v1/pods/:id/runtime-metrics` (và/by-uid/:uid/...)
     - `GET /api/v1/pods/:id/processes`
     - `GET /api/v1/pods/:id/network-connections`
     - `GET /api/v1/pods/:id/events` (hoặc tương đương)
   - Validate `podUid` (reject rỗng / "0") khi ingest (POST) nếu có.
   - Đăng ký route trong `core/internal/api/routes.go`.

2. **Tạo hoặc khôi phục `agent/internal/poddetail/reporter.go`**  
   - Thu thập metrics/processes (và sau này network/events) theo design; skip khi `uid == ""` hoặc `uid == "0"`.
   - Gửi qua stream hoặc HTTP theo cơ chế hiện tại của Core.

3. **Dashboard**  
   - Thêm tab Process, Network, Events trên Pod Detail; gọi 4 API trên với `pod.id ?? pod.uid`.
   - Thêm cột Container cho bảng Network (nếu API trả về `containerName`).

4. **Tài liệu**  
   - Tạo lại `Pod_Detail_Implementation_Reference.md` (hoặc tương đương) với §6 (API response, xử lý empty response) từ nội dung đã mô tả trong session, để đồng bộ tài liệu với implementation khi làm xong bước 1–3.

Sau khi thực hiện các bước trên, có thể chạy lại script verify (`verify-pod-detail-api-and-db.sh`, `verify-pod-detail-empty-response.sh`) và đối chiếu lại với 11 Findings để cập nhật bảng trạng thái trong tài liệu này.
