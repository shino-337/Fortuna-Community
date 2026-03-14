# FORTUNA UI DESIGN SYSTEM

Version: 1.0
Product: Fortuna Kubernetes Security Platform
Purpose: Standardize UI layout, spacing, typography and components while preserving the current visual style.

---

# 1. Executive Summary

Fortuna already has a solid UI foundation:

* Tailwind CSS based styling
* consistent dark theme
* grid layout patterns
* reusable cards and tables
* logical page structure

However, the system currently behaves as a **semi-structured UI system** rather than a **formal design system**.

Common issues observed:

* inconsistent typography hierarchy
* inconsistent spacing usage
* card hierarchy not clearly defined
* container width uncontrolled
* inconsistent component sizing
* layout patterns not formally documented

These issues create visual inconsistencies across pages.

The goal of this document is **not to redesign the UI**, but to:

* standardize UI rules
* improve consistency
* formalize layout patterns
* prevent future UI drift

The existing visual identity must remain unchanged.

---

# 2. Current UI Assessment

## Strengths

Fortuna UI already demonstrates strong architectural structure.

### Layout

* consistent page header pattern
* grid-based KPI layout
* structured card composition
* predictable dashboard patterns

### Visual Identity

* consistent dark theme
* strong contrast hierarchy
* appropriate icon usage
* well-structured table design

### Page Architecture

Pages follow a consistent hierarchy:

```
Page
 ├ Page Header
 ├ KPI Section
 ├ Visualization Section
 ├ Table Section
 └ Navigation / Related Links
```

This is already close to enterprise-grade UI architecture.

---

## Weaknesses

### 1 Typography Scale

Font sizes exist but are not formally standardized.

Observed sizes:

```
10px
11px
12px
14px
18px
24px
```

Developers sometimes mix these arbitrarily.

---

### 2 Spacing System

Spacing exists but lacks strict rules.

Current usage:

```
gap-1
gap-2
gap-3
gap-4
gap-6
```

Without restrictions, UI drift happens quickly.

---

### 3 Card Hierarchy

Multiple card types exist but are not defined.

Examples:

* large data cards
* small navigation cards
* information panels

They should have **clear variants**.

---

### 4 Container Width

Currently:

```
max-w-full
```

This causes UI stretching on large displays.

---

### 5 Component Sizing

Buttons, inputs, and icons do not yet follow a strict sizing system.

---

# 3. Design System Principles

Fortuna UI should follow five design principles.

### Consistency

Every page must follow the same design rules.

### Predictability

Users should instantly understand layout patterns.

### Visual Hierarchy

Important data must stand out clearly.

### Density Balance

Security platforms require dense information without visual clutter.

### Component Reuse

All UI patterns should be reusable components.

---

# 4. Typography System

Typography hierarchy must be fixed.

## Font Family

```
Primary: Inter / system sans-serif
Monospace: font-mono
```

Monospace should be used for:

* IDs
* IP addresses
* commands
* hashes

---

## Typography Scale

| Usage       | Size | Tailwind    |
| ----------- | ---- | ----------- |
| Page Title  | 24px | text-2xl    |
| Card Title  | 18px | text-lg     |
| Body        | 14px | text-sm     |
| Helper      | 12px | text-xs     |
| Badge       | 11px | text-[11px] |
| Micro Label | 10px | text-[10px] |

---

## Typography Rules

Page titles must use:

```
text-2xl font-bold
```

Card titles must use:

```
text-lg font-semibold
```

Section labels must use:

```
text-xs uppercase tracking-wider
```

Body content must use:

```
text-sm
```

Badges must use:

```
text-[11px]
```

---

# 5. Spacing System

Spacing must follow an **8px scale**.

Allowed spacing tokens:

```
gap-2   (8px)
gap-4   (16px)
gap-6   (24px)
gap-8   (32px)
```

Avoid:

```
gap-1
gap-3
gap-5
```

---

## Section Spacing

Page sections must use:

```
space-y-8
```

Card internal spacing:

```
p-6 (primary)
p-4 (secondary)
p-3 (small panel)
```

---

# 6. Layout System

## Page Container

Pages must use a constrained layout.

```
max-w-[1600px]
mx-auto
px-6
```

This prevents layout stretching on ultra-wide displays.

