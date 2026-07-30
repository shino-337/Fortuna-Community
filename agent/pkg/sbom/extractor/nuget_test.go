package extractor

import "testing"

func TestParseNuGetPackagesLock(t *testing.T) {
	data := []byte(`{
  "version": 2,
  "dependencies": {
    "net8.0": {
      "Newtonsoft.Json": {
        "type": "Direct",
        "requested": "[13.0.3, )",
        "resolved": "13.0.3",
        "contentHash": "abc="
      }
    }
  }
}`)
	pkgs := parseNuGetPackagesLock(data)
	if len(pkgs) != 1 || pkgs[0].Name != "Newtonsoft.Json" || pkgs[0].Version != "13.0.3" {
		t.Fatalf("%+v", pkgs)
	}
}

func TestParseNuGetPackagesLock_MultiTF(t *testing.T) {
	data := []byte(`{
  "version": 2,
  "dependencies": {
    "net8.0": { "A": { "resolved": "1.0.0" } },
    "net6.0": { "B": { "resolved": "2.0.0" } }
  }
}`)
	pkgs := parseNuGetPackagesLock(data)
	if len(pkgs) != 2 {
		t.Fatalf("len=%d %+v", len(pkgs), pkgs)
	}
}

func TestParseNuGetPackagesLock_Empty(t *testing.T) {
	if n := len(parseNuGetPackagesLock([]byte(`{"version":1}`))); n != 0 {
		t.Fatalf("expected 0, got %d", n)
	}
}

func TestParseNuGetProjectAssets(t *testing.T) {
	data := []byte(`{
  "version": 3,
  "targets": {
    "net8.0": {
      "Newtonsoft.Json/13.0.3": { "type": "package" }
    }
  }
}`)
	pkgs := parseNuGetProjectAssets(data)
	if len(pkgs) != 1 || pkgs[0].Name != "Newtonsoft.Json" || pkgs[0].Version != "13.0.3" {
		t.Fatalf("%+v", pkgs)
	}
}

func TestSplitNuGetPackageKey(t *testing.T) {
	n, v := splitNuGetPackageKey("My.Lib/2.0.0")
	if n != "My.Lib" || v != "2.0.0" {
		t.Fatalf("%q %q", n, v)
	}
}

func TestNuGetParser_Parse(t *testing.T) {
	fs := NewFilesystem()
	fs.files["/src/packages.lock.json"] = []byte(`{"version":1,"dependencies":{"net6.0":{"X":{"resolved":"1.0.0"}}}}`)

	p := NewNuGetParser()
	pkgs, err := p.Parse(fs)
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 1 || pkgs[0].Name != "X" {
		t.Fatalf("%+v", pkgs)
	}
}
