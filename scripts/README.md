# Fortuna Scripts Reference

**Last Updated**: 2025-12-29

---

## Overview

This directory contains scripts for building, deploying, and managing Fortuna components.

---

## Build Scripts

### Primary Build Scripts

#### `build-with-containerd.sh`
**Purpose**: Build Fortuna images using nerdctl and import to containerd

**Usage**:
```bash
# Standard build
./scripts/build-with-containerd.sh

# Build with version
VERSION=v1.0.0 ./scripts/build-with-containerd.sh

# Export images for distribution
EXPORT_IMAGES=true ./scripts/build-with-containerd.sh
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

