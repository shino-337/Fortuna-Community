# Fortuna Helm Chart (Legacy)

**Note**: This is a legacy chart. For new deployments, use `helm/fortuna/` chart.

This chart is maintained for backward compatibility only.

## Installation

```bash
helm install fortuna ./helm/ksam --namespace fortuna --create-namespace
```

## Configuration

See `values.yaml` for configuration options.

## Components

- **Core**: Fortuna Core Controller
- **Agent**: Fortuna Agent (DaemonSet)
- **Dashboard**: Fortuna Dashboard (Web UI)
- **PostgreSQL**: Database (optional, can use external DB)

## Migration

To migrate from this chart to the new `fortuna/` chart:

1. Export current values: `helm get values fortuna -n fortuna > old-values.yaml`
2. Uninstall old chart: `helm uninstall fortuna -n fortuna`
3. Install new chart: `helm install fortuna ./helm/fortuna -n fortuna -f old-values.yaml`

