# Kiểm tra danh sách tài liệu và script

**Ngày kiểm tra:** 2026-03-09

**Đánh giá tổng quan dự án:** Xem [PROJECT_ASSESSMENT_OVERVIEW.md](PROJECT_ASSESSMENT_OVERVIEW.md).

**Trạng thái script:** Đã chạy `git checkout HEAD -- scripts/` – toàn bộ script đã được khôi phục (91 file .sh).

---

## 1. Danh sách thư mục tài liệu (docs/)

| Thư mục | Mô tả | Số file (ước lượng) |
|---------|--------|----------------------|
| docs/ (root) | README, DOCS_STRUCTURE, AGENT_CORE_*, TESTCASE_MONITOR, ... | Nhiều |
| docs/01-getting-started/ | Chuẩn bị môi trường (đã xóa trong working tree: ENVIRONMENT_REQUIREMENTS.md) | - |
| docs/02-architecture/ | Kiến trúc, ARCHITECTURE, COMPONENTS, pod-detail-*, INDEX (đã xóa) | Nhiều |
| docs/03-components/ | podDetail, sbom, dashboad (typo), ... | Nhiều |
| docs/04-development/ | migrations, MIGRATIONS, FORTUNA_ENABLE_SEED_DATA, ... | Nhiều |
| docs/05-operations/ | DEPLOYMENT, PRODUCTION_DEPLOYMENT, CLEAN_REBUILD_*, ... | Nhiều |
| docs/07-guides/ | SBOM_DASHBOARD_NO_DATA, THREAT_VELOCITY, RISK_CENTER_*, ... | Nhiều |
| docs/test-results/ | Báo cáo test, verify-*, E2E-*, FULL-RUN-*, ... | Nhiều |
| docs/e2e/ | PCE_RUNTIME_E2E_SUMMARY.md | 1 |
| docs-prod/ | Tài liệu production (04-USER_GUIDE, 05-OPERATIONS, 06-CONFIGURATION, ...) | Nhiều |

**Lưu ý:** `.gitignore` có các dòng bỏ qua một số thư mục docs (ví dụ `docs/06-reference/`, `docs/04-development/`, `docs/01-getting-started/`, …). File đã được track trước đó vẫn nằm trong git; file mới trong các thư mục bị ignore sẽ không được add.

---

## 2. Danh sách script (scripts/) – theo thư mục

### pipeline/
- full-clean-database-rebuild-deploy.sh
- clean-rebuild-redeploy-and-test.sh
- run-clean-rebuild-deploy-no-timeout.sh
- **Đã xóa trong working tree:** full-rebuild-sync-deploy-and-e2e.sh

### deploy/
- pre-deployment-checks.sh, ensure-flannel.sh, ensure-storage-class.sh
- ensure-control-plane-label.sh, ensure-cluster-addons.sh
- fix-flannel-vxlan.sh, fix-dns-config.sh, apply-core-master-only.sh
- **Đã xóa trong working tree:** deploy-fortuna-robust.sh

### clean/
- cleanup-environment.sh, clean-containerd-images.sh, clean-rebuild-dashboard.sh
- clean-host-images-and-junk.sh, clean-e2e-test-images.sh, cleanup-orphaned-migrations.sh
- **Đã xóa trong working tree:** clean-evicted-completed-pods.sh

### build/
- build-and-load-containerd.sh, build-dashboard-containerd.sh, build-production.sh
- export-agent-image-for-workers.sh

### verify/
- check-full-deployment.sh, check-env-rebuild-deploy.sh, check-pod-risk.sh
- verify-dashboard-*.sh, verify-database-schema.sh, verify-agent-*.sh
- verify-pod-data.sh, verify-test-data.sh, verify-api-detailed.sh, verify-via-api-only.sh
- verify-after-clean-rebuild-deploy.sh, verify-pod-detail-api-and-db.sh, ...
- expand-disk-worker01-remote.sh

### e2e/
- run-e2e-full.sh, run-e2e-all-verify.sh, run-e2e-with-capability-report.sh
- run-e2e-complete-with-monitor.sh, run-pod-detail-test-suite.sh, run-dashboard-data-tests.sh
- e2e-sbom-verify.sh, e2e-risk-center-verify.sh, e2e-pod-delete-cleanup-verify.sh
- test-priority1-apis.sh, test-sbom-pod-flow.sh, test-pod-*.sh, test-pod-detail-ping-flow.sh, test-pod-detail-lodash-network.sh, test-pce-api.sh, ...
- **Đã xóa trong working tree:** run-e2e-tests.sh, e2e-dashboard-data.sh, test-pce-e2e.sh, test-runtime-signals-e2e.sh

### monitor/
- monitor-testcases.sh, monitor-runtime-signals.sh, monitor-agent-core.sh, monitor-agent-core-errors.sh

