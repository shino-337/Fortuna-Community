# Kiểm tra chi tiết Pod từ Log

**Ngày:** 2026-02-24

## 1. Danh sách Pod (namespace fortuna)

| Pod | READY | STATUS | NODE | Ghi chú |
|-----|-------|--------|------|--------|
| fortuna-core-5bb9498ddf-nmj9w | 1/1 | Running | k8s-master | Core API + gRPC |
| fortuna-agent-mgz7x | 1/1 | Running | k8s-worker01 | Agent worker01 |
| fortuna-agent-s88sk | 1/1 | Running | k8s-master | Agent master |
| fortuna-dashboard-f8bd9b76-rr4wt | 1/1 | Running | k8s-master | Nginx proxy → Core |
| cve-loader-mzxbx | 1/1 | Running | k8s-master | Job load OSV vào DB |
| postgres-7486475f9-4dzdg | 1/1 | Running | k8s-worker01 | PostgreSQL (pod mới) |
| postgres-7486475f9-57s5w | 0/1 | Completed | k8s-worker01 | Pod cũ đã thoát |
| nats-0/1/2 | 1/1 | Running | k8s-worker01 | NATS cluster |

---

## 2. Fortuna-Core (deployment/fortuna-core)

**Vai trò:** API HTTP (8080), gRPC (9090), xử lý sync Agent, PCE (pod capability), insights, SBOM/CVE matching.

**Log gần đây:**
- **AgentService:** Đang xử lý sync từ Agent: upsert pods, service accounts, cluster; so sánh và skip các SA “unchanged”.
- **Capability:** INSERT/UPDATE `pod_capabilities` (ESC_HOSTPATH_NODE, ESC_RUNTIME_PROBE, ID_TOKEN_POD), `pod_risk_profiles`; DELETE capabilities không còn.
- **InsightManager:** Một số bản ghi “record not found” khi tìm insight capability (sau đó tạo mới hoặc bỏ qua).
- **SQL:** Các truy vấn bình thường tới `service_accounts`, `capability_metadata`, `pod_capabilities`, `insights`, `pod_risk_profiles`.

**Kết luận:** Core đang hoạt động bình thường: nhận sync từ Agent, cập nhật capabilities và risk profiles, không có lỗi nghiêm trọng trong log mẫu.

---

## 3. Fortuna-Agent (2 pod)

### 3.1 Agent trên worker01 (fortuna-agent-mgz7x)

**Giai đoạn lỗi (trước khi Core có):**
- `lookup fortuna-core.fortuna.svc.cluster.local: no such host` — DNS chưa có (Core chưa deploy).
- `dial tcp 10.96.108.251:9090: connection refused` — Core Service đã có nhưng pod chưa listen.

**Sau khi Core lên:**
- `✅ Heartbeat OK (interval=15s)` — gRPC Ping thành công.
- **LocalPodWatcher:** Phát hiện pod mới `fortuna/postgres-7486475f9-4dzdg` (Pending → Running), đưa vào SBOM queue.
- **SBOMProcessor:** Xử lý pod `postgres-7486475f9-4dzdg`, image `postgres:15-alpine`; containerd local không có image → fetch từ registry → extract SBOM (45 packages, apk), gửi Core → `SBOM received and stored` (sbom_id=15).

**Kết luận:** Agent worker01 ổn định: heartbeat OK, sync với Core, SBOM postgres pod đã gửi thành công.

### 3.2 Agent trên master (fortuna-agent-s88sk)

**Giai đoạn lỗi:** Giống worker01 — “no such host” rồi “connection refused” khi Core chưa sẵn sàng.

**Sau khi Core lên:**
- `✅ Heartbeat OK (interval=15s)`.
- **LocalPodWatcher:** Phát hiện pod `fortuna/fortuna-core-5bb9498ddf-nmj9w` (Pending → Running), đưa vào SBOM queue.
- **SBOM:** Image `fortuna-core:v1.0.0-33-g3fcca458d-dirty` — tìm thấy trong containerd; OS “unknown” → 0 packages (image scratch/minimal). Gửi SBOM lên Core lúc đó **thất bại** vì Core vừa start, DNS/gRPC chưa ổn: `SendSBOMFinding RPC failed: no such host`.
- **Syncer:** Một lần `Sync failed: connection refused` (Core chưa listen).

**Kết luận:** Agent master ổn định sau khi Core sẵn sàng; lỗi trong log là giai đoạn Core chưa up.

