# Đánh giá toàn diện: Thêm & Verify Risk Rule từ giao diện

Tài liệu mô tả chi tiết luồng xử lý từ Dashboard (agent) tới Core, cơ chế lưu trữ/đồng bộ, và cách rule mới hiển thị trên dashboard.

---

## 1. Tổng quan chức năng mới

| Chức năng | Mô tả |
|-----------|--------|
| **Thêm rule (form)** | Settings → Risk Rules → Add → điền form → Verify (tùy chọn) → Save. |
| **Verify rule** | Gửi payload (JSON) lên server, server validate và trả `{ valid, errors[] }`. Không ghi DB. |
| **Sửa / Xóa rule** | Edit hoặc Delete trên từng dòng; API PUT/DELETE. |
| **Import YAML** | Paste YAML → Verify YAML (server parse + validate) → Import (server parse + validate + create). |
| **Export YAML** | Tải một rule hoặc toàn bộ rule dạng file YAML. |

Toàn bộ **validation và ghi DB** thực hiện **chỉ trên server**; Dashboard chỉ gửi dữ liệu và hiển thị kết quả/lỗi.

---

## 2. Dashboard (agent / giao diện) xử lý như thế nào

### 2.1 Trang Settings → Risk Rules

- **Khi mở tab Risk Rules:** gọi `api.getRiskRules()` → `GET /api/v1/risk/rules`.
- **Response:** `{ rules: RiskRuleItem[], total: number, source: "db" | "files" }`.
  - `source === "db"`: danh sách từ bảng `risk_rules`; hiển thị nút Add, Import YAML, Export YAML và các nút Edit/Delete/Export từng dòng.
  - `source === "files"`: danh sách đọc từ `FORTUNA_RULES_DIR` (read-only); không hiển thị Add/Edit/Delete/Import.

### 2.2 Thêm rule (form)

1. User bấm **Add rule** → mở modal, form trống (hoặc chọn template).
2. **Verify (tùy chọn):** bấm **Verify** → `api.validateRiskRule(formRule)` → `POST /api/v1/risk/rules/validate` (body JSON). Không có bước validate trên client; server trả `{ valid, errors }` → hiển thị lỗi hoặc “Rule is valid and ready to save”.
3. **Save:** bấm **Save** → `api.createRiskRule(payload)` → `POST /api/v1/risk/rules` (body JSON). Nếu server trả 400 kèm `errors`, dashboard hiển thị từng dòng lỗi trong modal; nếu thành công thì đóng modal và gọi lại `loadRiskRules()` để refresh bảng.

**Dashboard không** tự validate form trước khi gửi; mọi quyết định hợp lệ/không hợp lệ do Core trả về.

### 2.3 Import YAML

1. Bấm **Import YAML** → mở modal với textarea.
2. **Verify YAML:** paste YAML → bấm **Verify YAML** → `api.validateRiskRuleYaml(importYaml)` → `POST /api/v1/risk/rules/validate` với `Content-Type: application/x-yaml`, body là chuỗi YAML. Server parse YAML → `ValidateRule()` → trả `{ valid, errors }`; dashboard hiển thị kết quả.
3. **Import:** bấm **Import** → `api.importRiskRuleYaml(importYaml)` → `POST /api/v1/risk/rules/import` với body YAML. Server parse → validate → tạo rule (giống Create) → trả rule đã tạo hoặc 400 + errors. Thành công thì đóng modal và `loadRiskRules()`.

### 2.4 Export

- **Export YAML (tất cả):** `api.getRiskRulesExportYaml()` → `GET /api/v1/risk/rules/export` → nhận nội dung YAML → tạo Blob và trigger download `risk-rules.yaml`.
- **Export một rule:** `api.getRiskRulesExportYaml(ruleId)` → `GET /api/v1/risk/rules/export?id=<rule_id>` → download `risk-rule-<id>.yaml`.

### 2.5 Các phương thức API mà Dashboard gọi

