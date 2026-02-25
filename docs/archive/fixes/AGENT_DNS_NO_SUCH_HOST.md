# Agent: "lookup fortuna-core.fortuna.svc.cluster.local ... no such host"

*Cập nhật: 2026-02-05.*

---

## 1. Triệu chứng

Trong log agent xuất hiện:

```
transport: Error while dialing: dial tcp: lookup fortuna-core.fortuna.svc.cluster.local on 10.96.0.10:53: no such host
⚠️  Heartbeat failed: Ping RPC failed: ...
```

Sau đó có thể thấy `[Syncer] ✅ Full sync completed` và `✅ Heartbeat OK` — lỗi chỉ xảy ra trong một khoảng thời gian ngắn.

---

## 2. Nguyên nhân

- **10.96.0.10** là ClusterIP của Kubernetes DNS (CoreDNS/kube-dns). Agent gửi truy vấn DNS tới đây để resolve `fortuna-core.fortuna.svc.cluster.local`.
- **"no such host"** có nghĩa DNS trả về NXDOMAIN (không có bản ghi) hoặc truy vấn DNS thất bại (timeout/unreachable) và resolver báo "no such host".

**Thường gặp khi:**

1. **Sau rebuild/deploy:** Khi `rollout restart` Core và Agent, Core pod bị terminate rồi tạo lại. Trong thời gian ngắn:
   - Service `fortuna-core` vẫn tồn tại (ClusterIP không đổi), nhưng CoreDNS có thể chưa kịp cập nhật hoặc pod network trên worker chưa ổn định.
   - Agent (đặc biệt trên **worker node**) gọi DNS ngay lúc đó → nhận "no such host" hoặc lỗi kết nối.
2. **Multi-node:** Agent chạy trên worker, Core chạy trên control-plane. Đường đi DNS (pod → 10.96.0.10 → CoreDNS) có thể bị ảnh hưởng tạm thời (CNI, CoreDNS restart).
3. **Thứ tự deploy:** Nếu Agent được deploy/restart trước khi Service Core và DNS ổn định, các lần gọi đầu tiên có thể thất bại.

---

## 3. Đã xử lý trong code

### Agent

- **Retry Connect khi khởi động:** Nếu DNS chưa resolve được lúc agent start, agent **không** thoát ngay mà retry tối đa 12 lần, mỗi lần cách 10s (~2 phút). Sau khi Core và DNS ổn định, Connect thành công.
- **Reconnect khi Heartbeat thất bại liên tục:** Sau **3 lần** Ping/Heartbeat thất bại liên tiếp (ví dụ do Core restart, connection bị đứt), agent gọi **Reconnect** (đóng connection cũ, Dial lại) rồi **RegisterAgent** lại. Không cần restart pod agent.

### Deploy

- **deploy-fortuna-robust.sh** có **Step 8b:** Chạy một pod tạm trong namespace fortuna để `nslookup fortuna-core.fortuna.svc.cluster.local`. Nếu fail, chỉ ghi cảnh báo (Agent vẫn được deploy và sẽ tự retry/reconnect).

---

## 4. Cách kiểm tra

```bash
# DNS từ trong cluster (namespace fortuna)
kubectl run dns-test --image=busybox:1.36 --rm -i --restart=Never -n fortuna -- nslookup fortuna-core.fortuna.svc.cluster.local

# Log agent (sau khi fix: sẽ thấy retry Connect hoặc "Reconnected and re-registered" nếu từng lỗi)
kubectl logs -n fortuna -l app.kubernetes.io/component=agent --tail=50
```

---

## 5. Nếu vẫn lỗi kéo dài

- Kiểm tra CoreDNS: `kubectl get pods -n kube-system -l k8s-app=kube-dns` (hoặc CoreDNS).
- Kiểm tra Service Core: `kubectl get svc fortuna-core -n fortuna` và `kubectl get endpoints fortuna-core -n fortuna`.
- Multi-node: Chạy `./scripts/deploy/fix-flannel-vxlan.sh` nếu nghi ngờ pod network; hoặc dùng IP fallback (trong deploy-fortuna-robust khi DNS test fail đã có hướng dẫn dùng IP + TLS_ENABLED=false).
