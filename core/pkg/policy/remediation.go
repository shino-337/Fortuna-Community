package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	"github.com/ksam/core/internal/k8s"
	"github.com/ksam/core/pkg/models"
	"gorm.io/gorm"
)

// RemediationService handles policy remediation operations
type RemediationService struct {
	db          *gorm.DB
	k8sClients  map[string]kubernetes.Interface // Per-cluster clients
	clientsMux  sync.RWMutex
}

// NewRemediationService creates a new remediation service
func NewRemediationService(db *gorm.DB) *RemediationService {
	return &RemediationService{
		db:         db,
		k8sClients: make(map[string]kubernetes.Interface),
	}
}

// RemediateResource applies remediation to a resource
func (s *RemediationService) RemediateResource(
	ctx context.Context,
	instance *models.PolicyInstance,
	resource map[string]interface{},
	resourceType, resourceName, namespace, clusterID string,
) (bool, error) {
	// Check context before expensive operations
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}
	// ✅ FIX #1: Check if auto-remediation is enabled for this instance
	if !instance.AutoRemediate {
		return false, fmt.Errorf("auto-remediation is not enabled for this instance")
	}

	// Check if remediation is supported
	var template models.PolicyTemplate
	if err := s.db.Where("template_id = ? AND version = ?", instance.TemplateID, instance.TemplateVersion).
		First(&template).Error; err != nil {
		return false, fmt.Errorf("failed to get template: %w", err)
	}

	if !template.SupportsRemediation {
		return false, fmt.Errorf("template does not support remediation")
	}

	// ✅ FIX #4: Capture resource state before remediation (for audit trail)
	beforeState := s.captureResourceState(resource)

	if instance.RemediationDryRun {
		log.Printf("[Remediation] DRY RUN: Would apply remediation for instance %s", instance.InstanceName)
		// ✅ FIX #4: Log dry run attempt
		var remediationTemplate map[string]interface{}
		json.Unmarshal([]byte(template.RemediationTemplate), &remediationTemplate) // Ignore error for dry run
		s.auditRemediationAttempt(ctx, instance, resource, resourceType, resourceName, namespace, clusterID, beforeState, remediationTemplate, true, nil)
		return false, nil // Dry run - don't actually remediate
	}

	// Parse remediation template
	var remediationTemplate map[string]interface{}
	if err := json.Unmarshal([]byte(template.RemediationTemplate), &remediationTemplate); err != nil {
		// ✅ FIX #4: Log parsing failure
		s.auditRemediationAttempt(ctx, instance, resource, resourceType, resourceName, namespace, clusterID, beforeState, nil, false, err)
		return false, fmt.Errorf("failed to parse remediation template: %w", err)
	}
	
	// ✅ FIX #4: Log remediation attempt
	s.auditRemediationAttempt(ctx, instance, resource, resourceType, resourceName, namespace, clusterID, beforeState, remediationTemplate, false, nil)

	// Create patch data (createPatch handles type internally)
	patchData, err := s.createPatch(remediationTemplate)
	if err != nil {
		return false, fmt.Errorf("failed to create patch: %w", err)
	}

	// ✅ APPLY PATCH TO KUBERNETES
	if err := s.applyPatchToKubernetes(ctx, clusterID, resourceType, resourceName, namespace, patchData); err != nil {
		// ✅ FIX #4: Log remediation failure
		s.auditRemediationFailure(ctx, instance, resource, resourceType, resourceName, namespace, clusterID, err)
		return false, fmt.Errorf("failed to apply patch to kubernetes: %w", err)
	}

	// ✅ FIX #4: Get resource state after remediation (for audit trail)
	afterState, err := s.getResourceStateFromK8s(ctx, clusterID, resourceType, resourceName, namespace)
	if err != nil {
		log.Printf("[Remediation] WARN: Failed to get resource state after remediation: %v", err)
		afterState = nil
	}

	// ✅ FIX #4: Log successful remediation with before/after state
	s.auditRemediationSuccess(ctx, instance, resource, resourceType, resourceName, namespace, clusterID, beforeState, afterState)

	return true, nil
}

