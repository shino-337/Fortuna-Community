package policy

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/cel-go/cel"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// Resource represents a Kubernetes resource to be evaluated
type Resource struct {
	Type      string                 `json:"type"`      // Pod, Deployment, etc.
	UID       string                 `json:"uid"`       // Resource UID
	Name      string                 `json:"name"`      // Resource name
	Namespace string                 `json:"namespace"` // Namespace
	ClusterID string                 `json:"clusterId"` // Cluster ID
	Spec      map[string]interface{} `json:"spec"`      // Resource spec (for CEL evaluation)
	Labels    map[string]string      `json:"labels"`    // Resource labels
}

// Violation represents a policy violation
type Violation struct {
	InstanceID   uint   `json:"instanceId"`
	InstanceName string `json:"instanceName"`
	TemplateID   string `json:"templateId"`
	TemplateName string `json:"templateName"`
	ResourceType string `json:"resourceType"`
	ResourceUID  string `json:"resourceUid"`
	ResourceName string `json:"resourceName"`
	Namespace    string `json:"namespace"`
	ClusterID    string `json:"clusterId"`
	Severity     string `json:"severity"`
	Action       string `json:"action"`
	Message      string `json:"message"`
}

// Evaluator evaluates resources against policies using CEL
type Evaluator struct {
	db     *gorm.DB
	celEnv *cel.Env

	// CEL program cache (in-memory)
	programCache    map[string]cel.Program // map[templateID:version]Program
	programCacheMux sync.RWMutex

	// Policy cache (in-memory)
	templates    map[string]*models.PolicyTemplate // map[templateID:version]Template
	templatesMux sync.RWMutex

	instances    map[string]*models.PolicyInstance // map[instanceName]Instance
	instancesMux sync.RWMutex

	// Refresh ticker
	refreshTicker *time.Ticker
	stopRefresh   chan struct{}
}

