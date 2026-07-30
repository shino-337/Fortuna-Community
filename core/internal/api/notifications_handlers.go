package api

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// GetNotifications returns notifications from the notifications table (real data).
func GetNotifications(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !db.Migrator().HasTable("notifications") {
			c.JSON(http.StatusOK, gin.H{
				"notifications": []map[string]interface{}{},
				"total":         0,
				"unreadCount":   0,
			})
			return
		}
		synthesizeSecurityNotifications(db)
		limit := 50
		if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
			if v, err := strconv.Atoi(raw); err == nil && v > 0 {
				limit = v
			}
		}
		if limit > 100 {
			limit = 100
		}
		unreadOnly := strings.EqualFold(strings.TrimSpace(c.Query("unreadOnly")), "true")

		base := db.Model(&models.Notification{}).Where("deleted_at IS NULL")
		if unreadOnly {
			base = base.Where("read_at IS NULL")
		}

		var total int64
		if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		var unreadCount int64
		if err := db.Model(&models.Notification{}).Where("deleted_at IS NULL AND read_at IS NULL").Count(&unreadCount).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		var list []models.Notification
		if err := base.Order("created_at DESC").Limit(limit).Find(&list).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		podNames := podDisplayNamesByUID(db, notificationResourceUIDs(list))
		notifications := make([]map[string]interface{}, 0, len(list))
		for _, n := range list {
			ts := n.CreatedAt.Format(time.RFC3339)
			resourceName := trimOrFallback(n.ResourceName, podNames[n.ResourceUID])
			message := n.Message
			route := n.Route
			if resourceName != "" {
				message = humanizeNotificationMessage(message, n.ResourceUID, resourceName)
				route = humanizeNotificationRoute(route, n.ResourceUID, resourceName)
			}
			notifications = append(notifications, map[string]interface{}{
				"id":           n.ID,
				"title":        n.Title,
				"message":      message,
				"severity":     n.Severity,
				"timestamp":    ts,
				"source":       n.Source,
				"category":     n.Category,
				"route":        route,
				"clusterId":    n.ClusterID,
				"resourceUid":  n.ResourceUID,
				"resourceName": resourceName,
				"readAt":       n.ReadAt,
			})
		}
		c.JSON(http.StatusOK, gin.H{
			"notifications": notifications,
			"total":         total,
			"unreadCount":   unreadCount,
		})
	}
}

func synthesizeSecurityNotifications(db *gorm.DB) {
	if !db.Migrator().HasColumn(&models.Notification{}, "dedupe_key") {
		return
	}
	now := time.Now().UTC()
	for _, n := range derivedInsightNotifications(db, now) {
		createNotificationIfMissing(db, n)
	}
	for _, n := range derivedAttackPathNotifications(db, now) {
		createNotificationIfMissing(db, n)
	}
	for _, n := range derivedCVENotifications(db, now) {
		createNotificationIfMissing(db, n)
	}
	for _, n := range derivedMalwareNotifications(db, now) {
		createNotificationIfMissing(db, n)
	}
}

func notificationResourceUIDs(list []models.Notification) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(list))
	for _, n := range list {
		uid := strings.TrimSpace(n.ResourceUID)
		if uid == "" {
			continue
		}
		if _, ok := seen[uid]; ok {
			continue
		}
		seen[uid] = struct{}{}
		out = append(out, uid)
	}
	return out
}

func podDisplayNamesByUID(db *gorm.DB, uids []string) map[string]string {
	out := map[string]string{}
	if len(uids) == 0 || !db.Migrator().HasTable("pods") {
		return out
	}
	var pods []models.Pod
	if err := db.Where("deleted_at IS NULL AND uid IN ?", uids).Find(&pods).Error; err != nil {
		return out
	}
	for _, p := range pods {
		name := podDisplayName(p.Namespace, p.Name, p.UID)
		if name != "" {
			out[p.UID] = name
		}
	}
	return out
}

func attackPathPodUIDs(paths []models.AttackPath) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		uid := strings.TrimSpace(p.PodUID)
		if uid == "" {
			continue
		}
		if _, ok := seen[uid]; ok {
			continue
		}
		seen[uid] = struct{}{}
		out = append(out, uid)
	}
	return out
}

func cveMatchPodUIDs(matches []models.CVEMatch) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		uid := strings.TrimSpace(m.PodUID)
		if uid == "" {
			continue
		}
		if _, ok := seen[uid]; ok {
			continue
		}
		seen[uid] = struct{}{}
		out = append(out, uid)
	}
	return out
}

func malwareMatchPodUIDs(matches []models.MalwareMatch) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		uid := strings.TrimSpace(m.PodUID)
		if uid == "" {
			continue
		}
		if _, ok := seen[uid]; ok {
			continue
		}
		seen[uid] = struct{}{}
		out = append(out, uid)
	}
	return out
}

