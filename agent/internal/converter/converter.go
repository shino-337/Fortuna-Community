package converter

import (
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/watch"

	fortuna "github.com/ksam/agent/proto/gen/proto"
)

// PodToInventoryItem converts a Pod to InventoryItem
func PodToInventoryItem(pod *corev1.Pod, clusterID string, eventType watch.EventType) *fortuna.InventoryItem {
	labels := make(map[string]string)
	for k, v := range pod.Labels {
		labels[k] = v
	}

	return &fortuna.InventoryItem{
		Kind:      "Pod",
		Uid:       string(pod.UID),
		Name:      pod.Name,
		Namespace: pod.Namespace,
		Labels:    labels,
		RawJson:   marshalObject(pod),
		Timestamp: pod.CreationTimestamp.Unix(),
	}
}

// ServiceAccountToInventoryItem converts a ServiceAccount to InventoryItem
func ServiceAccountToInventoryItem(sa *corev1.ServiceAccount, clusterID string, eventType watch.EventType) *fortuna.InventoryItem {
	labels := make(map[string]string)
	for k, v := range sa.Labels {
		labels[k] = v
	}

	return &fortuna.InventoryItem{
		Kind:      "ServiceAccount",
		Uid:       string(sa.UID),
		Name:      sa.Name,
		Namespace: sa.Namespace,
		Labels:    labels,
		RawJson:   marshalObject(sa),
		Timestamp: sa.CreationTimestamp.Unix(),
	}
}

// RoleToInventoryItem converts a Role to InventoryItem
func RoleToInventoryItem(role *rbacv1.Role, clusterID string, eventType watch.EventType) *fortuna.InventoryItem {
	labels := make(map[string]string)
	for k, v := range role.Labels {
		labels[k] = v
	}

	return &fortuna.InventoryItem{
		Kind:      "Role",
		Uid:       string(role.UID),
		Name:      role.Name,
		Namespace: role.Namespace,
		Labels:    labels,
		RawJson:   marshalObject(role),
		Timestamp: role.CreationTimestamp.Unix(),
	}
}

// RoleBindingToInventoryItem converts a RoleBinding to InventoryItem
func RoleBindingToInventoryItem(rb *rbacv1.RoleBinding, clusterID string, eventType watch.EventType) *fortuna.InventoryItem {
	labels := make(map[string]string)
	for k, v := range rb.Labels {
		labels[k] = v
	}

	return &fortuna.InventoryItem{
		Kind:      "RoleBinding",
		Uid:       string(rb.UID),
		Name:      rb.Name,
		Namespace: rb.Namespace,
		Labels:    labels,
		RawJson:   marshalObject(rb),
		Timestamp: rb.CreationTimestamp.Unix(),
	}
}

// ClusterRoleToInventoryItem converts a ClusterRole to InventoryItem
func ClusterRoleToInventoryItem(cr *rbacv1.ClusterRole, clusterID string, eventType watch.EventType) *fortuna.InventoryItem {
	labels := make(map[string]string)
	for k, v := range cr.Labels {
		labels[k] = v
	}

	return &fortuna.InventoryItem{
		Kind:      "ClusterRole",
		Uid:       string(cr.UID),
		Name:      cr.Name,
		Namespace: "", // Cluster-scoped
		Labels:    labels,
		RawJson:   marshalObject(cr),
		Timestamp: cr.CreationTimestamp.Unix(),
	}
}

// ClusterRoleBindingToInventoryItem converts a ClusterRoleBinding to InventoryItem
func ClusterRoleBindingToInventoryItem(crb *rbacv1.ClusterRoleBinding, clusterID string, eventType watch.EventType) *fortuna.InventoryItem {
	labels := make(map[string]string)
	for k, v := range crb.Labels {
		labels[k] = v
	}

	return &fortuna.InventoryItem{
		Kind:      "ClusterRoleBinding",
		Uid:       string(crb.UID),
		Name:      crb.Name,
		Namespace: "", // Cluster-scoped
		Labels:    labels,
		RawJson:   marshalObject(crb),
		Timestamp: crb.CreationTimestamp.Unix(),
	}
}

// marshalObject marshals a Kubernetes object to JSON
func marshalObject(obj interface{}) string {
	data, err := json.Marshal(obj)
	if err != nil {
		return "{}"
	}
	return string(data)
}

