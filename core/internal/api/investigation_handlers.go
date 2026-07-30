package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/authorization"
	invpkg "github.com/fortuna/core/pkg/investigation"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/securityaudit"
)

const investigationRetentionYears = 7

type investigationEntitySnapshotDTO struct {
	CapturedAt string         `json:"capturedAt"`
	Entity     map[string]any   `json:"entity,omitempty"`
	Evidence   map[string]any   `json:"evidence,omitempty"`
	Score      map[string]any   `json:"score,omitempty"`
	Graph      map[string]any   `json:"graph,omitempty"`
}

type investigationEntityDTO struct {
	ID       string                          `json:"id"`
	Type     string                          `json:"type"`
	Label    string                          `json:"label"`
	Href     string                          `json:"href,omitempty"`
	Meta     map[string]string               `json:"meta,omitempty"`
	PinnedAt string                          `json:"pinnedAt"`
	Snapshot *investigationEntitySnapshotDTO `json:"snapshot,omitempty"`
}

type investigationHandoffNoteDTO struct {
	ID        string   `json:"id"`
	Body      string   `json:"body"`
	Author    string   `json:"author"`
	CreatedAt string   `json:"createdAt"`
	Mentions  []string `json:"mentions,omitempty"`
}

type investigationCollaborationDTO struct {
	Assignees    []string                      `json:"assignees"`
	Watchers     []string                      `json:"watchers"`
	TeamID       string                        `json:"teamId,omitempty"`
	HandoffNotes []investigationHandoffNoteDTO `json:"handoffNotes"`
}

type investigationNoteDTO struct {
	ID        string `json:"id"`
	Body      string `json:"body"`
	CreatedAt string `json:"createdAt"`
	Author    string `json:"author"`
}

type investigationRemediationDTO struct {
	ID                string `json:"id"`
	Title             string `json:"title"`
	Status            string `json:"status"`
	Owner             string `json:"owner"`
	DueAt             string `json:"dueAt,omitempty"`
	Notes             string `json:"notes,omitempty"`
	CreatedAt         string `json:"createdAt"`
	ExternalSystem    string `json:"externalSystem,omitempty"`
	ExternalTicketURL string `json:"externalTicketUrl,omitempty"`
	ExternalTicketKey string `json:"externalTicketKey,omitempty"`
}

type investigationTimelineDTO struct {
	ID            uint           `json:"id"`
	EventID       string         `json:"eventId"`
	CaseID        string         `json:"caseId"`
	EventType     string         `json:"eventType"`
	Summary       string         `json:"summary"`
	ActorUserID   uint           `json:"actorUserId"`
	ActorUsername string         `json:"actorUsername"`
	Before        map[string]any `json:"before,omitempty"`
	After         map[string]any `json:"after,omitempty"`
	Details       map[string]any `json:"details,omitempty"`
	CreatedAt     string         `json:"createdAt"`
}

type investigationPinRequest struct {
	Type  string            `json:"type"`
	Label string            `json:"label"`
	Href  string            `json:"href,omitempty"`
	Meta  map[string]string `json:"meta,omitempty"`
}

type investigationCaseDTO struct {
	ID                  string                        `json:"id"`
	Title               string                        `json:"title"`
	Status              string                        `json:"status"`
	Owner               string                        `json:"owner"`
	ClusterID           *string                       `json:"clusterId"`
	Notes               []investigationNoteDTO        `json:"notes"`
	Entities            []investigationEntityDTO      `json:"entities"`
	RemediationActions  []investigationRemediationDTO `json:"remediationActions"`
	Collaboration       investigationCollaborationDTO `json:"collaboration"`
	CreatedAt           string                        `json:"createdAt"`
	UpdatedAt           string                        `json:"updatedAt"`
	SLADueAt            *string                       `json:"slaDueAt,omitempty"`
	ArchivedAt          *string                       `json:"archivedAt,omitempty"`
	RetentionUntil      *string                       `json:"retentionUntil,omitempty"`
	CreatedByUserID     uint                          `json:"createdByUserId"`
}

