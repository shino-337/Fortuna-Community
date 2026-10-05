# Try Fortuna locally in one command

This sets up a throwaway [kind](https://kind.sigs.k8s.io/) cluster on your machine, installs Fortuna from the published images, and loads one example: a pod whose ServiceAccount is bound to `cluster-admin`. It is meant for evaluation, not production.

## Requirements

- Docker running, with about 4 CPUs and 6 GB of memory available to it
- [kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) 0.24 or newer
- `kubectl` and `openssl`
- Internet access to pull images from `ghcr.io` and Docker Hub

## Start

```bash
git clone https://github.com/shino-337/Fortuna-Community.git
cd Fortuna-Community
./scripts/demo/up.sh
```

The first run takes several minutes: images are pulled and Core runs its database migrations. When it finishes, the script prints how to open the dashboard and the generated admin password:

```bash
export KUBECONFIG=$PWD/.fortuna-demo/kubeconfig
kubectl -n fortuna port-forward svc/fortuna-dashboard 8081:80
# open http://localhost:8081 and log in as admin
```

The demo uses its own kubeconfig in `.fortuna-demo/`, so your existing kubectl contexts are not changed. The admin password is saved in `.fortuna-demo/admin-password`.

## What to look at

The example is the S2 scenario from [`scenarios/s2-rbac-only.yaml`](../../scenarios/s2-rbac-only.yaml):

```
pod fortuna-test/rbac-pod
  └─ runs as ServiceAccount sa-rbac
       └─ bound by ClusterRoleBinding crb-rbac-admin
            └─ to ClusterRole cluster-admin  →  full control of the cluster
```

The Agent syncs inventory every 5 minutes, so the finding can take a few minutes to appear. Then follow [Your first RBAC investigation](FIRST_FINDING.md) from step 3: find the pod, follow the path, remove the binding and confirm the path disappears after the next sync.

## Options

| Variable | Default | Purpose |
|---|---|---|
| `FORTUNA_VERSION` | `latest` | Image tag to install, for example `v1.0.0` |
| `FORTUNA_REGISTRY` | `ghcr.io/shino-337/fortuna-community` | Image registry prefix |
| `FORTUNA_DEMO_CLUSTER` | `fortuna-demo` | kind cluster name |
| `FORTUNA_DEMO_PORT` | `8081` | Local port shown in the port-forward hint |
| `FORTUNA_DEMO_TIMEOUT` | `600s` | How long to wait for each component |

`latest` follows the `main` branch, which matches the manifests in a `main` checkout. To try a release, check out its tag and set `FORTUNA_VERSION` to the same tag.

## Clean up

```bash
./scripts/demo/down.sh
```

This deletes the kind cluster and `.fortuna-demo/`.

## Troubleshooting

- **A rollout times out.** Check `kubectl -n fortuna get pods` and `kubectl -n fortuna logs deploy/fortuna-core`. Core's first start runs many migrations; rerun `./scripts/demo/up.sh` (it reuses the cluster) or raise `FORTUNA_DEMO_TIMEOUT`.
- **Pods stay `Pending`.** Docker may not have enough CPU or memory; raise its limits.
- **Image pull errors.** Check that your machine can reach `ghcr.io`.
- **No CPU or memory in Pod Detail.** kind has no metrics-server, and the Agent reads usage only from the Metrics API. Install it with `kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml` and add `--kubelet-insecure-tls` to its arguments (kind kubelets use self-signed certificates); keep that flag to the demo.

For a real cluster, use the [Quickstart](QUICKSTART.md) instead.
