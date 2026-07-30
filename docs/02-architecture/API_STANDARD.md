# API Architecture And Route Standard

## Overview

This document defines the API architecture and routing standards for the Fortuna platform. Goals: standardize REST structure, eliminate route duplication, prevent router conflicts, simplify frontend integration.

## System API Architecture

```
Dashboard (React + JWT) → Nginx proxy → Core API (Gin :8080)
Agent (gRPC + mTLS) → Core gRPC (:9090) + HTTP fallback (:8080)
```

- **Authentication:** JWT Bearer token for user-facing API; mTLS for agent communication
- **Format:** JSON (REST), Protobuf (gRPC)
- **Versioning:** `/api/v1/*`

## Route Domains

### Auth & Identity

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/auth/login` | User login → JWT |
| POST | `/api/v1/auth/register` | User registration |
| GET | `/api/v1/me` | Current user profile |
| PUT | `/api/v1/change-password` | Change password |

### Cluster & Inventory

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/clusters` | List clusters |
| GET | `/api/v1/clusters/stats` | Cluster statistics |
| GET | `/api/v1/clusters/:id` | Cluster detail |
| GET | `/api/v1/inventory/pod-capabilities` | Pod capabilities list |
| GET | `/api/v1/inventory/pod-capabilities/summary` | PCE summary |
| GET | `/api/v1/inventory/pod-capabilities/trends` | PCE trends |

### Risk & Insights

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/risk/insights` | List insights (risks) |
| GET | `/api/v1/risk/insights/:id` | Insight detail |
| POST | `/api/v1/risk/insights/:id/resolve` | Resolve insight |
| POST | `/api/v1/risk/insights/:id/acknowledge` | Acknowledge insight |
| POST | `/api/v1/risk/insights/:id/dismiss` | Dismiss insight |
| GET | `/api/v1/risk/rules` | List risk rules |
| POST | `/api/v1/risk/rules` | Create risk rule |
| PUT | `/api/v1/risk/rules/:id` | Update risk rule |
| DELETE | `/api/v1/risk/rules/:id` | Delete risk rule |
| POST | `/api/v1/risk/rules/validate` | Validate rule (dry-run) |
| POST | `/api/v1/risk/rules/import` | Import rule from YAML |
| GET | `/api/v1/risk/rules/export` | Export rules to YAML |

### Runtime

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/runtime/signals` | Runtime security signals |
| POST | `/api/v1/runtime/events` | Ingest runtime events |
| GET | `/api/v1/runtime/pods/:uid/metrics` | Pod runtime metrics |
| GET | `/api/v1/runtime/pods/:uid/processes` | Pod processes |
| GET | `/api/v1/runtime/pods/:uid/network` | Pod network connections |
| GET | `/api/v1/runtime/pods/:uid/network/top-destinations` | Pod top destinations |
| GET | `/api/v1/runtime/pods/:uid/events` | Pod K8s events |
| GET | `/api/v1/runtime/network-activity` | Cluster-wide network activity |

### Policy

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/policy/rules` | List policy rules (YAML catalog) |
| GET | `/api/v1/policy/rules/uid/:uid` | Policy rule detail by stable UID |
| POST | `/api/v1/policy/rules/reload` | Reload rules from YAML |
| GET | `/api/v1/policy/templates` | Policy templates |
| GET | `/api/v1/policy/instances` | Policy instances |

### SBOM & CVE

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/sbom` | List SBOMs per pod |
| GET | `/api/v1/sbom/:id` | SBOM detail |
| GET | `/api/v1/sbom/:id/components` | SBOM components |
| GET | `/api/v1/sbom/:id/cves` | CVE matches for SBOM |

### Agent Ingest

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/agent/sync` | Full pod sync (gRPC preferred) |
| POST | `/api/v1/agent/pod-runtime-metrics` | Pod runtime metrics |
| POST | `/api/v1/agent/pod-processes` | Pod processes |
| POST | `/api/v1/agent/pod-network-connections` | Pod network connections |
| POST | `/api/v1/agent/pod-events` | K8s events |

### Dashboard Aggregates

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/dashboard/stats` | Global dashboard stats |
| GET | `/api/v1/risk/insights/summary` | Risk breakdown by severity |

### Utility

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/health` | Health check |
| GET | `/api/v1/ws/pod/:uid` | WebSocket: pod detail live |
| GET | `/api/v1/ws/risks` | WebSocket: risk updates |
| GET | `/api/v1/capability-metadata` | Capability catalog |
| GET | `/api/v1/notifications` | Notifications |

## Route Design Principles

1. **Domain-first:** Routes organized by business domain (risk, runtime, inventory, policy)
2. **Ingest separated:** Agent ingest routes under `/agent/*`, separate from user-facing API
3. **Dashboard as aggregate:** `/dashboard/*` for pre-computed/cached data, not domain logic
4. **Consistent verbs:** GET (read), POST (create/action), PUT (update), DELETE (remove)
5. **Nested resources:** `/:domain/:resource/:id/:sub-resource`

## Risk Rules vs Policy Rules

| Aspect | Risk Rules | Policy Rules |
|--------|-----------|--------------|
| **API** | `/api/v1/risk/rules` (CRUD) | `/api/v1/policy/rules` (read + reload) |
| **Storage** | Database (`risk_rules` table) | YAML files (`FORTUNA_RULES_DIR`) |
| **UI** | Settings → Risk Rules | Rules page |
| **Purpose** | Operational: evaluate resources → create insights | Catalog: browse rules, test, view metrics |
| **Engine** | RiskWorker (DB priority, then YAML fallback) | RulesManager / YAMLEngine |
| **Format support** | JSON + YAML (validate, import, export) | YAML only |

## Error Response Format

```json
{
  "error": "string",
  "code": "string",
  "details": {}
}
```

Standard HTTP status codes: 200 (OK), 201 (Created), 400 (Bad Request), 401 (Unauthorized), 403 (Forbidden), 404 (Not Found), 500 (Internal Server Error).
