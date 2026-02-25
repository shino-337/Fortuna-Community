# Báo cáo: Deploy Core mới + Rollout + Test Pod-delete cleanup

**Ngày:** 2026-02-22  
**Nội dung:** Build core mới, rollout deployment fortuna-core, chạy test kiểm tra điều chỉnh cleanup/hiển thị khi pod xóa.

---

## 1. Tiến độ thực hiện

| Bước | Trạng thái | Ghi chú |
|------|------------|---------|
| Build core + agent (SKIP_DASHBOARD=true) | ✅ Hoàn thành | fortuna-core:latest, fortuna-agent:latest load vào containerd (k8s.io) |
| Rollout restart fortuna-core | ✅ Hoàn thành | deployment "fortuna-core" successfully rolled out |
| E2E: Pod-delete cleanup verify | ✅ Chạy xong | Script: scripts/e2e/e2e-pod-delete-cleanup-verify.sh |

---

## 2. Kết quả E2E

- **APIs đã chỉnh (PCE, runtime risk):** GET pod-capabilities/summary/capability, summary/severity, trends?days=7, runtime-risk/summary → trả về JSON hợp lệ (chỉ đếm pod còn tồn tại).
- **Pod count vs PCE:** Số pod active và tổng PCE theo cluster nhất quán (APIs dùng JOIN pods deleted_at IS NULL).
- **Luồng xóa pod:** Tạo pod test → xóa → đợi correlator; runtime-risk cho pod UID đã xóa trả 404/error (cleanup đã chạy).
- **Lưu ý:** Pod count trước/sau xóa có thể không giảm ngay nếu agent chưa gửi Deleted hoặc sync chậm; list API vẫn chỉ hiển thị pod chưa xóa.

---

## 3. Cách chạy lại

```bash
# Build core (và agent), load vào containerd
BUILD_TAG=latest SKIP_DASHBOARD=true bash scripts/build/build-and-load-containerd.sh

# Rollout core
kubectl rollout restart deployment fortuna-core -n fortuna
kubectl rollout status deployment fortuna-core -n fortuna --timeout=180s

# E2E pod-delete cleanup & display
NAMESPACE=fortuna bash scripts/e2e/e2e-pod-delete-cleanup-verify.sh
```

---

## 4. Tài liệu liên quan

- **Logic cleanup/hiển thị:** docs/POD_DELETE_CLEANUP_AND_DISPLAY.md  
- **Deploy:** docs/DEPLOY_FIX_AND_CLEAN_REDEPLOY.md, scripts/deploy/deploy-fortuna-robust.sh  
