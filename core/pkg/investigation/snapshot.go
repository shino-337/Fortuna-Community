package investigation

import (
	"encoding/json"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// EntitySnapshot captures point-in-time evidence for a pinned entity.
type EntitySnapshot struct {
	CapturedAt string         `json:"capturedAt"`
	Entity     map[string]any `json:"entity,omitempty"`
	Evidence   map[string]any `json:"evidence,omitempty"`
	Score      map[string]any `json:"score,omitempty"`
	Graph      map[string]any `json:"graph,omitempty"`
}

// BuildEntitySnapshot loads best-effort snapshots for findings/pods; attack paths use supplied meta.
func BuildEntitySnapshot(db *gorm.DB, entityType, label, href string, meta map[string]string) EntitySnapshot {
	out := EntitySnapshot{
		CapturedAt: timeNowRFC3339(),
		Entity: map[string]any{
			"type":  entityType,
			"label": label,
			"href":  href,
			"meta":  meta,
		},
	}
	if db == nil {
		return out
	}
	switch strings.ToLower(strings.TrimSpace(entityType)) {
	case "finding":
		id := meta["insightId"]
		if id == "" {
			id = extractPathID(href, "/risks/")
		}
		if id != "" {
			var insight models.Insight
			if err := db.First(&insight, "id = ?", id).Error; err == nil {
				b, _ := json.Marshal(insight)
				_ = json.Unmarshal(b, &out.Entity)
				out.Score = map[string]any{
					"severity":            insight.Severity,
					"cvss":                insight.CVSS,
					"finalRiskConfidence": insight.FinalRiskConfidence,
					"status":              insight.Status,
					"degraded":            insight.Degraded,
				}
				if insight.Evidence != "" {
					var ev any
					_ = json.Unmarshal([]byte(insight.Evidence), &ev)
					out.Evidence = map[string]any{"raw": ev}
				}
			}
		}
	case "pod":
		uid := meta["uid"]
		if uid == "" {
			uid = extractPathID(href, "/pods/uid/")
		}
		if uid != "" && db.Migrator().HasTable(&models.Pod{}) {
			var pod models.Pod
			if err := db.Where("uid = ?", uid).First(&pod).Error; err == nil {
				b, _ := json.Marshal(pod)
				_ = json.Unmarshal(b, &out.Entity)
			}
		}
	case "attack_path":
		out.Graph = map[string]any{
			"headline": meta["headline"],
			"confidence": meta["confidence"],
			"maxStrength": meta["maxStrength"],
			"variants": meta["variants"],
		}
	}
	return out
}

func extractPathID(href, prefix string) string {
	h := strings.TrimSpace(href)
	if h == "" {
		return ""
	}
	if idx := strings.Index(h, prefix); idx >= 0 {
		rest := h[idx+len(prefix):]
		if cut := strings.IndexAny(rest, "?#"); cut >= 0 {
			rest = rest[:cut]
		}
		return strings.Trim(rest, "/")
	}
	return ""
}

func timeNowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}
