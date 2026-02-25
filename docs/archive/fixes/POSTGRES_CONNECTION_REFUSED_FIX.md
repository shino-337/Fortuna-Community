# Lỗi: connection refused tới Postgres

**Lỗi:** `failed to connect to host=postgres.fortuna.svc.cluster.local ... dial tcp 10.99.248.179:5432: connect: connection refused`

## Nguyên nhân

1. **Node k8s-worker01 có DiskPressure** — kubelet đã đặt taint `node.kubernetes.io/disk-pressure:NoSchedule` và **evict** tất cả pod Postgres (và các pod khác) để giải phóng dung lượng.
2. **Không có pod Postgres nào đang Running** — tất cả pod Postgres trong namespace `fortuna` đang ở trạng thái **Evicted** hoặc **Completed**, nên Service `postgres` không còn endpoint → kết nối bị **connection refused**.
3. **Pod mới không schedule được** — pod Postgres mới (Pending) không thể schedule vì:
   - **k8s-worker01:** có taint `disk-pressure` (cần ~488MB trở lên để kubelet bỏ pressure).
   - **k8s-master:** có taint `node-role.kubernetes.io/control-plane`; deployment Postgres hiện tại không có toleration nên không chạy trên master.

## Cách xử lý

### Bước 1: Giải phóng dung lượng trên k8s-worker01

**Cần chạy trên chính node k8s-worker01** (SSH hoặc console):

```bash
# Xóa image không dùng (containerd)
sudo crictl rmi --prune
# hoặc nếu dùng nerdctl
sudo nerdctl -n k8s.io system prune -f

# Xóa container đã dừng
sudo crictl rm -f $(sudo crictl ps -aq 2>/dev/null) 2>/dev/null || true

# Kiểm tra dung lượng
df -h /var/lib/containerd
df -h /var/lib/kubelet
```

Sau khi đủ dung lượng, kubelet sẽ tự bỏ taint **DiskPressure** (vài phút). Kiểm tra:

```bash
kubectl describe node k8s-worker01 | grep -A2 Taints
kubectl describe node k8s-worker01 | grep DiskPressure
```

### Bước 2: Xóa pod evicted/completed để Deployment tạo pod mới

Sau khi node **không còn** DiskPressure (Taints rỗng hoặc không còn `disk-pressure`):

```bash
# Xóa các pod Postgres đã Evicted/Completed
kubectl delete pods -n fortuna -l app=postgres --field-selector=status.phase!=Running

# Hoặc xóa từng nhóm
kubectl get pods -n fortuna -l app=postgres --no-headers | awk '$3!="Running"{print $1}' | xargs -r kubectl delete pod -n fortuna
```

Deployment sẽ tạo pod Postgres mới. Pod mới sẽ bind lại **PVC postgres-pvc** (dữ liệu vẫn trên volume).

### Bước 3: Kiểm tra Postgres và Core

```bash
kubectl get pods -n fortuna -l app=postgres
kubectl get endpoints -n fortuna postgres
# Khi pod Running và Ready, endpoints sẽ có IP:5432

# Restart Core để nó kết nối lại DB
kubectl rollout restart deployment/fortuna-core -n fortuna
kubectl rollout status deployment/fortuna-core -n fortuna
```

## Nếu không thể giải phóng đủ dung lượng trên worker01

- **Tăng dung lượng ổ đĩa** cho node worker01 (resize disk, thêm volume).
- **Hoặc** chuyển Postgres sang chạy trên master: chỉnh deployment Postgres dùng `nodeSelector: node-role.kubernetes.io/control-plane: ""` và toleration cho control-plane; **lưu ý** PVC `postgres-pvc` (local-path) đang bound ở worker01, pod trên master sẽ không mount được PVC đó trừ khi dùng storage khác (ví dụ NFS hoặc PVC mới trên master).

## Tóm tắt

| Việc | Lệnh / Ghi chú |
|------|----------------|
| Nguyên nhân | Node worker01 DiskPressure → Postgres pods evicted → không có endpoint |
| Sửa nhanh | Giải phóng disk trên worker01 → xóa pod evicted/completed → đợi pod Postgres mới Running → restart Core nếu cần |
