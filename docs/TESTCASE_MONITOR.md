# Monitor chi tiết các testcase

Danh sách testcase verify/E2E: script, mục đích, lệnh chạy, tiêu chí pass, cách theo dõi kết quả.

---

## 1. Verify (scripts/verify/)

| # | Script | Mục đích | Lệnh | Tiêu chí pass | Monitor |
|---|--------|----------|------|----------------|---------|
| 1 | check-full-deployment.sh | Cluster, namespace, workloads, pods, services, secrets, RBAC, Core /health, Dashboard HTML, Agent DaemonSet | `./scripts/verify/check-full-deployment.sh` | ✅ toàn bộ mục (cluster, pods Running, Core 200, Dashboard HTML, Agent desired=ready) | Đếm dòng ❌/⚠️; cuối script in ERRORS/WARNINGS |
| 2 | verify-agent-core-connectivity.sh | Agent ↔ Core: DNS, endpoint, gRPC (từ pod agent) | `./scripts/verify/verify-agent-core-connectivity.sh` | DNS resolve, Core có endpoint, kết nối gRPC thành công | Output "OK" / "FAIL" từng bước |
| 3 | verify-dashboard-api.sh | Dashboard API (stats, clusters, agents) qua Core | `./scripts/verify/verify-dashboard-api.sh` | HTTP 200, JSON hợp lệ | In response/status từng API. *Cần port-forward Core:* `kubectl port-forward -n fortuna svc/fortuna-core 8080:8080` hoặc `CORE_URL=...` |
| 4 | verify-dashboard-apis.sh | Nhiều API dashboard (stats, clusters, agents, insights) | `./scripts/verify/verify-dashboard-apis.sh` | Các API trả 200 | In kết quả từng endpoint. Dùng *kubectl exec* vào Core pod (không cần port-forward). |
| 5 | verify-dashboard-issues.sh | Kiểm tra lỗi thường gặp dashboard (empty, stale) | `./scripts/verify/verify-dashboard-issues.sh` | Không phát hiện issue hoặc báo rõ | In từng check |
| 6 | verify-database-schema.sh | Schema DB (tables, columns) | `./scripts/verify/verify-database-schema.sh` | Các bảng/cột cần thiết tồn tại | In danh sách table/column |
| 7 | verify-agent-availability.sh | Agent pods Ready, đăng ký với Core | `./scripts/verify/verify-agent-availability.sh` | Agent Ready, có trong Core/agents | In trạng thái từng agent |
| 8 | verify-pod-count.sh | Số pod đồng bộ với Core vs K8s | `./scripts/verify/verify-pod-count.sh` | Số lượng khớp hoặc giải thích chênh lệch | In count Core vs K8s |
| 9 | verify-pod-data.sh | Dữ liệu pod trong DB (namespace, node, v.v.) | `./scripts/verify/verify-pod-data.sh` | Dữ liệu hợp lệ | In mẫu/query |
| 10 | verify-test-data.sh | Dữ liệu test (insights, SBOM, CVE) | `./scripts/verify/verify-test-data.sh` | Có dữ liệu test theo mong đợi | In kết quả check |
| 11 | verify-threat-velocity-pipeline.sh | Luồng Threat Velocity (runtime → signals → insights) | `./scripts/verify/verify-threat-velocity-pipeline.sh` | Pipeline hoạt động | In từng bước pipeline |
| 12 | check-pod-risk.sh | Pod risk (critical/high) | `./scripts/verify/check-pod-risk.sh` | Liệt kê pod risk hoặc "none" | In danh sách pod/risk |
| 13 | verify-after-deploy-and-e2e.sh | Tổng hợp sau deploy + E2E (pods, API, DB) | `./scripts/verify/verify-after-deploy-and-e2e.sh` | Các mục verify pass | In từng nhóm kết quả |

*Script phụ (không phải testcase): `expand-disk-worker01-remote.sh` — utility mở rộng disk trên node.*

---

## 2. E2E (scripts/e2e/)

### 2.1 Chạy tổng hợp (entry point: run-e2e.sh)

**Chi tiết test case và luồng code:** [docs/e2e/E2E-TestCases-And-Runner.md](e2e/E2E-TestCases-And-Runner.md).

