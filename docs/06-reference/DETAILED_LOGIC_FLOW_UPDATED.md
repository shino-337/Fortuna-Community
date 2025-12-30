# KSAM Platform - Detailed Logic Flow (Updated)

**Last Updated**: $(date)  
**Version**: 2.0

---

## Complete End-to-End Flow

### Phase 1: Pod Detection and SBOM Extraction

```
┌─────────────────────────────────────────────────────────────┐
│ 1. Pod Created in Kubernetes                                │
│    - Pod UID: 0c12ecc6-68f5-4f88-8caf-a4c61648ca09          │
│    - Namespace: fortuna                                      │
│    - Name: test-pod-final-1766923148                        │
│    - Image: nginx:latest                                     │
└─────────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────────┐
│ 2. Agent Local Pod Watcher Detects Pod                      │
│    - File: agent/internal/watcher/pod_watcher_local.go     │
│    - Event: AddFunc/UpdateFunc triggered                    │
│    - Action: Enqueue pod to SBOM work queue                 │
└─────────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────────┐
│ 3. SBOM Work Queue (Asynchronous)                           │
│    - File: agent/internal/sbom/queue.go                    │
│    - Worker processes pod from queue                        │
│    - Non-blocking: Allows continuous pod detection          │
└─────────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────────┐
│ 4. SBOM Extraction                                           │
│    - File: agent/pkg/sbom/extractor/extractor.go           │
│    - Detect OS from /etc/os-release                         │
│    - Select parsers based on OS:                            │
│      * Debian: dpkg, npm, pip, gomod                       │
│      * Alpine: apk, npm, pip, gomod                         │
│    - Extract packages and generate PURLs                   │
│    - Time: 1-3 minutes                                      │
└─────────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────────┐
│ 5. Agent Sends SBOM to Core via gRPC                        │
│    - File: agent/internal/client/grpc_client_mtls.go        │
│    - Method: SendSBOMFinding                                │
│    - Payload: SBOM data with components                     │
└─────────────────────────────────────────────────────────────┘
```

### Phase 2: Core SBOM Processing

```
┌─────────────────────────────────────────────────────────────┐
│ 6. Core Receives SBOM via gRPC                             │
│    - File: core/internal/grpc/handler_sbom.go              │
│    - Handler: SendSBOMFinding                               │
│    - Action: Store SBOM in database                         │
└─────────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────────┐
│ 7. SBOM Storage                                              │
│    - Check for existing SBOM by image_digest                │
│    - If exists: Reuse (UPDATE)                              │
│    - If new: Create (INSERT)                                │
│    - Store components in sbom_components table              │
│    - Deduplication: Unique constraint on (sbom_id, purl)   │
└─────────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────────┐
│ 8. Core Publishes SBOM_CREATED Event                        │
│    - File: core/internal/grpc/handler_sbom.go              │
│    - NATS Subject: ksam.sbom.created                        │
│    - Payload:                                                │
│      {                                                       │
│        "sbom_id": 85,                                       │
│        "pod_uid": "0c12ecc6-...",                           │
│        "pod_namespace": "fortuna",                          │
│        "pod_name": "test-pod-final-...",                    │
│        "container_name": "test-container",                   │
│        "container_image": "nginx:latest",                   │
│        "image_digest": "sha256:..."                         │
│      }                                                       │
└─────────────────────────────────────────────────────────────┘
```

### Phase 3: CVE Matching

```
┌─────────────────────────────────────────────────────────────┐
│ 9. CVE Matcher Worker Receives Event                       │
│    - File: core/pkg/worker/cve_matcher_worker.go           │
│    - Subject: ksam.sbom.created                             │
│    - Method: ProcessEvent                                   │
└─────────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────────┐
│ 10. Load SBOM and Components                                │
│     - Load SBOM from database by sbom_id                    │
│     - Load all components from sbom_components             │
│     - Extract PURLs from components                         │
└─────────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────────┐
│ 11. PURL Parsing and Ecosystem Normalization               │
│     - File: core/pkg/cve/matcher/purl_parser.go           │
│     - Parse PURL: pkg:PACKAGE_TYPE_DEB/libxml2@2.12.7...   │
│     - Extract:                                              │
│       * Package Name: libxml2                               │
│       * Version: 2.12.7+dfsg+really2.9.14-2.1+deb13u2      │
│       * Ecosystem: PACKAGE_TYPE_DEB                        │
│     - Normalize: PACKAGE_TYPE_DEB → debian                  │
│     - File: core/pkg/cve/matcher/matcher.go                │
│     - Function: normalizeQueryEcosystem                     │
└─────────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────────┐
│ 12. Bulk CVE Query                                          │
│     - File: core/pkg/cve/database/manager.go               │
│     - Group packages by ecosystem                           │
│     - Query package_vulnerabilities table in bulk           │
│     - Returns: Map[package_name][]CVE                       │
│     - Example:                                              │
│       libxml2 → [CVE-2025-26434]                            │
└─────────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────────┐
│ 13. Version Comparison                                      │
│     - File: core/pkg/cve/matcher/version_comparator.go    │
│     - For Debian:                                           │
│       * Library: github.com/knqyf263/go-deb-version        │
│       * Parse: debversion.NewVersion(installed)            │
│       * Compare: v1.Compare(v2)                            │
│     - For RPM/Alpine/npm/pypi/go:                           │
│       * Library: hashicorp/go-version                       │
│       * Compare: version.Compare(installed, constraint)    │
│     - For Unsupported:                                      │
│       * Return explicit error (no fallback)                 │
│     - Result: true if vulnerable, false if not              │
└─────────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────────┐
│ 14. Create CVE Matches                                      │
│     - File: core/pkg/worker/cve_matcher_worker.go         │
│     - For each vulnerable package:                          │
│       * Create CVEMatch object                              │
│       * Set: sbom_id, pod_uid, cve_id, package_name, etc.  │
│     - Batch insert with ON CONFLICT DO NOTHING              │
│     - Deduplication: Unique constraint                      │
│     - Store in cve_matches table                            │
└─────────────────────────────────────────────────────────────┘
```

