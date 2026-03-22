package extractor

import "testing"

func TestCargoParser_Parse(t *testing.T) {
	fs := NewFilesystem()
	fs.files["/src/Cargo.lock"] = []byte(`# This file is automatically @generated
[[package]]
name = "serde"
version = "1.0.190"

[[package]]
name = "libc"
version = "0.2.150"
`)

	p := NewCargoParser()
	pkgs, err := p.Parse(fs)
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 2 {
		t.Fatalf("len=%d %+v", len(pkgs), pkgs)
	}
}

func TestParseCargoLockPackages(t *testing.T) {
	data := []byte(`[[package]]
name = "a-b"
version = "1.0.0"
`)
	pkgs := parseCargoLockPackages(data)
	if len(pkgs) != 1 || pkgs[0].Name != "a-b" || pkgs[0].Version != "1.0.0" {
		t.Fatalf("%+v", pkgs)
	}
}
