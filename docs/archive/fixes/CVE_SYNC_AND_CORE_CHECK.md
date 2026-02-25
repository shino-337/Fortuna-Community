# Kiểm tra Sync CVE và Process từ Core

**Ngày:** 2026-02-24

## 0. Trạng thái CVE trên DB (kiểm tra mới nhất)

| Bảng | Số bản ghi | Ghi chú |
|------|------------|--------|
| **cves** | 0 | Reference từ OSV (cve-loader Job) — chưa load |
| **package_vulnerabilities** | 0 | Reference package↔CVE — chưa load |
| **cve_matches** | 92 | Match SBOM↔CVE (Core CVEMatcherWorker ghi) |

- **cve_matches:** 92 dòng, 92 CVE ID khác nhau; theo severity: CRITICAL 75, HIGH 13, MEDIUM 4. Ví dụ CVE: `ALPINE-CVE-2021-28831`, `ALPINE-CVE-2021-42374`, …
- **Lưu ý:** Toàn bộ 92 `cve_matches` đang trỏ tới `sbom_id = 5`; trong bảng `sboms` hiện chỉ còn bản ghi `id = 1` (pod `dns-check-*`, image busybox:1.36). Nghĩa là 92 bản ghi này là **orphan** (SBOM cũ đã bị xóa sau reset DB hoặc không còn trong `sboms`).

## 1. Tổng quan luồng CVE

- **Sync (host):** `scripts/utils/sync-package-vulnerability-source.sh` — tải OSV bulk (`all.zip` ~1GB), giải nén vào `cve-data/all`, ghi marker `.osv-sync-marker`. Không chạy trên Core.
- **Load (host → K8s Job):** `scripts/utils/load-cve-data.sh` — có thể gọi sync (khi `AUTO_SYNC_CVE_SOURCE=true`), kiểm tra `cve-data/all` có file JSON, tạo Job `cve-loader` mount hostPath `cve-data`, chạy binary `/app/cve-loader` ghi vào DB (bảng `cves`, `package_vulnerabilities`). Core **không** chạy sync hay tạo Job.
- **Core:** Chỉ đọc DB: bảng `cves` và `package_vulnerabilities` do cve-loader Job điền; Core dùng để match CVE với SBOM (CVEMatcherWorker) và ghi `cve_matches` / insights vulnerability.

## 2. Kết quả kiểm tra

### 2.1 Local (sync)

| Mục | Kết quả |
|-----|--------|
| `cve-data/all` | Tồn tại nhưng **0 file JSON** (sau khi CLEAN_LOCAL_SOURCE_AFTER_LOAD hoặc sync chưa hoàn tất) |
| `.osv-sync-marker` | Có (rỗng, Feb 23) |
| `has_local_source()` | false (vì json_count = 0) → lần chạy sync tiếp theo sẽ **tải lại** OSV (không skip) |

### 2.2 DB (từ Core / Postgres)

| Bảng | Số bản ghi |
|------|------------|
| `cves` | **0** |
| `package_vulnerabilities` | **0** |

→ Reference CVE chưa được load (đúng với việc không có file local và deploy không chạy sync khi `AUTO_SYNC_CVE_SOURCE=false`).

### 2.3 Deploy

- `deploy-fortuna-robust.sh` Step 8c gọi `load-cve-data.sh` khi `AUTO_LOAD_CVE_ON_DEPLOY=true`.
- Trong lần deploy trước, log ghi `AUTO_SYNC_CVE_SOURCE=false, skipping source sync` → sync không chạy; sau đó `No CVE JSON files found in cve-data/all` → load bị bỏ qua.
- **Không** có Job `cve-loader` nào đang tồn tại (chưa từng tạo hoặc đã xóa).

### 2.4 Kiểm tra từ Core (process / API)

- Core **không** thực hiện sync hay tạo cve-loader Job. Core chỉ:
  - Chạy migrations (tạo bảng `cves`, `package_vulnerabilities`, `cve_matches`, …).
  - Đọc `cves` + `package_vulnerabilities` khi match CVE cho SBOM (CVEMatcherWorker).
- **Endpoint kiểm tra:** Đã bổ sung trong `GET /health/dashboard-data-integrity`:
  - `crossChecks.cvesCount`: số bản ghi bảng `cves`.
  - `crossChecks.packageVulnerabilitiesCount`: số bản ghi bảng `package_vulnerabilities`.
  - Alert `cve_reference_empty` khi bảng `cves` tồn tại nhưng rỗng (gợi ý chạy sync + load).

Sau khi deploy Core mới, có thể kiểm tra từ Core bằng:

```bash
kubectl port-forward -n fortuna svc/fortuna-core 8080:8080
curl -s http://localhost:8080/health/dashboard-data-integrity | jq '.crossChecks | {cvesCount, packageVulnerabilitiesCount}, .alerts'
```

## 3. Sync CVE khi chạy thử

- Chạy với `PRECHECK_ONLY=true` và `AUTO_SYNC_CVE_SOURCE=true`: script vẫn gọi sync trước (sync tải OSV ~1GB) nên bị timeout; **sync đã bắt đầu tải** (curl tiến độ thấy vài chục MB).
- Sync hoạt động đúng: khi không có local source, script vào nhánh download; cần đủ thời gian (~10–30 phút tùy mạng) và disk (~16Gi + inode).

## 4. Khuyến nghị

1. **Bật sync khi deploy:** Khi gọi `load-cve-data.sh` từ deploy, nên để `AUTO_SYNC_CVE_SOURCE=true` (mặc định trong script là true; nếu môi trường set `false` thì cần xem lại).
2. **Chạy sync + load thủ công nếu cần ngay:**
   - Sync (nền hoặc terminal riêng): `CVE_DATA_DIR=/home/k8s/KSAM/cve-data bash scripts/utils/sync-package-vulnerability-source.sh`
   - Sau khi sync xong (có file trong `cve-data/all`): `AUTO_SYNC_CVE_SOURCE=false bash scripts/utils/load-cve-data.sh` (hoặc bật AUTO_SYNC để script tự sync lần đầu).
3. **Kiểm tra từ Core:** Dùng `GET /health/dashboard-data-integrity` (trường `cvesCount`, `packageVulnerabilitiesCount` và `alerts`) để xác nhận trạng thái reference CVE sau khi load.
