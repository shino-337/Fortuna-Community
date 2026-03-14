# Luồng SBOM (Agent → Core) và kiểm tra thực tế trên cluster

## 1. Luồng xử lý SBOM (code)

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│ AGENT (mỗi node, DaemonSet)                                                      │
├─────────────────────────────────────────────────────────────────────────────────┤
│ 1. LocalPodWatcher (informer): watch pod có spec.nodeName = NODE_NAME            │
│    → Pod Running được đưa vào queue (chan *corev1.Pod), buffer 30                 │
│    → Log: "→ Queued pod ns/name for async processing" hoặc "Queue full, dropping" │
│ 2. SBOM WorkQueue: 2 workers (SBOM_WORKERS) lấy pod từ queue                      │
│    → Log: "[Worker N] Processing pod ns/name"                                     │
│ 3. ProcessPod: với từng container trong pod                                      │
│    a. ExtractSBOM(imageRef): containerd/registry → layers → virtual FS           │
│       → parsers (dpkg/apk/npm/pip/gomod) → RawSBOM (packages + digest)           │
│       Log: "🔍 Extracting SBOM...", "✅ Extracted N packages" hoặc "SBOM extraction failed" │
│    b. convertToProto(pod, container, rawSBOM) → pb.SBOMFinding                    │
│    c. grpcClient.SendSBOMFinding(ctx, sbomFinding) → Core gRPC                     │
│       Log: "Sending SBOM: pod=... image=...", "✅ SBOM sent to Core: sbom_id=..."  │
│       Hoặc lỗi: "failed to send SBOM to Core: client not connected"               │
└─────────────────────────────────────────────────────────────────────────────────┘
                                        │
                                        ▼ gRPC (mTLS)
┌─────────────────────────────────────────────────────────────────────────────────┐
│ CORE                                                                             │
├─────────────────────────────────────────────────────────────────────────────────┤
│ 4. SendSBOMFinding (handler_sbom.go): nhận pb.SBOMFinding                         │
│    Log: "[SBOM] Received SBOM from agent=... pod=... image=..."                   │
│ 5. DB: upsert sboms theo pod_uid (1 row per pod), insert sbom_components          │
│    Log: "[SBOM] Created new SBOM id=N for pod_uid=..." hoặc "Updated existing..." │
│ 6. NATS: publish "fortuna.sbom.created" → CVE matcher worker (nếu có NATS)        │
│ 7. Dashboard/API: GET /api/v1/inventory/pods/:uid/sbom → query sboms WHERE pod_uid │
│    → Nếu không có row: 404 "sbom not found for pod"                               │
└─────────────────────────────────────────────────────────────────────────────────┘
```

**Lưu ý:** Từ bản có retry (agent/internal/sbom/queue.go): khi SendSBOMFinding thất bại do lỗi thoáng qua (client not connected, connection refused, …), Agent **tự retry** tối đa 3 lần, mỗi lần cách 30s, bằng cách đẩy lại pod vào queue. Chi tiết thứ tự khởi động và kịch bản mất SBOM: **docs/SBOM_AGENT_CORE_ORDER_AND_DATA_LOSS.md**.

---

## 2. Kết quả kiểm tra thực tế (cluster hiện tại)

### 2.1 Database

| Bảng    | Số bản ghi | Ghi chú |
|---------|------------|--------|
| pods    | 19         | Pod đã sync từ agent (cluster inventory) |
| sboms   | 1          | Chỉ 1 pod có SBOM |

**Pod có SBOM:**

- `pod_uid`: `cb634e29-63a7-41d3-92e2-4ecb99f99c5d`
- `pod_name`: `website-vuln-lodash`
- `namespace`: `fortuna-e2e`
- `created_at`: 2026-03-13 15:28:09

Các pod còn lại (fortuna-core, fortuna-dashboard, kube-flannel, nats, postgres, coredns, …) **không có** row trong `sboms` → Dashboard gọi GET `/inventory/pods/:uid/sbom` sẽ 404 "sbom not found for pod".

### 2.2 Agent logs (nguyên nhân thiếu SBOM)

**Agent trên k8s-master (fortuna-agent-d2dtd):**

- **fortuna-core pod:** Extract 106 packages xong, nhưng **"Failed to process container core: failed to send SBOM to Core: client not connected"** (Core chưa sẵn sàng hoặc mất kết nối).
- **website-vuln-lodash (fortuna-e2e):** Queued → Extract 109 packages → **"✅ SBOM sent to Core: sbom_id=1 message=SBOM received and stored"** (15:28:09) → đúng với 1 row trong DB.
- Có log **"Reconnect failed"**, **"Heartbeat failed: client not connected"** → giai đoạn Core restart/network/DNS agent mất kết nối gRPC.

**Agent trên k8s-worker01 (fortuna-agent-8cxst):**

- **kube-flannel:** Extract 51 packages xong, **"Failed to process container kube-flannel: failed to send SBOM to Core: client not connected"**.
- Nhiều **"Reconnect failed"**, **"no such host"** (fortuna-core.fortuna.svc.cluster.local) → Agent worker không resolve được Core qua DNS hoặc Core chưa lên.

**Kết luận:** Phần lớn pod đã được agent extract SBOM nhưng **gửi thất bại vì lúc đó client không kết nối được Core**. Chỉ pod được xử lý **sau khi** Core sẵn sàng và agent đã kết nối lại (website-vuln-lodash) mới có SBOM trong DB.

### 2.3 Dashboard không có thông tin SBOM

- Danh sách pod: từ **GET /api/v1/inventory/pods** (sync từ DB `pods`).
- Khi mở Pod detail hoặc SBOM tab: **GET /api/v1/inventory/pods/:uid/sbom**.
- Core query `sboms` WHERE `pod_uid = :uid`; với 19 pod chỉ 1 có row → 18 pod trả về 404 → Dashboard hiển thị không có SBOM (empty / "sbom not found for pod").

---

## 3. Lệnh kiểm tra nhanh trên cluster

```bash
NAMESPACE=fortuna

