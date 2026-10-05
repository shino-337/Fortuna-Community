# API Architecture And Route Standard

This document describes the public API shape used by Fortuna Core. It is intentionally a routing standard and domain map, not a full endpoint reference.

## Transport

```text
Dashboard -> nginx -> Core REST API :8080
Agent -> Core gRPC :9090 over mTLS
Agent and sensors -> Core HTTP ingest :8080 with a per-Agent token (or the legacy shared token)
```

- User-facing REST APIs use JSON under `/api/v1`.
- Runtime layer APIs also expose selected `/api/v2/runtime` endpoints.
- Agent gRPC traffic uses mTLS.
- HTTP ingest routes require an Agent credential: a per-Agent token when `FORTUNA_AGENT_CREDENTIAL_REGISTRY` is set, otherwise the legacy shared `FORTUNA_INGEST_TOKEN`. See [scoped Agent credentials](../../deploy/scoped-agent-credentials/README.md).
- Protected user routes require JWT auth. `AUTH_ENABLED=false` is refused unless `FORTUNA_DEV_MODE=1`.

## Route Domains

| Domain | Prefix | Purpose |
|--------|--------|---------|
| Auth | `/api/v1/auth/*`, `/api/v1/me`, `/api/v1/change-password` | Login, registration, current user, password change |
| Users and sessions | `/api/v1/users/*`, `/api/v1/sessions/*` | User and session administration |
| Dashboard | `/api/v1/dashboard/*` | Dashboard summary and metric aggregates |
| Inventory | `/api/v1/inventory/*`, `/api/v1/resources/*` | Pods, service accounts, deployments, replicasets, clusters and their nodes, SBOM inventory |
| Cluster certificates | `/api/v1/cluster/certificates/*` | Core certificate information; rotation only when `TLS_ENABLED=true` |
| Risk | `/api/v1/risk/*` | Insights, findings workflow, risk scores, risk analytics, exceptions |
| Policy | `/api/v1/policy/*` | Policy rules, templates, instances, rule metrics and matches |
| Runtime | `/api/v1/runtime/*`, `/api/v2/runtime/*` | Runtime events, signals, process/network facts |
| Graph | `/api/v1/graph`, `/api/v1/graph/attack-paths/*` | Cluster-scoped graph and attack paths |
| Audit and governance | `/api/v1/audit/*`, `/api/v1/governance/*` | Audit logs, reports, security activity, access review |
| Investigations | `/api/v1/investigations/*` | Investigation cases, timeline, pinned entities |
| Malware | `/api/v1/malware/*` | Malware threat views |
| Agent ingest | `/api/v1/agent/*`, `POST /api/v2/runtime/{events,producers,coverage,source-health}` | Agent inventory, Pod Detail and runtime HTTP ingest |
| WebSocket | `/api/v1/ws/*` | Live pod detail and risk updates; the JWT travels in `Sec-WebSocket-Protocol` (`fortuna.v1`, `fortuna.bearer.<jwt>`), never in the URL |
| Observability | `/api/v1/metrics/*`, `/api/v1/monitoring/*`, `/api/v1/error-logs`, `/api/v1/agents/status`, `/api/v1/health/dashboard-data-integrity` | System, worker, agent, pipeline, catalog health and log views |
| Notifications | `/api/v1/notifications*` | Notification list and read state |
| Capability catalog | `/api/v1/capability-metadata*`, `/api/v1/promotion-rules*` | Capability metadata and promotion rules |

## Route Design Principles

1. Group routes by product domain.
2. Keep agent ingest separate from user-facing read APIs.
3. Use `/dashboard/*` for aggregate views, not domain ownership.
4. Use stable Kubernetes UIDs for pod-scoped resources.
5. Enforce cluster scope on cluster-owned records.
6. Protect every user route with explicit permission middleware.
7. Use POST for actions and ingest, PATCH for partial state changes, PUT for updates, DELETE for removal. Policy instance and template PUT are partial: omitted fields keep their stored values.

## Common Endpoint Patterns

| Pattern | Example | Notes |
|---------|---------|-------|
| Collection read | `GET /api/v1/risk/insights` | Supports filters where implemented by the handler |
| Entity read | `GET /api/v1/inventory/pods/:uid` | Pod and resource routes prefer Kubernetes UID |
| Action | `POST /api/v1/risk/insights/:id/acknowledge` | Workflow state transitions are action endpoints |
| Partial update | `PATCH /api/v1/risk/insights/:id` | Used for partial status changes |
| Bulk action | `POST /api/v1/risk/insights/bulk` | Used when a command targets multiple entities |
| Ingest | `POST /api/v1/agent/sync` | Authenticated with an Agent credential, not a user JWT |

## Error Response Format

```json
{
  "error": "string",
  "code": "string"
}
```

Every error carries `error`; `code` (and `retryable` on some 503 responses) is added where clients branch on the cause. Status codes in use: `200`, `201`, `202`, `204`, `400`, `401`, `403`, `404`, `409`, `413`, `429`, `500`, `501` and `503`. A 5xx body never contains internal error text.

## Implementation Source

The active route contract is registered in `core/internal/api/routes.go` and the domain route files beside it. When changing routes, update those registrations and this document together.
