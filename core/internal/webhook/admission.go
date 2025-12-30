package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	"github.com/fortuna/core/pkg/messaging"
	"github.com/fortuna/core/pkg/metrics"
	"github.com/fortuna/core/pkg/policy"
	"gorm.io/gorm"
)

var (
	runtimeScheme = runtime.NewScheme()
	codecs        = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codecs.UniversalDeserializer()
)

func init() {
	_ = corev1.AddToScheme(runtimeScheme)
}

// AdmissionWebhook handles Kubernetes admission webhook requests
// Phase 2.7: Async architecture with fast path (<100ms) and slow path (async)
type AdmissionWebhook struct {
	evaluator *policy.Evaluator
	eventBus  *messaging.NATSClient
	db        *gorm.DB
}

// NewAdmissionWebhook creates a new admission webhook handler
func NewAdmissionWebhook(db *gorm.DB, evaluator *policy.Evaluator, eventBus *messaging.NATSClient) *AdmissionWebhook {
	return &AdmissionWebhook{
		evaluator: evaluator,
		eventBus:  eventBus,
		db:        db,
	}
}

// Handle processes admission webhook requests (FAST PATH ONLY)
// Phase 2.7: Fast path - <100ms, no I/O blocking operations
func (w *AdmissionWebhook) Handle(resp http.ResponseWriter, req *http.Request) {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		durationMs := float64(duration.Nanoseconds()) / 1e6 // Convert to milliseconds
		
		// Record admission latency metric
		metrics.AdmissionLatencyMs.WithLabelValues("validate").Observe(durationMs)
		
		if duration > 100*time.Millisecond {
			log.Printf("[Webhook] ⚠️  SLOW request: %v", duration)
		} else {
			log.Printf("[Webhook] ✅ Fast path: %v", duration)
		}
	}()

	// Parse admission request
	var admissionReview admissionv1.AdmissionReview
	if err := json.NewDecoder(req.Body).Decode(&admissionReview); err != nil {
		log.Printf("[Webhook] Failed to decode admission request: %v", err)
		http.Error(resp, fmt.Sprintf("Failed to decode request: %v", err), http.StatusBadRequest)
		return
	}

	admissionRequest := admissionReview.Request
	if admissionRequest == nil {
		log.Printf("[Webhook] Admission request is nil")
		metrics.AdmissionErrorsTotal.WithLabelValues("parse_error").Inc()
		http.Error(resp, "Admission request is nil", http.StatusBadRequest)
		return
	}

	// Parse resource (fast: pure JSON parsing)
	resource, err := w.parseResource(admissionRequest)
	if err != nil {
		log.Printf("[Webhook] Failed to parse resource: %v", err)
		metrics.AdmissionErrorsTotal.WithLabelValues("parse_error").Inc()
		// Fail open: allow resource if parsing fails
		w.sendResponse(resp, &admissionReview, true, fmt.Sprintf("Failed to parse resource: %v", err))
		return
	}

	// Context with strict timeout (100ms for fast path)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Evaluate policies (FAST: in-memory only, no I/O)
	celStart := time.Now()
	violations, err := w.evaluator.EvaluateFast(ctx, resource)
	celDuration := time.Since(celStart)
	celDurationMs := float64(celDuration.Nanoseconds()) / 1e6
	
	// Record CEL evaluation time (use "unknown" if template ID not available)
	metrics.CELEvaluationMs.WithLabelValues("unknown").Observe(celDurationMs)
	
	if err != nil {
		// On error, fail open (allow) with logging
		log.Printf("[Webhook] Evaluation error: %v", err)
		metrics.AdmissionErrorsTotal.WithLabelValues("eval_error").Inc()
		w.sendResponse(resp, &admissionReview, true, fmt.Sprintf("Evaluation error: %v", err))
		return
	}

	// Check for blocking violations
	hasBlockingViolation := false
	var blockMessage string

	for _, v := range violations {
		if v.Action == "block" {
			hasBlockingViolation = true
			blockMessage = fmt.Sprintf("Policy violation: %s - %s", v.InstanceName, v.Message)
			break
		}
	}

	// ✅ Phase 2.7: Emit event ASYNC (non-blocking) for slow path
	if len(violations) > 0 {
		// Fire-and-forget: send to event bus (non-blocking)
		w.publishViolationEvent(violations, resource, admissionRequest)
	}

	// Return decision (fast: no DB, no external calls)
	if hasBlockingViolation {
		log.Printf("[Webhook] 🚫 BLOCKED: %s", blockMessage)
		metrics.AdmissionDeniedCount.WithLabelValues("policy_violation").Inc()
		w.sendResponse(resp, &admissionReview, false, blockMessage)
		return
	}

	// Allow resource
	log.Printf("[Webhook] ✅ ALLOWED: %s/%s in %s", resource.Type, resource.Name, resource.Namespace)
	metrics.AdmissionAllowedCount.WithLabelValues(resource.Type).Inc()
	w.sendResponse(resp, &admissionReview, true, "Policy check passed")
}

