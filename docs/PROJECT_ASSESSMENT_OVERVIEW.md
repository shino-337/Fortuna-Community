# Đánh giá tổng quan dự án FortunaK8s (KSAM)

**Ngày đánh giá:** 2026-03-09

---

## 1. Khôi phục script đã xóa

- **Thao tác đã thực hiện:** `git checkout HEAD -- scripts/` để đồng bộ toàn bộ thư mục `scripts/` với commit hiện tại (HEAD).
- **Kết quả:** Các script từng bị xóa trên working tree đã được khôi phục, gồm:
  - `scripts/deploy/deploy-fortuna-robust.sh`
  - `scripts/pipeline/full-rebuild-sync-deploy-and-e2e.sh`
  - `scripts/clean/clean-evicted-completed-pods.sh`
  - E2E entry: `scripts/e2e/run-e2e.sh` (--suite=full|risk-center|…); các script đơn lẻ: e2e-risk-center-full.sh, test-priority1-apis.sh, e2e-dashboard-data.sh, test-pce-e2e.sh, test-runtime-signals-e2e.sh, v.v.
  - `scripts/utils/load-cve-data.sh`, `sync-image-tag-and-clean.sh`, `sync-k8s-data.sh`
- **Tổng số script .sh hiện có:** **91** (trong `scripts/`).

---

## 2. Tổng quan dự án

| Mục | Mô tả |
|-----|--------|
| **Tên** | FortunaK8s (KSAM) |
| **Mục đích** | Nền tảng **K8s Security & Risk Management**: SBOM, CVE, insights, Pod Capability Engine (PCE), runtime signals, dashboard. |
| **Repo** | Monorepo (Go workspace: core, agent, api; dashboard React/Vite; deploy K8s; docs; scripts). |

**Thành phần chính:**

- **Core** – API trung tâm (REST/gRPC), PostgreSQL, NATS, migrations, PCE scheduler.
- **Agent** – DaemonSet (một pod/node): đồng bộ pod, SBOM, gRPC (mTLS) tới Core.
- **Dashboard** – Giao diện web (React): Risk Center, SBOM, pod detail, capabilities.

---

## 3. Mã nguồn (source code)

### 3.1 Go (Core, Agent, API)

| Thành phần | Đường dẫn | Số file .go (ước lượng) | Ghi chú |
|------------|-----------|---------------------------|--------|
| **Core** | `core/` | ~215 | cmd, internal (api, config, grpc, health, middleware, scheduler, storage, webhook), migrations, pkg (models, risk, policy, worker, …). |
| **Agent** | `agent/` | ~38 | cmd, internal (client, config, runtime, sbom, syncer), pkg. |
| **API (shared)** | `api/` | proto + generated | gRPC definitions, cve/sbom/service .pb.go. |

**Go workspace:** `go.work` khai báo `use ./core`, `./agent`, `./api` (Go 1.24).

### 3.2 Dashboard (frontend)

| Chỉ số | Giá trị |
|--------|--------|
| **Công nghệ** | React, TypeScript, Vite |
| **File TS/TSX** | ~55 |
| **Thư mục chính** | `dashboard/components`, `pages`, `lib`, `hooks`, `store`, `constants` |

**Trang chính:** Risk Center, SBOM, Resources, Pod Detail, Clusters, Insights, Rules, Settings, …

### 3.3 Khác

- **CVE data:** `cve-data/all/` – nguồn CVE từ OSV (sync all.zip), dùng cho cve-loader load vào PostgreSQL; `cve-data/e2e/` – dữ liệu mẫu cho e2e (nếu có). **Không xóa** `cve-data/all/` trừ khi đã load xong vào DB và dùng option dọn trong load-cve-data.sh.
- **Deploy:** YAML K8s, Helm (chart `helm/fortuna`), script-prod (build/deploy/verify/clean).

---

## 4. Tài liệu (documentation)