type investigationCasePatch struct {
	Title              *string                         `json:"title"`
	Status             *string                         `json:"status"`
	Owner              *string                         `json:"owner"`
	ClusterID          *string                         `json:"clusterId"`
	SLADueAt           *string                         `json:"slaDueAt"`
	Entities           *[]investigationEntityDTO       `json:"entities"`
	Notes              *[]investigationNoteDTO         `json:"notes"`
	RemediationActions *[]investigationRemediationDTO  `json:"remediationActions"`
	Collaboration      *investigationCollaborationDTO  `json:"collaboration"`
}

type investigationCreateRequest struct {
	Title     string  `json:"title"`
	Owner     string  `json:"owner"`
	ClusterID *string `json:"clusterId"`
}

func investigationUser(c *gin.Context) (*models.User, bool) {
	raw, ok := c.Get("user")
	if !ok || raw == nil {
		return nil, false
	}
	u, ok := raw.(*models.User)
	return u, ok && u != nil
}

func investigationIsAdmin(c *gin.Context) bool {
	u, ok := investigationUser(c)
	if !ok {
		return false
	}
	return strings.EqualFold(authorization.NormalizeRole(u.Role), models.RoleAdmin)
}

func investigationClusterInScope(c *gin.Context, clusterID *string) bool {
	if clusterID == nil || strings.TrimSpace(*clusterID) == "" {
		return true
	}
	u, ok := investigationUser(c)
	if !ok {
		return false
	}
	if investigationIsAdmin(c) {
		return true
	}
	doc := authorization.ParseScopeDocument(u.ScopeJSON)
	if !doc.RestrictsClusters() {
		return true
	}
	return doc.ClusterAllowed(strings.TrimSpace(*clusterID))
}

func investigationCanAccessCase(c *gin.Context, row *models.InvestigationCase) bool {
	if row == nil {
		return false
	}
	if investigationIsAdmin(c) {
		return true
	}
	u, ok := investigationUser(c)
	if !ok {
		return false
	}
	if row.CreatedByUserID == u.ID {
		return investigationClusterInScope(c, row.ClusterID)
	}
	if strings.EqualFold(strings.TrimSpace(row.Owner), strings.TrimSpace(u.Username)) {
		return investigationClusterInScope(c, row.ClusterID)
	}
	return false
}

func investigationCanMutateCase(c *gin.Context, row *models.InvestigationCase) bool {
	if !investigationCanAccessCase(c, row) {
		return false
	}
	if investigationIsAdmin(c) {
		return true
	}
	u, ok := investigationUser(c)
	if !ok {
		return false
	}
	return row.CreatedByUserID == u.ID || strings.EqualFold(strings.TrimSpace(row.Owner), strings.TrimSpace(u.Username))
}

