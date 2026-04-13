# Dashboard Component

## Overview

The Fortuna Security Dashboard is a React 19 single-page application providing security visibility for Kubernetes clusters. It serves as the primary UI for security investigation, risk analysis, and operational monitoring.

**Tech Stack:** React 19, Vite, TypeScript, Tailwind CSS, Zustand (state), Recharts + D3 (charts), Lucide React (icons)

**Deployment:** Nginx serves static files + proxies `/api/*` to Core (`fortuna-core:8080`). Access via `kubectl port-forward -n fortuna svc/fortuna-dashboard 8081:80`.

## Architecture

### Page Tree

```
Dashboard (Home)
├── Clusters → Cluster Detail (Overview / Inventory / Agents / Security)
├── Resources Explorer (Pods / ServiceAccounts / Roles / RoleBindings)
│   └── Pod Detail (Runtime / Processes / Network / Events / Security)
├── Risk Center (Insights) → Risk Detail
│   └── Reference tab (Runtime Signals, Capability Metadata)
├── Capabilities → Capability Detail
├── Network Activity (connections / pods / destinations / talkers views)
├── SBOM Analysis → SBOM Detail (components, CVEs)
├── Settings (Risk Rules, Policy Rules)
└── Notifications / Audit
```

### Data Flow

```
Browser → Dashboard (Nginx :80)
    ↓ /api/* proxy
Core API (:8080)
    ↓
PostgreSQL / NATS
```

- **Authentication:** JWT Bearer token. Core requires `AUTH_ENABLED=true`.
- **Auto-refresh:** Configurable polling interval per page (default 30s). Store: `useRefreshStore`.
- **Time window:** Risk Center and Runtime Signals use `sinceMinutes` filter (All / 5m / 10m / 15m / 30m). Store: `timeWindowStore` (localStorage key: `fortuna-time-window`, default: 15).

### Cluster Identity Flow

```
Kubernetes API → Agent (auto-discovery) → Core (normalize + deduplicate) → DB → API → Dashboard
```

Agent discovers cluster metadata (kube-system namespace UID, API server URL, version) and reports to Core. Core maintains single source of truth in `clusters` table.

## Key API Endpoints

| Endpoint | Purpose | Dashboard Page |
|----------|---------|---------------|
| `GET /api/v1/clusters` | Cluster list with stats | Clusters |
| `GET /api/v1/clusters/stats` | Cluster summary stats | Dashboard Home |
| `GET /api/v1/dashboard/stats` | Global dashboard statistics | Dashboard Home |
| `GET /api/v1/insights/summary` | Risk breakdown (C/H/M/L) | Dashboard Home |
| `GET /api/v1/risks` | Risk/insight list (filterable) | Risk Center |
| `GET /api/v1/risks/:id` | Risk detail | Risk Detail |
| `GET /api/v1/runtime-signals` | Runtime security signals | Risk Center Reference |
| `GET /api/v1/pod-capabilities/summary` | PCE summary | Dashboard Home, Capabilities |
| `GET /api/v1/sbom` | SBOM list per pod | SBOM Analysis |
| `GET /api/v1/runtime/network-activity` | Network activity (cluster-wide) | Network Activity |
| `GET /runtime/pods/:uid/network` | Pod network connections | Pod Detail |
| `GET /runtime/pods/:uid/metrics` | Pod runtime metrics | Pod Detail |
| `GET /runtime/pods/:uid/processes` | Pod processes | Pod Detail |

## Technical Details

### Key Stores (Zustand)

| Store | Purpose |
|-------|---------|
| `useRefreshStore` | Auto-refresh interval management |
| `timeWindowStore` | Time window filter (sinceMinutes) |
| `useClusterStore` | Global cluster selection (planned) |

### Cross-links Between Pages

| Source | Link Target | URL Pattern |
|--------|------------|-------------|
| Resources Explorer → Pod | Pod Detail | `/resources/pods/${pod.id}` |
| Resources Explorer → SA | Identity Detail | `/identities/uid/${resource.id}` |
| Pod Detail → Node | Node Detail | `/clusters/${clusterId}/nodes/${nodeName}` |
| Risk Center → Risk | Risk Detail | `/risks/${risk.id}` |
| Severity card → Risk Center | Pre-filtered risks | `/risks?severity=critical` |

### Build & Deploy

```bash
cd dashboard && npm run build    # Vite build → dist/
# Docker: dashboard/Dockerfile (nginx + static files)
```

### Key Files

| File | Purpose |
|------|---------|
| `dashboard/pages/PodDetail.tsx` | Pod Detail page |
| `dashboard/pages/NetworkActivity.tsx` | Network Activity page |
| `dashboard/pages/Insights.tsx` | Risk Center |
| `dashboard/components/CapabilityMetadataBrowser.tsx` | Capability catalog browser |
| `dashboard/lib/api.ts` | API client functions |
| `dashboard/types.ts` | TypeScript type definitions |
| `dashboard/nginx.conf` | Nginx proxy config |

## Related Documentation

- [Pod Detail Component](../podDetail/README.md)
- [Risk Center Component](../risk-center/README.md)
- [Network Activity Component](../networkActivity/README.md)
