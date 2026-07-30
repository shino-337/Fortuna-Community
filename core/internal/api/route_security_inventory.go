package api

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/fortuna/core/pkg/authorization"
)

// Audit sensitivity classification for governance reports (required on every inventoried route).
const (
	auditNonSensitiveRead = "non_sensitive_read"
	auditSensitiveRead    = "sensitive_read"
	auditWrite            = "write"
	auditBulk             = "bulk"
	auditExport           = "export"
	auditIngestToken      = "ingest_token"
	auditWebSocket        = "websocket_stream"
)

const (
	authPublic     = "public"
	authJWT        = "jwt"
	authIngest     = "ingest_token"
	graphNone      = "none"
	graphSummary   = "summary"
	graphPaths     = "paths"
	graphTraversal = "traversal"
	graphEntity    = "entity"
	graphCypher    = "cypher"
)

// RouteSecuritySpec is the governance contract for a single HTTP route (method + full path).
// Used for startup verification and permission coverage reporting.
type RouteSecuritySpec struct {
	Method            string                   `json:"method"`
	Path              string                   `json:"path"`
	AuthMode          string                   `json:"authMode"`
	Optional          bool                     `json:"optional"`
	IsWrite           bool                     `json:"isWrite"`
	IsBulk            bool                     `json:"isBulk"`
	IsExport          bool                     `json:"isExport"`
	IsWebSocket       bool                     `json:"isWebSocket"`
	GraphClass        string                   `json:"graphClass"`
	AuditSensitivity  string                   `json:"auditSensitivity"`
	PrimaryPermission authorization.Permission `json:"primaryPermission,omitempty"`
}

// RouteVerifyOptions toggles optional routes that exist only when subsystems are wired.
type RouteVerifyOptions struct {
	CertRoutesRegistered    bool
	CVEMatchRouteRegistered bool
}

type routeB struct {
	method, path, auth, audit, graph string
	optional                         bool
	write, bulk, export, ws          bool
	perm                             authorization.Permission
}

func (b routeB) spec() RouteSecuritySpec {
	if b.audit == "" {
		panic("route security: missing audit class for " + b.method + " " + b.path)
	}
	if b.write && (b.audit == auditNonSensitiveRead) {
		panic("route security: write with non_sensitive_read audit: " + b.method + " " + b.path)
	}
	if b.bulk && b.audit != auditBulk {
		panic("route security: bulk route must use auditBulk: " + b.method + " " + b.path)
	}
	if b.export && b.audit != auditExport {
		panic("route security: export route must use auditExport: " + b.method + " " + b.path)
	}
	if b.ws && b.audit != auditWebSocket {
		panic("route security: ws route must use auditWebSocket: " + b.method + " " + b.path)
	}
	if strings.HasPrefix(b.path, "/api/v1/graph") || strings.Contains(b.path, "/graph/") {
		if b.graph == "" || b.graph == graphNone {
			panic("route security: graph route missing graph class: " + b.method + " " + b.path)
		}
	}
	return RouteSecuritySpec{
		Method:            b.method,
		Path:              b.path,
		AuthMode:          b.auth,
		Optional:          b.optional,
		IsWrite:           b.write,
		IsBulk:            b.bulk,
		IsExport:          b.export,
		IsWebSocket:       b.ws,
		GraphClass:        b.graph,
		AuditSensitivity:  b.audit,
		PrimaryPermission: b.perm,
	}
}

