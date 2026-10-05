# Production deployment

Install with the [Quickstart](../getting-started/QUICKSTART.md), then apply the changes below before relying on Fortuna outside a lab.

## Images

- Pin an immutable release tag or digest from a registry that every node can reach. Do not use `latest` or locally loaded containerd images on multi-node clusters.
- After a rollout, confirm the running image: `kubectl -n fortuna get pods -o jsonpath='{range .items[*]}{.metadata.name}{" "}{.status.containerStatuses[0].imageID}{"\n"}{end}'`.
- An image rollback does not reverse database migrations. Take a database backup before upgrading.

## Credentials

- Set `FORTUNA_ADMIN_PASSWORD` from your secret manager. Never use the bootstrap password `Fortuna_ChangeMe_123!` outside an isolated lab.
- While `FORTUNA_ADMIN_PASSWORD` is set, Core syncs the admin password to that value whenever it no longer matches, so rotate the admin password through the secret, not only in the UI.
- Keep the CA from `create_mtls_secret.sh` (`.certs/`) offline and access-controlled. Do not regenerate it with `MTLS_REGEN=1` on a running installation; Agents would stop being trusted.
- Move from the shared ingest token to per-Agent credentials: HTTP tokens and gRPC client certificates bound to one Agent and one cluster. See [scoped Agent credentials](../../deploy/scoped-agent-credentials/README.md) and [per-Agent mTLS](../../deploy/scoped-agent-credentials/MTLS.md). Without the gRPC registry, Core refuses Agent SBOM writes over gRPC.

## Agent

- Keep the bundled Agent security settings. [Agent privileges](../reference/SECURITY.md#agent-privileges) explains each one and what remains; `python3 scripts/verify/test-agent-privileges.py` checks your copy of the manifests.
- Install metrics-server if you want CPU and memory in Pod Detail. The Agent reads it through the read-only Metrics API.
- If node-root exposure through the containerd socket is not acceptable, remove the `image-export` container and set `SBOM_PREFER_REGISTRY=1`; images from private registries then get no SBOM.
- Alert on denied or non-read requests from the `fortuna-agent` ServiceAccount in the Kubernetes audit log.

## Network

- Apply `deploy/infrastructure/network-policies.yaml` and confirm your CNI enforces NetworkPolicy. NATS has no client authentication; only Core may reach it. Agent pods accept no ingress.
- Set `FORTUNA_TRUSTED_PROXIES` on Core to the CIDR of the dashboard proxy or ingress only. Core trusts no proxy when the variable is unset, but the bundled manifest sets it to all RFC 1918 ranges so that it works out of the box; narrow it.
- Set `FORTUNA_WS_ALLOWED_ORIGINS` to the real dashboard origin(s).
- Core serves unauthenticated Prometheus metrics on port 9091 (`FORTUNA_METRICS_ADDR`). The Service does not expose it; restrict it to your Prometheus with a NetworkPolicy, or unset the variable to disable it.
- Expose the dashboard through your ingress with TLS. Do not expose Core's HTTP port publicly; remote Agents should reach it through a dedicated, access-controlled endpoint.

## Data

- Use the bundled `deploy/infrastructure/postgresql.yaml` (single instance) or an external PostgreSQL (`postgresql.enabled=false` and `secrets.databaseUrl` in the Helm chart).
- Back up PostgreSQL regularly (`pg_dump -Fc`). The [maintenance procedure](BACKUP_AND_RESET.md) shows a verified backup before a reset.
- Check migration and admin state after upgrades:

```bash
kubectl -n fortuna exec deploy/postgres -- psql -U postgres -d fortuna -c "select max(version), count(*) from schema_migrations;"
kubectl -n fortuna exec deploy/postgres -- psql -U postgres -d fortuna -c "select username, role, must_change_password, bootstrap_credential from users order by id;"
```

## Optional components

- **Runtime evidence:** install Falco with `./scripts/deploy/install-falco-fortuna.sh`. The built-in eBPF sensor is an experimental no-op scaffold and the bundled manifest grants it no capabilities; leave it disabled. Signed sensor health is described in [runtime source health](RUNTIME_SENSORS.md).
- **Admission webhook:** enable only through `./scripts/deploy/enable-webhook.sh`; see the [webhook guide](WEBHOOK.md).
- **ServiceAccount revocation:** previewed, reviewed revocation needs a per-cluster kubeconfig; see [ServiceAccount mutations](SERVICEACCOUNT_MUTATIONS.md).
