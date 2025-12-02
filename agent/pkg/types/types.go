package types

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	corev1 "k8s.io/api/core/v1"
	appsv1 "k8s.io/api/apps/v1"
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

// DeploymentData represents collected Deployment data
type DeploymentData struct {
	Name                string                     `json:"name"`
	Namespace           string                     `json:"namespace"`
	UID                 string                     `json:"uid"`
	Replicas            int32                      `json:"replicas"`
	ReadyReplicas       int32                      `json:"readyReplicas"`
	AvailableReplicas   int32                      `json:"availableReplicas"`
	UnavailableReplicas int32                      `json:"unavailableReplicas"`
	UpdatedReplicas     int32                      `json:"updatedReplicas"`
	Strategy            string                     `json:"strategy"`
	Labels              map[string]string          `json:"labels,omitempty"`
	Annotations         map[string]string          `json:"annotations,omitempty"`
	Selector            map[string]string          `json:"selector,omitempty"`
	Containers          []ContainerInfo            `json:"containers,omitempty"`
	Conditions          []appsv1.DeploymentCondition `json:"conditions,omitempty"`
	CreatedAt           metav1.Time                `json:"createdAt"`
}

// ContainerInfo contains simplified container information
type ContainerInfo struct {
	Name      string            `json:"name"`
	Image     string            `json:"image"`
	Resources ResourceInfo      `json:"resources,omitempty"`
}

// ResourceInfo contains resource requests and limits
type ResourceInfo struct {
	Requests map[string]string `json:"requests,omitempty"`
	Limits   map[string]string `json:"limits,omitempty"`
}

