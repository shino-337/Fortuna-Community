# Local Containerd Build And Deploy

Use this path for local development when you need to build images from the current source tree. For normal users and multi-node clusters, prefer published images from GHCR as described in [Quickstart](../01-getting-started/QUICKSTART.md).

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

## Full Local Pipeline

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