### utils/
- push-images-to-workers.sh, port-forward-dashboard.sh, manage-port-forwards.sh
- create_mtls_secret.sh, import-to-containerd.sh, fix-k8s-swap-for-kubelet.sh
- sync-git-with-local.sh, sync-package-vulnerability-source.sh
- ensure-cve-tables.sh, ensure-initial-schema.sh, validate-migrations.sh
- reset-worker-node.sh, reset-worker-remote.sh, fix-dns-issues.sh
- apply-nats-single-replica.sh, remove-tracked-ignored-files.sh, ...
- **Đã xóa trong working tree:** load-cve-data.sh, sync-image-tag-and-clean.sh, sync-k8s-data.sh

---

## 3. Tại sao một số script (và docs) “bị xóa”?

### 3.1 Trạng thái trong Git

- Các file có trạng thái **D** (deleted) trong `git status`: **vẫn còn trong lịch sử git** (commit hiện tại), nhưng **đã bị xóa trên ổ đĩa** (working tree). Tức là ai/cái gì đó đã xóa file trên disk, chưa chắc đã `git add`/`git commit` việc xóa.

### 3.2 Các nguyên nhân có thể

1. **Xóa tay hoặc refactor**
   - Người dùng / dev xóa file (hoặc refactor rồi xóa file cũ) nhưng chưa muốn commit, hoặc quên commit.

2. **Script `sync-git-with-local.sh`**
   - Script này **không tự xóa file trên ổ đĩa**.
   - Nó chỉ tìm các file **đã bị xóa trên ổ đĩa** (`git ls-files --deleted`), rồi nếu người dùng gõ `y` thì chạy `git rm <file>` để **ghi nhận việc xóa vào git** (stage deletion).
   - **Trình tự điển hình:** trước đó file đã bị xóa trên disk (bởi tay hoặc công cụ khác) → sau đó chạy `sync-git-with-local.sh` và confirm → deletion được stage; nếu rồi `git commit` thì file sẽ biến mất khỏi repo.
   - Kết luận: script không “tự động xóa” file, mà chỉ “đồng bộ git với tình trạng đã xóa sẵn trên disk”.

3. **`.gitignore`**
   - `.gitignore` **chỉ ảnh hưởng file chưa được track**: không add file mới trùng pattern, **không xóa** file đã track hay xóa file trên ổ đĩa.
   - Trong repo có một số pattern như:
     - `scripts/deploy-fortuna-robust.sh`
     - `scripts/load-cve-database.sh`
     - `scripts/apply-core-master-only.sh`
     - `scripts/apply-nats-single-replica.sh`
     - và các pattern `scripts/fix-*.sh`, `scripts/clean-*.sh`, `scripts/cleanup-*.sh`, ...
   - Nếu file **đã từng được commit** trước khi có các dòng ignore này thì file vẫn được track; việc thêm vào `.gitignore` không làm file biến mất. Script “bị xóa” ở đây là do **đã mất trên disk**, không phải do `.gitignore` xóa.

4. **Công cụ / IDE**
   - Một số IDE hoặc công cụ “dọn repo” / “sync với git” có thể có tính năng xóa file local theo ignore hoặc theo trạng thái git; nếu từng chạy nhầm có thể làm mất file trên disk. Cần kiểm tra lại thao tác đã làm trên repo.

### 3.3 Cách khôi phục script/docs đã xóa trên disk (vẫn còn trong git)

File vẫn trong commit hiện tại (HEAD) thì khôi phục working tree bằng:

```bash
# Khôi phục một file
git checkout HEAD -- path/to/script.sh

# Khôi phục nhiều file (ví dụ script deploy + pipeline)
git checkout HEAD -- scripts/deploy/deploy-fortuna-robust.sh
git checkout HEAD -- scripts/pipeline/full-rebuild-sync-deploy-and-e2e.sh
git checkout HEAD -- scripts/utils/load-cve-data.sh
# ... tương tự với file khác

# Khôi phục toàn bộ file đang ở trạng thái "deleted"
git status --short | awk '/^ D|^D / {print $2}' | xargs -r git checkout HEAD --
```

Sau khi chạy, working tree lại có đủ file; nếu không muốn coi là “đã xóa trong repo” thì **đừng** chạy `sync-git-with-local.sh` và confirm `y` cho những file đó.

---

## 4. Khuyến nghị

- **Không** dùng `sync-git-with-local.sh` với confirm `y` nếu chưa chắc đã muốn xóa hẳn các file đó khỏi repo.
- Định kỳ so sánh `scripts/README.md` (và docs tương ứng) với danh sách file thực tế trong `scripts/` và `docs/` để cập nhật tài liệu và phát hiện file bị xóa nhầm.
- File quan trọng (ví dụ `deploy-fortuna-robust.sh`, `push-images-to-workers.sh`, `full-rebuild-sync-deploy-and-e2e.sh`) nên được nhắc trong README và có đường dẫn rõ ràng; khi có thay đổi (đổi tên/xóa) nên cập nhật README và DOCS_STRUCTURE ngay.

---

*Tài liệu này được tạo tự động từ kiểm tra repo; nên cập nhật lại khi cấu trúc docs/ hoặc scripts/ thay đổi.*
