# Fortuna API Architecture & Route Standard

Version: 1.0
Status: Proposed
Target: Fortuna Core / Dashboard / Agent

---

# 1. Overview

This document defines the **API architecture and routing standard** for the Fortuna security platform.

Goals:

* Standardize REST API structure
* Eliminate duplicated routes
* Prevent router conflicts
* Simplify frontend API integration
* Enable scaling to **20–200 Kubernetes nodes**
* Align with architecture used by modern cloud security platforms such as
  Sysdig and Aqua Security.

---

# 2. System Architecture Overview

Fortuna platform components:

```
+----------------------+
| Dashboard (React)    |
| JWT Auth             |
+----------+-----------+
           |
           v
+----------------------+
| Fortuna Core API     |
| REST API (Gin)       |
+----------+-----------+
           |
           v
+----------------------+
| Event Bus            |
| NATS Stream          |
+----------+-----------+
           |
           v
+----------------------+
| Fortuna Agent        |
| gRPC + mTLS          |
| Runtime Monitoring   |
+----------+-----------+
           |
           v
+----------------------+
| Kubernetes Nodes     |
| containerd runtime   |
+----------------------+
```

Communication model:

| Component        | Protocol    |
| ---------------- | ----------- |
| Dashboard → Core | REST + JWT  |
| Agent → Core     | gRPC + mTLS |
| Core → Event Bus | NATS Stream |

---

# 3. API Base Path

All APIs must use version prefix.

```
/api/v1
```

Example:

```
/api/v1/pods
/api/v1/risk/scores
/api/v1/runtime/events
```

---

# 4. Identifier Rules

Public APIs must **never expose database IDs**.

Use Kubernetes identifiers.

| Resource       | Identifier |
| -------------- | ---------- |
| Pod            | UID        |
| ServiceAccount | UID        |
| Deployment     | UID        |
| Node           | Name       |

Correct:

```
/pods/:uid
```

Incorrect:

```
/pods/by-id/:id
/pods/:id
/pods/by-uid/:uid
```

---

# 5. Resource Naming Convention

Use **plural nouns**.

Correct:

```
pods
nodes
deployments
serviceaccounts
clusters
```

Avoid:

```
pod
node
deployment
service-account
```

---

# 6. Nested Resource Pattern

Subresources must follow hierarchical structure.

Pattern:

```
/resource/:uid/subresource
```

Example:

```
/pods/:uid/processes
/pods/:uid/network
/pods/:uid/events
/pods/:uid/sbom
```

---

# 7. HTTP Method Convention

| Method | Usage            |
| ------ | ---------------- |
| GET    | retrieve data    |
| POST   | create resource  |
| PUT    | replace resource |
| PATCH  | partial update   |
| DELETE | remove resource  |

---

# 8. API Domain Architecture

Fortuna APIs are organized into **four core domains**.

```
inventory
runtime
risk
graph
```

This domain model matches modern cloud security platforms.

---

# 9. Domain: Inventory

Inventory APIs represent **cluster assets**.

Resources:

```
pods
nodes
deployments
serviceaccounts
namespaces
clusters
```

Examples:

```
GET /api/v1/inventory/pods
GET /api/v1/inventory/pods/:uid
GET /api/v1/inventory/serviceaccounts
GET /api/v1/inventory/nodes
```

Subresources:

```
/inventory/pods/:uid/spec
/inventory/pods/:uid/capabilities
/inventory/pods/:uid/volumes
```

---

# 10. Domain: Runtime

Runtime APIs provide **live security telemetry**.

Sources:

* eBPF
* container runtime
* syscall events

Endpoints:

```
GET /api/v1/runtime/events
GET /api/v1/runtime/processes
GET /api/v1/runtime/network
```

Pod-level runtime data:

```
GET /api/v1/runtime/pods/:uid/processes
GET /api/v1/runtime/pods/:uid/network
GET /api/v1/runtime/pods/:uid/events
GET /api/v1/runtime/pods/:uid/metrics
```

---

# 11. Domain: Risk

Risk APIs aggregate security posture.

Examples:

```
GET /api/v1/risk/scores
GET /api/v1/risk/scores/:uid
```

Pod risk report:

```
GET /api/v1/risk/pods/:uid/report
```

Analytics:

```
GET /api/v1/risk/analytics/trends
GET /api/v1/risk/analytics/comparison
GET /api/v1/risk/analytics/correlation
```

---

# 12. Domain: Graph

Graph APIs power **attack path analysis**.

Endpoints:

```
GET /api/v1/graph
GET /api/v1/graph/attack-paths/:uid
GET /api/v1/graph/blast-radius/:uid
```

Graph data sources:

* RBAC relationships
* ServiceAccount usage
* Network connections
* Runtime privilege escalation

---

# 13. Domain: Audit

Audit APIs provide investigation logs.

```
GET /api/v1/audit/logs
GET /api/v1/audit/reports
```

---

# 14. Pod API Specification

## List pods