// CollectedData represents all collected data from a cluster
type CollectedData struct {
	ClusterID           string                   `json:"clusterId"`
	ServiceAccounts     []ServiceAccountData     `json:"serviceAccounts"`
	RoleBindings        []RoleBindingData        `json:"roleBindings"`
	ClusterRoleBindings []ClusterRoleBindingData `json:"clusterRoleBindings"`
	Roles               []RoleData               `json:"roles"`
	ClusterRoles        []ClusterRoleData        `json:"clusterRoles"`
	Pods                []PodData                `json:"pods"`
	Deployments         []DeploymentData         `json:"deployments"`
	ReplicaSets         []ReplicaSetData         `json:"replicasets"`
	CollectedAt         metav1.Time              `json:"collectedAt"`
	// Delta sync flags
	IsFullSync          bool                     `json:"isFullSync,omitempty"` // true for full sync, false for delta
	IsDeltaSync         bool                     `json:"isDeltaSync,omitempty"` // true if only sending changes
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

// ConvertDeployment converts k8s Deployment to our data type
func ConvertDeployment(dep *appsv1.Deployment) DeploymentData {
	// Extract strategy
	strategy := "RollingUpdate"
	if dep.Spec.Strategy.Type == appsv1.RecreateDeploymentStrategyType {
		strategy = "Recreate"
	}

	// Extract selector labels
	selector := make(map[string]string)
	if dep.Spec.Selector != nil && dep.Spec.Selector.MatchLabels != nil {
		selector = dep.Spec.Selector.MatchLabels
	}

	// Extract container info from pod template
	containers := make([]ContainerInfo, 0, len(dep.Spec.Template.Spec.Containers))
	for _, c := range dep.Spec.Template.Spec.Containers {
		resources := ResourceInfo{
			Requests: make(map[string]string),
			Limits:   make(map[string]string),
		}

		// Convert resource requests
		if c.Resources.Requests != nil {
			if cpu := c.Resources.Requests.Cpu(); cpu != nil {
				resources.Requests["cpu"] = cpu.String()
			}
			if mem := c.Resources.Requests.Memory(); mem != nil {
				resources.Requests["memory"] = mem.String()
			}
		}

		// Convert resource limits
		if c.Resources.Limits != nil {
			if cpu := c.Resources.Limits.Cpu(); cpu != nil {
				resources.Limits["cpu"] = cpu.String()
			}
			if mem := c.Resources.Limits.Memory(); mem != nil {
				resources.Limits["memory"] = mem.String()
			}
		}

		containers = append(containers, ContainerInfo{
			Name:      c.Name,
			Image:     c.Image,
			Resources: resources,
		})
	}

	// Get replicas (handle nil case)
	replicas := int32(1)
	if dep.Spec.Replicas != nil {
		replicas = *dep.Spec.Replicas
	}

	return DeploymentData{
		Name:                dep.Name,
		Namespace:           dep.Namespace,
		UID:                 string(dep.UID),
		Replicas:            replicas,
		ReadyReplicas:       dep.Status.ReadyReplicas,
		AvailableReplicas:   dep.Status.AvailableReplicas,
		UnavailableReplicas: dep.Status.UnavailableReplicas,
		UpdatedReplicas:     dep.Status.UpdatedReplicas,
		Strategy:            strategy,
		Labels:              dep.Labels,
		Annotations:         dep.Annotations,
		Selector:            selector,
		Containers:          containers,
		Conditions:          dep.Status.Conditions,
		CreatedAt:           dep.CreationTimestamp,
	}
}

// ReplicaSetData represents collected ReplicaSet data
type ReplicaSetData struct {
	Name                string                     `json:"name"`
	Namespace           string                     `json:"namespace"`
	UID                 string                     `json:"uid"`
	Replicas            int32                      `json:"replicas"`
	ReadyReplicas       int32                      `json:"readyReplicas"`
	AvailableReplicas   int32                      `json:"availableReplicas"`
	FullyLabeledReplicas int32                     `json:"fullyLabeledReplicas"`
	Labels              map[string]string          `json:"labels,omitempty"`
	Annotations         map[string]string          `json:"annotations,omitempty"`
	Selector            map[string]string          `json:"selector,omitempty"`
	Containers          []ContainerInfo             `json:"containers,omitempty"`
	Conditions          []appsv1.ReplicaSetCondition `json:"conditions,omitempty"`
	OwnerKind           string                     `json:"ownerKind,omitempty"`
	OwnerName           string                     `json:"ownerName,omitempty"`
	OwnerUID            string                     `json:"ownerUid,omitempty"`
	CreatedAt           metav1.Time                `json:"createdAt"`
}

// ConvertReplicaSet converts k8s ReplicaSet to our data type
func ConvertReplicaSet(rs *appsv1.ReplicaSet) ReplicaSetData {
	// Extract selector labels
	selector := make(map[string]string)
	if rs.Spec.Selector != nil && rs.Spec.Selector.MatchLabels != nil {
		selector = rs.Spec.Selector.MatchLabels
	}

	// Extract container info from pod template
	containers := make([]ContainerInfo, 0, len(rs.Spec.Template.Spec.Containers))
	for _, c := range rs.Spec.Template.Spec.Containers {
		resources := ResourceInfo{
			Requests: make(map[string]string),
			Limits:   make(map[string]string),
		}

		// Convert resource requests
		if c.Resources.Requests != nil {
			if cpu := c.Resources.Requests.Cpu(); cpu != nil {
				resources.Requests["cpu"] = cpu.String()
			}
			if mem := c.Resources.Requests.Memory(); mem != nil {
				resources.Requests["memory"] = mem.String()
			}
		}

		// Convert resource limits
		if c.Resources.Limits != nil {
			if cpu := c.Resources.Limits.Cpu(); cpu != nil {
				resources.Limits["cpu"] = cpu.String()
			}
			if mem := c.Resources.Limits.Memory(); mem != nil {
				resources.Limits["memory"] = mem.String()
			}
		}

		containers = append(containers, ContainerInfo{
			Name:      c.Name,
			Image:     c.Image,
			Resources: resources,
		})
	}

	// Extract owner reference
	ownerKind := ""
	ownerName := ""
	ownerUID := ""
	if len(rs.OwnerReferences) > 0 {
		owner := rs.OwnerReferences[0]
		ownerKind = owner.Kind
		ownerName = owner.Name
		ownerUID = string(owner.UID)
	}

	// Get replicas (handle nil case)
	replicas := int32(1)
	if rs.Spec.Replicas != nil {
		replicas = *rs.Spec.Replicas
	}

	return ReplicaSetData{
		Name:                rs.Name,
		Namespace:           rs.Namespace,
		UID:                 string(rs.UID),
		Replicas:            replicas,
		ReadyReplicas:       rs.Status.ReadyReplicas,
		AvailableReplicas:   rs.Status.AvailableReplicas,
		FullyLabeledReplicas: rs.Status.FullyLabeledReplicas,
		Labels:              rs.Labels,
		Annotations:         rs.Annotations,
		Selector:            selector,
		Containers:          containers,
		Conditions:          rs.Status.Conditions,
		OwnerKind:           ownerKind,
		OwnerName:           ownerName,
		OwnerUID:            ownerUID,
		CreatedAt:           rs.CreationTimestamp,
	}
}

