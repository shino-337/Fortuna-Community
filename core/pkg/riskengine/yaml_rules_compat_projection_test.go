package riskengine

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Keep rollout-safe compatibility: any rule moved to object.securityState
// must retain object.fortuna fallback until migration parity is complete.
func TestYAMLRules_SecurityStateRulesKeepFortunaFallback(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	pkgDir := filepath.Dir(thisFile)
	repoRoot := filepath.Clean(filepath.Join(pkgDir, "..", "..", ".."))
	rulesDir := filepath.Join(repoRoot, "core", "rules")

	entries, err := os.ReadDir(rulesDir)
	if err != nil {
		t.Fatalf("read rules dir: %v", err)
	}

	var missingFallback []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		p := filepath.Join(rulesDir, e.Name())
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read rule %s: %v", e.Name(), err)
		}
		s := string(b)
		if strings.Contains(s, "object.securityState") && !strings.Contains(s, "object.fortuna") {
			missingFallback = append(missingFallback, e.Name())
		}
	}

	if len(missingFallback) > 0 {
		t.Fatalf("rules missing fortuna fallback while using securityState: %v", missingFallback)
	}
}
