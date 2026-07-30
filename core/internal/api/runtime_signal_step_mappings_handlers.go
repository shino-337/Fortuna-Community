package api

import (
	"crypto/sha256"
	"encoding/json"
	"encoding/hex"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"sync"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

var signalStepTokenPattern = regexp.MustCompile(`^[A-Z0-9_]+$`)

const (
	mappingWriteModeDisabled = "DISABLED"
	mappingWriteModeDryRun   = "DRY_RUN"
	mappingWriteModeEnabled  = "ENABLED"
)

var allowedSignalStepMappings = map[string][]string{
	"FILE_READ":   {"CRED_ACCESS"},
	"PROC_EXEC":   {"EXEC", "PRIV_ESC"},
	"NET_CONNECT": {"LATERAL_MOVE"},
	"DNS_QUERY":   {"RECON"},
}

var blastThresholdCache struct {
	sync.Mutex
	UpdatedAt time.Time
	Total24h  int64
}

type runtimeSignalStepMappingReq struct {
	SignalType    string     `json:"signalType"`
	StepID        string     `json:"stepId"`
	Enabled       *bool      `json:"enabled"`
	EffectiveFrom *time.Time `json:"effectiveFrom"`
	Force         bool       `json:"force"`
	Reason        string     `json:"reason"`
}

// GetRuntimeSignalStepMappings lists signal->step mappings (optional enabled filter).
func GetRuntimeSignalStepMappings(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		q := db.Model(&models.RuntimeSignalStepMapping{})
		if enabled := c.Query("enabled"); enabled != "" {
			if b, err := strconv.ParseBool(enabled); err == nil {
				q = q.Where("enabled = ?", b)
			}
		}
		now := time.Now()
		if activeOnly := c.Query("activeOnly"); activeOnly == "true" {
			q = q.Where("(effective_from IS NULL OR effective_from <= ?)", now)
		}
		var rows []models.RuntimeSignalStepMapping
		if err := q.Order("signal_type, step_id").Find(&rows).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"mappings": rows, "count": len(rows)})
	}
}

// UpsertRuntimeSignalStepMapping creates or updates one mapping row.
func UpsertRuntimeSignalStepMapping(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		mode := runtimeMappingWriteMode()
		if mode == mappingWriteModeDisabled {
			c.JSON(http.StatusForbidden, gin.H{"error": "runtime mapping writes are disabled"})
			return
		}

		var req runtimeSignalStepMappingReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		req.SignalType = strings.ToUpper(strings.TrimSpace(req.SignalType))
		req.StepID = strings.ToUpper(strings.TrimSpace(req.StepID))
		if req.SignalType == "" || req.StepID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "signalType and stepId are required"})
			return
		}
		if !signalStepTokenPattern.MatchString(req.SignalType) || !signalStepTokenPattern.MatchString(req.StepID) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "signalType and stepId must match ^[A-Z0-9_]+$"})
			return
		}
		if !isSemanticallyAllowed(req.SignalType, req.StepID) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "semantic validation failed for signalType -> stepId"})
			return
		}
		enabled := true
		if req.Enabled != nil {
			enabled = *req.Enabled
		}
		if exceedsSignalFanout(db, req.SignalType, req.StepID, 2) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "maxStepsPerSignal exceeded (2)"})
			return
		}
		if strictSignalConflictEnabled() {
			var conflict int64
			_ = db.Model(&models.RuntimeSignalStepMapping{}).
				Where("signal_type = ? AND step_id <> ?", req.SignalType, req.StepID).
				Count(&conflict).Error
			if conflict > 0 {
				c.JSON(http.StatusConflict, gin.H{"error": "strict mode conflict: signal_type already mapped to another step"})
				return
			}
		}
		force := req.Force || c.Query("force") == "true"
		if force && strings.TrimSpace(req.Reason) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "reason is required when force=true"})
			return
		}
		affected := affectedSignalsEstimate(db, req.SignalType)
		topNS := affectedTopNamespaces(db, req.SignalType, 3)
		threshold := dynamicBlastRadiusThreshold(db)
		if affected > threshold && !force {
			c.JSON(http.StatusConflict, gin.H{
				"error":            "blast radius too large; require force=true",
				"affectedEstimate": affected,
				"threshold":        threshold,
				"topNamespaces":    topNS,
			})
			return
		}

		var row models.RuntimeSignalStepMapping
		actor := actorUsername(c)
		err := db.Where("signal_type = ? AND step_id = ?", req.SignalType, req.StepID).First(&row).Error
		if err == nil {
			oldRow := row
			row.Enabled = enabled
			row.EffectiveFrom = normalizeEffectiveFrom(req.EffectiveFrom)
			row.UpdatedBy = actor
			if mode == mappingWriteModeDryRun {
				writeRuntimeMappingAudit(db, c, "DRY_RUN_UPDATE", oldRow, row, affected, req.Reason)
				c.JSON(http.StatusOK, gin.H{"mode": mappingWriteModeDryRun, "mapping": row, "affectedEstimate": affected, "topNamespaces": topNS})
				return
			}
			if err := db.Save(&row).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			writeRuntimeMappingAudit(db, c, "UPDATE", oldRow, row, affected, req.Reason)
			c.JSON(http.StatusOK, row)
			return
		}
		if err != nil && err != gorm.ErrRecordNotFound {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		row = models.RuntimeSignalStepMapping{
			SignalType:    req.SignalType,
			StepID:        req.StepID,
			Enabled:       enabled,
			EffectiveFrom: normalizeEffectiveFrom(req.EffectiveFrom),
			CreatedBy:     actor,
			UpdatedBy:     actor,
		}
		if mode == mappingWriteModeDryRun {
			writeRuntimeMappingAudit(db, c, "DRY_RUN_CREATE", models.RuntimeSignalStepMapping{}, row, affected, req.Reason)
			c.JSON(http.StatusOK, gin.H{"mode": mappingWriteModeDryRun, "mapping": row, "affectedEstimate": affected, "topNamespaces": topNS})
			return
		}
		if err := db.Create(&row).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		writeRuntimeMappingAudit(db, c, "CREATE", models.RuntimeSignalStepMapping{}, row, affected, req.Reason)
		c.JSON(http.StatusCreated, row)
	}
}

