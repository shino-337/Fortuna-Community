# E2E Test Cases và Runner – Tổng hợp theo chức năng và luồng code hiện tại

Tài liệu này mô tả **toàn bộ test E2E** của Fortuna, đúng với **chức năng và luồng code hiện tại**. Một script runner duy nhất: **`scripts/e2e/run-e2e.sh`**.

---

## 1. Cách chạy

```bash
# Từ repo root (kubectl trỏ tới cluster có namespace fortuna)
./scripts/e2e/run-e2e.sh                    # Mặc định: suite full (cluster + API + Risk Center 17 TCs + priority1)
./scripts/e2e/run-e2e.sh --suite=risk-center   # Chỉ Risk Center (17 TCs)
./scripts/e2e/run-e2e.sh --suite=priority1    # Chỉ Priority 1 APIs
./scripts/e2e/run-e2e.sh --suite=runtime      # Runtime signals E2E
./scripts/e2e/run-e2e.sh --suite=pce          # PCE + promotion flow
./scripts/e2e/run-e2e.sh --suite=full-report  # Báo cáo chi tiết (E2E-FULL + capability)
NAMESPACE=my-ns ./scripts/e2e/run-e2e.sh
```

**Yêu cầu:** Core pod Running trong namespace (mặc định `fortuna`). JWT: admin / admin123 (hoặc FORTUNA_E2E_USER / FORTUNA_E2E_PASSWORD).

---

## 2. Các suite và test case (đúng luồng code)

### 2.1. Risk Center (17 TCs) – `--suite=risk-center`

Script: `scripts/e2e/e2e-risk-center-full.sh`.  
Báo cáo: `test-results/risk-center-e2e-YYYYMMDD-HHMMSS.md`.

| ID | Test case | API / Luồng code | Điều kiện |
|----|-----------|-------------------|-----------|
| TC-01 | GET /risks (list, pagination) | `GetInsightsListCached` → join insights (+ risk_scores nếu withScores=1) | 200, JSON có insights[], total |
| TC-02 | GET /risks?withScores=1 | Cùng handler, query withScores=1 → totalScore, priorityLevel từ risk_scores | 200 |
| TC-03 | GET /risks?priorityLevel=P0 | Filter theo priorityLevel (P0–P4) | 200 |
| TC-04 | GET /insights/summary | GetInsightsSummaryCached → getInsightsSummaryData (cluster hoặc global) | 200, total/critical/high/medium/low |
| TC-05 | GET /insights/summary/by-cluster | GetInsightsSummaryByCluster → GROUP BY cluster_id | 200, byCluster[] |
| TC-05b | GET /insights/summary/global | GetInsightsSummaryGlobalCached → getInsightsSummaryData("", sinceMinutes) | 200, total/... |
| TC-06 | GET /risks/export | ExportRisksCSV → CSV attachment | 200, body CSV |
| TC-07 | GET /risks/export?format=pdf | Cùng handler, format=pdf → HTML in PDF | 200, body HTML |
| TC-08 | GET /risk-rules | GetRiskRulesList → DB risk_rules hoặc FORTUNA_RULES_DIR (source=db|files) | 200, rules[], total, source |
| TC-09 | GET /risk-rules/:id | GetRiskRuleByID (rule_id) | 200 hoặc SKIP nếu không có rule |
| TC-10 | POST /risk-rules | CreateRiskRule → RuleToRiskRule, db.Create (chỉ khi source=db) | 201; script xóa rule cũ trước để tránh duplicate |
| TC-11 | PUT /risk-rules/:id | UpdateRiskRule (chỉ khi source=db) | 200 |
| TC-12 | DELETE /risk-rules/:id | DeleteRiskRule soft-delete (chỉ khi source=db) | 200/204 |
| TC-13 | GET /pod-capabilities/trends | GetPodCapabilitiesTrend (7 ngày) | 200 |
| TC-14 | GET /pod-capabilities/summary/namespace | GetPodCapabilitiesSummaryByNamespace (heatmap) | 200 |
| TC-15 | GET /runtime-signals | Runtime signals (tab Reference) | 200 |
| TC-16 | WebSocket GET /ws/risks | RisksWS → 101/400/401 | Endpoint tồn tại |

