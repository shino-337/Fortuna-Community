# Fortuna Platform Architecture

**Version**: 1.0  
**Last Updated**: 2026-01-05  
**Status**: Production Ready

---

## Overview

Fortuna is a comprehensive security and risk management platform for Kubernetes clusters. It provides real-time vulnerability detection, SBOM (Software Bill of Materials) extraction, CVE matching, security insights generation, Pod Capability Engine (PCE), and attack path analysis.

### Key Capabilities

- **SBOM Extraction**: Automatic extraction of Software Bill of Materials from container images
- **CVE Detection**: Real-time matching of Common Vulnerabilities and Exposures
- **Security Insights**: Automated generation of security insights and recommendations
- **Policy Engine**: Configurable security policies and enforcement
- **Risk Scoring**: Comprehensive risk assessment for workloads
- **Pod Capability Engine (PCE)**: Runtime capability detection and attack step inference
- **Attack Path Analysis**: Graph-based attack path visualization and analysis
- **Runtime Signals**: Real-time security signal detection and correlation

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
  - Pod Capability Engine (PCE)
  - Attack path analysis
  - Runtime signal processing
  - API services
  - Database management

**Key Features**:
- **NATS JetStream**: Event-driven architecture
- **PostgreSQL**: Persistent storage
- **gRPC Server**: Agent communication
- **REST API**: External access (51+ endpoints)
- **Workers**: Asynchronous processing
- **PCE Scheduler**: Periodic capability evaluation
- **Admission Webhook**: Policy enforcement

### 3. Dashboard (Deployment)
- **Purpose**: Web-based user interface
- **Responsibilities**:
  - Security dashboard and visualization
  - Risk center and insights management
  - SBOM analysis and vulnerability browsing
  - Attack path visualization
  - Pod capabilities and runtime signals monitoring
  - Capability metadata browser

**Key Features**:
- **React + TypeScript**: Modern frontend framework
- **Real-time Updates**: API-driven data refresh
- **Multiple Views**: Dashboard, Risks, SBOM, Attack Paths, Capabilities

### 4. Infrastructure
- **PostgreSQL**: Database for SBOMs, CVEs, insights, policies, capabilities, attack steps
- **NATS JetStream**: Message queue for event processing (3-replica cluster)
- **Metrics**: Core service exposes `/metrics` endpoint (Prometheus format)

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

#### `pod_capabilities`
- **Purpose**: Store pod capabilities detected by PCE
- **Key Columns**:
  - `pod_uid`, `namespace`, `capability_id`
  - `severity`, `state` (detected, confirmed, exploited, chained)
  - `confidence`, `evidence` (JSONB), `mitre_techniques`
  - `first_seen_at`, `last_seen_at`

#### `capability_metadata`
- **Purpose**: Semantic metadata for capabilities
- **Key Columns**:
  - `capability_id`, `domain`, `category`, `description`
  - `severity_base`, `confidence_base`
  - `preconditions` (JSONB), `produces_attack_steps` (JSONB)
  - `expires_with_instance`, `supports_runtime_promotion`

#### `pod_attack_steps`
- **Purpose**: Store attack steps inferred from capabilities
- **Key Columns**:
  - `pod_uid`, `step_id`, `category`, `confidence`
  - `description`, `evidence` (JSONB)

#### `runtime_signals`
- **Purpose**: Semantic runtime security signals
- **Key Columns**:
  - `pod_uid`, `signal_type`, `category`, `confidence`
  - `evidence` (JSONB)

#### `promotion_rules`
- **Purpose**: Rules for promoting capability states based on signals
- **Key Columns**:
  - `capability_id`, `signal_type`, `min_occurrences`
  - `required_capabilities` (JSONB), `promote_to`, `confidence_boost`

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

1. **fortuna-raw**
   - **Purpose**: Raw events from agents
   - **Retention**: WorkQueuePolicy, 24h, 100K messages, 1GB
   - **Subjects**: `fortuna.raw.pods`, `fortuna.raw.serviceaccounts`, `fortuna.raw.roles`, `fortuna.raw.rolebindings`

2. **fortuna-events**
   - **Purpose**: Normalized events and SBOM/CVE processing
   - **Retention**: WorkQueuePolicy, 48h, 200K messages, 2GB
   - **Subjects**: `fortuna.events.runtime`, `fortuna.sbom.>`, `fortuna.cve.>`

3. **fortuna-insights**
   - **Purpose**: Insight generation events
   - **Retention**: LimitsPolicy, 48h, 50K messages, 512MB
   - **Subjects**: `fortuna.insights.created`

4. **fortuna-normalized**
   - **Purpose**: Normalized event processing
   - **Retention**: WorkQueuePolicy, 24h, 100K messages, 1GB
   - **Subjects**: `fortuna.normalized.>`

### Workers

1. **SBOM Worker**
   - **Subject**: `fortuna.sbom.created`
   - **Purpose**: Process SBOM creation events

2. **CVE Matcher Worker**
   - **Subject**: `fortuna.sbom.created`
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

## Pod Capability Engine (PCE)

### Overview

