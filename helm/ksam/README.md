# Fortuna Helm Chart (Legacy)

Helm chart để deploy Kubernetes Service Account Manager.

## Installation

```bash
helm install fortuna ./helm/ksam
```

## Configuration

Xem `values.yaml` để biết các tùy chọn cấu hình.

## Components

- **Core**: Fortuna Core Controller
- **Agent**: Fortuna Agent (DaemonSet)
- **Dashboard**: Fortuna Dashboard (Web UI)
- **PostgreSQL**: Database (optional, có thể dùng external DB)

