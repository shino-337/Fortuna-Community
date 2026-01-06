# KSAM Platform - Database Schema Documentation (Updated)

**Last Updated**: $(date)  
**Version**: 2.0

---

## Overview

This document describes the complete database schema for the KSAM Platform, including all tables, columns, indexes, and relationships.

---

## Core Tables

### 1. `sboms`

**Purpose**: Store extracted SBOMs from container images

**Columns**:
| Column | Type | Nullable | Description |
|--------|------|----------|-------------|
| `id` | bigint | NO | Primary key |
| `pod_uid` | varchar(255) | YES | Pod UID |
| `pod_name` | varchar(255) | YES | Pod name |
| `namespace` | varchar(255) | YES | Kubernetes namespace |
| `container_name` | varchar(255) | YES | Container name |
| `image_name` | varchar(255) | YES | Image name |
| `image_digest` | varchar(255) | YES | Image digest (SHA256) |
| `sbom_content` | jsonb | YES | Full SBOM JSON |
| `labels` | jsonb | YES | Pod labels |
| `annotations` | jsonb | YES | Pod annotations |
| `created_at` | timestamp | YES | Creation timestamp |
| `updated_at` | timestamp | YES | Update timestamp |
| `deleted_at` | timestamp | YES | Soft delete timestamp |

**Indexes**:
- Primary: `id`
- Unique: `idx_sboms_image_digest` on `(image_digest)` WHERE `deleted_at IS NULL`
- Performance: `pod_uid`, `pod_name`, `namespace`, `image_digest`

**Relationships**:
- One-to-many with `sbom_components`
- One-to-many with `cve_matches`

---

### 2. `sbom_components`

**Purpose**: Store individual packages from SBOMs

**Columns**:
| Column | Type | Nullable | Description |
|--------|------|----------|-------------|
| `id` | bigint | NO | Primary key |
| `sbom_id` | bigint | NO | Foreign key to `sboms` |
| `component_name` | varchar(255) | NO | Package name |
| `component_version` | varchar(100) | YES | Package version |
| `purl` | text | YES | Package URL |
| `ecosystem` | varchar(50) | YES | Ecosystem (derived from PURL) |
| `created_at` | timestamp | YES | Creation timestamp |
| `updated_at` | timestamp | YES | Update timestamp |
| `deleted_at` | timestamp | YES | Soft delete timestamp |

**Indexes**:
- Primary: `id`
- Foreign: `fk_sboms_components` on `(sbom_id)` REFERENCES `sboms(id)`
- Unique: `idx_sbom_components_unique_sbom_purl` on `(sbom_id, purl)` WHERE `deleted_at IS NULL`
- Performance: `sbom_id`, `component_name`, `purl`

**Relationships**:
- Many-to-one with `sboms`

---

### 3. `cve_matches`

**Purpose**: Store matched CVEs for packages

**Columns**:
| Column | Type | Nullable | Description |
|--------|------|----------|-------------|
| `id` | bigint | NO | Primary key |
| `sbom_id` | bigint | NO | Foreign key to `sboms` |
| `pod_uid` | varchar(255) | YES | Pod UID (from Agent) |
| `container_name` | varchar(255) | YES | Container name (from Agent) |
| `cve_id` | varchar(20) | NO | CVE identifier |
| `package_name` | varchar(255) | NO | Package name (direct from Agent) |
| `package_version` | varchar(100) | YES | Installed version |
| `purl` | varchar(500) | YES | Package URL (pkg:type/name@version) |
| `p_url` | varchar(500) | YES | Alternative name for purl (used in some code paths) |
| `severity` | varchar(20) | NO | CRITICAL, HIGH, MEDIUM, LOW |
| `cvss` | decimal(4,1) | YES | CVSS score (standardized) |
| `fixed_version` | varchar(255) | YES | Fixed version |
| `matched_by` | varchar(255) | YES | Matcher identifier (e.g., fortuna-core-cve-matcher) |
| `matched_at` | timestamp | YES | Match timestamp |
| `created_at` | timestamp | YES | Creation timestamp |
| `updated_at` | timestamp | YES | Update timestamp |
| `deleted_at` | timestamp | YES | Soft delete timestamp |

