package extractor

import "testing"

// TestNpmParser_parsePackageLock_V2EmptyVersionsFallsBackToV1 verifies A6: when lockfile v2 has
// `packages` populated but no usable versions, we still parse top-level v1 `dependencies`.
func TestNpmParser_parsePackageLock_V2EmptyVersionsFallsBackToV1(t *testing.T) {
	p := NewNpmParser()
	// lockfileVersion 2 shape: packages present but inner versions empty → v2 yields 0 packages.
	lock := `{
  "lockfileVersion": 2,
  "packages": {
    "node_modules/legacy-dep": {}
  },
  "dependencies": {
    "legacy-dep": { "version": "1.2.3" }
  }
}`
	pkgs, err := p.parsePackageLock([]byte(lock))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, pkg := range pkgs {
		if pkg.Name == "legacy-dep" && pkg.Version == "1.2.3" && pkg.Type == "npm" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected legacy-dep@1.2.3 from v1 dependencies fallback, got %+v", pkgs)
	}
}

func TestNpmParser_parsePackageLock_V2WithVersionsStillPreferred(t *testing.T) {
	p := NewNpmParser()
	lock := `{
  "lockfileVersion": 2,
  "packages": {
    "node_modules/foo": { "version": "2.0.0" }
  },
  "dependencies": {
    "bar": { "version": "9.9.9" }
  }
}`
	pkgs, err := p.parsePackageLock([]byte(lock))
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) == 0 {
		t.Fatal("expected v2 packages")
	}
	hasFoo := false
	for _, pkg := range pkgs {
		if pkg.Name == "foo" && pkg.Version == "2.0.0" {
			hasFoo = true
		}
	}
	if !hasFoo {
		t.Fatalf("expected v2 path to win for foo@2.0.0, got %+v", pkgs)
	}
}