// NewEvaluator creates a new policy evaluator
func NewEvaluator(db *gorm.DB) (*Evaluator, error) {
	// Initialize CEL environment
	// Custom functions can be added later if needed
	env, err := cel.NewEnv(
		cel.Variable("resource", cel.DynType),
		cel.Variable("cluster", cel.StringType),
		cel.Variable("namespace", cel.StringType),
		cel.Variable("labels", cel.MapType(cel.StringType, cel.StringType)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL environment: %w", err)
	}

	e := &Evaluator{
		db:           db,
		celEnv:       env,
		programCache: make(map[string]cel.Program),
		templates:    make(map[string]*models.PolicyTemplate),
		instances:    make(map[string]*models.PolicyInstance),
		stopRefresh:  make(chan struct{}),
	}

	// Load templates and compile CEL programs
	if err := e.loadTemplates(); err != nil {
		return nil, fmt.Errorf("failed to load templates: %w", err)
	}

	// Load instances
	// Check if policy_instances table exists before loading
	tableExists := e.db.Migrator().HasTable(&models.PolicyInstance{})
	if tableExists {
		if err := e.loadInstances(); err != nil {
			log.Printf("Warning: Failed to load policy instances: %v (continuing with empty instances)", err)
			// Continue with empty instances - policies will be loaded from templates only
		}
	} else {
		log.Println("Warning: policy_instances table does not exist (migrations may not have completed). Continuing with templates only.")
	}

	// Start periodic refresh (every 5 minutes)
	e.startPeriodicRefresh(5 * time.Minute)

	log.Printf("[PolicyEvaluator] Initialized with %d templates and %d instances",
		len(e.templates), len(e.instances))

	return e, nil
}

// loadTemplates loads all policy templates and compiles CEL programs
func (e *Evaluator) loadTemplates() error {
	var templates []models.PolicyTemplate

	err := e.db.Where("deleted_at IS NULL").Find(&templates).Error
	if err != nil {
		return fmt.Errorf("failed to query templates: %w", err)
	}

	e.templatesMux.Lock()
	defer e.templatesMux.Unlock()

	e.programCacheMux.Lock()
	defer e.programCacheMux.Unlock()

	// Clear existing caches
	e.templates = make(map[string]*models.PolicyTemplate)
	e.programCache = make(map[string]cel.Program)

	for i := range templates {
		template := &templates[i]

		// Store template
		key := fmt.Sprintf("%s:%s", template.TemplateID, template.Version)
		e.templates[key] = template

		// Compile CEL expression
		ast, issues := e.celEnv.Compile(template.CELExpression)
		if issues != nil && issues.Err() != nil {
			log.Printf("[PolicyEvaluator] WARN: CEL compilation error for %s: %v",
				template.TemplateID, issues.Err())
			continue
		}

		prg, err := e.celEnv.Program(ast)
		if err != nil {
			log.Printf("[PolicyEvaluator] WARN: CEL program error for %s: %v",
				template.TemplateID, err)
			continue
		}

		// Cache program in memory
		e.programCache[key] = prg

		log.Printf("[PolicyEvaluator] Loaded template %s:%s and compiled CEL program",
			template.TemplateID, template.Version)
	}

	return nil
}

// loadInstances loads all policy instances into memory
func (e *Evaluator) loadInstances() error {
	var instances []models.PolicyInstance

	err := e.db.Where("deleted_at IS NULL AND enabled = true").Find(&instances).Error
	if err != nil {
		return fmt.Errorf("failed to query instances: %w", err)
	}

	e.instancesMux.Lock()
	defer e.instancesMux.Unlock()

	// Clear existing cache
	e.instances = make(map[string]*models.PolicyInstance)

	for i := range instances {
		instance := &instances[i]
		e.instances[instance.InstanceName] = instance
	}

	return nil
}

// EvaluateFast evaluates a resource against all applicable policies (FAST PATH - no I/O)
func (e *Evaluator) EvaluateFast(ctx context.Context, resource *Resource) ([]*Violation, error) {
	violations := []*Violation{}

	// Get applicable instances (fast: in-memory lookup)
	instances := e.getApplicableInstances(resource)

	for _, instance := range instances {
		// Check context timeout
		select {
		case <-ctx.Done():
			return violations, ctx.Err()
		default:
		}

		// Skip disabled instances
		if !instance.Enabled {
			continue
		}

		// Get template (fast: in-memory lookup)
		templateKey := fmt.Sprintf("%s:%s", instance.TemplateID, instance.TemplateVersion)

		e.templatesMux.RLock()
		template := e.templates[templateKey]
		e.templatesMux.RUnlock()

		if template == nil {
			log.Printf("[PolicyEvaluator] WARN: Template %s not found for instance %s",
				templateKey, instance.InstanceName)
			continue
		}

		// Get compiled CEL program (fast: in-memory lookup)
		e.programCacheMux.RLock()
		prg := e.programCache[templateKey]
		e.programCacheMux.RUnlock()

		if prg == nil {
			log.Printf("[PolicyEvaluator] WARN: CEL program not found for template %s",
				templateKey)
			continue
		}

		// Evaluate (fast: pre-compiled program)
		matched, err := e.evaluateCELProgram(prg, resource)
		if err != nil {
			log.Printf("[PolicyEvaluator] Evaluation error for %s: %v", instance.InstanceName, err)
			continue
		}

		if matched {
			// Create violation
			violation := &Violation{
				InstanceID:   uint(instance.ID),
				InstanceName: instance.InstanceName,
				TemplateID:   template.TemplateID,
				TemplateName: template.Name,
				ResourceType: resource.Type,
				ResourceUID:  resource.UID,
				ResourceName: resource.Name,
				Namespace:    resource.Namespace,
				ClusterID:    resource.ClusterID,
				Severity:     e.getSeverity(instance, template),
				Action:       e.getAction(instance, template),
				Message:      e.getMessage(instance, template),
			}

			violations = append(violations, violation)
		}
	}

	return violations, nil
}

// EvaluateInstance evaluates a single policy instance against a resource (prevents race conditions)
func (e *Evaluator) EvaluateInstance(
	ctx context.Context,
	instance *models.PolicyInstance,
	resource *Resource,
) (bool, error) {
	// Check context
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}

	// Check if instance is enabled
	if !instance.Enabled {
		return false, nil
	}

	// Check if instance matches scope
	if !e.matchesScope(instance, resource) {
		return false, nil
	}

	// Get template (fast: in-memory lookup)
	templateKey := fmt.Sprintf("%s:%s", instance.TemplateID, instance.TemplateVersion)

	e.templatesMux.RLock()
	template := e.templates[templateKey]
	e.templatesMux.RUnlock()

	if template == nil {
		log.Printf("[PolicyEvaluator] WARN: Template %s not found for instance %s",
			templateKey, instance.InstanceName)
		return false, fmt.Errorf("template not found: %s", templateKey)
	}

	// Get compiled CEL program (fast: in-memory lookup)
	e.programCacheMux.RLock()
	prg := e.programCache[templateKey]
	e.programCacheMux.RUnlock()

	if prg == nil {
		log.Printf("[PolicyEvaluator] WARN: CEL program not found for template %s",
			templateKey)
		return false, fmt.Errorf("CEL program not found for template: %s", templateKey)
	}

	// Evaluate (fast: pre-compiled program)
	matched, err := e.evaluateCELProgram(prg, resource)
	if err != nil {
		return false, fmt.Errorf("CEL evaluation error: %w", err)
	}

	return matched, nil
}

