# Script paths – tra cứu nhanh

*Cập nhật: 2026-02-21*

Mọi script gọi từ **repo root** với đường dẫn đầy đủ: `./scripts/<thư mục>/<script>.sh`. Không còn file tại `scripts/*.sh` (root).

---

## Pipeline (clean / rebuild / deploy)

| Script | Đường dẫn đầy đủ |
|--------|-------------------|
| Full clean + rebuild + deploy | `./scripts/pipeline/full-clean-database-rebuild-deploy.sh` |
| Full rebuild + sync tag + deploy + E2E | `./scripts/pipeline/full-rebuild-sync-deploy-and-e2e.sh` |
| Clean + rebuild + deploy + test | `./scripts/pipeline/clean-rebuild-redeploy-and-test.sh` |
| Run clean-rebuild-deploy (no timeout) | `./scripts/pipeline/run-clean-rebuild-deploy-no-timeout.sh` |

**Ví dụ:** `./scripts/pipeline/full-clean-database-rebuild-deploy.sh --db-reset`

---

## Deploy

| Script | Đường dẫn đầy đủ |
|--------|-------------------|
| Deploy robust (namespace, infra, RBAC, core, agent, dashboard) | `./scripts/deploy/deploy-fortuna-robust.sh` |
| Pre-deployment checks | `./scripts/deploy/pre-deployment-checks.sh` |
| Ensure cluster addons (kube-proxy, CoreDNS) | `./scripts/deploy/ensure-cluster-addons.sh` |
| Ensure StorageClass (local-path) | `./scripts/deploy/ensure-storage-class.sh` |
| Apply Core master-only | `./scripts/deploy/apply-core-master-only.sh` |
| Fix Flannel VXLAN (multi-node) | `./scripts/deploy/fix-flannel-vxlan.sh` |
| Fix DNS config | `./scripts/deploy/fix-dns-config.sh` |

---

## Build

| Script | Đường dẫn đầy đủ |
|--------|-------------------|
| Build core + agent + dashboard (nerdctl → containerd) | `./scripts/build/build-and-load-containerd.sh` |
| Build dashboard only | `./scripts/build/build-dashboard-containerd.sh` |
| Build production (Docker, push optional) | `./scripts/build/build-production.sh` |

---

## Clean

| Script | Đường dẫn đầy đủ |
|--------|-------------------|
| Clean environment (port-forward, images, cache) | `./scripts/clean/cleanup-environment.sh` |
| Clean containerd images (fortuna) | `./scripts/clean/clean-containerd-images.sh` |
| Clean + rebuild dashboard only | `./scripts/clean/clean-rebuild-dashboard.sh` |
| Cleanup orphaned migrations (codebase) | `./scripts/clean/cleanup-orphaned-migrations.sh` |

---

## Verify

| Script | Đường dẫn đầy đủ |
|--------|-------------------|
| Check full deployment | `./scripts/verify/check-full-deployment.sh` |
| Verify dashboard API (port-forward) | `./scripts/verify/verify-dashboard-api.sh` |
| Verify dashboard APIs (from Core pod) | `./scripts/verify/verify-dashboard-apis.sh` |
| Verify dashboard issues | `./scripts/verify/verify-dashboard-issues.sh` |
| Verify database schema | `./scripts/verify/verify-database-schema.sh` |
| Verify agent availability | `./scripts/verify/verify-agent-availability.sh` |
| Verify pod data | `./scripts/verify/verify-pod-data.sh` |
| Verify pod count | `./scripts/verify/verify-pod-count.sh` |
| Verify test data | `./scripts/verify/verify-test-data.sh` |
| Check pod risk | `./scripts/verify/check-pod-risk.sh` |

---

## E2E & Test