### 4.1 Số lượng & cấu trúc

- **Tổng file .md (docs + docs-prod):** ~244.
- **Thư mục chính:**
  - **docs/** – Tài liệu dev/ops: 01-getting-started, 02-architecture, 03-components, 04-development, 05-operations, 06-reference, 07-guides, 08-tutorials, test-results, e2e, archive, backlog.
  - **docs-prod/** – Tài liệu production: overview, architecture, user guide, operations, configuration.

### 4.2 Điểm vào

- **docs/README.md** – Chỉ mục đầy đủ (quick start, architecture, operations, components, reference).
- **docs-prod/README.md** – Tài liệu hướng production.
- **deploy/README.md** – Checklist deploy, manifests, Helm, xử lý lỗi.
- **scripts/README.md** – Mô tả toàn bộ script (pipeline, deploy, clean, build, verify, e2e, monitor).

### 4.3 Một số tài liệu quan trọng

- **Pod Detail:** `docs/03-components/podDetail/` (POD_DETAIL_SPEC, POD_SYNC_ARCHITECTURE_AND_DATA_MODEL_SPEC, podDetail_page_UIUX.md, testSuite.md).
- **Kiến trúc:** ARCHITECTURE.md, COMPONENTS.md, REPOSITORY_STRUCTURE.md.
- **Vận hành:** PRODUCTION_DEPLOYMENT.md, CLEAN_REBUILD_REDEPLOY_AND_VERIFY.md, AGENT_CORE_ERRORS_MONITOR.md.
- **Kiểm kê script/docs:** DOCS_AND_SCRIPTS_INVENTORY.md (danh sách script, docs, lý do một số file bị xóa).

---

## 5. Scripts

### 5.1 Phân bố theo thư mục

| Thư mục | Số script | Nội dung chính |
|---------|-----------|----------------|
| **pipeline/** | 4 | full-clean-database-rebuild-deploy.sh, full-rebuild-sync-deploy-and-e2e.sh, clean-rebuild-redeploy-and-test.sh, run-clean-rebuild-deploy-no-timeout.sh |
| **deploy/** | 9 | deploy-fortuna-robust.sh, pre-deployment-checks.sh, ensure-flannel.sh, ensure-storage-class.sh, ensure-cluster-addons.sh, ensure-control-plane-label.sh, fix-flannel-vxlan.sh, fix-dns-config.sh, apply-core-master-only.sh |
| **clean/** | 7 | cleanup-environment.sh, clean-containerd-images.sh, clean-rebuild-dashboard.sh, clean-host-images-and-junk.sh, clean-e2e-test-images.sh, cleanup-orphaned-migrations.sh, clean-evicted-completed-pods.sh |
| **build/** | 4 | build-and-load-containerd.sh, build-dashboard-containerd.sh, build-production.sh, export-agent-image-for-workers.sh |
| **verify/** | 19 | check-full-deployment.sh, check-env-rebuild-deploy.sh, verify-dashboard-*.sh, verify-database-schema.sh, verify-agent-*.sh, verify-pod-data.sh, verify-api-detailed.sh, verify-via-api-only.sh, … |
| **e2e/** | 22 | run-e2e.sh (entry), run-e2e-full.sh, run-e2e-with-capability-report.sh, e2e-risk-center-full.sh, e2e-sbom-verify.sh, test-priority1-apis.sh, test-sbom-pod-flow.sh, test-pod-*.sh, … |
| **monitor/** | 4 | monitor-testcases.sh, monitor-runtime-signals.sh, monitor-agent-core.sh, monitor-agent-core-errors.sh |
| **utils/** | 20 | push-images-to-workers.sh, port-forward-dashboard.sh, manage-port-forwards.sh, create_mtls_secret.sh, import-to-containerd.sh, fix-k8s-swap-for-kubelet.sh, load-cve-data.sh, sync-k8s-data.sh, … |

### 5.2 Script quan trọng cho build/deploy/verify

- **Pipeline:** `full-clean-database-rebuild-deploy.sh` (clean + rebuild + deploy), `full-rebuild-sync-deploy-and-e2e.sh` (sync tag + deploy + E2E).
- **Deploy:** `deploy-fortuna-robust.sh` (infra, mTLS, Core, Agent, Dashboard).
- **Build:** `build-and-load-containerd.sh` (core, agent, dashboard → containerd k8s.io).
- **Push image:** `push-images-to-workers.sh` (đẩy image lên master + worker; dùng `scripts/utils/push-images.config`).
- **Verify:** `check-full-deployment.sh`, `check-env-rebuild-deploy.sh`.

---

## 6. Deploy & hạ tầng

### 6.1 Kubernetes manifests

- **Workload chính:** `fortuna-core-deployment.yaml`, `fortuna-agent-daemonset.yaml`, `dashboard-deployment.yaml`, `fortuna-rbac.yaml`, `webhook-service.yaml`, `dashboard-nginx-configmap.yaml`.
- **Hạ tầng:** `deploy/infrastructure/postgresql.yaml`, `postgresql-with-age.yaml`, `nats.yaml`, `redis.yaml`, `postgresql-local-pv.yaml`.
- **Khác:** `core-secrets.yaml`, `webhook-config.yaml`, `risk-evaluation-cronjob.yaml`, `cve-database-sample.yaml`, `deploy/certs/`, `deploy/e2e/`.

### 6.2 Helm

- Chart: `helm/fortuna/` (Chart.yaml, values.yaml, templates: core, agent, dashboard, rbac).

### 6.3 Script production

- `script-prod/`: build.sh, deploy.sh, clean.sh, verify.sh, lib.sh, config.env.example – build/deploy theo phiên bản, config-driven.

---

## 7. Đánh giá tóm tắt

| Hạng mục | Đánh giá ngắn |
|----------|----------------|
| **Mã nguồn** | Cấu trúc rõ (core/agent/dashboard/api), Go workspace thống nhất, dashboard React/Vite có components và pages đầy đủ. Một số file từng bị xóa đã khôi phục (core/cmd/main.go, agent internal, dashboard PodDetail, lib/api). |
| **Tài liệu** | Nhiều (≈244 .md), có chỉ mục (docs/README, docs-prod, deploy, scripts). Pod Detail, kiến trúc, vận hành có tài liệu riêng. Một số thư mục docs bị .gitignore (reference, 04-development, …) – cần lưu ý khi thêm/sửa. |
| **Script** | 91 script, chia rõ pipeline/deploy/clean/build/verify/e2e/monitor/utils. Script chính (deploy-fortuna-robust, full-clean-database-rebuild-deploy, push-images-to-workers, build-and-load-containerd) đã có và đã khôi phục nếu từng mất. |
| **Deploy** | Đủ manifest cho Core, Agent, Dashboard, RBAC, infra (Postgres, NATS). Có Helm và script-prod cho production. |

### Khuyến nghị

1. **Script:** Tránh xóa file trên ổ đĩa nếu chưa muốn loại bỏ khỏi repo; dùng `sync-git-with-local.sh` có chủ đích và chỉ sau khi đã review danh sách file bị xóa.
2. **Docs:** Cập nhật `docs/DOCS_STRUCTURE.md` và `docs/README.md` khi thêm/xóa thư mục tài liệu; giữ `DOCS_AND_SCRIPTS_INVENTORY.md` (hoặc tương đương) để kiểm kê script/docs định kỳ.
3. **README:** README gốc và docs/README đã nêu rõ quick start, cấu trúc repo, link deploy/scripts – nên giữ đồng bộ với script và deploy thực tế.

---

*Tài liệu này là bản đánh giá tổng quan tại thời điểm 2026-03-09; nên cập nhật lại khi cấu trúc hoặc quy trình thay đổi.*
