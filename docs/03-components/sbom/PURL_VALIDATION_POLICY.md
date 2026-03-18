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