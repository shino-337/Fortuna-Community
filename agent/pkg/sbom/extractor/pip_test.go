package extractor

import "testing"

func TestPipParser_Parse_DiscoverRequirementsInAppRoot(t *testing.T) {
	fs := NewFilesystem()
	fs.files["/app/requirements.txt"] = []byte("flask=2.0.0\n")

	pkgs, err := NewPipParser().Parse(fs)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, p := range pkgs {
		if p.Name == "flask" && p.Version == "2.0.0" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected flask@2.0.0, got %+v", pkgs)
	}
}

func TestPipParser_Parse_DiscoverNestedDistInfoMetadata(t *testing.T) {
	fs := NewFilesystem()
	fs.files["/usr/lib/python3/dist-packages/flask.dist-info/METADATA"] = []byte("Name: flask\nVersion: 2.3.4\n")

	pkgs, err := NewPipParser().Parse(fs)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, p := range pkgs {
		if p.Name == "flask" && p.Version == "2.3.4" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected flask@2.3.4 from dist-info METADATA, got %+v", pkgs)
	}
}

func TestPipParser_Parse_OperatorNotExactBecomesUnknown(t *testing.T) {
	fs := NewFilesystem()
	fs.files["/requirements.txt"] = []byte("flask>=2.0\n")

	pkgs, err := NewPipParser().Parse(fs)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, p := range pkgs {
		if p.Name == "flask" {
			found = true
			if p.Version != "unknown" {
				t.Fatalf("expected flask version=unknown, got %q", p.Version)
			}
		}
	}
	if !found {
		t.Fatalf("expected flask to be discovered")
	}
}

func TestPipParser_Parse_OperatorsBecomeUnknown(t *testing.T) {
	fs := NewFilesystem()
	fs.files["/requirements.txt"] = []byte("a~=1.2\nb!=2.0\nc<3.0\nd>1.0\ne<=9.9\n")

	pkgs, err := NewPipParser().Parse(fs)
	if err != nil {
		t.Fatal(err)
	}

	wantUnknown := map[string]bool{"a": false, "b": false, "c": false, "d": false, "e": false}
	for _, p := range pkgs {
		if _, ok := wantUnknown[p.Name]; ok {
			if p.Version != "unknown" {
				t.Fatalf("expected %s version=unknown, got %q", p.Name, p.Version)
			}
			wantUnknown[p.Name] = true
		}
	}
	for n, ok := range wantUnknown {
		if !ok {
			t.Fatalf("expected package %q parsed from requirements", n)
		}
	}
}

func TestPipParser_Parse_DiscoverRequirementsInWorkspaceRoot(t *testing.T) {
	fs := NewFilesystem()
	fs.files["/workspace/requirements.txt"] = []byte("urllib3==2.2.1\n")

	pkgs, err := NewPipParser().Parse(fs)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range pkgs {
		if p.Name == "urllib3" && p.Version == "2.2.1" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected urllib3==2.2.1 from /workspace/requirements.txt, got %+v", pkgs)
	}
}

func TestPipParser_Parse_DedupRequirementsAndMetadata(t *testing.T) {
	fs := NewFilesystem()
	fs.files["/requirements.txt"] = []byte("flask==2.3.4\n")
	fs.files["/usr/lib/python3/dist-packages/flask.dist-info/METADATA"] = []byte("Name: flask\nVersion: 2.3.4\n")

	pkgs, err := NewPipParser().Parse(fs)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, p := range pkgs {
		if p.Name == "flask" && p.Version == "2.3.4" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected deduped flask@2.3.4 once, got count=%d packages=%+v", count, pkgs)
	}
}