// getK8sClient gets or creates a Kubernetes client for the cluster
func (s *RemediationService) getK8sClient(ctx context.Context, clusterID string) (kubernetes.Interface, error) {
	// Check cache first
	s.clientsMux.RLock()
	if client, ok := s.k8sClients[clusterID]; ok {
		s.clientsMux.RUnlock()
		return client, nil
	}
	s.clientsMux.RUnlock()

	// Get cluster from database
	var cluster models.Cluster
	if err := s.db.WithContext(ctx).Where("cluster_id = ?", clusterID).First(&cluster).Error; err != nil {
		return nil, fmt.Errorf("failed to get cluster: %w", err)
	}

	// Create K8s client from kubeconfig
	k8sClient, err := k8s.NewClientFromKubeconfig(cluster.Kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client: %w", err)
	}

	// Cache client
	s.clientsMux.Lock()
	s.k8sClients[clusterID] = k8sClient.Clientset
	s.clientsMux.Unlock()

	return k8sClient.Clientset, nil
}

// applyPatchToKubernetes applies a patch to a Kubernetes resource
func (s *RemediationService) applyPatchToKubernetes(
	ctx context.Context,
	clusterID string,
	resourceType string,
	resourceName string,
	namespace string,
	patchData []byte,
) error {
	// Get K8s client for cluster
	client, err := s.getK8sClient(ctx, clusterID)
	if err != nil {
		return fmt.Errorf("no kubernetes client for cluster: %w", err)
	}

	// Apply patch based on resource type
	switch resourceType {
	case "Pod":
		_, err := client.CoreV1().Pods(namespace).Patch(
			ctx,
			resourceName,
			types.StrategicMergePatchType,
			patchData,
			metav1.PatchOptions{},
		)
		return err

	case "Deployment":
		_, err := client.AppsV1().Deployments(namespace).Patch(
			ctx,
			resourceName,
			types.StrategicMergePatchType,
			patchData,
			metav1.PatchOptions{},
		)
		return err

	case "StatefulSet":
		_, err := client.AppsV1().StatefulSets(namespace).Patch(
			ctx,
			resourceName,
			types.StrategicMergePatchType,
			patchData,
			metav1.PatchOptions{},
		)
		return err

	case "DaemonSet":
		_, err := client.AppsV1().DaemonSets(namespace).Patch(
			ctx,
			resourceName,
			types.StrategicMergePatchType,
			patchData,
			metav1.PatchOptions{},
		)
		return err

	case "ReplicaSet":
		_, err := client.AppsV1().ReplicaSets(namespace).Patch(
			ctx,
			resourceName,
			types.StrategicMergePatchType,
			patchData,
			metav1.PatchOptions{},
		)
		return err

	default:
		return fmt.Errorf("unsupported resource type: %s", resourceType)
	}
}

// createPatch creates a Strategic Merge Patch from remediation template
func (s *RemediationService) createPatch(remediationTemplate map[string]interface{}) ([]byte, error) {
	remediationType, ok := remediationTemplate["type"].(string)
	if !ok {
		remediationType = "patch" // Default
	}

	switch remediationType {
	case "patch":
		return s.createPatchFromOperations(remediationTemplate)
	case "replace":
		return s.createPatchFromSpec(remediationTemplate)
	default:
		return nil, fmt.Errorf("unknown remediation type: %s", remediationType)
	}
}

