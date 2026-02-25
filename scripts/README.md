# Fortuna Scripts Reference

**Last Updated**: 2026-02-21

---

## Cấu trúc thư mục

Scripts được sắp xếp theo nhóm trong thư mục con. **Luôn gọi theo đường dẫn đầy đủ** `./scripts/<thư mục>/<script>.sh` (không còn file/symlink tại `scripts/` root).

| Thư mục    | Nội dung |
|------------|----------|
| **pipeline/** | full-clean-database-rebuild-deploy.sh, full-rebuild-sync-deploy-and-e2e.sh, clean-rebuild-redeploy-and-test.sh |
| **deploy/**   | deploy-fortuna-robust.sh, pre-deployment-checks.sh, apply-core-master-only.sh, fix-flannel-vxlan.sh, fix-dns-config.sh |
| **clean/**    | cleanup-environment.sh, clean-containerd-images.sh, clean-rebuild-dashboard.sh, cleanup-orphaned-migrations.sh |
| **build/**    | build-and-load-containerd.sh, build-dashboard-containerd.sh, build-production.sh |
| **verify/**   | check-full-deployment.sh, verify-dashboard-*.sh, verify-database-schema.sh, verify-agent-availability.sh, verify-pod-data.sh, verify-test-data.sh, check-pod-risk.sh |
| **e2e/**      | run-e2e-full.sh, run-e2e-with-capability-report.sh, run-e2e-tests.sh, run-dashboard-data-tests.sh, e2e-dashboard-data.sh, e2e-sbom-verify.sh, test-*.sh |
| **monitor/**  | monitor-agent-core.sh, monitor-runtime-signals.sh |
| **utils/**    | push-images-to-workers.sh, load-cve-data.sh, create_mtls_secret.sh, manage-port-forwards.sh, port-forward-dashboard.sh, sync-k8s-data.sh, reset-worker-*.sh, fix-dns-issues.sh, import-to-containerd.sh, validate-migrations.sh, ... |

**Ví dụ:** `./scripts/pipeline/full-clean-database-rebuild-deploy.sh`, `./scripts/build/build-and-load-containerd.sh`, `./scripts/clean/cleanup-orphaned-migrations.sh`.

**Đã xóa:** Thư mục `scripts/archive/` (24 script cũ/outdate) đã được xóa hoàn toàn để tránh noise. Script đang dùng nằm trong 8 thư mục trên.

---

## Hiện trạng

- **Deploy YAML**: Chỉ dùng `deploy/fortuna-core-deployment.yaml`, `deploy/fortuna-agent-daemonset.yaml`, `deploy/dashboard-deployment.yaml`. Các file cũ `core-deployment.yaml`, `agent-daemonset.yaml` đã xóa.
- **Image prefix**: `fortuna` (core, agent, dashboard). Namespace containerd: `k8s.io`.
- **Namespace K8s**: `fortuna`.

---

## Pipeline (Clean / Rebuild / Deploy)

### `full-clean-database-rebuild-deploy.sh` **(script chính – full reset)**

Clean toàn bộ image fortuna (nerdctl), tùy chọn clean DB, rebuild (core, agent, dashboard), deploy. **Không** cập nhật image tag trong `deploy/*.yaml` (dùng tag sẵn có trong file).

```bash
./scripts/pipeline/full-clean-database-rebuild-deploy.sh              # clean images + rebuild + deploy
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --db         # + xóa dữ liệu DB (DELETE, giữ schema)
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --db-reset   # + full reset DB (DROP tables; Core chạy lại migrations)
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --skip-rebuild   # chỉ clean + deploy
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --skip-deploy    # chỉ clean + rebuild
```

**Database & Agent sync:** Core chạy migrations khi khởi động. Nếu agent log `[Syncer] Sync failed: status=500` và Core log `column "kubeconfig" of relation "clusters" does not exist`, chạy với `--db-reset` rồi deploy lại.

Chi tiết: **`docs/05-operations/DEPLOYMENT_CONTAINERD.md`**.

### `full-rebuild-sync-deploy-and-e2e.sh`

Clean → Rebuild (NO_CACHE) → **đồng bộ image tag vào deploy YAMLs** → Deploy → (tùy chọn push images lên worker) → E2E (run-e2e-with-capability-report.sh hoặc run-e2e-full.sh).

```bash
./scripts/pipeline/full-rebuild-sync-deploy-and-e2e.sh              # full run
./scripts/pipeline/full-rebuild-sync-deploy-and-e2e.sh --skip-push-workers
./scripts/pipeline/full-rebuild-sync-deploy-and-e2e.sh --skip-e2e   # bỏ E2E report
```

### `clean-rebuild-redeploy-and-test.sh`

Clean → Rebuild → Deploy → Chạy test + monitor. Gọi `full-clean-database-rebuild-deploy.sh` rồi check-full-deployment, test-priority1-apis, verify-dashboard-api, v.v.

```bash
./scripts/pipeline/clean-rebuild-redeploy-and-test.sh
./scripts/pipeline/clean-rebuild-redeploy-and-test.sh --db
./scripts/pipeline/clean-rebuild-redeploy-and-test.sh --skip-rebuild --skip-deploy   # chỉ test + monitor
```

### `deploy-fortuna-robust.sh`

Chỉ deploy (không clean/rebuild): namespace, postgres, nats, RBAC, core, agent, dashboard. Có pre-deployment checks, DNS fallback, verification. Dùng file: `deploy/fortuna-core-deployment.yaml`, `deploy/fortuna-agent-daemonset.yaml`, `deploy/dashboard-deployment.yaml`.

```bash
./scripts/deploy/deploy-fortuna-robust.sh
USE_IP_FALLBACK=false ./scripts/deploy/deploy-fortuna-robust.sh
```

### `pre-deployment-checks.sh`

Kiểm tra cluster trước khi deploy: kubectl, containerd/nerdctl, DNS, CoreDNS, network.

```bash
./scripts/deploy/pre-deployment-checks.sh
```

---

## Clean (dọn môi trường)

### `cleanup-environment.sh` **(script chính – clean env)**

Dọn port-forward, E2E/test namespaces, completed/failed/evicted pods, old images (giữ 3 mới nhất hoặc aggressive), build cache. **Không** rebuild/deploy.

```bash
./scripts/clean/cleanup-environment.sh                # clean mặc định (giữ 3 image mới nhất)
./scripts/clean/cleanup-environment.sh --db          # + xóa E2E test data trong Postgres
./scripts/clean/cleanup-environment.sh --aggressive # + test ns khác, image không latest, evicted pods, prune -a
```

### `clean-containerd-images.sh`

Chỉ xóa image fortuna/ksam trong containerd (namespace `k8s.io`). Có `--dry-run`, `--all`, `--prefix`.

```bash
./scripts/clean/clean-containerd-images.sh           # xóa image fortuna
./scripts/clean/clean-containerd-images.sh --dry-run
./scripts/clean/clean-containerd-images.sh --all    # xóa tất cả image (nguy hiểm)
```

### `clean-rebuild-dashboard.sh`

Chỉ clean image dashboard + rebuild dashboard + apply `deploy/dashboard-deployment.yaml`. Dùng khi chỉ sửa dashboard.

```bash
./scripts/clean/clean-rebuild-dashboard.sh
```

### `cleanup-orphaned-migrations.sh`

Archive file migration orphan (codebase cleanup). Không liên quan runtime/K8s.

```bash
./scripts/clean/cleanup-orphaned-migrations.sh
```

---

## Build

### `build-and-load-containerd.sh` **(build chính)**

Build core, agent, dashboard bằng nerdctl và load vào containerd (namespace `k8s.io`).

```bash
./scripts/build/build-and-load-containerd.sh
SKIP_DASHBOARD=true ./scripts/build/build-and-load-containerd.sh   # bỏ qua dashboard
NO_CACHE=true ./scripts/build/build-and-load-containerd.sh        # build không cache
EXPORT_IMAGES=true ./scripts/build/build-and-load-containerd.sh   # export tar sau khi build
```

### `build-dashboard-containerd.sh`

Chỉ build image dashboard (nerdctl → containerd).

```bash
./scripts/build/build-dashboard-containerd.sh
```

### `build-production.sh`

Build image cho production registry (Docker), có thể push.

```bash
./scripts/build/build-production.sh
PUSH_IMAGES=true ./scripts/build/build-production.sh
```

---

## Verify & Check

### `check-full-deployment.sh`

Kiểm tra cluster, namespace, workload, pod, service, Core health, dashboard, agent DaemonSet.

```bash
./scripts/verify/check-full-deployment.sh
```

### `verify-dashboard-api.sh`

Verify API Dashboard qua port-forward: login + gọi các endpoint chính. Cần `CORE_URL` (mặc định localhost:8080).

```bash
kubectl port-forward -n fortuna svc/fortuna-core 8080:8080 &
./scripts/verify/verify-dashboard-api.sh
```

### `verify-dashboard-apis.sh`

Verify tất cả API mà Dashboard dùng, gọi từ trong Core pod (không cần port-forward).

```bash
./scripts/verify/verify-dashboard-apis.sh
```

### `verify-dashboard-issues.sh`

Kiểm tra tổng hợp: image pod vs latest, Risk Center API, SBOM API, DB, UI.

```bash
./scripts/verify/verify-dashboard-issues.sh
```

### `verify-database-schema.sh`

Kiểm tra schema DB (migrations, bảng).

```bash
./scripts/verify/verify-database-schema.sh
```

### `verify-agent-availability.sh`, `verify-pod-data.sh`, `verify-test-data.sh`, `check-pod-risk.sh`

Verify agent, pod data, test data, pod risk (theo từng mục đích).

---

## E2E & Test

### `run-e2e-full.sh` **(E2E report chính)**

E2E đầy đủ: cluster/pods, Core API (health + responses), DB, dashboard. Ghi report: `docs/test-results/E2E-FULL-<timestamp>.md`.

```bash
./scripts/e2e/run-e2e-full.sh
```

### `run-e2e-with-capability-report.sh`

E2E + capability report. Được gọi bởi `full-rebuild-sync-deploy-and-e2e.sh` khi có sẵn.

```bash
./scripts/e2e/run-e2e-with-capability-report.sh
```

### `run-e2e-tests.sh`

E2E test cases (SBOM injector, test pods, API). Output: `docs/test-results/E2E-TEST-EXECUTION-<timestamp>.md`.

```bash
./scripts/e2e/run-e2e-tests.sh
```

### `test-priority1-apis.sh`

Gọi Core API từ trong Core pod: promotion-rules, runtime-signals, v.v.

```bash
./scripts/e2e/test-priority1-apis.sh
```

### `e2e-sbom-verify.sh`, `test-sbom-pod-flow.sh`, `test-runtime-signals-e2e.sh`

Verify SBOM, flow SBOM pod, E2E runtime signals.

### `run-dashboard-data-tests.sh`, `e2e-dashboard-data.sh`, `e2e-risk-center-verify.sh`, `test-pce-e2e.sh`

Chuỗi test dữ liệu Dashboard (Threat Velocity, PCE Trend). **Risk Center**: `e2e-risk-center-verify.sh` seed 1 insight + verify `/risks`, `/insights/summary`, `/runtime-signals`; được gọi trong `run-e2e-complete-with-monitor.sh`.

---

## Monitor & Utility

Tất cả nằm trong `scripts/monitor/` hoặc `scripts/utils/`. Gọi đầy đủ: `./scripts/monitor/...`, `./scripts/utils/...`.

### `monitor-agent-core.sh`, `monitor-runtime-signals.sh` (monitor/)

Xem trạng thái pod, health, log Agent/Core; monitor runtime signals.

### `manage-port-forwards.sh`, `port-forward-dashboard.sh` (utils/)

Quản lý port-forward (Core, Dashboard).

### `load-cve-data.sh` (utils/)

Load dữ liệu CVE vào DB.

### `create_mtls_secret.sh`

Tạo secret mTLS cho Core/Agent (cert với SANs). **Chỉ dùng script này** (không còn create-mtls-secrets.sh).

```bash
./scripts/utils/create_mtls_secret.sh
```

### `push-images-to-workers.sh`

Đẩy image từ node master sang worker (multi-node). Đọc image tag từ `deploy/fortuna-core-deployment.yaml`, `deploy/fortuna-agent-daemonset.yaml`.

```bash
./scripts/utils/push-images-to-workers.sh
```

### `sync-k8s-data.sh`, `reset-worker-node.sh`, `reset-worker-remote.sh`, `apply-core-master-only.sh`, `fix-dns-config.sh`

Đồng bộ K8s, reset worker, apply core master-only, fix DNS (dùng `deploy/fortuna-agent-daemonset.yaml`).

---

## Tổ chức theo nhóm

| Nhóm        | Script chính |
|------------|--------------|
| Pipeline   | full-clean-database-rebuild-deploy.sh, full-rebuild-sync-deploy-and-e2e.sh, deploy-fortuna-robust.sh, clean-rebuild-redeploy-and-test.sh |
| Clean      | cleanup-environment.sh, clean-containerd-images.sh, clean-rebuild-dashboard.sh, cleanup-orphaned-migrations.sh |
| Build      | build-and-load-containerd.sh, build-dashboard-containerd.sh, build-production.sh |
| Verify     | check-full-deployment.sh, verify-dashboard-api.sh, verify-dashboard-apis.sh, verify-dashboard-issues.sh, verify-database-schema.sh |
| E2E / Test | run-e2e-full.sh, run-e2e-complete-with-monitor.sh, run-e2e-with-capability-report.sh, test-priority1-apis.sh, e2e-dashboard-data.sh, e2e-risk-center-verify.sh, test-runtime-signals-e2e.sh, e2e-sbom-verify.sh, test-sbom-pod-flow.sh |
| Monitor    | monitor-agent-core.sh, monitor-runtime-signals.sh |
| Utility    | pre-deployment-checks.sh, manage-port-forwards.sh, port-forward-dashboard.sh, load-cve-data.sh, create_mtls_secret.sh, push-images-to-workers.sh |

---

## Script đã xóa (obsolete / duplicate)

- **build-with-containerd.sh** – thay bằng `build-and-load-containerd.sh`
- **load-cve-database.sh** – hardcoded path; dùng `load-cve-data.sh`
- **copy-containerd-images-to-nodes.sh** – trùng với `push-images-to-workers.sh`
- **fix_dns_isues.sh**, **fix_nats_storage.sh** – typo/duplicate; dùng `fix-dns-config.sh`
- **migrate-imports.sh** – one-time migration, hardcoded path
- **build-and-import-containerd.sh**, **full-clean-rebuild-deploy.sh**, **full-clean-rebuild-redeploy.sh**, **quick-deploy-containerd.sh**, **cleanup-repo.sh**, **build-dashboard.sh**, **run-e2e-comprehensive-report.sh**, **clean-all.sh**, **clean-for-dashboard-build.sh**, **clean-all-containerd-images.sh**, **create-mtls-secrets.sh**
- **deploy/core-deployment.yaml**, **deploy/agent-daemonset.yaml** – chỉ dùng fortuna-core-deployment.yaml, fortuna-agent-daemonset.yaml
- **scripts/archive/** (24 script cũ: build-and-deploy*.sh, deploy-fortuna*.sh, fix-*.sh, …) – đã xóa hoàn toàn để tránh noise

---

**Tài liệu chi tiết**: `docs/05-operations/DEPLOYMENT_CONTAINERD.md`, `deploy/README.md`. **Tra cứu đường dẫn script**: `docs/05-operations/SCRIPT_PATHS_REFERENCE.md`.

**Lưu ý**: Mọi script gọi theo đường dẫn đầy đủ từ repo root, ví dụ: `./scripts/pipeline/full-clean-database-rebuild-deploy.sh`, `./scripts/e2e/run-e2e-complete-with-monitor.sh`, `./scripts/utils/load-cve-data.sh`. Không còn script tại `scripts/*.sh` (root).
