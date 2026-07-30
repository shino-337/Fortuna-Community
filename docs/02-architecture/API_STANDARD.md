# API Architecture And Route Standard

This document describes the public API shape used by Fortuna Core. It is intentionally a routing standard and domain map, not a full endpoint reference.

## Transport

```text
Dashboard -> nginx -> Core REST API :8080
Agent -> Core gRPC :9090 over mTLS
Agent and sensors -> Core HTTP ingest :8080 with ingest token
```

- User-facing REST APIs use JSON under `/api/v1`.
- Runtime layer APIs also expose selected `/api/v2/runtime` endpoints.
- Agent gRPC traffic uses mTLS.
- HTTP ingest routes require `FORTUNA_INGEST_TOKEN`.
- Protected user routes require JWT auth when `AUTH_ENABLED=true`.

## Route Domains

| Domain | Prefix | Purpose |
|--------|--------|---------|
| Auth | `/api/v1/auth/*`, `/api/v1/me`, `/api/v1/change-password` | Login, registration, current user, password change |
| Users and sessions | `/api/v1/users/*`, `/api/v1/sessions/*` | User and session administration |
| Dashboard | `/api/v1/dashboard/*` | Dashboard summary and metric aggregates |
| Inventory | `/api/v1/inventory/*`, `/api/v1/resources/*` | Pods, service accounts, deployments, replicasets, clusters, SBOM inventory |
| Cluster operations | `/api/v1/cluster/*` | Cluster info, nodes, and certificate operations |
| Risk | `/api/v1/risk/*` | Insights, findings workflow, risk scores, risk analytics, runtime risk, exceptions |
| Policy | `/api/v1/policy/*` | Policy rules, templates, instances, rule metrics and matches |
| Runtime | `/api/v1/runtime/*`, `/api/v2/runtime/*` | Runtime events, signals, process/network facts, runtime security state |
| Graph | `/api/v1/graph/*` | Graph summary, blast radius, attack paths, permissions, advanced graph query |
| Audit and governance | `/api/v1/audit/*`, `/api/v1/governance/*` | Audit logs, reports, security activity, access review |
| Investigations | `/api/v1/investigations/*` | Investigation cases, timeline, pinned entities |
| Malware | `/api/v1/malware/*` | Malware package checks and threat views |
| Agent ingest | `/api/v1/agent/*` | Agent inventory/runtime HTTP ingest fallback |
| WebSocket | `/api/v1/ws/*` | Live pod detail and risk updates |
| Observability | `/api/v1/metrics/*`, `/api/v1/monitoring/*`, `/api/v1/error-logs` | System, worker, agent, pipeline, and log views |

## Route Design Principles

1. Group routes by product domain.
2. Keep agent ingest separate from user-facing read APIs.
3. Use `/dashboard/*` for aggregate views, not domain ownership.
4. Use stable Kubernetes UIDs for pod-scoped resources.
5. Enforce cluster scope on cluster-owned records.
6. Protect every user route with explicit permission middleware.
7. Use POST for actions and ingest, PATCH for partial state changes, PUT for full updates, DELETE for removal.

## Common Endpoint Patterns

| Pattern | Example | Notes |
|---------|---------|-------|
| Collection read | `GET /api/v1/risk/insights` | Supports filters where implemented by the handler |
| Entity read | `GET /api/v1/inventory/pods/:uid` | Pod and resource routes prefer Kubernetes UID |
| Action | `POST /api/v1/risk/insights/:id/acknowledge` | Workflow state transitions are action endpoints |
| Partial update | `PATCH /api/v1/risk/insights/:id` | Used for partial status changes |
| Bulk action | `POST /api/v1/risk/insights/bulk` | Used when a command targets multiple entities |
| Ingest | `POST /api/v1/agent/sync` | Authenticated with ingest token, not user JWT |

## Error Response Format

```json
{
  "error": "string",
  "code": "string",
  "details": {}
}
```

Standard HTTP status codes are used: `200`, `201`, `400`, `401`, `403`, `404`, and `500`.

## Implementation Source

The active route contract is registered in `core/internal/api/routes.go` and the domain route files beside it. When changing routes, update those registrations and this document together.
