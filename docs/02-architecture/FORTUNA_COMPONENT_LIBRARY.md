# FORTUNA COMPONENT LIBRARY

Version: 1.0
Framework: React + Tailwind
Purpose: Define reusable UI components and enforce consistent UI architecture across Fortuna.

---

# 1. Why Component Library Matters

Fortuna UI currently has many reusable patterns:

* cards
* tabs
* page headers
* filter bars
* tables
* KPI panels

However these patterns are **implemented per page instead of being centralized components**.

This creates problems:

* duplicated code
* inconsistent UI
* layout drift
* harder maintenance

The goal is to extract these patterns into a **centralized component library**.

---

# 2. Component Architecture

All shared UI components must live under:

```
dashboard/
 ├ design-system/
 │   ├ tokens/
 │   ├ components/
 │   ├ layouts/
 │   └ charts/
```

Recommended structure:

```
design-system
 ├ tokens
 │   spacing.ts
 │   typography.ts
 │   colors.ts
 │
 ├ components
 │   Card.tsx
 │   Section.tsx
 │   Badge.tsx
 │   Button.tsx
 │   Tabs.tsx
 │   Input.tsx
 │   Table.tsx
 │
 ├ layouts
 │   PageLayout.tsx
 │   PageHeader.tsx
 │   KPIGrid.tsx
 │
 ├ charts
 │   TrendChart.tsx
 │   Histogram.tsx
 │   Heatmap.tsx
```

---

# 3. Core Layout Components

## PageLayout

Wrapper for every page.

Responsibilities:

* container width
* page spacing
* vertical rhythm

Example:

```tsx
<PageLayout>
   <PageHeader />
   <Section />
   <Section />
</PageLayout>
```

Implementation guideline:

```
max-w-[1600px]
mx-auto
px-6
space-y-8
```

---

## PageHeader

Displays page title and actions.

Structure:

```
Title
Description
Actions
```

Example:

```tsx
<PageHeader
  title="Risk Center"
  description="Security insights across clusters"
  actions={<Button>Export</Button>}
/>
```

Rules:

* Title → `text-2xl`
* Description → `text-sm`
* Actions aligned right

---

## Section

Sections group logical content blocks.

Example:

```tsx
<Section title="Risk Overview">
   <Card />
</Section>
```

Spacing:

```
space-y-6
```

---

# 4. Core UI Components

## Card

The most frequently used component.

Variants:

```
primary
secondary
panel
```

Example:

```tsx
<Card variant="primary">
   <CardHeader title="Risk Trend" />
   <CardContent>
      <TrendChart />
   </CardContent>
</Card>
```

Variant rules:

| Variant   | Padding |
| --------- | ------- |
| primary   | p-6     |
| secondary | p-4     |
| panel     | p-3     |

---

## Badge

Used for severity, status, labels.

Example:

```tsx
<Badge variant="critical">Critical</Badge>
```

Variants:

```
critical
high
medium
low
info
```

Typography:

```
text-[11px]
font-semibold
```

---

## Button

Button variants:

```
primary
secondary
ghost
danger
```

Sizes:

```
sm
md
lg
```

Example:

```tsx
<Button variant="primary" size="md">
  View Risk
</Button>
```

---

## Tabs

Standardized tab navigation.

Example:

```tsx
<Tabs
  value={activeTab}
  onChange={setActiveTab}
  items={[
    { id: "overview", label: "Overview" },
    { id: "metrics", label: "Metrics" }
  ]}
/>
```

Styling:

```
inline-flex
gap-1
rounded-lg
p-1
```

---

## Table

Tables must be consistent.

Example:

```tsx
<Table
  columns={columns}
  data={rows}
/>
```

Table responsibilities:

* column layout
* sorting
* pagination
* row styling

Row height:

```
36px
```

---

# 5. Data Visualization Components

Security platforms rely heavily on charts.

Standard chart components:

```
TrendChart
Histogram
Heatmap
SeverityDistribution
```

Responsibilities:

* consistent axes
* tooltip styling
* color mapping

Charts must share the same theme.

---

# 6. Filter Components

Filtering is critical in security tools.

Shared filter components:

```
FilterBar
SearchInput
SelectFilter
DateRangeFilter
```

Example:

```tsx
<FilterBar>
  <SelectFilter label="Severity" />
  <SelectFilter label="Namespace" />
  <SearchInput />
</FilterBar>
```

---

# 7. Navigation Components

Reusable navigation elements:

```
SegmentTabs
Breadcrumb
BackButton
```

Example:

```tsx
<Breadcrumb
  items={[
    "Resources",
    "Pods",
    "Pod Detail"
  ]}
/>
```

---

# 8. Anti-Patterns

Avoid:

### Page-specific card implementations

Bad:

```
<Card className="p-6 shadow custom">
```

Good:

```
<Card variant="primary">
```

---

### Hardcoded spacing

Bad:

```
mt-3
gap-5
```

Good:

```
gap-4
gap-6
```

---

### Inline layout logic

Layout logic must live in components.

---

# 9. Migration Strategy

Migration must be gradual.

Step 1

Extract:

* Card
* PageHeader
* Tabs

Step 2

Extract:

* Table
* FilterBar

Step 3

Extract:

* Charts
* KPI panels

---

# 10. Expected Outcome

After adopting the component library:

* UI becomes predictable
* pages become smaller
* developers build faster
* design consistency improves dramatically

Most importantly:

Future UI development will remain stable.

---

END OF DOCUMENT
