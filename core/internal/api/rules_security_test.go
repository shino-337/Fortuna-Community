package api

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveRuleFileRejectsTraversalIDs(t *testing.T) {
	dir := t.TempDir()
	badIDs := []string{
		"../../etc/evil",
		"..%2F..%2Fetc%2Fevil",
		"..",
		"evil/../../x",
		`evil\..\x`,
		"cis..5",
		"",
	}

	for _, id := range badIDs {
		if path, err := resolveRuleFile(dir, id); err == nil {
			t.Fatalf("resolveRuleFile(%q) succeeded with %q", id, path)
		}
	}
}

func TestResolveRuleFileAllowsCatalogRuleIDs(t *testing.T) {
	dir := t.TempDir()
	ids := []string{
		"cis-5.1.4",
		"rbac-pods-exec-attach-portforward",
		"runtime_suspicious_exec_signal",
	}

	for _, id := range ids {
		path, err := resolveRuleFile(dir, id)
		if err != nil {
			t.Fatalf("resolveRuleFile(%q) failed: %v", id, err)
		}
		if !strings.HasPrefix(path, filepath.Clean(dir)+string(filepath.Separator)) {
			t.Fatalf("path %q escaped %q", path, dir)
		}
		if filepath.Base(path) != id+".yaml" {
			t.Fatalf("unexpected file name for %q: %q", id, path)
		}
	}
}
