# SBOM Generator Component

The SBOM Generator creates Software Bill of Materials for container images, enabling vulnerability scanning and supply chain analysis.

---

## Overview

**Status**: ✅ Production Ready (MVP2)
**Method**: Custom local-first extraction
**Dependencies**: Zero (no external tools)
**Performance**: ~30 seconds per image

---

## Key Features

✅ **Zero Dependencies**
- No Syft, Trivy, or external tools required
- Pure Go implementation
- Works in air-gapped environments

✅ **Package Manager Support**
- **Debian/Ubuntu**: dpkg database (`/var/lib/dpkg/status`)
- **Alpine**: apk database (`/lib/apk/db/installed`)
- **Red Hat/CentOS**: rpm database (`/var/lib/rpm`)
- **Node.js**: package.json + package-lock.json
- **Python**: requirements.txt, Pipfile, setup.py
- **Go**: go.mod

✅ **PURL Generation**
- Standard Package URLs for all components
- Ecosystem-specific formatting
- Version normalization

---

## Architecture

```
Pod Created/Updated
    ↓
Image Digest Extracted
    ↓
Check SBOM Cache (by digest)
    ├─▶ Found → Use Cached SBOM
    │
    └─▶ Not Found:
        ↓
    Pull Image Layers (local first, registry if needed)
        ↓
    Extract Package Databases
        ├─▶ /var/lib/dpkg/status (Debian)
        ├─▶ /lib/apk/db/installed (Alpine)
        ├─▶ /var/lib/rpm (Red Hat)
        ├─▶ /usr/local/lib/node_modules (Node)
        └─▶ /usr/local/lib/python*/site-packages (Python)
        ↓
    Parse Package Information
        ↓
    Generate PURLs
        ↓
    Create SBOM
        ├─▶ sboms table
        └─▶ sbom_components table
        ↓
    Emit SBOM_CREATED event (NATS: ksam.sbom.created)
        ↓
    CVE Matcher Worker
        ↓
    Persist cve_matches + Create Vulnerability Insights
```

---

## Quick Start

### Verify SBOM Generation

```bash
# Check existing SBOMs
kubectl exec -n ksam postgres-* -- psql -U postgres -d ksam -c \
  "SELECT image_name, image_tag, component_count, created_at
   FROM sboms
   WHERE deleted_at IS NULL
   ORDER BY created_at DESC
   LIMIT 10;"
```

### Trigger SBOM for New Image

```bash
# Deploy pod with new image
kubectl run test-sbom --image=nginx:1.19.0 -n default

# Wait for SBOM generation (~30 seconds)
sleep 35

# Check SBOM created
kubectl exec -n ksam postgres-* -- psql -U postgres -d ksam -c \
  "SELECT * FROM sboms WHERE image_name LIKE '%nginx%' AND image_tag = '1.19.0';"
```

### View SBOM Components

```bash
# Get SBOM ID
SBOM_ID=$(kubectl exec -n ksam postgres-* -- psql -U postgres -d ksam -t -c \
  "SELECT id FROM sboms WHERE image_name LIKE '%nginx%' AND image_tag = '1.19.0' LIMIT 1;")

# View components
kubectl exec -n ksam postgres-* -- psql -U postgres -d ksam -c \
  "SELECT component_name, component_version, purl
   FROM sbom_components
   WHERE sbom_id = '$SBOM_ID'
   LIMIT 20;"
```

---

## Database Schema

### sboms Table

```sql
CREATE TABLE sboms (
    id SERIAL PRIMARY KEY,
    image_name VARCHAR(255) NOT NULL,
    image_tag VARCHAR(255) NOT NULL,
    image_digest VARCHAR(255) UNIQUE NOT NULL,
    sbom_format VARCHAR(50) NOT NULL DEFAULT 'cyclonedx-json',
    sbom_content JSONB NOT NULL,
    component_count INT NOT NULL DEFAULT 0,
    os_packages INT DEFAULT 0,
    language_packages INT DEFAULT 0,
    generator VARCHAR(100) DEFAULT 'custom',
    generator_version VARCHAR(50),
    generated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    use_count INT DEFAULT 1,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);
```

