# NVD_API_KEY — Cấu hình tích hợp NVD (Core)

## Mục đích

**Fortuna Core** dùng [NVD API 2.0](https://nvd.nist.gov/developers/start-here) làm **fallback** khi OSV/Postgres không có kết quả khớp (đặc biệt SBOM heuristic / distroless). Biến môi trường **`NVD_API_KEY`** là **tùy chọn** nhưng **được khuyến nghị** trên môi trường production để tăng **rate limit** (NIST: không có key ~5 request/30s; có key ~50 request/30s — tham khảo chính sách NVD hiện hành).

## Luồng trong code

| Thành phần | File | Hành vi |
|------------|------|--------|
| Tạo client | `core/pkg/cve/database/manager.go` → `NewNVDClientForManager()` | Đọc `NVD_API_KEY`; nếu có → `client.SetAPIKey(key)` |
| HTTP | `core/pkg/cve/database/nvd/client.go` | Header `apiKey: <key>` khi gọi `services.nvd.nist.gov` |
| Tắt NVD | Cùng `manager.go` | `FORTUNA_NVD_DISABLED=1` hoặc `true` → không tạo client (không fallback NVD) |

## Cách lấy API key

1. Đăng ký tại: [Request an API Key | NVD](https://nvd.nist.gov/developers/request-an-api-key).
2. Giữ key bí mật; **không** commit vào git.

## Gán giá trị cho người dùng / vận hành

### Chạy local / shell

```bash
export NVD_API_KEY="your-nvd-api-key-here"
# Tùy chọn: bật test tích hợp NVD
export RUN_NVD_INTEGRATION=1
```

### Kubernetes (Deployment / Pod)

Inject qua **Secret** (khuyến nghị):

```yaml
# Ví dụ: tạo secret (một lần)
# kubectl create secret generic fortuna-nvd -n <namespace> --from-literal=nvd-api-key='<YOUR_KEY>'

apiVersion: apps/v1
kind: Deployment
metadata:
  name: fortuna-core
spec:
  template:
    spec:
      containers:
        - name: core
          env:
            - name: NVD_API_KEY
              valueFrom:
                secretKeyRef:
                  name: fortuna-nvd
                  key: nvd-api-key
                  optional: true   # optional: true nếu cho phép chạy không có key
```

Nếu dùng **Helm / Kustomize**, map tương đương: `env.valueFrom.secretKeyRef`.

### Docker Compose (ví dụ)

```yaml
services:
  fortuna-core:
    environment:
      NVD_API_KEY: ${NVD_API_KEY:-}
```

File `.env` cạnh compose (không commit):

```env
NVD_API_KEY=your-nvd-api-key-here
```

## Biến liên quan

| Biến | Ý nghĩa |
|------|--------|
| `NVD_API_KEY` | Key NVD; tăng hạn mức gọi API. |
| `FORTUNA_NVD_DISABLED` | `1` / `true` → tắt hoàn toàn client NVD. |
| `RUN_NVD_INTEGRATION` | Chủ yếu cho **test** (`go test`): `1` cùng với key để chạy integration test NVD. |

## Kiểm tra

- Log Core: khi 429, client log gợi ý set `NVD_API_KEY` (`nvd/client.go`).
- Test (cần DB + key hoặc `RUN_NVD_INTEGRATION=1`): xem `core/pkg/worker/FULLFLOW_NVD_REPORT.md`.

## Tham khảo thêm

- `core/README.md` — mục **NVD API (CVE matcher)**.
- `docs/03-components/sbom/SBOM_Flow_distroless_Remediation.md` — bảng env Core.
