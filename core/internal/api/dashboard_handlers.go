package api

import (
	"encoding/csv"
	"encoding/json"
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/authorization"
	"github.com/fortuna/core/pkg/metrics"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/risk"
	"github.com/fortuna/core/pkg/securityaudit"
)

type DashboardStatsDTO struct {
	DataStatus       string `json:"dataStatus,omitempty"`
	TotalClusters    int64  `json:"totalClusters"`
	ActiveAgents     int64  `json:"activeAgents"`
	RunningPods      int64  `json:"runningPods"`
	TotalRisks       int64  `json:"totalRisks"`
	CriticalRisks    int64  `json:"criticalRisks"`
	Resolved24h      int64  `json:"resolved24h"`           // Insights resolved in last 24h
	AffectedPodCount int64  `json:"affectedPodCount"`      // Distinct pods with at least one active insight (Affected Workloads)
	ClusterName      string `json:"clusterName,omitempty"` // When clusterId filter is set: display name from K8s (via agent sync)
}

type ThreatVelocityPoint struct {
	Date          string `json:"date"`
	CriticalCount int64  `json:"critical"`
	HighCount     int64  `json:"high"`
	MediumCount   int64  `json:"medium"`
	LowCount      int64  `json:"low"`
}

// GetThreatVelocity returns daily counts of insights grouped by severity.
// Query param days: 1–30 (default 7). Query param clusterId: optional.
// Query param byType: "vulnerability" (default) = CVE + supply_chain_malware insights; "all" = every insight_type (RBAC, capability, etc.).
// Pod filter: only count Pod insights when pod exists (deleted_at IS NULL).