func podDisplayName(namespace, name, fallbackUID string) string {
	name = strings.TrimSpace(name)
	namespace = strings.TrimSpace(namespace)
	if name == "" {
		return strings.TrimSpace(fallbackUID)
	}
	if namespace == "" {
		return name
	}
	return namespace + "/" + name
}

func humanizeNotificationMessage(message, uid, resourceName string) string {
	uid = strings.TrimSpace(uid)
	resourceName = strings.TrimSpace(resourceName)
	if uid == "" || resourceName == "" {
		return message
	}
	return strings.ReplaceAll(message, uid, resourceName)
}

func humanizeNotificationRoute(route, uid, resourceName string) string {
	if strings.TrimSpace(route) == "" || strings.TrimSpace(uid) == "" || strings.TrimSpace(resourceName) == "" {
		return route
	}
	searchTerm := resourceSearchTerm(resourceName, uid)
	for _, candidate := range []string{uid, resourceName} {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		needle := "search=" + url.QueryEscape(candidate)
		if strings.Contains(route, needle) {
			return strings.Replace(route, needle, "search="+url.QueryEscape(searchTerm), 1)
		}
	}
	return route
}

func resourceSearchTerm(resourceName, fallbackUID string) string {
	resourceName = strings.TrimSpace(resourceName)
	if resourceName == "" {
		return strings.TrimSpace(fallbackUID)
	}
	if idx := strings.LastIndex(resourceName, "/"); idx >= 0 && idx < len(resourceName)-1 {
		return strings.TrimSpace(resourceName[idx+1:])
	}
	return resourceName
}

func createNotificationIfMissing(db *gorm.DB, n models.Notification) {
	if strings.TrimSpace(n.DedupeKey) == "" {
		return
	}
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now().UTC()
	}
	_ = db.Where("dedupe_key = ? AND deleted_at IS NULL", n.DedupeKey).FirstOrCreate(&n).Error
}

func derivedInsightNotifications(db *gorm.DB, now time.Time) []models.Notification {
	if !db.Migrator().HasTable("insights") {
		return nil
	}
	var insights []models.Insight
	if err := db.Where("deleted_at IS NULL AND status = ? AND lower(severity) IN ?", "active", []string{"critical", "high"}).
		Order("detected_at DESC").
		Limit(8).
		Find(&insights).Error; err != nil {
		return nil
	}
	out := make([]models.Notification, 0, len(insights))
	for _, i := range insights {
		resource := strings.TrimSpace(i.ResourceName)
		if resource == "" {
			resource = strings.TrimSpace(i.ResourceUID)
		}
		resourceDisplay := podDisplayName(i.ResourceNamespace, resource, i.ResourceUID)
		createdAt := i.DetectedAt
		if createdAt.IsZero() {
			createdAt = now
		}
		out = append(out, models.Notification{
			Title:        fmt.Sprintf("%s finding: %s", titleWord(i.Severity), compactTitle(i.Title, 72)),
			Message:      fmt.Sprintf("%s %s requires review.", i.ResourceType, resourceDisplay),
			Severity:     strings.ToLower(i.Severity),
			Source:       "risk-engine",
			Category:     "risk",
			Route:        "/risks/findings?search=" + url.QueryEscape(resourceSearchTerm(resourceDisplay, i.ResourceUID)),
			DedupeKey:    fmt.Sprintf("insight:%d", i.ID),
			ResourceUID:  i.ResourceUID,
			ResourceName: resourceDisplay,
			CreatedAt:    createdAt,
		})
	}
	return out
}

func derivedAttackPathNotifications(db *gorm.DB, now time.Time) []models.Notification {
	if !db.Migrator().HasTable("attack_paths") {
		return nil
	}
	var paths []models.AttackPath
	if err := db.Order("total_risk DESC, updated_at DESC").
		Limit(6).
		Find(&paths).Error; err != nil {
		return nil
	}
	podNames := podDisplayNamesByUID(db, attackPathPodUIDs(paths))
	out := make([]models.Notification, 0, len(paths))
	for _, p := range paths {
		if p.TotalRisk < 60 {
			continue
		}
		severity := "high"
		if p.TotalRisk >= 80 {
			severity = "critical"
		}
		createdAt := p.UpdatedAt
		if createdAt.IsZero() {
			createdAt = now
		}
		resourceName := trimOrFallback(podNames[p.PodUID], p.PodUID)
		out = append(out, models.Notification{
			Title:        fmt.Sprintf("%s attack path detected", titleWord(severity)),
			Message:      fmt.Sprintf("%s has path %s with risk %.0f across %d steps.", resourceName, p.PathID, p.TotalRisk, p.Length),
			Severity:     severity,
			Source:       "attack-path",
			Category:     "attack-path",
			Route:        "/attack-paths?path=" + url.QueryEscape(p.PathID),
			DedupeKey:    fmt.Sprintf("attack-path:%d", p.ID),
			ResourceUID:  p.PodUID,
			ResourceName: resourceName,
			CreatedAt:    createdAt,
		})
	}
	return out
}

