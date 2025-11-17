# KSAM Helm Chart

Helm chart để deploy Kubernetes Service Account Manager.

## Installation

```bash
helm install ksam ./helm/ksam
```

## Configuration

Xem `values.yaml` để biết các tùy chọn cấu hình.

## Components

- **Core**: KSAM Core Controller
- **Agent**: KSAM Agent (DaemonSet)
- **Dashboard**: KSAM Dashboard (Web UI)
- **PostgreSQL**: Database (optional, có thể dùng external DB)

