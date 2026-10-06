package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/authorization"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/risk"
)

// maxLinkedFindings bounds how many linked findings one case response resolves.
const maxLinkedFindings = 200

// investigationLinkedFindingDTO is a finding linked to a case, read live from the findings table so the
// case shows the finding's current status rather than the snapshot taken when it was linked.
type investigationLinkedFindingDTO struct {
	InsightID         string   `json:"insightId"`
	Title             string   `json:"title"`
	Status            string   `json:"status"`
	SeverityHint      string   `json:"severityHint"`
	FinalLevel        string   `json:"finalLevel,omitempty"`
	FinalScore        *float64 `json:"finalScore,omitempty"`
	ClusterID         string   `json:"clusterId,omitempty"`
	ResourceType      string   `json:"resourceType"`
	ResourceNamespace string   `json:"resourceNamespace,omitempty"`
	ResourceName      string   `json:"resourceName"`
	ResourceUID       string   `json:"resourceUid,omitempty"`
	LinkedAt          string   `json:"linkedAt"`
	// Missing is set when the finding no longer exists; the case keeps the link and its snapshot.
	Missing bool `json:"missing,omitempty"`
}

// linkedInsightID returns the finding a case entity points at, from its metadata or its link.
func linkedInsightID(e investigationEntityDTO) string {
	if !strings.EqualFold(strings.TrimSpace(e.Type), "finding") {
		return ""
	}
	if id := strings.TrimSpace(e.Meta["insightId"]); id != "" {
		return id
	}
	h := strings.TrimSpace(e.Href)
	if idx := strings.Index(h, "/risks/"); idx >= 0 {
		rest := h[idx+len("/risks/"):]
		if cut := strings.IndexAny(rest, "?#/"); cut >= 0 {
			rest = rest[:cut]
		}
		return strings.TrimSpace(rest)
	}
	return ""
}

// validInsightID reports whether id is a finding primary key, so it is safe to use in a LIKE pattern.
func validInsightID(id string) bool {
	if id == "" {
		return false
	}
	_, err := strconv.ParseUint(id, 10, 64)
	return err == nil
}

// insightInRequestScope is requireInsightClusterScope without writing a response.
func insightInRequestScope(db *gorm.DB, c *gin.Context, insight models.Insight) (bool, error) {
	if _, restricted := middleware.ScopedClusterIDs(c); !restricted {
		return true, nil
	}
	clusterID := strings.TrimSpace(insight.ClusterID)
	if clusterID == "" {
		cid, err := resourceUIDClusterID(db, insight.ResourceUID)
		if err != nil {
			return false, err
		}
		if cid == "" {
			return false, nil
		}
		clusterID = cid
	}
	return middleware.ClusterAllowed(c, clusterID), nil
}