| Script | Đường dẫn đầy đủ |
|--------|-------------------|
| E2E full report | `./scripts/e2e/run-e2e-full.sh` |
| E2E complete + monitor + Risk Center | `./scripts/e2e/run-e2e-complete-with-monitor.sh` |
| E2E with capability report | `./scripts/e2e/run-e2e-with-capability-report.sh` |
| E2E test execution | `./scripts/e2e/run-e2e-tests.sh` |
| Dashboard data tests | `./scripts/e2e/run-dashboard-data-tests.sh` |
| E2E dashboard data (Threat Velocity, PCE) | `./scripts/e2e/e2e-dashboard-data.sh` |
| E2E Risk Center (seed insight + APIs) | `./scripts/e2e/e2e-risk-center-verify.sh` |
| E2E Dashboard consistency (K8s/DB/API) | `./scripts/e2e/test-dashboard-consistency-e2e.sh` |
| Priority 1 APIs | `./scripts/e2e/test-priority1-apis.sh` |
| Runtime signals E2E | `./scripts/e2e/test-runtime-signals-e2e.sh` |
| SBOM verify | `./scripts/e2e/e2e-sbom-verify.sh` |
| SBOM pod flow | `./scripts/e2e/test-sbom-pod-flow.sh` |
| Pod sync flow | `./scripts/e2e/test-pod-sync-flow.sh` |
| Pod risk flow | `./scripts/e2e/test-pod-risk-flow.sh` |
| Pod critical risk (cluster id) | `./scripts/e2e/test-pod-critical-risk-cluster-id.sh` |
| Pod recreate storage | `./scripts/e2e/test-pod-recreate-storage.sh` |
| Promotion flow | `./scripts/e2e/test-promotion-flow.sh` |
| PCE API | `./scripts/e2e/test-pce-api.sh` |
| PCE E2E | `./scripts/e2e/test-pce-e2e.sh` |
| Runtime probe E2E | `./scripts/e2e/test-runtime-probe-e2e.sh` |
| Helper dùng chung cho E2E scripts | `./scripts/e2e/common.sh` |

---

## Monitor

| Script | Đường dẫn đầy đủ |
|--------|-------------------|
| Monitor Agent + Core (logs, health) | `./scripts/monitor/monitor-agent-core.sh` |
| Monitor runtime signals | `./scripts/monitor/monitor-runtime-signals.sh` |

---

## Utils

| Script | Đường dẫn đầy đủ |
|--------|-------------------|
| Manage port-forwards | `./scripts/utils/manage-port-forwards.sh` |
| Port-forward dashboard | `./scripts/utils/port-forward-dashboard.sh` |
| Load CVE data | `./scripts/utils/load-cve-data.sh` |
| Create mTLS secret | `./scripts/utils/create_mtls_secret.sh` |
| Push images to workers | `./scripts/utils/push-images-to-workers.sh` |
| Sync image tag + clean | `./scripts/utils/sync-image-tag-and-clean.sh` |
| Sync git with local | `./scripts/utils/sync-git-with-local.sh` |
| Sync K8s data | `./scripts/utils/sync-k8s-data.sh` |
| Reset worker node | `./scripts/utils/reset-worker-node.sh` |
| Reset worker remote | `./scripts/utils/reset-worker-remote.sh` |
| Fix DNS issues | `./scripts/utils/fix-dns-issues.sh` |
| Import to containerd | `./scripts/utils/import-to-containerd.sh` |
| Validate migrations | `./scripts/utils/validate-migrations.sh` |
| Remove tracked ignored files | `./scripts/utils/remove-tracked-ignored-files.sh` |
| Create GitHub release | `./scripts/utils/create-github-release.sh` |
| Apply NATS single replica | `./scripts/utils/apply-nats-single-replica.sh` |

---

## Thường dùng

| Mục đích | Lệnh |
|----------|------|
| Deploy lần đầu / lại | `./scripts/deploy/deploy-fortuna-robust.sh` |
| Clean toàn bộ + rebuild + deploy | `./scripts/pipeline/full-clean-database-rebuild-deploy.sh` |
| Clean + reset DB + rebuild + deploy | `./scripts/pipeline/full-clean-database-rebuild-deploy.sh --db-reset` |
| Build image (core, agent, dashboard) | `./scripts/build/build-and-load-containerd.sh` |
| Chạy E2E đầy đủ + Risk Center | `./scripts/e2e/run-e2e-complete-with-monitor.sh` |
| Kiểm tra deployment | `./scripts/verify/check-full-deployment.sh` |
| Port-forward (trạng thái / start / stop) | `./scripts/utils/manage-port-forwards.sh status \| start \| stop` |

---

**Chi tiết từng script:** `scripts/README.md`  
**Deploy / containerd:** `docs/DEPLOYMENT_CONTAINERD.md`, `deploy/README.md`
