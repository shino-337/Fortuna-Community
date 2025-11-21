# Graph Namespace Filtering Failure Analysis

## Overview
This document explains why the Kubernetes RBAC Graph UI fails to display nodes and edges when filtering by namespace, despite the API returning correct data.

## Root Causes Summary
The failure is not caused by API or backend sync issues. It is primarily due to frontend rendering logic inside **GraphVisualization.tsx**, where nodes are filtered out before reaching Cytoscape, making edges disappear as well.

---

# 1. Root Cause Analysis

## 1.1 Node width/height computed as 0 → node dropped

The UI uses auto‑sizing:

```ts
const validNodes = nodes.filter(node => {
  const width = node.data?.width
  const height = node.data?.height
  return width && height && width > 0 && height > 0
})
```

If width or height = 0 → node is discarded → Cytoscape never sees it.

### Why this happens
- `estimateTextDimensions()` returns `0` when:
  - label is empty
  - label too long and wrapped incorrectly
  - type has no NODE_CONFIGS entry → fallback config missing size
- Namespaces typically return a smaller set of nodes → higher chance all nodes have invalid dimensions → entire graph becomes empty.

---

## 1.2 Missing NODE_CONFIGS causes runaway fallback
Current config only includes:

- serviceaccount
- role
- clusterrole
- namespace
- cluster

But missing essential RBAC types:

- rolebinding
- clusterrolebinding
- reference
- sa-binding
- pod / workload nodes

When a type is missing:

```ts
const nodeConfig = NODE_CONFIGS[node.type] || {}
```

An empty config → minWidth = undefined → computed width may be ≤ 0 → node dropped.

---

## 1.3 Edge filtering depends on node set

```ts
filteredEdges = data.edges.filter(edge =>
  nodeIds.has(edge.source) && nodeIds.has(edge.target)
)
```

If nodeIds = empty → all edges disappear.

Thus, **one lost node cascades into full graph disappearance**.

---

## 1.4 React Query invalidation race condition
In GraphVisualization.tsx:

```ts
queryClient.invalidateQueries({ queryKey: ['graph'] })
refetch()
```

This may cause:
- stale graph for 200–500 ms
- flash of empty graph

This does not create the namespace bug, but makes debugging harder.

---

# 2. Required Fixes

## 2.1 Always guarantee non-zero node dimensions

```ts
const width = Math.max(
  nodeConfig.minWidth,
  Math.min(nodeConfig.maxWidth, (textWidth || 0) + nodeConfig.padding * 2)
)

const height = Math.max(
  nodeConfig.minHeight || 40,
  (textHeight || nodeConfig.fontSize) + nodeConfig.padding * 2
)
```

---

## 2.2 Add missing NODE_CONFIGS for all RBAC objects

Suggested additions:

```ts
rolebinding: {
  shape: "round-rectangle",
  backgroundColor: "#9b59b6",
  fontSize: 13,
  minWidth: 140,
  maxWidth: 240,
},
clusterrolebinding: {
  shape: "round-rectangle",
  backgroundColor: "#8e44ad",
  fontSize: 13,
  minWidth: 140,
  maxWidth: 240,
},
reference: {
  shape: "rectangle",
  backgroundColor: "#7f8c8d",
  fontSize: 12
}
```

---

## 2.3 Introduce fallback node type “unknown”
Prevent silent failures:

```ts
const type = node.type || 'unknown'
const nodeConfig = NODE_CONFIGS[type] || NODE_CONFIGS['unknown']
```

---

## 2.4 Namespace Scoped Graph Logic
When namespace filter active:

Include nodes of type:
- serviceaccount (in namespace)
- role (in namespace)
- rolebinding (in namespace)
- clusterrolebinding where SA is subject
- clusterroles linked to those bindings
- pods using those serviceaccounts

This reduces missing-edge problems.

---

# 3. Verification Checklist

| Test Case | Expectation |
|-----------|-------------|
| Filter namespace = “default” | Graph shows all SA + RB + Roles |
| Filter namespace with 1 SA | Graph should not disappear |
| Missing NODE_CONFIGS | Graph still renders via fallback “unknown” |
| Long labels | Node auto-sizes without shrinking |
| Empty labels | Node keeps minimum size |

---

# 4. Conclusion
The namespace filter bug originates from **node auto-sizing + missing type configs**, not from backend data.  
Once node dimensions are guaranteed and all RBAC types defined, the graph will display reliably in all filtering modes.