---

## Grid System

Fortuna uses a **12 column grid conceptually**.

Typical layouts:

### KPI Layout

```
grid-cols-2 md:grid-cols-4
gap-4
```

---

### Dashboard Layout

```
grid-cols-3
Trend: span 2
Distribution: span 1
```

---

### Detail Layout

```
grid-cols-2
```

---

# 7. Card System

Cards must be categorized.

---

## Primary Card

Used for:

* charts
* main data blocks
* tables

Structure:

```
<Card class="p-6">
  Title
  Content
</Card>
```

---

## Secondary Card

Used for:

* quick links
* summaries
* navigation blocks

Structure:

```
<Card class="p-4">
  Small title
  Content
</Card>
```

---

## Micro Section

Used inside cards.

Example:

```
Service Account
Namespace
Owner
```

Style:

```
text-xs uppercase
```

---

# 8. Component Sizing

## Buttons

| Size    | Height |
| ------- | ------ |
| SM      | 32px   |
| Default | 40px   |
| LG      | 48px   |

Standard padding:

```
px-4 py-2
```

---

## Inputs

All inputs must use:

```
h-10
text-sm
```

---

## Icons

| Usage   | Size |
| ------- | ---- |
| Inline  | 16px |
| Section | 20px |
| Feature | 24px |

---

# 9. Tabs System

Tabs must use a shared component.

Example:

```
<SegmentTabs />
```

Standard structure:

```
inline-flex flex-wrap
gap-1
p-1
rounded-lg
```

Avoid:

```
w-fit
```

---

# 10. Table Design

Tables should remain dense but readable.

Row style:

```
py-2 text-sm
```

Header style:

```
font-medium
bg-slate-800
```

Rules:

* no excessive row padding
* maintain compact vertical density

---

# 11. Wireframe Analysis & Improvements

## Dashboard

Current layout is strong.

Recommended adjustment:

Trend chart should visually dominate.

Current concept:

```
Trend + PCE
Top Risks
PCE Summary
```

Suggested improvement:

```
Trend (full width)
Top Risks + PCE Summary
```

Trend is the most important signal.

---

## Risk Center Overview

Current layout:

```
Trend (2/3)
Histogram (1/3)
```

This is correct.

Improvement suggestion:

Add **risk velocity indicator** near the chart.

---

## Risk Findings Page

Filter bar is good but can improve usability.

Recommendation:

```
Sticky filter bar
```

When scrolling large tables.

---

## PCE Page

Layout:

```
Trend
Heatmap + Table
```

Correct architecture.

Improvement:

Allow clicking heatmap cell to filter table.

---

## Pod Detail

Layout is good but the hero section could be improved.

Current:

```
Pod name
Namespace
Node
```

Suggestion:

Add:

```
Status
Risk level
Restart count
```

Directly in the header row.

---

## Cluster Detail

Inventory tab could be visually improved.

Suggestion:

Replace simple lists with:

```
card-based node summary
```

---

## Risk Detail

Current layout works but should evolve later into:

```
attack path graph
```

Future improvement.

---

# 12. Component Architecture

Design system folder should exist.

```
/dashboard/design-system
```

Structure:

```
tokens/
  spacing.ts
  typography.ts
  layout.ts

components/
  Card.tsx
  PageLayout.tsx
  PageHeader.tsx
  Section.tsx
  Tabs.tsx
```

---

# 13. Refactor Roadmap

Refactoring should happen in phases.

---

## Phase 1 — Typography + Spacing

Tasks:

* standardize text hierarchy
* enforce spacing scale
* update card padding

---

## Phase 2 — Layout Standardization

Tasks:

* introduce max container width
* standardize grid usage
* normalize card hierarchy

---

## Phase 3 — Component Extraction

Tasks:

* extract Card component
* extract Tabs component
* extract PageHeader

---

## Phase 4 — Page Cleanup

Tasks:

* unify dashboard layout
* unify detail page layout
* normalize table styling

---

# 14. Expected Outcome

After applying this design system:

* UI will become visually consistent
* layouts will feel balanced
* components will be reusable
* future UI development will remain stable

Most importantly:

Fortuna UI will move from a **developer UI** to a **professional security platform UI**.

---

END OF DOCUMENT
