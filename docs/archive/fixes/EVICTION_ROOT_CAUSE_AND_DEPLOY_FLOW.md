# Nguyên nhân Eviction (Core/Agent) và Luồng Deploy / Migration / Load OSV

## 1. Nguyên nhân chi tiết: Pod Core, Agent bị Evicted

### 1.1 Kết luận

**Nguyên nhân trực tiếp:** Node **k8s-worker01** bị **DiskPressure** (thiếu ephemeral-storage). Kubelet evict mọi pod chạy trên node đó khi ngưỡng disk vượt quá.

**Các pod bị ảnh hưởng trên worker01:**
- **postgres** (StatefulSet/Deployment) – evicted
- **nats** – evicted
- **fortuna-agent** (DaemonSet, 1 pod trên mỗi node) – evicted trên worker01
- **fortuna-core-*** (các replica cũ từ deployment trước) – evicted khi từng được schedule trên worker01

**Pod không bị evict trên master:** fortuna-core, fortuna-dashboard, fortuna-agent (trên k8s-master) chạy trên **k8s-master**; master không bị DiskPressure nên vẫn Running.

### 1.2 Bằng chứng từ cluster

**Sự kiện (events):**
```
Warning  EvictionThresholdMet   45m (x790 over 2d13h)  kubelet  Attempting to reclaim ephemeral-storage
Warning  FreeDiskSpaceFailed    ...  kubelet  Failed to garbage collect required amount of images. Attempted to free 718178713 bytes, but only found 0 bytes eligible to free.
Warning  Evicted                 pod/postgres-...   The node had condition: [DiskPressure].
Warning  Evicted                 pod/nats-0          The node had condition: [DiskPressure].
Warning  Evicted                 pod/fortuna-agent-...  The node had condition: [DiskPressure].
Normal  NodeHasNoDiskPressure   23s  kubelet  Node k8s-worker01 status is now: NodeHasNoDiskPressure
```
Sau khi dọn disk / GC, node chuyển lại NoDiskPressure; pod mới (postgres, nats) được schedule lại và có thể bị evict lại nếu disk đầy trở lại.

**Nats vừa evict lại:**
```
Warning  Evicted  pod/nats-0  The node was low on resource: ephemeral-storage. Threshold quantity: 1569603441, available: 1381220Ki.
```

**So sánh node:**
| Node         | Allocatable ephemeral-storage | DiskPressure (lịch sử) |
|-------------|--------------------------------|--------------------------|
| k8s-master  | ~45 GB                         | Không                    |
| k8s-worker01| ~9.4 GB (10218772Ki)           | Có (EvictionThresholdMet, FreeDiskSpaceFailed) |

Worker01 có ít disk hơn nhiều; khi đầy (image, layer, log), kubelet không thu hồi đủ qua image GC (“only found 0 bytes eligible to free”) nên evict pod.

### 1.3 Tại sao đầy disk trên worker01?

- **Image và layer containerd:** Postgres, NATS, fortuna-agent, Core (nếu từng chạy ở đây) kéo image; nhiều replica cũ → nhiều layer/image.
- **OSV sync / CVE load:** Chạy trên **host** (master), dữ liệu `cve-data/all` nằm trên master, **không** trực tiếp chiếm disk worker01. Nhưng nếu CVE loader Job hoặc job khác chạy trên worker01 thì có thể góp phần.
- **Log, container writable layer:** Pod chạy trên worker01 ghi log và FS → tăng dùng ephemeral-storage.
- **Kubelet image GC:** Khi disk gần đầy, kubelet cố thu hồi bằng cách xóa image không dùng. Trên worker vẫn “Failed to free ... only found 0 bytes eligible” → có thể image đang được tham chiếu hoặc cấu hình eviction/disk quá chặt.

### 1.4 Khuyến nghị

1. **Tăng disk hoặc giảm tải trên worker01:** Tăng dung lượng volume ephemeral-storage, hoặc chuyển workload nặng (postgres, nats) sang node có nhiều disk hơn (ví dụ master nếu cho phép).
2. **Ưu tiên schedule infra lên master (nếu chấp nhận):** Postgres/NATS có thể chạy trên master (có nhiều disk) bằng nodeSelector/tolerations; Core đã chạy trên master.
3. **Dọn disk định kỳ trên worker01:** Xóa image không dùng (`ctr -n k8s.io images prune`), dọn log, xóa pod Evicted/Completed (script `scripts/clean/clean-evicted-completed-pods.sh`).
4. **Điều chỉnh eviction:** Cấu hình kubelet `imagefs.available` / `nodefs.available` nếu cần (ví dụ nới ngưỡng) sau khi đã tăng disk hoặc giảm workload.