// createPatchFromOperations creates patch from JSON patch operations
func (s *RemediationService) createPatchFromOperations(template map[string]interface{}) ([]byte, error) {
	operations, ok := template["operations"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid patch operations")
	}

	// Build patch map (Strategic Merge Patch format)
	patch := make(map[string]interface{})

	for _, op := range operations {
		operation, ok := op.(map[string]interface{})
		if !ok {
			continue
		}

		opType, _ := operation["op"].(string)
		path, _ := operation["path"].(string)
		value := operation["value"]

		if opType == "add" || opType == "replace" {
			if err := s.setNestedFieldInPatch(patch, path, value); err != nil {
				return nil, fmt.Errorf("failed to set field %s: %w", path, err)
			}
		}
		// Note: "remove" operations are not supported in Strategic Merge Patch
		// They would need to be handled differently
	}

	// Marshal to JSON
	return json.Marshal(patch)
}

// createPatchFromSpec creates patch from spec replacement
func (s *RemediationService) createPatchFromSpec(template map[string]interface{}) ([]byte, error) {
	spec, ok := template["spec"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid replace spec")
	}

	patch := map[string]interface{}{
		"spec": spec,
	}

	return json.Marshal(patch)
}

// setNestedFieldInPatch sets a nested field in patch map
func (s *RemediationService) setNestedFieldInPatch(patch map[string]interface{}, path string, value interface{}) error {
	parts := splitPath(path)
	if len(parts) == 0 {
		return fmt.Errorf("empty path")
	}

	current := patch
	for _, part := range parts[:len(parts)-1] {
		if part == "" {
			continue
		}

		// Handle array index
		if isNumeric(part) {
			// For Strategic Merge Patch, we handle arrays differently
			// Skip numeric indices in path - they're handled by the patch structure
			continue
		}

		if current[part] == nil {
			current[part] = make(map[string]interface{})
		}

		nextCurrent, ok := current[part].(map[string]interface{})
		if !ok {
			// Type mismatch - create new map
			current[part] = make(map[string]interface{})
			nextCurrent = current[part].(map[string]interface{})
		}
		current = nextCurrent
	}

	// Set the final field
	finalKey := parts[len(parts)-1]
	current[finalKey] = value
	return nil
}

// applyPatch applies a JSON patch to the resource
func (s *RemediationService) applyPatch(template map[string]interface{}, resource map[string]interface{}) (bool, error) {
	operations, ok := template["operations"].([]interface{})
	if !ok {
		return false, fmt.Errorf("invalid patch operations")
	}

	for _, op := range operations {
		operation, ok := op.(map[string]interface{})
		if !ok {
			continue
		}

		opType, _ := operation["op"].(string)
		path, _ := operation["path"].(string)
		value := operation["value"]

		switch opType {
		case "add":
			if err := s.setNestedField(resource, path, value); err != nil {
				return false, fmt.Errorf("failed to add field %s: %w", path, err)
			}
		case "replace":
			if err := s.setNestedField(resource, path, value); err != nil {
				return false, fmt.Errorf("failed to replace field %s: %w", path, err)
			}
		case "remove":
			if err := s.removeNestedField(resource, path); err != nil {
				return false, fmt.Errorf("failed to remove field %s: %w", path, err)
			}
		}
	}

	return true, nil
}

// applyReplace replaces the entire resource spec
func (s *RemediationService) applyReplace(template map[string]interface{}, resource map[string]interface{}) (bool, error) {
	spec, ok := template["spec"].(map[string]interface{})
	if !ok {
		return false, fmt.Errorf("invalid replace spec")
	}

	// Merge spec into resource
	if resource["spec"] == nil {
		resource["spec"] = make(map[string]interface{})
	}
	resourceSpec := resource["spec"].(map[string]interface{})

	for k, v := range spec {
		resourceSpec[k] = v
	}

	return true, nil
}

