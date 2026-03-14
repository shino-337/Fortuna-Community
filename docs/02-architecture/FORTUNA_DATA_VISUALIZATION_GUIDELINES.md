# FORTUNA DATA VISUALIZATION GUIDELINES

Version: 1.0
Scope: Dashboard, Risk Center, Resource Detail, Findings Tables

This document standardizes how data visualizations are used in Fortuna.

The goal is to ensure that:

* charts are consistent
* risk information is visually obvious
* users can quickly interpret security posture

All visualizations must support **security investigation workflows**.

---

# 1. Design Principles

Fortuna visualizations must follow these principles:

### 1.1 Scan First

Users must understand system state within **3 seconds**.

Important signals must be visually dominant.

Example:

```
Risk Trend Chart
Large
Centered
```

---

### 1.2 Security First

Color must communicate **risk severity**, not decoration.

Avoid neutral charts when security state exists.

Bad example:

```
Blue histogram
```

Correct example:

```
Histogram with severity color bands
```

---

### 1.3 Investigation Oriented

Charts should lead users to investigation.

Each visualization must answer:

```
What is wrong?
Where should I click next?
```

---

# 2. Standard Chart Types Used in Fortuna

Fortuna currently uses 4 main visualization types.

These must be standardized.

| Chart Type            | Usage                   |
| --------------------- | ----------------------- |
| Trend Chart           | Risk evolution          |
| Histogram             | Risk score distribution |
| Severity Distribution | CVE severity            |
| KPI Panels            | System overview         |

---

# 3. Trend Chart

Used in:

```
Dashboard
Risk Center
```

Purpose:

Show how system risk evolves.

Example data:

```
risk_score
high_risks
critical_risks
```

Layout rules:

```
width: full container
height: 260px
```

Design rules:

```
line thickness: 2px
smooth curve
grid lines minimal
```

Tooltip must show:

```
timestamp
risk score
critical count
high count
```

Color rules:

```
risk_score = purple
critical = red
high = orange
```

Trend charts must always show **time context**.

Allowed ranges:

```
24h
7d
30d
```

---

# 4. Histogram

Used in:

```
Risk Score Distribution
```

Example:

```
0–20
20–40
40–60
60–80
80–100
```

Design rules:

```
bar width: fixed
gap: small
```

Color mapping:

| Range  | Color    |
| ------ | -------- |
| 0–20   | green    |
| 20–40  | yellow   |
| 40–60  | orange   |
| 60–80  | red      |
| 80–100 | dark red |

Purpose:

Identify clusters of risky resources.

User interaction:

Click bar → filter table below.

---

# 5. Severity Distribution

Used in:

```
CVE views
SBOM findings
```

Chart options:

```
stacked bar
or donut chart
```

Standard severity colors:

| Severity | Color   |
| -------- | ------- |
| Critical | #dc2626 |
| High     | #f97316 |
| Medium   | #facc15 |
| Low      | #22c55e |

Rules:

Critical must visually dominate.

Never sort severities alphabetically.

Correct order:

```
Critical
High
Medium
Low
```

---

# 6. KPI Panels

Used in:

```
Dashboard
Cluster overview
Pod detail summary
```

Example KPIs:

```
Clusters
Pods
Agents
Active Risks
```

Structure:

```
title
value
delta
trend indicator
```

Example:

```
Pods
1,342
+12%
```

Design rules:

```
font size: large
value prominence
icon optional
```

Delta color:

```
positive improvement → green
negative → red
```

---

# 7. Table Visualization Rules

Tables are the **primary data visualization** in security tools.

Used for:

```
Risks
CVE
Pods
Agents
```

Rules:

Row height:

```
36px
```

Columns must include:

```
severity badge
name
context
timestamp
```

Example row:

```
CRITICAL
CVE-2024-XXXX
openssl
pod/payment-service
```

Severity must be the **first column**.

---

# 8. Color Mapping Rules

Color is reserved for security meaning.

Forbidden uses:

```
decorative gradients
random colors
```

Approved palette:

```
critical → red
high → orange
medium → yellow
low → green
info → blue
```

Backgrounds must remain neutral.

---

# 9. Visualization Hierarchy

Every page must follow visual hierarchy.

Example:

```
KPI row

Primary chart

Secondary panels

Data tables
```

Charts must **not compete visually**.

Only one primary visualization per page.

---

# 10. Interaction Patterns

Charts must support exploration.

Allowed interactions:

```
hover tooltips
click to filter
range selection
```

Example:

```
Click histogram bucket → filter risk table
```

---

# 11. Accessibility

Charts must remain readable.

Requirements:

```
contrast ratio > 4.5
icons for severity
labels visible
```

Color cannot be the only signal.

Example:

```
Critical = red + icon
```

---

# 12. Anti-Patterns

Avoid:

### Overly complex charts

Security users prefer clarity.

Bad example:

```
radar charts
3D charts
```

---

### Excessive animations

Security dashboards are analytical tools.

Animations should be minimal.

---

### Mixed color semantics

Red must always mean danger.

Never reuse red for neutral metrics.

---

# 13. Implementation Recommendation

Use a single chart library.

Recommended:

```
recharts
or
visx
```

All charts must share:

```
theme
tooltip style
axis style
legend style
```

---

END OF DOCUMENT
