# Kiểm tra công việc (Tasks 1–7) và Clean Build / Deploy

Tài liệu này tóm tắt các thay đổi đã thực hiện tới Task 7 (theo phân tích UI từ các tài liệu FORTUNA) và cách chạy **clean build + deploy** để kiểm tra.

---

## 1. Trạng thái các task đã hoàn thành

| # | Task | Trạng thái | Ghi chú |
|---|------|------------|--------|
| 1 | Exploitability / Business Impact / Time Decay (Risk Center) | ✅ | API + sort + badge + drawer header; verify trên cluster thật. |
| 2 | SBOM/CVE severity distribution endpoint | ✅ | Đã có sẵn qua `GET /inventory/pods/:uid/sbom` (vulnerabilitySummary). |
| 3 | SBOM/CVE frontend charts | ✅ | PodDetail đã có bar severity; Risk Center dùng histogram. |
| 4 | Schema risk_explanation + remediation | ✅ | Migration 077, model Insight, DB columns. |
| 5 | API + UI explanation/remediation | ✅ | GetInsight trả field; Risk Detail drawer có Summary + Recommended Actions. |
| 6 | Cross-resource context endpoint | ✅ | `GET /risk/insights/:id/context` (pods, cluster, rules); unit test SQLite. |
| 7 | Cross-resource navigation UI | ✅ | Risk Center → Pod/Cluster/Rule; PodDetail/SBOM → Risk Center. |

---

## 2. File / thay đổi chính cần kiểm tra

### Backend (Core)

- `core/internal/api/dashboard_handlers.go` – InsightWithScore (exploitability, businessImpact, timeDecay); getInsightsListData(hasScoreBin); ExportRisksCSV(hasScoreBin).
- `core/internal/api/insights_handlers.go` – GetInsightContext, InsightContextResponse.
- `core/internal/api/routes_risk.go` – `GET /risk/insights/:id/context`.
- `core/internal/api/risks_export_test.go` – AutoMigrate RiskScore; test không dùng clusterId.
- `core/internal/api/risks_list_with_scores_test.go` – assert V2 fields.
- `core/internal/api/insights_context_test.go` – TestGetInsightContext_PodWithRule, TestGetInsightContext_NotFound.
- `core/pkg/models/implementation_guide.go` – RiskExplanation, Remediation trên Insight.
- `core/migrations/077_add_insight_explanation_remediation.go` – risk_explanation, remediation.
- `core/migrations/migrations.go` – đăng ký Migration077.

### Frontend (Dashboard)

- `dashboard/lib/api.ts` – getInsightContext; getInsight (riskExplanation, remediation); getRisks map (exploitabilityScore, businessImpactScore, timeDecay).
- `dashboard/types.ts` – Insight: riskExplanation, remediation, exploitabilityScore, businessImpactScore, timeDecay.
- `dashboard/pages/Insights.tsx` – riskSort (exploitability_desc/asc); sortedRisks; useScores; drawer Summary (explanation, remediation, Context Links); selectedRiskContext, getInsightContext trong load.
- `dashboard/pages/PodDetail.tsx` – nút "View related risks in Risk Center" (navigate `/risks?resourceNamespace=&search=`).
- `dashboard/pages/Sbom.tsx` – useNavigate; nút "View related risks" trong actions card SBOM (navigate `/risks?resourceNamespace=&search=`).

### Docs

- `docs/02-architecture/DASHBOARD_UX_API_ARCHITECT_AUDIT.md` – dòng GET /risk/insights/:id/context.
- `docs/02-architecture/UI_Audit_Questionnaire.md` – Risk Detail wireframe (context links, remediation).

---

## 3. Clean build (trước khi deploy)

```bash
# Từ thư mục gốc repo
cd /home/k8s/KSAM

# Core: build + test
cd core && go build ./cmd/... && go test ./... && cd ..

# Dashboard: build (cần Node/npm; nếu không có thì bỏ qua, deploy dùng image đã build)
# cd dashboard && npm ci && npm run build && cd ..
```

- Core: `go build ./cmd/...` và `go test ./...` đã chạy **thành công** (đã verify trong session).
- Sbom.tsx: đã bổ sung `useNavigate` từ `react-router-dom` và `const navigate = useNavigate()` để nút "View related risks" hoạt động.

---

## 4. Clean build + deploy (pipeline đầy đủ)

Script chính: **`scripts/pipeline/full-clean-database-rebuild-deploy.sh`**

**Chạy có menu (khuyến nghị):**

```bash
cd /home/k8s/KSAM
./scripts/pipeline/full-clean-database-rebuild-deploy.sh
# hoặc
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --menu
```

Chọn:

- **1** – Full: clean images + rebuild (core, agent, dashboard) + deploy + rollout restart.
- **2** – Full + Clear DB (DELETE data, giữ schema).
- **3** – Full + Reset DB (DROP tables; Core chạy lại migrations khi start).
- **7** – Chỉ reset DB (không clean/rebuild/deploy).

**Chạy không menu (trực tiếp):**

```bash
# Full clean + rebuild + deploy
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --full

# Full + reset DB (schema mới, có migration 077)
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --full --db-reset
```

Pipeline thực hiện:

1. **Clean** – port-forward, namespace E2E, xóa image fortuna (nerdctl), prune.
2. **DB (tùy chọn)** – clear data hoặc reset full (reset_database_full.sql).
3. **Rebuild** – `scripts/build/build-and-load-containerd.sh` (core, agent, dashboard → nerdctl → containerd).
4. **Push images** – `scripts/utils/push-images-to-workers.sh` (nếu có cấu hình) để tránh ErrImageNeverPull.
5. **Deploy** – addons, Flannel, StorageClass, `scripts/deploy/deploy-fortuna-robust.sh`, rollout restart Core/Dashboard/Agent.

Chạy nền (tránh timeout):

```bash
RUN_ASYNC=1 ./scripts/pipeline/full-clean-database-rebuild-deploy.sh --full
tail -f /tmp/clean-rebuild-deploy.log
```

---

## 5. Sau khi deploy – kiểm tra nhanh

1. **Core API**
   - `GET /api/v1/risk/insights?withScores=1` – response có `totalScore`, `priorityLevel`, `exploitabilityScore`, `businessImpactScore`, `timeDecay`.
   - `GET /api/v1/risk/insights/:id/context` – có `insight`, `pods`, `cluster`, `rules`.

2. **Dashboard**
   - Risk Center: sort "Exploitability high to low"; mở 1 risk → drawer Summary có "Context Links" (View pod, View cluster, Rule); có "Recommended Actions" nếu backend trả remediation.
   - PodDetail: card Pod detail (agent) có nút "View related risks in Risk Center" → sang `/risks` với filter.
   - SBOM: chọn 1 pod, card SBOM có nút "View related risks" → sang `/risks` với filter.

3. **DB**
   - Migration 077 đã chạy: bảng `insights` có cột `risk_explanation`, `remediation` (kiểm tra trong PostgreSQL nếu cần).

---

*Cập nhật theo trạng thái tới Task 7; các task 8–10 (layout chuẩn hóa, E2E Risk Center, docs tổng hợp) thực hiện tiếp sau.*
