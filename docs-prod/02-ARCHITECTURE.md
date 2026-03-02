# Kiến trúc Fortuna (Production)

## 1. Tổng quan kiến trúc

```
                    ┌─────────────────────────────────────────────────────────┐
                    │                    Kubernetes Cluster                     │
  ┌──────────────┐  │  ┌─────────────┐     gRPC (mTLS)      ┌─────────────┐  │
  │   Browser    │  │  │   Agent     │ ◄──────────────────► │    Core      │  │
  │   (User)     │  │  │ (DaemonSet  │                      │ (Deployment) │  │
  └──────┬───────┘  │  │  per node)  │                      └──────┬──────┘  │
         │          │  └─────────────┘                             │          │
         │ HTTP     │        │                                      │          │
         │          │        │ SBOM / Pod sync                      │          │
         ▼          │        ▼                                      ▼          │
  ┌──────────────┐  │  ┌─────────────┐                      ┌─────────────┐  │
  │  Dashboard   │  │  │  (Agent     │                      │ PostgreSQL  │  │
  │ (Deployment) │  │  │   x N nodes)│                      │   + NATS   │  │
  └──────────────┘  │  └─────────────┘                      └─────────────┘  │
                    └─────────────────────────────────────────────────────────┘
```

---

## 2. Thành phần chính

### 2.1 Core (Deployment)

- **Vai trò:** Trung tâm xử lý, lưu trữ, API.
- **Chạy trên:** Node control-plane (nodeSelector + tolerations).
- **Cổng:** HTTP 8080 (REST, health, metrics), gRPC 9090 (Agent).
- **Tính năng:** SBOM lưu DB, CVE matching, tạo insights, PCE scheduler, runtime signals, REST API (51+ endpoint), migrations DB khi khởi động.
- **Phụ thuộc:** PostgreSQL, NATS JetStream.

### 2.2 Agent (DaemonSet)

- **Vai trò:** Mỗi node một pod; theo dõi pod, trích xuất SBOM, đồng bộ với Core.
- **Giao tiếp:** gRPC (mTLS) tới Core.
- **Cluster identity:** Tự phát hiện từ K8s API (kube-system UID) hoặc override qua ConfigMap/env.
- **SBOM:** Nhiều parser (dpkg, apk, rpm, npm, pip, gomod), hàng đợi xử lý bất đồng bộ.

### 2.3 Dashboard (Deployment)

- **Vai trò:** Giao diện web (React + TypeScript).
- **Dữ liệu:** Gọi API Core (proxy /api tới Core), hiển thị Dashboard, Risk Center, SBOM, PCE, runtime signals.

### 2.4 Hạ tầng

- **PostgreSQL:** Schema do Core quản lý (migrations), lưu clusters, pods, SBOM, CVE, insights, capabilities, runtime_signals, v.v.
- **NATS JetStream:** Hàng đợi sự kiện (SBOM_CREATED, v.v.), 3 replica cho HA.

---

## 3. Luồng dữ liệu chính

1. **Pod xuất hiện** → Agent phát hiện → Đưa vào queue SBOM.
2. **Agent trích xuất SBOM** → Gửi Core qua gRPC → Core lưu DB.
3. **Core publish NATS** → Worker CVE matching → So khớp CVE với package (PURL, ecosystem).
4. **Tạo insights** (critical/high/medium) → Lưu DB → API phục vụ Dashboard/Risk Center.
5. **PCE:** Scheduler đánh giá capability, promotion rules; runtime events → runtime_signals.
6. **Dashboard:** Gọi REST API (stats, clusters, risks, insights, SBOM, pod-capabilities, runtime-signals).

---

## 4. Bảo mật

- **mTLS:** Agent ↔ Core dùng certificate (fortuna-core-tls, fortuna-agent-tls, fortuna-ca-cert).
- **Auth:** Core bật JWT (admin/admin123 mặc định; production nên đổi và dùng secret).
- **RBAC:** ServiceAccount fortuna-core, fortuna-agent; ClusterRole/ClusterRoleBinding cho Core và Agent.
- **Secrets:** Certificate và config nhạy cảm lưu Kubernetes Secret, không hardcode.

---

## 5. High availability & khả năng mở rộng

- **NATS:** 3 replica StatefulSet.
- **PostgreSQL:** Single instance trong deploy mẫu; production có thể dùng HA (Patroni, Cloud SQL, RDS).
- **Core:** 1 replica mặc định; có thể tăng replicas nếu stateless (cần chia sẻ DB/NATS).
- **Agent:** DaemonSet – tự scale theo số node.

---

## 6. Quan sát (Observability)

- **Health:** Core `/health`, `/health/dashboard-data-integrity`.
- **Metrics:** Core `/metrics` (Prometheus format).
- **Log:** stdout/stderr Core và Agent; thu thập bằng logging stack (e.g. EFK, Loki) tùy môi trường.

---

**Tiếp theo:** [Chức năng chi tiết](03-FEATURES.md)
