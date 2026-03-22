package extractor

import "testing"

func TestGoModParser_Parse_DiscoverGoSumGoModFromAppRoot_Dedup(t *testing.T) {
	fs := NewFilesystem()
	fs.files["/app/go.sum"] = []byte("github.com/gin-gonic/gin v1.2.3 h1:hash\n")
	fs.files["/app/go.mod"] = []byte("module example.com/x\nrequire github.com/gin-gonic/gin v1.2.3\n")

	pkgs, err := NewGoModParser().Parse(fs)
	if err != nil {
		t.Fatal(err)
	}

	// Expect exactly 1 unique package for gin@v1.2.3.
	var count int
	for _, p := range pkgs {
		if p.Name == "github.com/gin-gonic/gin" && p.Version == "v1.2.3" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 gin@v1.2.3, got %d packages: %+v", count, pkgs)
	}
}

func TestGoModParser_Parse_FallbackFindPathsBySuffix(t *testing.T) {
	fs := NewFilesystem()
	// No go.mod/go.sum at common roots; only nested path.
	fs.files["/deep/workspace/project/go.mod"] = []byte("module example.com/x\nrequire github.com/pkg/errors v0.9.1\n")

	pkgs, err := NewGoModParser().Parse(fs)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, p := range pkgs {
		if p.Name == "github.com/pkg/errors" && p.Version == "v0.9.1" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected github.com/pkg/errors@v0.9.1 from suffix fallback, got %+v", pkgs)
	}
}

