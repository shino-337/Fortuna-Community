# Local Containerd Build And Deploy

Use this path for local development when you need to build images from the current source tree. For normal users and multi-node clusters, prefer published images from GHCR as described in [Quickstart](../getting-started/QUICKSTART.md).

## Requirements

- Kubernetes cluster using containerd.
- `kubectl`.
- One build backend: `nerdctl`, `docker`, or `buildctl`.
- `ctr` when Docker-built images must be imported into containerd.

## Build Images

From the repository root:

```bash
./scripts/build/build-and-load-containerd.sh
```

Useful options:

```bash
BUILD_TOOL=docker ./scripts/build/build-and-load-containerd.sh
NO_CACHE=true ./scripts/build/build-and-load-containerd.sh
SKIP_DASHBOARD=true ./scripts/build/build-and-load-containerd.sh
VERSION=my-local-tag ./scripts/build/build-and-load-containerd.sh
```

The script builds `fortuna-core`, `fortuna-agent`, and `fortuna-dashboard`, then loads tags into the containerd namespace used by Kubernetes.

## Update an Existing Local Single-Node Installation

For an existing `fortuna` namespace, use a new tag and update only the three
workload images. This preserves PostgreSQL, NATS, secrets, and runtime sensor
configuration. Do not use the full-clean pipeline or a database reset for a
routine application update.

```bash
kubectl config current-context
kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.name}{" DiskPressure="}{range .status.conditions[?(@.type=="DiskPressure")]}{.status}{end}{"\n"}{end}'
kubectl -n fortuna get deploy/fortuna-core deploy/fortuna-dashboard ds/fortuna-agent

FORTUNA_LOCAL_TAG="local-$(git rev-parse --short HEAD)-$(date -u +%Y%m%d%H%M)"
BUILD_TOOL=nerdctl VERSION="$FORTUNA_LOCAL_TAG" ./scripts/build/build-and-load-containerd.sh

# The image build cache also consumes node disk. Do not begin rollout while
# the node reports DiskPressure. Prune only the BuildKit cache if needed;
# this does not remove the images loaded into containerd/k8s.io.
df -h /
buildctl du
kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.name}{" DiskPressure="}{range .status.conditions[?(@.type=="DiskPressure")]}{.status}{end}{"\n"}{end}'
# If BuildKit cache is reclaimable and disk is tight:
# buildctl prune --all
# Wait until every target node reports DiskPressure=False.

kubectl -n fortuna set image deployment/fortuna-core core="fortuna-core:$FORTUNA_LOCAL_TAG"
kubectl -n fortuna rollout status deployment/fortuna-core --timeout=300s
kubectl -n fortuna set image daemonset/fortuna-agent agent="fortuna-agent:$FORTUNA_LOCAL_TAG" image-export="fortuna-agent:$FORTUNA_LOCAL_TAG"
kubectl -n fortuna rollout status daemonset/fortuna-agent --timeout=300s
kubectl -n fortuna set image deployment/fortuna-dashboard dashboard="fortuna-dashboard:$FORTUNA_LOCAL_TAG"
kubectl -n fortuna rollout status deployment/fortuna-dashboard --timeout=300s

NAMESPACE=fortuna ./scripts/verify/check-full-deployment.sh
```

Check the container names in the current workloads before `set image`; the
commands above match the bundled manifests. `IfNotPresent` allows kubelet to
use the locally loaded, uniquely tagged images. A local image only exists on
the node where it was built, so do not use this path for a multi-node rollout
unless the tag is imported on every eligible node. Record the tag and worktree
state: a dirty worktree cannot be reconstructed from its Git commit alone.
Applying the GHCR-tagged manifests afterward can revert these image settings.
If rollout fails, inspect pod events/logs before using `kubectl rollout undo`;
an image rollback does not reverse database migrations.
In particular, a startup failure on `backfill risk_scores cluster ownership`
means legacy unowned and already-owned risk-score rows collide on a unique
identity. Restore the previous Core image and reconcile those rows under an
explicit data-retention decision; do not delete or merge them as an automatic
deploy step.
On a single-node cluster, kubelet may continue rejecting pods briefly after
disk space is reclaimed; wait for `DiskPressure=False` before retrying rather
than repeatedly restarting workloads.

## Full Local Pipeline

Before a rebuild or reset, read [preserve Falco delivery state](RUNTIME_SENSORS.md#preserve-falco-delivery-state): routine rebuilds and database resets do not clear node-local Falco state.

### Rebuild/reset commands

For a clean local rebuild and deploy:

```bash
export FORTUNA_PACKAGE_SOURCE=local
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --full
```

Common variants:

```bash
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --full --db-reset
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --full --with-runtime
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --only-core
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --only-dashboard
```

The pipeline derives `VERSION` from Git unless `VERSION` is set. Use `FORTUNA_PACKAGE_SOURCE=local` when the rollout should use locally built `fortuna-*:${VERSION}` images. The default package source is `github`, which keeps workloads on `ghcr.io/shino-337/fortuna-community/*:${FORTUNA_VERSION}`.

To deploy images already built and loaded into the local containerd namespace,
set the exact `VERSION` and add `--skip-rebuild`. The pipeline verifies that all
three versioned images exist, then updates the manifests and workload image
references to that tag. For a fresh application database, add `--db-reset` only
after taking and checking the dump described below; this does not delete NATS
or any PVC.

`--db-reset` is destructive: it recreates the `public` schema in the dedicated
`fortuna` database, removing users, vulnerability catalogs, migration history,
and all other application rows. It does not remove the PostgreSQL PVC or
Kubernetes Secrets. Take and verify a restricted-access `pg_dump -Fc` backup
before running it; the [deployment maintenance guide](BACKUP_AND_RESET.md)
has a command sequence. The Core image must be able to migrate a fresh database.
After the reset, confirm an admin was bootstrapped from the current Secret and
reload the OSV/CVE catalog if no startup source is configured. A clean reset
does not validate migration of populated legacy data, so keep that as a
separate acceptance gate.

On a single-node local-image deployment, the post-reset CVE loader can run as
a Kubernetes Job using the newly deployed Core image and in-cluster PostgreSQL
DNS. For a CPU-saturated lab node, lower only its scheduling request:

```bash
CVE_LOAD_MODE=job CORE_IMAGE="fortuna-core:${VERSION}" \
  CVE_LOADER_CPU_REQUEST=25m CLEAN_LOCAL_SOURCE_AFTER_LOAD=false \
  ./scripts/verify/ensure-cve-catalog-ready.sh
```

The guard verifies database rows and an active catalog generation; merely
having local marker files is not sufficient after a reset. Keep the source
files until the Job completes successfully.

## Multi-Node Clusters

Local containerd images exist only on the node where they were built. For multi-node clusters:

1. Prefer a registry and set workloads to `ghcr.io/shino-337/fortuna-community/<image>:<tag>`.
2. If registryless, copy images to workers:

```bash
./scripts/utils/push-images-to-workers.sh
```

Use registry tags for production. Registryless copying is a local development fallback.

## Verify

```bash
kubectl get pods -n fortuna -o wide
kubectl rollout status -n fortuna deployment/fortuna-core --timeout=300s
kubectl rollout status -n fortuna daemonset/fortuna-agent --timeout=300s
kubectl rollout status -n fortuna deployment/fortuna-dashboard --timeout=300s
```

If a pod still runs old code, verify the image tag in the workload and the image available on the node that scheduled the pod.