Luồng: Core API (Gin) → GORM → PostgreSQL; cache 60s cho /risks và /insights/summary(/global). Risk rules: bảng risk_rules (migration 075) hoặc YAML từ env.

---

### 2.2. Priority 1 APIs – `--suite=priority1`

Script: `scripts/e2e/test-priority1-apis.sh`.  
Các API: promotion-rules, promotion-rules/capability/..., promotion-rules/signal/..., runtime-signals, runtime-signals/pods/:podUid, agents/status, clusters, dashboard/stats, insights/summary.

Luồng: JWT từ auth/login → curl từ Core pod tới localhost:8080.

---

### 2.3. Runtime signals E2E – `--suite=runtime`

Script: `scripts/e2e/test-runtime-signals-e2e.sh`.  
1) POST /runtime-events (ingest); 2) Kiểm tra DB runtime_events, runtime_signals; 3) GET /runtime-signals; 4) GET /runtime-signals/pods/:podUid.

Luồng: Ingest → Core lưu DB → GET trả về từ DB.

---

### 2.4. PCE & Promotion – `--suite=pce`

Scripts: `test-pce-e2e.sh`, `test-pce-api.sh`, `test-promotion-flow.sh`.  
Nội dung: Pod capabilities (sync từ Agent), promotion rules (capability/signal), API pod-capabilities, runtime-signals theo pod.

Luồng: Agent sync pod → Core cập nhật pod_capabilities; runtime_events → runtime_signals; promotion_rules từ DB/capability_metadata.

---

### 2.5. Dashboard data (threat-velocity, PCE trend) – `--suite=dashboard`

Script: `scripts/e2e/e2e-dashboard-data.sh`.  
Tạo pod (privileged), đợi sync Core, POST runtime-events, (tùy chọn) evaluate insights; kiểm tra GET threat-velocity, GET pod-capabilities/trends.

Luồng: Pod → Agent → Core (pods, pod_capabilities); runtime-events → DB; dashboard API đọc từ insights/pod_capabilities.

---

### 2.6. SBOM & Pod flow – `--suite=sbom`

Scripts: `e2e-sbom-verify.sh`, `test-sbom-pod-flow.sh`.  
Kiểm tra pod có trong API /pods, /sbom; (tùy chọn) full flow: tạo pod → đợi SBOM từ Agent → CVE match → insights.

Luồng: Agent gRPC SubmitSBOM → Core lưu SBOM; CVE matcher worker → insights.

---

### 2.7. Pod Detail (metrics, processes, network) – `--suite=pod-detail`

Scripts: `test-pod-detail-ping-flow.sh`, `test-pod-detail-lodash-network.sh`, `run-pod-detail-test-suite.sh`.  
Unit tests (Spec Hash, PCE, Pod Detail fields) + DB verification + Pod Detail API (runtime-metrics, processes, network).

Luồng: Agent gửi metrics/processes/network lên Core (pod_uid) → pod_runtime_metrics, pod_processes, pod_network_connections; GET /pods/by-uid/:uid/runtime-metrics, /processes, /network-connections, /events, /spec, /capabilities (pod-scoped APIs dùng UID thống nhất).

---

### 2.8. Full report (cluster + API + DB + Dashboard + scripts) – `--suite=full-report`

Script: `run-e2e-full.sh` (ghi E2E-FULL-*.md) hoặc `run-e2e-with-capability-report.sh` (ghi E2E-WITH-CAPABILITY-*.md).  
Nội dung: Cluster/pods/services, Core API (JWT, nhiều endpoint), DB (Postgres row counts), Dashboard URL, test-pce-api, test-priority1-apis, (capability) PCE/promotion/runtime-signals.

---

## 3. File script được giữ lại (sau dọn dẹp)