# 1) Số SBOM vs số pod trong DB
PG_POD=$(kubectl -n $NAMESPACE get pods -l app=postgres -o jsonpath='{.items[0].metadata.name}')
kubectl -n $NAMESPACE exec $PG_POD -- psql -U postgres -d fortuna -t -c \
  "SELECT (SELECT COUNT(*) FROM pods WHERE deleted_at IS NULL) AS pods, (SELECT COUNT(*) FROM sboms WHERE deleted_at IS NULL) AS sboms;"

# 2) Pod nào có SBOM
kubectl -n $NAMESPACE exec $PG_POD -- psql -U postgres -d fortuna -t -c \
  "SELECT pod_uid, pod_name, namespace FROM sboms WHERE deleted_at IS NULL;"

# 3) Agent: log SBOM / send / fail
kubectl logs -n $NAMESPACE -l app=fortuna-agent --tail=500 | grep -iE 'SBOM|SendSBOM|Queued|Failed to process|client not connected'

# 4) Core: log nhận SBOM
kubectl logs -n $NAMESPACE deployment/fortuna-core --tail=300 | grep '\[SBOM\]'
```

---

## 4. Khuyến nghị

1. **Đảm bảo Core sẵn sàng và Agent kết nối được Core (DNS, mTLS, network)**  
   Nếu không, mọi SendSBOMFinding có thể fail với "client not connected". Cần xử lý DNS (fortuna-core.fortuna.svc.cluster.local), Service/endpoints và certs.

2. **Retry khi gửi SBOM thất bại (transient)**  
   **Đã triển khai:** Trong agent/internal/sbom/queue.go, khi ProcessPod trả lỗi transient (client not connected, connection refused, …), pod được re-queue tối đa 3 lần, mỗi lần delay 30s. Xem docs/SBOM_AGENT_CORE_ORDER_AND_DATA_LOSS.md.

3. **Re-sync SBOM cho pod đã có trong cluster**  
   Hiện tại pod chỉ được queue khi watcher thấy Add/Update (Running). Pod đã chạy từ trước và đã bị "Failed to process" sẽ không tự động queue lại. Có thể thêm cơ chế định kỳ hoặc thủ công: queue lại các pod chưa có SBOM trong DB (so với danh sách pod trên node) để agent extract và gửi lại.

4. **Dashboard**  
   Giữ nguyên luồng hiện tại: GET pods → GET pods/:uid/sbom. Chỉ khi DB có row `sboms` cho `pod_uid` thì API mới trả về SBOM; sau khi khắc phục Agent/Core và retry/re-sync, số pod có SBOM sẽ tăng và dashboard sẽ hiển thị tương ứng.
