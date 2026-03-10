# Fortuna OSV Loader Enhancement

## Safe Refactor Specification (Architecture Preserving)

**Project:** FortunaHub
**Component:** CVE Loader – OSV Parser
**Primary Source:** Open Source Vulnerabilities Database (OSV)
**Goal:** Improve OSV ingestion coverage while **strictly preserving the existing system architecture**

---

# 1. Purpose

This specification defines a **controlled improvement of the OSV vulnerability loader** to reduce the number of records marked as:

```
Skipped (no packages)
```

while maintaining full compatibility with the existing Fortuna architecture.

The changes must:

* preserve the current ingestion pipeline
* avoid breaking the vulnerability matching engine
* prevent data loss from OSV ingestion
* ensure database integrity
* remain compatible with current SBOM scanning

The system must **retain as much vulnerability intelligence as possible** while respecting the original design.

---

# 2. Architecture Constraints (MANDATORY)

The following architectural principles **must not be violated**.

### 2.1 Existing Data Flow

The pipeline must remain:

```
OSV JSON
   ↓
OSV Loader
   ↓
ConvertToPackageVulnerabilities()
   ↓
Database
   ↓
Vulnerability Matching
   ↓
SBOM Scan
```

The agent **must not introduce new processing layers** that alter this flow.

---

### 2.2 Database Ownership

Tables remain unchanged in purpose:

| Table                     | Purpose                                           |
| ------------------------- | ------------------------------------------------- |
| `cves`                    | Stores vulnerability metadata                     |
| `package_vulnerabilities` | Stores package-specific vulnerability information |

The loader **must not change the meaning of these tables**.

---

### 2.3 Separation of Responsibilities

| Component | Responsibility                  |
| --------- | ------------------------------- |
| Loader    | Normalize vulnerability data    |
| Database  | Persist vulnerability records   |
| Matcher   | Evaluate package version impact |
| Scanner   | Analyze SBOM packages           |

The loader **must not move matching logic into the parser**.

---

# 3. Current Behavior

Current loader location:

```
core/pkg/cve/loader/osv_parser.go
```

Primary function:

```
ConvertToPackageVulnerabilities()
```

Observed behavior:

```
CVE stored in table: cves
Package vulnerability stored in: package_vulnerabilities
```

However many records produce:

```
Skipped (no packages)
```

Meaning:

```
0 package_vulnerabilities created
```

---

# 4. Known Skip Conditions

The agent must verify these behaviors directly in the codebase.

### 4.1 Missing affected field

Example:

```
affected = null
```

Current result:

```
skip record
```

---

### 4.2 Missing package metadata

If:

```
package.name == ""
OR
package.ecosystem == ""
```

Current result:

```
skip
```

---

### 4.3 Git-only ranges

Example:

```
ranges:
  type: GIT
```

Current behavior:

```
ignored
```

Result:

```
0 package records
```

---

### 4.4 Introduced-only ranges

Example:

```
events:
  introduced: 1.2.0
```

Current logic requires:

```
fixed
OR
last_affected
```

Otherwise skipped.

---

### 4.5 No ranges and no versions

Example:

```
ranges: []
versions: []
```

Result:

```
skip
```

---

# 5. Root Problem

OSV intentionally allows **partial vulnerability information**.

However the current loader only stores records when it can produce:

```
package + ecosystem + usable version range
```

This causes **large numbers of vulnerabilities to be discarded**.

The loader currently enforces **data completeness** rather than **data preservation**.

---

# 6. Refactor Goals

The loader must be improved to:

1. Reduce unnecessary skipped records
2. Preserve vulnerability intelligence
3. Maintain compatibility with existing matching logic
4. Avoid introducing false positives
5. Avoid duplicate vulnerability entries

---

# 7. Required Codebase Investigation

Before implementing changes, the agent must audit the following modules.

---

## 7.1 OSV Loader

Inspect:

```
core/pkg/cve/loader/
```

Tasks:

* locate all skip conditions
* identify version parsing logic
* analyze range conversion logic

Deliverable:

```
documented skip logic
```

---

## 7.2 Database Schema