| Script | Vai trò |
|--------|---------|
| **run-e2e.sh** | Entry point duy nhất; nhận --suite=... và gọi script tương ứng. |
| **common.sh** | Helper: NAMESPACE, get_core_pod, get_jwt_token, core_api_get, wait_for_pod_in_db. |
| **e2e-risk-center-full.sh** | Risk Center 17 TCs (API only). |
| **e2e-risk-center-verify.sh** | Tùy chọn: seed 1 insight E2E rồi verify /risks, /insights/summary (dùng khi cần data có sẵn). |
| **test-priority1-apis.sh** | Priority 1 APIs (promotion-rules, runtime-signals, agents, clusters, dashboard/stats, …). |
| **test-runtime-signals-e2e.sh** | Runtime signals: POST runtime-events, DB, GET runtime-signals. |
| **test-pce-e2e.sh** | PCE E2E (dùng common.sh). |
| **test-pce-api.sh** | PCE API tests. |
| **test-promotion-flow.sh** | Pod capabilities, runtime signals, promotion rules. |
| **e2e-dashboard-data.sh** | Pod + sync + runtime-events, verify threat-velocity, pod-capabilities/trends. |
| **e2e-sbom-verify.sh** | Pod trong API/SBOM; optional full SBOM flow. |
| **test-sbom-pod-flow.sh** | Full SBOM flow: tạo pod, đợi SBOM, gọi /sbom. |
| **e2e-pod-delete-cleanup-verify.sh** | APIs 200, pod count consistency, tạo pod → xóa → kiểm tra lại. |
| **test-dashboard-consistency-e2e.sh** | Dashboard consistency. |
| **test-runtime-probe-e2e.sh** | Runtime probe. |
| **test-pod-detail-ping-flow.sh** | Pod Detail metrics. |
| **test-pod-detail-lodash-network.sh** | Pod Detail network (image website-vuln-lodash). |
| **run-e2e-full.sh** | Báo cáo chi tiết: cluster, Core API, DB, Dashboard, test-pce-api, test-priority1. |
| **run-e2e-with-capability-report.sh** | Báo cáo theo từng bước + capability rules, PCE, runtime. |
| **run-pod-detail-test-suite.sh** | Unit + DB + Pod Detail mapping. |
| **run-dashboard-data-tests.sh** | Chuỗi: CVE load (opt), dashboard data E2E, PCE E2E, SBOM flow (opt). |

---

## 4. File đã loại bỏ (outdated / trùng)

| File | Lý do |
|------|--------|
| **run-e2e-tests.sh** | Tham chiếu doc cũ (End-to-end-testcase-verify-05012026.md), chứa inline Go SBOM injector (~638 dòng); thay bằng run-e2e.sh + e2e-risk-center-full + run-e2e-full. |
| **run-e2e-complete-with-monitor.sh** | Trùng với run-e2e-all-verify (full + priority1 + runtime + dashboard + risk-center + logs); đã gộp vào run-e2e.sh --suite=full. |
| **run-e2e-all-verify.sh** | Thay bằng run-e2e.sh --suite=full (gọi run-e2e-full + e2e-risk-center-full + test-priority1 + …). |

---

## 5. Thư mục báo cáo

- **test-results/** (repo root): risk-center-e2e-*.md, E2E-FULL-*.md, E2E-WITH-CAPABILITY-*.md.
- **docs/test-results/**: có thể chứa báo cáo cũ (full-regression-*); runner mới ghi vào test-results/ hoặc docs/test-results/ tùy cấu hình.

---

## 6. Pipeline

- **full-rebuild-sync-deploy-and-e2e.sh**: Sau deploy gọi `run-e2e-with-capability-report.sh` hoặc `run-e2e-full.sh`. Có thể đổi thành `run-e2e.sh --suite=full-report`.
- **clean-rebuild-redeploy-and-test.sh**: Gọi test-priority1-apis, test-runtime-signals-e2e, test-pod-critical-risk-cluster-id.

Tài liệu Risk Center chi tiết 17 TCs: [Risk-Center-E2E-TestCases.md](../03-components/risk-center/Risk-Center-E2E-TestCases.md).
