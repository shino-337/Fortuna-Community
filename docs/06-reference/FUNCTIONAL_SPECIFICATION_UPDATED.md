# KSAM Platform - Functional Specification (Updated)

**Last Updated**: $(date)  
**Version**: 2.0

---

## Overview

This document describes the functional capabilities of the KSAM Platform, including all features, workflows, and behaviors as of the latest updates.

---

## Core Functionalities

### 1. Pod Detection and Monitoring

**Function**: Detect and monitor Kubernetes pods in real-time

**Implementation**:
- Agent runs as DaemonSet on each node
- Local pod watcher monitors pods on the node
- Asynchronous work queue prevents blocking

**Key Features**:
- Real-time pod detection
- Non-blocking SBOM processing
- Automatic retry on failures

---

### 2. SBOM Extraction

**Function**: Extract Software Bill of Materials from container images

**Process**:
1. Agent detects new/updated pod
2. Extracts container image digest
3. Runs OS-aware parsers:
   - **Debian/Ubuntu**: dpkg, npm, pip, gomod
   - **Alpine**: apk, npm, pip, gomod
   - **RHEL/CentOS**: rpm, npm, pip, gomod
4. Generates SBOM with components and PURLs
5. Sends to Core via gRPC

**Output**:
- SBOM stored in `sboms` table
- Components stored in `sbom_components` table
- PURLs parsed and normalized

**Optimizations**:
- OS-aware parser selection (reduces noise)
- Asynchronous processing (non-blocking)
- Image digest caching

---

### 3. CVE Matching

**Function**: Match detected packages against known CVEs

**Process**:

#### Step 1: Component Extraction
- Load components from `sbom_components` table
- Parse PURLs to extract:
  - Package name
  - Package version
  - Ecosystem

#### Step 2: Ecosystem Normalization
- Normalize ecosystem names:
  - `PACKAGE_TYPE_DEB` → `debian`
  - `PACKAGE_TYPE_APK` → `alpine`
  - `package_type_dpkg` → `debian`
  - `package_type_rpm` → `linux`
  - `apk` → `alpine`

#### Step 3: Bulk CVE Query
- Group packages by ecosystem
- Query `package_vulnerabilities` table in bulk
- Returns potential CVEs for each package

#### Step 4: Version Comparison
- **Debian**: Uses `go-deb-version` library
  - Handles complex formats: `2.12.7+dfsg+really2.9.14-2.1+deb13u2`
  - Supports epoch, repackaging markers
- **RPM/Alpine/npm/pypi/go**: Uses `hashicorp/go-version`
- **Unsupported**: Returns explicit error

#### Step 5: Match Creation
- Create `CVEMatch` records for vulnerable packages
- Store in `cve_matches` table
- Deduplication via unique constraint

**Output**:
- CVE matches stored in `cve_matches` table
- Includes: CVE ID, package, version, severity, CVSS

---

### 4. Insight Generation

**Function**: Generate security insights from CVE matches

**Process**:

#### Step 1: Filter by Severity
- Only process CRITICAL, HIGH, MEDIUM (configurable)
- Skip LOW severity matches

#### Step 2: Build Insights
- Use CVE matches directly (no re-query)
- Load components in bulk
- Build insight objects with:
  - Resource context (pod UID, namespace, name)
  - CVE information
  - Affected component and version
  - Recommendation

#### Step 3: Batch Upsert
- Deduplicate by `(resource_uid, cve_id, insight_type)`
- Batch insert with `ON CONFLICT DO UPDATE`
- Update existing insights if needed

**Output**:
- Insights stored in `insights` table
- Accessible via API

**Insight Fields**:
- `resource_type`: "Pod"
- `resource_uid`: Pod UID
- `insight_type`: "vulnerability"
- `severity`: "critical", "high", "medium", "low"
- `cve_id`: CVE identifier
- `affected_component`: Package name
- `affected_version`: Installed version
- `cvss`: CVSS score
- `status`: "active", "resolved", "dismissed"

---

### 5. API Services

**Function**: Expose insights and system status via REST API

#### Health Check
**GET** `/health`
- Returns system health status
- Checks database connectivity

#### Insights API
**GET** `/api/v1/insights`
- **Query Parameters**:
  - `resource_uid`: Filter by resource UID
  - `resource_type`: Filter by resource type
  - `resource_namespace`: Filter by namespace
  - `resource_name`: Filter by resource name
  - `insight_type`: Filter by insight type
  - `severity`: Filter by severity
  - `status`: Filter by status (default: "active")
  - `sbom_id`: Filter by SBOM ID
  - `page`, `pageSize`: Pagination

**Response Format**:
```json
{
  "insights": [
    {
      "id": 1,
      "resourceType": "Pod",
      "resourceNamespace": "fortuna",
      "resourceName": "test-pod",
      "resourceUid": "0c12ecc6-68f5-4f88-8caf-a4c61648ca09",
      "insightType": "vulnerability",
      "severity": "medium",
      "title": "CVE-2025-26434 in libxml2",
      "description": "Vulnerability CVE-2025-26434 (MEDIUM) detected...",
      "recommendation": "Update image/package to a fixed version...",
      "cveId": "CVE-2025-26434",
      "affectedComponent": "libxml2",
      "affectedVersion": "2.12.7+dfsg+really2.9.14-2.1+deb13u2",
      "cvss": 5.0,
      "status": "active",
      "detectedAt": "2025-12-28T11:57:16Z"
    }
  ],
  "page": 1,
  "pageSize": 50,
  "total": 1
}
```