**Indexes**:
- Primary: `id`
- Foreign: `fk_sboms_cve_matches` on `(sbom_id)` REFERENCES `sboms(id)`
- Unique: `idx_cve_matches_unique_sbom_package_cve` on `(sbom_id, package_name, cve_id)` WHERE `deleted_at IS NULL`
- Performance: `sbom_id`, `pod_uid`, `cve_id`, `package_name`, `severity`

**Relationships**:
- Many-to-one with `sboms`
- Many-to-one with `cves` (via `cve_id`)

**Deduplication**:
- Unique constraint prevents duplicate matches
- `ON CONFLICT DO NOTHING` used in batch inserts

---

### 4. `insights`

**Purpose**: Store security insights for resources

**Columns**:
| Column | Type | Nullable | Description |
|--------|------|----------|-------------|
| `id` | bigint | NO | Primary key |
| `resource_type` | varchar(50) | NO | Pod, Node, ServiceAccount, etc. |
| `resource_namespace` | varchar(255) | YES | Kubernetes namespace |
| `resource_name` | varchar(255) | NO | Resource name |
| `resource_uid` | varchar(255) | NO | Unique resource identifier |
| `insight_type` | varchar(50) | NO | vulnerability, misconfiguration, etc. |
| `severity` | varchar(20) | NO | critical, high, medium, low |
| `title` | varchar(500) | NO | Insight title |
| `description` | text | NO | Detailed description |
| `recommendation` | text | YES | Remediation recommendation |
| `cve_id` | varchar(20) | YES | CVE identifier (for vulnerability insights) |
| `affected_component` | varchar(255) | YES | Package name (for vulnerability insights) |
| `affected_version` | varchar(100) | YES | Installed version (for vulnerability insights) |
| `cvss` | real | YES | CVSS score |
| `status` | varchar(20) | YES | active, resolved, dismissed (default: active) |
| `detected_at` | timestamp | NO | Detection timestamp |
| `resolved_at` | timestamp | YES | Resolution timestamp |
| `created_at` | timestamp | YES | Creation timestamp |
| `updated_at` | timestamp | YES | Update timestamp |
| `deleted_at` | timestamp | YES | Soft delete timestamp |

**Indexes**:
- Primary: `id`
- Unique: `idx_insights_unique_resource_cve_type` on `(resource_uid, cve_id, insight_type)` WHERE `deleted_at IS NULL`
- Performance: `resource_uid`, `resource_type`, `resource_namespace`, `resource_name`, `insight_type`, `severity`, `status`, `cve_id`, `affected_component`

**Relationships**:
- Many-to-one with resources (via `resource_uid`)

**Deduplication**:
- Unique constraint prevents duplicate insights
- `ON CONFLICT DO UPDATE` used in batch upserts
- Pre-insert deduplication prevents ON CONFLICT errors

**Note**: `fixed_version` column exists in this table (added for vulnerability insights)

---

### 5. `cves`

**Purpose**: Store CVE metadata

**Columns**:
| Column | Type | Nullable | Description |
|--------|------|----------|-------------|
| `cve_id` | varchar(20) | NO | Primary key |
| `severity` | varchar(20) | NO | CRITICAL, HIGH, MEDIUM, LOW |
| `cvss_score` | real | YES | CVSS score |
| `title` | text | YES | CVE title |
| `description` | text | YES | CVE description |
| `published_at` | timestamp | YES | Publication timestamp |
| `created_at` | timestamp | YES | Creation timestamp |
| `updated_at` | timestamp | YES | Update timestamp |

**Indexes**:
- Primary: `cve_id`
- Performance: `severity`, `cvss_score`

**Relationships**:
- One-to-many with `package_vulnerabilities`
- One-to-many with `cve_matches` (via `cve_id`)

---

### 6. `package_vulnerabilities`

**Purpose**: Store package-to-CVE mappings with version constraints

**Columns**:
| Column | Type | Nullable | Description |
|--------|------|----------|-------------|
| `id` | bigint | NO | Primary key |
| `cve_id` | varchar(20) | NO | Foreign key to `cves` |
| `package_name` | varchar(255) | NO | Package name |
| `ecosystem` | varchar(50) | NO | Ecosystem (debian, alpine, npm, etc.) |
| `constraint` | text | YES | Version constraint (e.g., ">= 1.0, < 2.0") |
| `created_at` | timestamp | YES | Creation timestamp |
| `updated_at` | timestamp | YES | Update timestamp |