The Pod Capability Engine (PCE) is a core component that detects, tracks, and analyzes security capabilities of Kubernetes pods. It provides a foundation for attack path analysis and runtime security assessment.

### Capability Detection

**Static Capabilities** (from PodSpec):
- Privileged containers (`ESC_PRIV_POD`)
- Host namespace access (`ESC_HOSTPID_POD`, `ESC_HOSTIPC_POD`)
- HostPath mounts (`ESC_HOSTPATH_NODE`)
- HostNetwork (`NET_HOSTNETWORK`)
- ServiceAccount token access (`ID_TOKEN_POD`)
- RBAC permissions (`API_RBAC_WRITE_CLUSTER`)
- Control plane namespace (`CTRL_CONTROL_PLANE_POD`)

**Runtime Capabilities** (from runtime signals):
- Container escape attempts (`ESC_RUNTIME_PROC_ROOT`)
- Active escape confirmation (`ESC_RUNTIME_ACTIVE`)
- Runtime probe detection (`ESC_RUNTIME_PROBE`)

### Capability States

1. **detected**: Initial state from static PodSpec analysis
2. **confirmed**: Promoted when runtime signals match promotion rules
3. **exploited**: Active exploitation detected via runtime signals
4. **chained**: Multiple capabilities combined for attack path

### State Promotion

Promotion rules define how capabilities move between states:
- **Signal-based**: Runtime signals trigger state promotion
- **Occurrence-based**: Minimum signal occurrences required
- **Prerequisite-based**: Required capabilities must exist
- **Confidence boost**: Confidence increases with promotion

### Attack Step Inference

When capabilities reach `exploited` state, attack steps are automatically inferred:
- Query `capability_metadata.produces_attack_steps`
- Create `PodAttackStep` records with evidence
- Link steps to form attack paths

### Runtime Signals

Runtime signals provide semantic layer for runtime events:
- **Signal Types**: `PROC_ROOT_PIVOT`, `NETWORK_SNIFFING`, `RBAC_ABUSE`, etc.
- **Categories**: Container Escape, Network, Credential Access, etc.
- **Confidence**: Signal confidence score (0.0-1.0)
- **Evidence**: JSONB evidence payload

### PCE Scheduler

Periodic evaluation (default: 6h) to:
- Re-evaluate pod capabilities
- Update capability states
- Generate attack steps
- Clean up terminated pod instances

---

## Deployment Architecture

### Kubernetes Resources

1. **Agent (DaemonSet)**
   - Runs on all nodes (including control-plane)
   - Resources: 256Mi-1Gi memory, 100m-500m CPU
   - Containerd socket access
   - mTLS client certificates

2. **Core (Deployment)**
   - Runs on control-plane nodes
   - Replicas: 1 (configurable)
   - Resources: 256Mi-1Gi memory, 100m-1000m CPU
   - mTLS server certificates
   - Webhook TLS certificates
   - Containerd socket access

3. **Dashboard (Deployment)**
   - Runs on control-plane nodes
   - Resources: 64Mi-128Mi memory, 50m-100m CPU
   - Nginx-based static serving

4. **PostgreSQL (StatefulSet)**
   - Persistent storage
   - Configurable resources
   - Automatic migrations

5. **NATS (StatefulSet)**
   - Message queue (JetStream)
   - Replicas: 3 (production)
   - High availability cluster

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

## API Endpoints Summary

### Core APIs

**SBOM & CVE**:
- `GET /api/v1/sbom` - List SBOMs
- `GET /api/v1/sbom/:podId` - Get SBOM detail
- `GET /api/v1/risks` - List security risks/insights

**Pod Capabilities (PCE)**:
- `GET /api/v1/pod-capabilities` - List pod capabilities
- `GET /api/v1/pods/:podUid/capabilities` - Get pod capabilities
- `GET /api/v1/pod-capabilities/summary/*` - Summary endpoints (cluster, capability, namespace, severity)
- `GET /api/v1/pod-capabilities/trends` - Capability trends

**Capability Metadata**:
- `GET /api/v1/capability-metadata` - List capability metadata
- `GET /api/v1/capability-metadata/:capabilityId` - Get metadata

**Attack Steps**:
- `GET /api/v1/attack-steps/pods/:podUid` - Get pod attack steps
- `GET /api/v1/attack-steps/summary` - Attack step summary

**Runtime Signals**:
- `GET /api/v1/runtime-signals` - List runtime signals
- `GET /api/v1/runtime-signals/pods/:podUid` - Get pod signals

**Promotion Rules**:
- `GET /api/v1/promotion-rules` - List promotion rules
- `GET /api/v1/promotion-rules/capability/:capabilityId` - Rules by capability
- `GET /api/v1/promotion-rules/signal/:signalType` - Rules by signal type

**Attack Paths**:
- `GET /api/v1/attack-paths/graph` - Attack path graph

**Dashboard**:
- `GET /api/v1/dashboard/stats` - Dashboard statistics
- `GET /api/v1/dashboard/metrics/threat-velocity` - Threat velocity metrics

See [API Reference](API_REFERENCE.md) for complete documentation.

---

**Document Version**: 2.1  
**Last Updated**: 2026-01-29