// SetRuntimeSignalStepMappingEnabled toggles enabled state for one mapping row by id.
func SetRuntimeSignalStepMappingEnabled(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		mode := runtimeMappingWriteMode()
		if mode == mappingWriteModeDisabled {
			c.JSON(http.StatusForbidden, gin.H{"error": "runtime mapping writes are disabled"})
			return
		}

		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil || id <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		var raw map[string]interface{}
		if err := c.ShouldBindJSON(&raw); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if _, ok := raw["signalType"]; ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "signalType is immutable in PATCH"})
			return
		}
		if _, ok := raw["stepId"]; ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "stepId is immutable in PATCH"})
			return
		}
		b, _ := json.Marshal(raw)
		var body struct {
			Enabled       *bool      `json:"enabled"`
			EffectiveFrom *time.Time `json:"effectiveFrom"`
			Reason        string     `json:"reason"`
		}
		if err := json.Unmarshal(b, &body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		var row models.RuntimeSignalStepMapping
		if err := db.First(&row, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "mapping not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		oldRow := row
		if body.Enabled != nil {
			row.Enabled = *body.Enabled
		}
		row.EffectiveFrom = normalizeEffectiveFrom(body.EffectiveFrom)
		row.UpdatedBy = actorUsername(c)
		force := c.Query("force") == "true"
		if force && strings.TrimSpace(body.Reason) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "reason is required when force=true"})
			return
		}
		affected := affectedSignalsEstimate(db, row.SignalType)
		topNS := affectedTopNamespaces(db, row.SignalType, 3)
		threshold := dynamicBlastRadiusThreshold(db)
		if affected > threshold && !force {
			c.JSON(http.StatusConflict, gin.H{
				"error":            "blast radius too large; require force=true",
				"affectedEstimate": affected,
				"threshold":        threshold,
				"topNamespaces":    topNS,
			})
			return
		}
		if mode == mappingWriteModeDryRun {
			writeRuntimeMappingAudit(db, c, "DRY_RUN_TOGGLE", oldRow, row, affected, body.Reason)
			c.JSON(http.StatusOK, gin.H{"mode": mappingWriteModeDryRun, "mapping": row, "affectedEstimate": affected, "topNamespaces": topNS})
			return
		}
		if err := db.Save(&row).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		writeRuntimeMappingAudit(db, c, "TOGGLE", oldRow, row, affected, body.Reason)
		c.JSON(http.StatusOK, row)
	}
}