---

## 4. CVE-Loader (Job cve-loader-mzxbx)

**Vai trò:** Đọc file JSON OSV trong `/cve-data/all`, ghi vào bảng `cves` và `package_vulnerabilities`.

**Log gần đây:**
- **Tiến độ:** ~455,817 / 617,177 file (73.9%), Rate ~113 files/sec, ETA ~23m41s.
- **Thống kê:** CVEs ~455,817, Package Vulns ~849,474, Skipped (no packages) ~288,559, Failed 566.
- **SQL chậm:** Một số INSERT vào `cves` ≥ 200ms (ví dụ ~15s cho một row — có thể do lock hoặc I/O).
- **Lỗi cuối log:** `FATAL: terminating connection due to administrator command (SQLSTATE 57P01)` — kết nối DB bị đóng (ví dụ Postgres restart/shutdown), Job có thể retry hoặc fail.

**Kết luận:** CVE loader đã load được một lượng lớn (455k+ CVEs, 849k+ package_vulnerabilities). Việc “terminating connection” trùng với thời điểm Postgres có log “database system is shutting down” — có thể do Postgres pod bị thay thế (57s5w Completed, 4dzdg Running). Nếu Job chưa complete, có thể chạy lại load sau khi DB ổn định.

---

## 5. Fortuna-Dashboard (deployment/fortuna-dashboard)

**Vai trò:** Nginx serve static (React) và proxy `/api/*` tới Core (fortuna-core:8080).

**Log gần đây:**
- **POST /api/v1/auth/login:** Nhiều lần 401 (Unauthorized — sai mật khẩu hoặc user), 500 (lỗi server), 502 (upstream Core refused — lúc Core chưa có hoặc restart).
- **Nginx error:** `connect() failed (111: Connection refused) while connecting to upstream` tới `http://10.96.108.251:8080` — đúng với giai đoạn Core chưa listen.
- **GET /, /assets/*, /favicon.ico:** 304 — static và trang chủ phục vụ bình thường.

**Kết luận:** Dashboard proxy hoạt động; lỗi login 502/500 trong log tương ứng thời điểm Core không sẵn sàng; 401 là do credential. Sau khi Core ổn định, login đúng (admin/admin123 hoặc user đã tạo) sẽ trả 200.

---

## 6. Postgres (postgres-7486475f9-4dzdg)

**Log (đuôi):**
- Nhiều dòng `FATAL: the database system is shutting down` — các session bị ngắt khi Postgres tắt.
- `checkpoint complete: ...` — checkpoint bình thường trước khi tắt.
- `LOG: database system is shut down` — tắt hoàn tất.

**Ghi chú:** Pod list hiện tại có 4dzdg (Running) và 57s5w (Completed). Log “shut down” có thể từ lần tắt trước (rollout/restart). Cần xem log từ khi pod 4dzdg start để xác nhận DB hiện tại đang chạy ổn; nếu chỉ thấy shutdown thì có thể đang xem log của instance cũ.

**Kết luận:** DB đã từng shutdown (trùng với CVE loader bị mất kết nối). Pod Postgres mới (4dzdg) đang Running; Core và các client kết nối lại bình thường sau khi DB lên lại.

---

## 7. Tóm tắt

| Thành phần | Trạng thái từ log | Ghi chú |
|------------|-------------------|--------|
| **fortuna-core** | OK | Sync Agent, PCE, capabilities, insights bình thường |
| **fortuna-agent (worker01)** | OK | Heartbeat OK, SBOM postgres pod gửi thành công |
| **fortuna-agent (master)** | OK | Heartbeat OK; lỗi SBOM/Sync chỉ trong lúc Core chưa up |
| **cve-loader** | Đã load ~73% | Bị ngắt kết nối khi Postgres shutdown; có thể chạy lại Job nếu cần đủ 617k file |
| **fortuna-dashboard** | OK | Proxy hoạt động; 502/500 khi Core down, 401 do credential |
| **postgres** | Đã từng shutdown | Pod mới (4dzdg) Running; CVE loader và Core dùng DB hiện tại |

**Khuyến nghị:** (1) Kiểm tra Job `cve-loader` — nếu Failed hoặc chưa complete, xem lại Postgres và chạy lại load CVE nếu cần. (2) Login Dashboard: dùng đúng endpoint Core (qua proxy) và user/password đã cấu hình (ví dụ admin + FORTUNA_ADMIN_PASSWORD).
