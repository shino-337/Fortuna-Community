1. Vấn đề kiến trúc trong routes.go
1. Inconsistent identifiers

Ví dụ trong file:

/serviceaccounts/by-uid/:saUid
/serviceaccounts/:id

Pods:

/pods/by-id/:id
/pods/by-uid/:uid
/pods/:podUid/processes

Có 4 kiểu identifier:

id
uid
podUid
saUid

Điều này dẫn tới:

frontend phải biết logic riêng từng resource

router conflict

API contract khó maintain

2. RPC style route

Ví dụ:

/risk/scores/:uid/calculate

Đây là RPC pattern.

REST chuẩn nên là:

POST /pods/:uid/risk-score/recalculate
3. Resource không hierarchical

Ví dụ:

/runtime-risk/pods/:podUid

Nhưng đúng REST:

/pods/:uid/runtime-risk
4. Duplicate endpoints

Audit:

/audit
/audit-logs
/reports
/audit/reports

4 endpoint cùng chức năng.

5. Router order hack

Bạn có comment:

register BEFORE /pods/:id to avoid route conflict

Đây là dấu hiệu API design sai.

# Fortuna API Route Standard

Version: 1.0

This document defines the canonical API routing rules for Fortuna Core.

---

# 1. Base Path

All APIs must start with:

/api/v1

Example:

/api/v1/pods
/api/v1/serviceaccounts

---

# 2. Identifier Rules

Public APIs must NOT expose database IDs.

Use Kubernetes UID instead.

Correct:

/pods/:uid

Incorrect:

/pods/by-id/:id
/pods/:id

---

# 3. Resource Naming

Use plural nouns.

pods
nodes
deployments
serviceaccounts
clusters

---

# 4. Nested Resource Pattern

Subresources must follow:

/resource/:uid/subresource

Example:

/pods/:uid/processes
/pods/:uid/network
/pods/:uid/events

---

# 5. Parameter Naming

Standard parameters:

| Resource | Parameter |
|--------|--------|
| Pod | uid |
| ServiceAccount | uid |
| Deployment | uid |
| Node | name |

Never use:

:id
:podUid
:saUid

---

# 6. HTTP Methods

GET    retrieve
POST   create
PUT    replace
PATCH  partial update
DELETE delete

---

# 7. Pod API

List pods

GET /api/v1/pods

Pod detail

GET /api/v1/pods/:uid

---

# 8. Pod Subresources

Processes

GET /api/v1/pods/:uid/processes

Network

GET /api/v1/pods/:uid/network

Events

GET /api/v1/pods/:uid/events

Runtime metrics

GET /api/v1/pods/:uid/runtime-metrics

Capabilities

GET /api/v1/pods/:uid/capabilities

Spec

GET /api/v1/pods/:uid/spec

SBOM

GET /api/v1/pods/:uid/sbom

Risk report

GET /api/v1/pods/:uid/risk-report

---

# 9. ServiceAccount API

GET /api/v1/serviceaccounts
GET /api/v1/serviceaccounts/:uid
GET /api/v1/serviceaccounts/:uid/permissions

---

# 10. Risk API

GET /api/v1/risk/scores
GET /api/v1/pods/:uid/risk-score

---

# 11. Runtime Risk

GET /api/v1/pods/:uid/runtime-risk

---

# 12. Graph API

GET /api/v1/graph
GET /api/v1/graph/attack-paths/:uid
GET /api/v1/graph/blast-radius/:uid

---

# 13. Analytics API

GET /api/v1/analytics/risk/trends
GET /api/v1/analytics/risk/comparison

---

# 14. Audit

GET /api/v1/audit/logs
GET /api/v1/audit/reports

---

# 15. Deprecation

Legacy routes must return headers:

Deprecation: true
Sunset: 2026-06-01

---

# 16. Security

Dashboard APIs:

JWT

Agent ingestion:

gRPC + mTLS


Refactor mapping (Old → New)
Pods
Old	New
/pods/by-id/:id	remove
/pods/by-uid/:uid	/pods/:uid
/pods/:podUid/processes	/pods/:uid/processes
/pods/:podUid/network-connections	/pods/:uid/network
/pods/:podUid/events	/pods/:uid/events
/pods/:podUid/runtime-metrics	/pods/:uid/runtime-metrics
Runtime risk

Old:

/runtime-risk/pods/:podUid

New:

/pods/:uid/runtime-risk
SBOM

Old:

/sbom/:podUid

New:

/pods/:uid/sbom
Risk report

Old:

/risks/pods/:podUid/report

New:

/pods/:uid/risk-report
ServiceAccounts

Old:

/serviceaccounts/by-uid/:saUid

New:

/serviceaccounts/:uid
Audit

Old:

/audit
/audit-logs
/reports

New:

/audit/logs
/audit/reports
4. Router structure đề xuất

Refactor routes.go thành modules:

routes.go
routes_pods.go
routes_risk.go
routes_graph.go
routes_policy.go
routes_audit.go
5. New router layout
func SetupRoutes(router *gin.Engine, db *gorm.DB, cfg *config.Config) {

    api := router.Group("/api/v1")

    registerAuthRoutes(api)
    registerAgentRoutes(api)

    protected := api.Group("/")
    protected.Use(AuthMiddleware)

    registerPodRoutes(protected)
    registerRiskRoutes(protected)
    registerServiceAccountRoutes(protected)
    registerGraphRoutes(protected)
    registerPolicyRoutes(protected)
    registerAuditRoutes(protected)

}
6. Pod routes (refactored)
func registerPodRoutes(api *gin.RouterGroup) {

    pods := api.Group("/pods")

    pods.GET("", GetPods)
    pods.GET("/:uid", GetPodByUID)

    pods.GET("/:uid/processes", GetPodProcesses)
    pods.GET("/:uid/network", GetPodNetwork)
    pods.GET("/:uid/events", GetPodEvents)

    pods.GET("/:uid/runtime-metrics", GetPodRuntimeMetrics)

    pods.GET("/:uid/spec", GetPodSpec)
    pods.GET("/:uid/capabilities", GetPodCapabilities)

    pods.GET("/:uid/sbom", GetSBOM)
    pods.GET("/:uid/risk-report", GetPodRiskReport)

    pods.GET("/:uid/runtime-risk", GetPodRuntimeRisk)

}
7. Risk routes
func registerRiskRoutes(api *gin.RouterGroup) {

    risk := api.Group("/risk")

    risk.GET("/scores", GetRiskScores)
    risk.GET("/scores/:uid", GetRiskScore)

    risk.GET("/analytics/trends", GetRiskTrendsAnalytics)
    risk.GET("/analytics/comparison", GetRiskComparison)
    risk.GET("/analytics/correlation", GetRiskCorrelation)

}
8. Agent ingest (future)

Bạn đang dùng HTTP:

/api/v1/agent/*

Nhưng kiến trúc tốt hơn là:

Agent → gRPC
Core → NATS
Dashboard → REST

Phù hợp với scale 50-200 nodes bạn đã nói.

9. Immediate benefits

Refactor này sẽ:

giảm ~40% route

loại bỏ Gin wildcard conflict

API contract rõ ràng

frontend API client đơn giản

dễ maintain khi thêm feature

10. Kiến nghị quan trọng (cho Fortuna)

Với kiến trúc bạn đang build (runtime security + attack path + sbom), API nên tổ chức thành 4 domain chính:

inventory
runtime
risk
graph

Ví dụ:

/inventory/pods
/runtime/events
/risk/scores
/graph/attack-paths