**Indexes**:
- Primary: `id`
- Foreign: `fk_cves_package_vulnerabilities` on `(cve_id)` REFERENCES `cves(cve_id)`
- Performance: `cve_id`, `package_name`, `ecosystem`

**Relationships**:
- Many-to-one with `cves`

---

## Schema Evolution

### Migration History

1. **Initial Schema**: Basic tables for pods, SBOMs, components
2. **Migration 030**: Insights schema migration (removed old columns, added new ones)
3. **Migration 032**: CVE matches migration (removed `component_id`, added `package_name`)
4. **Migration 033**: Added unique constraints
5. **Migration 034**: Standardized CVSS types (numeric → real)
6. **Migration 036**: Added missing SBOM columns (`pod_uid`, `pod_name`, etc.)

### Current Schema State

- ✅ All old columns removed
- ✅ All new columns in place
- ✅ Unique constraints active
- ✅ Indexes optimized
- ✅ Foreign keys established

---

## Query Patterns

### Common Queries

#### 1. Get Insights for Pod
```sql
SELECT * FROM insights 
WHERE resource_uid = '0c12ecc6-68f5-4f88-8caf-a4c61648ca09'
  AND status = 'active'
  AND deleted_at IS NULL
ORDER BY severity DESC, cvss DESC;
```

#### 2. Get CVE Matches for SBOM
```sql
SELECT cve_id, package_name, severity, cvss 
FROM cve_matches 
WHERE sbom_id = 85
  AND deleted_at IS NULL
ORDER BY severity DESC, cvss DESC;
```

#### 3. Get Components for SBOM
```sql
SELECT component_name, component_version, purl 
FROM sbom_components 
WHERE sbom_id = 85
  AND deleted_at IS NULL;
```

#### 4. Bulk CVE Query
```sql
SELECT DISTINCT cve_id, package_name, constraint, severity, cvss_score
FROM package_vulnerabilities pv
JOIN cves c ON pv.cve_id = c.cve_id
WHERE pv.ecosystem = 'debian'
  AND pv.package_name IN ('libxml2', 'nginx', ...);
```

---

## Data Integrity

### Constraints

1. **Foreign Keys**:
   - `sbom_components.sbom_id` → `sboms.id`
   - `cve_matches.sbom_id` → `sboms.id`
   - `package_vulnerabilities.cve_id` → `cves.cve_id`

2. **Unique Constraints**:
   - `sboms`: `(image_digest)` WHERE `deleted_at IS NULL`
   - `sbom_components`: `(sbom_id, purl)` WHERE `deleted_at IS NULL`
   - `cve_matches`: `(sbom_id, package_name, cve_id)` WHERE `deleted_at IS NULL`
   - `insights`: `(resource_uid, cve_id, insight_type)` WHERE `deleted_at IS NULL`

3. **Check Constraints**:
   - `severity` in `cve_matches`: CRITICAL, HIGH, MEDIUM, LOW
   - `severity` in `insights`: critical, high, medium, low
   - `status` in `insights`: active, resolved, dismissed

---

## Performance Considerations

### Index Strategy

1. **Unique Indexes**: Prevent duplicates, enable efficient upserts
2. **Partial Indexes**: WHERE `deleted_at IS NULL` for active records
3. **Composite Indexes**: For common query patterns
4. **Covering Indexes**: Include frequently accessed columns

### Query Optimization

1. **Bulk Queries**: Group by ecosystem, use IN clauses
2. **Batch Inserts**: Use batch size 100-500
3. **Connection Pooling**: Monitor and tune pool size
4. **Prepared Statements**: Via GORM (automatic)

---

## Recent Schema Changes (2025-12-28)

### Removed Columns
- `insights.fixed_version` (not in schema)
- `cve_matches.component_id` (replaced by `package_name`)
- `insights.affected_resources` (JSONB, replaced by direct fields)
- `insights.type` (replaced by `insight_type`)

### Added Columns
- `insights.resource_type`, `resource_namespace`, `resource_name`, `resource_uid`
- `insights.affected_component`, `affected_version`
- `cve_matches.package_name`, `package_version`, `purl`
- `sboms.pod_uid`, `pod_name`, `namespace`, `container_name`

### Type Changes
- `cve_matches.cvss`: `numeric` → `real`
- `cves.cvss_score`: `numeric` → `real`
- `insights.cvss`: `numeric` → `real`

---

**Document Version**: 2.0  
**Last Updated**: $(date)

