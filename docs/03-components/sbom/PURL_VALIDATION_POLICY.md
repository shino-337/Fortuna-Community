# PURL Validation Policy

## 1. Problem Statement

PURL is currently treated as:

> "parseable = valid"

This is insufficient and introduces:

- Ecosystem mismatch
- Malformed versions
- Injection via crafted PURLs

Goal:
> Enforce **semantic correctness** of PURL before persistence and matching.

---

## 2. Validation Layers

### Layer 1: Syntax Validation

Use standard parser:

- must conform to pkg:<type>/<name>@<version>

If fail:
→ regenerate PURL from (type, name, version)

---

### Layer 2: Ecosystem Consistency

Validate:

| Type     | Expected ecosystem |
|----------|-------------------|
| go       | go                |
| npm      | npm               |
| deb      | debian/ubuntu     |
| apk      | alpine            |

If mismatch:
→ log warning
→ normalize ecosystem

---

### Layer 3: Name Validation

#### Go
- must be full module path
- must contain domain (github.com/...)

#### OS
- must match package manager naming rules

If invalid:
→ mark component:
invalid_name = true


---

### Layer 4: Version Validation

#### Go:
- semver OR pseudo-version:
  vX.Y.Z
  vX.Y.Z-yyyymmdd-hash

#### OS:
- allow distro-specific formats

If invalid:
→ downgrade confidence

---

## 3. Normalization Rules

### Go

Normalize:


pkg:golang/... → pkg:go/...


Trim:
- leading "v" inconsistencies
- redundant prefixes

---

## 4. Trust Levels

Assign:


trust_level:
HIGH (agent-generated valid)
MEDIUM (normalized)
LOW (fallback/generated)


---

## 5. Enforcement Points

### Core Ingest

- validate before DB write
- store:

purl_validated = true|false
trust_level


### Matcher

- skip:
  - invalid + low trust

---

## 6. Logging Policy

- rate-limit warnings per SBOM
- include:
  - original PURL
  - normalized PURL
  - reason

---

## 7. Security Considerations

Prevent:

- PURL spoofing
- version poisoning
- ecosystem confusion

---

## 8. Acceptance Criteria

- No invalid PURL reaches matcher
- All Go modules normalized to pkg:go
- Invalid inputs do not break pipeline
- Logs are actionable, not noisy