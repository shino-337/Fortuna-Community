# Fortuna – Production Documentation

Production-focused documentation for **FortunaK8s** – K8S Security & Risk Management Platform.

---

## Nội dung chính (Main contents)

| # | Document | Mô tả |
|---|----------|--------|
| 1 | [Project overview](01-PROJECT_OVERVIEW.md) | Mục tiêu, phạm vi, đối tượng, thuật ngữ |
| 2 | [Architecture](02-ARCHITECTURE.md) | Kiến trúc hệ thống, luồng dữ liệu, HA, bảo mật |
| 3 | [Features](03-FEATURES.md) | SBOM, CVE, PCE, Runtime Signals, Dashboard, Pod Detail |
| 4 | [User guide](04-USER_GUIDE.md) | Truy cập Dashboard, Risk Center, SBOM, Resources, Pod Detail, API |
| 5 | [Operations](05-OPERATIONS.md) | Deploy, nâng cấp, monitoring, xử lý sự cố, backup, script pipeline |
| 6 | [Configuration](06-CONFIGURATION.md) | Biến môi trường, secrets, registry, tài nguyên |

---

## Bản đồ tài liệu (Documentation map)

| Chủ đề | docs-prod | docs/ (development) |
|--------|-----------|----------------------|
| Tổng quan / Kiến trúc / Tính năng | 01–03 | 02-architecture/, 03-components/ |
| Hướng dẫn sử dụng / Vận hành / Cấu hình | 04–06 | 05-operations/, 07-guides/ |
| Build, deploy thủ công, pipeline | 05, script-prod | 01-getting-started/, 05-operations/, scripts/README.md |
| API chi tiết | Tham chiếu trong 03, 04 | 06-reference/API_REFERENCE.md |
| Pod Detail (spec, schema, test) | 03, 04 | 03-components/podDetail/POD_DETAIL_SPEC.md, POD_SYNC_ARCHITECTURE_AND_DATA_MODEL_SPEC.md, testSuite.md |
| Migrations DB | 05, 06 | 04-development/MIGRATIONS.md, core/migrations/ |
| E2E / Test / Verify | 05 | docs/TESTCASE_MONITOR.md, scripts/verify/, scripts/e2e/ |
| Xử lý lỗi Agent/Core | 05 | docs/AGENT_CORE_ERRORS_MONITOR.md |

---

## Scripts quan trọng

### Production (script-prod/)

- **build** – Build image có tag version; tùy chọn push registry.
- **deploy** – Deploy lên cluster (config-driven, image versioned).
- **clean** – Dọn workload, tùy chọn image/DB (có xác nhận).
- **verify** – Kiểm tra health và trạng thái sau deploy.

Chi tiết: [script-prod/README.md](../script-prod/README.md).

### Development / pipeline (scripts/)

- **Pipeline:** `full-clean-database-rebuild-deploy.sh` (clean + rebuild + deploy), `clean-rebuild-redeploy-and-test.sh` (pipeline + test).
- **Deploy:** `deploy-fortuna-robust.sh` (deploy đầy đủ, mTLS, Flannel, StorageClass).
- **Verify:** `check-full-deployment.sh`, `verify-database-schema.sh`, `verify-pod-detail-api-and-db.sh`, `verify-agent-core-connectivity.sh`.
- **Build:** `build-and-load-containerd.sh`, `clean-rebuild-dashboard.sh`.
- **Push image lên node:** `push-images-to-workers.sh` (cần SSH hoặc config).

Chi tiết: [scripts/README.md](../scripts/README.md).

---

## Quick links

- **Deploy manifests:** [deploy/README.md](../deploy/README.md)
- **Tài liệu dev & E2E:** [docs/](../docs/README.md)
- **Script dev & pipeline:** [scripts/](../scripts/README.md)
- **Root README:** [README.md](../README.md)

---

**FortunaK8s** – K8S Security & Risk Management Platform  
**Doc version:** 1.1 · **Updated:** 2026-03-03
