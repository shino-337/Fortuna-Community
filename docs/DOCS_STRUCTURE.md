# Cấu trúc thư mục tài liệu (Documentation Structure)

**Mục đích:** Sắp xếp tài liệu theo thư mục chuẩn; tài liệu cũ/outdated chuyển vào `archive/`.

---

## Thư mục chính

| Thư mục | Nội dung |
|---------|----------|
| **docs/** (root) | README.md (điểm vào), DOCS_STRUCTURE.md, **AGENT_CORE_ERRORS_MONITOR.md** (monitor & xử lý lỗi Agent/Core) |
| **01-getting-started/** | Chuẩn bị môi trường, build, quickstart, deployment cơ bản |
| **02-architecture/** | Kiến trúc hệ thống, ADR, policy engine, migration design |
| **03-components/** | Core, Agent, PCE, SBOM, CVE, runtime signals, data sync |
| **04-development/** | Migrations, seed data, logic (orphan pod, cluster id), testing |
| **05-operations/** | Deployment production, checklist, containerd, clean rebuild, port-forward, network, storage |
| **06-reference/** | API reference, script paths |
| **07-guides/** | UI/UX spec, dashboard (features, filters, clusters), risk center |
| **08-tutorials/** | Hướng dẫn từng bước (nếu có) |
| **test-results/** | README + kết quả test mới nhất (giữ tối thiểu) |
| **e2e/** | Tài liệu E2E (scenarios, summary) |
| **archive/** | Tài liệu cũ / one-off / đã superseded |

---

## Archive

- **archive/fixes/** – Báo cáo sửa lỗi one-off (Postgres, disk, eviction, DNS, CVE sync, …).
- **archive/task-lists/** – TODO, PENDING_TASKS, UI_AND_PENDING_TASKS (đã xử lý hoặc outdated).
- **archive/analysis/** – Phân tích/debug cũ (dashboard analysis, migration audit report).
- **archive/test-results/** – Báo cáo test theo ngày (E2E-*, CLEAN-*, DEBUG-*, …).
- **archive/implementation-plans/** – Kế hoạch triển khai đã hoàn thành hoặc superseded.

---

## Quy ước

- Tài liệu **đang dùng** nằm trong 01–08 theo chủ đề.
- Tài liệu **cũ / one-off / đã thay thế** chuyển vào `archive/` (có thể có thư mục con).
- **test-results:** Chỉ giữ README và (tùy chọn) 1–2 báo cáo mới nhất; phần còn lại chuyển `archive/test-results/`.
- Cập nhật **docs/README.md** sau khi di chuyển để link đúng đường dẫn.
