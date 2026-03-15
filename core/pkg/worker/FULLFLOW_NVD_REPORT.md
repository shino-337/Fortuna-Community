# Báo cáo test full luồng: SBOM distroless → NVD fallback → DB → Risk

## Cách chạy

```bash
cd core
RUN_NVD_INTEGRATION=1 go test -v -run TestFullFlow_DistrolessSBOM_NVD ./pkg/worker/ -timeout 120s
```

Hoặc dùng API key (tránh rate limit):

```bash
NVD_API_KEY=your-key go test -v -run TestFullFlow_DistrolessSBOM_NVD ./pkg/worker/ -timeout 120s
```

---

## Dữ liệu thực tế dùng trong test

| Thông tin | Giá trị |
|-----------|---------|
| **Image** | `registry.k8s.io/kube-controller-manager:v1.29.15` |
| **Image digest** | `sha256:a1b2c3d4e5f6789012345678901234567890123456789012345678901234ab` |
| **Pod UID** | `a1b2c3d4-e5f6-7890-abcd-ef1234567890` |
| **Pod** | `kube-system/kube-controller-manager-xyz` |
| **Container** | `kube-controller-manager` |
| **Package** | `kube-controller-manager@v1.29.15` |
| **PURL** | `pkg:generic/kube-controller-manager@v1.29.15` |
| **SbomSource** | `distroless-heuristic` |
| **Confidence** | `medium` |

---

## Quá trình từng bước (kết quả chạy thực tế)

### BƯỚC 0: Khởi tạo DB
- SQLite in-memory.
- Migrate: `sboms`, `sbom_components`, `cves`, `package_vulnerabilities`, `cve_matches`, `insights`.
- Tạo unique index cho `cve_matches` và `insights`.

### BƯỚC 1: Tạo SBOM distroless
- Tạo 1 SBOM với image digest, tag, pod UID/name/namespace, container, `SbomSource=distroless-heuristic`.
- Tạo 1 component: `kube-controller-manager@v1.29.15`, PURL generic, `Source=distroless-heuristic`.

### BƯỚC 2: Chạy CVE matcher worker
- Gửi event `sbom.created` (SBOM_ID, PodUID, ImageDigest, ...).
- Worker: load SBOM → bulk query Postgres (ecosystem=generic, package=kube-controller-manager) → **0 CVE**.
- Kích hoạt **NVD fallback** (heuristic SBOM, package trong whitelist).
- NVD API trả **5 CVE**.
- Với mỗi CVE (constraint rỗng): `EnsureCVEExists` → ghi vào bảng `cves`; tạo **CVEMatch** với `MatchedBy=nvd-fallback`.
- Persist `cve_matches` vào DB.
- Tạo **5 vulnerability insights** cho pod (CRITICAL/HIGH/MEDIUM).

### BƯỚC 3: Kiểm tra `cve_matches`
- **Tổng match:** 5, đều **nvd-fallback**.
- Ví dụ: CVE-2020-8555, CVE-2019-11252, CVE-2020-8566, CVE-2024-0793, CVE-2025-13281.
- Mỗi match: Package=`kube-controller-manager`, Version=`v1.29.15`, MatchedBy=`nvd-fallback`.

### BƯỚC 4: Kiểm tra bảng `cves`
- **5 bản ghi CVE**, tất cả `source=nvd`.
- Mỗi bản ghi có: CVEID, Severity, CVSSScore, Description, Source=nvd (đã persist từ NVD).

### BƯỚC 5: Kiểm tra insights (risk)
- **5 vulnerability insights** cho pod UID đã cho.
- Mỗi insight: CVEID, Severity (medium/high), Title, AffectedComponent=`kube-controller-manager`, AffectedVersion=`v1.29.15`.

### BƯỚC 6: View chi tiết (như dashboard)
- Query `cve_matches` + Preload("CVE") theo SBOM ID.
- **5 CVE** hiển thị với mô tả đầy đủ từ bảng `cves` (dashboard dùng đúng luồng này).

---

## Kết quả tổng hợp

| Hạng mục | Kết quả |
|----------|--------|
| **cve_matches** | 5 (100% nvd-fallback) |
| **cves** | 5 (source=nvd) |
| **insights** | 5 (vulnerability risk) |
| **Package version** | v1.29.15 |
| **Image digest** | sha256:a1b2c3d4... |
| **Luồng** | SBOM distroless → Postgres 0 → NVD → persist CVE + matches → insights ✅ |

---

## Ghi chú

- Lỗi `no such table: insights` trong log đến từ **risk score calculation** chạy async (scorer dùng session/connection khác với test DB). Không ảnh hưởng kết quả test: bảng `insights` tồn tại và đã query được 5 insight ở BƯỚC 5.
- Test dùng NVD API thật; không mock. Cần `RUN_NVD_INTEGRATION=1` hoặc `NVD_API_KEY` để chạy.