func derivedCVENotifications(db *gorm.DB, now time.Time) []models.Notification {
	if !db.Migrator().HasTable("cve_matches") {
		return nil
	}
	var matches []models.CVEMatch
	if err := db.Where("deleted_at IS NULL AND lower(severity) IN ?", []string{"critical", "high"}).
		Order("matched_at DESC").
		Limit(8).
		Find(&matches).Error; err != nil {
		return nil
	}
	podNames := podDisplayNamesByUID(db, cveMatchPodUIDs(matches))
	out := make([]models.Notification, 0, len(matches))
	for _, m := range matches {
		createdAt := m.MatchedAt
		if createdAt.IsZero() {
			createdAt = now
		}
		resourceName := trimOrFallback(podNames[m.PodUID], m.PodUID)
		out = append(out, models.Notification{
			Title:        fmt.Sprintf("%s CVE matched: %s", titleWord(m.Severity), m.CVEID),
			Message:      fmt.Sprintf("%s@%s in pod %s.", m.PackageName, m.PackageVersion, trimOrFallback(resourceName, "unknown pod")),
			Severity:     strings.ToLower(m.Severity),
			Source:       "cve-matcher",
			Category:     "cve",
			Route:        "/resources?tab=Pod&search=" + url.QueryEscape(resourceSearchTerm(resourceName, m.PodUID)),
			DedupeKey:    fmt.Sprintf("cve-match:%d", m.ID),
			ResourceUID:  m.PodUID,
			ResourceName: resourceName,
			CreatedAt:    createdAt,
		})
	}
	return out
}

func derivedMalwareNotifications(db *gorm.DB, now time.Time) []models.Notification {
	if !db.Migrator().HasTable("malware_matches") {
		return nil
	}
	var matches []models.MalwareMatch
	if err := db.Order("matched_at DESC").
		Limit(8).
		Find(&matches).Error; err != nil {
		return nil
	}
	podNames := podDisplayNamesByUID(db, malwareMatchPodUIDs(matches))
	out := make([]models.Notification, 0, len(matches))
	for _, m := range matches {
		createdAt := m.MatchedAt
		if createdAt.IsZero() {
			createdAt = now
		}
		severity := "critical"
		if !strings.EqualFold(m.Reason, "malware") {
			severity = "high"
		}
		resourceName := trimOrFallback(podNames[m.PodUID], podDisplayName(m.Namespace, "", m.PodUID))
		out = append(out, models.Notification{
			Title:        fmt.Sprintf("%s package detected", titleWord(m.Reason)),
			Message:      fmt.Sprintf("%s@%s in pod %s.", m.PackageName, m.PackageVersion, trimOrFallback(resourceName, "unknown pod")),
			Severity:     severity,
			Source:       "malware-matcher",
			Category:     "malware",
			Route:        "/resources?tab=Pod&search=" + url.QueryEscape(resourceSearchTerm(resourceName, m.PodUID)),
			DedupeKey:    fmt.Sprintf("malware-match:%d", m.ID),
			ResourceUID:  m.PodUID,
			ResourceName: resourceName,
			CreatedAt:    createdAt,
		})
	}
	return out
}

func compactTitle(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	if max <= 1 {
		return s[:max]
	}
	return strings.TrimSpace(s[:max-1]) + "..."
}

func trimOrFallback(s, fallback string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return fallback
	}
	return s
}

func titleWord(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return "Info"
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// MarkNotificationRead marks one notification as read.
func MarkNotificationRead(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !db.Migrator().HasTable("notifications") {
			c.JSON(http.StatusNotFound, gin.H{"error": "notifications table not found"})
			return
		}
		id, err := strconv.Atoi(strings.TrimSpace(c.Param("id")))
		if err != nil || id <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid notification id"})
			return
		}
		now := time.Now().UTC()
		tx := db.Model(&models.Notification{}).
			Where("id = ? AND deleted_at IS NULL AND read_at IS NULL", id).
			Update("read_at", now)
		if tx.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": tx.Error.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"updated": tx.RowsAffected})
	}
}

// MarkAllNotificationsRead marks all current notifications as read.
func MarkAllNotificationsRead(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !db.Migrator().HasTable("notifications") {
			c.JSON(http.StatusOK, gin.H{"updated": 0})
			return
		}
		now := time.Now().UTC()
		tx := db.Model(&models.Notification{}).
			Where("deleted_at IS NULL AND read_at IS NULL").
			Update("read_at", now)
		if tx.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": tx.Error.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"updated": tx.RowsAffected})
	}
}