```
GET /api/v1/inventory/pods
```

Response:

```
{
  "items": [
    {
      "uid": "123",
      "name": "nginx",
      "namespace": "default",
      "node": "node-1",
      "riskScore": 52
    }
  ]
}
```

---

## Pod detail

```
GET /api/v1/inventory/pods/:uid
```

---

## Pod Processes

```
GET /api/v1/runtime/pods/:uid/processes
```

---

## Pod Network

```
GET /api/v1/runtime/pods/:uid/network
```

---

## Pod Runtime Events

```
GET /api/v1/runtime/pods/:uid/events
```

---

## Pod SBOM

```
GET /api/v1/inventory/pods/:uid/sbom
```

---

## Pod Risk Report

```
GET /api/v1/risk/pods/:uid/report
```

---

# 15. Route Refactor Mapping

## Pods

| Old Route                         | New Route                    |
| --------------------------------- | ---------------------------- |
| /pods/by-id/:id                   | removed                      |
| /pods/by-uid/:uid                 | /inventory/pods/:uid         |
| /pods/:podUid/processes           | /runtime/pods/:uid/processes |
| /pods/:podUid/network-connections | /runtime/pods/:uid/network   |
| /pods/:podUid/events              | /runtime/pods/:uid/events    |
| /pods/:podUid/runtime-metrics     | /runtime/pods/:uid/metrics   |

---

## Runtime Risk

Old:

```
/runtime-risk/pods/:podUid
```

New:

```
/risk/pods/:uid/runtime
```

---

## SBOM

Old:

```
/sbom/:podUid
```

New:

```
/inventory/pods/:uid/sbom
```

---

## Risk Report

Old:

```
/risks/pods/:podUid/report
```

New:

```
/risk/pods/:uid/report
```

---

## ServiceAccounts

Old:

```
/serviceaccounts/by-uid/:saUid
```

New:

```
/inventory/serviceaccounts/:uid
```

---

## Audit

Old:

```
/audit
/audit-logs
/reports
```

New:

```
/audit/logs
/audit/reports
```

---

# 16. Router Structure (Gin)

Routes should be grouped by domain.

Example:

```go
func SetupRoutes(router *gin.Engine) {

    api := router.Group("/api/v1")

    registerInventoryRoutes(api)
    registerRuntimeRoutes(api)
    registerRiskRoutes(api)
    registerGraphRoutes(api)
    registerAuditRoutes(api)

}
```

---

# 17. Inventory Routes

```go
func registerInventoryRoutes(api *gin.RouterGroup) {

    inv := api.Group("/inventory")

    pods := inv.Group("/pods")
    pods.GET("", GetPods)
    pods.GET("/:uid", GetPod)

    pods.GET("/:uid/spec", GetPodSpec)
    pods.GET("/:uid/capabilities", GetCapabilities)
    pods.GET("/:uid/sbom", GetSBOM)

}
```

---

# 18. Runtime Routes

```go
func registerRuntimeRoutes(api *gin.RouterGroup) {

    runtime := api.Group("/runtime")

    runtime.GET("/events", GetRuntimeEvents)

    pods := runtime.Group("/pods")

    pods.GET("/:uid/processes", GetPodProcesses)
    pods.GET("/:uid/network", GetPodNetwork)
    pods.GET("/:uid/events", GetPodEvents)
    pods.GET("/:uid/metrics", GetPodRuntimeMetrics)

}
```

---

# 19. Risk Routes

```go
func registerRiskRoutes(api *gin.RouterGroup) {

    risk := api.Group("/risk")

    risk.GET("/scores", GetRiskScores)
    risk.GET("/scores/:uid", GetRiskScore)

    risk.GET("/analytics/trends", GetRiskTrends)
    risk.GET("/analytics/comparison", GetRiskComparison)

    pods := risk.Group("/pods")
    pods.GET("/:uid/report", GetPodRiskReport)

}
```

---

# 20. Graph Routes

```go
func registerGraphRoutes(api *gin.RouterGroup) {

    graph := api.Group("/graph")

    graph.GET("", GetGraph)
    graph.GET("/attack-paths/:uid", GetAttackPaths)
    graph.GET("/blast-radius/:uid", GetBlastRadius)

}
```

---

# 21. Security

Dashboard authentication:

```
JWT token
```

Header:

```
Authorization: Bearer <token>
```

Agent communication:

```
gRPC + mTLS
```

---

# 22. Deprecation Policy

Deprecated routes must return headers.

```
Deprecation: true
Sunset: 2026-06-01
```

After sunset date, routes must be removed.

---

# 23. Benefits

Adopting this architecture will:

* eliminate route duplication
* prevent router conflicts
* simplify frontend API usage
* enable horizontal scaling
* support multi-cluster expansion
* improve maintainability

---

# 24. Future Extensions

Planned API domains:

```
policy
compliance
supply-chain
```

Examples:

```
/policy/rules
/compliance/benchmarks
/supply-chain/images
```