func marshalJSONSlice(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func parseEntities(raw string) []investigationEntityDTO {
	if strings.TrimSpace(raw) == "" {
		return []investigationEntityDTO{}
	}
	var out []investigationEntityDTO
	_ = json.Unmarshal([]byte(raw), &out)
	if out == nil {
		return []investigationEntityDTO{}
	}
	return out
}

func parseNotes(raw string) []investigationNoteDTO {
	if strings.TrimSpace(raw) == "" {
		return []investigationNoteDTO{}
	}
	var out []investigationNoteDTO
	_ = json.Unmarshal([]byte(raw), &out)
	if out == nil {
		return []investigationNoteDTO{}
	}
	return out
}

func parseRemediation(raw string) []investigationRemediationDTO {
	if strings.TrimSpace(raw) == "" {
		return []investigationRemediationDTO{}
	}
	var out []investigationRemediationDTO
	_ = json.Unmarshal([]byte(raw), &out)
	if out == nil {
		return []investigationRemediationDTO{}
	}
	return out
}

func defaultCollaboration() investigationCollaborationDTO {
	return investigationCollaborationDTO{
		Assignees:    []string{},
		Watchers:     []string{},
		HandoffNotes: []investigationHandoffNoteDTO{},
	}
}

func parseCollaboration(raw string) investigationCollaborationDTO {
	if strings.TrimSpace(raw) == "" {
		return defaultCollaboration()
	}
	var out investigationCollaborationDTO
	_ = json.Unmarshal([]byte(raw), &out)
	if out.Assignees == nil {
		out.Assignees = []string{}
	}
	if out.Watchers == nil {
		out.Watchers = []string{}
	}
	if out.HandoffNotes == nil {
		out.HandoffNotes = []investigationHandoffNoteDTO{}
	}
	return out
}

func snapshotToDTO(s invpkg.EntitySnapshot) *investigationEntitySnapshotDTO {
	return &investigationEntitySnapshotDTO{
		CapturedAt: s.CapturedAt,
		Entity:     s.Entity,
		Evidence:   s.Evidence,
		Score:      s.Score,
		Graph:      s.Graph,
	}
}

func toInvestigationDTO(row models.InvestigationCase) investigationCaseDTO {
	var sla *string
	if row.SLADueAt != nil {
		s := row.SLADueAt.UTC().Format(time.RFC3339)
		sla = &s
	}
	var archived, retention *string
	if row.ArchivedAt != nil {
		s := row.ArchivedAt.UTC().Format(time.RFC3339)
		archived = &s
	}
	if row.RetentionUntil != nil {
		s := row.RetentionUntil.UTC().Format(time.RFC3339)
		retention = &s
	}
	return investigationCaseDTO{
		ID:                 row.ID,
		Title:              row.Title,
		Status:             invpkg.NormalizeStatus(row.Status),
		Owner:              row.Owner,
		ClusterID:          row.ClusterID,
		Notes:              parseNotes(row.NotesJSON),
		Entities:           parseEntities(row.EntitiesJSON),
		RemediationActions: parseRemediation(row.RemediationJSON),
		Collaboration:      parseCollaboration(row.CollaborationJSON),
		CreatedAt:          row.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:          row.UpdatedAt.UTC().Format(time.RFC3339),
		SLADueAt:           sla,
		ArchivedAt:         archived,
		RetentionUntil:     retention,
		CreatedByUserID:    row.CreatedByUserID,
	}
}

func logInvestigationTimeline(db *gorm.DB, c *gin.Context, caseID, eventType, summary string, before, after, details any) {
	invpkg.AppendTimeline(db, c, caseID, eventType, summary, before, after, details)
}

func logInvestigationAudit(db *gorm.DB, c *gin.Context, action, caseID, result string, details any) {
	sev := securityaudit.ClassifyResultSeverity(action, result)
	ev := securityaudit.FromRequest(
		c,
		authorization.ToStrings(middleware.GrantedPermissions(c)),
		c.GetString(middleware.CtxJWTSessionID),
		action,
		"investigation_case",
		caseID,
		result,
		sev,
		"jwt",
		nil,
		nil,
		details,
		nil,
	)
	securityaudit.Append(db, &ev)
}

func ListInvestigationCases(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !db.Migrator().HasTable(&models.InvestigationCase{}) {
			c.JSON(http.StatusOK, gin.H{"items": []investigationCaseDTO{}, "total": 0})
			return
		}
		u, ok := investigationUser(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		var rows []models.InvestigationCase
		q := db.Model(&models.InvestigationCase{})
		if !investigationIsAdmin(c) {
			q = q.Where("created_by_user_id = ? OR LOWER(owner) = LOWER(?)", u.ID, u.Username)
		}
		if err := q.Order("updated_at DESC").Find(&rows).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		filtered := make([]investigationCaseDTO, 0, len(rows))
		for i := range rows {
			if investigationCanAccessCase(c, &rows[i]) {
				filtered = append(filtered, toInvestigationDTO(rows[i]))
			}
		}
		c.JSON(http.StatusOK, gin.H{"items": filtered, "total": len(filtered)})
	}
}

func GetInvestigationCaseStats(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !db.Migrator().HasTable(&models.InvestigationCase{}) {
			c.JSON(http.StatusOK, gin.H{"openCases": 0, "overdueRemediation": 0})
			return
		}
		u, ok := investigationUser(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		var rows []models.InvestigationCase
		q := db.Model(&models.InvestigationCase{})
		if !investigationIsAdmin(c) {
			q = q.Where("created_by_user_id = ? OR LOWER(owner) = LOWER(?)", u.ID, u.Username)
		}
		if err := q.Order("updated_at DESC").Find(&rows).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		now := time.Now().UTC()
		openCases := 0
		overdue := 0
		for i := range rows {
			if !investigationCanAccessCase(c, &rows[i]) {
				continue
			}
			dto := toInvestigationDTO(rows[i])
			if invpkg.IsTerminalOpen(dto.Status) {
				openCases++
			}
			for _, rem := range dto.RemediationActions {
				if rem.Status == "done" || rem.DueAt == "" {
					continue
				}
				if t, err := time.Parse(time.RFC3339, rem.DueAt); err == nil && t.Before(now) {
					overdue++
				}
			}
		}
		c.JSON(http.StatusOK, gin.H{"openCases": openCases, "overdueRemediation": overdue})
	}
}

func GetInvestigationCase(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.Param("id"))
		if id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "id required"})
			return
		}
		var row models.InvestigationCase
		if err := db.First(&row, "id = ?", id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "case not found"})
			return
		}
		if !investigationCanAccessCase(c, &row) {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		c.JSON(http.StatusOK, toInvestigationDTO(row))
	}
}

