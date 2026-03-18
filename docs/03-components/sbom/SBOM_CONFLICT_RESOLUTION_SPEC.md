# SBOM Conflict Resolution Specification

## 1. Problem Statement

Current SBOM ingestion aggregates components from multiple sources:

- gomod (static dependency)
- gobinary (runtime build info)
- OS packages
- heuristic (distroless)

This leads to:

- Duplicate components (same logical package, different versions)
- Conflicting versions across sources
- Double CVE matching → inflated risk score

Goal:
> Produce a **single canonical component set** per SBOM for deterministic and accurate CVE matching.

---

## 2. Design Overview

Introduce a resolution layer:

ComponentResolver

Pipeline:

Raw Components (Agent/Core)
        ↓
Normalization (PURL canonicalization)
        ↓
Grouping (by canonical identity)
        ↓
Conflict Resolution
        ↓
Resolved Components (matchable set)
        ↓
Matcher

---

## 3. Canonical Identity

Group components by:
canonical_key = {
ecosystem,
normalized_name
}


Rules:

- Go:
  use full module path (multi-segment)
- OS:
  include namespace (e.g. debian:openssl)
- generic:
  fallback to name

---

## 4. Source Priority Model

Each component has:


source ∈ {gobinary-main, gobinary, gomod, os, heuristic}
confidence ∈ {low, medium, high}


Priority:

| Source         | Priority |
|----------------|----------|
| gobinary       | 100      |
| gomod          | 80       |
| os package     | 60       |
| heuristic      | 20       |
| gobinary-main  | 0 (non-matchable) |

---

## 5. Resolution Algorithm

For each canonical_key group:

### Step 1: Filter
- Drop components with:
  - invalid PURL
  - missing version

### Step 2: Remove non-matchable
- Exclude:
  - source == gobinary-main

### Step 3: Select winner

Sort by:

(priority DESC, confidence DESC)


Pick top component as:


resolved_component


### Step 4: Shadow others

Mark remaining as:


shadowed = true
shadow_reason = "lower_priority" | "version_conflict"


---

## 6. Output Model

Resolved component:


{
purl,
name,
version,
ecosystem,
source,
confidence,
resolved: true
}


Shadowed component:


{
...,
resolved: false,
shadowed_by: <purl>
}


---

## 7. Edge Cases

### Case 1: Same version, different sources
→ keep highest priority, shadow rest

### Case 2: Version mismatch
→ DO NOT merge
→ pick highest priority

### Case 3: Only heuristic available
→ allow match but flag:

low_confidence = true


---

## 8. Integration Points

- Run in Core BEFORE event publish OR in Worker BEFORE matching
- MUST be deterministic

---

## 9. Acceptance Criteria

- No duplicate CVE matches for same logical package
- gobinary overrides gomod consistently
- heuristic never overrides real sources
- deterministic output across runs