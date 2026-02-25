# Sizing disk và khắc phục triệt để Eviction (DiskPressure)

## 1. Tóm tắt nguyên nhân và đã làm

- **Nguyên nhân:** Node **k8s-worker01** bị **DiskPressure** (ephemeral-storage dưới ngưỡng) → kubelet evict toàn bộ pod trên node (postgres, nats, fortuna-agent).
- **Đã làm:**
  1. Xóa toàn bộ pod Evicted/Completed (script `scripts/clean/clean-evicted-completed-pods.sh` hoặc lệnh trong doc).
  2. Sửa manifest: **Postgres** và **NATS** schedule **chỉ trên master** (`nodeSelector: kubernetes.io/hostname: k8s-master` + tolerations control-plane) để không chạy trên worker01 → giảm tải disk worker01.
  3. Worker01 chỉ còn: DaemonSet fortuna-agent + system pods (flannel, coredns, kube-proxy) → nhu cầu disk thấp hơn nhiều.

---

## 2. Chi tiết lý do Eviction

### 2.1 Sự kiện và thông báo

- `EvictionThresholdMet` – kubelet bắt đầu reclaim ephemeral-storage.
- `FreeDiskSpaceFailed` – “Attempted to free 717293977 bytes, but only found **0 bytes eligible** to free” (không xóa được image/layer nào).
- `Evicted` – “The node had condition: [DiskPressure]” hoặc “The node was low on resource: **ephemeral-storage**. Threshold quantity: **1569603441**, available: 1381092Ki”.

### 2.2 Ngưỡng eviction (kubelet mặc định)

- **nodefs.available** < 10% → đánh dấu DiskPressure.
- **imagefs.available** < 15% → evict theo thứ tự ưu tiên (BestEffort → Burstable → Guaranteed).

Số liệu đo được trên **k8s-worker01**:

- **Allocatable ephemeral-storage:** 9 417 620 260 bytes ≈ **8,77 GiB**.
- **Threshold quantity:** 1 569 603 441 bytes ≈ **1,46 GiB** (tương đương ~17% allocatable → gần với ngưỡng imagefs 15%).
- Khi **available** < ~1,46 GiB → eviction. Kubelet cố thu hồi ~700 MiB bằng image GC nhưng không thu được (“0 bytes eligible”) → tiếp tục evict.

### 2.3 Tại sao disk đầy trên worker01

- **Capacity nhỏ:** worker01 ~10 GiB ephemeral-storage (capacity), ~9,4 GiB allocatable; master ~48 GiB.
- **Workload nặng trên worker01:** Postgres (image + layer + log), NATS x3 (image + jetstream 10GB PVC mỗi pod), fortuna-agent (image + log) → tổng image + writable layer + log vượt quá dung lượng an toàn.
- **Image GC không đủ:** Các image đang được pod sử dụng hoặc được tham chiếu nên kubelet không xóa được → “0 bytes eligible to free”.

---

## 3. Yêu cầu sizing disk (cụ thể)

### 3.1 Công thức tham chiếu

- **Allocatable** = capacity − kube-reserved − system-reserved − eviction-threshold.
- Eviction khi **free** < 10% (nodefs) hoặc 15% (imagefs). Cần đủ dung lượng cho:
  - Image + layer (containerd)
  - Log (container + kubelet)
  - Writable layer container
  - Margin an toàn ≥ 15–20% free sau khi chạy ổn định

### 3.2 Khuyến nghị theo vai trò node

| Vai trò node | Ephemeral-storage (allocatable) | Ghi chú |
|--------------|---------------------------------|--------|
| **Master (control-plane + Core + Dashboard + Agent + Postgres + NATS)** | **≥ 40 GiB** | Hiện tại ~45 GiB là đủ. |
| **Worker chỉ chạy Agent + system (Flannel, CoreDNS, kube-proxy)** | **≥ 15 GiB** | Đủ cho image agent + vài image hệ thống + log + margin. |
| **Worker chạy thêm Postgres + NATS (hoặc workload stateful khác)** | **≥ 40 GiB** | Tránh DiskPressure khi thêm image + log + layer. |

### 3.3 Áp dụng cho cluster hiện tại

- **k8s-master:** Giữ **≥ 40 GiB** (hoặc tương đương 48 GiB như hiện tại). Đủ cho Core, Dashboard, Agent, Postgres, NATS và system.
- **k8s-worker01:** 
  - **Nếu đã chuyển Postgres + NATS sang master (theo mục 4):** **≥ 15 GiB** allocatable (ví dụ capacity 20–25 GiB tùy reserved).
  - **Nếu vẫn chạy Postgres + NATS trên worker01:** **≥ 40 GiB** allocatable (ví dụ capacity 50 GiB). Hiện tại ~10 GiB là **không đủ** và là nguyên nhân eviction.

