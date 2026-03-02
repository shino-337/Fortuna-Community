# Fortuna Helm Chart

Helm chart cho Fortuna – nền tảng bảo mật và quản lý rủi ro Kubernetes (SBOM, CVE, PCE, Runtime Signals).

## Yêu cầu

- Kubernetes 1.24+
- Helm 3
- **mTLS secrets** phải tồn tại trước khi cài: `fortuna-core-tls`, `fortuna-agent-tls`, `fortuna-ca-cert`, `fortuna-webhook-tls` (ví dụ: `./scripts/utils/create_mtls_secret.sh`)
- **PostgreSQL** và **NATS** chạy trong cùng namespace (hoặc cấu hình `core.env.databaseUrl` / `core.env.natsEndpoint` trỏ tới endpoint khác)

## Cài đặt

```bash
# Từ thư mục gốc repo
helm upgrade --install fortuna ./helm/fortuna -n fortuna --create-namespace

# Override image tag / registry
helm upgrade --install fortuna ./helm/fortuna -n fortuna --create-namespace \
  --set image.tag=v1.0.0 \
  --set image.registry=registry.company.com/fortuna

# Dùng file values tùy chỉnh
helm upgrade --install fortuna ./helm/fortuna -n fortuna -f my-values.yaml
```

## Giá trị chính (values.yaml)

| Nhóm | Key | Mô tả |
|------|-----|--------|
| image | image.registry, image.tag, image.pullPolicy | Registry, tag, pull policy chung |
| core | core.replicaCount, core.env.*, core.resources | Core deployment |
| agent | agent.env.*, agent.resources | Agent DaemonSet |
| dashboard | dashboard.replicaCount, dashboard.service.type | Dashboard |
| tls | tls.coreSecretName, tls.agentSecretName, tls.caSecretName, tls.webhookSecretName | Tên secret mTLS |

## Gỡ cài đặt

```bash
helm uninstall fortuna -n fortuna
```

Lưu ý: PVC (PostgreSQL, NATS) và namespace không bị xóa tự động. Xóa thủ công nếu cần.

## Liên kết

- [deploy/README.md](../../deploy/README.md) – Deploy bằng kubectl
- [docs-prod/](../../docs-prod/) – Tài liệu production
