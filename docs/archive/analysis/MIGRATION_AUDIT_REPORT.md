# Báo cáo kiểm tra Migration

**Ngày:** 2026-02-03  
**Phạm vi:** Toàn bộ migration trong `core/migrations/` (001–061, bao gồm mvp2).

---

## 1. Tổng quan

| Nhóm | Số lượng | Ghi chú |
|------|----------|--------|
| Migration đăng ký trong `RunMigrations` | 61 | 001–003, 008–016, 018–061 (bỏ qua 004–007, 017) |
| File SQL/Go trong thư mục | 59+ | Một số migration dùng inline SQL trong Go |
| File **không** được runner gọi | 3 | Đã xác định và loại bỏ (xem mục 4) |

---

## 2. Trùng lặp schema / dữ liệu

### 2.1 Trùng định nghĩa bảng (không gây trùng dữ liệu)

- **001 vs 002 – `users`**  
  - `001_initial_schema.sql` **không** tạo bảng `users` (chỉ có clusters, nodes, namespaces, pods, RBAC, insights, policies, audit_logs).  
  - `002_add_users.sql` tạo `users` với `CREATE TABLE IF NOT EXISTS`.  
  - **Kết luận:** Không trùng. 002 là nơi duy nhất tạo `users`.

- **001 vs 010 – `nodes`, `policies`, `insights`**  
  - 001: tạo `nodes`, `insights`, `policies` (schema đầy đủ).  
  - 010: lại có `CREATE TABLE IF NOT EXISTS` cho `nodes`, `policies`, `insights` và thêm `events_index`.  
  - Vì dùng `IF NOT EXISTS`, 010 không tạo lại bảng nếu 001 đã chạy trước; 010 chủ yếu đảm bảo tồn tại và có thể thêm cột qua `DO $$ ... ALTER TABLE ... END $$`.  
  - **Kết luận:** Trùng **định nghĩa** trong file, không trùng **dữ liệu**; thứ tự 001 → 010 đúng, không cần sửa.

- **020 vs mvp2/006 – SBOM**  
  - Migration **020** (trong `mvp2_migrations.go`) tạo bảng SBOM bằng **inline SQL** trong Go, **không** đọc file `mvp2/006_add_sbom_tables.sql`.  
  - File `mvp2/006_add_sbom_tables.sql` schema khác (ví dụ `component_count`, `os_packages`, …) so với 020.  
  - **Kết luận:** 006 không còn được dùng; đã đề xuất xóa file 006 (xem mục 4).

### 2.2 Migration một lần / cleanup

- **035_evaluate_trivy_tables.go**  
  - Kiểm tra các bảng Trivy cũ (`trivy_scans`, `trivy_vulnerabilities`, …), có dữ liệu thì log, sau đó DROP hoặc đánh dấu deprecated.  
  - Không tạo bảng mới, không gây trùng dữ liệu.

- **053_fix_minikube_cluster_display_name.go**  
  - Sửa `display_name` cho cluster theo biến môi trường (một lần).  
  - Không trùng schema/dữ liệu.

### 2.3 Kết luận trùng lặp

- **Không có** migration nào gây **trùng dữ liệu** (duplicate rows).  
- Có **trùng định nghĩa** (001 vs 010 cho nodes/insights/policies) nhưng an toàn nhờ `IF NOT EXISTS` và thứ tự chạy.  
- Các migration cũ 030–039 đã được gộp vào 030/031/032 (ghi chú trong `migrations.go`); không còn file migration cũ tương ứng trong repo.

---

## 3. Migration không sử dụng (đã xử lý)

Các **file** sau không được bất kỳ migration nào trong code đọc hoặc gọi:

| File | Lý do |
|------|--------|
| `mvp2/006_add_sbom_tables.sql` | Migration 020 dùng inline SQL trong Go; không tham chiếu tới file này. Doc MIGRATIONS.md từng gợi ý chạy tay 006 → đã cập nhật doc nói rõ dùng migration 020 / Core. |
| `015_add_policy_instances.sql` | `Migration015_AddPolicyInstances` chỉ gọi `db.AutoMigrate(&models.PolicyInstance{})`, không đọc file SQL. |
| `016_add_policy_violations.sql` | `Migration016_AddPolicyViolations` chỉ gọi `db.AutoMigrate(&models.PolicyViolation{})`, không đọc file SQL. |

**Hành động đã thực hiện:** Xóa 3 file trên để tránh nhầm lẫn và đảm bảo “migration không sử dụng được loại bỏ”.

---

## 4. Đăng ký migration (var block)

Trong `migrations.go`, biến `var` dùng để tham chiếu tới từng migration (tránh dead code elimination). Hai migration **058** và **059** đã có trong slice `RunMigrations` nhưng chưa có trong block `var`.