// ListInvestigationFindings returns the findings linked to a case with their current status and risk
// level. Findings outside the caller's cluster scope are counted in hidden, never returned.
// GET /investigations/:id/findings
func ListInvestigationFindings(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.Param("id"))
		var row models.InvestigationCase
		if err := db.First(&row, "id = ?", id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "case not found"})
			return
		}
		if !investigationCanAccessCase(c, &row) {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		if !authorization.HasPermission(middleware.GrantedPermissions(c), authorization.PermissionFindingsRead) {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "required_permission": string(authorization.PermissionFindingsRead)})
			return
		}

		type link struct {
			id       string
			linkedAt string
			label    string
		}
		links := make([]link, 0)
		seen := map[string]bool{}
		for _, e := range parseEntities(row.EntitiesJSON) {
			iid := linkedInsightID(e)
			if !validInsightID(iid) || seen[iid] {
				continue
			}
			seen[iid] = true
			links = append(links, link{id: iid, linkedAt: e.PinnedAt, label: e.Label})
		}
		truncated := false
		if len(links) > maxLinkedFindings {
			links = links[:maxLinkedFindings]
			truncated = true
		}
		ids := make([]string, 0, len(links))
		for _, l := range links {
			ids = append(ids, l.id)
		}
		byID := map[string]models.Insight{}
		if len(ids) > 0 {
			var rows []models.Insight
			if err := db.Where("id IN ?", ids).Find(&rows).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load linked findings"})
				return
			}
			for _, r := range rows {
				byID[strconv.FormatUint(uint64(r.ID), 10)] = r
			}
		}

		items := make([]investigationLinkedFindingDTO, 0, len(links))
		hidden := 0
		for _, l := range links {
			insight, ok := byID[l.id]
			if !ok {
				items = append(items, investigationLinkedFindingDTO{InsightID: l.id, Title: l.label, LinkedAt: l.linkedAt, Missing: true})
				continue
			}
			inScope, err := insightInRequestScope(db, c, insight)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "could not check cluster scope"})
				return
			}
			if !inScope {
				hidden++
				continue
			}
			dto := investigationLinkedFindingDTO{
				InsightID:         l.id,
				Title:             insight.Title,
				Status:            insight.Status,
				SeverityHint:      insight.Severity,
				ClusterID:         insight.ClusterID,
				ResourceType:      insight.ResourceType,
				ResourceNamespace: insight.ResourceNamespace,
				ResourceName:      insight.ResourceName,
				ResourceUID:       insight.ResourceUID,
				LinkedAt:          l.linkedAt,
			}
			if s := preferredScoreForResource(db, insight.ClusterID, insight.ResourceUID); s != nil {
				score := s.TotalScore
				dto.FinalScore = &score
				dto.FinalLevel = risk.DeriveFinalLevelFromScore(score)
			}
			items = append(items, dto)
		}
		c.JSON(http.StatusOK, gin.H{"items": items, "hidden": hidden, "truncated": truncated})
	}
}

// resolveFindingPin checks a finding pin against the caller and the case, and returns the entity key,
// label, link and metadata to store. It writes the error response and returns ok=false when the pin
// is not allowed: the caller must be able to read the finding, and a case bound to a cluster only
// takes findings from that cluster.
func resolveFindingPin(db *gorm.DB, c *gin.Context, row *models.InvestigationCase, req investigationPinRequest) (key, label, href string, meta map[string]string, ok bool) {
	meta = map[string]string{}
	for k, v := range req.Meta {
		meta[k] = v
	}
	iid := linkedInsightID(investigationEntityDTO{Type: "finding", Href: req.Href, Meta: meta})
	if !validInsightID(iid) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "finding id required"})
		return "", "", "", nil, false
	}
	if !authorization.HasPermission(middleware.GrantedPermissions(c), authorization.PermissionFindingsRead) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "required_permission": string(authorization.PermissionFindingsRead)})
		return "", "", "", nil, false
	}
	var insight models.Insight
	if err := db.First(&insight, "id = ?", iid).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "finding not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load finding"})
		}
		return "", "", "", nil, false
	}
	inScope, err := insightInRequestScope(db, c, insight)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not check cluster scope"})
		return "", "", "", nil, false
	}
	if !inScope {
		// Same answer as a missing finding, so a pin cannot probe other clusters.
		c.JSON(http.StatusNotFound, gin.H{"error": "finding not found"})
		return "", "", "", nil, false
	}
	if row.ClusterID != nil && strings.TrimSpace(*row.ClusterID) != "" &&
		strings.TrimSpace(insight.ClusterID) != "" && strings.TrimSpace(insight.ClusterID) != strings.TrimSpace(*row.ClusterID) {
		c.JSON(http.StatusConflict, gin.H{"error": "the finding is in a different cluster than the case"})
		return "", "", "", nil, false
	}
	meta["insightId"] = iid
	label = strings.TrimSpace(insight.Title)
	if label == "" {
		label = "Finding " + iid
	}
	return "finding:" + iid, label, "#/risks/" + iid, meta, true
}

// caseLinksFinding reports whether a case links the given finding.
func caseLinksFinding(row *models.InvestigationCase, insightID string) bool {
	for _, e := range parseEntities(row.EntitiesJSON) {
		if linkedInsightID(e) == insightID {
			return true
		}
	}
	return false
}
