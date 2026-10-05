# Changelog

## Unreleased

### Docs

- Removed internal working notes (audit plan, remediation status, dated integration and performance records, public-release checklist). The reference docs now describe current behavior only.
- The local CI guide moved to `docs/development/LOCAL_CI.md` and also covers the two-cluster integration test and the populated-migration rehearsal.
- Merged the component catalog into the architecture guide and the use cases into the user guide, and removed the per-folder index pages; `docs/README.md` is the single index.

### Scripts

- Removed the lab pipeline (`full-clean-database-rebuild-deploy.sh`), `deploy-fortuna-robust.sh` and the helpers only it used (Flannel install and VXLAN repair, control-plane labelling, DNS and prerequisite checks), the shell E2E suite, and the clean, monitor and one-off verify scripts. Install with Helm or the plain manifests; build from source with `build-and-load-containerd.sh` and deploy with `helm install --set image.tag=...`.
- `verify-k8s-e2e.sh` is now `scripts/verify/verify-scenarios.sh`, and `test-webhook-bootstrap.py` moved to `scripts/verify/`.

## v1.0.0 (refreshed 2026-10-05)

The `v1.0.0` tag was moved from the July 2026 build (`895d73d`) to the current `main`, and its images were rebuilt. Installs that pinned `sha-895d73df322c` keep the original build. The original release notes are kept in the GitHub release history.

### Install

- Helm chart in `deploy/helm/fortuna`. It generates the application secrets and a private CA with the Core, Agent and webhook certificates, keeps them on upgrade, and never stores the CA key.
- `deploy/*.yaml` is rendered from the chart and checked in CI. Workload images are `ghcr.io/shino-337/fortuna-community/fortuna-*:latest` instead of local build tags.
- One PostgreSQL manifest: `postgresql-with-age.yaml` was merged into `infrastructure/postgresql.yaml`.
- The dashboard nginx ConfigMap was removed. The dashboard image renders its own config and reads the cluster DNS server from the pod, instead of a hardcoded `10.96.0.10`. If you apply the new image with an old manifest, remove the `nginx-config` volume mount.
- The dashboard Service is now `ClusterIP`. Use `kubectl port-forward`, an Ingress, or the Helm value `dashboard.service.type=LoadBalancer`.
- The install scripts no longer edit tracked manifests. `kubectl set image` failures stop the run. Failed pre-deployment checks stop `deploy-fortuna-robust.sh` unless `SKIP_PREDEPLOY_CHECKS=true`. Loading CVE data during a deploy is opt-in (`AUTO_LOAD_CVE_ON_DEPLOY=true`).
- `install-falco-fortuna.sh` requires Helm to be installed already. It no longer downloads an installer and pipes it to bash.

### Security

- Agent (#58, #63):
  - No `pods/exec` and no `nodes/proxy`. CPU and memory come from `metrics.k8s.io`, and process and socket data from the read-only host `/proc`.
  - No host namespaces and no capabilities. The root filesystem is read-only.
  - The containerd socket is mounted only in a sidecar that has no token.
  - CI enforces all of this with an allowlist guard.
- Pod Detail encryption:
  - The install scripts and the chart generate a key.
  - An invalid key stops Core instead of being ignored.
  - `POD_DETAIL_ENCRYPTION_KEY_PREVIOUS` keeps older rows readable after a rotation.
  - Values that cannot be decrypted show a placeholder instead of ciphertext.
- The Agent redacts credential-looking command-line arguments and URL passwords. Process-diff events keep only the executable path.
- mTLS:
  - The CA is now valid for 10 years. Certificates are still valid for 365 days.
  - `rotate_mtls_secret.sh` now issues new certificates from the existing CA. Before, it did nothing once the secrets existed.
  - See "Certificate expiry" in the production guide.
- Seccomp `RuntimeDefault`, no ServiceAccount token, and no privilege escalation for the dashboard, PostgreSQL, NATS, the Core init containers and the risk CronJob. The CronJob's curl image is pinned.

### Since the July build

- Audit remediation #23 to #57: cluster-scoped identity and inventory, per-Agent HTTP and gRPC credentials, runtime evidence and source health, reviewed ServiceAccount revocation, and availability fixes.
- Documentation consolidated (#59 to #62). New configuration reference in `docs/reference/CONFIGURATION.md`, checked against the manifests and source in CI.

### CI

- `gofmt`, `shellcheck --severity=error`, `helm lint`, a drift check for rendered manifests, the Agent privilege guard on a Helm install, and a docs, manifest and source sync check.