---

## 2. Luồng Deploy hiện tại

Script chính: **`scripts/deploy/deploy-fortuna-robust.sh`** (chỉ deploy, không clean/rebuild).

| Bước | Nội dung |
|------|----------|
| 1 | Pre-deployment checks (`pre-deployment-checks.sh`) |
| 2 | Tạo namespace `fortuna` (nếu chưa có) |
| 3 | Cleanup: xóa deployment/core cũ (theo label/name), xóa service, xóa pod orphan; sleep 8s |
| 4 | Infra: Nếu chưa có Postgres → apply `deploy/infrastructure/postgresql-with-age.yaml`, đợi ready. Nếu chưa có NATS → apply `deploy/infrastructure/nats.yaml`, đợi ready. |
| 4b | Multi-node: đảm bảo pod network (Flannel VXLAN) |
| 5 | Kiểm tra image containerd (fortuna-core, …) |
| 6 | Kiểm tra DNS hoặc fallback IP cho Postgres/Core |
| 7 | Apply RBAC (`deploy/fortuna-rbac.yaml`) |
| 8 | Deploy Core: `deploy/fortuna-core-deployment.yaml`; cấu hình `DATABASE_URL` nếu dùng IP; đợi deployment available. |
| 8a | Kiểm tra Core service có endpoints. |
| 8b | Kiểm tra DNS cho `fortuna-core.$NAMESPACE.svc.cluster.local`. |
| **8c** | **Load CVE/OSV:** Nếu `AUTO_LOAD_CVE_ON_DEPLOY=true` và có `scripts/utils/load-cve-data.sh`: gọi với `AUTO_SYNC_CVE_SOURCE`, `RESET_CVE_TABLES`, `CVE_DATA_DIR`. |
| 9 | Deploy Agent: `deploy/fortuna-agent-daemonset.yaml`; cấu hình `CORE_GRPC_ENDPOINT`/TLS nếu dùng IP; đợi rollout. |
| 9b | Deploy Dashboard (ConfigMap + deployment). |
| 10 | Rollout restart Core, Dashboard, Agent để dùng image mới; đợi Core available; sleep 15s (migrations + sync). |
| 11 | Verification: in trạng thái Core, Agent, Dashboard, endpoints. |

**Lưu ý:** Core chạy migration khi **khởi động** (xem mục 3). Step 8c chạy **sau** khi Core ready, nên migration đã chạy xong trước khi load CVE.

---

## 3. Luồng Migration hiện tại

- **Điểm gọi:** Core process khởi động trong `core/cmd/main.go` → `storage.Migrate(tempDB)` → `migrations.RunMigrations(db)` (và sau đó `RunPostMigrations`).
- **Thứ tự:** Trong `core/migrations/migrations.go`, `RunMigrations` chạy lần lượt:
  - 001–003: schema cơ bản (clusters, nodes, users, audit_logs)
  - 008–009: deployments, replicasets
  - 010–011: implementation guide, insights soft delete
  - 012–016: risk_scores, policy_templates/instances/violations (MVP2)
  - 018–022: risk V2, CVE tables, SBOM tables, cve_matches, …
  - 023–065: indexes, cleanup, capability_metadata, clusters region/endpoint, pods cleanup index, advisory columns, users deleted_at, …
- **CVE/SBOM:** Bảng CVE (ví dụ `cves`, `package_vulnerabilities`, `cve_matches`) được tạo bởi migration (019, 022, 032, …). Load OSV (step 8c) **phụ thuộc** vào các bảng này đã tồn tại; `load-cve-data.sh` kiểm tra bảng `cves` trước khi tạo Job load.

**Tóm tắt:** Migration chạy **một lần khi Core start**; không có bước deploy riêng “chạy migration” ngoài việc deploy Core và đợi Core ready.

---

## 4. Luồng Load OSV hiện tại

### 4.1 Sync nguồn OSV (optional, trước khi load)

**Script:** `scripts/utils/sync-package-vulnerability-source.sh`

