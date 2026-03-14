# FORTUNA SECURITY PRODUCT UX GUIDE

Version: 1.0
Scope: Investigation workflow across Fortuna UI

This guide ensures Fortuna supports real security investigation workflows.

Security users do not browse dashboards.

They **hunt problems**.

The UI must support this workflow.

---

# 1. Security Investigation Model

Typical workflow:

```
Alert
→ Risk discovery
→ Context analysis
→ Root cause
→ Remediation
```

Fortuna must support this sequence.

---

# 2. Entry Points

Users usually enter investigation through:

```
Dashboard
Risk Center
Findings table
```

Example flow:

```
Dashboard
→ High Risk spike
→ Click risk
→ Pod detail
```

This navigation must be fast.

---

# 3. Investigation Layout Pattern

All investigation pages must show:

```
Context
Evidence
Navigation
```

Example:

```
Pod Detail

Metadata
Summary risk
Tabs
```

---

# 4. Context Panel

Every entity page must show context.

Example:

```
Pod name
Namespace
Cluster
Node
```

Displayed at top of page.

Purpose:

User must immediately know **where the problem exists**.

---

# 5. Evidence Panels

Evidence explains why risk exists.

Examples in Fortuna:

```
CVE list
SBOM packages
Runtime data
```

Evidence must be structured.

Example:

```
CVE-2024-XXXXX
Severity: Critical
Package: openssl
Version: 1.1
```

---

# 6. Risk Explanation

Security tools often fail here.

Fortuna must explain:

```
why this risk matters
```

Example:

```
This container contains a critical OpenSSL vulnerability.
It allows remote code execution.
```

Risk explanation must appear inside risk detail.

---

# 7. Navigation Between Evidence

Security users jump between views.

Example:

```
Pod
→ SBOM
→ CVE
→ Risk rule
```

UI must support quick transitions.

Use:

```
tabs
inline links
breadcrumbs
```

---

# 8. Risk Prioritization UX

Users must quickly identify:

```
top risks
```

UI patterns:

```
severity badges
sorting
filters
```

Example:

```
sort by severity
sort by exploitability
```

---

# 9. Remediation Guidance

Every risk must support remediation.

Example:

```
Upgrade package
Patch container image
Apply runtime policy
```

UI pattern:

```
Remediation section
```

---

# 10. Cross-Resource Navigation

Security problems rarely exist in isolation.

Example flow:

```
CVE
→ Pod
→ Image
→ Cluster
```

Fortuna should support:

```
clickable references
```

Example:

```
pod/payment-service
image: registry/app:v1
```

---

# 11. Investigation Speed

Security users are impatient.

UI must minimize clicks.

Rules:

```
3 clicks to root cause
```

Example:

```
Dashboard
→ Risk
→ Pod
```

---

# 12. Timeline View (Future Enhancement)

Runtime security often needs timelines.

Example:

```
container start
policy violation
network activity
```

Future UI may include:

```
event timeline
```

---

# 13. Security UX Anti-Patterns

Avoid:

### Hidden severity

Severity must always be visible.

Bad:

```
CVE list without severity
```

---

### Weak context

Every risk must show:

```
where it exists
```

---

### Investigation dead ends

Users must always have a next step.

Example:

```
View pod
View image
View cluster
```

---

# 14. Expected Result

After applying these UX rules:

* users investigate faster
* platform feels professional
* findings become actionable

Fortuna becomes a **security investigation tool**, not just a scanner.

---

END OF DOCUMENT
