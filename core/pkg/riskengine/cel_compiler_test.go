package riskengine

import (
	"testing"
)

func TestCELCompiler(t *testing.T) {
	compiler, err := NewCELCompiler()
	if err != nil {
		t.Fatalf("Failed to create CEL compiler: %v", err)
	}

	t.Run("Simple CEL Expression", func(t *testing.T) {
		expression := "object.roleRef.name == 'cluster-admin'"

		data := map[string]interface{}{
			"object": map[string]interface{}{
				"roleRef": map[string]interface{}{
					"name": "cluster-admin",
				},
			},
		}

		result, err := compiler.Evaluate(expression, data)
		if err != nil {
			t.Fatalf("Failed to evaluate CEL expression: %v", err)
		}
		if !result {
			t.Error("Expected true, got false")
		}
	})

	t.Run("Complex CEL Expression", func(t *testing.T) {
		expression := "object.metadata.namespace == 'default' && object.spec.containers.size() > 0"

		data := map[string]interface{}{
			"object": map[string]interface{}{
				"metadata": map[string]interface{}{
					"namespace": "default",
				},
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"name": "test",
						},
					},
				},
			},
		}

		result, err := compiler.Evaluate(expression, data)
		if err != nil {
			t.Fatalf("Failed to evaluate CEL expression: %v", err)
		}
		if !result {
			t.Error("Expected true, got false")
		}
	})

	t.Run("CEL Compilation Error", func(t *testing.T) {
		expression := "object.roleRef.name =" // Invalid syntax

		_, err := compiler.Compile(expression)
		if err == nil {
			t.Error("Expected compilation error, got nil")
		}
	})

	t.Run("CEL Type Error", func(t *testing.T) {
		expression := "object.roleRef.name" // Returns string, not boolean

		data := map[string]interface{}{
			"object": map[string]interface{}{
				"roleRef": map[string]interface{}{
					"name": "cluster-admin",
				},
			},
		}

		_, err := compiler.Evaluate(expression, data)
		if err == nil {
			t.Error("Expected error for non-boolean expression, got nil")
		}
	})

	t.Run("CEL Cache", func(t *testing.T) {
		// Create a fresh compiler for cache test to avoid interference from other tests
		cacheCompiler, err := NewCELCompiler()
		if err != nil {
			t.Fatalf("Failed to create CEL compiler: %v", err)
		}

		expression := "object.name == 'test'"

		// First compile
		_, err = cacheCompiler.Compile(expression)
		if err != nil {
			t.Fatalf("Failed to compile: %v", err)
		}

		// Second compile should use cache
		_, err = cacheCompiler.Compile(expression)
		if err != nil {
			t.Fatalf("Failed to compile (cached): %v", err)
		}

		// Check cache size (should be 1 for this expression)
		cacheSize := cacheCompiler.GetCacheSize()
		if cacheSize < 1 {
			t.Errorf("Expected cache size >= 1, got %d", cacheSize)
		}

		// Clear cache
		cacheCompiler.ClearCache()
		cacheSize = cacheCompiler.GetCacheSize()
		if cacheSize != 0 {
			t.Errorf("Expected cache size 0 after clear, got %d", cacheSize)
		}
	})
}

func BenchmarkCELEvaluation(b *testing.B) {
	compiler, _ := NewCELCompiler()

	expression := "object.roleRef.name == 'cluster-admin'"
	data := map[string]interface{}{
		"object": map[string]interface{}{
			"roleRef": map[string]interface{}{
				"name": "cluster-admin",
			},
		},
	}

	// Compile once (cached)
	compiler.Compile(expression)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		compiler.Evaluate(expression, data)
	}
}