| Phương thức Dashboard | HTTP | Endpoint Core | Mục đích |
|------------------------|------|----------------|----------|
| `api.getRiskRules()` | GET | `/api/v1/risk/rules` | Lấy danh sách rule (hiển thị bảng). |
| `api.getRiskRule(id)` | GET | `/api/v1/risk/rules/:id` | Lấy chi tiết một rule (mở Edit). |
| `api.validateRiskRule(rule)` | POST | `/api/v1/risk/rules/validate` | Verify form (body JSON). |
| `api.validateRiskRuleYaml(yaml)` | POST | `/api/v1/risk/rules/validate` | Verify YAML (body YAML, Content-Type: application/x-yaml). |
| `api.createRiskRule(rule)` | POST | `/api/v1/risk/rules` | Tạo rule mới (body JSON). |
| `api.updateRiskRule(id, rule)` | PUT | `/api/v1/risk/rules/:id` | Cập nhật rule. |
| `api.deleteRiskRule(id)` | DELETE | `/api/v1/risk/rules/:id` | Xóa (soft-delete) rule. |
| `api.importRiskRuleYaml(yaml)` | POST | `/api/v1/risk/rules/import` | Tạo rule từ YAML. |
| `api.getRiskRulesExportYaml(ruleId?)` | GET | `/api/v1/risk/rules/export` hoặc `?id=...` | Lấy YAML một hoặc toàn bộ rule. |

---

## 3. Core xử lý như thế nào

### 3.1 Đăng ký route (thứ tự quan trọng)

Trong `core/internal/api/routes_risk.go`, các route risk rules được đăng ký **trước** các route có `:id` để path cố định không bị nhầm với id:

```text
GET  /risk/rules           → GetRiskRulesList(db)
GET  /risk/rules/export    → ExportRiskRulesYAML(db)
POST /risk/rules/validate  → ValidateRiskRule()
POST /risk/rules/import    → ImportRiskRule(db)
GET  /risk/rules/:id       → GetRiskRuleByID(db)
POST /risk/rules           → CreateRiskRule(db)
PUT  /risk/rules/:id       → UpdateRiskRule(db)
DELETE /risk/rules/:id     → DeleteRiskRule(db)
```

### 3.2 Validate (POST /risk/rules/validate)

- **Handler:** `ValidateRiskRule()` (core/internal/api/risk_rules_handlers.go).
- **Body:** JSON hoặc YAML (Content-Type: application/x-yaml hoặc text/yaml).
- **Luồng:**
  1. `bindRuleFromBody(c)`: đọc body, nếu Content-Type là YAML thì `yaml.Unmarshal` vào `riskengine.Rule`, ngược lại `json.Unmarshal`.
  2. `riskengine.ValidateRule(req)`: kiểm tra id (bắt buộc, format `[a-z0-9][a-z0-9\-]{0,127}`), name, severity, category, base_score 0–10, aggregation, conditions (ít nhất một; type expression phải có expression, type resource phải có field + operator).
  3. Trả về `200` với `{ "valid": true }` hoặc `{ "valid": false, "errors": ["..."] }`. Không ghi DB.

### 3.3 Create (POST /risk/rules)

- **Handler:** `CreateRiskRule(db)`.
- **Body:** JSON (form Add rule).
- **Luồng:**
  1. `c.ShouldBindJSON(&req)` → `riskengine.Rule`.
  2. `riskengine.ValidateRule(&req)` → nếu có lỗi trả 400 + `errors`.
  3. `riskengine.RuleToRiskRule(&req)` → `models.RiskRule`.
  4. `db.Create(m)` → ghi bảng `risk_rules`.
  5. `riskengine.ReloadGlobalFromDB()` → engine (đã đăng ký bởi RiskWorker) reload danh sách rule từ DB.
  6. Nếu `riskengine.GetRiskRulesExportDir()` không rỗng (FORTUNA_RULES_DIR/risk): `ExportRuleToFile(&req, dir)` → ghi file `<rule_id>.yaml`.
  7. Trả 201 + JSON rule (riskRuleToAPI).

### 3.4 Import (POST /risk/rules/import)

- **Handler:** `ImportRiskRule(db)`.
- **Body:** YAML (hoặc JSON, theo Content-Type).
- **Luồng:** Giống Create: `bindRuleFromBody` → `ValidateRule` → `RuleToRiskRule` → `db.Create` → `ReloadGlobalFromDB()` → export file (nếu có dir) → trả 201.

### 3.5 Update (PUT /risk/rules/:id) và Delete (DELETE /risk/rules/:id)

- **Update:** Bind JSON → build `riskengine.Rule` với id từ URL → `ValidateRule` → tìm bản ghi theo `rule_id` → `db.Save` → ReloadGlobalFromDB → ExportRuleToFile (ghi đè file YAML).
- **Delete:** `db.Delete(&models.RiskRule{}, "rule_id = ?", id)` (soft-delete) → ReloadGlobalFromDB → `RemoveRuleFile(dir, id)` (xóa file YAML nếu có).