**Hành động đã thực hiện:** Thêm vào `var`:

- `_ = Migration058_AddInsightsEvidenceViolatedRules`
- `_ = Migration059_AddNodeMetadataColumns`

---

## 5. Danh sách migration theo thứ tự (đang chạy)

| # | Hàm / File | Mô tả ngắn |
|---|------------|------------|
| 001 | Migration001_InitialSchema | Schema ban đầu (clusters, nodes, namespaces, pods, RBAC, insights, policies, audit_logs) |
| 002 | Migration002_AddUsers | Bảng users |
| 003 | Migration003_AddUserToAuditLogs | user_id trên audit_logs |
| 004–007 | (bỏ qua) | Không có hàm |
| 008 | Migration008_AddDeployments | Bảng deployments |
| 009 | Migration009_AddReplicaSets | Bảng replicasets |
| 010 | Migration010_ImplementationGuideSchema | nodes, policies, insights, events_index (IF NOT EXISTS) + cột bổ sung |
| 011 | Migration011_AddInsightsSoftDelete | deleted_at, status trên insights |
| 012 | Migration012_AddRiskScores | risk_scores (mvp2/001_risk_scores.sql) |
| 013 | Migration013_AddRiskScoresDeletedAt | deleted_at trên risk_scores (mvp2/002_*) |
| 014 | Migration014_AddPolicyTemplates | policy_templates (mvp2/003_*) |
| 015 | Migration015_AddPolicyInstances | policy_instances (AutoMigrate) |
| 016 | Migration016_AddPolicyViolations | policy_violations (AutoMigrate) |
| 017 | (bỏ qua) | Không có hàm |
| 018 | Migration018_AddRiskScoresV2Columns | Cột V2 risk_scores (mvp2/004_*) |
| 019 | Migration019_AddCVETables | cves, package_vulnerabilities, … (inline SQL) |
| 020 | Migration020_AddSBOMTables | sboms, sbom_components, cve_matches (inline SQL) |
| 021 | Migration021_FixSBOMSchema | p_url → purl, insights.source |
| 022 | Migration022_AddCVEColumnsToInsights | Cột CVE trên insights |
| 023–029 | Các file 023_*.go – 029_*.go | Index SBOM/CVE, performance, unique constraints |
| 030 | Migration030_MigrateInsightsSchemaComplete | Gộp migration insights |
| 031 | Migration031_CleanupDuplicateIndexes | Xóa index trùng |
| 032 | Migration032_MigrateCVEMatchesComplete | Gộp migration cve_matches |
| 033–036 | 033–036 *.go | Unique constraints, CVSS, Trivy cleanup, cột SBOM |
| 037 | Migration037_AddSoftDeleteToResources | deleted_at trên resource tables |
| 038 | Migration038_AddAgentsTable | Bảng agents |
| 039 | Migration039_AddDeletedAtToClusters | deleted_at trên clusters |
| 040–052 | 040_*.go – 052_*.go | Insights, pod capabilities, REP, pod_instances, runtime_signals, capability_metadata, promotion_rules, attack_steps, seed, SBOM one row per pod |
| 053 | Migration053_FixMinikubeClusterDisplayName | Sửa display_name theo env |
| 054–057 | 054–057 *.go | Cluster metadata, drop unique name, notifications, error_logs |
| 058 | Migration058_AddInsightsEvidenceViolatedRules | evidence, violated_rules trên insights |
| 059 | Migration059_AddNodeMetadataColumns | role, os, runtime trên nodes |
| 060 | Migration060_AddCapabilityMetadataExtendedColumns | Cột mở rộng capability_metadata |
| 061 | Migration061_SeedCapabilityMetadataExtended | Seed metadata mở rộng từ spec |

---

## 6. Khuyến nghị

1. **Giữ nguyên** thứ tự và danh sách migration hiện tại; không gộp thêm 001/010 vì đã an toàn với `IF NOT EXISTS`.
2. **Đã loại bỏ** 3 file không dùng: `mvp2/006_add_sbom_tables.sql`, `015_add_policy_instances.sql`, `016_add_policy_violations.sql`.
3. **Đã bổ sung** 058 và 059 vào block `var` trong `migrations.go`.
4. **Doc:** Đã cập nhật `docs/MIGRATIONS.md` để không gợi ý chạy tay `006_add_sbom_tables.sql`; thay bằng hướng dẫn kiểm tra/chạy migration 020 qua Core.

---

**Tác giả báo cáo:** Migration audit (script/kiểm tra thủ công).  
**File tham chiếu:** `core/migrations/migrations.go`, `core/migrations/mvp2_migrations.go`, và toàn bộ file trong `core/migrations/`.