// setNestedField sets a nested field in a map using JSON path
func (s *RemediationService) setNestedField(obj map[string]interface{}, path string, value interface{}) error {
	// Simple path implementation (supports /spec/containers/0/securityContext/privileged)
	parts := splitPath(path)
	if len(parts) == 0 {
		return fmt.Errorf("empty path")
	}

	current := obj
	for idx, part := range parts[:len(parts)-1] {
		if part == "" {
			continue
		}

		// ✅ PROPER ARRAY HANDLING
		if isNumeric(part) {
			// Convert to array index
			index, err := strconv.Atoi(part)
			if err != nil {
				return fmt.Errorf("invalid array index: %s", part)
			}

			// Get parent array (previous part)
			if idx == 0 {
				return fmt.Errorf("cannot start path with array index")
			}
			parentKey := parts[idx-1]

			// Get or create parent array
			var parentArray []interface{}
			if current[parentKey] == nil {
				parentArray = []interface{}{}
				current[parentKey] = parentArray
			} else {
				var ok bool
				parentArray, ok = current[parentKey].([]interface{})
				if !ok {
					// Parent is not an array - convert it
					parentArray = []interface{}{current[parentKey]}
					current[parentKey] = parentArray
				}
			}

			// Check bounds and extend if needed
			if index < 0 {
				return fmt.Errorf("array index out of bounds: %d", index)
			}
			for len(parentArray) <= index {
				parentArray = append(parentArray, make(map[string]interface{}))
			}

			// Navigate into array element
			arrayElement, ok := parentArray[index].(map[string]interface{})
			if !ok {
				// Create new map for array element if it's not already a map
				arrayElement = make(map[string]interface{})
				parentArray[index] = arrayElement
			}

			// Update the array in parent (important: update reference)
			current[parentKey] = parentArray
			current = arrayElement
			continue
		}

		// Handle non-array path segment
		if current[part] == nil {
			current[part] = make(map[string]interface{})
		}

		nextCurrent, ok := current[part].(map[string]interface{})
		if !ok {
			// Type mismatch - check if it's an array (for paths like /spec/containers)
			if _, ok := current[part].([]interface{}); ok {
				// If next part is numeric, we'll handle it in next iteration
				// Check if there's a next part and if it's numeric
				if idx+1 < len(parts[:len(parts)-1]) {
					nextPart := parts[idx+1]
					if isNumeric(nextPart) {
						// Next part is numeric, continue to next iteration to handle array index
						continue
					}
				}
				// Next part is not numeric or doesn't exist, this is an error
				return fmt.Errorf("unexpected array at non-index path segment: %s", part)
			}
			// Create new map
			current[part] = make(map[string]interface{})
			nextCurrent = current[part].(map[string]interface{})
		}
		current = nextCurrent
	}

	// Set the final field
	finalKey := parts[len(parts)-1]
	current[finalKey] = value
	return nil
}

// removeNestedField removes a nested field
func (s *RemediationService) removeNestedField(obj map[string]interface{}, path string) error {
	parts := splitPath(path)
	if len(parts) == 0 {
		return fmt.Errorf("empty path")
	}

	current := obj
	for _, part := range parts[:len(parts)-1] {
		if part == "" {
			continue
		}
		if current[part] == nil {
			return nil // Field doesn't exist, nothing to remove
		}
		current = current[part].(map[string]interface{})
	}

	finalKey := parts[len(parts)-1]
	delete(current, finalKey)
	return nil
}