func runtimeMappingWriteMode() string {
	mode := strings.ToUpper(strings.TrimSpace(os.Getenv("FORTUNA_RUNTIME_MAPPING_WRITE_MODE")))
	switch mode {
	case mappingWriteModeDisabled, mappingWriteModeDryRun, mappingWriteModeEnabled:
		return mode
	default:
		return mappingWriteModeEnabled
	}
}

func blastRadiusAbsCapFromEnv() int64 {
	raw := strings.TrimSpace(os.Getenv("FORTUNA_RUNTIME_MAPPING_BLAST_RADIUS_ABS_CAP"))
	if raw == "" {
		return 10000
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n <= 0 {
		return 10000
	}
	return n
}

func blastRadiusRatioFromEnv() float64 {
	raw := strings.TrimSpace(os.Getenv("FORTUNA_RUNTIME_MAPPING_BLAST_RADIUS_RATIO"))
	if raw == "" {
		return 0.2
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || v <= 0 {
		return 0.2
	}
	return v
}

func dynamicBlastRadiusThreshold(db *gorm.DB) int64 {
	absCap := blastRadiusAbsCapFromEnv()
	if db == nil || !db.Migrator().HasTable(&models.RuntimeSignal{}) {
		return maxInt64(50, absCap)
	}
	total := cachedTotalSignals24h(db)
	ratioCap := int64(blastRadiusRatioFromEnv() * float64(total))
	if ratioCap <= 0 {
		ratioCap = 1
	}
	threshold := absCap
	if ratioCap < absCap {
		threshold = ratioCap
	}
	return maxInt64(50, threshold)
}

func strictSignalConflictEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("FORTUNA_RUNTIME_MAPPING_STRICT_SIGNAL_UNIQUE")), "true")
}

func isSemanticallyAllowed(signalType, stepID string) bool {
	cands, ok := allowedSignalStepMappings[signalType]
	if !ok {
		return false
	}
	for _, c := range cands {
		if c == stepID {
			return true
		}
	}
	return false
}

func exceedsSignalFanout(db *gorm.DB, signalType, incomingStep string, maxSteps int64) bool {
	var rows []models.RuntimeSignalStepMapping
	if err := db.Where("signal_type = ?", signalType).Find(&rows).Error; err != nil {
		return false
	}
	steps := map[string]bool{incomingStep: true}
	for _, r := range rows {
		steps[strings.ToUpper(strings.TrimSpace(r.StepID))] = true
	}
	return int64(len(steps)) > maxSteps
}

func affectedSignalsEstimate(db *gorm.DB, signalType string) int64 {
	if db == nil || !db.Migrator().HasTable(&models.RuntimeSignal{}) {
		return 0
	}
	var n int64
	_ = db.Model(&models.RuntimeSignal{}).Where("signal_type = ?", signalType).Count(&n).Error
	return n
}

func affectedTopNamespaces(db *gorm.DB, signalType string, limit int) []string {
	if db == nil || !db.Migrator().HasTable(&models.RuntimeSignal{}) || !db.Migrator().HasTable(&models.Pod{}) {
		return nil
	}
	if limit <= 0 {
		limit = 3
	}
	type row struct {
		Namespace string
		Count     int64
	}
	var rows []row
	_ = db.Raw(`
		SELECT p.namespace AS namespace, COUNT(*) AS count
		FROM runtime_signals rs
		JOIN pods p ON p.uid = rs.pod_uid
		WHERE rs.signal_type = ? AND p.deleted_at IS NULL
		GROUP BY p.namespace
		ORDER BY count DESC
		LIMIT ?
	`, signalType, limit).Scan(&rows).Error
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		if strings.TrimSpace(r.Namespace) != "" {
			out = append(out, r.Namespace)
		}
	}
	return out
}

func actorUsername(c *gin.Context) string {
	if v, ok := c.Get("username"); ok {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			return s
		}
	}
	return "system"
}

func actorUserID(c *gin.Context) uint {
	if v, ok := c.Get("userID"); ok {
		if n, ok := v.(uint); ok {
			return n
		}
	}
	return 0
}

