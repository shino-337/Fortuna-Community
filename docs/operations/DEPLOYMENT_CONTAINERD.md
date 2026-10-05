# Build and deploy from source

Use this path when you need images built from the current source tree. Everyone else should install the published images as described in [Install on a cluster](../getting-started/QUICKSTART.md).

## Requirements

- A Kubernetes cluster that uses containerd, and `kubectl`.
- One build backend: `nerdctl`, `docker` (with buildx) or `buildctl`. Docker-built images are imported with `ctr`.
- Helm 3 for a fresh install.

## Build the images

From the repository root:

```bash
export FORTUNA_LOCAL_TAG="local-$(git rev-parse --short HEAD)-$(date -u +%Y%m%d%H%M)"
VERSION="$FORTUNA_LOCAL_TAG" ./scripts/build/build-and-load-containerd.sh
```

The script builds `fortuna-core`, `fortuna-agent` and `fortuna-dashboard` and loads them into the `k8s.io` containerd namespace that kubelet uses. Useful options: `BUILD_TOOL=docker|nerdctl|buildctl`, `NO_CACHE=true`, `SKIP_DASHBOARD=true`, and `BUILD_CORE_ONLY`, `BUILD_AGENT_ONLY` or `BUILD_DASHBOARD_ONLY=true`. To push to a registry instead, set `REGISTRY=<registry/prefix> PUSH_IMAGES=true`.

Use a new tag for every build. A local image exists only on the node where it was built: on a multi-node cluster, push to a registry, or copy the images to the other nodes with `./scripts/utils/push-images-to-workers.sh`. Builds also fill the BuildKit cache; wait until no node reports `DiskPressure=True` before rolling out (`buildctl prune --all` frees the cache without touching loaded images).

## Fresh install

Install the chart with the local images (empty registry, local tag):

```bash
helm install fortuna deploy/helm/fortuna --namespace fortuna --create-namespace \
  --set image.registry= --set image.tag="$FORTUNA_LOCAL_TAG"
```

Then follow [Wait for the rollout](../getting-started/QUICKSTART.md#wait-for-the-rollout) and the rest of the install guide.

## Update an existing installation

Change only the images. PostgreSQL, NATS, secrets and runtime sensors stay as they are. With Helm:

```bash
helm upgrade fortuna deploy/helm/fortuna -n fortuna --reuse-values --set image.tag="$FORTUNA_LOCAL_TAG"
```

For a plain-manifest install:

```bash
kubectl -n fortuna set image deployment/fortuna-core core="fortuna-core:$FORTUNA_LOCAL_TAG"
kubectl -n fortuna set image daemonset/fortuna-agent agent="fortuna-agent:$FORTUNA_LOCAL_TAG" image-export="fortuna-agent:$FORTUNA_LOCAL_TAG"
kubectl -n fortuna set image deployment/fortuna-dashboard dashboard="fortuna-dashboard:$FORTUNA_LOCAL_TAG"
kubectl -n fortuna rollout status deployment/fortuna-core --timeout=600s
NAMESPACE=fortuna ./scripts/verify/check-full-deployment.sh
```

Applying the manifests from `deploy/` again afterwards switches the workloads back to the published images.

## When a rollout fails

- Read the pod events and logs before `kubectl rollout undo`. Rolling back the image does not reverse database migrations.
- Before upgrading an installation that has data, rehearse the migrations on a backup with [`rehearse-populated-migration.py`](../development/LOCAL_CI.md#two-cluster-integration-and-migration-rehearsal).
- A Core startup failure on `backfill risk_scores cluster ownership` means older unowned risk-score rows collide with owned ones. Restore the previous Core image and reconcile those rows deliberately; never delete or merge them as a deploy step.
- If a pod still runs old code, compare the workload image with the image present on the node that runs the pod.

To start over with an empty database, follow [backup and reset](BACKUP_AND_RESET.md), then reload the vulnerability catalog with `./scripts/utils/load-cve-data.sh`. Database resets do not clear node-local Falco state; read [preserve Falco delivery state](RUNTIME_SENSORS.md#preserve-falco-delivery-state) first.