| # | Script | Mục đích | Lệnh | Tiêu chí pass | Monitor |
|---|--------|----------|------|----------------|---------|
| 1 | **run-e2e.sh** | Entry point E2E: chạy theo suite (risk-center \| full \| priority1 \| runtime \| pce \| dashboard \| sbom \| full-report) | `./scripts/e2e/run-e2e.sh` hoặc `--suite=risk-center` | Các suite chạy đúng script tương ứng, báo PASS/FAIL | Stdout + file report từng script con |
| 2 | run-e2e-full.sh | Cluster, pods, Core API (health, capability-metadata, promotion-rules, runtime-signals, attack-steps, pod capabilities), DB row counts, Dashboard URL, test-pce-api, test-priority1-apis, Core unit tests | `./scripts/e2e/run-e2e-full.sh` hoặc `run-e2e.sh --suite=full` | File E2E-FULL-*.md có đủ section, HTTP 200 cho API (hoặc 401 nếu chưa login) | File `docs/test-results/E2E-FULL-<timestamp>.md` |
| 3 | run-e2e-with-capability-report.sh | Full luồng + capability: metadata, promotion rules, pod capabilities, runtime signals, attack steps, test-pce-e2e, test-promotion-flow, test-runtime-signals-e2e | `./scripts/e2e/run-e2e-with-capability-report.sh` hoặc `run-e2e.sh --suite=full-report` | File E2E-WITH-CAPABILITY-*.md, các API test PASS | File `docs/test-results/E2E-WITH-CAPABILITY-<timestamp>.md` |

### 2.2 Testcase đơn lẻ (API / luồng)

| # | Script | Mục đích | Lệnh | Tiêu chí pass | Monitor |
|---|--------|----------|------|----------------|---------|
| 5 | test-priority1-apis.sh | GET promotion-rules, promotion-rules/capability/ESC_HOSTPATH_NODE, runtime-signals, attack-steps/summary (cần JWT: admin/admin123) | `NAMESPACE=fortuna ./scripts/e2e/test-priority1-apis.sh` | ✅ Status 200, Count/response có dữ liệu | In từng Test 1/2/3/4 + ✅/❌ |
| 6 | test-runtime-signals-e2e.sh | POST runtime-events → DB runtime_events/runtime_signals → GET runtime-signals, runtime-signals/pods/:podUid | `NAMESPACE=fortuna ./scripts/e2e/test-runtime-signals-e2e.sh` | POST 200, DB có row, GET 200 có data | In [OK]/[FAIL] từng bước |
| 7 | test-pce-api.sh | PCE API (capability metadata, promotion rules, pod capabilities) | `./scripts/e2e/test-pce-api.sh` | HTTP 200, response hợp lệ | In response/status |
| 8 | test-pce-e2e.sh | E2E PCE: privileged pod → sync → pod_capabilities + runtime_signals | `./scripts/e2e/test-pce-e2e.sh` | Pod tạo, Core sync, capabilities/signals có trong DB/API | In từng giai đoạn |
| 9 | test-promotion-flow.sh | Luồng promotion (rules → capabilities → promotion logic) | `./scripts/e2e/test-promotion-flow.sh` | Các bước promotion đúng | In kết quả từng bước |
| 10 | e2e-dashboard-data.sh | Pod + Core sync + runtime-events + insights/evaluate/historical → verify threat-velocity & PCE trends API | `NAMESPACE=fortuna ./scripts/e2e/e2e-dashboard-data.sh` | API threat-velocity, PCE trends trả data | In [OK]/[FAIL] |
| 11 | e2e-risk-center-full.sh | 17 TCs Risk Center: /risks, /insights/summary, risk-rules CRUD, global summary, runtime-signals, v.v. (chi tiết: docs/e2e/E2E-TestCases-And-Runner.md) | `NAMESPACE=fortuna ./scripts/e2e/e2e-risk-center-full.sh` hoặc `run-e2e.sh --suite=risk-center` | 17/17 TCs PASS, report risk-center-e2e-*.md | In [OK]/[FAIL] từng TC + file report |
| 12 | e2e-sbom-verify.sh | SBOM: pod → Agent extract → Core lưu → API/DB có SBOM | `./scripts/e2e/e2e-sbom-verify.sh` | SBOM xuất hiện trong Core/DB | In kết quả từng bước |
| 13 | test-sbom-pod-flow.sh | Luồng pod → SBOM → CVE match → insights | `./scripts/e2e/test-sbom-pod-flow.sh` | Pod, SBOM, insight có trong hệ thống | In từng bước |
| 14 | test-pod-sync-flow.sh | Pod sync: tạo pod → Agent sync → Core có pod | `./scripts/e2e/test-pod-sync-flow.sh` | Pod xuất hiện trong Core/DB | In sync result |
| 15 | test-pod-risk-flow.sh | Luồng risk: pod → risk calculation → API risk | `./scripts/e2e/test-pod-risk-flow.sh` | Risk API trả đúng pod/risk | In risk result |
| 16 | test-pod-critical-risk-cluster-id.sh | Pod critical risk theo cluster_id | `./scripts/e2e/test-pod-critical-risk-cluster-id.sh` | API/DB khớp cluster_id và critical risk | In kết quả |
| 17 | e2e-pod-delete-cleanup-verify.sh | Xóa pod → verify cleanup (DB, API) | `./scripts/e2e/e2e-pod-delete-cleanup-verify.sh` | Sau xóa, dữ liệu pod/risk được dọn | In trước/sau |
| 18 | test-dashboard-consistency-e2e.sh | Nhất quán dữ liệu dashboard (clusters, agents, insights) | `./scripts/e2e/test-dashboard-consistency-e2e.sh` | Số liệu nhất quán giữa các API | In so sánh |
| 19 | test-runtime-probe-e2e.sh | Runtime probe E2E | `./scripts/e2e/test-runtime-probe-e2e.sh` | Probe gửi/nhận đúng | In probe result |
| 20 | run-dashboard-data-tests.sh | Tổ hợp: CVE load (optional), e2e-dashboard-data, test-pce-e2e, test-sbom-pod-flow (optional), verify dashboard APIs | `./scripts/e2e/run-dashboard-data-tests.sh` | Các script con pass hoặc warn | In từng step |
| 21 | test-pod-recreate-storage.sh | Pod recreate: cùng UID / xóa rồi tạo lại (new UID) → kiểm tra storage/DB | `NAMESPACE=fortuna ./scripts/e2e/test-pod-recreate-storage.sh` | DB lưu đúng, không trùng bản ghi | In từng phase |

