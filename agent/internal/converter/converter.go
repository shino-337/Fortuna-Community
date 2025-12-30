package converter

import (
	"encoding/json"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/watch"

	fortuna "github.com/fortuna/api/proto/agent"
)

// PodToInventoryItem converts a Pod to InventoryItem
// Layer 1: Agent Prevention - Add timestamp and eventType
func PodToInventoryItem(pod *corev1.Pod, clusterID string, eventType watch.EventType) *fortuna.InventoryItem {
	labels := make(map[string]string)
	for k, v := range pod.Labels {
		labels[k] = v
	}

	// Layer 1: Use current time instead of CreationTimestamp
	timestamp := time.Now().Unix()

	// Layer 1: Add eventType to raw_json metadata
	rawJSON := marshalObject(pod)
	var podJSON map[string]interface{}
	if err := json.Unmarshal([]byte(rawJSON), &podJSON); err == nil {
		// Add metadata fields
		if podJSON == nil {
			podJSON = make(map[string]interface{})
		}
		// Convert eventType to string: "Added", "Modified", "Deleted"
		eventTypeStr := "Added"
		switch eventType {
		case watch.Added:
			eventTypeStr = "Added"
		case watch.Modified:
			eventTypeStr = "Modified"
		case watch.Deleted:
			eventTypeStr = "Deleted"
		}
		podJSON["_fortuna_event_type"] = eventTypeStr
		podJSON["_fortuna_timestamp"] = timestamp
		podJSON["_fortuna_cluster_id"] = clusterID
		
		// Re-marshal with metadata
		if rawJSONBytes, err := json.Marshal(podJSON); err == nil {
			rawJSON = string(rawJSONBytes)
		}
	}

	return &fortuna.InventoryItem{
		Kind:      "Pod",
		Uid:       string(pod.UID),
		Name:      pod.Name,
		Namespace: pod.Namespace,
		Labels:    labels,
		RawJson:   rawJSON,
		Timestamp: timestamp, // Layer 1: Current time
	}
}

// ServiceAccountToInventoryItem converts a ServiceAccount to InventoryItem
// Bug 5 Fix: Use eventType like PodToInventoryItem
func ServiceAccountToInventoryItem(sa *corev1.ServiceAccount, clusterID string, eventType watch.EventType) *fortuna.InventoryItem {
	labels := make(map[string]string)
	for k, v := range sa.Labels {
		labels[k] = v
	}

	// Bug 5 Fix: Use current time and add eventType metadata
	timestamp := time.Now().Unix()
	rawJSON := marshalObject(sa)
	var saJSON map[string]interface{}
	if err := json.Unmarshal([]byte(rawJSON), &saJSON); err == nil {
		if saJSON == nil {
			saJSON = make(map[string]interface{})
		}
		eventTypeStr := "Added"
		switch eventType {
		case watch.Added:
			eventTypeStr = "Added"
		case watch.Modified:
			eventTypeStr = "Modified"
		case watch.Deleted:
			eventTypeStr = "Deleted"
		}
		saJSON["_fortuna_event_type"] = eventTypeStr
		saJSON["_fortuna_timestamp"] = timestamp
		saJSON["_fortuna_cluster_id"] = clusterID
		
		if rawJSONBytes, err := json.Marshal(saJSON); err == nil {
			rawJSON = string(rawJSONBytes)
		}
	}

	return &fortuna.InventoryItem{
		Kind:      "ServiceAccount",
		Uid:       string(sa.UID),
		Name:      sa.Name,
		Namespace: sa.Namespace,
		Labels:    labels,
		RawJson:   rawJSON,
		Timestamp: timestamp,
	}
}

// RoleToInventoryItem converts a Role to InventoryItem
// Bug 5 Fix: Use eventType like PodToInventoryItem
func RoleToInventoryItem(role *rbacv1.Role, clusterID string, eventType watch.EventType) *fortuna.InventoryItem {
	labels := make(map[string]string)
	for k, v := range role.Labels {
		labels[k] = v
	}

	// Bug 5 Fix: Use current time and add eventType metadata
	timestamp := time.Now().Unix()
	rawJSON := marshalObject(role)
	var roleJSON map[string]interface{}
	if err := json.Unmarshal([]byte(rawJSON), &roleJSON); err == nil {
		if roleJSON == nil {
			roleJSON = make(map[string]interface{})
		}
		eventTypeStr := "Added"
		switch eventType {
		case watch.Added:
			eventTypeStr = "Added"
		case watch.Modified:
			eventTypeStr = "Modified"
		case watch.Deleted:
			eventTypeStr = "Deleted"
		}
		roleJSON["_fortuna_event_type"] = eventTypeStr
		roleJSON["_fortuna_timestamp"] = timestamp
		roleJSON["_fortuna_cluster_id"] = clusterID
		
		if rawJSONBytes, err := json.Marshal(roleJSON); err == nil {
			rawJSON = string(rawJSONBytes)
		}
	}

	return &fortuna.InventoryItem{
		Kind:      "Role",
		Uid:       string(role.UID),
		Name:      role.Name,
		Namespace: role.Namespace,
		Labels:    labels,
		RawJson:   rawJSON,
		Timestamp: timestamp,
	}
}

