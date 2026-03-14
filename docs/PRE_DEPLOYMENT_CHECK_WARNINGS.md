# Pre-deployment check warnings – lý do và xử lý

Script `scripts/deploy/pre-deployment-checks.sh` có thể báo 2 cảnh báo (⚠️). Dưới đây là nguyên nhân và cách xử lý (nếu cần).

---

## 1. High CoreDNS restart count (ví dụ: 12)

**Thông báo:** `⚠️ High CoreDNS restart count: 12 (may indicate issues)`

### Cách script tính

- Lấy tất cả pod CoreDNS (`kube-system`, label `k8s-app=kube-dns`).
- Cộng cột **RESTARTS** (tổng restart của mọi pod).
- Nếu tổng **> 10** → báo cảnh báo.

Ví dụ: 2 pod CoreDNS, mỗi pod 6 lần restart → tổng 12 → cảnh báo.

### Nguyên nhân thường gặp

| Nguyên nhân | Giải thích |
|-------------|------------|
| **Node resource pressure** | Node thiếu memory/CPU → CoreDNS bị OOMKilled hoặc evicted → restart. |
| **ConfigMap CoreDNS thay đổi** | Chỉnh `kube-system/coredns` ConfigMap rồi rollout/restart → số lần restart tăng. |
| **Node reboot / cluster khởi động lại** | Nhiều lần tắt/bật node hoặc cluster → pod restart nhiều lần. |
| **Lỗi CoreDNS (loop, crash)** | CoreDNS crash hoặc DNS loop → kubelet restart container. |
| **Upgrade / cài addon** | Chạy `kubeadm upgrade`, `ensure-cluster-addons.sh`, hoặc script sửa DNS (vd. fix-dns-config.sh) → restart CoreDNS. |

### Khi nào có thể bỏ qua

- Cluster mới cài, đã nhiều lần chỉnh addon/DNS hoặc reboot → số restart tích lũy cao là bình thường.
- **DNS vẫn hoạt động** (như output của bạn: CoreDNS service IP ✅, test connectivity ✅, test resolution postgres/nats ✅) → thường chỉ cần theo dõi, chưa cần sửa.

### Khi nào nên xử lý

- Restart tăng **liên tục** sau mỗi lần chạy check (vd. 12 → 15 → 18).
- CoreDNS không Ready, hoặc test DNS resolution fail.
- Pod CoreDNS CrashLoopBackOff / OOMKilled.

**Cách kiểm tra thêm:**

```bash
# Restart từng pod CoreDNS
kubectl get pods -n kube-system -l k8s-app=kube-dns -o wide

# Event và lý do restart (OOMKilled, ExitCode, ...)
kubectl describe pods -n kube-system -l k8s-app=kube-dns

# Log CoreDNS
kubectl logs -n kube-system -l k8s-app=kube-dns --tail=50
```

**Gợi ý:** Nếu muốn giảm cảnh báo khi cluster ổn định nhưng đã tích lũy nhiều restart, có thể tăng ngưỡng trong script (vd. từ 10 lên 15 hoặc 20). Hiện tại ngưỡng nằm tại `scripts/deploy/pre-deployment-checks.sh` (dòng ~117: `if [ "$RESTART_COUNT" -gt 10 ]`).

---

## 2. No worker nodes labeled

**Thông báo:** `⚠️ No worker nodes labeled (Core will use nodeSelector workaround)`

### Cách script kiểm tra

- Đếm node có label: `node-role.kubernetes.io/worker`.
- Nếu **= 0** → báo cảnh báo.

### Vì sao thường gặp

- **kubeadm** mặc định chỉ gắn nhãn **control-plane** (`node-role.kubernetes.io/control-plane`) cho node init, **không** tự gắn `node-role.kubernetes.io/worker` cho node nào.
- Cluster 1 node (all-in-one): thường chỉ có control-plane, không có node “worker” theo chuẩn kubeadm.
- Cluster nhiều node: node “worker” có thể chưa được ai gắn label `node-role.kubernetes.io/worker`.

→ Rất nhiều cluster **bình thường** nhưng vẫn có cảnh báo này.

### “Core will use nodeSelector workaround” nghĩa là gì?

- **Core** (Fortuna) **không** schedule lên node có label worker.
- Core dùng **nodeSelector**: `node-role.kubernetes.io/control-plane` (xem `deploy/fortuna-core-deployment.yaml`).
- Nghĩa là Core chạy trên **control-plane**; việc **không** có node nào có label `worker` **không** ảnh hưởng đến việc Core chạy.
- “Workaround” ở đây là: dù không có worker label, deploy vẫn thiết kế để chạy đúng (Core trên control-plane, Agent có thể chạy mọi node).

### Khi nào cần gắn label worker

- Chỉ khi **ứng dụng khác** hoặc **policy** của bạn yêu cầu node có `node-role.kubernetes.io/worker` (vd. chỉ schedule workload trên worker).
- Với Fortuna Core/Agent/Dashboard: **không bắt buộc** phải có worker label để chạy.

**Nếu muốn gắn label cho node (ví dụ node tên `k8s-worker`):**

```bash
kubectl label node k8s-worker node-role.kubernetes.io/worker=
```

**Tóm lại:** Cảnh báo này chủ yếu mang tính thông tin (cluster không dùng chuẩn “worker” của kubeadm). Nếu Core và DNS đều OK như output của bạn, có thể bỏ qua hoặc gắn label worker nếu bạn cần cho workload khác.