### Phase 4: Insight Generation

```
┌─────────────────────────────────────────────────────────────┐
│ 15. Filter Matches by Severity                              │
│     - File: core/pkg/worker/cve_matcher_worker.go         │
│     - Filter: CRITICAL, HIGH, MEDIUM (configurable)         │
│     - Skip: LOW severity                                   │
└─────────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────────┐
│ 16. Build Insights                                          │
│     - File: core/pkg/worker/cve_matcher_worker.go         │
│     - Use matches directly (no re-query)                    │
│     - Load components in bulk                               │
│     - Build insight objects:                                │
│       * Resource context (from event)                       │
│       * CVE information (from match)                       │
│       * Component information (from component)              │
│     - Function: buildVulnInsightFromEvent                   │
└─────────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────────┐
│ 17. Batch Upsert Insights                                   │
│     - File: core/pkg/riskengine/insight_manager.go        │
│     - Deduplicate by (resource_uid, cve_id, insight_type)    │
│     - Batch insert with ON CONFLICT DO UPDATE              │
│     - Update existing insights if needed                    │
│     - Store in insights table                               │
└─────────────────────────────────────────────────────────────┘
```

### Phase 5: API Access

```
┌─────────────────────────────────────────────────────────────┐
│ 18. Query Insights via API                                  │
│     - Endpoint: GET /api/v1/insights                        │
│     - File: core/internal/api/insights_handlers.go         │
│     - Filter by resource_uid, severity, status, etc.        │
│     - Return JSON response with insights                    │
└─────────────────────────────────────────────────────────────┘
```

---

## Detailed Component Logic

### PURL Parsing Logic

**Input**: `pkg:PACKAGE_TYPE_DEB/libxml2@2.12.7+dfsg+really2.9.14-2.1+deb13u2`

**Process**:
1. Parse PURL format
2. Extract path components
3. Handle `PACKAGE_TYPE_*` prefixes
4. Extract package name and version

**Output**:
```go
PURL{
    Type: "pkg",
    Ecosystem: "package_type_deb",  // Will be normalized
    Name: "libxml2",
    Version: "2.12.7+dfsg+really2.9.14-2.1+deb13u2"
}
```

### Ecosystem Normalization Logic

**Input**: `package_type_deb` (from PURL)

**Process**:
1. Check for `PACKAGE_TYPE_*` prefix
2. Remove prefix: `package_type_deb` → `deb`
3. Map to database ecosystem:
   - `deb` → `debian`
   - `apk` → `alpine`
   - `rpm` → `linux`

**Output**: `debian` (for database query)

### Version Comparison Logic (Debian)

**Input**:
- Installed: `2.12.7+dfsg+really2.9.14-2.1+deb13u2`
- Constraint: `>= 1.0, < 2.14.5`

**Process**:
1. Parse installed version:
   ```go
   v1, err := debversion.NewVersion("2.12.7+dfsg+really2.9.14-2.1+deb13u2")
   ```
2. Parse constraint parts:
   ```go
   parts := strings.Split(">= 1.0, < 2.14.5", ",")
   for each part:
       op, targetVersion := parseConstraint(part)
       v2, err := debversion.NewVersion(targetVersion)
       cmp := v1.Compare(v2)
       // Apply operator
   ```
3. Compare: `v1.Compare(v2)` returns -1, 0, or 1
4. Apply operator: `<`, `<=`, `>`, `>=`, `==`

**Output**: `true` if vulnerable, `false` if not

### Insight Building Logic

**Input**:
- Event: `SBOMCreatedEvent` (pod context)
- Component: `SBOMComponent` (package info)
- Match: `CVEMatch` (CVE info)

**Process**:
1. Build description:
   ```
   "Vulnerability {cve_id} ({severity}) detected in package {package}@{version} 
    for pod {namespace}/{name} (container={container}, image={image}). 
    Fixed version: {fixed_version}"
   ```