func CreateInvestigationCase(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		u, ok := investigationUser(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		var req investigationCreateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		title := strings.TrimSpace(req.Title)
		if title == "" {
			title = "Investigation " + time.Now().UTC().Format(time.RFC3339)
		}
		if req.ClusterID != nil && !investigationClusterInScope(c, req.ClusterID) {
			c.JSON(http.StatusForbidden, gin.H{"error": "cluster out of scope"})
			return
		}
		owner := strings.TrimSpace(req.Owner)
		if owner == "" {
			owner = u.Username
		}
		row := models.InvestigationCase{
			ID:                "case_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
			Title:             title,
			Status:            invpkg.StatusOpen,
			Owner:             owner,
			ClusterID:         req.ClusterID,
			CreatedByUserID:   u.ID,
			EntitiesJSON:      "[]",
			NotesJSON:         "[]",
			RemediationJSON:   "[]",
			CollaborationJSON: marshalJSONSlice(defaultCollaboration()),
		}
		if err := db.Create(&row).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		logInvestigationAudit(db, c, securityaudit.ActionInvestigationsCaseCreate, row.ID, "success", map[string]interface{}{
			"title": row.Title,
		})
		logInvestigationTimeline(db, c, row.ID, invpkg.EventCaseCreated, "Investigation case created", nil, map[string]any{
			"status": row.Status,
			"owner":  row.Owner,
		}, nil)
		c.JSON(http.StatusCreated, toInvestigationDTO(row))
	}
}

func timelineRowToDTO(row models.InvestigationActivityLog) investigationTimelineDTO {
	dto := investigationTimelineDTO{
		ID:            row.ID,
		EventID:       row.EventID,
		CaseID:        row.CaseID,
		EventType:     row.EventType,
		Summary:       row.Summary,
		ActorUserID:   row.ActorUserID,
		ActorUsername: row.ActorUsername,
		CreatedAt:     row.CreatedAt.UTC().Format(time.RFC3339),
	}
	_ = json.Unmarshal([]byte(row.BeforeJSON), &dto.Before)
	_ = json.Unmarshal([]byte(row.AfterJSON), &dto.After)
	_ = json.Unmarshal([]byte(row.DetailsJSON), &dto.Details)
	return dto
}

