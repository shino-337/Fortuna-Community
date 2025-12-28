# KSAM Platform Architecture - Updated Documentation

**Last Updated**: $(date)  
**Version**: 2.0

---

## Overview

KSAM (Kubernetes Service Account Management) Platform is a comprehensive security and risk management system for Kubernetes clusters. It provides real-time vulnerability detection, SBOM (Software Bill of Materials) extraction, CVE matching, and security insights generation.

---

## System Components

### 1. Agent (DaemonSet)
- **Purpose**: Runs on each Kubernetes node
- **Responsibilities**:
  - Pod detection and monitoring
  - SBOM extraction from container images
  - Asynchronous SBOM processing queue
  - gRPC communication with Core

**Key Features**:
- **Asynchronous Work Queue**: Prevents blocking pod detection during SBOM extraction
- **OS-Aware Parser Selection**: Only runs relevant parsers (dpkg for Debian, apk for Alpine, etc.)
- **Multi-Parser Support**: dpkg, apk, rpm, npm, pip, gomod

### 2. Core (Deployment)
- **Purpose**: Central processing and storage
- **Responsibilities**:
  - SBOM storage and management
  - CVE matching and vulnerability detection
  - Insight generation
  - Policy evaluation
  - API services
  - Database management

**Key Features**:
- **NATS JetStream**: Event-driven architecture
- **PostgreSQL**: Persistent storage
- **gRPC Server**: Agent communication
- **REST API**: External access
- **Workers**: Asynchronous processing

### 3. Infrastructure
- **PostgreSQL**: Database for SBOMs, CVEs, insights, policies
- **NATS JetStream**: Message queue for event processing
- **Prometheus**: Metrics collection

---

## Data Flow Architecture

### End-to-End Flow

```
1. Pod Created
   ↓
2. Agent Detects Pod (Local Pod Watcher)
   ↓
3. Agent Enqueues Pod to SBOM Queue (Asynchronous)
   ↓
4. Agent Worker Extracts SBOM
   ↓
5. Agent Sends SBOM to Core via gRPC
   ↓
6. Core Stores SBOM in Database
   ↓
7. Core Publishes SBOM_CREATED Event (NATS)
   ↓
8. CVE Matcher Worker Processes Event
   ↓
9. CVE Matching Logic:
   - Parse PURLs from SBOM components
   - Normalize ecosystem (PACKAGE_TYPE_DEB → debian)
   - Query CVE database
   - Version comparison (ecosystem-specific)
   ↓
10. Persist CVE Matches to Database
   ↓
11. Generate Insights (CRITICAL/HIGH/MEDIUM)
   ↓
12. Store Insights in Database
   ↓
13. API Exposes Insights
```

---

## Database Schema

### Key Tables

#### `sboms`
- **Purpose**: Store extracted SBOMs
- **Key Columns**:
  - `id`, `pod_uid`, `pod_name`, `namespace`, `container_name`
  - `image_name`, `image_digest`
  - `sbom_content` (JSONB)
  - `created_at`, `updated_at`

#### `sbom_components`
- **Purpose**: Store individual packages from SBOMs
- **Key Columns**:
  - `sbom_id`, `component_name`, `component_version`
  - `purl` (Package URL)
  - `ecosystem` (derived from PURL)

#### `cve_matches`
- **Purpose**: Store matched CVEs for packages
- **Key Columns**:
  - `sbom_id`, `pod_uid`, `container_name`
  - `cve_id`, `package_name`, `package_version`
  - `purl`, `severity`, `cvss` (real)
  - `fixed_version`, `matched_by`
  - **Unique Constraint**: `(sbom_id, package_name, cve_id)`

#### `insights`
- **Purpose**: Store security insights for resources
- **Key Columns**:
  - `resource_type`, `resource_namespace`, `resource_name`, `resource_uid`
  - `insight_type` (vulnerability, misconfiguration, etc.)
  - `severity` (critical, high, medium, low)
  - `cve_id`, `affected_component`, `affected_version`
  - `cvss` (real), `status` (active, resolved, dismissed)
  - **Unique Constraint**: `(resource_uid, cve_id, insight_type)`

#### `cves`
- **Purpose**: Store CVE metadata
- **Key Columns**: `cve_id`, `severity`, `cvss_score`, `title`

