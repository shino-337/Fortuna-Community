package riskengine

import (
	"fmt"
	"sync"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	"github.com/google/cel-go/common/types"
)

// CELCompiler compiles and caches CEL expressions
type CELCompiler struct {
	env           *cel.Env
	programs      map[string]cel.Program
	programsMutex sync.RWMutex
}

// NewCELCompiler creates a new CEL compiler with Fortuna-specific environment
func NewCELCompiler() (*CELCompiler, error) {
	// Create CEL environment with custom variables for Kubernetes objects
	env, err := cel.NewEnv(
		// Core Kubernetes object variables
		cel.Declarations(
			decls.NewVar("object", decls.NewMapType(decls.String, decls.Dyn)),
			decls.NewVar("metadata", decls.NewMapType(decls.String, decls.Dyn)),
			decls.NewVar("spec", decls.NewMapType(decls.String, decls.Dyn)),
			decls.NewVar("status", decls.NewMapType(decls.String, decls.Dyn)),
		),
		// Note: Custom functions can be added later if needed
		// For now, CEL standard functions are sufficient
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create CEL environment: %w", err)
	}

	return &CELCompiler{
		env:      env,
		programs: make(map[string]cel.Program),
	}, nil
}

// Compile compiles a CEL expression and caches the result
func (c *CELCompiler) Compile(expression string) (cel.Program, error) {
	// Check cache first
	c.programsMutex.RLock()
	if prog, exists := c.programs[expression]; exists {
		c.programsMutex.RUnlock()
		return prog, nil
	}
	c.programsMutex.RUnlock()

	// Compile expression
	ast, issues := c.env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("CEL compilation error: %w", issues.Err())
	}

	// Create program
	prog, err := c.env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("CEL program creation error: %w", err)
	}

	// Cache compiled program
	c.programsMutex.Lock()
	c.programs[expression] = prog
	c.programsMutex.Unlock()

	return prog, nil
}

// Evaluate evaluates a CEL expression against input data
func (c *CELCompiler) Evaluate(expression string, data map[string]interface{}) (bool, error) {
	// Get or compile program
	prog, err := c.Compile(expression)
	if err != nil {
		return false, err
	}

	// Prepare input for CEL evaluation
	// CEL expects a map with variable names as keys
	celInput := make(map[string]interface{})
	
	// Add object as root
	if obj, ok := data["object"].(map[string]interface{}); ok {
		celInput["object"] = obj
		
		// Extract metadata, spec, status if present
		if metadata, ok := obj["metadata"].(map[string]interface{}); ok {
			celInput["metadata"] = metadata
		}
		if spec, ok := obj["spec"].(map[string]interface{}); ok {
			celInput["spec"] = spec
		}
		if status, ok := obj["status"].(map[string]interface{}); ok {
			celInput["status"] = status
		}
	} else {
		// If no "object" key, use data directly as object
		celInput["object"] = data
		if metadata, ok := data["metadata"].(map[string]interface{}); ok {
			celInput["metadata"] = metadata
		}
		if spec, ok := data["spec"].(map[string]interface{}); ok {
			celInput["spec"] = spec
		}
		if status, ok := data["status"].(map[string]interface{}); ok {
			celInput["status"] = status
		}
	}

	// Evaluate
	result, _, err := prog.Eval(celInput)
	if err != nil {
		return false, fmt.Errorf("CEL evaluation error: %w", err)
	}

	// Convert result to boolean
	val := result.Value()
	switch v := val.(type) {
	case bool:
		return v, nil
	case types.Bool:
		return bool(v), nil
	default:
		return false, fmt.Errorf("CEL expression must return boolean, got %T", val)
	}
}

// ClearCache clears the compiled program cache (useful for hot-reload)
func (c *CELCompiler) ClearCache() {
	c.programsMutex.Lock()
	defer c.programsMutex.Unlock()
	c.programs = make(map[string]cel.Program)
}

// GetCacheSize returns the number of cached programs
func (c *CELCompiler) GetCacheSize() int {
	c.programsMutex.RLock()
	defer c.programsMutex.RUnlock()
	return len(c.programs)
}

