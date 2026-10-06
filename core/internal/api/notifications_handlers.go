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
	"gorm.io/gorm/clause"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/risk"
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
		offset := 0
		if v, err := strconv.Atoi(strings.TrimSpace(c.Query("offset"))); err == nil && v > 0 {
			offset = v
		}
		unreadOnly := strings.EqualFold(strings.TrimSpace(c.Query("unreadOnly")), "true")
		userID := notificationUserID(c)

		base := withReadState(scopedNotifications(db, c), userID)
		if unreadOnly {
			base = base.Where("nr.notification_id IS NULL")
		}

		var total int64
		if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		var unreadCount int64
		if err := withReadState(scopedNotifications(db, c), userID).Where("nr.notification_id IS NULL").Count(&unreadCount).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		var list []notificationWithReadState
		if err := base.Select("notifications.*, nr.read_at AS user_read_at").
			Order("notifications.created_at DESC, notifications.id DESC").Limit(limit).Offset(offset).Find(&list).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		rows := make([]models.Notification, 0, len(list))
		for _, n := range list {
			rows = append(rows, n.Notification)
		}
		podNames := podDisplayNamesByUID(db, notificationResourceUIDs(rows))
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
				"readAt":       n.UserReadAt,
			})
		}
		c.JSON(http.StatusOK, gin.H{
			"notifications": notifications,
			"total":         total,
			"unreadCount":   unreadCount,
		})
	}
}

// scopedNotifications limits notifications to the caller's cluster scope.
// Notifications name pods, findings and attack paths, so a user restricted to
// some clusters must neither read nor mark as read those of other clusters.
// Every notification is derived from a cluster resource, so a row without a
// cluster is hidden from restricted users rather than shown to all of them.
func scopedNotifications(db *gorm.DB, c *gin.Context) *gorm.DB {
	q := db.Model(&models.Notification{}).Where("notifications.deleted_at IS NULL")
	if ids, restricted := middleware.ScopedClusterIDs(c); restricted {
		if len(ids) == 0 {
			return q.Where("1 = 0")
		}
		q = q.Where("notifications.cluster_id IN ?", ids)
	}
	return q
}

// notificationWithReadState is a notification plus the caller's own read time.
type notificationWithReadState struct {
	models.Notification
	UserReadAt *time.Time
}

// withReadState joins the caller's read marks as "nr". Read state is per user,
// so a notification is unread for the caller while nr.notification_id is NULL.
func withReadState(q *gorm.DB, userID uint) *gorm.DB {
	return q.Joins("LEFT JOIN notification_reads nr ON nr.notification_id = notifications.id AND nr.user_id = ?", userID)
}

func notificationUserID(c *gin.Context) uint {
	if u, ok := investigationUser(c); ok {
		return u.ID
	}
	return actorUserID(c)
}

