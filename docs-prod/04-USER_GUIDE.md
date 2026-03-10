# Hướng dẫn sử dụng FortunaK8s (Production)

## 1. Truy cập Dashboard

### 1.1 Port-forward (phát triển / kiểm tra)

```bash
# Core API (nếu cần gọi API trực tiếp)
kubectl port-forward -n fortuna svc/fortuna-core 8080:8080

# Dashboard (trình duyệt)
kubectl port-forward -n fortuna svc/fortuna-dashboard 8081:80
```

Mở trình duyệt: **http://localhost:8081**

**Nếu báo lỗi "address already in use" (cổng 8081 đã dùng):** Đã có port-forward đang chạy → mở trực tiếp http://localhost:8081; hoặc dừng tiến trình cũ: `pkill -f 'port-forward.*fortuna-dashboard'` rồi chạy lại; hoặc dùng cổng khác: `kubectl port-forward -n fortuna svc/fortuna-dashboard 8082:80` → http://localhost:8082.

### 1.2 Production (LoadBalancer / Ingress)

- Nếu Dashboard dùng Service type **LoadBalancer:** lấy EXTERNAL-IP hoặc hostname từ `kubectl get svc -n fortuna fortuna-dashboard`.
- Nếu dùng **Ingress:** truy cập theo host và path đã cấu hình (HTTPS khuyến nghị).

### 1.3 Đăng nhập

- **Mặc định:** username `admin`, password `admin123`.
- **Production:** Đổi password qua cấu hình Core / seed user; không dùng mặc định trên môi trường thật.

---

## 2. Trang chủ (Dashboard)

- **Số liệu tổng:** Clusters, Agents, Pods, Total risks, Critical/High risks, Resolved 24h.
- **Threat Velocity:** Biểu đồ xu hướng risk theo ngày (7 ngày mặc định).
- **PCE Trends:** Xu hướng capability theo thời gian.
- **Cluster Health:** Danh sách cluster, trạng thái sync, agent.

---

## 3. Risk Center

- **Risks:** Danh sách risk/insight theo cluster; filter theo severity (Critical/High/Medium).
- **Summary:** Tổng số findings, phân bố critical/high.
- **Chi tiết:** Click từng risk → xem pod, CVE, capability, mô tả.
- **Runtime Signals (tab):** Tín hiệu runtime theo pod/severity.
- **PCE drill-down:** Lọc theo capability, pod name (nếu UI hỗ trợ).

---

## 4. SBOM

- **Danh sách SBOM:** Theo pod / namespace / image.
- **Chi tiết:** Package, version, PURL; CVE liên quan nếu có.
- **Export:** CSV/JSON (tùy tính năng Dashboard); API `/api/v1/sbom` hỗ trợ export.

---

## 5. Resources / Pod Detail

- **Pod list:** Danh sách pod đồng bộ từ Agent (Resources tab).
- **Pod Detail (trang chi tiết pod):**
  - **Header cards:** Status, **Pod IP**, **Start Time**, **Uptime**, **Restart Count**, **QoS Class**, Risk Count, Service Account, Created.
  - **Overview tab:** Namespace, Node, Pod IP, Service Account, UID; **Identity & Ownership** (Owner Type, Owner Name, ReplicaSet, QoS Class) khi có dữ liệu.
  - Tab **SBOM**, **Related Risks**.
- Dữ liệu Pod IP / Start Time / Owner / QoS do Agent gửi lên Core; nếu thấy "—" là Agent chưa sync bản mới hoặc pod chưa được sync lại. Kiểm tra: `./scripts/verify/verify-pod-detail-api-and-db.sh`.
- Dùng để điều hướng từ Risk Center hoặc kiểm tra SBOM theo pod.

---

## 6. Gọi API trực tiếp (curl / script)

1. **Lấy JWT:**
   ```bash
   TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
     -H "Content-Type: application/json" \
     -d '{"username":"admin","password":"admin123"}' | jq -r '.token')
   ```
2. **Gọi endpoint:**
   ```bash
   curl -s -H "Authorization: Bearer $TOKEN" "http://localhost:8080/api/v1/dashboard/stats"
   curl -s -H "Authorization: Bearer $TOKEN" "http://localhost:8080/api/v1/risks?clusterId=xxx"
   ```
3. **Production:** Thay `localhost:8080` bằng URL Core (qua Ingress/LoadBalancer); dùng HTTPS.

---

## 7. Quy trình sử dụng điển hình

1. **Sau deploy:** Đăng nhập Dashboard → kiểm tra số cluster/agent/pod và risks.
2. **Hàng ngày:** Mở Risk Center → xem Critical/High → xử lý theo quy trình nội bộ (patch, thay image, giảm capability).
3. **Audit / Compliance:** Export SBOM/CVE từ SBOM trang hoặc API; dùng cho báo cáo.
4. **Điều tra pod:** Vào Resources → chọn pod → xem capabilities, runtime signals, SBOM.

---

**Tiếp theo:** [Vận hành](05-OPERATIONS.md)