func emitPatchTimelineEvents(db *gorm.DB, c *gin.Context, caseID string, before, after investigationCaseDTO) {
	if before.Status != after.Status {
		logInvestigationTimeline(db, c, caseID, invpkg.EventStatusChanged,
			"Status changed to "+after.Status,
			map[string]any{"status": before.Status},
			map[string]any{"status": after.Status}, nil)
	}
	if before.Owner != after.Owner {
		logInvestigationTimeline(db, c, caseID, invpkg.EventAssignmentChanged,
			"Owner assigned to "+after.Owner,
			map[string]any{"owner": before.Owner},
			map[string]any{"owner": after.Owner}, nil)
	}
	bCollab, aCollab := before.Collaboration, after.Collaboration
	if !collaborationEqual(bCollab, aCollab) {
		logInvestigationTimeline(db, c, caseID, invpkg.EventCollaborationUpdated,
			"Collaboration updated",
			map[string]any{"collaboration": bCollab},
			map[string]any{"collaboration": aCollab}, nil)
		for _, n := range diffNewHandoffNotes(bCollab.HandoffNotes, aCollab.HandoffNotes) {
			logInvestigationTimeline(db, c, caseID, invpkg.EventHandoffNote,
				"Handoff note added",
				nil, map[string]any{"note": n}, map[string]any{"mentions": n.Mentions})
		}
	}
	diffEntitiesTimeline(db, c, caseID, before.Entities, after.Entities)
	diffRemediationTimeline(db, c, caseID, before.RemediationActions, after.RemediationActions)
	if len(after.Notes) > len(before.Notes) {
		for i := 0; i < len(after.Notes)-len(before.Notes); i++ {
			n := after.Notes[i]
			logInvestigationTimeline(db, c, caseID, invpkg.EventNoteAdded,
				"Note added", nil, map[string]any{"note": n}, nil)
		}
	}
}

func collaborationEqual(a, b investigationCollaborationDTO) bool {
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return string(ab) == string(bb)
}

func diffNewHandoffNotes(before, after []investigationHandoffNoteDTO) []investigationHandoffNoteDTO {
	seen := map[string]struct{}{}
	for _, n := range before {
		seen[n.ID] = struct{}{}
	}
	var added []investigationHandoffNoteDTO
	for _, n := range after {
		if _, ok := seen[n.ID]; !ok {
			added = append(added, n)
		}
	}
	return added
}

func diffEntitiesTimeline(db *gorm.DB, c *gin.Context, caseID string, before, after []investigationEntityDTO) {
	beforeIDs := map[string]investigationEntityDTO{}
	for _, e := range before {
		beforeIDs[e.ID] = e
	}
	afterIDs := map[string]investigationEntityDTO{}
	for _, e := range after {
		afterIDs[e.ID] = e
	}
	for id, e := range afterIDs {
		if _, ok := beforeIDs[id]; !ok {
			details := map[string]any{"entityId": id, "type": e.Type, "label": e.Label}
			if e.Snapshot != nil {
				details["snapshotCapturedAt"] = e.Snapshot.CapturedAt
			}
			logInvestigationTimeline(db, c, caseID, invpkg.EventEntityPinned,
				"Pinned "+e.Type+": "+e.Label, nil, map[string]any{"entity": e}, details)
		}
	}
	for id, e := range beforeIDs {
		if _, ok := afterIDs[id]; !ok {
			logInvestigationTimeline(db, c, caseID, invpkg.EventEntityUnpinned,
				"Unpinned "+e.Type+": "+e.Label, map[string]any{"entity": e}, nil, map[string]any{"entityId": id})
		}
	}
}

