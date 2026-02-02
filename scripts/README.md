# Fortuna Scripts Reference

**Last Updated**: 2026-01-31

---

## Overview

Scripts for building, deploying, testing, and cleaning Fortuna components.

---

## Pipeline (Clean / Rebuild / Deploy)

### `full-clean-rebuild-redeploy.sh` (recommended for full reset)
Full clean (all images, cache, port-forwards, optional DB E2E data) → Rebuild (core, agent, dashboard) → Redeploy.

```bash
NO_CACHE=true ./scripts/full-clean-rebuild-redeploy.sh        # no-cache rebuild
NO_CACHE=true ./scripts/full-clean-rebuild-redeploy.sh --db    # also clean E2E data from Postgres
# Options: --skip-clean | --skip-rebuild | --skip-deploy | --db
```

After a long run, if Core deployment was removed during cleanup, re-apply Core manually:
`kubectl apply -f deploy/fortuna-core-deployment.yaml` then `kubectl rollout status deployment/fortuna-core -n fortuna`.

### `full-clean-rebuild-deploy.sh`
Uses `cleanup-environment.sh` (keeps latest 3 images per component) and `build-and-load-containerd.sh` with optional `NO_CACHE=true`.

```bash
NO_CACHE=true ./scripts/full-clean-rebuild-deploy.sh
# Options: --skip-clean | --skip-rebuild | --skip-deploy | --db
```

### `cleanup-environment.sh`
Clean K8s: port-forward, E2E/test namespaces, completed/failed pods, old images (keeps latest 3 per component), build cache.

```bash
./scripts/cleanup-environment.sh
./scripts/cleanup-environment.sh --db   # also remove E2E test data from Postgres
```

### `check-full-deployment.sh`
Verify cluster, namespace, workloads, pods, services, Core health, dashboard, agent DaemonSet.

```bash
./scripts/check-full-deployment.sh
```

### `test-priority1-apis.sh`
Calls Core APIs from inside the Core pod (promotion-rules, runtime-signals). Uses pod selector `app.kubernetes.io/component=core`. When Core has `AUTH_ENABLED=true`, API responses may require an `Authorization` header; call from dashboard or with a bearer token for full pass.

```bash
./scripts/test-priority1-apis.sh
```

### `run-e2e-full.sh`
E2E run with detailed report: cluster/pods, Core API (health and API responses; APIs may return 401 when auth is enabled), DB row counts, dashboard. Output: `docs/test-results/E2E-FULL-<timestamp>.md`.

```bash
./scripts/run-e2e-full.sh
```

---

## Build Scripts

### Primary Build Scripts

#### `build-and-load-containerd.sh`
**Purpose**: Build Fortuna images (core, agent) with nerdctl and load into containerd (namespace k8s.io)

**Usage**:
```bash
./scripts/build-and-load-containerd.sh
# Optional: VERSION=v1.0.0; EXPORT_IMAGES=true for export
```

**Output**: Images in containerd namespace `k8s.io`

---

#### `build-and-import-containerd.sh`
**Purpose**: Complete workflow - build and import to containerd

**Usage**:
```bash
./scripts/build-and-import-containerd.sh
```

---

#### `build-production.sh`
**Purpose**: Build images for production registry (Docker)

**Usage**:
```bash
# Build only
./scripts/build-production.sh

# Build and push
PUSH_IMAGES=true ./scripts/build-production.sh
```

---

#### `import-to-containerd.sh`
**Purpose**: Import images from tar files to containerd

**Usage**:
```bash
# Import from tar file
./scripts/import-to-containerd.sh /path/to/image.tar

# Import using nerdctl
./scripts/import-to-containerd.sh -m nerdctl /path/to/image.tar

# Import from registry
./scripts/import-to-containerd.sh -m nerdctl docker.io/fortuna/core:v1.0.0
```

---

## Deployment Scripts

### Primary Deployment Scripts

#### `deploy-fortuna-robust.sh`
**Purpose**: Comprehensive deployment with DNS fallback and verification

**Usage**:
```bash
# Standard deployment
./scripts/deploy-fortuna-robust.sh

# Without IP fallback
USE_IP_FALLBACK=false ./scripts/deploy-fortuna-robust.sh
```

