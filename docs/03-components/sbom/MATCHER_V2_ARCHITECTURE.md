# Matcher V2 Architecture

## 1. Problem Statement

Current matcher:

- Works correctly
- But still:
  - Sensitive to noisy SBOM
  - Dependent on runtime NVD calls
  - Limited context awareness

Goal:
> Build a **deterministic, scalable, low-noise matcher pipeline**

---

## 2. High-Level Architecture
Event (SBOM Created)
↓
Load Snapshot
↓
Component Resolver
↓
PURL Validation Filter
↓
Matching Engine
↓
CVE Matches
↓
Risk Engine


---

## 3. Matching Engine Design

### Input


resolved_components[]


### Processing

#### Step 1: Parse PURL

Extract:


ecosystem
name
version


---

#### Step 2: Alias Resolution (Go)

- lookup:
  go_module_alias

- generate candidates:
  - exact match
  - prefix match

---

#### Step 3: Bulk Query


SELECT * FROM osv_packages
JOIN osv_vulnerabilities
WHERE package IN (...)


---

#### Step 4: Version Constraint Matching

Evaluate:


affected ranges


→ match version

---

## 4. NVD Strategy (V2)

### Current
- runtime API calls

### Target

- Local mirror:
  nvd_vulnerabilities
  nvd_configurations

- Precomputed constraints

---

## 5. Caching Strategy

Cache key:


hash(purl, mirror_version)


TTL:
- 30 min (default)

Invalidate:
- mirror_state.version change

---

## 6. Idempotency Model

Key:


(sbom_id, version, mirror_version, event_id)


---

## 7. Match Output Model


{
component_purl,
cve_id,
severity,
source (OSV|NVD),
matched_version,
confidence
}


---

## 8. Noise Reduction Rules

Skip:

- gobinary-main
- low-trust PURL
- shadowed components

---

## 9. Observability

Metrics:

- match_rate
- skipped_components
- cache_hit_ratio

Logs:


[MatcherV2]
sbom_id=
components=
matched=
skipped=
duration=


---

## 10. Future Extension

### Reachability-aware matching

Combine:

- SBOM
- runtime trace
- call graph

---

## 11. Acceptance Criteria

- Deterministic matching (same input → same output)
- No duplicate matches
- Reduced false positives vs V1
- No runtime dependency on external APIs