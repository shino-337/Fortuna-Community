package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/api/listlimit"
	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/models"
)

// GetResources returns a normalized resource list for the dashboard explorer.
// Query params: kind, namespace, cluster
func GetResources(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		db := db.WithContext(c.Request.Context())
		kind := c.Query("kind")
		namespace := c.Query("namespace")
		clusterID := strings.TrimSpace(c.Query("cluster"))
		scopedClusterIDs, restricted := middleware.ScopedClusterIDs(c)
		if clusterID != "" && restricted && !middleware.ClusterAllowed(c, clusterID) {
			middleware.AbortClusterScopeDenied(db, c, clusterID)
			return
		}
		tables := []struct{ resourceKind, table string }{
			{"Pod", "pods"}, {"ServiceAccount", "service_accounts"}, {"Role", "roles"},
			{"ClusterRole", "cluster_roles"}, {"RoleBinding", "role_bindings"},
			{"ClusterRoleBinding", "cluster_role_bindings"},
		}
		for _, entry := range tables {
			if kind == "" || kind == entry.resourceKind {
				if !requireAvailabilityTables(c, db, "resource_inventory_schema_unavailable",
					"Resource inventory requires the "+entry.table+" schema", entry.table) {
					return
				}
			}
		}

		resources := make([]map[string]interface{}, 0)
		// One budget across all kinds: each kind query fetches at most the
		// remaining budget plus one row so truncation is detectable.
		limit := listlimit.Parse(c, resourcesDefaultLimit, resourcesMaxLimit)
		truncated := false
		remaining := func() int { return limit - len(resources) }
		bounded := func(query *gorm.DB, namespaced bool) *gorm.DB {
			if namespaced {
				query = query.Order("cluster_id, namespace, name, uid")
			} else {
				query = query.Order("cluster_id, name, uid")
			}
			return query.Limit(remaining() + 1)
		}

		appendResource := func(kind string, name string, namespace string, uid string, clusterID string) {
			if len(resources) >= limit {
				truncated = true
				return
			}
			resources = append(resources, map[string]interface{}{
				"kind":      kind,
				"name":      name,
				"namespace": namespace,
				"uid":       uid,
				"clusterId": clusterID,
			})
		}

		applyClusterScope := func(query *gorm.DB) *gorm.DB {
			if clusterID != "" {
				return query.Where("cluster_id = ?", clusterID)
			}
			if restricted {
				return query.Where("cluster_id IN ?", scopedClusterIDs)
			}
			return query
		}

		if kind == "" || kind == "Pod" {
			var pods []models.Pod
			query := db.Model(&models.Pod{})
			if namespace != "" {
				query = query.Where("namespace = ?", namespace)
			}
			query = applyClusterScope(query)
			if err := bounded(query, true).Find(&pods).Error; err != nil {
				respondDataUnavailable(c, "resource_inventory_query_failed", "Resource inventory could not be loaded")
				return
			}
			for _, pod := range pods {
				appendResource("Pod", pod.Name, pod.Namespace, pod.UID, pod.ClusterID)
			}
		}

		if kind == "" || kind == "ServiceAccount" {
			var sas []models.ServiceAccount
			query := db.Model(&models.ServiceAccount{})
			if namespace != "" {
				query = query.Where("namespace = ?", namespace)
			}
			query = applyClusterScope(query)
			if err := bounded(query, true).Find(&sas).Error; err != nil {
				respondDataUnavailable(c, "resource_inventory_query_failed", "Resource inventory could not be loaded")
				return
			}
			for _, sa := range sas {
				appendResource("ServiceAccount", sa.Name, sa.Namespace, sa.UID, sa.ClusterID)
			}
		}

		if kind == "" || kind == "Role" {
			var roles []models.Role
			query := db.Model(&models.Role{})
			if namespace != "" {
				query = query.Where("namespace = ?", namespace)
			}
			query = applyClusterScope(query)
			if err := bounded(query, true).Find(&roles).Error; err != nil {
				respondDataUnavailable(c, "resource_inventory_query_failed", "Resource inventory could not be loaded")
				return
			}
			for _, role := range roles {
				appendResource("Role", role.Name, role.Namespace, role.UID, role.ClusterID)
			}
		}

		if kind == "" || kind == "ClusterRole" {
			var roles []models.ClusterRole
			query := db.Model(&models.ClusterRole{})
			query = applyClusterScope(query)
			if err := bounded(query, false).Find(&roles).Error; err != nil {
				respondDataUnavailable(c, "resource_inventory_query_failed", "Resource inventory could not be loaded")
				return
			}
			for _, role := range roles {
				appendResource("ClusterRole", role.Name, "", role.UID, role.ClusterID)
			}
		}

		if kind == "" || kind == "RoleBinding" {
			var bindings []models.RoleBinding
			query := db.Model(&models.RoleBinding{})
			if namespace != "" {
				query = query.Where("namespace = ?", namespace)
			}
			query = applyClusterScope(query)
			if err := bounded(query, true).Find(&bindings).Error; err != nil {
				respondDataUnavailable(c, "resource_inventory_query_failed", "Resource inventory could not be loaded")
				return
			}
			for _, binding := range bindings {
				appendResource("RoleBinding", binding.Name, binding.Namespace, binding.UID, binding.ClusterID)
			}
		}

		if kind == "" || kind == "ClusterRoleBinding" {
			var bindings []models.ClusterRoleBinding
			query := db.Model(&models.ClusterRoleBinding{})
			query = applyClusterScope(query)
			if err := bounded(query, false).Find(&bindings).Error; err != nil {
				respondDataUnavailable(c, "resource_inventory_query_failed", "Resource inventory could not be loaded")
				return
			}
			for _, binding := range bindings {
				appendResource("ClusterRoleBinding", binding.Name, "", binding.UID, binding.ClusterID)
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"resources": resources,
			"total":     len(resources),
			"truncated": truncated,
		})
	}
}