// parseResource parses Kubernetes resource from admission request
func (w *AdmissionWebhook) parseResource(req *admissionv1.AdmissionRequest) (*policy.Resource, error) {
	var obj runtime.Object
	var gvk schema.GroupVersionKind

	// Determine object type from request
	switch req.Kind.Kind {
	case "Pod":
		obj = &corev1.Pod{}
		gvk = schema.GroupVersionKind{
			Group:   corev1.SchemeGroupVersion.Group,
			Version: corev1.SchemeGroupVersion.Version,
			Kind:    "Pod",
		}
	case "Deployment":
		// For now, only handle Pods
		return nil, fmt.Errorf("unsupported resource type: %s", req.Kind.Kind)
	default:
		return nil, fmt.Errorf("unsupported resource type: %s", req.Kind.Kind)
	}

	// Decode object
	_, _, err := deserializer.Decode(req.Object.Raw, &gvk, obj)
	if err != nil {
		return nil, fmt.Errorf("failed to decode object: %w", err)
	}

	// Extract resource information
	var resourceName, namespace, uid string
	var spec map[string]interface{}
	var labels map[string]string

	switch pod := obj.(type) {
	case *corev1.Pod:
		resourceName = pod.Name
		namespace = pod.Namespace
		uid = string(pod.UID)
		labels = pod.Labels

		// Convert Pod to map for CEL evaluation
		podJSON, err := json.Marshal(pod)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal pod: %w", err)
		}

		var podMap map[string]interface{}
		if err := json.Unmarshal(podJSON, &podMap); err != nil {
			return nil, fmt.Errorf("failed to unmarshal pod: %w", err)
		}

		// Extract spec for CEL evaluation
		if specObj, ok := podMap["spec"].(map[string]interface{}); ok {
			spec = specObj
		} else {
			spec = make(map[string]interface{})
		}
	default:
		return nil, fmt.Errorf("unsupported object type: %T", obj)
	}

	// Extract cluster ID from request (if available in annotations or labels)
	clusterID := "default"
	if req.UID != "" {
		// Use request UID as cluster identifier (can be enhanced)
		clusterID = string(req.UID)[:8] // Use first 8 chars
	}

	return &policy.Resource{
		Type:      req.Kind.Kind,
		UID:       uid,
		Name:      resourceName,
		Namespace: namespace,
		ClusterID: clusterID,
		Spec:      spec,
		Labels:    labels,
	}, nil
}

// publishViolationEvent publishes violation event to NATS (non-blocking)
// Phase 2.7: Slow path - async processing
func (w *AdmissionWebhook) publishViolationEvent(
	violations []*policy.Violation,
	resource *policy.Resource,
	req *admissionv1.AdmissionRequest,
) {
	if w.eventBus == nil {
		log.Printf("[Webhook] ⚠️  Event bus not available, skipping async processing")
		return
	}

	// Create violation event
	event := map[string]interface{}{
		"type":       "policy.violation.detected",
		"timestamp":  time.Now().Unix(),
		"violations": violations,
		"resource": map[string]interface{}{
			"type":      resource.Type,
			"uid":       resource.UID,
			"name":      resource.Name,
			"namespace": resource.Namespace,
			"clusterId": resource.ClusterID,
		},
		"request": map[string]interface{}{
			"uid":      string(req.UID),
			"kind":     req.Kind.String(),
			"operation": string(req.Operation),
		},
	}

	// Publish to NATS (non-blocking, fire-and-forget)
	eventJSON, err := json.Marshal(event)
	if err != nil {
		log.Printf("[Webhook] Failed to marshal violation event: %v", err)
		metrics.EventPublishFailCount.WithLabelValues("violation_detected").Inc()
		return
	}

	// Publish to policy.violation.detected subject
	if err := w.eventBus.Publish("fortuna.policy.violation.detected", eventJSON); err != nil {
		log.Printf("[Webhook] Failed to publish violation event: %v", err)
		metrics.EventPublishFailCount.WithLabelValues("violation_detected").Inc()
		// Don't fail webhook if event publish fails
	} else {
		log.Printf("[Webhook] 📤 Published violation event to slow path: %d violations", len(violations))
		metrics.EventPublishSuccessCount.WithLabelValues("violation_detected").Inc()
	}
}

// sendResponse sends admission response
func (w *AdmissionWebhook) sendResponse(resp http.ResponseWriter, review *admissionv1.AdmissionReview, allowed bool, message string) {
	response := &admissionv1.AdmissionResponse{
		UID:     review.Request.UID,
		Allowed: allowed,
	}

	if !allowed {
		response.Result = &metav1.Status{
			Message: message,
			Code:    http.StatusForbidden,
		}
	}

	review.Response = response
	review.Request = nil // Clear request to reduce response size

	resp.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(resp).Encode(review); err != nil {
		log.Printf("[Webhook] Failed to encode response: %v", err)
		http.Error(resp, fmt.Sprintf("Failed to encode response: %v", err), http.StatusInternalServerError)
		return
	}
}

// HealthCheck handles health check requests
func (w *AdmissionWebhook) HealthCheck(resp http.ResponseWriter, req *http.Request) {
	resp.WriteHeader(http.StatusOK)
	resp.Write([]byte("OK"))
}

