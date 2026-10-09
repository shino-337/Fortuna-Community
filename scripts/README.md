# Scripts

Run every script from the repository root. Installing Fortuna needs none of them when you use the Helm chart; see [Install on a cluster](../docs/getting-started/QUICKSTART.md).

| Directory | Script | Purpose |
|---|---|---|
| `demo/` | `up.sh`, `down.sh` | Create and remove the [local kind demo](../docs/getting-started/DEMO.md) |
| `build/` | `build-and-load-containerd.sh` | Build the Core, Agent and Dashboard images from source and load them into containerd ([guide](../docs/operations/DEPLOYMENT_CONTAINERD.md)) |
| | `render-manifests.sh` | Re-render `deploy/*.yaml` from the Helm chart; `--check` fails when they differ (CI) |
| `deploy/` | `ensure-storage-class.sh` | Install the `local-path` provisioner when the cluster has no StorageClass |
| | `sync-remote-agent.sh` | Install or update the Agent on remote clusters ([multi-cluster](../docs/getting-started/QUICKSTART.md#add-a-remote-cluster-optional)) |
| | `install-falco-fortuna.sh` | Install Falco with the settings the Agent reads ([runtime sensors](../docs/operations/RUNTIME_SENSORS.md)) |
| | `enable-webhook.sh` | Enable the optional admission webhook ([guide](../docs/operations/WEBHOOK.md)) |
| | `agent-credential-tool.py`, `agent-certificate-tool.py` | Issue, rotate and revoke per-Agent HTTP tokens and mTLS certificates ([guide](../deploy/scoped-agent-credentials/README.md)) |
| `utils/` | `ensure-fortuna-secrets.sh`, `create_mtls_secret.sh` | Create the application secrets and the CA and certificates for plain-manifest installs |
| | `rotate_mtls_secret.sh` | Renew the certificates from the existing CA ([certificate expiry](../docs/operations/PRODUCTION_DEPLOYMENT.md#certificate-expiry)) |
| | `load-cve-data.sh` | Download the OSV vulnerability catalog and load it into PostgreSQL; uses `sync-package-vulnerability-source.sh` |
| | `push-images-to-workers.sh` | Copy locally built images to other nodes over SSH when there is no registry; node credentials go in `push-images.config` (see the `.example`, ignored by Git) |
| | `create-github-release.sh` | Create a `vX.Y.Z` tag and GitHub release |
| `verify/` | `check-full-deployment.sh` | Check that Core, Dashboard, Agent, PostgreSQL, NATS and RBAC are healthy |
| | `verify-multicluster-sync.sh` | Compare pod counts in Kubernetes, the database and the API for every cluster |
| | `verify-scenarios.sh` | Check the attack paths Fortuna reports for the [scenarios](../scenarios/README.md) |
| | `run-local-ci.sh` | Run the CI workflow locally ([local CI](../docs/development/LOCAL_CI.md)) |
| | `run-two-cluster-integration.py`, `rehearse-populated-migration.py` | Two-cluster integration test and upgrade rehearsal on a database backup (same guide) |
| | `check-*.py`, `test-*.py` | Static checks and script tests run by CI |

## Writing scripts

- Start with `set -euo pipefail` and a header that says what the script does and how to call it.
- Make destructive steps opt-in (a flag or `DRY_RUN=1`), never hardcode credentials, hosts or kubeconfig paths, and never edit tracked manifests.
- Add the script to the table above. CI runs `bash -n` and `shellcheck --severity=error` on every `.sh` file and fails when a doc names a path that does not exist.
