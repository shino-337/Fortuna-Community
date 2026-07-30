package extractor

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"strings"
)

// TestGoBinaryParser_NoFiles ensures parser returns empty when no binaries are present.
func TestGoBinaryParser_NoFiles(t *testing.T) {
	fs := NewFilesystem()
	p := NewGoBinaryParser()
	pkgs, err := p.Parse(fs)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(pkgs) != 0 {
		t.Fatalf("expected 0 packages, got %d", len(pkgs))
	}
}

// TestGoBinaryParser_ParsesRealGoBinary verifies that GoBinaryParser detects a real Go binary
// and extracts at least one Go module with non-empty name and version (P2-1).
func TestGoBinaryParser_ParsesRealGoBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Go binary parser integration test in short mode")
	}

	// Build a small Go binary from testdata package using the go tool.
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "gobinary-test")

	cmd := exec.Command("go", "build", "-o", binPath, "./pkg/sbom/extractor/testdata/gobinary")
	// Run from agent module root (../../.. from this package: pkg/sbom/extractor -> agent).
	cmd.Dir = "../../.."
	cmd.Env = append(os.Environ(), "GO111MODULE=on")

	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build test Go binary: %v (output: %s)", err, string(out))
	}

	data, err := os.ReadFile(binPath)
	if err != nil {
		t.Fatalf("failed to read built binary: %v", err)
	}

	fs := NewFilesystem()
	// Place the binary under /bin so it is within GoBinaryParser's search paths.
	fs.files["/bin/gobinary-test"] = data

	p := NewGoBinaryParser()
	pkgs, err := p.Parse(fs)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	// Ensure we have at least one Go dependency package with non-empty name and version.
	var depFound, mainFound bool
	for _, pkg := range pkgs {
		if pkg.Type == "go-binary" && pkg.Source == "gobinary-main" && pkg.Name != "" && pkg.Version != "" {
			mainFound = true
			if pkg.PURL == "" || !strings.HasPrefix(pkg.PURL, "pkg:go/") {
				t.Fatalf("expected main package to have Go PURL pkg:go/...; got PURL=%q pkg=%+v", pkg.PURL, pkg)
			}
		}
		if pkg.Type == "go" && pkg.Source == "gobinary" && pkg.Name != "" && pkg.Version != "" {
			depFound = true
			if pkg.PURL == "" || !strings.HasPrefix(pkg.PURL, "pkg:go/") {
				t.Fatalf("expected dep package to have Go PURL pkg:go/...; got PURL=%q pkg=%+v", pkg.PURL, pkg)
			}
		}
	}
	if !mainFound {
		t.Fatalf("expected a main go-binary package from gobinary parser; got %+v", pkgs)
	}
	if !depFound {
		t.Fatalf("expected at least one Go dependency package from gobinary parser; got %+v", pkgs)
	}
}

// TestIsPseudoVersion verifies pseudo-version detection for Go modules.
func TestIsPseudoVersion(t *testing.T) {
	cases := []struct {
		version string
		want    bool
	}{
		{"v0.0.0-20230912-abcdef", true},
		{"v1.2.3-0.20230912-abcdef", true},
		{"v1.2.3", false},
		{"1.2.3", false},
		{"", false},
	}
	for _, c := range cases {
		got := isPseudoVersion(c.version)
		if got != c.want {
			t.Errorf("isPseudoVersion(%q) = %v, want %v", c.version, got, c.want)
		}
	}
}

