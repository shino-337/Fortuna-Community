# Agent: Runtime Metrics & Process Data (Pod Detail)

## Tại sao Agent chưa gửi runtime-metrics / processes lên Core

### 1. Core đã sẵn sàng

- **API:** Core đã có endpoint nhận và trả dữ liệu:
  - `POST /api/v1/pod-runtime-metrics` – nhận danh sách metrics theo container (CPU, memory, state, restart count) theo `podUid`.
  - `POST /api/v1/pod-processes` – nhận snapshot process (PID, user, CPU%, mem%, command) theo `podUid`.
  - `GET /api/v1/pods/:id/runtime-metrics` và `GET /api/v1/pods/:id/processes` – dashboard dùng để hiển thị tab Runtime / Processes.
- **DB:** Bảng `pod_runtime_metrics` và `pod_processes` đã có (migration 071, 072).

### 2. Agent chưa có luồng thu thập và gửi

Trong codebase Agent **không có**:

- **Runtime metrics:** Không có code gọi nguồn metrics (kubelet stats/summary hoặc metrics API), không có code build payload và **không có** HTTP POST tới `pod-runtime-metrics`.
- **Processes:** Không có code lấy danh sách process trong container (exec vào container hoặc CRI), không build payload và **không có** POST tới `pod-processes`.

Tức là **chưa từng có tính năng “Pod Detail – Runtime / Process” ở phía Agent**; chỉ có sẵn ở Core và Dashboard.

### 3. Luồng Agent hiện tại gửi gì lên Core

| Luồng            | Giao thức | Endpoint / RPC        | Mục đích                          |
|------------------|-----------|------------------------|-----------------------------------|
| Full sync        | HTTP      | `POST /api/v1/agent/sync` | Pods, RBAC, SA, … (metadata)      |
| SBOM             | gRPC      | `SendSBOMFinding`     | Thành phần phần mềm trong image   |
| Runtime events   | HTTP      | `POST /api/v1/runtime-events` | Sự kiện runtime (tùy chọn, từ file) |
| Heartbeat/Ping   | gRPC      | `Ping`                 | Agent còn sống                    |

Không có luồng nào trong bảng trên gửi **container CPU/memory/state** hay **danh sách process trong container**.

### 4. Kết luận

- **Lý do “chưa gửi”:** Agent chưa được triển khai (implement) phần thu thập và gửi runtime-metrics / processes; đây là tính năng chưa có trong Agent, không phải lỗi cấu hình.
- **Để có dữ liệu thật (không dùng seed):** Cần implement trong Agent:
  - Thu thập **runtime metrics** (ví dụ từ kubelet `/stats/summary` qua API server proxy) và POST lên `pod-runtime-metrics`.
  - Thu thập **process list** (ví dụ exec `ps` trong từng container trên node của agent) và POST lên `pod-processes`.

Tài liệu này mô tả cách triển khai đó và cấu hình liên quan.

---

## Thiết kế triển khai (dữ liệu thực)

### Nguồn runtime metrics

- **Kubelet Summary API:**  
  `GET /api/v1/nodes/<NodeName>/proxy/stats/summary` (qua API server).  
  Trả về CPU (nano core), memory (usage/working set) theo pod và container. Agent chạy DaemonSet trên từng node, gọi proxy cho **chính node đó** (`cfg.NodeName`).
- **State & restart count:** Lấy từ Pod spec/status (container status) đã có khi sync/list pod, gộp với dữ liệu stats theo `(pod UID, container name)`.

### Nguồn process list

- **Exec vào container:** Với mỗi pod trên node, từng container: dùng Kubernetes Exec (`kubectl exec` tương đương) chạy lệnh kiểu `ps -eo pid,ppid,user,%cpu,%mem,args` (hoặc lệnh tương thích với image), parse stdout và gửi lên Core.
- Chỉ thu thập cho pod có `nodeName == cfg.NodeName`.

### Gửi lên Core

- Dùng **Core HTTP endpoint** (`CORE_HTTP_ENDPOINT`), giống syncer:
  - `POST .../api/v1/pod-runtime-metrics` – body JSON đúng format Core (podUid, clusterId, namespace, metrics[]).
  - `POST .../api/v1/pod-processes` – body JSON (podUid, clusterId, namespace, processes[]).
- Nếu Core bật auth: Agent cần gửi Bearer token (login hoặc token cấu hình); hiện syncer không gửi auth nên thường dùng khi Core tắt auth hoặc có route nội bộ.

### Cấu hình Agent (đã implement)

- **Env (trong agent):**
  - `POD_DETAIL_RUNTIME_METRICS_ENABLED` (default `true`) – bật gửi runtime metrics.
  - `POD_DETAIL_RUNTIME_METRICS_INTERVAL` (default `2m`) – chu kỳ thu thập + POST metrics.
  - `POD_DETAIL_PROCESSES_ENABLED` (default `true`) – bật gửi process snapshot.
  - `POD_DETAIL_PROCESSES_INTERVAL` (default `3m`) – chu kỳ exec + POST processes.
- Dùng chung `CORE_HTTP_ENDPOINT`, `ClusterID`, `NodeName`, `WATCH_NAMESPACE` với syncer.

**Code:** `agent/internal/poddetail/reporter.go` – gọi kubelet `stats/summary` (qua API server proxy) và exec `ps` trong từng container trên node, rồi POST lên Core.

Sau khi deploy agent mới và bật config (mặc định đã bật), Pod Detail trên dashboard sẽ nhận **dữ liệu thực** từ agent (Runtime tab và Processes tab) mà không cần seed.
