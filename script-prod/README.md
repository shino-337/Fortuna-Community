# Fortuna Production Scripts

Production scripts: **build**, **deploy**, **clean**, **verify**. Configure via `config.env` (copy from `config.env.example`).

---

## Requirements

- **kubectl** pointing at the target cluster
- **nerdctl** or **docker** to build images; **containerd** namespace `k8s.io` if K8s uses containerd
- **Bash** 4+ (scripts use `set -euo pipefail`)

---

## Configuration

```bash
cp config.env.example config.env
# Edit: VERSION, REGISTRY, NAMESPACE, ...
```

| Variable | Description | Example |
|----------|-------------|---------|
| `VERSION` | Image tag (use a version for production) | `v1.0.0` |
| `REGISTRY` | Registry (leave empty for local-only build) | `registry.company.com/fortuna` |
| `NAMESPACE` | Kubernetes namespace | `fortuna` |
| `CONTAINERD_NAMESPACE` | Containerd namespace (nerdctl) | `k8s.io` |
| `SKIP_BUILD_CORE` | Skip Core build | `0` or `1` |
| `SKIP_BUILD_AGENT` | Skip Agent build | `0` or `1` |
| `SKIP_BUILD_DASHBOARD` | Skip Dashboard build | `0` or `1` |
| `PUSH_IMAGES` | Push to registry after build | `0` or `1` |
| `LOG_DIR` | Script log directory | `./logs` (created if missing) |

---

## Scripts

### build.sh

Builds Core, Agent, and Dashboard images with tag **VERSION**. Logs to `$LOG_DIR/build.log`.

```bash
./script-prod/build.sh
# Or with config loaded:
source config.env 2>/dev/null || true
VERSION=v1.0.0 ./script-prod/build.sh
```

- Uses **nerdctl** (preferred) or **docker**.
- Images: `fortuna-core:$VERSION`, `fortuna-agent:$VERSION`, `fortuna-dashboard:$VERSION`.
- If `PUSH_IMAGES=1` and `REGISTRY` is set, tags and pushes to the registry.

### deploy.sh

Deploys to the cluster: namespace, infra (PostgreSQL, NATS), mTLS (if needed), RBAC, Core, Agent, Dashboard. Uses image tag **VERSION**.

```bash
./script-prod/deploy.sh
```

- Reads `config.env` if present.
- Calls existing scripts: ensure-flannel, ensure-storage-class, deploy-fortuna-robust; then sets deployment images to VERSION.
- Log: `$LOG_DIR/deploy.log`.

### clean.sh

Removes workloads; optionally removes local images and/or DB data. **Prompts for confirmation** unless `-y` / `--yes`.

```bash
./script-prod/clean.sh
# Options:
#   --images     Remove fortuna-* images from containerd/docker
#   --db         Clear DB (DELETE data, keep schema)
#   --db-reset   Full DB reset (DROP tables)
#   -y / --yes   Skip confirmation (use with care in CI)
```

- By default only removes Core, Agent, and Dashboard in the namespace; does not remove PostgreSQL/NATS unless requested.
- Log: `$LOG_DIR/clean.log`.

### verify.sh

Checks health and status after deploy: namespace, pods Running, Core /health, Dashboard HTTP, Agent DaemonSet.

```bash
./script-prod/verify.sh
```

- Exit 0 if all checks pass; otherwise exit 1 and print errors.
- Uses `scripts/verify/check-full-deployment.sh` when available.

---

## Typical workflow

1. **First run or after code changes:**
   ```bash
   ./script-prod/build.sh
   ./script-prod/deploy.sh
   ./script-prod/verify.sh
   ```
2. **Image upgrade only:** Update VERSION in config → build → deploy → verify.
3. **Cleanup:** Run `./script-prod/clean.sh` and choose whether to remove images/DB.

---

## Logs

- Default directory: `./script-prod/logs/` (build.log, deploy.log, clean.log, verify.log).
- Override with `LOG_DIR` in config.

---

## See also

- [docs-prod/README.md](../docs-prod/README.md)
- [deploy/README.md](../deploy/README.md)
- [scripts/pipeline/full-clean-database-rebuild-deploy.sh](../scripts/pipeline/full-clean-database-rebuild-deploy.sh) (dev pipeline)
