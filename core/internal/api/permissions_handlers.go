package api

import (
	"encoding/json"
	"fmt"
	"github.com/fortuna/core/pkg/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	rbacv1 "k8s.io/api/rbac/v1"
	"net/http"
)

// These are synchronized RBAC grants, not a live Kubernetes authorization decision.
type ServiceAccountPermissions struct {
	ServiceAccountID    uint                           `json:"serviceAccountId"`
	RoleBindings        []RoleBindingPermission        `json:"roleBindings"`
	ClusterRoleBindings []ClusterRoleBindingPermission `json:"clusterRoleBindings"`
	EffectiveRules      []Rule                         `json:"effectiveRules"`
}
type RoleBindingPermission struct {
	RoleBinding models.RoleBinding  `json:"roleBinding"`
	Role        *models.Role        `json:"role,omitempty"`
	ClusterRole *models.ClusterRole `json:"clusterRole,omitempty"`
}
type ClusterRoleBindingPermission struct {
	ClusterRoleBinding models.ClusterRoleBinding `json:"clusterRoleBinding"`
	ClusterRole        models.ClusterRole        `json:"clusterRole"`
}
type Rule struct {
	Verbs           []string `json:"verbs"`
	APIGroups       []string `json:"apiGroups"`
	Resources       []string `json:"resources"`
	ResourceNames   []string `json:"resourceNames"`
	NonResourceURLs []string `json:"nonResourceURLs"`
	Scope           string   `json:"scope"`
	Namespace       string   `json:"namespace,omitempty"`
	BindingKind     string   `json:"bindingKind"`
	BindingName     string   `json:"bindingName"`
}

// A missing SA namespace defaults only to a RoleBinding's own namespace.
// ClusterRoleBindings have no namespace to default to.
func serviceAccountSubjectMatches(subject rbacv1.Subject, namespace string, sa *models.ServiceAccount) bool {
	switch subject.Kind {
	case "ServiceAccount":
		if subject.APIGroup != "" {
			return false
		}
		ns := subject.Namespace
		if ns == "" {
			ns = namespace
		}
		return ns != "" && ns == sa.Namespace && subject.Name == sa.Name
	case "User":
		return subject.APIGroup == rbacv1.GroupName && subject.Name == "system:serviceaccount:"+sa.Namespace+":"+sa.Name
	case "Group":
		if subject.APIGroup != rbacv1.GroupName {
			return false
		}
		return subject.Name == "system:serviceaccounts" || subject.Name == "system:serviceaccounts:"+sa.Namespace || subject.Name == "system:authenticated"
	}
	return false
}

