# FORTUNA UI LAYOUT PATTERNS

Version: 1.0
Purpose: Define layout patterns used across Fortuna.

These patterns ensure all pages share consistent structure.

---

# 1. Core Layout Philosophy

Security platforms must balance:

* high information density
* fast scanning
* logical grouping
* strong hierarchy

Fortuna uses a **structured dashboard architecture**.

Hierarchy:

```
Page
 ├ Header
 ├ KPI Section
 ├ Visualization Section
 ├ Data Section
 └ Navigation
```

---

# 2. Dashboard Layout

Used by:

* Dashboard
* Risk Center

Pattern:

```
Header
KPI Row
Primary Visualization
Secondary Panels
Tables
```

Wireframe:

```
Header

KPI KPI KPI KPI

Trend Chart

Top Risks | Summary
```

Rules:

Trend chart must dominate visual space.

---

# 3. Analytics Layout

Used by:

* Risk Findings
* PCE Exposure

Pattern:

```
Header
Visualization
Filters
Table
```

Wireframe:

```
Histogram

Filter Bar

Large Table
```

Rules:

Filters must be visible.

Recommended improvement:

```
sticky filter bar
```

---

# 4. Detail Page Layout

Used by:

* Pod Detail
* Cluster Detail
* Risk Detail

Pattern:

```
Header
Summary Row
Tabs
Tab Content
Related Links
```

Wireframe:

```
Pod Name + Metadata

Summary Cards

Tabs

Content

Related Resources
```

Rules:

Tabs must be visible and clear.

---

# 5. Table Page Layout

Used by:

* Agents
* Resources
* Risk Rules

Pattern:

```
Header
Filters
Table
Pagination
```

Wireframe:

```
Header

Filters

Table

Pagination
```

Tables must support:

* sorting
* filtering
* pagination

---

# 6. Chart Layout Patterns

Charts must follow consistent hierarchy.

### Primary chart

Large chart.

Example:

```
Risk Trend
PCE Trend
```

### Secondary chart

Small supporting visualization.

Example:

```
Risk score histogram
severity distribution
```

Layout:

```
Primary chart: 2 columns
Secondary: 1 column
```

---

# 7. Detail Tabs Pattern

Tabs used for entity exploration.

Example:

```
Overview
Metrics
Security
Events
Spec
```

Rules:

* max 7 tabs
* short labels
* icons optional

---

# 8. Navigation Patterns

Navigation must follow a predictable flow.

Example:

```
Dashboard
 → Risk Center
 → Resource List
 → Resource Detail
```

Avoid deep navigation.

---

# 9. KPI Layout

KPI cards must follow a standard grid.

Pattern:

```
grid-cols-4
```

Example KPIs:

```
Clusters
Pods
Agents
Risks
```

---

# 10. Future Layout Improvements

Possible future enhancements:

Attack path visualization

```
Graph topology
```

Runtime security timeline

```
event timeline
```

Cluster security posture map

```
cluster topology graph
```

---

# 11. Layout Consistency Rules

Every page must:

* use PageLayout
* use Section blocks
* avoid custom grid logic
* follow typography hierarchy

---

# 12. Expected Result

After applying layout patterns:

* UI becomes predictable
* users learn navigation faster
* development becomes faster
* platform looks enterprise-grade

---

END OF DOCUMENT
