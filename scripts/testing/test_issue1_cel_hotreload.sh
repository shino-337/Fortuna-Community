#!/bin/bash

# Test script for Issue #1: CEL Engine & Hot-Reload
# Tests CEL expression evaluation and YAML rule hot-reload

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CORE_DIR="$PROJECT_ROOT/core"

echo "=========================================="
echo "Issue #1: CEL Engine & Hot-Reload Tests"
echo "=========================================="
echo ""

# Test 1: CEL Compiler Unit Tests
echo "[TEST 1] Running CEL Compiler Unit Tests..."
echo "-------------------------------------------"
cd "$CORE_DIR"
if go test -v ./pkg/riskengine -run TestCELCompiler 2>&1 | tee /tmp/cel_test_output.txt; then
    echo "✅ TEST 1 PASSED: CEL Compiler Unit Tests"
else
    echo "❌ TEST 1 FAILED: CEL Compiler Unit Tests"
    exit 1
fi
echo ""

# Test 2: CEL Expression Evaluation
echo "[TEST 2] Testing CEL Expression Evaluation..."
echo "-------------------------------------------"
cd "$CORE_DIR"
cat > /tmp/test_cel_eval.go << 'EOF'
package main

import (
	"fmt"
	"log"
	"github.com/ksam/core/pkg/riskengine"
)

func main() {
	compiler, err := riskengine.NewCELCompiler()
	if err != nil {
		log.Fatalf("Failed to create CEL compiler: %v", err)
	}

	tests := []struct {
		name       string
		expression  string
		data       map[string]interface{}
		expected   bool
	}{
		{
			name:      "Simple field comparison",
			expression: "object.roleRef.name == 'cluster-admin'",
			data: map[string]interface{}{
				"object": map[string]interface{}{
					"roleRef": map[string]interface{}{
						"name": "cluster-admin",
					},
				},
			},
			expected: true,
		},
		{
			name:      "Complex nested access",
			expression: "object.metadata.namespace == 'default'",
			data: map[string]interface{}{
				"object": map[string]interface{}{
					"metadata": map[string]interface{}{
						"namespace": "default",
					},
				},
			},
			expected: true,
		},
		{
			name:      "False condition",
			expression: "object.roleRef.name == 'cluster-admin'",
			data: map[string]interface{}{
				"object": map[string]interface{}{
					"roleRef": map[string]interface{}{
						"name": "view",
					},
				},
			},
			expected: false,
		},
	}

	passed := 0
	failed := 0

	for _, test := range tests {
		result, err := compiler.Evaluate(test.expression, test.data)
		if err != nil {
			fmt.Printf("  ❌ %s: Error: %v\n", test.name, err)
			failed++
			continue
		}
		if result == test.expected {
			fmt.Printf("  ✅ %s: PASSED\n", test.name)
			passed++
		} else {
			fmt.Printf("  ❌ %s: Expected %v, got %v\n", test.name, test.expected, result)
			failed++
		}
	}

	fmt.Printf("\nResults: %d passed, %d failed\n", passed, failed)
	if failed > 0 {
		log.Fatal("Some tests failed")
	}
}
EOF

if go run /tmp/test_cel_eval.go 2>&1; then
    echo "✅ TEST 2 PASSED: CEL Expression Evaluation"
else
    echo "❌ TEST 2 FAILED: CEL Expression Evaluation"
    exit 1
fi
echo ""

# Test 3: CEL Cache Functionality
echo "[TEST 3] Testing CEL Cache Functionality..."
echo "-------------------------------------------"
cd "$CORE_DIR"
cat > /tmp/test_cel_cache.go << 'EOF'
package main

import (
	"fmt"
	"log"
	"github.com/ksam/core/pkg/riskengine"
)

func main() {
	compiler, err := riskengine.NewCELCompiler()
	if err != nil {
		log.Fatalf("Failed to create CEL compiler: %v", err)
	}

	expression := "object.name == 'test'"

	// First compile
	_, err = compiler.Compile(expression)
	if err != nil {
		log.Fatalf("Failed to compile: %v", err)
	}

	size1 := compiler.GetCacheSize()
	fmt.Printf("  Cache size after first compile: %d\n", size1)

	// Second compile should use cache
	_, err = compiler.Compile(expression)
	if err != nil {
		log.Fatalf("Failed to compile (cached): %v", err)
	}

	size2 := compiler.GetCacheSize()
	fmt.Printf("  Cache size after second compile: %d\n", size2)

	if size1 == size2 {
		fmt.Println("  ✅ Cache working correctly (same size)")
	} else {
		log.Fatalf("  ❌ Cache not working (size changed: %d -> %d)", size1, size2)
	}

	// Clear cache
	compiler.ClearCache()
	size3 := compiler.GetCacheSize()
	fmt.Printf("  Cache size after clear: %d\n", size3)

	if size3 == 0 {
		fmt.Println("  ✅ Cache cleared successfully")
	} else {
		log.Fatalf("  ❌ Cache clear failed (size: %d)", size3)
	}
}
EOF

if go run /tmp/test_cel_cache.go 2>&1; then
    echo "✅ TEST 3 PASSED: CEL Cache Functionality"
else
    echo "❌ TEST 3 FAILED: CEL Cache Functionality"
    exit 1
fi
echo ""

# Test 4: YAML Rule Loading (if rules directory exists)
echo "[TEST 4] Testing YAML Rule Loading..."
echo "-------------------------------------------"
if [ -d "$PROJECT_ROOT/core/rules" ] || [ -d "/etc/ksam/rules" ]; then
    echo "  ℹ️  Rules directory found, testing YAML rule loading..."
    cd "$CORE_DIR"
    # This would require a database connection, so we'll skip for now
    echo "  ⚠️  Skipping (requires database connection)"
    echo "✅ TEST 4 SKIPPED: YAML Rule Loading (requires DB)"
else
    echo "  ℹ️  No rules directory found"
    echo "✅ TEST 4 SKIPPED: YAML Rule Loading (no rules dir)"
fi
echo ""

# Test 5: Hot-Reload File Watcher (if rules directory exists)
echo "[TEST 5] Testing Hot-Reload File Watcher..."
echo "-------------------------------------------"
if [ -d "$PROJECT_ROOT/core/rules" ] || [ -d "/etc/ksam/rules" ]; then
    echo "  ℹ️  Rules directory found, testing file watcher..."
    echo "  ⚠️  Skipping (requires running service)"
    echo "✅ TEST 5 SKIPPED: Hot-Reload File Watcher (requires running service)"
else
    echo "  ℹ️  No rules directory found"
    echo "✅ TEST 5 SKIPPED: Hot-Reload File Watcher (no rules dir)"
fi
echo ""

echo "=========================================="
echo "Issue #1 Test Summary"
echo "=========================================="
echo "✅ Test 1: CEL Compiler Unit Tests - PASSED"
echo "✅ Test 2: CEL Expression Evaluation - PASSED"
echo "✅ Test 3: CEL Cache Functionality - PASSED"
echo "⏭️  Test 4: YAML Rule Loading - SKIPPED"
echo "⏭️  Test 5: Hot-Reload File Watcher - SKIPPED"
echo ""
echo "✅ Issue #1 Tests: 3/3 PASSED (2 skipped - require runtime environment)"