func synthesizeSecurityNotifications(db *gorm.DB) {
	if !db.Migrator().HasColumn(&models.Notification{}, "dedupe_key") {
		return
	}
	now := time.Now().UTC()
	// Finding alerts already stored follow the finding's current risk level first,
	// so an alert never says critical while the finding it opens says low.
	reconcileFindingNotifications(db)
	for _, n := range derivedInsightNotifications(db, now) {
		upsertDerivedNotification(db, n)
	}
	for _, n := range derivedAttackPathNotifications(db, now) {
		upsertDerivedNotification(db, n)
	}
	for _, n := range derivedCVENotifications(db, now) {
		upsertDerivedNotification(db, n)
	}
	for _, n := range derivedMalwareNotifications(db, now) {
		upsertDerivedNotification(db, n)
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

// upsertDerivedNotification stores a derived notification once per dedupe key.
// When the row already exists, its severity, text and link are refreshed so the
// bell always describes the source as it is now; read state is kept.
func upsertDerivedNotification(db *gorm.DB, n models.Notification) {
	if strings.TrimSpace(n.DedupeKey) == "" {
		return
	}
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now().UTC()
	}
	want := n
	if err := db.Where("dedupe_key = ? AND deleted_at IS NULL", n.DedupeKey).FirstOrCreate(&n).Error; err != nil {
		return
	}
	updates := map[string]interface{}{}
	if want.Severity != n.Severity {
		updates["severity"] = want.Severity
	}
	if want.Title != n.Title {
		updates["title"] = want.Title
	}
	if want.Message != n.Message {
		updates["message"] = want.Message
	}
	if want.Route != n.Route {
		updates["route"] = want.Route
	}
	// Rows written before notifications carried a cluster have none; stamp it
	// so cluster-scoped users can see them.
	if clusterID := strings.TrimSpace(want.ClusterID); clusterID != "" && strings.TrimSpace(n.ClusterID) == "" {
		updates["cluster_id"] = clusterID
	}
	if len(updates) > 0 {
		_ = db.Model(&models.Notification{}).Where("id = ?", n.ID).Updates(updates).Error
	}
}

// findingNotificationMinScore is the lowest risk score that raises a finding alert (the high band).
const findingNotificationMinScore = riskLevelHighMinScore

// findingAlertQuery selects open findings whose resource scores high or critical.
func findingAlertQuery(db *gorm.DB) *gorm.DB {
	q := db.Model(&models.Insight{}).
		Where("insights.deleted_at IS NULL AND insights.status = ?", "active").
		// Same rule as the findings list and counts: a Pod finding counts only while its pod exists.
		Where(`(insights.resource_type != 'Pod' OR EXISTS (
			SELECT 1 FROM pods p WHERE p.cluster_id = insights.cluster_id AND p.uid = insights.resource_uid AND p.deleted_at IS NULL))`)
	return whereInsightRiskScoreAtLeast(q, "insights", findingNotificationMinScore)
}

// reconcileFindingNotifications brings stored finding alerts in line with the
// findings they point at. Alerts whose finding is gone, closed, or no longer
// scores high or critical are removed; the rest take the current level.
func reconcileFindingNotifications(db *gorm.DB) {
	if !db.Migrator().HasTable("insights") {
		return
	}
	const batch = 500
	var lastID uint
	for {
		var rows []models.Notification
		if err := db.Where("deleted_at IS NULL AND id > ? AND dedupe_key LIKE ?", lastID, "insight:%").
			Order("id ASC").Limit(batch).Find(&rows).Error; err != nil || len(rows) == 0 {
			return
		}
		lastID = rows[len(rows)-1].ID
		byInsight := map[uint][]models.Notification{}
		ids := make([]uint, 0, len(rows))
		for _, n := range rows {
			id, err := strconv.ParseUint(strings.TrimPrefix(n.DedupeKey, "insight:"), 10, 64)
			if err != nil {
				continue
			}
			if _, ok := byInsight[uint(id)]; !ok {
				ids = append(ids, uint(id))
			}
			byInsight[uint(id)] = append(byInsight[uint(id)], n)
		}
		var still []models.Insight
		if len(ids) > 0 {
			if err := findingAlertQuery(db).Where("insights.id IN ?", ids).Find(&still).Error; err != nil {
				return
			}
		}
		scores := preferredScoresForInsights(db, still)
		qualifying := map[uint]models.Notification{}
		for _, i := range still {
			if score, ok := scores[riskScoreKey(i.ClusterID, i.ResourceUID)]; ok {
				qualifying[i.ID] = findingNotification(i, score, time.Now().UTC())
			}
		}
		var stale []uint
		for id, list := range byInsight {
			want, ok := qualifying[id]
			for _, n := range list {
				if !ok {
					stale = append(stale, n.ID)
					continue
				}
				upsertDerivedNotification(db, want)
			}
		}
		if len(stale) > 0 {
			_ = db.Where("id IN ?", stale).Delete(&models.Notification{}).Error
		}
		if len(rows) < batch {
			return
		}
	}
}

func derivedInsightNotifications(db *gorm.DB, now time.Time) []models.Notification {
	if !db.Migrator().HasTable("insights") {
		return nil
	}
	if !db.Migrator().HasTable(&models.RiskScore{}) {
		return nil
	}
	var insights []models.Insight
	if err := findingAlertQuery(db).
		Order("insights.detected_at DESC, insights.id DESC").
		Limit(8).
		Find(&insights).Error; err != nil {
		return nil
	}
	scores := preferredScoresForInsights(db, insights)
	out := make([]models.Notification, 0, len(insights))
	for _, i := range insights {
		score, ok := scores[riskScoreKey(i.ClusterID, i.ResourceUID)]
		if !ok {
			continue
		}
		out = append(out, findingNotification(i, score, now))
	}
	return out
}

// findingNotification describes one finding alert. Its severity is the finding's
// risk level (the band of its resource's score), the same level the finding's
// page shows; the rule severity is only mentioned in the text.
func findingNotification(i models.Insight, score float64, now time.Time) models.Notification {
	resource := strings.TrimSpace(i.ResourceName)
	if resource == "" {
		resource = strings.TrimSpace(i.ResourceUID)
	}
	resourceDisplay := podDisplayName(i.ResourceNamespace, resource, i.ResourceUID)
	createdAt := i.DetectedAt
	if createdAt.IsZero() {
		createdAt = now
	}
	level := risk.DeriveFinalLevelFromScore(score)
	message := fmt.Sprintf("%s %s has risk score %.0f/100 and requires review.", i.ResourceType, resourceDisplay, score)
	if sev := strings.ToLower(strings.TrimSpace(i.Severity)); sev != "" && sev != level {
		message = fmt.Sprintf("%s %s has risk score %.0f/100 (rule severity %s) and requires review.", i.ResourceType, resourceDisplay, score, sev)
	}
	return models.Notification{
		Title:        fmt.Sprintf("%s risk finding: %s", titleWord(level), compactTitle(i.Title, 72)),
		Message:      message,
		Severity:     level,
		Source:       "risk-engine",
		Category:     "risk",
		Route:        fmt.Sprintf("/risks/%d", i.ID),
		DedupeKey:    fmt.Sprintf("insight:%d", i.ID),
		ClusterID:    i.ClusterID,
		ResourceUID:  i.ResourceUID,
		ResourceName: resourceDisplay,
		CreatedAt:    createdAt,
	}
}

// Attack path risk is on a 0-10 scale; these are the Attack Paths page's critical and high bands.
const (
	attackPathCriticalRisk = 9.0
	attackPathHighRisk     = 7.0
)

func derivedAttackPathNotifications(db *gorm.DB, now time.Time) []models.Notification {
	if !db.Migrator().HasTable("attack_paths") {
		return nil
	}
	var paths []models.AttackPath
	if err := db.Where("total_risk >= ?", attackPathHighRisk).
		Order("total_risk DESC, updated_at DESC").
		Limit(6).
		Find(&paths).Error; err != nil {
		return nil
	}
	podNames := podDisplayNamesByUID(db, attackPathPodUIDs(paths))
	out := make([]models.Notification, 0, len(paths))
	for _, p := range paths {
		severity := "high"
		if p.TotalRisk >= attackPathCriticalRisk {
			severity = "critical"
		}
		createdAt := p.UpdatedAt
		if createdAt.IsZero() {
			createdAt = now
		}
		resourceName := trimOrFallback(podNames[p.PodUID], p.PodUID)
		out = append(out, models.Notification{
			Title:        fmt.Sprintf("%s attack path detected", titleWord(severity)),
			Message:      fmt.Sprintf("%s has path %s with risk %.1f/10 across %d steps.", resourceName, p.PathID, p.TotalRisk, p.Length),
			Severity:     severity,
			Source:       "attack-path",
			Category:     "attack-path",
			Route:        "/attack-paths?path=" + url.QueryEscape(p.PathID),
			DedupeKey:    fmt.Sprintf("attack-path:%d", p.ID),
			ClusterID:    p.ClusterID,
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
			Route:        podSBOMRoute(m.PodUID, m.ClusterID),
			DedupeKey:    fmt.Sprintf("cve-match:%d", m.ID),
			ClusterID:    m.ClusterID,
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
			Route:        podSBOMRoute(m.PodUID, m.ClusterID),
			DedupeKey:    fmt.Sprintf("malware-match:%d", m.ID),
			ClusterID:    m.ClusterID,
			ResourceUID:  m.PodUID,
			ResourceName: resourceName,
			CreatedAt:    createdAt,
		})
	}
	return out
}

// podSBOMRoute opens the pod's SBOM tab, where the matched CVE or package is
// listed with the same severity the notification shows.
func podSBOMRoute(podUID, clusterID string) string {
	q := url.Values{}
	if c := strings.TrimSpace(clusterID); c != "" {
		q.Set("clusterId", c)
	}
	q.Set("tab", "sbom")
	return "/resources/pods/uid/" + url.PathEscape(strings.TrimSpace(podUID)) + "?" + q.Encode()
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

// MarkNotificationRead marks one notification as read for the caller.
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
		userID := notificationUserID(c)
		if userID == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		var visible int64
		if err := scopedNotifications(db, c).Where("notifications.id = ?", id).Count(&visible).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if visible == 0 {
			c.JSON(http.StatusOK, gin.H{"updated": 0})
			return
		}
		tx := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.NotificationRead{
			NotificationID: uint(id),
			UserID:         userID,
			ReadAt:         time.Now().UTC(),
		})
		if tx.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": tx.Error.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"updated": tx.RowsAffected})
	}
}

// MarkAllNotificationsRead marks every notification the caller can see as read for the caller.
func MarkAllNotificationsRead(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !db.Migrator().HasTable("notifications") {
			c.JSON(http.StatusOK, gin.H{"updated": 0})
			return
		}
		userID := notificationUserID(c)
		if userID == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		unread := withReadState(scopedNotifications(db, c), userID).
			Where("nr.notification_id IS NULL").
			Select("notifications.id, ?, ?", userID, time.Now().UTC())
		tx := db.Exec("INSERT INTO notification_reads (notification_id, user_id, read_at) ?", unread)
		if tx.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": tx.Error.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"updated": tx.RowsAffected})
	}
}