func diffRemediationTimeline(db *gorm.DB, c *gin.Context, caseID string, before, after []investigationRemediationDTO) {
	bMap := map[string]investigationRemediationDTO{}
	for _, r := range before {
		bMap[r.ID] = r
	}
	aMap := map[string]investigationRemediationDTO{}
	for _, r := range after {
		aMap[r.ID] = r
	}
	for id, r := range aMap {
		if prev, ok := bMap[id]; !ok {
			logInvestigationTimeline(db, c, caseID, invpkg.EventRemediationAdded,
				"Remediation added: "+r.Title, nil, map[string]any{"remediation": r}, nil)
		} else if remediationChanged(prev, r) {
			logInvestigationTimeline(db, c, caseID, invpkg.EventRemediationUpdated,
				"Remediation updated: "+r.Title,
				map[string]any{"remediation": prev},
				map[string]any{"remediation": r}, nil)
		}
	}
	for id, r := range bMap {
		if _, ok := aMap[id]; !ok {
			logInvestigationTimeline(db, c, caseID, invpkg.EventRemediationRemoved,
				"Remediation removed: "+r.Title, map[string]any{"remediation": r}, nil, nil)
		}
	}
}

func remediationChanged(a, b investigationRemediationDTO) bool {
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return string(ab) != string(bb)
}

func PatchInvestigationCase(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.Param("id"))
		var row models.InvestigationCase
		if err := db.First(&row, "id = ?", id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "case not found"})
			return
		}
		if !investigationCanMutateCase(c, &row) {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		beforeDTO := toInvestigationDTO(row)
		var patch investigationCasePatch
		if err := c.ShouldBindJSON(&patch); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if patch.Title != nil {
			row.Title = strings.TrimSpace(*patch.Title)
		}
		if patch.Status != nil {
			next := invpkg.NormalizeStatus(strings.TrimSpace(*patch.Status))
			if err := invpkg.ValidateTransition(beforeDTO.Status, next, investigationIsAdmin(c)); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			row.Status = next
		}
		if patch.Owner != nil {
			row.Owner = strings.TrimSpace(*patch.Owner)
		}
		if patch.ClusterID != nil {
			cid := strings.TrimSpace(*patch.ClusterID)
			if cid == "" {
				row.ClusterID = nil
			} else {
				if !investigationClusterInScope(c, &cid) {
					c.JSON(http.StatusForbidden, gin.H{"error": "cluster out of scope"})
					return
				}
				row.ClusterID = &cid
			}
		}
		if patch.SLADueAt != nil {
			s := strings.TrimSpace(*patch.SLADueAt)
			if s == "" {
				row.SLADueAt = nil
			} else {
				t, err := time.Parse(time.RFC3339, s)
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": "invalid slaDueAt"})
					return
				}
				utc := t.UTC()
				row.SLADueAt = &utc
			}
		}
		if patch.Entities != nil {
			row.EntitiesJSON = marshalJSONSlice(*patch.Entities)
		}
		if patch.Notes != nil {
			row.NotesJSON = marshalJSONSlice(*patch.Notes)
		}
		if patch.RemediationActions != nil {
			row.RemediationJSON = marshalJSONSlice(*patch.RemediationActions)
		}
		if patch.Collaboration != nil {
			row.CollaborationJSON = marshalJSONSlice(*patch.Collaboration)
		}
		if err := db.Save(&row).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		afterDTO := toInvestigationDTO(row)
		emitPatchTimelineEvents(db, c, row.ID, beforeDTO, afterDTO)
		logInvestigationAudit(db, c, securityaudit.ActionInvestigationsCaseUpdate, row.ID, "success", nil)
		c.JSON(http.StatusOK, afterDTO)
	}
}