Inspect tables:

```
cves
package_vulnerabilities
```

Verify:

* unique constraints
* nullable fields
* version range storage format
* whether open ranges are allowed

Agent must confirm **schema compatibility** before implementing changes.

---

## 7.3 Vulnerability Matching Engine

Inspect modules:

```
core/pkg/vuln
core/pkg/scanner
core/pkg/sca
```

Determine:

* semver comparison logic
* explicit version handling
* behavior when range is incomplete

The agent must ensure new records **do not break matcher logic**.

---

## 7.4 API Layer

Inspect:

```
api/
pkg/api/
server/
```

Verify APIs can safely represent:

```
unknown ranges
open ranges
git ranges
```

---

# 8. Improved Parsing Strategy

The loader should preserve more vulnerability information while remaining compatible with current matching behavior.

---

## 8.1 Introduced-only ranges

Current behavior:

```
skip
```

New behavior:

Create open range:

```
>= introduced
```

Example:

```
introduced: 1.2.0
```

Stored range:

```
>=1.2.0
```

This record must be clearly marked.

---

## 8.2 Explicit versions fallback

If:

```
ranges empty
versions not empty
```

Create package vulnerability entries for each version.

---

## 8.3 Git ranges preservation

If:

```
range.type = GIT
```

Do not discard the vulnerability.

Store metadata fields:

```
introduced_commit
fixed_commit
repository
```

Even if the matcher does not use them yet.

---

## 8.4 Package-only vulnerabilities

If package metadata exists but no version information is present.

Store minimal record:

```
package
ecosystem
cve_id
```

This preserves intelligence without enforcing version logic.

---

# 9. Data Quality Classification

Each vulnerability record must include a classification.

| Level   | Description                    |
| ------- | ------------------------------ |
| FULL    | Complete version range         |
| PARTIAL | Open range or incomplete range |
| UNKNOWN | Version information missing    |

This classification helps downstream systems decide how to interpret the vulnerability.

---

# 10. Loader Metrics

Enhance ingestion logs.

Example summary:

```
OSV Loader Summary

total_records: 180000
range_records: 110000
explicit_versions: 20000
open_ranges: 30000
git_ranges: 10000
package_only: 5000
skipped: 5000
```

Metrics help detect ingestion anomalies.

---

# 11. Safety Requirements

The implementation must guarantee:

* no duplicate vulnerabilities
* no modification of existing CVE identifiers
* no corruption of existing vulnerability records
* compatibility with existing scans

If the agent detects potential matcher conflicts, the change must be flagged rather than applied.

---

# 12. Required Tests

The agent must add unit tests covering the following cases.

---

### Test 1 – Standard Range

Input:

```
introduced + fixed
```

Expected:

```
range vulnerability created
```

---

### Test 2 – Introduced Only

Input:

```
introduced only
```

Expected:

```
open range vulnerability created
```

---

### Test 3 – Explicit Versions

Input:

```
versions present
ranges empty
```

Expected:

```
explicit vulnerability entries created
```

---

### Test 4 – Git Range

Input:

```
range.type = GIT
```

Expected:

```
git metadata stored
```

---

### Test 5 – Package Only

Input:

```
package exists
no version information
```

Expected:

```
minimal vulnerability record created
```

---

# 13. Deliverables

The agent must produce:

### 1. Codebase Analysis Report

Including:

* parser behavior
* skip logic
* affected modules

---

### 2. Implementation

Including:

* updated loader logic
* any required schema adjustments
* safe parsing strategy

---

### 3. Unit Tests

Tests covering all supported parsing cases.

---

### 4. Loader Metrics

Enhanced ingestion statistics.

---

### 5. Documentation

Update developer documentation explaining:

* parsing rules
* vulnerability data classification
* ingestion metrics

---

# 14. Expected Outcome

After implementation:

* significantly fewer `Skipped (no packages)` records
* higher OSV vulnerability coverage
* preserved compatibility with Fortuna architecture
* safer ingestion pipeline

The loader will **prioritize preserving vulnerability intelligence while remaining fully compatible with the current system design**.

---