// FortunaRouteSecurityInventory is the authoritative route↔authorization contract for Fortuna REST/WS APIs.
func FortunaRouteSecurityInventory(opts RouteVerifyOptions) []RouteSecuritySpec {
	var bb []routeB

	add := func(b routeB) { bb = append(bb, b) }

	// --- Public auth ---
	add(routeB{"POST", "/api/v1/auth/login", authPublic, auditNonSensitiveRead, graphNone, false, false, false, false, false, ""})
	add(routeB{"POST", "/api/v1/auth/register", authPublic, auditWrite, graphNone, false, true, false, false, false, authorization.PermissionAuthRegister})

	// --- Ingest (token; not JWT RBAC) ---
	for _, p := range []string{
		"/api/v1/agent/sync",
		"/api/v1/agent/pod-runtime-metrics",
		"/api/v1/agent/pod-processes",
		"/api/v1/agent/pod-network-connections",
		"/api/v1/agent/pod-events",
	} {
		add(routeB{"POST", p, authIngest, auditIngestToken, graphNone, false, false, false, false, false, ""})
	}
	add(routeB{"POST", "/api/v1/runtime/events", authIngest, auditIngestToken, graphNone, false, false, false, false, false, ""})
	add(routeB{"POST", "/api/v2/runtime/events", authIngest, auditIngestToken, graphNone, false, false, false, false, false, ""})

	// --- JWT /api/v1 (alphabetical by path prefix groups) ---
	add(routeB{"GET", "/api/v1/agents/status", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionObservabilityAgentsRead})
	add(routeB{"GET", "/api/v1/capability-metadata", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionInventoryRead})
	add(routeB{"GET", "/api/v1/capability-metadata/:capabilityId", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionInventoryRead})
	add(routeB{"POST", "/api/v1/change-password", authJWT, auditWrite, graphNone, false, true, false, false, false, authorization.PermissionAuthPasswordChange})

	clusterCertOptional := !opts.CertRoutesRegistered
	add(routeB{"GET", "/api/v1/cluster/certificates/info", authJWT, auditSensitiveRead, graphNone, clusterCertOptional, false, false, false, false, authorization.PermissionInventoryRead})
	add(routeB{"POST", "/api/v1/cluster/certificates/rotate", authJWT, auditWrite, graphNone, clusterCertOptional, true, false, false, false, authorization.PermissionClusterCertificatesRotate})
	add(routeB{"GET", "/api/v1/cluster/certificates/rotation/history", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionInventoryRead})
	add(routeB{"GET", "/api/v1/cluster/info", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionInventoryRead})
	add(routeB{"GET", "/api/v1/cluster/:id/nodes", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionInventoryRead})

	add(routeB{"GET", "/api/v1/dashboard/metrics/threat-velocity", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionFindingsRead})
	add(routeB{"GET", "/api/v1/dashboard/stats", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionFindingsRead})
	add(routeB{"GET", "/api/v1/debug/technique-overlay", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionSystemDebug})
	add(routeB{"GET", "/api/v1/error-logs", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionObservabilityLogsRead})
	add(routeB{"GET", "/api/v1/governance/access-review", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionSystemAuditRead})
	add(routeB{"GET", "/api/v1/governance/correlation-signals", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionSystemAuditRead})
	add(routeB{"GET", "/api/v1/governance/emergency-access", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionSystemAuditRead})
	add(routeB{"GET", "/api/v1/governance/investigation-events", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionSystemAuditRead})
	add(routeB{"GET", "/api/v1/investigations", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionInvestigationsRead})
	add(routeB{"GET", "/api/v1/investigations/stats", authJWT, auditNonSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionInvestigationsRead})
	add(routeB{"POST", "/api/v1/investigations", authJWT, auditWrite, graphNone, true, false, false, false, false, authorization.PermissionInvestigationsWrite})
	add(routeB{"GET", "/api/v1/investigations/:id/timeline", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionInvestigationsRead})
	add(routeB{"GET", "/api/v1/investigations/:id", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionInvestigationsRead})
	add(routeB{"POST", "/api/v1/investigations/:id/pin", authJWT, auditWrite, graphNone, true, false, false, false, false, authorization.PermissionInvestigationsWrite})
	add(routeB{"PATCH", "/api/v1/investigations/:id", authJWT, auditWrite, graphNone, true, false, false, false, false, authorization.PermissionInvestigationsWrite})
	add(routeB{"DELETE", "/api/v1/investigations/:id", authJWT, auditWrite, graphNone, true, false, false, false, false, authorization.PermissionInvestigationsDelete})
	add(routeB{"GET", "/api/v1/governance/permission-explorer", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionSystemAuditRead})
	add(routeB{"GET", "/api/v1/governance/security-activity", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionSystemAuditRead})
	add(routeB{"GET", "/api/v1/health/dashboard-data-integrity", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionObservabilityMetricsRead})

	// Inventory
	add(routeB{"GET", "/api/v1/inventory/clusters", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionInventoryRead})
	add(routeB{"GET", "/api/v1/inventory/clusters/stats", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionInventoryRead})
	add(routeB{"GET", "/api/v1/inventory/clusters/:id", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionInventoryRead})
	add(routeB{"GET", "/api/v1/inventory/clusters/:id/agents", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionInventoryRead})
	add(routeB{"GET", "/api/v1/inventory/clusters/:id/inventory", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionInventoryRead})
	add(routeB{"GET", "/api/v1/inventory/clusters/:id/nodes/:nodeName", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionInventoryRead})
	add(routeB{"GET", "/api/v1/inventory/clusters/:id/overview", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionInventoryRead})
	add(routeB{"GET", "/api/v1/inventory/clusters/:id/security-summary", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionInventoryRead})
	add(routeB{"GET", "/api/v1/inventory/deployments", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionInventoryRead})
	add(routeB{"GET", "/api/v1/inventory/deployments/:id", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionInventoryRead})
	add(routeB{"GET", "/api/v1/inventory/pod-capabilities", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionInventoryRead})
	add(routeB{"GET", "/api/v1/inventory/pod-capabilities/summary", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionInventoryRead})
	add(routeB{"GET", "/api/v1/inventory/pod-capabilities/summary/capability", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionInventoryRead})
	add(routeB{"GET", "/api/v1/inventory/pod-capabilities/summary/cluster", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionInventoryRead})
	add(routeB{"GET", "/api/v1/inventory/pod-capabilities/summary/namespace", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionInventoryRead})
	add(routeB{"GET", "/api/v1/inventory/pod-capabilities/summary/severity", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionInventoryRead})
	add(routeB{"GET", "/api/v1/inventory/pod-capabilities/trends", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionInventoryRead})
	add(routeB{"GET", "/api/v1/inventory/pods", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionInventoryRead})
	add(routeB{"GET", "/api/v1/inventory/pods/:uid", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionInventoryRead})
	add(routeB{"GET", "/api/v1/inventory/pods/:uid/capabilities", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionInventoryRead})
	add(routeB{"GET", "/api/v1/inventory/pods/:uid/sbom", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionInventoryRead})
	add(routeB{"GET", "/api/v1/inventory/pods/:uid/spec", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionInventoryRead})
	add(routeB{"GET", "/api/v1/inventory/replicasets", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionInventoryRead})
	add(routeB{"GET", "/api/v1/inventory/replicasets/:id", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionInventoryRead})
	add(routeB{"GET", "/api/v1/inventory/sbom", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionInventoryRead})
	add(routeB{"GET", "/api/v1/inventory/serviceaccounts", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionInventoryRead})
	add(routeB{"GET", "/api/v1/inventory/serviceaccounts/:uid", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionInventoryRead})
	add(routeB{"GET", "/api/v1/inventory/serviceaccounts/:uid/permissions", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionInventoryRead})
	add(routeB{"PUT", "/api/v1/inventory/serviceaccounts/:uid", authJWT, auditWrite, graphNone, false, true, false, false, false, authorization.PermissionInventoryModify})
	add(routeB{"DELETE", "/api/v1/inventory/serviceaccounts/:uid", authJWT, auditWrite, graphNone, false, true, false, false, false, authorization.PermissionInventoryDelete})
	add(routeB{"POST", "/api/v1/inventory/serviceaccounts/bulk/delete", authJWT, auditBulk, graphNone, false, true, true, false, false, authorization.PermissionInventoryBulk})
	add(routeB{"POST", "/api/v1/inventory/serviceaccounts/bulk/disable", authJWT, auditBulk, graphNone, false, true, true, false, false, authorization.PermissionInventoryBulk})
	add(routeB{"POST", "/api/v1/bulk/serviceaccounts/disable", authJWT, auditBulk, graphNone, false, true, true, false, false, authorization.PermissionInventoryBulk})
	add(routeB{"DELETE", "/api/v1/bulk/serviceaccounts/delete", authJWT, auditBulk, graphNone, false, true, true, false, false, authorization.PermissionInventoryBulk})
	add(routeB{"POST", "/api/v1/inventory/serviceaccounts/disable-inactive", authJWT, auditBulk, graphNone, false, true, true, false, false, authorization.PermissionInventoryBulk})

	cveOpt := !opts.CVEMatchRouteRegistered
	add(routeB{"POST", "/api/v1/internal/trigger-cve-match", authJWT, auditWrite, graphNone, cveOpt, true, false, false, false, authorization.PermissionInternalCVETrigger})

	add(routeB{"GET", "/api/v1/malware/check", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionMalwareRead})
	add(routeB{"POST", "/api/v1/malware/db/upload", authJWT, auditWrite, graphNone, false, true, false, false, false, authorization.PermissionMalwareUpload})
	add(routeB{"GET", "/api/v1/malware/stats", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionMalwareRead})
	add(routeB{"GET", "/api/v1/malware/threats", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionMalwareRead})
	add(routeB{"GET", "/api/v1/malware/threats/:pod_uid", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionMalwareRead})
	add(routeB{"GET", "/api/v1/me", authJWT, auditNonSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionAuthSession})
	add(routeB{"GET", "/api/v1/metrics/policy-evaluation-cost", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionObservabilityMetricsRead})
	add(routeB{"GET", "/api/v1/metrics/system", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionObservabilityMetricsRead})
	add(routeB{"GET", "/api/v1/metrics/workers", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionObservabilityMetricsRead})
	add(routeB{"GET", "/api/v1/monitoring/agents", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionObservabilityAgentsRead})
	add(routeB{"GET", "/api/v1/monitoring/pipeline-health", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionObservabilityMetricsRead})
	add(routeB{"GET", "/api/v1/notifications", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionObservabilityMetricsRead})
	add(routeB{"PATCH", "/api/v1/notifications/:id/read", authJWT, auditWrite, graphNone, false, true, false, false, false, authorization.PermissionObservabilityMetricsRead})
	add(routeB{"POST", "/api/v1/notifications/read-all", authJWT, auditWrite, graphNone, false, true, false, false, false, authorization.PermissionObservabilityMetricsRead})

	// Policy engine
	add(routeB{"DELETE", "/api/v1/policy/instances/:instanceName", authJWT, auditWrite, graphNone, false, true, false, false, false, authorization.PermissionPoliciesDelete})
	add(routeB{"GET", "/api/v1/policy/instances", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionPoliciesRead})
	add(routeB{"GET", "/api/v1/policy/instances/:instanceName", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionPoliciesRead})
	add(routeB{"POST", "/api/v1/policy/instances", authJWT, auditWrite, graphNone, false, true, false, false, false, authorization.PermissionPoliciesDraft})
	add(routeB{"PUT", "/api/v1/policy/instances/:instanceName", authJWT, auditWrite, graphNone, false, true, false, false, false, authorization.PermissionPoliciesDraft})
	add(routeB{"DELETE", "/api/v1/policy/rules/uid/:uid", authJWT, auditWrite, graphNone, false, true, false, false, false, authorization.PermissionPoliciesDelete})
	add(routeB{"DELETE", "/api/v1/policy/rules/:id", authJWT, auditWrite, graphNone, false, true, false, false, false, authorization.PermissionPoliciesDelete})
	add(routeB{"GET", "/api/v1/policy/rules", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionPoliciesRead})
	add(routeB{"GET", "/api/v1/policy/rules/uid/:uid", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionPoliciesRead})
	add(routeB{"GET", "/api/v1/policy/rules/:id", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionPoliciesRead})
	add(routeB{"GET", "/api/v1/policy/rules/uid/:uid/matches", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionPoliciesRead})
	add(routeB{"GET", "/api/v1/policy/rules/:id/matches", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionPoliciesRead})
	add(routeB{"GET", "/api/v1/policy/rules/uid/:uid/metrics", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionPoliciesRead})
	add(routeB{"GET", "/api/v1/policy/rules/:id/metrics", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionPoliciesRead})
	add(routeB{"POST", "/api/v1/policy/rules", authJWT, auditWrite, graphNone, false, true, false, false, false, authorization.PermissionPoliciesDraft})
	add(routeB{"POST", "/api/v1/policy/rules/uid/:uid/test", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionPoliciesRead})
	add(routeB{"POST", "/api/v1/policy/rules/:id/test", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionPoliciesRead})
	add(routeB{"POST", "/api/v1/policy/rules/reload", authJWT, auditWrite, graphNone, false, true, false, false, false, authorization.PermissionPoliciesPublish})
	add(routeB{"PUT", "/api/v1/policy/rules/uid/:uid", authJWT, auditWrite, graphNone, false, true, false, false, false, authorization.PermissionPoliciesDraft})
	add(routeB{"PUT", "/api/v1/policy/rules/:id", authJWT, auditWrite, graphNone, false, true, false, false, false, authorization.PermissionPoliciesDraft})
	add(routeB{"DELETE", "/api/v1/policy/templates/:templateId/:version", authJWT, auditWrite, graphNone, false, true, false, false, false, authorization.PermissionPoliciesDelete})
	add(routeB{"GET", "/api/v1/policy/templates", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionPoliciesRead})
	add(routeB{"GET", "/api/v1/policy/templates/:templateId", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionPoliciesRead})
	add(routeB{"POST", "/api/v1/policy/templates", authJWT, auditWrite, graphNone, false, true, false, false, false, authorization.PermissionPoliciesDraft})
	add(routeB{"PUT", "/api/v1/policy/templates/:templateId/:version", authJWT, auditWrite, graphNone, false, true, false, false, false, authorization.PermissionPoliciesDraft})

	add(routeB{"GET", "/api/v1/promotion-rules", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionRulesRead})
	add(routeB{"GET", "/api/v1/promotion-rules/capability/:capabilityId", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionRulesRead})
	add(routeB{"GET", "/api/v1/promotion-rules/signal/:signalType", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionRulesRead})
	add(routeB{"GET", "/api/v1/rbac/permission-catalog", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionSystemAuditRead})
	add(routeB{"GET", "/api/v1/resources", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionInventoryRead})
	add(routeB{"GET", "/api/v1/resources/:kind/:uid", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionInventoryRead})

	// Risk + insights + rules
	riskPaths := []routeB{
		{"GET", "/api/v1/risk/analytics/comparison", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionFindingsRead},
		{"GET", "/api/v1/risk/analytics/correlation", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionFindingsRead},
		{"GET", "/api/v1/risk/analytics/runtime-cve", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionFindingsRead},
		{"GET", "/api/v1/risk/analytics/supply-chain", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionFindingsRead},
		{"GET", "/api/v1/risk/analytics/trends", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionFindingsRead},
		{"GET", "/api/v1/risk/attack-steps/summary", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionFindingsRead},
		{"DELETE", "/api/v1/risk/exceptions/:id", authJWT, auditWrite, graphNone, false, true, false, false, false, authorization.PermissionFindingsExceptionDelete},
		{"GET", "/api/v1/risk/exceptions", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionFindingsRead},
		{"POST", "/api/v1/risk/exceptions", authJWT, auditWrite, graphNone, false, true, false, false, false, authorization.PermissionFindingsExceptionCreate},
		{"GET", "/api/v1/risk/grouped", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionFindingsRead},
		{"GET", "/api/v1/risk/histogram", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionFindingsRead},
		{"DELETE", "/api/v1/risk/insights/:id", authJWT, auditWrite, graphNone, false, true, false, false, false, authorization.PermissionFindingsDelete},
		{"POST", "/api/v1/risk/insights/:id/acknowledge", authJWT, auditWrite, graphNone, false, true, false, false, false, authorization.PermissionFindingsAck},
		{"POST", "/api/v1/risk/insights/:id/dismiss", authJWT, auditWrite, graphNone, false, true, false, false, false, authorization.PermissionFindingsDismiss},
		{"POST", "/api/v1/risk/insights/:id/resolve", authJWT, auditWrite, graphNone, false, true, false, false, false, authorization.PermissionFindingsResolve},
		{"PATCH", "/api/v1/risk/insights/:id", authJWT, auditWrite, graphNone, false, true, false, false, false, authorization.PermissionFindingsAck},
		{"POST", "/api/v1/risk/insights/bulk", authJWT, auditBulk, graphNone, false, true, true, false, false, authorization.PermissionFindingsBulk},
		{"POST", "/api/v1/risk/insights/evaluate", authJWT, auditWrite, graphNone, false, true, false, false, false, authorization.PermissionRiskEvaluate},
		{"POST", "/api/v1/risk/insights/evaluate/historical", authJWT, auditWrite, graphNone, false, true, false, false, false, authorization.PermissionRiskEvaluate},
		{"GET", "/api/v1/risk/insights/export", authJWT, auditExport, graphNone, false, false, false, true, false, authorization.PermissionExportFindings},
		{"GET", "/api/v1/risk/insights", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionFindingsRead},
		{"GET", "/api/v1/risk/insights/:id", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionFindingsRead},
		{"GET", "/api/v1/risk/insights/:id/context", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionFindingsRead},
		{"GET", "/api/v1/risk/insights/summary", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionFindingsRead},
		{"GET", "/api/v1/risk/insights/summary/by-cluster", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionFindingsRead},
		{"GET", "/api/v1/risk/insights/summary/global", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionFindingsRead},
		{"GET", "/api/v1/risk/pods/:uid/attack-steps", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionFindingsRead},
		{"GET", "/api/v1/risk/pods/:uid/report", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionFindingsRead},
		{"GET", "/api/v1/risk/pods/:uid/runtime", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionRuntimeRead},
		{"GET", "/api/v1/risk/pods/:uid/runtime/events", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionRuntimeRead},
		{"GET", "/api/v1/risk/priorities", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionFindingsRead},
		{"DELETE", "/api/v1/risk/rules/:id", authJWT, auditWrite, graphNone, false, true, false, false, false, authorization.PermissionRulesDelete},
		{"GET", "/api/v1/risk/rules", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionRulesRead},
		{"GET", "/api/v1/risk/rules/:id", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionRulesRead},
		{"GET", "/api/v1/risk/rules/export", authJWT, auditExport, graphNone, false, false, false, true, false, authorization.PermissionRulesExport},
		{"POST", "/api/v1/risk/rules", authJWT, auditWrite, graphNone, false, true, false, false, false, authorization.PermissionRulesWrite},
		{"POST", "/api/v1/risk/rules/import", authJWT, auditWrite, graphNone, false, true, false, false, false, authorization.PermissionRulesImport},
		{"POST", "/api/v1/risk/rules/validate", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionRulesRead},
		{"PUT", "/api/v1/risk/rules/:id", authJWT, auditWrite, graphNone, false, true, false, false, false, authorization.PermissionRulesWrite},
		{"GET", "/api/v1/risk/runtime/summary", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionRuntimeRead},
		{"GET", "/api/v1/risk/runtime/top", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionRuntimeRead},
		{"GET", "/api/v1/risk/scores", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionFindingsRead},
		{"POST", "/api/v1/risk/scores/sync", authJWT, auditWrite, graphNone, false, true, false, false, false, authorization.PermissionRiskEvaluate},
		{"GET", "/api/v1/risk/scores/:uid", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionFindingsRead},
		{"POST", "/api/v1/risk/scores/:uid/calculate", authJWT, auditWrite, graphNone, false, true, false, false, false, authorization.PermissionRiskEvaluate},
		{"GET", "/api/v1/risk/top", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionFindingsRead},
		{"GET", "/api/v1/risk/trends", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionFindingsRead},
	}
	for _, x := range riskPaths {
		add(x)
	}

	// Runtime v1
	rtPaths := []routeB{
		{"GET", "/api/v1/runtime/network-activity", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionRuntimeRead},
		{"PATCH", "/api/v1/runtime/signal-step-mappings/:id/enabled", authJWT, auditWrite, graphNone, false, true, false, false, false, authorization.PermissionRuntimeMappingWrite},
		{"GET", "/api/v1/runtime/signal-step-mappings", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionRuntimeRead},
		{"POST", "/api/v1/runtime/signal-step-mappings", authJWT, auditWrite, graphNone, false, true, false, false, false, authorization.PermissionRuntimeMappingWrite},
		{"GET", "/api/v1/runtime/signals", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionRuntimeRead},
		{"GET", "/api/v1/runtime/signals/suppression-stats", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionRuntimeRead},
		{"GET", "/api/v1/runtime/pods/:uid/events", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionRuntimeRead},
		{"GET", "/api/v1/runtime/pods/:uid/metrics", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionRuntimeRead},
		{"GET", "/api/v1/runtime/pods/:uid/network", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionRuntimeRead},
		{"GET", "/api/v1/runtime/pods/:uid/network/top-destinations", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionRuntimeRead},
		{"GET", "/api/v1/runtime/pods/:uid/processes", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionRuntimeRead},
		{"GET", "/api/v1/runtime/pods/:uid/signals", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionRuntimeRead},
	}
	for _, x := range rtPaths {
		add(x)
	}

	// Graph
	add(routeB{"GET", "/api/v1/graph", authJWT, auditSensitiveRead, graphSummary, false, false, false, false, false, authorization.PermissionGraphReadSummary})
	add(routeB{"GET", "/api/v1/graph/accessible/:uid", authJWT, auditSensitiveRead, graphTraversal, false, false, false, false, false, authorization.PermissionGraphQueryTraversal})
	add(routeB{"GET", "/api/v1/graph/attack-paths/:uid", authJWT, auditSensitiveRead, graphPaths, false, false, false, false, false, authorization.PermissionGraphReadPaths})
	add(routeB{"GET", "/api/v1/graph/attack-paths/bundle", authJWT, auditSensitiveRead, graphPaths, false, false, false, false, false, authorization.PermissionGraphReadPaths})
	add(routeB{"GET", "/api/v1/graph/attack-paths/chains", authJWT, auditSensitiveRead, graphPaths, false, false, false, false, false, authorization.PermissionGraphReadPaths})
	add(routeB{"GET", "/api/v1/graph/attack-paths/graph", authJWT, auditSensitiveRead, graphPaths, false, false, false, false, false, authorization.PermissionGraphReadPaths})
	add(routeB{"GET", "/api/v1/graph/attack-paths/objectives", authJWT, auditSensitiveRead, graphPaths, false, false, false, false, false, authorization.PermissionGraphReadPaths})
	add(routeB{"GET", "/api/v1/graph/attack-paths/summary", authJWT, auditSensitiveRead, graphPaths, false, false, false, false, false, authorization.PermissionGraphReadPaths})
	add(routeB{"GET", "/api/v1/graph/blast-radius/:uid", authJWT, auditSensitiveRead, graphPaths, false, false, false, false, false, authorization.PermissionGraphReadPaths})
	add(routeB{"GET", "/api/v1/graph/permissions/:uid", authJWT, auditSensitiveRead, graphEntity, false, false, false, false, false, authorization.PermissionGraphQueryEntity})
	add(routeB{"POST", "/api/v1/graph/query", authJWT, auditSensitiveRead, graphCypher, false, false, false, false, false, authorization.PermissionGraphQueryAdvanced})
	add(routeB{"GET", "/api/v1/graph/risky-pods", authJWT, auditSensitiveRead, graphPaths, false, false, false, false, false, authorization.PermissionGraphReadPaths})
	add(routeB{"GET", "/api/v1/graph/shortest-path", authJWT, auditSensitiveRead, graphTraversal, false, false, false, false, false, authorization.PermissionGraphQueryTraversal})

	// Audit logs (legacy operational)
	add(routeB{"GET", "/api/v1/audit/logs", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionSystemAuditRead})
	add(routeB{"GET", "/api/v1/audit/reports", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionSystemAuditRead})

	// Sessions + users
	add(routeB{"DELETE", "/api/v1/sessions/:id", authJWT, auditWrite, graphNone, false, true, false, false, false, authorization.PermissionSessionsRevoke})
	add(routeB{"DELETE", "/api/v1/sessions/revoke-all", authJWT, auditWrite, graphNone, false, true, false, false, false, authorization.PermissionSessionsRevoke})
	add(routeB{"GET", "/api/v1/sessions", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionSessionsRead})
	add(routeB{"DELETE", "/api/v1/users/:id", authJWT, auditWrite, graphNone, false, true, false, false, false, authorization.PermissionUsersDelete})
	add(routeB{"GET", "/api/v1/users", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionUsersRead})
	add(routeB{"PATCH", "/api/v1/users/:id", authJWT, auditWrite, graphNone, false, true, false, false, false, authorization.PermissionUsersUpdate})

	// WebSockets
	add(routeB{"GET", "/api/v1/ws/pod/:uid", authJWT, auditWebSocket, graphNone, false, false, false, false, true, authorization.PermissionInventoryRead})
	add(routeB{"GET", "/api/v1/ws/risks", authJWT, auditWebSocket, graphNone, false, false, false, false, true, authorization.PermissionFindingsRead})

	// v2 runtime
	add(routeB{"GET", "/api/v2/runtime/pods/:uid/capabilities", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionInventoryRead})
	add(routeB{"GET", "/api/v2/runtime/pods/:uid/facts", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionRuntimeRead})
	add(routeB{"GET", "/api/v2/runtime/pods/:uid/incidents", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionRuntimeRead})
	add(routeB{"GET", "/api/v2/runtime/pods/:uid/security-state", authJWT, auditSensitiveRead, graphNone, false, false, false, false, false, authorization.PermissionRuntimeRead})

	out := make([]RouteSecuritySpec, 0, len(bb))
	seen := make(map[string]struct{})
	for _, b := range bb {
		k := b.method + " " + b.path
		if _, ok := seen[k]; ok {
			panic("duplicate route spec: " + k)
		}
		seen[k] = struct{}{}
		out = append(out, b.spec())
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Method < out[j].Method
	})
	return out
}

