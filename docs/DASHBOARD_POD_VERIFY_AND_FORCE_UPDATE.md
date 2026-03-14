# Kiểm tra Dashboard và Agent trong Pod – ép cập nhật bản mới

## 1. Kết quả exec vào pod (khi kiểm tra)

Trong pod dashboard hiện tại đã kiểm tra:

- **Đường dẫn static:** `/usr/share/nginx/html/`
- **Thời gian file trong image:** `Mar 13 06:27` (assets, index.html)
- **Pod:** `fortuna-dashboard-5d4d84c46b-5cnxb`, **AGE 8h** – pod không được tạo lại gần đây
- **Deployment:** `imagePullPolicy: Never` → image phải có sẵn trên node (k8s-master), không pull từ registry

**Kết luận:** Nếu chạy full clean rebuild + deploy mà vẫn không thấy thay đổi UI thì thường do:

1. **Pod cũ chưa bị thay thế** – rollout restart không tạo pod mới (hoặc không chạy đủ bước).
2. **Image mới chưa có trên node** – build chạy trên máy khác, image `fortuna-dashboard:latest` chưa được push/load lên node đang chạy dashboard (k8s-master).

---

## 2. Lệnh exec kiểm tra nhanh

```bash
NAMESPACE=fortuna
POD=$(kubectl get pods -n $NAMESPACE -l app=fortuna-dashboard -o jsonpath='{.items[0].metadata.name}')

# Xem build time trong image (sau khi thêm version.txt vào Dockerfile)
kubectl exec -n $NAMESPACE $POD -- cat /usr/share/nginx/html/version.txt 2>/dev/null || echo "no version.txt"

# Liệt kê file và ngày giờ
kubectl exec -n $NAMESPACE $POD -- ls -la /usr/share/nginx/html/
kubectl exec -n $NAMESPACE $POD -- ls -la /usr/share/nginx/html/assets/

# Có dùng design-system spacing trong bundle không
kubectl exec -n $NAMESPACE $POD -- sh -c "grep -c 'space-y-8' /usr/share/nginx/html/assets/*.js 2>/dev/null || true"
```

Sau khi build lại có thêm `version.txt`, file này sẽ cho biết thời điểm build image (theo giờ container).

**Nếu `cat /usr/share/nginx/html/version.txt` báo *No such file or directory*:** pod đang chạy **image cũ** (chưa rebuild hoặc chưa rollout). Cần rebuild dashboard và rollout lại:

```bash
# Trên node có chạy dashboard (vd. k8s-master), từ repo root
cd /home/k8s/KSAM
nerdctl --namespace k8s.io build -f dashboard/Dockerfile -t fortuna-dashboard:latest --no-cache .
kubectl rollout restart deployment/fortuna-dashboard -n fortuna
kubectl rollout status deployment/fortuna-dashboard -n fortuna --timeout=90s
# Kiểm tra
kubectl exec -n fortuna deployment/fortuna-dashboard -- cat /usr/share/nginx/html/version.txt
```

Hoặc chạy full pipeline: `./scripts/pipeline/full-clean-database-rebuild-deploy.sh --full`

---

## 2b. Agent – kiểm tra version trong pod

Agent image (từ Dockerfile có `version.txt`) ghi build time vào **`/app/version.txt`** trong container.

```bash
NAMESPACE=fortuna
# Một pod agent bất kỳ (DaemonSet có nhiều pod trên nhiều node)
POD=$(kubectl get pods -n $NAMESPACE -l app=fortuna-agent -o jsonpath='{.items[0].metadata.name}')
kubectl exec -n $NAMESPACE $POD -- cat /app/version.txt 2>/dev/null || echo "no version.txt"
```

**Nếu báo *No such file or directory*:** pod đang chạy **image cũ**. Rebuild agent và rollout lại DaemonSet:

```bash
cd /home/k8s/KSAM
nerdctl --namespace k8s.io build -f agent/Dockerfile -t fortuna-agent:latest --no-cache .
kubectl rollout restart daemonset/fortuna-agent -n fortuna
kubectl rollout status daemonset/fortuna-agent -n fortuna --timeout=120s
# Kiểm tra (pod bất kỳ)
kubectl exec -n fortuna daemonset/fortuna-agent -- cat /app/version.txt
```

**Multi-node:** image phải có trên **mọi node** (build trên từng node hoặc dùng `./scripts/utils/push-images-to-workers.sh` đẩy `fortuna-agent:latest`). Sau đó rollout restart daemonset để tất cả pod agent dùng image mới.

---

## 3. Ép dashboard dùng bản mới

### Bước 1: Rebuild và đảm bảo image có trên node chạy dashboard

- Rebuild trên **chính máy là k8s-master** (node chạy dashboard), **hoặc**
- Sau khi rebuild, chạy push image lên mọi node (kể cả master):

```bash
# Từ repo, sau khi build xong
./scripts/utils/push-images-to-workers.sh
```

(Trong script push cần cấu hình đẩy cả lên k8s-master nếu build chạy ở máy khác.)

### Bước 2: Restart deployment để tạo pod mới

```bash
kubectl rollout restart deployment/fortuna-dashboard -n fortuna
kubectl rollout status deployment/fortuna-dashboard -n fortuna --timeout=90s
```

### Bước 3: Nếu vẫn thấy pod cũ / không đổi

Xóa pod để Kubernetes tạo lại từ deployment (vẫn dùng image `fortuna-dashboard:latest` trên node):

```bash
kubectl delete pod -n fortuna -l app=fortuna-dashboard
# Đợi pod mới Ready
kubectl get pods -n fortuna -l app=fortuna-dashboard -w
```

Sau khi pod mới Ready, exec lại và kiểm tra `version.txt` + ngày file trong `/usr/share/nginx/html/` để xác nhận đúng bản mới.

---

## 3b. Ép agent dùng bản mới

1. **Rebuild agent** (trên node build hoặc từng node nếu multi-node):
   ```bash
   nerdctl --namespace k8s.io build -f agent/Dockerfile -t fortuna-agent:latest --no-cache .
   ```
2. **Đưa image lên mọi node** (nếu build không chạy trên từng node): `./scripts/utils/push-images-to-workers.sh` (đẩy core + agent).
3. **Rollout DaemonSet:**
   ```bash
   kubectl rollout restart daemonset/fortuna-agent -n fortuna
   kubectl rollout status daemonset/fortuna-agent -n fortuna --timeout=120s
   ```
4. **Kiểm tra:** `kubectl exec -n fortuna daemonset/fortuna-agent -- cat /app/version.txt`

---

## 4. Pipeline nên chạy đủ

Khi chạy full clean rebuild + deploy:

1. **Build** – `build-and-load-containerd.sh` build **core, agent, dashboard** (trừ khi `SKIP_DASHBOARD=true` / `BUILD_CORE_ONLY=true`).
2. **Đưa image lên node** – `push-images-to-workers.sh` đẩy core + agent lên mọi node (dashboard không nằm trong script push; nếu multi-node thì build dashboard trên node chạy dashboard hoặc bổ sung push dashboard).
3. **Deploy + rollout** – script restart deployment core, dashboard và daemonset agent, chờ `rollout status` cả ba.

Nếu build chạy trên máy khác, cần push image lên đúng node chạy từng workload (vd. k8s-master cho core/dashboard).