### sbom_components Table

```sql
CREATE TABLE sbom_components (
    id SERIAL PRIMARY KEY,
    sbom_id INTEGER REFERENCES sboms(id),
    component_name VARCHAR(255) NOT NULL,
    component_version VARCHAR(255) NOT NULL,
    component_type VARCHAR(50),
    purl VARCHAR(512),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    -- Dedup key for cache-first upsert
    UNIQUE(sbom_id, purl)
);
```

### cve_matches Table

```sql
CREATE TABLE cve_matches (
    id SERIAL PRIMARY KEY,
    sbom_id INTEGER NOT NULL REFERENCES sboms(id),
    component_id INTEGER NOT NULL REFERENCES sbom_components(id),
    cve_id VARCHAR(20) NOT NULL,
    severity VARCHAR(20) NOT NULL,
    cvss_score DECIMAL(3,1),
    fixed_version VARCHAR(255),
    matched_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    UNIQUE(sbom_id, component_id, cve_id)
);
```

---

## Implementation Details

### Package Extraction Examples

#### Debian/Ubuntu (dpkg)

**Source File**: `/var/lib/dpkg/status`

**Format**:
```
Package: libssl1.1
Version: 1.1.1f-1ubuntu2
Architecture: amd64
Status: install ok installed
```

**PURL**: `pkg:deb/ubuntu/libssl1.1@1.1.1f-1ubuntu2?arch=amd64`

#### Alpine (apk)

**Source File**: `/lib/apk/db/installed`

**Format**:
```
P:musl
V:1.2.2-r0
A:x86_64
```

**PURL**: `pkg:apk/alpine/musl@1.2.2-r0?arch=x86_64`

#### Node.js (npm)

**Source File**: `/package.json` or `/usr/local/lib/node_modules/*/package.json`

**Format**:
```json
{
  "name": "express",
  "version": "4.17.1"
}
```

**PURL**: `pkg:npm/express@4.17.1`

---

## Configuration

### Environment Variables

```yaml
# SBOM Generation
SBOM_ENABLED: "true"
SBOM_CACHE_ENABLED: "true"
SBOM_LOCAL_EXTRACTION: "true"

# Image Pulling
IMAGE_PULL_TIMEOUT: "300s"
IMAGE_REGISTRY_AUTH: "false"

# Processing
SBOM_WORKER_COUNT: "5"
SBOM_BATCH_SIZE: "10"
```

---

## Performance

| Image Size | Layers | Components | Time | Notes |
|------------|--------|------------|------|-------|
| nginx:1.19.0 | 6 | 135 | ~25s | Debian base |
| nginx:alpine | 7 | 70 | ~15s | Alpine base |
| node:14 | 9 | 200+ | ~45s | Many npm packages |
| python:3.9 | 10 | 150+ | ~35s | pip packages |

**Optimization**:
- Image digest caching (prevents duplicate extraction)
- Local layer access when available
- Parallel component parsing
- Database batching

---

## Troubleshooting

### SBOM Not Generated for Image

**Check Pod Events**:
```bash
kubectl describe pod <pod-name> -n <namespace>
```

**Check Core Logs**:
```bash
kubectl logs -n ksam ksam-core-* | grep -i "sbom\\|image"
```

**Common Issues**:
1. **Image pull failed**: Check image registry access
2. **Unsupported base image**: Check if package manager is supported
3. **Timeout**: Large images may exceed timeout

### Duplicate SBOM Components

**Symptom**: Database errors about duplicate keys

**Fixed in MVP2**: Unique constraint on `(sbom_id, component_name, component_version, purl)`

