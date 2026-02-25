# Báo cáo E2E Full – Kiểm tra đầy đủ chức năng, luồng dữ liệu và giao diện

**Ngày thực hiện**: 2026-02-03  
**Phiên bản báo cáo**: 1.0  
**Kết quả tự động**: `E2E-COMPREHENSIVE-20260203-074047.md`

---

## 1. Tổng quan

### 1.1 Mục tiêu

- Kiểm tra **đầy đủ** từng chức năng hiện có: hiển thị UI/UX và thao tác giao diện.
- Thực hiện **full luồng end-to-end**: process xử lý, dữ liệu K8s thực tế, dữ liệu ghi nhận DB, API và giao diện.
- Tạo báo cáo chi tiết: nội dung thực hiện, chức năng đã kiểm tra, kết quả theo từng bước và kết quả cuối cùng.

### 1.2 Phạm vi

| Hạng mục | Nội dung |
|----------|----------|
| **Kubernetes** | Nodes, Pods (fortuna), Services |
| **Core API** | Auth, Health, toàn bộ API phục vụ Dashboard |
| **Database** | Row counts, sample clusters/insights, đồng bộ với K8s |
| **Luồng dữ liệu** | K8s → Agent → Core → DB; Dashboard → API → Core |
| **Giao diện** | Checklist từng trang: Dashboard, Clusters, Risk Center, Resources, Capabilities, Rules, Attack Paths, Monitoring, Settings, v.v. |

### 1.3 Kết quả tổng hợp (tự động)

| Loại | Số lượng |
|------|----------|
| **PASS** | 38 API tests |
| **WARN** | 1 (GET /metrics/system – response không có key `timestamp`) |
| **FAIL** | 0 |

---

## 2. Process xử lý – Luồng dữ liệu

### 2.1 Luồng K8s → Core → Database

```
[Kubernetes Cluster]
    │
    ├── Pods / Nodes / Resources
    │       │
    │       ▼
    [Fortuna Agent (DaemonSet)]
    │   - Thu thập: pods, SBOM, runtime signals, capabilities
    │   - Gửi qua NATS / HTTP tới Core
    │       │
    │       ▼
    [Fortuna Core]
    │   - API HTTP :8080
    │   - Workers: correlator, risk, SBOM
    │   - Ghi/đọc DB
    │       │
    │       ▼
    [PostgreSQL]
        - clusters, pods, insights, pod_capabilities,
          runtime_signals, agents, promotion_rules, capability_metadata, ...
```

### 2.2 Luồng Dashboard (UI) → API → Hiển thị

```
[Browser] → http://localhost:8081 (hoặc NodePort)
    │
    ├── Login → POST /api/v1/auth/login → JWT
    │
    ├── Mỗi trang gọi API tương ứng:
    │   - Dashboard (/)     → GET /dashboard/stats, /insights/summary, /dashboard/metrics/threat-velocity,
    │                         /pod-capabilities/trends, /notifications, /agents/status, ...
    │   - Clusters          → GET /clusters, /clusters/stats, /clusters/:id/overview, ...
    │   - Risk Center       → GET /risks, /insights/summary (với sinceMinutes theo time range)
    │   - Resources         → GET /resources, /pods
    │   - Capabilities      → GET /pod-capabilities/summary, /capability-metadata, /promotion-rules
    │   - Rules             → GET /rules
    │   - Attack Paths      → GET /attack-paths/graph, /graph
    │   - Monitoring        → GET /metrics/system, /agents/status, ...
    │   - Settings          → GET /me, ...
    │
    └── Dashboard (nginx) proxy /api → Core (fortuna-core:8080)
```

### 2.3 Đồng bộ dữ liệu (kết quả kiểm tra)

- **Pods**: K8s namespace `fortuna` = 8 pods (Core, Dashboard, Agent x2, NATS x3, Postgres). DB bảng `pods` = 26 (toàn bộ workload pods mà Agent đã đồng bộ từ cluster).
- **Agents**: DB `agents` (active) = 2, K8s agent pods (Running) = 2 → **khớp**.

---

## 3. Chức năng đã kiểm tra (theo từng bước)

### 3.1 Kubernetes – Cluster & Pods

| Bước | Nội dung | Kết quả |
|------|----------|---------|
| 1.1 | Nodes (kubectl get nodes) | 2 nodes: k8s-master, k8s-worker01, Ready |
| 1.2 | Pods namespace fortuna | 8 pods Running (core, dashboard, agent x2, nats x3, postgres) |
| 1.3 | Services | fortuna-core (ClusterIP 8080), fortuna-dashboard (LoadBalancer NodePort 30124), postgres, nats |