### 3.4 Con số tối thiểu an toàn

- **Worker chỉ Agent + system:** capacity **20 GiB** trở lên (sau reserved → allocatable ~15 GiB).
- **Worker có stateful (Postgres/NATS):** capacity **50 GiB** trở lên (allocatable ~40 GiB).
- **Master (toàn bộ Fortuna + infra):** capacity **50 GiB** trở lên (allocatable ~40 GiB).

---

## 4. Khắc phục triệt để đã áp dụng

### 4.1 Manifest đã sửa

- **`deploy/infrastructure/postgresql-with-age.yaml`**  
  - Thêm `nodeSelector: kubernetes.io/hostname: k8s-master` và `tolerations` cho control-plane.  
  - Postgres chỉ schedule trên master.

- **`deploy/infrastructure/nats.yaml`**  
  - Thêm cùng `nodeSelector` và `tolerations`.  
  - NATS chỉ schedule trên master.

Deploy mới (fresh install) sẽ đặt Postgres và NATS trên master, worker01 chỉ còn Agent + system → giảm triệt để eviction do disk trên worker.

### 4.2 Lưu ý cluster hiện tại

- Sau khi apply manifest mới, **Postgres** có thể vẫn đang chạy trên worker01 (pod cũ chưa bị thay). **Không xóa pod Postgres** cho đến khi đã backup và sẵn sàng migrate (Cách 2) hoặc đã tăng disk worker01 (Cách 1).
- Nếu pod Postgres bị xóa hoặc restart, pod mới sẽ schedule trên **master** (đã có nodeSelector). Nếu PVC `postgres-pvc` đã bind PV trên worker01 (RWO), pod trên master **sẽ không mount được** và ở trạng thái Pending. Khi đó cần migrate theo Cách 2 hoặc tạm revert nodeSelector cho postgres và tăng disk worker01.
- **NATS** sau khi apply: pod mới (khi replace) cũng sẽ schedule trên master; PVC cũ trên worker01 không dùng được trên master. Để chuyển NATS sang master dùng Cách 3 (scale 0 → xóa PVC → scale 3).

### 4.3 Cluster đã có sẵn (Postgres/NATS đang trên worker01)

Nếu Postgres/NATS đang chạy trên worker01 với PVC đã bind tại đó:

**Cách 1 – Không mất dữ liệu (khuyến nghị):** **Tăng disk worker01** theo bảng sizing (≥ 40 GiB allocatable nếu giữ Postgres+NATS; hoặc ≥ 15 GiB nếu sau đó chuyển hết sang master). Sau khi tăng disk, dọn pod Evicted và theo dõi DiskPressure.

**Cách 2 – Chuyển Postgres sang master (có mất dữ liệu DB nếu không backup):**

1. Backup DB (pg_dump / snapshot).
2. Scale deployment postgres về 0 hoặc xóa deployment.
3. Xóa PVC `postgres-pvc` (và PV nếu cần).
4. Apply lại `deploy/infrastructure/postgresql-with-age.yaml` (đã có nodeSelector master).
5. Đợi Postgres ready trên master, restore backup.

**Cách 3 – Chuyển NATS sang master (mất dữ liệu JetStream):**

1. Scale StatefulSet nats về 0: `kubectl scale statefulset nats -n fortuna --replicas=0`.
2. Xóa PVC: `kubectl delete pvc -n fortuna -l app=nats` hoặc xóa từng `data-nats-0`, `data-nats-1`, `data-nats-2`.
3. Scale lại 3: `kubectl scale statefulset nats -n fortuna --replicas=3`.
4. Pod mới sẽ schedule trên master (đã có nodeSelector), PVC mới tạo trên master (local-path).

---

## 5. Dọn pod Evicted/Completed định kỳ

- Chạy script:  
  `bash scripts/clean/clean-evicted-completed-pods.sh`
- Hoặc một lần bằng tay:  
  Liệt kê pod Evicted/Completed/Failed rồi `kubectl delete pod ... --grace-period=0 --force` (có thể dùng vòng lặp từ output `kubectl get pods -A` hoặc jq như trong script).

Sau khi đã schedule Postgres + NATS trên master và (nếu cần) tăng disk worker01, số pod evicted sẽ không còn tích lũy do DiskPressure.

---

## 6. Tài liệu liên quan

- `docs/EVICTION_ROOT_CAUSE_AND_DEPLOY_FLOW.md` – Nguyên nhân eviction và luồng deploy/migration/OSV.
- `docs/POSTGRES_CONNECTION_REFUSED_FIX.md` – Khi Postgres evicted → connection refused.
- `scripts/clean/clean-evicted-completed-pods.sh` – Dọn pod Evicted/Completed/Failed.