**Verification**:
```sql
-- Check for duplicates
SELECT
    sbom_id,
    component_name,
    component_version,
    COUNT(*)
FROM sbom_components
GROUP BY sbom_id, component_name, component_version
HAVING COUNT(*) > 1;
```

### SBOM Missing Components

**Symptom**: Component count lower than expected

**Debug**:
```bash
# Enable debug logging
kubectl set env deployment/ksam-core -n ksam LOG_LEVEL=debug

# Check extraction logs
kubectl logs -n ksam ksam-core-* | grep -i "extract\\|package\\|component"
```

**Common Causes**:
- Package database not in expected location
- Unsupported package manager format
- Corrupted package database

---

## API Integration

### REST API

```bash
# List SBOMs
curl http://localhost:8080/api/v1/sboms

# Get specific SBOM
curl http://localhost:8080/api/v1/sboms/{sbom_id}

# Get SBOM by image digest
curl http://localhost:8080/api/v1/sboms/by-digest/{digest}

# Get SBOM components
curl http://localhost:8080/api/v1/sboms/{sbom_id}/components
```

### Response Format

```json
{
  "id": "uuid-123",
  "image_name": "nginx",
  "image_tag": "1.19.0",
  "image_digest": "sha256:abc123...",
  "component_count": 135,
  "created_at": "2025-01-15T10:00:00Z",
  "components": [
    {
      "name": "libssl1.1",
      "version": "1.1.1f-1ubuntu2",
      "type": "library",
      "purl": "pkg:deb/ubuntu/libssl1.1@1.1.1f-1ubuntu2"
    }
  ]
}
```

---

## Comparison with External Tools

| Feature | KSAM SBOM | Syft | Trivy |
|---------|-----------|------|-------|
| **Binary Size** | 0MB (built-in) | ~50MB | ~200MB |
| **Dependencies** | None | None | None |
| **Debian Support** | ✅ | ✅ | ✅ |
| **Alpine Support** | ✅ | ✅ | ✅ |
| **RPM Support** | ✅ | ✅ | ✅ |
| **npm Support** | ✅ | ✅ | ✅ |
| **Python Support** | ✅ | ✅ | ✅ |
| **Go Modules** | ✅ | ✅ | ✅ |
| **Rust Cargo** | ❌ | ✅ | ✅ |
| **Java Maven** | ❌ | ✅ | ✅ |
| **SPDX Output** | ❌ | ✅ | ✅ |
| **CycloneDX** | ❌ | ✅ | ✅ |
| **Database Storage** | ✅ | ❌ | ❌ |
| **Caching** | ✅ | ❌ | ✅ |

**KSAM Advantage**: Zero dependencies, database-native, built-in caching

**External Tool Advantage**: More package manager support, standard format output

---

## Related Components

- [CVE Scanner](../cve-scanner/) - Uses SBOMs for vulnerability matching
- [Risk Engine](../risk-engine/) - Creates insights from SBOM+CVE matches
- [Core](../core/) - Orchestrates SBOM generation

---

## Future Enhancements

### Planned Features

1. **Additional Package Managers**:
   - Java (Maven, Gradle)
   - Rust (Cargo)
   - Ruby (Bundler)

2. **Standard Formats**:
   - SPDX 2.3 output
   - CycloneDX 1.4 output
   - JSON export

3. **Registry Integration**:
   - Direct registry scanning (without pod deployment)
   - Scheduled scanning
   - Scan on image push (admission webhook)

4. **Performance**:
   - Parallel layer extraction
   - Incremental SBOM updates
   - Layer-level caching

---

## Testing

### Manual Test

```bash
# Test SBOM generation
./scripts/test_sbom_generation.sh

# Expected output:
# ✅ SBOM created for nginx:1.19.0
# ✅ 135 components detected
# ✅ PURLs generated correctly
```

### Unit Tests

```bash
cd core/pkg/sbom
go test -v
```

---

**Last Updated**: December 16, 2025
**Status**: ✅ Production Ready
**Documentation**: See archive/mvp2/ for implementation details