#### `package_vulnerabilities`
- **Purpose**: Store package-to-CVE mappings
- **Key Columns**: `cve_id`, `package_name`, `ecosystem`, `constraint`

---

## CVE Matching Logic

### Process Flow

1. **SBOM Component Extraction**
   - Extract components from SBOM
   - Parse PURLs to get package name, version, ecosystem

2. **Ecosystem Normalization**
   - `PACKAGE_TYPE_DEB` → `debian`
   - `PACKAGE_TYPE_APK` → `alpine`
   - `PACKAGE_TYPE_RPM` → `linux`
   - `package_type_dpkg` → `debian`
   - `package_type_rpm` → `linux`

3. **Bulk CVE Query**
   - Group packages by ecosystem
   - Query `package_vulnerabilities` table in bulk
   - Returns potential CVEs for each package

4. **Version Comparison**
   - **Debian**: Uses `github.com/knqyf263/go-deb-version` library
   - **RPM**: Uses `hashicorp/go-version` (semver-based)
   - **Alpine**: Uses `hashicorp/go-version` (semver-based)
   - **npm/pypi/go**: Uses `hashicorp/go-version` (semver)
   - **Unsupported**: Returns explicit error (no implicit fallback)

5. **CVE Match Creation**
   - Create `CVEMatch` records for vulnerable packages
   - Store in `cve_matches` table with deduplication

6. **Insight Generation**
   - Filter by severity (CRITICAL/HIGH/MEDIUM)
   - Build insights from CVE matches
   - Batch upsert to `insights` table

---

## Version Comparison Strategy

### Architectural Decision (ADR-001)

**Decision**: Use ecosystem-specific version comparison libraries

**Rationale**:
- Avoid re-implementing complex version algorithms
- Ensure accuracy for ecosystem-specific formats
- Leverage battle-tested solutions

**Implementation**:
- **Debian**: `github.com/knqyf263/go-deb-version`
- **RPM/Alpine/npm/pypi/go**: `hashicorp/go-version`
- **Unsupported**: Explicit error (no implicit semver fallback)

**Example**:
```go
// Debian version: 2.12.7+dfsg+really2.9.14-2.1+deb13u2
v1, _ := debversion.NewVersion(installed)
v2, _ := debversion.NewVersion(constraint)
cmp := v1.Compare(v2) // -1, 0, or 1
```

---

## Insight Generation Logic

### Process Flow

1. **CVE Match Persistence**
   - Persist CVE matches to `cve_matches` table
   - Use `ON CONFLICT DO NOTHING` for deduplication

2. **Insight Building**
   - Use persisted matches directly (no re-query)
   - Filter by severity (CRITICAL/HIGH/MEDIUM)
   - Load components in bulk
   - Build insights from matches and components

3. **Batch Upsert**
   - Deduplicate insights by `(resource_uid, cve_id, insight_type)`
   - Batch insert with `ON CONFLICT DO UPDATE`
   - Update existing insights if needed

### Insight Fields

- **Resource Context**: `resource_type`, `resource_namespace`, `resource_name`, `resource_uid`
- **Insight Info**: `insight_type`, `severity`, `title`, `description`, `recommendation`
- **CVE Info**: `cve_id`, `affected_component`, `affected_version`, `cvss`
- **Status**: `status` (active/resolved/dismissed), `detected_at`

---

## API Endpoints

### Insights API

**GET** `/api/v1/insights`
- **Query Parameters**:
  - `resource_uid`: Filter by resource UID
  - `resource_type`: Filter by resource type
  - `resource_namespace`: Filter by namespace
  - `resource_name`: Filter by resource name
  - `insight_type`: Filter by insight type
  - `severity`: Filter by severity
  - `status`: Filter by status (default: active)
  - `sbom_id`: Filter by SBOM ID
  - `page`, `pageSize`: Pagination

