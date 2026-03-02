# Fortuna – Tài liệu Production

Tài liệu chuẩn production cho nền tảng **Fortuna** (KSAM): bảo mật và quản lý rủi ro cho Kubernetes.

---

## Mục lục tài liệu

| # | Tài liệu | Nội dung |
|---|----------|----------|
| 1 | [Giới thiệu dự án](01-PROJECT_OVERVIEW.md) | Mô tả dự án, mục tiêu, phạm vi, đối tượng sử dụng |
| 2 | [Kiến trúc](02-ARCHITECTURE.md) | Kiến trúc hệ thống, luồng dữ liệu, thành phần, HA |
| 3 | [Chức năng](03-FEATURES.md) | Danh sách chức năng, SBOM, CVE, PCE, Runtime Signals, Dashboard |
| 4 | [Hướng dẫn sử dụng](04-USER_GUIDE.md) | Truy cập Dashboard, Risk Center, SBOM, API, quy trình sử dụng |
| 5 | [Vận hành](05-OPERATIONS.md) | Deploy, nâng cấp, monitoring, xử lý sự cố, backup |
| 6 | [Cấu hình](06-CONFIGURATION.md) | Biến môi trường, Secrets, Registry, tùy chỉnh production |

---

## Script production

Các script vận hành production nằm ở **`script-prod/`** (thư mục gốc repo):

- **build** – Build image với tag version, optional push registry
- **deploy** – Deploy lên cluster (config-driven, versioned images)
- **clean** – Dọn tài nguyên, image, optional DB (có xác nhận)
- **verify** – Kiểm tra health và trạng thái sau deploy

Chi tiết: [script-prod/README.md](../script-prod/README.md).

---

## Liên kết nhanh

- **Deploy manifests:** `deploy/` (xem [deploy/README.md](../deploy/README.md))
- **Tài liệu phát triển / E2E:** `docs/`
- **Script phát triển / pipeline:** `scripts/`

---

**Phiên bản tài liệu:** 1.0  
**Cập nhật:** 2026-03-02