// evaluateCELProgram evaluates a pre-compiled CEL program
func (e *Evaluator) evaluateCELProgram(prg cel.Program, resource *Resource) (bool, error) {
	// Prepare variables
	vars := map[string]interface{}{
		"resource":  resource.Spec,
		"cluster":   resource.ClusterID,
		"namespace": resource.Namespace,
		"labels":    resource.Labels,
	}

	// Evaluate (FAST: no compilation)
	out, _, err := prg.Eval(vars)
	if err != nil {
		return false, fmt.Errorf("CEL evaluation error: %w", err)
	}

	// Check result
	result, ok := out.Value().(bool)
	if !ok {
		return false, fmt.Errorf("CEL expression must return boolean, got %T", out.Value())
	}

	// ✅ FIX #5: CEL Logic Verification
	// CEL expression format: "resource.securityContext.runAsNonRoot == true"
	// - Compliant resource: CEL returns true → !true = false (no violation) ✓
	// - Non-compliant resource: CEL returns false → !false = true (violation) ✓
	// This logic is CORRECT: CEL expressions should return true for compliant resources
	// The negation converts "is compliant" to "has violation"
	// Rule matched (violation detected) if result is FALSE (non-compliant)
	// CEL expression should return true for compliant resources
	return !result, nil
}

// getApplicableInstances returns instances applicable to resource
func (e *Evaluator) getApplicableInstances(resource *Resource) []*models.PolicyInstance {
	applicable := []*models.PolicyInstance{}

	e.instancesMux.RLock()
	defer e.instancesMux.RUnlock()

	for _, instance := range e.instances {
		if e.matchesScope(instance, resource) {
			applicable = append(applicable, instance)
		}
	}

	return applicable
}

// matchesScope checks if instance scope matches resource (FAST)
func (e *Evaluator) matchesScope(instance *models.PolicyInstance, resource *Resource) bool {
	matcher := NewScopeMatcher()
	return matcher.Matches(instance, resource)
}

// Helper functions
func (e *Evaluator) getSeverity(instance *models.PolicyInstance, template *models.PolicyTemplate) string {
	if instance.Severity != "" {
		return instance.Severity
	}
	return template.DefaultSeverity
}

func (e *Evaluator) getAction(instance *models.PolicyInstance, template *models.PolicyTemplate) string {
	if instance.Action != "" {
		return instance.Action
	}
	return template.DefaultAction
}

func (e *Evaluator) getMessage(instance *models.PolicyInstance, template *models.PolicyTemplate) string {
	if instance.CustomMessage != "" {
		return instance.CustomMessage
	}
	return fmt.Sprintf("Policy violation: %s", template.Name)
}

// ReloadTemplates reloads templates and recompiles CEL programs
func (e *Evaluator) ReloadTemplates() error {
	return e.loadTemplates()
}

// ReloadInstances reloads policy instances
func (e *Evaluator) ReloadInstances() error {
	// Check if policy_instances table exists before loading
	var tableExists bool
	if err := e.db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema='public' AND table_name='policy_instances')").Scan(&tableExists).Error; err != nil {
		return fmt.Errorf("failed to check policy_instances table: %w", err)
	}

	if !tableExists {
		log.Printf("[PolicyEvaluator] policy_instances table does not exist, skipping reload")
		return nil
	}

	return e.loadInstances()
}

// startPeriodicRefresh starts periodic refresh of templates and instances
func (e *Evaluator) startPeriodicRefresh(interval time.Duration) {
	e.refreshTicker = time.NewTicker(interval)

	go func() {
		for {
			select {
			case <-e.refreshTicker.C:
				log.Printf("[PolicyEvaluator] Refreshing templates and instances...")
				if err := e.ReloadTemplates(); err != nil {
					log.Printf("[PolicyEvaluator] Template reload error: %v", err)
				}
				if err := e.ReloadInstances(); err != nil {
					log.Printf("[PolicyEvaluator] Instance reload error: %v", err)
				}
			case <-e.stopRefresh:
				return
			}
		}
	}()
}

// Shutdown gracefully stops the evaluator
func (e *Evaluator) Shutdown() {
	if e.refreshTicker != nil {
		e.refreshTicker.Stop()
	}
	close(e.stopRefresh)
}

// Helper functions for scope matching
func matchesPatterns(patterns []string, value string) bool {
	for _, pattern := range patterns {
		if matchesPatternString(pattern, value) {
			return true
		}
	}
	return false
}

func matchesPatternString(pattern, value string) bool {
	// Simple wildcard matching: "prod-*" matches "prod-cluster-1"
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(value, prefix)
	}
	return pattern == value
}

func contains(slice []string, value string) bool {
	for _, s := range slice {
		if s == value {
			return true
		}
	}
	return false
}