// VerifyFortunaRouteSecurityContract ensures Gin's registered /api/* routes match the governance inventory
// (bidirectional) and that sensitive routes declare audit classification.
func VerifyFortunaRouteSecurityContract(router *gin.Engine, opts RouteVerifyOptions) error {
	inv := FortunaRouteSecurityInventory(opts)
	want := make(map[string]RouteSecuritySpec)
	for _, s := range inv {
		if s.Optional {
			continue
		}
		want[s.Method+" "+s.Path] = s
	}

	got := make(map[string]struct{})
	for _, ri := range router.Routes() {
		p := ri.Path
		if !strings.HasPrefix(p, "/api/") {
			continue
		}
		key := ri.Method + " " + p
		got[key] = struct{}{}
		spec, ok := want[key]
		if !ok {
			// allow optional inventory entries not present in engine
			var foundOpt bool
			for _, s := range inv {
				if s.Optional && s.Method == ri.Method && s.Path == p {
					foundOpt = true
					break
				}
			}
			if foundOpt {
				continue
			}
			return fmt.Errorf("route not in security inventory (add spec or mark optional): %s %s", ri.Method, p)
		}
		if spec.AuthMode == authJWT {
			if spec.IsWrite && spec.PrimaryPermission == "" {
				return fmt.Errorf("jwt write route missing primary permission hint: %s", key)
			}
		}
	}

	for _, s := range inv {
		if s.Optional {
			continue
		}
		k := s.Method + " " + s.Path
		if _, ok := got[k]; !ok {
			return fmt.Errorf("inventory expects route not registered on this build: %s", k)
		}
	}
	return nil
}

// WriteRouteSecurityReport writes JSON inventory + coverage summary when FORTUNA_ROUTE_SECURITY_REPORT is set.
func WriteRouteSecurityReport(opts RouteVerifyOptions) error {
	path := strings.TrimSpace(os.Getenv("FORTUNA_ROUTE_SECURITY_REPORT"))
	if path == "" {
		return nil
	}
	inv := FortunaRouteSecurityInventory(opts)
	b, err := json.MarshalIndent(inv, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}
