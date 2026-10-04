# Production deployment

Install with the [Quickstart](../01-getting-started/QUICKSTART.md), then apply the changes below before relying on Fortuna outside a lab.

## Images

- Pin an immutable release tag or digest from a registry that every node can reach. Do not use `latest` or locally loaded containerd images on multi-node clusters.
- After a rollout, confirm the running image: `kubectl -n fortuna get pods -o jsonpath='{range .items[*]}{.metadata.name}{" "}{.status.containerStatuses[0].imageID}{"\n"}{end}'`.
- An image rollback does not reverse database migrations. Take a database backup before upgrading.

## Credentials

- Set `FORTUNA_ADMIN_PASSWORD` from your secret manager. Never use the bootstrap password `Fortuna_ChangeMe_123!` outside an isolated lab.
- While `FORTUNA_ADMIN_PASSWORD` is set, Core syncs the admin password to that value whenever it no longer matches, so rotate the admin password through the secret, not only in the UI.
- Keep the CA from `create_mtls_secret.sh` (`.certs/`) offline and access-controlled. Do not regenerate it with `MTLS_REGEN=1` on a running installation; Agents would stop being trusted.
- Move from the shared ingest token to per-Agent credentials: HTTP tokens and gRPC client certificates bound to one Agent and one cluster. See [scoped Agent credentials](../../deploy/scoped-agent-credentials/README.md) and [per-Agent mTLS](../../deploy/scoped-agent-credentials/MTLS.md). Without the gRPC registry, Core refuses Agent SBOM writes over gRPC.

## Network

- Apply `deploy/infrastructure/network-policies.yaml` and confirm your CNI enforces NetworkPolicy. NATS has no client authentication; only Core may reach it.
- Set `FORTUNA_TRUSTED_PROXIES` on Core to the CIDR of the dashboard proxy or ingress only. Core trusts no proxy when the variable is unset, but the bundled manifest sets it to all RFC 1918 ranges so that it works out of the box; narrow it.
- Set `FORTUNA_WS_ALLOWED_ORIGINS` to the real dashboard origin(s).
- Expose the dashboard through your ingress with TLS. Do not expose Core's HTTP port publicly; remote Agents should reach it through a dedicated, access-controlled endpoint.

## Data

- Use `deploy/infrastructure/postgresql-with-age.yaml` (the default) or an external PostgreSQL; `postgresql.yaml` is a simpler fallback only.
- Back up PostgreSQL regularly (`pg_dump -Fc`). The [maintenance procedure](BACKUP_AND_RESET.md) shows a verified backup before a reset.
- Check migration and admin state after upgrades:

```bash
kubectl -n fortuna exec deploy/postgres -- psql -U postgres -d fortuna -c "select max(version), count(*) from schema_migrations;"
kubectl -n fortuna exec deploy/postgres -- psql -U postgres -d fortuna -c "select username, role, must_change_password, bootstrap_credential from users order by id;"
```

## Optional components

- **Runtime evidence:** install Falco with `./scripts/deploy/install-falco-fortuna.sh`. The built-in eBPF sensor is experimental; keep `EBPF_SIMULATE` disabled. Signed sensor health is described in [runtime source health](RUNTIME_SENSORS.md).
- **Admission webhook:** enable only through `./scripts/deploy/enable-webhook.sh`; see the [webhook guide](WEBHOOK.md).
- **ServiceAccount revocation:** previewed, reviewed revocation needs a per-cluster kubeconfig; see [ServiceAccount mutations](SERVICEACCOUNT_MUTATIONS.md).