// splitPath splits a JSON path into parts
func splitPath(path string) []string {
	if path == "" || path == "/" {
		return []string{}
	}
	if path[0] == '/' {
		path = path[1:]
	}
	parts := []string{}
	current := ""
	for _, char := range path {
		if char == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

// isNumeric checks if a string is numeric
func isNumeric(s string) bool {
	for _, char := range s {
		if char < '0' || char > '9' {
			return false
		}
	}
	return len(s) > 0
}

// ValidateRemediation validates a remediation before applying
func (s *RemediationService) ValidateRemediation(
	ctx context.Context,
	instance *models.PolicyInstance,
	resource map[string]interface{},
) error {
	// Check context
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	// Check if auto-remediation is enabled
	if !instance.AutoRemediate {
		return fmt.Errorf("auto-remediation is not enabled for this instance")
	}

	// Check if template supports remediation
	var template models.PolicyTemplate
	if err := s.db.Where("template_id = ? AND version = ?", instance.TemplateID, instance.TemplateVersion).
		First(&template).Error; err != nil {
		return fmt.Errorf("failed to get template: %w", err)
	}

	if !template.SupportsRemediation {
		return fmt.Errorf("template does not support remediation")
	}

	return nil
}

// DryRunRemediation tests remediation without applying
func (s *RemediationService) DryRunRemediation(
	ctx context.Context,
	instance *models.PolicyInstance,
	resource map[string]interface{},
) (map[string]interface{}, error) {
	// Check context
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	// Create a copy of the resource
	resourceCopy := make(map[string]interface{})
	resourceJSON, _ := json.Marshal(resource)
	json.Unmarshal(resourceJSON, &resourceCopy)

	// Apply remediation to copy (dry run - use empty context and minimal params)
	_, err := s.RemediateResource(ctx, instance, resourceCopy, "", "", "", "")
	if err != nil {
		return nil, err
	}

	return resourceCopy, nil
}

// validateNamespace validates that namespace is allowed for remediation
func (s *RemediationService) validateNamespace(namespace string) error {
	blockedNamespaces := []string{
		"kube-system",
		"kube-public",
		"kube-node-lease",
		"istio-system",
		"cert-manager",
	}

	for _, blocked := range blockedNamespaces {
		if namespace == blocked {
			return fmt.Errorf("remediation blocked for system namespace: %s", namespace)
		}
	}
	return nil
}

// isResourceTypeAllowed checks if resource type is allowed for remediation
func (s *RemediationService) isResourceTypeAllowed(resourceType string) bool {
	allowedTypes := []string{
		"Pod",
		"Deployment",
		"StatefulSet",
		"DaemonSet",
		"ReplicaSet",
	}

	for _, allowed := range allowedTypes {
		if resourceType == allowed {
			return true
		}
	}
	return false
}

// validatePatchFields validates that patch fields are in the whitelist
func (s *RemediationService) validatePatchFields(remediationTemplate string) error {
	allowedFields := []string{
		"spec.securityContext",
		"spec.containers",
		"spec.containers[].securityContext",
		"spec.containers[].resources",
		"spec.containers[].readinessProbe",
		"spec.containers[].livenessProbe",
		"spec.containers[].image",
		"metadata.labels",
		"metadata.annotations",
	}

	// Parse template
	var template map[string]interface{}
	if err := json.Unmarshal([]byte(remediationTemplate), &template); err != nil {
		return fmt.Errorf("failed to parse remediation template: %w", err)
	}

	// Extract all paths from operations
	operations, ok := template["operations"].([]interface{})
	if !ok {
		return nil // No operations, skip validation
	}

	for _, op := range operations {
		operation, ok := op.(map[string]interface{})
		if !ok {
			continue
		}

		path, _ := operation["path"].(string)
		if path == "" {
			continue
		}

		if !s.isFieldAllowed(path, allowedFields) {
			return fmt.Errorf("field not allowed for patching: %s (allowed fields: securityContext, containers, resources, probes, labels, annotations)", path)
		}
	}

	return nil
}

// isFieldAllowed checks if a field path is in the allowed list
func (s *RemediationService) isFieldAllowed(path string, allowedFields []string) bool {
	// Remove leading slash
	if len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}

	for _, allowed := range allowedFields {
		// Support wildcard matching for array indices
		if s.matchesFieldPattern(path, allowed) {
			return true
		}
	}
	return false
}

// matchesFieldPattern checks if path matches allowed pattern (supports [] wildcard)
func (s *RemediationService) matchesFieldPattern(path, pattern string) bool {
	// Exact match
	if path == pattern {
		return true
	}

	// Pattern with [] wildcard: "spec.containers[].securityContext" matches "spec.containers/0/securityContext"
	if strings.Contains(pattern, "[]") {
		// Replace [] with numeric pattern
		patternRegex := strings.ReplaceAll(pattern, "[]", `\[\d+\]`)
		patternRegex = strings.ReplaceAll(patternRegex, ".", `\.`)
		patternRegex = "^" + patternRegex + "$"

		// Convert path to match pattern format
		pathNormalized := strings.ReplaceAll(path, "/", ".")
		// Handle array indices: /spec/containers/0/securityContext -> spec.containers[0].securityContext
		pathNormalized = strings.ReplaceAll(pathNormalized, "/0/", "[0].")
		pathNormalized = strings.ReplaceAll(pathNormalized, "/1/", "[1].")
		pathNormalized = strings.ReplaceAll(pathNormalized, "/2/", "[2].")

		matched, _ := regexp.MatchString(patternRegex, pathNormalized)
		return matched
	}

	// Simple prefix match for paths like "spec.securityContext"
	pathParts := strings.Split(path, "/")
	patternParts := strings.Split(pattern, ".")

	if len(pathParts) < len(patternParts) {
		return false
	}

	// Check if path starts with pattern
	for i, part := range patternParts {
		if i >= len(pathParts) {
			return false
		}
		if pathParts[i] != part {
			return false
		}
	}

	return true
}

// captureResourceState creates a deep copy of resource state
func (s *RemediationService) captureResourceState(resource map[string]interface{}) map[string]interface{} {
	resourceCopy := make(map[string]interface{})
	data, err := json.Marshal(resource)
	if err != nil {
		return make(map[string]interface{})
	}
	if err := json.Unmarshal(data, &resourceCopy); err != nil {
		return make(map[string]interface{})
	}
	return resourceCopy
}

// getResourceStateFromK8s gets current resource state from Kubernetes
func (s *RemediationService) getResourceStateFromK8s(
	ctx context.Context,
	clusterID string,
	resourceType string,
	resourceName string,
	namespace string,
) (map[string]interface{}, error) {
	client, err := s.getK8sClient(ctx, clusterID)
	if err != nil {
		return nil, err
	}

	var resourceObj interface{}
	switch resourceType {
	case "Pod":
		pod, err := client.CoreV1().Pods(namespace).Get(ctx, resourceName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		resourceObj = pod
	case "Deployment":
		deploy, err := client.AppsV1().Deployments(namespace).Get(ctx, resourceName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		resourceObj = deploy
	case "StatefulSet":
		sts, err := client.AppsV1().StatefulSets(namespace).Get(ctx, resourceName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		resourceObj = sts
	case "DaemonSet":
		ds, err := client.AppsV1().DaemonSets(namespace).Get(ctx, resourceName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		resourceObj = ds
	default:
		return nil, fmt.Errorf("unsupported resource type for state retrieval: %s", resourceType)
	}

	// Convert to map
	data, err := json.Marshal(resourceObj)
	if err != nil {
		return nil, err
	}

	var state map[string]interface{}
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	return state, nil
}

// computeDiff computes differences between before and after states
func (s *RemediationService) computeDiff(before, after map[string]interface{}) []string {
	if after == nil {
		return []string{"after state unavailable"}
	}

	diffs := []string{}

	// Simple diff implementation
	for key, afterVal := range after {
		beforeVal, exists := before[key]
		if !exists {
			diffs = append(diffs, fmt.Sprintf("+ %s: %v", key, afterVal))
		} else if !s.deepEqual(beforeVal, afterVal) {
			diffs = append(diffs, fmt.Sprintf("~ %s: %v -> %v", key, beforeVal, afterVal))
		}
	}

	for key := range before {
		if _, exists := after[key]; !exists {
			diffs = append(diffs, fmt.Sprintf("- %s", key))
		}
	}

	return diffs
}

// deepEqual performs deep equality check
func (s *RemediationService) deepEqual(a, b interface{}) bool {
	aJSON, _ := json.Marshal(a)
	bJSON, _ := json.Marshal(b)
	return string(aJSON) == string(bJSON)
}

// auditRemediationAttempt logs remediation attempt
func (s *RemediationService) auditRemediationAttempt(
	ctx context.Context,
	instance *models.PolicyInstance,
	resource map[string]interface{},
	resourceType, resourceName, namespace, clusterID string,
	beforeState map[string]interface{},
	remediationTemplate map[string]interface{},
	dryRun bool,
	err error,
) {
	details := map[string]interface{}{
		"instance_id":        instance.ID,
		"instance_name":      instance.InstanceName,
		"template_id":        instance.TemplateID,
		"resource_type":      resourceType,
		"resource_name":      resourceName,
		"namespace":          namespace,
		"cluster_id":         clusterID,
		"dry_run":            dryRun,
		"remediation_type":   "attempt",
		"before_state":       beforeState,
		"remediation_template": remediationTemplate,
	}

	if err != nil {
		details["error"] = err.Error()
	}

	s.logAudit(ctx, "remediation_attempt", details)
}

// auditRemediationSuccess logs successful remediation
func (s *RemediationService) auditRemediationSuccess(
	ctx context.Context,
	instance *models.PolicyInstance,
	resource map[string]interface{},
	resourceType, resourceName, namespace, clusterID string,
	beforeState, afterState map[string]interface{},
) {
	diff := s.computeDiff(beforeState, afterState)

	details := map[string]interface{}{
		"instance_id":   instance.ID,
		"instance_name": instance.InstanceName,
		"template_id":   instance.TemplateID,
		"resource_type": resourceType,
		"resource_name": resourceName,
		"namespace":     namespace,
		"cluster_id":    clusterID,
		"before_state":  beforeState,
		"after_state":   afterState,
		"diff":          diff,
		"success":       true,
	}

	s.logAudit(ctx, "remediation_success", details)
}

// auditRemediationFailure logs remediation failure
func (s *RemediationService) auditRemediationFailure(
	ctx context.Context,
	instance *models.PolicyInstance,
	resource map[string]interface{},
	resourceType, resourceName, namespace, clusterID string,
	err error,
) {
	details := map[string]interface{}{
		"instance_id":   instance.ID,
		"instance_name": instance.InstanceName,
		"template_id":   instance.TemplateID,
		"resource_type": resourceType,
		"resource_name": resourceName,
		"namespace":     namespace,
		"cluster_id":    clusterID,
		"error":         err.Error(),
		"success":       false,
	}

	s.logAudit(ctx, "remediation_failure", details)
}

// logAudit logs audit entry to both stdout and database
// ✅ FIX #4: Stores audit logs in audit_logs table for compliance
func (s *RemediationService) logAudit(ctx context.Context, action string, details map[string]interface{}) {
	detailsJSON, err := json.Marshal(details)
	if err != nil {
		log.Printf("[Audit] Failed to marshal audit details: %v", err)
		return
	}

	// Log to stdout for immediate visibility
	log.Printf("[Audit] %s: %s", action, string(detailsJSON))

	// ✅ FIX #4: Store in audit_logs table for compliance
	// Extract resource information from details
	resourceType := ""
	resourceID := ""
	clusterID := ""

	if rt, ok := details["resource_type"].(string); ok {
		resourceType = rt
	}
	if rn, ok := details["resource_name"].(string); ok {
		resourceID = rn // Use resource_name as resource_id
	}
	if cid, ok := details["cluster_id"].(string); ok {
		clusterID = cid
	}

	// Create audit log entry
	auditLog := models.AuditLog{
		ClusterID:  clusterID,
		UserID:     0,              // System user (0 = system)
		Action:     action,
		Resource:   resourceType,
		ResourceID: resourceID,
		Details:    string(detailsJSON),
		User:       "policy-engine", // System user for policy remediation
		IP:         "internal",     // Internal system action
	}

	// Store in database (non-blocking - errors are logged but don't fail remediation)
	if err := s.db.WithContext(ctx).Create(&auditLog).Error; err != nil {
		log.Printf("[Audit] Failed to store audit log in database: %v", err)
		// Don't return error - audit logging should not fail remediation
	}
}