func buildServiceAccountPermissions(db *gorm.DB, sa *models.ServiceAccount) (ServiceAccountPermissions, error) {
	out := ServiceAccountPermissions{ServiceAccountID: sa.ID, RoleBindings: []RoleBindingPermission{}, ClusterRoleBindings: []ClusterRoleBindingPermission{}, EffectiveRules: []Rule{}}
	var rbs []models.RoleBinding
	var crbs []models.ClusterRoleBinding
	var roles []models.Role
	var clusterRoles []models.ClusterRole
	// Batch reads avoid a query for every binding, and surface storage failures.
	for _, rows := range []any{&rbs, &crbs, &roles, &clusterRoles} {
		if err := db.Where("cluster_id = ?", sa.ClusterID).Order("id ASC").Find(rows).Error; err != nil {
			return out, err
		}
	}
	roleByName := map[[2]string]*models.Role{}
	for i := range roles {
		roleByName[[2]string{roles[i].Namespace, roles[i].Name}] = &roles[i]
	}
	clusterRoleByName := map[string]*models.ClusterRole{}
	for i := range clusterRoles {
		clusterRoleByName[clusterRoles[i].Name] = &clusterRoles[i]
	}
	resolve := func(subjectJSON, refJSON, namespace, kind, name string) (*models.Role, *models.ClusterRole, bool, error) {
		var subjects []rbacv1.Subject
		if err := json.Unmarshal([]byte(subjectJSON), &subjects); err != nil {
			return nil, nil, false, fmt.Errorf("invalid subjects: %w", err)
		}
		matched := false
		for _, subject := range subjects {
			if serviceAccountSubjectMatches(subject, namespace, sa) {
				matched = true
				break
			}
		}
		if !matched {
			return nil, nil, false, nil
		}
		var ref rbacv1.RoleRef
		if err := json.Unmarshal([]byte(refJSON), &ref); err != nil {
			return nil, nil, false, err
		}
		if ref.APIGroup != rbacv1.GroupName || ref.Name == "" {
			return nil, nil, false, fmt.Errorf("invalid roleRef")
		}
		var role *models.Role
		var cr *models.ClusterRole
		var rawRules string
		switch ref.Kind {
		case "Role":
			if kind != "RoleBinding" {
				return nil, nil, false, fmt.Errorf("ClusterRoleBinding cannot reference Role")
			}
			role = roleByName[[2]string{namespace, ref.Name}]
			if role == nil {
				return nil, nil, false, fmt.Errorf("referenced Role not synchronized")
			}
			rawRules = role.Rules
		case "ClusterRole":
			cr = clusterRoleByName[ref.Name]
			if cr == nil {
				return nil, nil, false, fmt.Errorf("referenced ClusterRole not synchronized")
			}
			rawRules = cr.Rules
		default:
			return nil, nil, false, fmt.Errorf("invalid role kind")
		}
		var rules []Rule
		if err := json.Unmarshal([]byte(rawRules), &rules); err != nil {
			return nil, nil, false, err
		}
		for _, rule := range rules {
			rule.Scope, rule.Namespace, rule.BindingKind, rule.BindingName = "cluster", "", kind, name
			if kind == "RoleBinding" {
				rule.Scope, rule.Namespace = "namespace", namespace
				// Non-resource requests cannot be authorized by a namespaced binding.
				rule.NonResourceURLs = nil
				if len(rule.Resources) == 0 {
					continue
				}
			}
			out.EffectiveRules = append(out.EffectiveRules, rule)
		}
		return role, cr, true, nil
	}
	for _, rb := range rbs {
		role, cr, matched, err := resolve(rb.Subjects, rb.RoleRef, rb.Namespace, "RoleBinding", rb.Name)
		if err != nil {
			return out, err
		}
		if matched {
			out.RoleBindings = append(out.RoleBindings, RoleBindingPermission{RoleBinding: rb, Role: role, ClusterRole: cr})
		}
	}
	for _, crb := range crbs {
		_, cr, matched, err := resolve(crb.Subjects, crb.RoleRef, "", "ClusterRoleBinding", crb.Name)
		if err != nil {
			return out, err
		}
		if matched {
			out.ClusterRoleBindings = append(out.ClusterRoleBindings, ClusterRoleBindingPermission{ClusterRoleBinding: crb, ClusterRole: *cr})
		}
	}
	return out, nil
}

// GetServiceAccountPermissions returns permissions for a specific ServiceAccount (by DB id).
func GetServiceAccountPermissions(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var sa models.ServiceAccount
		if err := db.First(&sa, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "ServiceAccount not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to load ServiceAccount"})
			return
		}
		if !authorizeServiceAccount(db, c, &sa) {
			return
		}
		permissions, err := buildServiceAccountPermissions(db.WithContext(c.Request.Context()), &sa)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to resolve permissions from synchronized RBAC inventory"})
			return
		}
		c.JSON(http.StatusOK, permissions)
	}
}

// GetServiceAccountPermissionsByUID returns permissions for a ServiceAccount by Kubernetes UID (inventory domain).
func GetServiceAccountPermissionsByUID(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := c.Param("uid")
		if uid == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "uid is required"})
			return
		}
		var sa models.ServiceAccount
		if err := db.Where("uid = ?", uid).First(&sa).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "ServiceAccount not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to load ServiceAccount"})
			return
		}
		if !authorizeServiceAccount(db, c, &sa) {
			return
		}
		permissions, err := buildServiceAccountPermissions(db.WithContext(c.Request.Context()), &sa)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to resolve permissions from synchronized RBAC inventory"})
			return
		}
		c.JSON(http.StatusOK, permissions)
	}
}