### 3.2 Auth & Core API

| Bước | Nội dung | Kết quả |
|------|----------|---------|
| 2.0 | Core pod tồn tại, JWT login (admin/admin123) | **PASS** |
| 2.1 | GET /health | **PASS** (200) |

### 3.3 API – Từng chức năng (tương ứng từng trang / block UI)

| Nhóm chức năng | API đã test | Kết quả |
|----------------|-------------|---------|
| **Dashboard – trang chủ** | GET /dashboard/stats, /dashboard/stats?sinceMinutes=60, /insights/summary, /insights/summary?sinceMinutes=1440 | **PASS** (200, có đủ key) |
| **Clusters** | GET /clusters, /clusters/stats | **PASS** |
| **Risk Center** | GET /risks, /insights | **PASS** |
| **Resources** | GET /resources, /pods | **PASS** |
| **Capabilities & PCE** | GET /pod-capabilities/summary, /pod-capabilities/summary/capability, /pod-capabilities/trends?days=7, /capability-metadata, /promotion-rules | **PASS** |
| **Runtime & Attack steps** | GET /runtime-signals?limit=5, /attack-steps/summary | **PASS** |
| **Notifications (Recent Activity)** | GET /notifications | **PASS** |
| **Dashboard charts** | GET /dashboard/metrics/threat-velocity?days=7 | **PASS** |
| **Agents (Cluster Health)** | GET /agents/status | **PASS** |
| **Attack Paths & Graph** | GET /attack-paths/graph, /graph | **PASS** |
| **Rules & Policies** | GET /rules | **PASS** |
| **SBOM** | GET /sbom | **PASS** |
| **Audit & Reports** | GET /audit, /reports | **PASS** |
| **Metrics & Error logs** | GET /metrics/system, /error-logs | **WARN** (metrics/system thiếu key timestamp), **PASS** (error-logs) |

### 3.4 Database – Dữ liệu ghi nhận

| Bảng | Số dòng (thời điểm test) |
|------|---------------------------|
| clusters | 1 |
| pods | 26 |
| insights | 24 |
| pod_capabilities | 69 |
| runtime_signals | 7 |
| agents | 2 |
| promotion_rules | 9 |
| capability_metadata | 11 |

**Sample insights (Risk Center)**: CVE-2025-31133 in busybox, severity medium, status active.

### 3.5 Đồng bộ K8s ↔ Core

- **Pods**: K8s (fortuna) = 8; DB (pods) = 26 → DB phản ánh toàn bộ workload pods đã sync từ cluster.
- **Agents**: DB = 2, K8s Running = 2 → **đồng bộ**.

---

## 4. Checklist UI/UX – Thao tác với giao diện (kiểm tra thủ công)

Cần mở Dashboard qua **NodePort** hoặc **port-forward**:

- Trong cluster: `http://192.168.56.100:30124`
- Từ máy host: `kubectl port-forward -n fortuna svc/fortuna-dashboard 8081:80` → `http://localhost:8081`

**Login**: admin / admin123

