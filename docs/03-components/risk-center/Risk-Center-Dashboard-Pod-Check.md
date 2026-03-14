# Risk Center – Kiểm tra Dashboard từ Pod đang chạy

Tài liệu ghi lại cách kiểm tra chi tiết từ pod dashboard đang chạy để xác nhận build có đủ tính năng Risk Center (kể cả Export CSV/PDF).

---

## 1. Thông tin Pod (kiểm tra lúc chạy)

- **Namespace:** `fortuna`
- **Label:** `app=fortuna-dashboard`
- **Lệnh lấy pod:**  
  `kubectl get pods -n fortuna -l app=fortuna-dashboard -o wide`
- **Image:** `fortuna-dashboard:latest` (trong `deploy/dashboard-deployment.yaml`)
- **ImageID (ví dụ):** `sha256:33ddb21870b6...` — dùng để đối chiếu sau khi rebuild.

---

## 2. Kiểm tra chi tiết bên trong Pod

### 2.1. Cấu trúc file build (static assets)

```bash
POD=$(kubectl get pods -n fortuna -l app=fortuna-dashboard -o jsonpath='{.items[0].metadata.name}')
kubectl exec -n fortuna $POD -- ls -la /usr/share/nginx/html/
kubectl exec -n fortuna $POD -- ls -la /usr/share/nginx/html/assets/
```

- **Kỳ vọng:** có `index.html`, thư mục `assets/` chứa `index-*.js` và `index-*.css` (tên có hash, ví dụ `index-BcQ8xF7B.js`).
- **Thời gian file:** trùng với lần build image gần nhất (để xác nhận không phải bản cũ).

### 2.2. Risk Center – Export CSV/PDF có trong bundle không?

Code Export CSV/PDF nằm trong bundle JS. Kiểm tra nhanh:

```bash
kubectl exec -n fortuna $POD -- sh -c "grep -l 'Export CSV\|exportRisksCSV\|exportRisksPDF' /usr/share/nginx/html/assets/*.js"
```

- **Kỳ vọng:** có ít nhất một file (ví dụ `index-*.js`) chứa chuỗi trên → build đang chạy có tính năng Export.

### 2.3. Build identifier (footer)

Sau khi thêm `__BUILD_TIME__` trong Vite, footer hiển thị **"Fortuna Dashboard · build &lt;timestamp&gt;"**. Timestamp được nhúng vào JS lúc build.

- **Trên UI:** mở Dashboard, cuộn xuống cuối trang → xem dòng build.
- **Trong pod:** chuỗi có thể bị minify, có thể tìm:  
  `grep -o 'Fortuna Dashboard.[^}]*build[^}]*' /usr/share/nginx/html/assets/index-*.js` (tùy minifier có thể cần điều chỉnh pattern).

---

## 3. Kết luận kiểm tra

| Kiểm tra | Ý nghĩa |
|----------|--------|
| Pod `Running`, image `fortuna-dashboard:latest` | Deployment đang dùng đúng image dashboard. |
| File trong `/usr/share/nginx/html/assets/` có timestamp mới | Nội dung trong container đến từ lần build gần nhất. |
| Bundle JS chứa `Export CSV` / `exportRisksCSV` / `exportRisksPDF` | Build có mã Risk Center Export CSV/PDF. |
| Footer hiển thị "Fortuna Dashboard · build &lt;timestamp&gt;" | Build có mã mới (footer + __BUILD_TIME__). |

Nếu mọi mục đều đúng mà trên UI vẫn không thấy nút Export: đảm bảo đang ở **Risk Center → tab "Risk Findings"** và hard refresh (Ctrl+Shift+R).

---

## 4. Clean và Rebuild Dashboard

Để đảm bảo dashboard được cập nhật từ code mới nhất và xử lý đủ thông tin Risk Center:

```bash
cd /home/k8s/KSAM
./scripts/clean/clean-rebuild-dashboard.sh
```

Script sẽ:

1. Dừng port-forward dashboard/core (nếu có).
2. Xóa toàn bộ image `fortuna-dashboard` cũ (nerdctl + ctr).
3. Prune build cache (nerdctl builder prune).
4. Xóa deployment `fortuna-dashboard` trong namespace `fortuna`.
5. Rebuild image bằng `scripts/build/build-dashboard-containerd.sh` (nerdctl build từ `dashboard/Dockerfile`).
6. Kiểm tra image mới trong containerd.
7. Apply lại `deploy/dashboard-deployment.yaml` (image: `fortuna-dashboard:latest`) và chờ deployment ready.

Sau khi chạy xong, kiểm tra lại theo mục 2; ImageID của pod sẽ đổi so với trước khi rebuild.

---

## 5. Kết quả lần chạy gần nhất (sau clean + rebuild)

- **Pod cũ (trước rebuild):** `fortuna-dashboard-6df5d9b7b8-k26nv`, imageID `sha256:33ddb21870b6...`, assets `index-BcQ8xF7B.js`, `index-CqmiIhgZ.css` (Mar 10 06:42).
- **Pod mới (sau rebuild):** `fortuna-dashboard-5f6d6bcc88-cfkws`, imageID `sha256:25090814cc...`, assets `index-B7oTejmV.js`, `index-nLMeeTns.css` (Mar 11 06:18). Bundle JS có chứa `Export CSV` / `exportRisksCSV` → Risk Center export đã có trong build.
- **Image:** `fortuna-dashboard:latest` (sha256:8be507f44551), build time 2026-03-11T06:14:38Z.