### 3.6 List (GET /risk/rules) – nguồn dữ liệu cho bảng dashboard

- **Handler:** `GetRiskRulesList(db)`.
- **Luồng:**
  1. Nếu DB có bảng `risk_rules`: query `WHERE deleted_at IS NULL ORDER BY rule_id`, map từng row sang `riskengine.RiskRuleSummary` (id, name, severity, description, category, enabled), trả `{ rules: list, total: len(list), source: "db" }`.
  2. Nếu không (hoặc không dùng DB): đọc từ `FORTUNA_RULES_DIR` qua `riskengine.ListRuleSummariesFromDir`, trả `{ rules: list, total: len(list), source: "files" }`.
- **Dashboard hiển thị:** Bảng với cột ID, Name, Severity, Category, Status (Enabled/Disabled); khi `source === "db"` thì thêm cột Actions (Export, Edit, Delete). Dữ liệu chính là `data.rules` từ response trên.

### 3.7 Get by ID (GET /risk/rules/:id) và Export YAML (GET /risk/rules/export)

- **Get by ID:** Query `risk_rules` theo `rule_id`, trả JSON đầy đủ (conditions, aggregation, base_score, tags, …) qua `riskRuleToAPI`.
- **Export:** Nếu có `?id=` thì query một rule và marshal sang YAML một object; không có id thì query tất cả và marshal mảng rule. Trả `Content-Type: application/x-yaml`, header `Content-Disposition: attachment; filename=risk-rules.yaml`.

---

## 4. Lưu trữ rule trong database và hiển thị lên dashboard

### 4.1 Bảng `risk_rules` (PostgreSQL)

- **Migration:** `core/migrations/075_add_risk_rules_table.go` → `db.AutoMigrate(&models.RiskRule{})`.
- **Model:** `core/pkg/models/risk_rule.go`:
  - `id` (PK), `rule_id` (unique, logical id), `name`, `category`, `severity`, `description`, `enabled`, `conditions` (JSON text), `aggregation`, `base_score`, `tags` (JSON text), `created_at`, `updated_at`, `deleted_at`.
- Rule mới thêm qua Create hoặc Import đều được ghi vào bảng này (soft-delete khi Delete).

### 4.2 Cách rule mới hiển thị lên dashboard

1. Sau khi Create/Update/Import/Delete thành công, dashboard gọi lại `api.getRiskRules()` (hoặc tương đương `loadRiskRules()`).
2. Core xử lý GET /risk/rules: đọc từ bảng `risk_rules` (deleted_at IS NULL), trả về mảng summary.
3. Dashboard nhận `{ rules, total, source: "db" }`, set state `setRiskRules(data.rules)`, `setRiskRulesSource(data.source)`.
4. Component render bảng: `riskRules.map(r => ...)` với các cột ID (`r.id`), Name (`r.name`), Severity (`r.severity`), Category (`r.category`), Status (`r.enabled`), và khi source là db thì cột Actions (Export, Edit, Delete).

Rule mới xuất hiện ngay trong bảng vì danh sách luôn lấy từ API (DB) sau mỗi thao tác thành công.

---

## 5. Đồng bộ và cơ chế

### 5.1 DB ↔ Engine (có hiệu lực ngay)

- **Khi khởi động:** RiskWorker (và/hoặc HistoricalRiskEvaluator) tạo `riskengine.Engine` (hoặc YAMLEngine), load rule từ DB qua `LoadRulesFromDB(db)`; engine được đăng ký toàn cục qua `riskengine.RegisterEngine(engine)`.
- **Sau Create/Update/Delete:** Handler gọi `riskengine.ReloadGlobalFromDB()` → engine đã đăng ký gọi `ReloadFromDB()`: đọc lại toàn bộ rule từ bảng `risk_rules` và gán vào `engine.rules` (có mutex). Lần đánh giá risk tiếp theo (RiskWorker.Process hoặc historical) dùng bộ rule mới → **rule mới có hiệu lực ngay**, không cần restart Core.

### 5.2 DB ↔ Thư mục (FORTUNA_RULES_DIR/risk)