// RoleBindingToInventoryItem converts a RoleBinding to InventoryItem
// Bug 5 Fix: Use eventType like PodToInventoryItem
func RoleBindingToInventoryItem(rb *rbacv1.RoleBinding, clusterID string, eventType watch.EventType) *fortuna.InventoryItem {
	labels := make(map[string]string)
	for k, v := range rb.Labels {
		labels[k] = v
	}

	// Bug 5 Fix: Use current time and add eventType metadata
	timestamp := time.Now().Unix()
	rawJSON := marshalObject(rb)
	var rbJSON map[string]interface{}
	if err := json.Unmarshal([]byte(rawJSON), &rbJSON); err == nil {
		if rbJSON == nil {
			rbJSON = make(map[string]interface{})
		}
		eventTypeStr := "Added"
		switch eventType {
		case watch.Added:
			eventTypeStr = "Added"
		case watch.Modified:
			eventTypeStr = "Modified"
		case watch.Deleted:
			eventTypeStr = "Deleted"
		}
		rbJSON["_fortuna_event_type"] = eventTypeStr
		rbJSON["_fortuna_timestamp"] = timestamp
		rbJSON["_fortuna_cluster_id"] = clusterID
		
		if rawJSONBytes, err := json.Marshal(rbJSON); err == nil {
			rawJSON = string(rawJSONBytes)
		}
	}

	return &fortuna.InventoryItem{
		Kind:      "RoleBinding",
		Uid:       string(rb.UID),
		Name:      rb.Name,
		Namespace: rb.Namespace,
		Labels:    labels,
		RawJson:   rawJSON,
		Timestamp: timestamp,
	}
}

// ClusterRoleToInventoryItem converts a ClusterRole to InventoryItem
// Bug 5 Fix: Use eventType like PodToInventoryItem
func ClusterRoleToInventoryItem(cr *rbacv1.ClusterRole, clusterID string, eventType watch.EventType) *fortuna.InventoryItem {
	labels := make(map[string]string)
	for k, v := range cr.Labels {
		labels[k] = v
	}

	// Bug 5 Fix: Use current time and add eventType metadata
	timestamp := time.Now().Unix()
	rawJSON := marshalObject(cr)
	var crJSON map[string]interface{}
	if err := json.Unmarshal([]byte(rawJSON), &crJSON); err == nil {
		if crJSON == nil {
			crJSON = make(map[string]interface{})
		}
		eventTypeStr := "Added"
		switch eventType {
		case watch.Added:
			eventTypeStr = "Added"
		case watch.Modified:
			eventTypeStr = "Modified"
		case watch.Deleted:
			eventTypeStr = "Deleted"
		}
		crJSON["_fortuna_event_type"] = eventTypeStr
		crJSON["_fortuna_timestamp"] = timestamp
		crJSON["_fortuna_cluster_id"] = clusterID
		
		if rawJSONBytes, err := json.Marshal(crJSON); err == nil {
			rawJSON = string(rawJSONBytes)
		}
	}

	return &fortuna.InventoryItem{
		Kind:      "ClusterRole",
		Uid:       string(cr.UID),
		Name:      cr.Name,
		Namespace: "", // Cluster-scoped
		Labels:    labels,
		RawJson:   rawJSON,
		Timestamp: timestamp,
	}
}

// ClusterRoleBindingToInventoryItem converts a ClusterRoleBinding to InventoryItem
// Bug 5 Fix: Use eventType like PodToInventoryItem
func ClusterRoleBindingToInventoryItem(crb *rbacv1.ClusterRoleBinding, clusterID string, eventType watch.EventType) *fortuna.InventoryItem {
	labels := make(map[string]string)
	for k, v := range crb.Labels {
		labels[k] = v
	}

	// Bug 5 Fix: Use current time and add eventType metadata
	timestamp := time.Now().Unix()
	rawJSON := marshalObject(crb)
	var crbJSON map[string]interface{}
	if err := json.Unmarshal([]byte(rawJSON), &crbJSON); err == nil {
		if crbJSON == nil {
			crbJSON = make(map[string]interface{})
		}
		eventTypeStr := "Added"
		switch eventType {
		case watch.Added:
			eventTypeStr = "Added"
		case watch.Modified:
			eventTypeStr = "Modified"
		case watch.Deleted:
			eventTypeStr = "Deleted"
		}
		crbJSON["_fortuna_event_type"] = eventTypeStr
		crbJSON["_fortuna_timestamp"] = timestamp
		crbJSON["_fortuna_cluster_id"] = clusterID
		
		if rawJSONBytes, err := json.Marshal(crbJSON); err == nil {
			rawJSON = string(rawJSONBytes)
		}
	}

	return &fortuna.InventoryItem{
		Kind:      "ClusterRoleBinding",
		Uid:       string(crb.UID),
		Name:      crb.Name,
		Namespace: "", // Cluster-scoped
		Labels:    labels,
		RawJson:   rawJSON,
		Timestamp: timestamp,
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

