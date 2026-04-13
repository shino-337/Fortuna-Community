# Fortuna UI Design System

**Version**: 2.0 | **Last Updated**: 2026-04-13

## Overview

The Fortuna UI Design System standardizes layout, spacing, typography, and components for the security dashboard. It ensures consistent visual language across all pages while supporting security investigation workflows.

**Foundation:** React 19 + Tailwind CSS + Dark theme

## Design Principles

### Security-First UX

1. **Scan First** — Users understand system state within 3 seconds
2. **Hunt Problems** — UI supports: Alert → Risk discovery → Context analysis → Root cause → Remediation
3. **Object-Centric** — Each entity (Cluster, Pod, Risk, Identity) has its own detail page
4. **Data Integrity** — Only display data with end-to-end traceability (Agent → Core → DB → API → UI)

### Layout Philosophy

```
Page
 ├─ Header (page title + breadcrumbs + actions)
 ├─ KPI Section (summary cards)
 ├─ Visualization Section (charts, graphs)
 ├─ Data Section (tables, lists)
 └─ Navigation (tabs, filters)
```

## Color System

### Severity Colors (Consistent Across All Pages)

| Severity | Tailwind Class | Hex | Usage |
|----------|---------------|-----|-------|
| Critical | `text-red-500` | #EF4444 | Critical risks, P0 alerts |
| High | `text-orange-500` | #F97316 | High severity items |
| Medium | `text-yellow-500` | #EAB308 | Medium severity items |
| Low | `text-blue-500` | #3B82F6 | Low severity, informational |

### Status Colors

| Status | Class | Usage |
|--------|-------|-------|
| Active | `text-red-400` | Active risks/incidents |
| Resolved | `text-green-400` | Resolved items |
| Acknowledged | `text-yellow-400` | Acknowledged but unresolved |
| Healthy | `text-emerald-400` | Healthy clusters/pods |

## Typography

| Level | Element | Size | Weight | Usage |
|-------|---------|------|--------|-------|
| H1 | Page title | `text-2xl` | `font-bold` | One per page |
| H2 | Section title | `text-xl` | `font-semibold` | Section headers |
| H3 | Card title | `text-lg` | `font-medium` | Card/panel headers |
| Body | Content | `text-sm` | `font-normal` | Default text |
| Caption | Metadata | `text-xs` | `font-normal` | Timestamps, IDs |

## Component Patterns

### Summary Card (KPI)

Used for: Dashboard home, Risk Center summary, PCE summary

```
┌─────────────────────┐
│  Label    [icon]    │
│  42                 │ ← large number
│  +3 since last scan │ ← optional delta
└─────────────────────┘
```

### Data Table

Standard across: Risk Center, Resources Explorer, SBOM, Capabilities

- Sortable columns (click header)
- Row click → detail page
- Severity badge (colored dot + text)
- Status badge
- Pagination (page size: 10/25/50)

### Filter Bar

Standard pattern for filterable pages:

```
[Severity ▼] [Status ▼] [Namespace ▼] [Search...] [Time window ▼] [⟳ Refresh]
```

### Detail Page

Standard layout for all detail pages (Risk, Pod, Cluster):

```
┌─────────────────────────────────────┐
│ ← Back    Title    [Actions ▼]     │
├─────────────────────────────────────┤
│ [Overview] [Security] [Events] ... │ ← tabs
├─────────────────────────────────────┤
│ Tab content                         │
└─────────────────────────────────────┘
```

## Data Visualization Guidelines

### Chart Types by Use Case

| Data Type | Chart | Example |
|-----------|-------|---------|
| Risk distribution | Donut/Pie | Severity breakdown |
| Time series | Line/Area | Risk trend, signal frequency |
| Comparison | Horizontal bar | Top risks by score |
| Status overview | Heatmap/Grid | Cluster health matrix |

### Visualization Rules

1. **No decorative charts** — every chart must answer a question
2. **Severity-first coloring** — red=critical is always dominant
3. **Interactive** — click chart segment → filtered view
4. **Time-aware** — show time context for all temporal data
5. **Library:** Recharts (primary) + D3 (custom)

## Component Library (Target)

Currently patterns are implemented per-page. Target: centralized components.

| Component | Status | Usage |
|-----------|--------|-------|
| `SeverityBadge` | ✅ Exists | Risk tables, detail pages |
| `StatusBadge` | ✅ Exists | Risk status display |
| `PageHeader` | 🔶 Partial | Per-page implementations |
| `FilterBar` | 🔶 Partial | Different per page |
| `DataTable` | 🔶 Partial | Shared patterns, not unified |
| `KPICard` | 🔶 Partial | Similar but not identical |
| `DetailPageLayout` | ❌ Missing | Each detail page rolls its own |
| `TabNavigation` | ❌ Missing | Inline per page |

## Known Gaps

| ID | Description | Priority |
|----|-------------|----------|
| DS-1 | Component library not centralized — patterns duplicated per page | P2 |
| DS-2 | Typography hierarchy inconsistent across pages | P3 |
| DS-3 | Spacing/padding not standardized (varies per developer) | P3 |
| DS-4 | Card hierarchy not clearly defined (primary vs secondary) | P3 |
| DS-5 | Container width uncontrolled on wide screens | P3 |
| DS-6 | No formal responsive breakpoint strategy | P3 |