// GetResourceDetail returns inspectable Kubernetes inventory metadata by kind + Kubernetes UID.
// It is intentionally scoped to RBAC inventory objects whose list view otherwise only has name/UID.
func GetResourceDetail(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		kind := strings.TrimSpace(c.Param("kind"))
		uid := strings.TrimSpace(c.Param("uid"))
		if kind == "" || uid == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "kind and uid are required"})
			return
		}

		parseJSON := func(raw string, fallback interface{}) interface{} {
			if strings.TrimSpace(raw) == "" {
				return fallback
			}
			var out interface{}
			if err := json.Unmarshal([]byte(raw), &out); err != nil {
				return fallback
			}
			return out
		}
		requireCluster := func(clusterID string) bool {
			if _, restricted := middleware.ScopedClusterIDs(c); !restricted {
				return true
			}
			if middleware.ClusterAllowed(c, clusterID) {
				return true
			}
			middleware.AbortClusterScopeDenied(db, c, clusterID)
			return false
		}

		switch kind {
		case "Role":
			var role models.Role
			if err := db.Where("uid = ? AND deleted_at IS NULL", uid).First(&role).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Role not found"})
				return
			}
			if !requireCluster(role.ClusterID) {
				return
			}
			var bindings []models.RoleBinding
			db.Where("cluster_id = ? AND namespace = ? AND deleted_at IS NULL", role.ClusterID, role.Namespace).Find(&bindings)
			referencedBy := make([]models.RoleBinding, 0)
			for _, binding := range bindings {
				if roleRefKind(binding.RoleRef) == "Role" && roleRefName(binding.RoleRef) == role.Name {
					referencedBy = append(referencedBy, binding)
				}
			}
			c.JSON(http.StatusOK, gin.H{
				"kind":         "Role",
				"resource":     role,
				"rules":        parseJSON(role.Rules, []interface{}{}),
				"referencedBy": referencedBy,
			})
		case "ClusterRole":
			var role models.ClusterRole
			if err := db.Where("uid = ? AND deleted_at IS NULL", uid).First(&role).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "ClusterRole not found"})
				return
			}
			if !requireCluster(role.ClusterID) {
				return
			}
			var rbs []models.RoleBinding
			var crbs []models.ClusterRoleBinding
			db.Where("cluster_id = ? AND deleted_at IS NULL", role.ClusterID).Find(&rbs)
			db.Where("cluster_id = ? AND deleted_at IS NULL", role.ClusterID).Find(&crbs)
			roleBindings := make([]models.RoleBinding, 0)
			clusterRoleBindings := make([]models.ClusterRoleBinding, 0)
			for _, binding := range rbs {
				if roleRefKind(binding.RoleRef) == "ClusterRole" && roleRefName(binding.RoleRef) == role.Name {
					roleBindings = append(roleBindings, binding)
				}
			}
			for _, binding := range crbs {
				if roleRefName(binding.RoleRef) == role.Name {
					clusterRoleBindings = append(clusterRoleBindings, binding)
				}
			}
			c.JSON(http.StatusOK, gin.H{
				"kind":                "ClusterRole",
				"resource":            role,
				"rules":               parseJSON(role.Rules, []interface{}{}),
				"roleBindings":        roleBindings,
				"clusterRoleBindings": clusterRoleBindings,
			})
		case "RoleBinding":
			var binding models.RoleBinding
			if err := db.Where("uid = ? AND deleted_at IS NULL", uid).First(&binding).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "RoleBinding not found"})
				return
			}
			if !requireCluster(binding.ClusterID) {
				return
			}
			detail := gin.H{
				"kind":     "RoleBinding",
				"resource": binding,
				"roleRef":  parseJSON(binding.RoleRef, gin.H{}),
				"subjects": parseJSON(binding.Subjects, []interface{}{}),
			}
			switch roleRefKind(binding.RoleRef) {
			case "ClusterRole":
				var role models.ClusterRole
				if err := db.Where("cluster_id = ? AND name = ? AND deleted_at IS NULL", binding.ClusterID, roleRefName(binding.RoleRef)).First(&role).Error; err == nil {
					detail["resolvedRole"] = role
					detail["rules"] = parseJSON(role.Rules, []interface{}{})
				}
			case "Role":
				var role models.Role
				if err := db.Where("cluster_id = ? AND namespace = ? AND name = ? AND deleted_at IS NULL", binding.ClusterID, binding.Namespace, roleRefName(binding.RoleRef)).First(&role).Error; err == nil {
					detail["resolvedRole"] = role
					detail["rules"] = parseJSON(role.Rules, []interface{}{})
				}
			}
			c.JSON(http.StatusOK, detail)
		case "ClusterRoleBinding":
			var binding models.ClusterRoleBinding
			if err := db.Where("uid = ? AND deleted_at IS NULL", uid).First(&binding).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "ClusterRoleBinding not found"})
				return
			}
			if !requireCluster(binding.ClusterID) {
				return
			}
			detail := gin.H{
				"kind":     "ClusterRoleBinding",
				"resource": binding,
				"roleRef":  parseJSON(binding.RoleRef, gin.H{}),
				"subjects": parseJSON(binding.Subjects, []interface{}{}),
			}
			if roleRefKind(binding.RoleRef) == "ClusterRole" {
				var role models.ClusterRole
				if err := db.Where("cluster_id = ? AND name = ? AND deleted_at IS NULL", binding.ClusterID, roleRefName(binding.RoleRef)).First(&role).Error; err == nil {
					detail["resolvedRole"] = role
					detail["rules"] = parseJSON(role.Rules, []interface{}{})
				}
			}
			c.JSON(http.StatusOK, detail)
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported resource kind"})
		}
	}
}

func roleRefName(raw string) string {
	var ref map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &ref); err != nil {
		return ""
	}
	if name, ok := ref["name"].(string); ok {
		return name
	}
	return ""
}

func roleRefKind(raw string) string {
	var ref map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &ref); err != nil {
		return ""
	}
	if kind, ok := ref["kind"].(string); ok {
		return kind
	}
	return ""
}