**Features**:
- Pre-deployment checks
- Comprehensive cleanup
- DNS testing with IP fallback
- Sequential deployment
- Verification at each step

---

#### `quick-deploy-containerd.sh`
**Purpose**: Quick deploy for containerd environments

**Usage**:
```bash
./scripts/quick-deploy-containerd.sh
```

---

#### `pre-deployment-checks.sh`
**Purpose**: Validate cluster readiness before deployment

**Usage**:
```bash
./scripts/pre-deployment-checks.sh
```

**Checks**:
- Kubernetes cluster accessibility
- Namespace existence
- CoreDNS health
- DNS resolution
- Network connectivity
- Node labels

---

## Cleanup Scripts

#### `clean-containerd-images.sh`
**Purpose**: Remove old Fortuna images from containerd

**Usage**:
```bash
# Dry run
./scripts/clean-containerd-images.sh --dry-run

# Clean all fortuna images
./scripts/clean-containerd-images.sh

# Clean with custom prefix
./scripts/clean-containerd-images.sh --prefix ksam
```

---

#### `clean-all-containerd-images.sh`
**Purpose**: Quick cleanup of all Fortuna images

**Usage**:
```bash
./scripts/clean-all-containerd-images.sh
```

---

## Utility Scripts

#### `create-mtls-secrets.sh`
**Purpose**: Generate mTLS certificates for Core and Agent

**Usage**:
```bash
./scripts/create-mtls-secrets.sh
```

---

#### `fix-dns-issues.sh`
**Purpose**: Troubleshoot and fix DNS resolution issues

**Usage**:
```bash
./scripts/fix-dns-issues.sh
```

---

#### `copy-containerd-images-to-nodes.sh`
**Purpose**: Distribute images to all cluster nodes

**Usage**:
```bash
./scripts/copy-containerd-images-to-nodes.sh
```

---

## Special Purpose Scripts

#### `load-cve-database.sh`
**Purpose**: Load CVE data into database

**Usage**:
```bash
./scripts/load-cve-database.sh
```

---

#### `apply-core-master-only.sh`
**Purpose**: Deploy Core only on master node

**Usage**:
```bash
./scripts/apply-core-master-only.sh
```

---

#### `apply-nats-single-replica.sh`
**Purpose**: Configure NATS for single replica mode

**Usage**:
```bash
./scripts/apply-nats-single-replica.sh
```

---

## Migration Scripts (Archive)

These scripts are archived as they were one-time migrations:

- `migrate-imports.sh`
- `cleanup-orphaned-migrations.sh`
- `validate-migrations.sh`

---

## Workflow Examples

### Complete Build and Deploy

```bash
# 1. Clean old images
./scripts/clean-containerd-images.sh

# 2. Build images
./scripts/build-with-containerd.sh

# 3. Deploy
./scripts/deploy-fortuna-robust.sh
```

### Multi-Node Deployment

```bash
# 1. Build on master
./scripts/build-with-containerd.sh

# 2. Distribute to all nodes
./scripts/copy-containerd-images-to-nodes.sh

# 3. Deploy
./scripts/deploy-fortuna-robust.sh
```

### Troubleshooting

```bash
# 1. Pre-deployment checks
./scripts/pre-deployment-checks.sh

# 2. Fix DNS if needed
./scripts/fix-dns-issues.sh

# 3. Deploy
./scripts/deploy-fortuna-robust.sh
```

---

## Script Organization

### Active Scripts
- Build scripts (4)
- Deployment scripts (3)
- Cleanup scripts (2)
- Utility scripts (6)
- Special purpose (3)

### Archived Scripts
- Obsolete scripts moved to `scripts/archive/`
- See `PROJECT_CLEANUP_ANALYSIS.md` for details

---

## Best Practices

1. **Always run pre-deployment checks** before deploying
2. **Clean old images** before building new ones
3. **Use robust deployment script** for production
4. **Verify images** after building
5. **Check logs** if deployment fails

---

**For detailed documentation, see**:
- `docs/05-operations/CONTAINERD_BUILD_GUIDE.md`
- `docs/05-operations/DEPLOYMENT_ISSUES_COMPREHENSIVE.md`
- `docs/01-getting-started/DEPLOYMENT_QUICK_START.md`

