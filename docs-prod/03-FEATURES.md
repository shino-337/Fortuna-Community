# Chức năng FortunaK8s (Production)

FortunaK8s – K8S Security & Risk Management Platform.

## 1. SBOM (Software Bill of Materials)

- **Trích xuất tự động:** Agent trích SBOM từ image container trên mỗi node (containerd).
- **Parser:** dpkg (Debian/Ubuntu), apk (Alpine), rpm, npm, pip, gomod; chọn theo OS/image.
- **Lưu trữ:** Core lưu vào PostgreSQL (sboms, sbom_components); API `/api/v1/sbom`, export theo pod/namespace.
- **Dashboard:** Trang SBOM xem theo pod, image, package; export CSV/JSON.

---

## 2. CVE (Common Vulnerabilities and Exposures)

- **Nguồn CVE:** Bảng `cves`, `package_vulnerabilities` (load từ NVD hoặc script `load-cve-data.sh`).
- **Matching:** Worker nhận sự kiện SBOM → so khớp package (PURL, ecosystem, version) với CVE → lưu `cve_matches`.
- **Mức độ:** Critical / High / Medium / Low theo CVSS hoặc severity CVE.
- **API:** CVE matches theo pod, image, cluster; dùng cho Risk Center và báo cáo.

---

## 3. Security Insights

- **Tạo tự động:** Từ CVE matches và policy; mỗi insight gắn resource (pod/cluster), severity, title, mô tả.
- **Trạng thái:** active, resolved; có thể cập nhật qua API.
- **Risk Center:** Danh sách risks/insights theo cluster, bộ lọc severity, tổng hợp (total, critical, high).
- **API:** `/api/v1/insights`, `/api/v1/insights/summary`, `/api/v1/risks`.

---

## 4. Pod Capability Engine (PCE)

- **Capability metadata:** Định nghĩa capability (ID, group, severity, MITRE), seed trong DB.
- **Promotion rules:** Quy tắc nâng cấp capability (theo signal hoặc capability khác).
- **Pod capabilities:** Agent/Core phát hiện capability trên pod (e.g. privileged, hostPath, token) → lưu `pod_capabilities`.
- **Attack steps:** Inference attack step từ capability/signal; API `/api/v1/attack-steps`, visualization.
- **API:** `/api/v1/capability-metadata`, `/api/v1/promotion-rules`, `/api/v1/pod-capabilities`, trends.

---

## 5. Runtime Signals

- **Thu thập:** Runtime events (e.g. proc escape, fs escape) gửi lên Core → lưu `runtime_events`, `runtime_signals`.
- **Tương quan:** Severity và promotion từ signal → insight/risk.
- **API:** `/api/v1/runtime-signals`, `/api/v1/runtime-signals/pods/:podUid`; dùng trong Risk Center và Dashboard.

---

## 6. Dashboard (Web UI)

- **Tổng quan:** Số cluster, agent, pod, risk; biểu đồ Threat Velocity, PCE trends.
- **Risk Center:** Danh sách risks, filter theo cluster/severity, chi tiết insight; tab Runtime Signals.
- **SBOM:** Duyệt SBOM theo pod/image, tìm kiếm package, export.
- **Resources / Pod detail:** Pod list; trang Pod Detail hiển thị:
  - **Header:** Status, Pod IP, Start Time, Uptime, Restart Count, QoS Class, Risk Count, Service Account, Created.
  - **Overview:** Namespace, Node, Pod IP, Service Account, UID; block **Identity & Ownership** (Owner Type, Owner Name, ReplicaSet, QoS Class) khi có dữ liệu.
  - Tab SBOM, Related Risks. Dữ liệu pod detail (pod_ip, start_time, owner_*, qos_class) từ Core API; cần Agent bản mới sync đầy đủ (xem [04-USER_GUIDE](04-USER_GUIDE.md)).
- **Đăng nhập:** JWT (mặc định admin/admin123); production nên đổi password và dùng secret.

---

## 7. API REST

- **Auth:** POST `/api/v1/auth/login` → JWT; gửi header `Authorization: Bearer <token>`.
- **Nhóm API:** dashboard/stats, clusters, agents, pods, risks, insights, sbom, pod-capabilities, runtime-signals, promotion-rules, capability-metadata, attack-steps, health, metrics.
- **Tài liệu chi tiết:** `docs/06-reference/API_REFERENCE.md`.

---

## 8. Pod Detail (POD_DETAIL_SPEC)

- **Cột DB (migration 068):** `pod_ip`, `start_time`, `restart_count`, `owner_kind`, `owner_name`, `replica_set_name`, `qos_class`.
- **API:** GET `/api/v1/pods`, `/pods/:id`, `/pods/by-uid/:uid` trả về các trường trên (camelCase: podIP, startTime, ownerKind, qosClass, …).
- **Spec hash (069, 070):** Agent gửi `specHash`; Core chỉ trigger PCE khi spec thay đổi; `last_evaluated_hash` tránh ghi đè kết quả cũ khi race. Chi tiết: [POD_SYNC_ARCHITECTURE_AND_DATA_MODEL_SPEC](../docs/03-components/podDetail/POD_SYNC_ARCHITECTURE_AND_DATA_MODEL_SPEC.md).

---

## 9. Khác

- **Cluster identity:** Tự phát hiện cluster (kube-system UID) hoặc override ConfigMap/env; Core là SSOT cho cluster.
- **Stale cleanup:** Cluster không sync &gt; 90 ngày có thể bị soft-delete; Dashboard mặc định chỉ hiển thị cluster active (7 ngày).
- **Migrations:** Core chạy migrations khi start; schema do code quản lý (thư mục core/migrations). Migration 068: pod detail columns; 069/070: spec_hash, last_evaluated_hash.
- **Admission Webhook:** Tùy chọn; dùng để enforce policy khi tạo/sửa resource.

---

**Tiếp theo:** [Hướng dẫn sử dụng](04-USER_GUIDE.md)
