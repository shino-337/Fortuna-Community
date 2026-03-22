# Pod network multi-node (Worker ↔ Master)

## Vấn đề

Trên cluster **multi-node** (ví dụ master + worker), pod trên **worker** có thể không kết nối được tới pod trên **master** (Core, CoreDNS). Hậu quả:

- Agent chạy trên worker không gọi được Core (gRPC 9090, HTTP 8080) → **SBOM và sync không lên Core/dashboard**.
- DNS từ worker pod: `nslookup fortuna-core.fortuna.svc.cluster.local` → timeout (không resolve được).
- Nguyên nhân thường gặp: **Flannel VXLAN** (hoặc CNI) chưa ổn định sau khi cluster/worker join → route hoặc tunnel giữa hai node chưa đúng.

## Cách xử lý triệt để

### 1. Chạy script sửa Flannel (khuyến nghị)

```bash
./scripts/deploy/fix-flannel-vxlan.sh
```

Script sẽ:

- Kiểm tra Flannel ConfigMap (Network 10.244.0.0/16, Backend vxlan).
- Kiểm tra PodCIDR trên từng node.
- **Restart DaemonSet Flannel** để VXLAN và route được tạo lại.
- (Nếu có Agent trên worker) Test kết nối từ Agent pod → Core service.

Sau khi chạy xong, đợi 30–60 giây rồi kiểm tra Agent logs (Heartbeat, SBOM).

### 2. Tích hợp vào deploy

Khi deploy bằng `./scripts/pipeline/full-clean-database-rebuild-deploy.sh` (hoặc `./scripts/deploy/deploy-fortuna-robust.sh`), nếu cluster có **hơn 1 node**, script sẽ **tự chạy** `scripts/deploy/fix-flannel-vxlan.sh` (Step 4b) sau bước deploy infrastructure (PostgreSQL, NATS) và trước khi deploy Core/Agent. Nhờ đó deploy mới trên multi-node sẽ ít gặp lỗi “Agent trên worker không kết nối được Core”.

### 3. Kiểm tra thủ công

- **DNS từ worker:**  
  Pod trên worker (ví dụ `nodeSelector: k8s-worker01`):

  ```bash
  nslookup fortuna-core.fortuna.svc.cluster.local
  ```

  Phải resolve được về ClusterIP của service `fortuna-core`.

- **HTTP từ worker pod tới Core:**  
  Thay `CORE_POD_IP` bằng pod IP của fortuna-core:

  ```bash
  wget -q -O- --timeout=5 http://<CORE_POD_IP>:8080/health
  ```

  Phải trả về JSON `{"status":"healthy",...}`.

- **Route trên từng node (nếu có quyền host):**  
  Trên master: `ip route | grep 10.244` → có route tới subnet worker (ví dụ `10.244.1.0/24 via ... dev flannel.1`).  
  Trên worker: có route tới subnet master (ví dụ `10.244.0.0/24 via ... dev flannel.1`).

## Tóm tắt

| Bước | Mục đích |
|------|----------|
| Chạy `fix-flannel-vxlan.sh` | Sửa pod network worker ↔ master (Flannel VXLAN) |
| Deploy mới (multi-node) | Script deploy đã gọi sẵn fix Flannel (Step 4b) |
| Kiểm tra DNS / wget từ worker | Xác nhận Agent có thể resolve và gọi Core |

Làm xong các bước trên thì pod network / DNS giữa worker và master được xử lý triệt để, deploy mới không còn lỗi tương tự (Agent trên worker không reach Core / SBOM không sync).