---

## Database Operations

### SBOM Storage

**Table**: `sboms`
- **Insert**: On SBOM receipt from agent
- **Update**: On SBOM reuse (same image digest)
- **Query**: By `pod_uid`, `pod_name`, `image_digest`

**Table**: `sbom_components`
- **Insert**: Batch insert on SBOM creation
- **Deduplication**: Unique constraint on `(sbom_id, purl)`
- **Query**: By `sbom_id`, `component_name`

### CVE Matching

**Table**: `cve_matches`
- **Insert**: Batch insert with `ON CONFLICT DO NOTHING`
- **Deduplication**: Unique constraint on `(sbom_id, package_name, cve_id)`
- **Query**: By `sbom_id`, `pod_uid`, `severity`

**Process**:
1. Query `package_vulnerabilities` by ecosystem and package names
2. Compare versions using ecosystem-specific libraries
3. Create matches for vulnerable packages
4. Batch insert with deduplication

### Insight Management

**Table**: `insights`
- **Insert/Update**: Batch upsert with `ON CONFLICT DO UPDATE`
- **Deduplication**: Unique constraint on `(resource_uid, cve_id, insight_type)`
- **Query**: By `resource_uid`, `severity`, `status`

**Process**:
1. Filter CVE matches by severity
2. Build insights from matches and components
3. Deduplicate before batch insert
4. Upsert with conflict resolution

---

## Event Processing

### NATS JetStream Events

#### SBOM_CREATED Event
- **Publisher**: Core (after SBOM storage)
- **Subscriber**: CVE Matcher Worker
- **Payload**:
  ```json
  {
    "sbom_id": 85,
    "pod_uid": "0c12ecc6-68f5-4f88-8caf-a4c61648ca09",
    "pod_namespace": "fortuna",
    "pod_name": "test-pod",
    "container_name": "test-container",
    "container_image": "nginx:latest",
    "image_digest": "sha256:..."
  }
  ```

#### Processing Flow
1. CVE Matcher Worker receives event
2. Loads SBOM from database
3. Matches CVEs
4. Persists matches
5. Generates insights
6. Stores insights

---

## Error Handling

### SBOM Extraction Errors
- **Parser Not Found**: Logged as warning, skipped
- **Extraction Failure**: Retried via NATS redelivery
- **gRPC Failure**: Retried with exponential backoff

### CVE Matching Errors
- **Version Parsing Error**: Logged, match skipped
- **Unsupported Ecosystem**: Logged, explicit error returned
- **Database Error**: Retried via NATS redelivery

### Insight Generation Errors
- **Component Not Found**: Logged, insight skipped
- **Batch Upsert Error**: Retried via NATS redelivery
- **Deduplication Error**: Handled by pre-insert deduplication

---

## Performance Characteristics

### SBOM Extraction
- **Time**: 1-3 minutes per pod (depends on image size)
- **Throughput**: Asynchronous, non-blocking
- **Optimization**: Work queue prevents blocking

### CVE Matching
- **Time**: 2-3 seconds for 150 packages
- **Queries**: 1-3 bulk queries (by ecosystem)
- **Optimization**: Bulk queries, batch processing

### Insight Generation
- **Time**: <500ms for 100 insights
- **Queries**: 2 bulk queries (components + matches)
- **Optimization**: Batch upsert, deduplication

---

## Configuration

### Agent Configuration
- **SBOM Queue Size**: Configurable
- **Parser Selection**: OS-aware
- **gRPC Timeout**: Configurable

### Core Configuration
- **Severity Filter**: CRITICAL, HIGH, MEDIUM (configurable)
- **Batch Sizes**: Configurable (default: 100-500)
- **NATS Retry**: Exponential backoff

### Database Configuration
- **Connection Pool**: Configurable
- **Query Timeout**: Configurable
- **Indexes**: Optimized for common queries

---

## Recent Updates (2025-12-28)

### Version Comparison Enhancement
- **Change**: Integrated `go-deb-version` library for Debian
- **Impact**: Accurate version comparison for complex Debian formats
- **Example**: `2.12.7+dfsg+really2.9.14-2.1+deb13u2` now parsed correctly

### Insight Worker Optimization
- **Change**: Removed unnecessary re-query, use matches directly
- **Impact**: Faster processing, no timing issues
- **Result**: Reliable insight generation

### Schema Alignment
- **Change**: Removed `fixed_version` from insights operations
- **Impact**: No SQL errors, correct data storage
- **Result**: Stable database operations

### Deduplication Enhancement
- **Change**: Pre-insert deduplication for insights
- **Impact**: No ON CONFLICT errors
- **Result**: Reliable batch processing

---

**Document Version**: 2.0  
**Last Updated**: $(date)