| Trang / Chức năng | Nội dung kiểm tra | Ghi chú |
|-------------------|-------------------|--------|
| **Login** | Nhập admin/admin123 → redirect về Dashboard, có JWT | |
| **Dashboard (/) ** | Thẻ Infrastructure (Clusters, Pods, Agents), Security Risks (Critical/High/Medium/Low, Affected Workloads), Risks & Trends (critical risks, charts), sidebar Cluster & Activity (Cluster Health, Pod Capabilities, Recent Activity), Time range filter | Dữ liệu lấy từ /dashboard/stats, /insights/summary (sinceMinutes), /pod-capabilities/trends, /notifications, /agents/status |
| **Clusters** | Danh sách cluster, vào cluster detail → overview, inventory, agents, security summary | GET /clusters, /clusters/:id/overview, ... |
| **Resources** | Tabs (Pod, Deployment, ...), Service Accounts (tab hoặc link Identities) | GET /resources |
| **Risk Center** | Danh sách risks/insights, filter severity, search, time range, chi tiết risk | GET /risks, /insights/summary (sinceMinutes) |
| **Capabilities** | Danh sách capability, summary theo cluster/namespace/severity | GET /pod-capabilities/summary/*, /capability-metadata |
| **Service Accounts** | Link từ sidebar → Resources?tab=ServiceAccount | GET /resources (type ServiceAccount) |
| **Rules & Policies** | Danh sách rules, chi tiết rule | GET /rules |
| **Attack Paths** | Đồ thị attack path | GET /attack-paths/graph, /graph |
| **Monitoring** | Metrics, agents status | GET /metrics/system, /agents/status |
| **Settings** | User, đổi mật khẩu | GET /me, POST /change-password |
| **Audit / Reports / Error logs** | Danh sách audit logs, reports, error logs | GET /audit, /reports, /error-logs |
| **Global search** | Gõ từ khóa → Enter/Search → chuyển tới Risk Center với ?search= | |
| **Cluster selector** | Đổi cluster → dữ liệu stats/insights lọc theo cluster | clusterId query param |
| **Time range** | 1h / 24h / 7d → sinceMinutes gửi vào API, số liệu cập nhật | |

---

## 5. Dữ liệu thực tế – Tóm tắt

### 5.1 Kubernetes (thời điểm test)

- **Nodes**: k8s-master (192.168.56.100), k8s-worker01 (192.168.56.101), Ready.
- **Pods fortuna**: fortuna-core, fortuna-dashboard, fortuna-agent (x2), nats (x3), postgres – đều Running.

### 5.2 Database (PostgreSQL)

- 1 cluster, 26 pods, 24 insights, 69 pod_capabilities, 7 runtime_signals, 2 agents, 9 promotion_rules, 11 capability_metadata.
- Insights mẫu: CVE-2025-31133 (medium, active).

### 5.3 API

- Toàn bộ API kiểm tra trả về HTTP 200 và payload có đủ key (trừ GET /metrics/system có 1 WARN về key `timestamp`).

---

## 6. Kết quả cuối cùng

### 6.1 Tổng kết theo hạng mục

| Hạng mục | Kết quả | Ghi chú |
|----------|---------|--------|
| **Kubernetes** | **PASS** | Nodes, pods, services ổn định |
| **Auth & Core** | **PASS** | Login, health OK |
| **API từng chức năng** | **PASS** (38/38 logic, 1 WARN) | 1 WARN: /metrics/system response format |
| **Database** | **PASS** | Dữ liệu có, đồng bộ với agents |
| **Đồng bộ K8s ↔ Core** | **PASS** | Agents khớp; pods DB phản ánh workload đã sync |
| **UI/UX (checklist)** | **Thủ công** | Cần mở Dashboard và làm theo checklist mục 4 |

### 6.2 Kết luận

- **Process xử lý**: Luồng K8s → Agent → Core → DB và Dashboard → API → Core hoạt động đúng; dữ liệu K8s được ghi nhận vào DB và phục vụ API.
- **API**: Đầy đủ API phục vụ từng chức năng (Dashboard, Clusters, Risk Center, Resources, Capabilities, Rules, Attack Paths, Monitoring, Audit, v.v.) đều trả về 200 và dữ liệu đúng format.
- **Database**: Số liệu ghi nhận nhất quán (clusters, pods, insights, pod_capabilities, agents, ...).
- **Giao diện**: Cần kiểm tra thủ công theo checklist mục 4 (đăng nhập, từng trang, time range, cluster selector, global search) để xác nhận hiển thị và thao tác UI/UX.

### 6.3 Khuyến nghị

1. **GET /metrics/system**: Kiểm tra handler trả về đúng format (có key `timestamp` hoặc chuẩn hóa response) để bỏ WARN.
2. **UI**: Chạy lần lượt checklist mục 4 sau khi mở Dashboard (NodePort hoặc port-forward).
3. **E2E tự động**: Dùng `scripts/e2e/run-e2e-complete-with-monitor.sh` (full E2E + Risk Center + API snapshot) hoặc `scripts/e2e/run-e2e-full.sh`; báo cáo: `docs/test-results/E2E-COMPLETE-WITH-MONITOR-<timestamp>.md` hoặc `E2E-FULL-<timestamp>.md`.

---

**Kết thúc báo cáo E2E Full.**

- Báo cáo chi tiết từng bước (raw): `docs/test-results/E2E-COMPREHENSIVE-20260203-074047.md`
- Script chạy lại: `scripts/e2e/run-e2e-complete-with-monitor.sh` hoặc `scripts/e2e/run-e2e-full.sh`
