# Fortuna Scripts Reference

**Last Updated**: 2026-02-02

---

## Overview

Scripts dùng cho build (nerdctl/containerd), deploy, test và verify Fortuna. Đã loại bỏ script cũ/trùng/bị lỗi; chỉ giữ bản đầy đủ và thống nhất.

---

## Pipeline (Clean / Rebuild / Deploy)

### `full-clean-database-rebuild-deploy.sh` **(script chính – full reset)**

Clean toàn bộ image fortuna (nerdctl), tùy chọn clean DB, rebuild (core, agent, dashboard), deploy.

```bash
# Clean images + rebuild + deploy (không đụng DB)
./scripts/full-clean-database-rebuild-deploy.sh

# Thêm: xóa dữ liệu DB (DELETE, giữ schema)
./scripts/full-clean-database-rebuild-deploy.sh --db

# Thêm: full reset DB (DROP tables; Core chạy lại migrations khi start)
./scripts/full-clean-database-rebuild-deploy.sh --db-reset

# Chỉ clean + deploy (dùng lại image hiện có)
./scripts/full-clean-database-rebuild-deploy.sh --skip-rebuild

# Chỉ clean + rebuild (không deploy)
./scripts/full-clean-database-rebuild-deploy.sh --skip-deploy
```

Chi tiết build/deploy containerd: **`docs/DEPLOYMENT_CONTAINERD.md`**.

### `clean-rebuild-redeploy-and-test.sh`

Clean → Rebuild → Deploy → Chạy test + monitor. Gọi `full-clean-database-rebuild-deploy.sh` rồi chạy check-full-deployment, test-priority1-apis, verify-dashboard-api, v.v.

```bash
./scripts/clean-rebuild-redeploy-and-test.sh              # full run
./scripts/clean-rebuild-redeploy-and-test.sh --db         # + DB clean
./scripts/clean-rebuild-redeploy-and-test.sh --skip-rebuild --skip-deploy   # chỉ test + monitor
```

### `deploy-fortuna-robust.sh`

Chỉ deploy (không clean/rebuild): namespace, postgres, nats, RBAC, core, agent, dashboard. Có pre-deployment checks, DNS fallback, verification.

```bash
./scripts/deploy-fortuna-robust.sh
USE_IP_FALLBACK=false ./scripts/deploy-fortuna-robust.sh
```

### `pre-deployment-checks.sh`

Kiểm tra cluster trước khi deploy: kubectl, containerd/nerdctl, DNS, CoreDNS, network.

```bash
./scripts/pre-deployment-checks.sh
```

---

## Build

### `build-and-load-containerd.sh` **(build chính)**

Build core, agent, dashboard bằng nerdctl và load vào containerd (namespace `k8s.io`).

```bash
./scripts/build-and-load-containerd.sh
SKIP_DASHBOARD=true ./scripts/build-and-load-containerd.sh   # bỏ qua dashboard
NO_CACHE=true ./scripts/build-and-load-containerd.sh        # build không cache
EXPORT_IMAGES=true ./scripts/build-and-load-containerd.sh    # export tar sau khi build
```

### `build-dashboard-containerd.sh`

Chỉ build image dashboard (nerdctl → containerd). Dùng khi chỉ sửa dashboard.

```bash
./scripts/build-dashboard-containerd.sh
```

### `build-production.sh`

Build image cho production registry (Docker), có thể push.

```bash
./scripts/build-production.sh
PUSH_IMAGES=true ./scripts/build-production.sh
```

---

## Verify & Check

### `check-full-deployment.sh`

Kiểm tra cluster, namespace, workload, pod, service, Core health, dashboard, agent DaemonSet.

```bash
./scripts/check-full-deployment.sh
```

### `verify-dashboard-api.sh`

Verify API Dashboard qua port-forward: login + gọi các endpoint chính. Cần `CORE_URL` (mặc định localhost:8080).

```bash
kubectl port-forward -n fortuna svc/fortuna-core 8080:8080 &
./scripts/verify-dashboard-api.sh
```

### `verify-dashboard-apis.sh`

Verify tất cả API mà Dashboard dùng, gọi từ trong Core pod (không cần port-forward).

```bash
./scripts/verify-dashboard-apis.sh
```

### `verify-dashboard-issues.sh`

Kiểm tra tổng hợp: image pod vs latest, Risk Center API, SBOM API, DB, UI.

```bash
./scripts/verify-dashboard-issues.sh
```

### `verify-database-schema.sh`

Kiểm tra schema DB (migrations, bảng).

```bash
./scripts/verify-database-schema.sh
```

---

## E2E & Test

### `run-e2e-full.sh`

E2E đầy đủ: cluster/pods, Core API (health + responses), DB, dashboard. Ghi report: `docs/test-results/E2E-FULL-<timestamp>.md`.

```bash
./scripts/run-e2e-full.sh
```

### `run-e2e-tests.sh`

E2E test cases (SBOM injector, test pods, API). Output: `docs/test-results/E2E-TEST-EXECUTION-<timestamp>.md`.

```bash
./scripts/run-e2e-tests.sh
```

### `test-priority1-apis.sh`

Gọi Core API từ trong Core pod: promotion-rules, runtime-signals, v.v.

```bash
./scripts/test-priority1-apis.sh
```

### `e2e-sbom-verify.sh`

Verify pod xuất hiện trong API SBOM; tùy chọn chạy full SBOM flow.

