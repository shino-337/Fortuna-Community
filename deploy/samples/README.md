# Deploy Samples

These files are examples for installs that use GitHub/GHCR packages.

## Private GHCR Packages

Use `ghcr-pull-secret.example.yaml` only as a template. Replace placeholders with a GitHub token that has `read:packages`, or create the same secret with:

```bash
kubectl -n fortuna create secret docker-registry ghcr-pull \
  --docker-server=ghcr.io \
  --docker-username="$GITHUB_USER" \
  --docker-password="$GITHUB_TOKEN" \
  --dry-run=client -o yaml | kubectl apply -f -
```

After `deploy/fortuna-rbac.yaml` creates the ServiceAccounts, attach the pull secret:

```bash
kubectl apply -f deploy/samples/ghcr-imagepullsecrets.example.yaml
```

## GitHub Package Tags

`github-packages-kustomization.example.yaml` shows how to pin the checked-in workload manifests to the GitHub release package tag `v1.0.0` with Kustomize.

Use it as a template in your own overlay directory named `kustomization.yaml`. Because the example references manifests from the parent `deploy/` directory, render overlays that keep this relative layout with:

```bash
kubectl kustomize <overlay-dir> --load-restrictor=LoadRestrictionsNone
```

The default package images are:

- `ghcr.io/shino-337/fortuna-community/fortuna-core:v1.0.0`
- `ghcr.io/shino-337/fortuna-community/fortuna-agent:v1.0.0`
- `ghcr.io/shino-337/fortuna-community/fortuna-dashboard:v1.0.0`