---

## 3. Chạy monitor nhanh (một lệnh)

Script **`./scripts/monitor/monitor-testcases.sh`** chạy lần lượt các testcase quan trọng và in kết quả chi tiết từng bước. Xem mục 4 bên dưới.

---

## 4. Script monitor testcase

**Lệnh:**

```bash
# Chạy các testcase chính và in kết quả ra stdout
./scripts/monitor/monitor-testcases.sh

# Ghi thêm vào file
./scripts/monitor/monitor-testcases.sh --report docs/test-results/MONITOR-TESTCASES-$(date +%Y%m%d-%H%M%S).txt
```

**Thứ tự:** check-full-deployment → test-priority1-apis → test-runtime-signals-e2e → e2e-risk-center-verify → verify-dashboard-api → verify-agent-core-connectivity.

**Env bỏ qua bước:** `SKIP_DEPLOYMENT=1`, `SKIP_PRIORITY1=1`, `SKIP_RUNTIME_SIGNALS=1`, `SKIP_RISK_CENTER=1`, `SKIP_DASHBOARD_API=1`, `SKIP_AGENT_CORE=1` (bất kỳ giá trị không rỗng = skip).

---

## 5. Báo cáo kết quả

| Nguồn | File / vị trí |
|-------|----------------|
| E2E tổng hợp | `docs/test-results/E2E-ALL-VERIFY-<timestamp>.md` |
| E2E full chi tiết | `docs/test-results/E2E-FULL-<timestamp>.md` |
| E2E + capability | `docs/test-results/E2E-WITH-CAPABILITY-<timestamp>.md` |
| E2E complete + monitor | `docs/test-results/E2E-COMPLETE-WITH-MONITOR-<timestamp>.md` |
| API results (raw) | `docs/test-results/e2e-api-results-<timestamp>.txt` |
| Log snapshot | `docs/test-results/e2e-logs-before/after-<timestamp>.txt` |

Namespace mặc định: `fortuna`. Có thể ghi đè: `NAMESPACE=my-ns ./scripts/...`
