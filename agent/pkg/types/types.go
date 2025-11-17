package types

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	corev1 "k8s.io/api/core/v1"
)

// ServiceAccountData represents collected ServiceAccount data
type ServiceAccountData struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	UID       string            `json:"uid"`
	Labels    map[string]string `json:"labels,omitempty"`
	Secrets   []string          `json:"secrets,omitempty"`
	CreatedAt metav1.Time       `json:"createdAt"`
}

// RoleBindingData represents collected RoleBinding data
type RoleBindingData struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	UID       string            `json:"uid"`
	RoleRef   rbacv1.RoleRef    `json:"roleRef"`
	Subjects  []rbacv1.Subject  `json:"subjects"`
	CreatedAt metav1.Time       `json:"createdAt"`
}

// ClusterRoleBindingData represents collected ClusterRoleBinding data
type ClusterRoleBindingData struct {
	Name      string            `json:"name"`
	UID       string            `json:"uid"`
	RoleRef   rbacv1.RoleRef    `json:"roleRef"`
	Subjects  []rbacv1.Subject  `json:"subjects"`
	CreatedAt metav1.Time       `json:"createdAt"`
}

// RoleData represents collected Role data
type RoleData struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	UID       string            `json:"uid"`
	Rules     []rbacv1.PolicyRule `json:"rules"`
	CreatedAt metav1.Time       `json:"createdAt"`
}

// ClusterRoleData represents collected ClusterRole data
type ClusterRoleData struct {
	Name      string            `json:"name"`
	UID       string            `json:"uid"`
	Rules     []rbacv1.PolicyRule `json:"rules"`
	CreatedAt metav1.Time       `json:"createdAt"`
}

// PodData represents Pod that uses a ServiceAccount
type PodData struct {
	Name            string `json:"name"`
	Namespace       string `json:"namespace"`
	ServiceAccount  string `json:"serviceAccount"`
	UID             string `json:"uid"`
}

// CollectedData represents all collected data from a cluster
type CollectedData struct {
	ClusterID           string                  `json:"clusterId"`
	ServiceAccounts     []ServiceAccountData    `json:"serviceAccounts"`
	RoleBindings        []RoleBindingData       `json:"roleBindings"`
	ClusterRoleBindings []ClusterRoleBindingData `json:"clusterRoleBindings"`
	Roles               []RoleData              `json:"roles"`
	ClusterRoles        []ClusterRoleData       `json:"clusterRoles"`
	Pods                []PodData               `json:"pods"`
	CollectedAt         metav1.Time             `json:"collectedAt"`
	// Delta sync flags
	IsFullSync          bool                    `json:"isFullSync,omitempty"` // true for full sync, false for delta
	IsDeltaSync         bool                    `json:"isDeltaSync,omitempty"` // true if only sending changes
}

// ConvertServiceAccount converts k8s ServiceAccount to our data type
func ConvertServiceAccount(sa *corev1.ServiceAccount) ServiceAccountData {
	secrets := make([]string, 0, len(sa.Secrets))
	for _, secret := range sa.Secrets {
		secrets = append(secrets, secret.Name)
	}

	return ServiceAccountData{
		Name:      sa.Name,
		Namespace: sa.Namespace,
		UID:       string(sa.UID),
		Labels:    sa.Labels,
		Secrets:   secrets,
		CreatedAt: sa.CreationTimestamp,
	}
}

// ConvertRoleBinding converts k8s RoleBinding to our data type
func ConvertRoleBinding(rb *rbacv1.RoleBinding) RoleBindingData {
	return RoleBindingData{
		Name:      rb.Name,
		Namespace: rb.Namespace,
		UID:       string(rb.UID),
		RoleRef:   rb.RoleRef,
		Subjects:  rb.Subjects,
		CreatedAt: rb.CreationTimestamp,
	}
}

// ConvertClusterRoleBinding converts k8s ClusterRoleBinding to our data type
func ConvertClusterRoleBinding(crb *rbacv1.ClusterRoleBinding) ClusterRoleBindingData {
	return ClusterRoleBindingData{
		Name:      crb.Name,
		UID:       string(crb.UID),
		RoleRef:   crb.RoleRef,
		Subjects:  crb.Subjects,
		CreatedAt: crb.CreationTimestamp,
	}
}

// ConvertRole converts k8s Role to our data type
func ConvertRole(role *rbacv1.Role) RoleData {
	return RoleData{
		Name:      role.Name,
		Namespace: role.Namespace,
		UID:       string(role.UID),
		Rules:     role.Rules,
		CreatedAt: role.CreationTimestamp,
	}
}

// ConvertClusterRole converts k8s ClusterRole to our data type
func ConvertClusterRole(cr *rbacv1.ClusterRole) ClusterRoleData {
	return ClusterRoleData{
		Name:      cr.Name,
		UID:       string(cr.UID),
		Rules:     cr.Rules,
		CreatedAt: cr.CreationTimestamp,
	}
}

// ConvertPod converts k8s Pod to our data type
func ConvertPod(pod *corev1.Pod) PodData {
	saName := pod.Spec.ServiceAccountName
	if saName == "" {
		saName = "default"
	}

	return PodData{
		Name:           pod.Name,
		Namespace:      pod.Namespace,
		ServiceAccount: saName,
		UID:            string(pod.UID),
	}
}