func writeRuntimeMappingAudit(db *gorm.DB, c *gin.Context, action string, oldRow, newRow models.RuntimeSignalStepMapping, affected int64, reason string) {
	if db == nil {
		return
	}
	detail := map[string]interface{}{
		"actor":           actorUsername(c),
		"action":          action,
		"signal_type":     newRow.SignalType,
		"step_id":         newRow.StepID,
		"old_value":       oldRow,
		"new_value":       newRow,
		"affected_estimate": affected,
		"timestamp":       time.Now().UTC().Format(time.RFC3339),
		"reason":          strings.TrimSpace(reason),
		"auth_source":     contextString(c, "auth_source", "unknown"),
		"scope_source":    contextString(c, "scope_source", "unknown"),
	}
	prevHash := lastRuntimeMappingAuditHash(db)
	detail["prev_hash"] = prevHash
	bNoHash, _ := json.Marshal(detail)
	h := sha256.Sum256(append([]byte(prevHash), bNoHash...))
	detail["hash"] = hex.EncodeToString(h[:])
	b, _ := json.Marshal(detail)
	entry := models.AuditLog{
		UserID:     actorUserID(c),
		Action:     strings.ToLower(action),
		Resource:   "runtime_signal_step_mapping",
		ResourceID: strconv.FormatUint(uint64(newRow.ID), 10),
		Details:    string(b),
		User:       actorUsername(c),
		IP:         c.ClientIP(),
	}
	if err := db.Create(&entry).Error; err == nil {
		maybeCreateAuditAnchor(db, detail["hash"].(string))
	}
}

func lastRuntimeMappingAuditHash(db *gorm.DB) string {
	var row models.AuditLog
	if err := db.Where("resource = ?", "runtime_signal_step_mapping").Order("id DESC").First(&row).Error; err != nil {
		return ""
	}
	if strings.TrimSpace(row.Details) == "" {
		return ""
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(row.Details), &m); err != nil {
		return ""
	}
	if v, ok := m["hash"].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func normalizeEffectiveFrom(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

func maybeCreateAuditAnchor(db *gorm.DB, chainHead string) {
	if db == nil || !db.Migrator().HasTable(&models.AuditAnchor{}) {
		return
	}
	everyN := int64(100)
	if raw := strings.TrimSpace(os.Getenv("FORTUNA_RUNTIME_MAPPING_AUDIT_ANCHOR_EVERY")); raw != "" {
		if n, err := strconv.ParseInt(raw, 10, 64); err == nil && n > 0 {
			everyN = n
		}
	}
	var count int64
	_ = db.Model(&models.AuditLog{}).Where("resource = ?", "runtime_signal_step_mapping").Count(&count).Error
	if count > 0 && count%everyN == 0 {
		_ = db.Create(&models.AuditAnchor{AnchorHash: chainHead}).Error
	}
}

func cachedTotalSignals24h(db *gorm.DB) int64 {
	blastThresholdCache.Lock()
	defer blastThresholdCache.Unlock()
	if time.Since(blastThresholdCache.UpdatedAt) < 2*time.Minute {
		return blastThresholdCache.Total24h
	}
	var total int64
	_ = db.Model(&models.RuntimeSignal{}).
		Where("created_at > ?", time.Now().Add(-24*time.Hour)).
		Count(&total).Error
	blastThresholdCache.Total24h = total
	blastThresholdCache.UpdatedAt = time.Now()
	return total
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// VerifyRuntimeMappingAuditChain recomputes the hash-chain and verifies it against
// the persisted head at/upto an optional anchor ID (0 means full chain).
func VerifyRuntimeMappingAuditChain(db *gorm.DB, anchorID uint) error {
	if db == nil {
		return nil
	}
	var logs []models.AuditLog
	q := db.Where("resource = ?", "runtime_signal_step_mapping").Order("id ASC")
	if anchorID > 0 && db.Migrator().HasTable(&models.AuditAnchor{}) {
		var anchor models.AuditAnchor
		if err := db.First(&anchor, anchorID).Error; err != nil {
			return err
		}
		_ = q.Find(&logs).Error
	} else {
		_ = q.Find(&logs).Error
	}
	prev := ""
	for _, row := range logs {
		if strings.TrimSpace(row.Details) == "" {
			continue
		}
		var d map[string]interface{}
		if err := json.Unmarshal([]byte(row.Details), &d); err != nil {
			return err
		}
		hash, _ := d["hash"].(string)
		prevHash, _ := d["prev_hash"].(string)
		if strings.TrimSpace(prevHash) != strings.TrimSpace(prev) {
			return gorm.ErrInvalidData
		}
		if len(strings.TrimSpace(hash)) < 32 {
			return gorm.ErrInvalidData
		}
		prev = hash
	}
	return nil
}

func contextString(c *gin.Context, key, fallback string) string {
	if v, ok := c.Get(key); ok {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			return s
		}
	}
	return fallback
}

