# Fortuna Helm chart

Installs Fortuna Core, the Agent DaemonSet, the dashboard, PostgreSQL and NATS.

```bash
helm install fortuna deploy/helm/fortuna --namespace fortuna --create-namespace
```

The chart is not published to a chart repository yet; install it from a checkout of the matching release tag. One release per cluster: the ClusterRoles, bindings and the optional ValidatingWebhookConfiguration are cluster-scoped.

## What it generates

With the defaults (`secrets.create=true`, `tls.generate=true`) the chart creates:

- `fortuna-secrets`: database URL, JWT secret, ingest token, admin password, Pod Detail encryption key; `postgres-credentials`: the PostgreSQL password. Empty values are generated once and reused on upgrade (read back with `lookup`, so `helm template` without a cluster generates new ones each time).
- `fortuna-ca-cert`, `fortuna-core-tls`, `fortuna-agent-tls`, `fortuna-webhook-tls`: a private CA (10 years) and certificates (`tls.validityDays`). The CA key is used only while rendering and is never stored, so renewing means a new CA; see `NOTES.txt` after install.

Set `secrets.create=false` and `tls.generate=false` to bring your own (the scripts in `scripts/utils/`, cert-manager in `deploy/certs/`, or an external secret store).

## Values

All settings, with comments, are in [`values.yaml`](values.yaml). The environment variables they map to are in the [configuration reference](../../../docs/reference/CONFIGURATION.md); anything without a value can be set with `core.extraEnv` or `agent.extraEnv`.

## The plain manifests

`deploy/*.yaml` is rendered from this chart with [`ci/raw-manifests-values.yaml`](ci/raw-manifests-values.yaml) (`manifestMode: raw`: no generated Secrets, optional components included). After changing a template or a default, run:

```bash
scripts/build/render-manifests.sh
```

CI runs `helm lint`, fails if `deploy/*.yaml` differs from the chart, and runs the Agent privilege guard on a rendered Helm install.