2. Build recommendation:
   ```
   "Update image/package to a fixed version (package {package} -> {fixed_version}, 
    or update image {image})."
   ```
3. Create insight object:
   ```go
   Insight{
       ResourceType: "Pod",
       ResourceNamespace: ev.PodNamespace,
       ResourceName: ev.PodName,
       ResourceUID: ev.PodUID,
       InsightType: "vulnerability",
       Severity: "medium",  // Lowercase
       Title: fmt.Sprintf("%s in %s", match.CVEID, component.ComponentName),
       Description: description,
       Recommendation: recommendation,
       CVEID: match.CVEID,
       CVSS: match.CVSS,
       AffectedComponent: component.ComponentName,
       AffectedVersion: component.ComponentVersion,
       Status: "active",
       DetectedAt: time.Now(),
   }
   ```

**Output**: `Insight` object ready for database storage

### Batch Upsert Logic

**Input**: Array of `Insight` objects

**Process**:
1. Deduplicate by `(resource_uid, cve_id, insight_type)`
2. Build SQL INSERT with VALUES
3. Use `ON CONFLICT DO UPDATE`:
   ```sql
   INSERT INTO insights (...) VALUES (...)
   ON CONFLICT (resource_uid, cve_id, insight_type)
   WHERE deleted_at IS NULL
   DO UPDATE SET
       description = EXCLUDED.description,
       recommendation = EXCLUDED.recommendation,
       cvss = EXCLUDED.cvss,
       severity = EXCLUDED.severity,
       affected_version = EXCLUDED.affected_version,
       status = CASE
           WHEN insights.status IN ('resolved', 'dismissed') THEN 'active'
           ELSE insights.status
       END,
       updated_at = EXCLUDED.updated_at
   ```

**Output**: Insights stored/updated in database

---

## Database Schema Details

### Insights Table

**Columns**:
- `id`: Primary key
- `resource_type`: "Pod", "Node", etc.
- `resource_namespace`: Kubernetes namespace
- `resource_name`: Resource name
- `resource_uid`: Unique resource identifier
- `insight_type`: "vulnerability", "misconfiguration", etc.
- `severity`: "critical", "high", "medium", "low"
- `title`: Insight title
- `description`: Detailed description
- `recommendation`: Remediation recommendation
- `cve_id`: CVE identifier (nullable)
- `affected_component`: Package name (nullable)
- `affected_version`: Installed version (nullable)
- `cvss`: CVSS score (real, nullable)
- `status`: "active", "resolved", "dismissed"
- `detected_at`: Detection timestamp
- `created_at`, `updated_at`, `deleted_at`: Timestamps

**Indexes**:
- Primary: `id`
- Unique: `(resource_uid, cve_id, insight_type)` WHERE `deleted_at IS NULL`
- Performance: `resource_uid`, `severity`, `status`, `cve_id`, etc.

### CVE Matches Table

**Columns**:
- `id`: Primary key
- `sbom_id`: Foreign key to `sboms`
- `pod_uid`: Pod UID
- `container_name`: Container name
- `cve_id`: CVE identifier
- `package_name`: Package name
- `package_version`: Installed version
- `purl`: Package URL
- `severity`: "CRITICAL", "HIGH", "MEDIUM", "LOW"
- `cvss`: CVSS score (real)
- `fixed_version`: Fixed version (nullable)
- `matched_by`: Matcher identifier
- `matched_at`: Match timestamp
- `created_at`, `updated_at`, `deleted_at`: Timestamps

**Indexes**:
- Primary: `id`
- Unique: `(sbom_id, package_name, cve_id)` WHERE `deleted_at IS NULL`
- Performance: `sbom_id`, `pod_uid`, `severity`, `cve_id`, etc.

---

## Error Handling and Recovery

### SBOM Extraction Errors
- **Parser Not Found**: Logged, skipped (expected for non-applicable parsers)
- **Extraction Failure**: Retried via NATS redelivery
- **gRPC Send Failure**: Retried with exponential backoff

### CVE Matching Errors
- **Version Parsing Error**: Logged, match skipped
- **Unsupported Ecosystem**: Logged, explicit error returned
- **Database Query Error**: Retried via NATS redelivery

### Insight Generation Errors
- **Component Not Found**: Logged, insight skipped
- **Batch Upsert Error**: Retried via NATS redelivery
- **Deduplication Error**: Handled by pre-insert deduplication

---

## Performance Metrics

### Typical Processing Times

- **SBOM Extraction**: 1-3 minutes per pod
- **CVE Matching**: 2-3 seconds for 150 packages
- **Insight Generation**: <500ms for 100 insights
- **Total End-to-End**: ~3-4 minutes per pod

### Database Query Counts

- **SBOM Storage**: 1 INSERT + N component INSERTs
- **CVE Matching**: 1-3 bulk queries (by ecosystem)
- **Insight Generation**: 2 bulk queries (components + matches)
- **Total**: ~5-10 queries per pod (optimized from 400+)

---

**Document Version**: 2.0  
**Last Updated**: $(date)