func PinInvestigationEntity(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.Param("id"))
		var row models.InvestigationCase
		if err := db.First(&row, "id = ?", id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "case not found"})
			return
		}
		if !investigationCanMutateCase(c, &row) {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		var req investigationPinRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		entityType := strings.TrimSpace(req.Type)
		label := strings.TrimSpace(req.Label)
		if entityType == "" || label == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "type and label required"})
			return
		}
		meta := req.Meta
		if meta == nil {
			meta = map[string]string{}
		}
		entityKey := entityType + ":" + label
		entities := parseEntities(row.EntitiesJSON)
		for _, e := range entities {
			if e.ID == entityKey || (req.Href != "" && e.Href == req.Href) {
				c.JSON(http.StatusOK, gin.H{"pinned": false, "entity": e, "case": toInvestigationDTO(row)})
				return
			}
		}
		snap := invpkg.BuildEntitySnapshot(db, entityType, label, req.Href, meta)
		pinned := investigationEntityDTO{
			ID:       entityKey,
			Type:     entityType,
			Label:    label,
			Href:     req.Href,
			Meta:     meta,
			PinnedAt: time.Now().UTC().Format(time.RFC3339),
			Snapshot: snapshotToDTO(snap),
		}
		entities = append([]investigationEntityDTO{pinned}, entities...)
		row.EntitiesJSON = marshalJSONSlice(entities)
		if err := db.Save(&row).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		logInvestigationTimeline(db, c, row.ID, invpkg.EventEntityPinned,
			"Pinned "+entityType+": "+label, nil, map[string]any{"entity": pinned},
			map[string]any{"snapshotCapturedAt": snap.CapturedAt})
		logInvestigationAudit(db, c, securityaudit.ActionInvestigationsCaseUpdate, row.ID, "success", map[string]any{"pin": entityKey})
		c.JSON(http.StatusOK, gin.H{"pinned": true, "entity": pinned, "case": toInvestigationDTO(row)})
	}
}

func ListInvestigationTimeline(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.Param("id"))
		var row models.InvestigationCase
		if err := db.Unscoped().First(&row, "id = ?", id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "case not found"})
			return
		}
		if !investigationCanAccessCase(c, &row) {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		if !db.Migrator().HasTable(&models.InvestigationActivityLog{}) {
			c.JSON(http.StatusOK, gin.H{"items": []investigationTimelineDTO{}, "total": 0})
			return
		}
		var rows []models.InvestigationActivityLog
		if err := db.Where("case_id = ?", id).Order("created_at ASC, id ASC").Find(&rows).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		items := make([]investigationTimelineDTO, 0, len(rows))
		for i := range rows {
			items = append(items, timelineRowToDTO(rows[i]))
		}
		c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items)})
	}
}

func DeleteInvestigationCase(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.Param("id"))
		var row models.InvestigationCase
		if err := db.First(&row, "id = ?", id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "case not found"})
			return
		}
		if !investigationIsAdmin(c) && !investigationCanMutateCase(c, &row) {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		beforeStatus := invpkg.NormalizeStatus(row.Status)
		now := time.Now().UTC()
		retention := now.AddDate(investigationRetentionYears, 0, 0)
		row.Status = invpkg.StatusArchived
		row.ArchivedAt = &now
		row.RetentionUntil = &retention
		if err := db.Save(&row).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if err := db.Delete(&row).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		logInvestigationTimeline(db, c, id, invpkg.EventCaseArchived,
			"Investigation case archived (soft delete)",
			map[string]any{"status": beforeStatus},
			map[string]any{"status": invpkg.StatusArchived, "retentionUntil": retention.Format(time.RFC3339)},
			map[string]any{"retentionYears": investigationRetentionYears})
		logInvestigationAudit(db, c, securityaudit.ActionInvestigationsCaseDelete, id, "success", map[string]any{
			"archived":       true,
			"retentionUntil": retention.Format(time.RFC3339),
		})
		c.JSON(http.StatusOK, gin.H{"archived": true, "id": id, "retentionUntil": retention.Format(time.RFC3339)})
	}
}