- **Điều kiện:** Chạy trên **host** (thư mục `CVE_DATA_DIR`, mặc định `.../cve-data`). Cần đủ disk: `MIN_FREE_GB` (mặc định 16), `MIN_FREE_INODES` (200000). Có thể bỏ qua nếu đã sync gần đây (marker `.osv-sync-marker`, `SYNC_HOURS`).
- **Luồng:**  
  1. Kiểm tra free space/inode.  
  2. Tải `all.zip` từ OSV bulk URL.  
  3. Giải nén vào thư mục tạm.  
  4. Chuẩn bị staging, swap thư mục vào `cve-data/all` (atomic).  
  5. Ghi marker.
- **Vị trí dữ liệu:** `cve-data/all` nằm trên **node chạy script** (thường là master). Worker01 **không** lưu bản copy này trừ khi script chạy từ worker01.

### 4.2 Load CVE vào DB (Step 8c)

**Script:** `scripts/utils/load-cve-data.sh`

- **Điều kiện:** `AUTO_LOAD_CVE_ON_DEPLOY=true`, script tồn tại và executable. Có thể bật/tắt sync bằng `AUTO_SYNC_CVE_SOURCE`, xóa dữ liệu CVE cũ bằng `RESET_CVE_TABLES`.
- **Luồng:**  
  1. **Sync (optional):** Gọi `sync-package-vulnerability-source.sh` nếu `AUTO_SYNC_CVE_SOURCE=true`.  
  2. **Kiểm tra dữ liệu:** Có thư mục `cve-data/all` và có file `*.json`.  
  3. **Kiểm tra Postgres + bảng CVE:** Cần pod Postgres running; kiểm tra bảng `cves` tồn tại (sau khi Core đã chạy migration).  
  4. **Reset (optional):** Nếu `RESET_CVE_TABLES=true` → truncate `cve_matches`, `package_vulnerabilities`, `cves`.  
  5. **Tạo Job `cve-loader`:**  
     - Image: fortuna-core (từ deployment hoặc env).  
     - Command: `/app/cve-loader`.  
     - **nodeSelector: `kubernetes.io/hostname: k8s-master`** + tolerations control-plane → Job chạy trên **master**.  
     - Mount hostPath `cve-data` (chứa `all/`) vào trong pod để đọc JSON.  
  Job đọc file từ host và ghi vào DB (kết nối Postgres qua service).

**Tóm tắt:** Sync chạy trên host (thường master); load chạy trong cluster bằng Job trên **master**, đọc dữ liệu từ host và ghi vào Postgres. Worker01 **không** phải nơi lưu `cve-data/all` hay chạy CVE loader mặc định; eviction trên worker01 do **disk trên chính worker01** (image, log, postgres/nats/agent khi chạy ở đó), không phải do OSV sync/load trực tiếp.

---

## 5. Sơ đồ tóm tắt

```
[Host/Master]
  sync-package-vulnerability-source.sh  →  cve-data/all (JSON)
                    ↓
  deploy-fortuna-robust.sh
    → namespace, cleanup
    → Postgres, NATS (có thể schedule worker01 → dễ evict nếu disk đầy)
    → Core (thường master) → main.go → RunMigrations() → bảng CVE tồn tại
    → Step 8c: load-cve-data.sh
         → sync (optional)   [trên host]
         → check CVE tables  [Postgres pod]
         → truncate (optional)
         → Job cve-loader   [schedule trên k8s-master, mount cve-data]
    → Agent (DaemonSet: master + worker01)
    → Dashboard

[Worker01]
  DiskPressure → evict: postgres, nats, fortuna-agent, (core replicas cũ)
  Nguyên nhân: ít disk, image/log/layer đầy, image GC không thu đủ.
```

---

## 6. Tài liệu liên quan

- `docs/POSTGRES_CONNECTION_REFUSED_FIX.md` – Postgres/eviction do disk.
- `scripts/clean/clean-evicted-completed-pods.sh` – Dọn pod Evicted/Completed.
- `scripts/deploy/deploy-fortuna-robust.sh` – Luồng deploy đầy đủ.
- `scripts/utils/sync-package-vulnerability-source.sh` – Sync OSV.
- `scripts/utils/load-cve-data.sh` – Load CVE vào DB và cấu hình Job.
