# Báo cáo lịch sử commit và lịch sử xóa file

**Ngày kiểm tra:** 2026-03-09

---

## 1. Mục đích

Kiểm tra lịch sử commit và lịch sử xóa file để làm rõ **tại sao một số nội dung đã thực hiện bị xóa mất** mà không rõ lý do. Báo cáo này tổng hợp các commit có xóa file (D) và giải thích ngữ cảnh.

---

## 2. Các commit chính có xóa file (theo thời gian gần → xa)

### 2.1 `98ac65dec` – 2026-03-02 – "Sync docs/deploy: remove duplicate deploy files, add docs-prod/script-prod, helm fortuna"

**Lý do ghi trong commit:** Dọn deploy trùng lặp, thêm docs-prod và script-prod, Helm fortuna.

**File bị xóa (trong commit):**
- **deploy:** `agent-rbac.yaml`, `core-service.yaml` (thay bằng fortuna-rbac.yaml và Service trong fortuna-core-deployment.yaml).
- **docs:** `docs/01-getting-started/MINIKUBE_SETUP.md`, nhiều file trong `docs/test-results/` (e2e-api-results-*, e2e-logs-*).
- **helm:** Toàn bộ `helm/ksam/` (Chart, templates, values) và `helm/README.md`, `helm/.DS_Store`; thay bằng chart `helm/fortuna`.

**Kết luận:** Xóa có chủ đích (refactor deploy/Helm, dọn test-results). Không liên quan Pod Detail.

---

### 2.2 `ca3e66179` – 2026-02-25 – "full refactor"

**File bị xóa (một phần danh sách):**
- **core/migrations:** `015_add_policy_instances.sql`, `016_add_policy_violations.sql`, `mvp2/006_add_sbom_tables.sql` (có thể đã chuyển sang Go migrations).
- **cve-data/all/:** Rất nhiều file CVE-*.json (CVE-1999-* đến CVE-2004-*). Có thể do đổi cấu trúc thư mục hoặc dùng nguồn CVE khác.

**Kết luận:** Refactor lớn; migrations và CVE data bị xóa/đổi cấu trúc. Không thấy xóa doc Pod Detail trong commit này.

---

### 2.3 `0630283af` – 2026-02-02 – "chore: cleanup and consolidate repo"

**Lý do ghi trong commit:** Cleanup và consolidate repo; xóa backup cũ, archive migrations, cấu trúc dashboard cũ; thêm PCE, agent, docs, scripts, deploy mới.