- **Write-through:** Mỗi khi Create/Update thành công, Core ghi thêm (hoặc ghi đè) file `FORTUNA_RULES_DIR/risk/<rule_id>.yaml`. Khi Delete thành công, Core xóa file tương ứng. Mục đích: lưu trữ lâu dài, backup, deploy/rebuild (volume hoặc git-sync).
- **Seed khi khởi động:** Trong `core/cmd/main.go`, sau migration gọi `riskengine.SeedRiskRulesFromExportDir(db)`: nếu bảng `risk_rules` **trống** và thư mục `FORTUNA_RULES_DIR/risk` tồn tại và có file `.yaml`, thì đọc từng file → parse YAML → `RuleToRiskRule` → `db.Create`. Nhờ vậy sau deploy mới hoặc DB mới, rule trong folder được khôi phục vào DB và sau đó hiển thị như bình thường qua GET /risk/rules.

### 5.3 Tóm tắt luồng dữ liệu

```text
[User] Add/Edit/Import (Dashboard)
    → POST/PUT /risk/rules hoặc POST /risk/rules/import (Core)
    → ValidateRule (server)
    → db.Create / db.Save (risk_rules)
    → ReloadGlobalFromDB() (engine dùng rule mới)
    → ExportRuleToFile (FORTUNA_RULES_DIR/risk/<id>.yaml)

[User] Mở / refresh Risk Rules (Dashboard)
    → GET /risk/rules (Core)
    → GetRiskRulesList: đọc risk_rules (hoặc fallback files)
    → { rules, total, source }
    → Bảng hiển thị rules, nút Add/Edit/Delete/Import/Export khi source === "db"

[RiskWorker / HistoricalRiskEvaluator]
    → Engine.EvaluateResource(...)
    → engine.rules (đã load từ DB, reload sau mỗi CRUD)
    → Tạo insights lưu DB → Dashboard hiển thị ở màn Risk/Insights
```

---

## 6. Các phương thức nội bộ Core (tóm tắt)

| Thành phần | Phương thức / Hàm | Mục đích |
|------------|-------------------|----------|
| **risk_rules_handlers** | `GetRiskRulesList`, `GetRiskRuleByID`, `ValidateRiskRule`, `CreateRiskRule`, `UpdateRiskRule`, `DeleteRiskRule`, `ImportRiskRule`, `ExportRiskRulesYAML` | API HTTP. |
| **risk_rules_handlers** | `bindRuleFromBody`, `isYAMLContentType`, `riskRuleToAPI`, `riskRuleToRule` | Parse body, convert DB ↔ API/engine. |
| **riskengine** | `ValidateRule(rule)` | Validate id, name, severity, category, base_score, conditions, aggregation. |
| **riskengine** | `RuleToRiskRule`, `LoadRulesFromDB`, `riskRuleToRule` (db_rules.go) | Chuyển đổi Rule ↔ models.RiskRule, load từ DB. |
| **riskengine** | `ReloadFromDB` (Engine), `ReloadFromDB` (YAMLEngine) | Cập nhật lại danh sách rule trong engine từ DB. |
| **riskengine** | `RegisterEngine`, `ReloadGlobalFromDB` | Đăng ký engine toàn cục và gọi reload sau CRUD. |
| **riskengine** | `GetRiskRulesExportDir`, `ExportRuleToFile`, `RemoveRuleFile`, `LoadRiskRulesFromExportDir`, `SeedRiskRulesFromExportDir` | Đồng bộ DB ↔ thư mục YAML (export/seed). |
| **worker** | `NewRiskWorker` → `riskengine.RegisterEngine(engine)` | Đăng ký engine khi start worker. |
| **main** | `riskengine.SeedRiskRulesFromExportDir(tempDB)` | Seed rule từ folder vào DB khi DB trống. |

---

## 7. Kết luận

- **Thêm/Verify rule từ giao diện:** Toàn bộ kiểm tra và ghi dữ liệu thực hiện trên Core; Dashboard chỉ gửi payload (form JSON hoặc YAML) và hiển thị kết quả/lỗi từ server.
- **Rule mới được lưu:** Vào bảng PostgreSQL `risk_rules` và (khi cấu hình) vào thư mục `FORTUNA_RULES_DIR/risk/` dạng file YAML.
- **Hiển thị lên dashboard:** Qua GET /api/v1/risk/rules → Core đọc từ `risk_rules` (hoặc fallback files) → trả `{ rules, total, source }` → bảng Settings → Risk Rules render từ `rules`; rule mới xuất hiện ngay sau khi thao tác thành công và refresh list.
- **Đồng bộ:** (1) Sau mỗi CRUD, engine reload từ DB để rule có hiệu lực ngay; (2) Write-through DB → folder; (3) Khi khởi động, nếu DB trống thì seed từ folder vào DB.

Tài liệu chi tiết luồng từng bước đã nêu trong các mục trên và trong `RISK_RULES_FLOW.md`.