```bash
./scripts/e2e-sbom-verify.sh
./scripts/e2e-sbom-verify.sh my-pod default
./scripts/e2e-sbom-verify.sh --full
```

### `test-sbom-pod-flow.sh`

Tạo pod test, đợi agent gửi SBOM, verify API /sbom và /sbom/:podId.

```bash
./scripts/test-sbom-pod-flow.sh
./scripts/test-sbom-pod-flow.sh --cleanup
```

### `test-runtime-signals-e2e.sh`

E2E runtime signals: POST runtime-events, kiểm tra DB, GET runtime-signals.

```bash
./scripts/test-runtime-signals-e2e.sh
```

### `run-dashboard-data-tests.sh` **(E2E dữ liệu cho Dashboard)**

Chạy chuỗi test để có dữ liệu cho biểu đồ Dashboard (Threat Velocity, PCE Trend) và Risk Center (Runtime / Escape). Gồm: load CVE (nếu có), deploy E2E vuln pod (optional), **e2e-dashboard-data.sh**, **test-pce-e2e.sh**, test-sbom-pod-flow, verify-dashboard-apis.

```bash
./scripts/run-dashboard-data-tests.sh
```

### `e2e-dashboard-data.sh`

E2E riêng cho dữ liệu Dashboard: tạo privileged pod, chờ sync Core (agent), POST runtime-events (PROC_ROOT_PIVOT, FS_ESCAPE_ATTEMPT), gọi insights/evaluate/historical, kiểm tra GET threat-velocity và GET pod-capabilities/trends (7 điểm, tổng risks/capabilities).

```bash
./scripts/e2e-dashboard-data.sh
```

### `test-pce-e2e.sh`

E2E PCE + Runtime: tạo privileged pod, **chờ pod xuất hiện trong Core (agent sync)**, chờ PCE evaluation, gửi runtime event, kiểm tra pod_capabilities và runtime_signals trong DB.

```bash
./scripts/test-pce-e2e.sh
```

---

## Monitor & Utility

### `monitor-agent-core.sh`

Xem trạng thái pod, health, log gần đây của Agent và Core.

```bash
./scripts/monitor-agent-core.sh
./scripts/monitor-agent-core.sh --follow --logs 50
```

### `monitor-runtime-signals.sh`

Monitor runtime signals: Agent reader, Core ingest, DB counts, API.

```bash
./scripts/monitor-runtime-signals.sh
./scripts/monitor-runtime-signals.sh --follow
```

### `manage-port-forwards.sh`

Quản lý port-forward (Core, Dashboard).

```bash
./scripts/manage-port-forwards.sh
```

### `port-forward-dashboard.sh`

Port-forward Dashboard (và có thể Core) để test từ local.

```bash
./scripts/port-forward-dashboard.sh
```

### `load-cve-data.sh`

Load dữ liệu CVE vào DB.

```bash
./scripts/load-cve-data.sh
```

### `create_mtls_secret.sh` / `create-mtls-secret.sh`

Tạo secret mTLS cho Core/Agent (nếu có).

```bash
./scripts/create_mtls_secret.sh
```

### `push-images-to-workers.sh`

Đẩy image từ node master sang worker (multi-node).

```bash
./scripts/push-images-to-workers.sh
```

### `sync-k8s-data.sh`, `reset-worker-node.sh`, `reset-worker-remote.sh`

Đồng bộ dữ liệu K8s, reset worker (dev/test).

---

## Script đã xóa (obsolete / duplicate)

- **build-and-import-containerd.sh** – gọi script không tồn tại (`build-with-containerd.sh`)
- **full-clean-rebuild-deploy.sh** – gọi `cleanup-environment.sh` (không tồn tại)
- **full-clean-rebuild-redeploy.sh** – thay bằng `full-clean-database-rebuild-deploy.sh` (có thêm --db-reset)
- **quick-deploy-containerd.sh** – gọi `build-with-containerd.sh` (không tồn tại)
- **cleanup-repo.sh** – script git untrack cũ, tham chiếu script đã xóa
- **build-dashboard.sh** – trùng chức năng với `build-dashboard-containerd.sh` và `build-and-load-containerd.sh`
- **run-e2e-comprehensive.sh** – trùng với `run-e2e-full.sh` (báo cáo E2E đầy đủ)

---

## Tổ chức

| Nhóm        | Script chính |
|------------|--------------|
| Pipeline   | full-clean-database-rebuild-deploy.sh, deploy-fortuna-robust.sh, clean-rebuild-redeploy-and-test.sh |
| Build      | build-and-load-containerd.sh, build-dashboard-containerd.sh, build-production.sh |
| Verify     | check-full-deployment.sh, verify-dashboard-api.sh, verify-dashboard-apis.sh, verify-dashboard-issues.sh, verify-database-schema.sh |
| E2E / Test | run-e2e-full.sh, run-e2e-tests.sh, test-priority1-apis.sh, e2e-sbom-verify.sh, test-sbom-pod-flow.sh, test-runtime-signals-e2e.sh |
| Monitor    | monitor-agent-core.sh, monitor-runtime-signals.sh |
| Utility    | pre-deployment-checks.sh, manage-port-forwards.sh, port-forward-dashboard.sh, load-cve-data.sh, create_mtls_secret.sh, push-images-to-workers.sh, sync-k8s-data.sh, reset-worker-*.sh |

---

**Tài liệu chi tiết**: `docs/DEPLOYMENT_CONTAINERD.md`, `deploy/README.md`.