**Response**:
```json
{
  "insights": [
    {
      "id": 1,
      "resourceType": "Pod",
      "resourceNamespace": "fortuna",
      "resourceName": "test-pod",
      "resourceUid": "...",
      "insightType": "vulnerability",
      "severity": "medium",
      "title": "CVE-2025-26434 in libxml2",
      "description": "...",
      "recommendation": "...",
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

## Event-Driven Architecture

### NATS JetStream Streams

1. **ksam-raw**
   - **Purpose**: Raw events from agents
   - **Retention**: WorkQueuePolicy, 24h, 1M messages, 10GB

2. **ksam-sbom-created**
   - **Purpose**: SBOM creation events
   - **Retention**: WorkQueuePolicy, 24h, 1M messages, 10GB

3. **ksam-events**
   - **Purpose**: Normalized events
   - **Retention**: WorkQueuePolicy, 48h, 1M messages, 10GB

4. **ksam-insights**
   - **Purpose**: Insight generation events
   - **Retention**: WorkQueuePolicy, 48h, 1M messages, 10GB

### Workers

1. **SBOM Worker**
   - **Subject**: `ksam.sbom.created`
   - **Purpose**: Process SBOM creation events

2. **CVE Matcher Worker**
   - **Subject**: `ksam.sbom.created`
   - **Purpose**: Match CVEs and generate insights
   - **Severity Filter**: CRITICAL, HIGH, MEDIUM (configurable)

---

## Performance Optimizations

### Database Optimizations

1. **Bulk Queries**
   - CVE matching: Bulk query by ecosystem
   - Component loading: Bulk load by package names
   - Insight creation: Batch upsert

2. **Indexes**
   - `cve_matches`: `(sbom_id, package_name, cve_id)` unique
   - `insights`: `(resource_uid, cve_id, insight_type)` unique
   - `sbom_components`: `(sbom_id, purl)` unique
   - Performance indexes on frequently queried columns

3. **Connection Pooling**
   - Prometheus metrics for pool utilization
   - Automatic warnings at 80% utilization

### Processing Optimizations

1. **Asynchronous SBOM Processing**
   - Agent uses work queue to prevent blocking
   - Allows continuous pod detection

2. **Batch Processing**
   - CVE matches: Batch insert with deduplication
   - Insights: Batch upsert with deduplication

3. **N+1 Query Elimination**
   - Bulk load components and matches
   - Use in-memory maps for O(1) lookups

---

## Security Considerations

1. **gRPC Communication**
   - mTLS support (configurable)
   - Currently disabled for development

2. **Database Security**
   - Connection pooling limits
   - Prepared statements (via GORM)

3. **API Security**
   - Status filtering (default: active only)
   - Resource-based access control (via UID)

---

## Recent Changes (2025-12-28)

### Version Comparison Fix
- **Issue**: Complex Debian versions not parsed correctly
- **Fix**: Integrated `go-deb-version` library
- **Impact**: Accurate CVE matching for Debian packages

### Insight Worker Fix
- **Issue**: Insights not generated due to re-query timing issues
- **Fix**: Use matches directly, removed re-query
- **Impact**: Reliable insight generation

### Schema Fixes
- **Issue**: SQL queries referenced non-existent columns
- **Fix**: Removed `fixed_version` from insights table operations
- **Impact**: No SQL errors, correct data storage

### Deduplication
- **Issue**: Duplicate insights causing ON CONFLICT errors
- **Fix**: Deduplicate before batch insert
- **Impact**: Reliable batch processing

---

## Deployment Architecture

### Kubernetes Resources

1. **Agent (DaemonSet)**
   - Runs on each node
   - Memory: 4Gi
   - Node selector: `kubernetes.io/hostname: minikube`

2. **Core (Deployment)**
   - Centralized processing
   - Replicas: 1 (configurable)
   - Resources: Configurable

3. **PostgreSQL (StatefulSet)**
   - Persistent storage
   - Configurable resources

4. **NATS (StatefulSet)**
   - Message queue
   - Replicas: 1 (development), 3 (production)

---

## Monitoring and Observability

### Metrics

1. **Database Connection Pool**
   - Active connections
   - Idle connections
   - Wait duration

2. **Worker Performance**
   - Processing time
   - Queue depth
   - Error rates

3. **CVE Matching**
   - Matches per SBOM
   - Processing time
   - Version comparison errors

### Logging

- Structured logging with prefixes
- Error tracking
- Performance metrics

---

## Future Enhancements

1. **Policy Engine Integration**
   - CEL-based evaluation
   - Template-instance pattern
   - Violation sampling

2. **Additional Ecosystems**
   - RPM version comparison library
   - Alpine version comparison library

3. **Real-time Alerts**
   - Webhook notifications
   - Email alerts
   - Slack integration

---

**Document Version**: 2.0  
**Last Updated**: $(date)

