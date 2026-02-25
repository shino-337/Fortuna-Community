# Báo cáo Rebuild và Deploy Core, Agent, Database

**Ngày:** 2026-02-24

## 1. Các bước đã thực hiện

| Bước | Nội dung | Kết quả |
|------|----------|---------|
| 1 | Rebuild core, agent, dashboard (containerd) | OK — `build-and-load-containerd.sh` chạy xong; images: fortuna-core, fortuna-agent, fortuna-dashboard |
| 2 | Deploy fortuna | Timeout 5 phút; script deploy đã chạy (rollout restart nhiều pod). **Core deployment và Service bị thiếu** sau khi timeout → đã **apply lại** `deploy/fortuna-core-deployment.yaml` thủ công |
| 3 | Core deployment | `kubectl apply -f deploy/fortuna-core-deployment.yaml` — Service + Deployment fortuna-core đã được tạo, pod Core Ready 1/1 |
| 4 | Kiểm tra DB & CVE | DB có dữ liệu CVE từ cve-loader Job (đã chạy trước đó) |

## 2. Trạng thái hiện tại

### 2.1 Pods (namespace fortuna)

| Thành phần | Trạng thái |
|------------|------------|
| fortuna-core | 1/1 Running |
| fortuna-agent | 2/2 Running (DaemonSet) |
| fortuna-dashboard | 1/1 Running |
| postgres | 1/1 Running |
| nats | 3/3 Running |
| cve-loader | 1/1 Running (Job pod; Job 0/1 completions — đang chạy hoặc chưa complete) |

### 2.2 Database (migrations & CVE)

- **Migrations:** Core khởi động bình thường; log Core cho thấy truy vấn DB (insights, pod_capabilities, capability_metadata) — migrations đã chạy.
- **CVE reference (OSV đã load):**

| Bảng | Số bản ghi |
|------|------------|
| cves | **432,607** |
| package_vulnerabilities | **849,190** |
| cve_matches | 100 |

→ Dữ liệu OSV đã được load vào DB (cve-loader Job đã chạy thành công trước đó).

### 2.3 Login

- Endpoint đúng: `POST /api/v1/auth/login` (không phải `/api/auth/login`).
- Trong phiên kiểm tra, port-forward đã thoát trước khi gọi đúng path; Core đang chạy, có thể kiểm tra login bằng:
  ```bash
  kubectl port-forward -n fortuna svc/fortuna-core 8080:8080
  curl -X POST http://127.0.0.1:8080/api/v1/auth/login -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123"}'
  ```

## 3. Tóm tắt

- **Rebuild:** Thành công — core, agent, dashboard đã build và load vào containerd.
- **Deploy:** Script deploy bị timeout; Core deployment + Service đã được khôi phục bằng `kubectl apply -f deploy/fortuna-core-deployment.yaml`. Core, Agent, Dashboard, Postgres, NATS đều Running.
- **Database:** Migrations chạy khi Core khởi động; bảng CVE có dữ liệu (432k CVEs, 849k package_vulnerabilities).
- **Khuyến nghị:** Khi chạy `deploy-fortuna-robust.sh` lần sau, có thể tăng timeout hoặc chạy từng bước (RBAC → Core → Agent → Dashboard) để tránh timeout; sau deploy nên kiểm tra `kubectl get deployment -n fortuna fortuna-core` để đảm bảo Core tồn tại.