**File bị xóa (nhóm chính):**
- **Backup .OLD:** agent/internal/client/*.OLD, agent/internal/watcher/*.OLD, core/pkg/riskengine/insight_manager.go.OLD.
- **core/migrations/archive:** toàn bộ thư mục (age, mvp2, mvp3).
- **dashboard/src/:** Toàn bộ cấu trúc cũ (App.tsx, main.tsx, components, pages, lib, store, …) – **thay bằng dashboard mới** (App, components, pages, lib, store ở root dashboard/).
- **deploy:** grafana, monitoring (prometheus, grafana-cert).

**Kết luận:** Đây là lần “đổi cấu trúc” lớn: dashboard chuyển từ `dashboard/src/` sang cấu trúc mới. Nội dung cũ (kể cả có thể có tài liệu tham chiếu implementation) nếu nằm trong `docs/` cũ hoặc file chưa được add vào git có thể mất theo. **Pod_Detail_Implementation_Reference.md không xuất hiện trong danh sách file từng được track trong git** (xem mục 4).

---

### 2.4 `36bb23066` – 2026-01-06 – "repo: Remove development/debug files from git tracking"

**Lý do ghi trong commit:** "Removed from git (kept local)" – bỏ track các file dev/debug, **giữ bản local**.

**File bị xóa khỏi git (untrack):**
- CLEANUP_SUMMARY.md, HELM_CHART_UPDATE.md, PROJECT_CLEANUP_ANALYSIS.md.
- docker-compose.*.
- **Nhiều script:** apply-core-master-only.sh, apply-nats-single-replica.sh, build-and-import-containerd.sh, build-with-containerd.sh, clean-*, copy-containerd-images-to-nodes.sh, create-mtls-secrets.sh, **deploy-fortuna-robust.sh**, fix-dns-issues.sh, import-to-containerd.sh, **load-cve-database.sh**, quick-deploy-containerd.sh, validate-migrations.sh, v.v.

**Kết luận:** Script và một số file bị **git rm** (bỏ track) với ý “giữ local”. Sau đó các script được đưa lại vào repo nhưng **đường dẫn thay đổi** (ví dụ từ `scripts/deploy-fortuna-robust.sh` sang `scripts/deploy/deploy-fortuna-robust.sh`). Nếu ai đó xóa file local hoặc clone repo mới, các file đã untrack sẽ **không còn** trong working tree. Đây là một nguyên nhân khiến “nội dung đã làm bị mất” nếu không còn bản local.

---

### 2.5 `6dbbf64fd` – 2025-12-23 – "clean and clear document"

**File bị xóa (docs):** Rất nhiều, gồm:
- docs/02-architecture: ARCHITECTURE_OLD, CORE_ONLY_ANALYSIS, EXECUTIVE_SUMMARY, **IMPLEMENTATION_CODE_EXAMPLES**, KSAM_ADR_FULL, REFACTORING_PLAN_AGENT_BASED, changelog.
- **docs/03-components/INDEX.md.**
- docs/09-archive: toàn bộ (agent, cleanup, cve, implementation/setup/*.md, old-archive, organization, sbom). Trong đó có nhiều **IMPLEMENTATION*.md** (IMPLEMENTATION_PLAN, IMPLEMENTATION_ROADMAP, MTLS_IMPLEMENTATION, MVP2_*, PHASE2_*, RATE_LIMITING_IMPLEMENTATION, …).

**Kết luận:** Đây là đợt “clean and clear document” – xóa doc cũ/archive và một số file implementation. **Nếu từng có file tên dạng Pod_Detail_Implementation_Reference.md hoặc tương tự** mà nằm trong thư mục bị xóa (ví dụ docs/03-components hoặc docs/09-archive), nó có thể đã bị xóa trong commit này. Trong output `git log --name-only` cho các commit xóa docs, **không có tên chính xác "Pod_Detail_Implementation_Reference.md"**; có thể tên khác hoặc nằm trong thư mục bị xóa hàng loạt.

---

### 2.6 Các commit khác có xóa docs

- **a5020269e, 7988fc383, 032901721:** Xóa nhiều doc deployment, migration, CVE, reference (DEPLOYMENT_SUMMARY, MIGRATION_*_FIX, CVE_*, E2E_*, …).
- **a6cbc43ea:** "docs+scripts: Complete v2.0 restructure" – xóa docs/architecture, docs/development, docs/getting-started (cấu trúc cũ).

---

## 3. Tại sao nội dung “bị xóa mất mà không rõ lý do”

Tổng hợp lại, có thể giải thích như sau:

| Nguyên nhân | Mô tả |
|-------------|--------|
| **1. Xóa có chủ đích trong commit** | Một số commit (98ac65dec, ca3e66179, 0630283af, 6dbbf64fd) **cố ý** xóa file: refactor deploy/Helm, đổi cấu trúc dashboard, “clean and clear document”, bỏ archive/implementation cũ. Nội dung nằm trong các file đó sẽ mất trên repo sau khi commit. |
| **2. Bỏ track nhưng “giữ local” (36bb23066)** | Nhiều script và file bị **git rm** (untrack) với ý “kept local”. Trên máy khác hoặc sau khi xóa local / clone mới, các file này **không còn** trong repo và cũng không có trong working tree. |
| **3. File chưa bao giờ commit** | Nếu **Pod_Detail_Implementation_Reference.md** (hoặc tài liệu tương tự) chỉ tồn tại trên máy local và chưa từng `git add`/`git commit`, thì nó **không có trong lịch sử git**. Mất ổ cứng, xóa nhầm, hoặc làm trên máy khác sẽ làm mất vĩnh viễn. |
| **4. Tên hoặc đường dẫn khác** | File có thể từng tồn tại với tên/đường dẫn khác (ví dụ nằm trong docs/09-archive/implementation hoặc docs/03-components với tên khác) và bị xóa trong đợt dọn docs (6dbbf64fd, 0630283af). |
| **5. Cấu trúc thư mục thay đổi** | Dashboard và docs đổi cấu trúc (dashboard/src → dashboard/; docs theo 01–09); file tham chiếu implementation có thể nằm trong đường dẫn cũ và bị xóa hoặc không còn được tham chiếu. |

---

## 4. Kiểm tra cụ thể: Pod_Detail_Implementation_Reference.md

- **Kết quả tìm trong toàn bộ lịch sử git:** **Không có** file nào tên chính xác `Pod_Detail_Implementation_Reference.md` trong `git log --all --name-only`.
- **Các file Pod Detail từng xuất hiện trong git:**  
  `core/internal/service/agent_service_pod_detail_test.go`, `core/migrations/068_add_pod_detail_columns.go`, `dashboard/pages/PodDetail.tsx`, `scripts/verify/verify-pod-detail-api-and-db.sh`.
- **Tài liệu Pod Detail hiện có trong repo:**  
  `docs/03-components/podDetail/POD_DETAIL_SPEC.md`, `POD_DETAIL_SERVICES_DESIGN.md`, `POD_SYNC_ARCHITECTURE_AND_DATA_MODEL_SPEC.md`, `podDetail_page_UIUX.md`, `podDetail_QA.md`, `testSuite.md`.

**Kết luận:**  
File **Pod_Detail_Implementation_Reference.md** có thể (1) chưa từng được commit, hoặc (2) từng tồn tại với tên/đường dẫn khác và bị xóa trong đợt “clean and clear document” / restructure. Để không mất nội dung tham chiếu implementation, nên **tái tạo** tài liệu từ các file hiện có (POD_DETAIL_SPEC, POD_DETAIL_SERVICES_DESIGN, POD_SYNC_ARCHITECTURE_AND_DATA_MODEL_SPEC, podDetail_page_UIUX, testSuite) và lưu thành một file tên rõ ràng (ví dụ `Pod_Detail_Implementation_Reference.md`) rồi commit.

---

## 5. Khuyến nghị

1. **Trước khi xóa hàng loạt:** Chạy `git status` và `git diff --name-only` để xem file nào sẽ mất; với doc/script quan trọng thì merge nội dung cần giữ vào file khác hoặc chuyển vào archive có chủ đích trước khi xóa.
2. **Tránh “Remove from git (kept local)” với file quan trọng:** Nếu bỏ track, file sẽ mất trên mọi clone/branch mới. Chỉ nên untrack khi thực sự chỉ dùng local và không cần chia sẻ qua repo.
3. **Pod Detail:** Tạo lại `Pod_Detail_Implementation_Reference.md` (hoặc tên tương đương) tổng hợp spec, services design, sync architecture, UI/UX và test suite, rồi đặt trong `docs/03-components/podDetail/` và commit.
4. **Backup / tag:** Trước các đợt refactor lớn (full refactor, clean document), tạo tag hoặc branch backup để có thể so sánh và khôi phục file nếu cần.

---

*Báo cáo dựa trên `git log --diff-filter=D` và `git show <commit> --name-status`; có thể bổ sung thêm commit nếu cần làm rõ thêm từng đợt xóa.*
