# Nội dung có thể xóa / image outdated

Chạy kiểm tra nhanh: `./scripts/clean/check-and-clean-host-resources.sh` (chỉ xem) hoặc `./scripts/clean/check-removable-content.sh` (nếu có).

---

## 1. Trong repo (có thể xóa thủ công)

| Mục | Vị trí | Kích thước | Ghi chú |
|-----|--------|------------|--------|
| Binary build cũ | `./fortuna-core` | ~76MB | Build local cũ, đã có trong .gitignore. Xóa: `rm -f fortuna-core` |
| Tar rỗng | `./fortuna-agent-latest.tar` | 0 | Export image trống. Xóa: `rm -f fortuna-agent-latest.tar` |
| Test results (root) | `./test-results/` | ~12K | 1–2 file report. Có thể xóa nếu không cần: `rm -rf test-results` |
| Báo cáo test cũ | `docs/test-results/*.md` | ~492K, 20 file | Theo DOCS_STRUCTURE: giữ README + 1–2 báo cáo mới nhất, còn lại chuyển `docs/archive/test-results/` hoặc xóa file cũ (theo ngày trong tên file) |
| Backup session | `backup-session-*/` (nếu có) | — | Thư mục backup tạm, không track git. Có thể xóa khi không cần: `rm -rf backup-session-*` |

---

## 2. Image containerd/nerdctl (outdated / có thể dọn)

- **Tag trùng nhiều digest:** `fortuna-core:latest` và `fortuna-agent:latest` thường có 2 image ID (bản mới build và bản cũ). Pod đang chạy dùng 1 ID; ID còn lại là “outdated” (vẫn tốn dung lượng).
- **Dangling:** `<none>:<none>` – image không còn tag, có thể xóa bằng prune.
- **Tag cũ:** `fortuna-*:v1.0.0-49-g649e4ff79-dirty` – nếu chỉ dùng `:latest` thì có thể xóa tag này để gọn.

**Cách dọn (trên host có nerdctl):**

```bash
# Chỉ kiểm tra (disk, images, cache)
./scripts/clean/check-and-clean-host-resources.sh

# Xóa image fortuna + prune cache + temp (sẽ mất image local → cần rebuild)
./scripts/clean/check-and-clean-host-resources.sh --clean -y

# Hoặc chỉ xóa image fortuna (giữ image K8s khác)
./scripts/clean/clean-containerd-images.sh          # xóa fortuna
./scripts/clean/clean-containerd-images.sh --dry-run # xem trước
```

Sau khi xóa image fortuna, cần rebuild nếu cần chạy Core/Agent: `./scripts/build/build-and-load-containerd.sh`.

---

## 3. Build cache (nerdctl)

- Builder cache có thể vài GB. Prune: `nerdctl --namespace k8s.io builder prune -a -f` (hoặc dùng `check-and-clean-host-resources.sh --clean -y`).

---

## 4. CVE data (sau khi load xong)

- **Lưu ý:** `cve-data/all/` là **nguồn CVE từ OSV** (sync từ OSV bulk), không phải "dữ liệu test". Chỉ nên dọn khi đã load xong vào PostgreSQL và không cần giữ file JSON local.
- Nếu đã load CVE vào DB và không cần giữ file JSON local: script load có option `CLEAN_LOCAL_SOURCE_AFTER_LOAD=true` (mặc định) để dọn `cve-data/all` **sau khi load**. Kiểm tra trong `scripts/utils/load-cve-data.sh`. Để có lại dữ liệu OSV sau khi dọn: chạy `./scripts/utils/sync-package-vulnerability-source.sh`.
