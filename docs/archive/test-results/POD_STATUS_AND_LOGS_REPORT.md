# Báo cáo trạng thái Pod và Log chi tiết

**Thời gian kiểm tra**: 2026-02-22  
**Cách lấy log**: `kubectl describe`, `kubectl get -o json`, `crictl logs` (trên node vì `kubectl logs` trả về Forbidden).

---

## 1. Tổng quan Pod (theo namespace)

| Namespace           | Pod                                      | READY | STATUS             | RESTARTS | NODE        | Ghi chú                    |
|--------------------|-------------------------------------------|-------|--------------------|----------|-------------|----------------------------|
| fortuna            | postgres-5484f7745d-gsr4f                  | 0/1   | Pending            | 0        | -           | Cũ, chưa schedule          |
| fortuna            | postgres-5898b788f6-9fqf2                 | 0/1   | Pending            | 0        | -           | Cần PVC bind               |
| kube-flannel       | kube-flannel-ds-8cqsz                     | 0/1   | CrashLoopBackOff   | 228      | k8s-master  | exitCode 1                 |
| kube-flannel       | kube-flannel-ds-wb8f4                     | 0/1   | CrashLoopBackOff   | 228      | k8s-worker01| exitCode 1                 |
| kube-system        | etcd-k8s-master                           | 1/1   | Running            | 22       | k8s-master  | OK                         |
| kube-system        | kube-apiserver-k8s-master                  | 1/1   | Running            | 21       | k8s-master  | OK                         |
| kube-system        | kube-controller-manager-k8s-master        | 1/1   | Running            | 28       | k8s-master  | OK                         |
| kube-system        | kube-scheduler-k8s-master                 | 1/1   | Running            | 25       | k8s-master  | OK                         |
| local-path-storage | local-path-provisioner-6d866c86c5-7vxt8   | 0/1   | CrashLoopBackOff   | 217      | k8s-master  | exitCode 1                 |

**Thiếu**: Không có CoreDNS, không có kube-proxy.

---

## 2. Log và sự kiện chi tiết

### 2.1 Fortuna – Postgres (Pending)

**Events** (pod `postgres-5898b788f6-9fqf2`):

```
Warning  FailedScheduling  (x240 over 19h)  0/2 nodes are available:
  1 node(s) didn't find available persistent volumes to bind,
  1 node(s) didn't match Pod's node affinity/selector.
```

- **Nguyên nhân**: PVC `postgres-pvc` (local-path) chưa bind vì local-path-provisioner không chạy được; đồng thời pod có `nodeSelector: node-role.kubernetes.io/control-plane` nên chỉ schedule được lên master.

### 2.2 Kube-Flannel (CrashLoopBackOff)

**Log container `kube-flannel`** (lấy bằng `crictl logs` trên master):

```
I0222 12:34:58 ... CLI flags config: { ... kubeSubnetMgr:true ... subnetFile:/run/flannel/subnet.env ... }
W0222 12:34:58 ... Neither --kubeconfig nor --master was specified. Using the inClusterConfig.
E0222 12:35:19 ... Failed to create SubnetManager: error retrieving pod spec for 'kube-flannel/kube-flannel-ds-8cqsz':
  Get "https://10.96.0.1:443/api/v1/namespaces/kube-flannel/pods/kube-flannel-ds-8cqsz":
  dial tcp 10.96.0.1:443: connect: connection refused
```

- **Nguyên nhân**: Flannel dùng in-cluster config, gọi API server qua ClusterIP **10.96.0.1:443** (service `kubernetes`) → **connection refused**. ClusterIP do kube-proxy phục vụ; cluster **không có kube-proxy** nên 10.96.0.1 không hoạt động.

### 2.3 Local-path-provisioner (CrashLoopBackOff)

**Log container** (lấy bằng `crictl logs`):

```
level=fatal msg="Error starting daemon: invalid empty flag helper-pod-file and it also does not exist at ConfigMap local-path-storage/local-path-config with err:
  Get \"https://10.96.0.1:443/api/v1/namespaces/local-path-storage/configmaps/local-path-config\":
  dial tcp 10.96.0.1:443: i/o timeout"
```

- **Nguyên nhân**: Provisioner cần đọc ConfigMap từ API server qua **10.96.0.1:443** → **i/o timeout**. Cùng gốc: không có kube-proxy, ClusterIP không hoạt động.

### 2.4 Control-plane (etcd, apiserver, controller-manager, scheduler)

- **Trạng thái**: Đều **Running**, không lỗi.
- **etcd**: Log bình thường (compaction, snapshot). Không có lỗi kết nối.

---

## 3. Nguyên nhân gốc

1. **Không có kube-proxy**  
   - Không có DaemonSet/pod `kube-proxy` trong `kube-system`.  
   - Service ClusterIP (ví dụ `kubernetes` 10.96.0.1) không được implement → mọi pod gọi API qua 10.96.0.1 đều fail (connection refused / i/o timeout).

2. **Hệ quả dây chuyền**  
   - Flannel và local-path-provisioner cần gọi API server → crash.  
   - PVC không được provision → Postgres không schedule được.

3. **Không có CoreDNS**  
   - Cluster cũng không có CoreDNS; nếu có kube-proxy thì vẫn cần CoreDNS cho DNS trong cluster.

---

## 4. Khuyến nghị

1. **Cài kube-proxy** (phù hợp với cách cài cluster, ví dụ kubeadm):
   - Ví dụ manifest kube-proxy DaemonSet (kubeadm): thường nằm trong `/etc/kubernetes/manifests` hoặc apply từ kubeadm phase addon.
   - Hoặc dùng:  
     `kubectl apply -f https://raw.githubusercontent.com/coreos/flannel/master/Documentation/kube-proxy.yml`  
     (nếu dùng đúng phiên bản tương thích với cluster).

2. **Sau khi kube-proxy Running**:
   - Flannel và local-path-provisioner có thể gọi được 10.96.0.1 → không còn crash vì lỗi API.
   - PVC local-path có thể được provision → Postgres có thể schedule và chạy.

3. **CoreDNS** (nếu chưa có):
   - Cài CoreDNS để cluster có DNS nội bộ (ví dụ `svc.cluster.local`).

4. **Kiểm tra lại**:
   - `kubectl get pods -n kube-system`
   - `kubectl get pods -A`
   - Sau khi Flannel và provisioner Running: `kubectl get pvc -n fortuna`, `kubectl get pods -n fortuna`.

---

## 5. Lệnh đã dùng để lấy thông tin

```bash
# Liệt kê pod
kubectl get pods -A -o wide

# Mô tả pod (events, volumes, node)
kubectl describe pod -n <namespace> <pod-name>

# Trạng thái container (exitCode, lastState)
kubectl get pod -n <namespace> <pod-name> -o jsonpath='{.status.containerStatuses[*]}'

# Log từ containerd trên node (khi kubectl logs bị Forbidden)
crictl ps -a
crictl logs <container-id>
```

---

*Báo cáo được tạo tự động từ kết quả kiểm tra trên cluster.*
