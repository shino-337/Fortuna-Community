package extractor

import (
	"testing"
)

func TestParseDpkgStatus_Monolithic(t *testing.T) {
	content := `Package: base-files
Status: install ok installed
Priority: required
Section: admin
Architecture: amd64
Version: 12.4+deb12u5

Package: libc6
Status: install ok installed
Architecture: amd64
Source: glibc
Version: 2.36-9+deb12u9

Package: removed-pkg
Status: deinstall ok config-files
Architecture: amd64
Version: 1.0
`
	pkgs := parseDpkgStatus(content)
	if len(pkgs) != 2 {
		t.Fatalf("expected 2 installed packages, got %d", len(pkgs))
	}
	if pkgs[0].Name != "base-files" || pkgs[0].Version != "12.4+deb12u5" || pkgs[0].Type != "deb" {
		t.Errorf("pkg[0] = %+v", pkgs[0])
	}
	if pkgs[1].Name != "libc6" || pkgs[1].Version != "2.36-9+deb12u9" {
		t.Errorf("pkg[1] = %+v", pkgs[1])
	}
	if pkgs[1].SourcePackage != "glibc" {
		t.Errorf("pkg[1] source package = %q, want glibc", pkgs[1].SourcePackage)
	}
}

func TestParseDpkgStatus_SourcePackageWithVersion(t *testing.T) {
	content := `Package: libssl3
Status: install ok installed
Architecture: amd64
Source: openssl (3.0.8-1)
Version: 3.0.8-1~deb12u2
`
	pkgs := parseDpkgStatus(content)
	if len(pkgs) != 1 {
		t.Fatalf("expected 1 package, got %d", len(pkgs))
	}
	if pkgs[0].SourcePackage != "openssl" {
		t.Fatalf("SourcePackage = %q, want openssl", pkgs[0].SourcePackage)
	}
	if pkgs[0].SourceVersion != "3.0.8-1" {
		t.Fatalf("SourceVersion = %q, want 3.0.8-1", pkgs[0].SourceVersion)
	}
}

func TestParseDpkgStatus_SingleEntry(t *testing.T) {
	content := `Package: iptables
Status: install ok installed
Priority: optional
Section: net
Installed-Size: 2408
Architecture: amd64
Version: 1.8.9-2
`
	pkgs := parseDpkgStatus(content)
	if len(pkgs) != 1 {
		t.Fatalf("expected 1 package, got %d", len(pkgs))
	}
	if pkgs[0].Name != "iptables" || pkgs[0].Version != "1.8.9-2" || pkgs[0].Arch != "amd64" {
		t.Errorf("got %+v", pkgs[0])
	}
}

func TestDpkgParser_StatusD(t *testing.T) {
	fs := NewFilesystem()
	fs.files["/var/lib/dpkg/status.d/base-files"] = []byte(`Package: base-files
Status: install ok installed
Architecture: amd64
Version: 12.4+deb12u5
`)
	fs.files["/var/lib/dpkg/status.d/base-files.md5sums"] = []byte("abc123  /usr/share/doc/base-files/changelog.gz\n")
	fs.files["/var/lib/dpkg/status.d/libc6"] = []byte(`Package: libc6
Status: install ok installed
Architecture: amd64
Version: 2.36-9+deb12u9
`)
	fs.files["/var/lib/dpkg/status.d/libc6.md5sums"] = []byte("def456  /lib/x86_64-linux-gnu/libc.so.6\n")

	parser := NewDpkgParser()
	pkgs, err := parser.Parse(fs)
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 2 {
		t.Fatalf("expected 2 packages from status.d, got %d", len(pkgs))
	}
	names := map[string]bool{}
	for _, p := range pkgs {
		names[p.Name] = true
		if p.Type != "deb" {
			t.Errorf("expected type deb, got %s for %s", p.Type, p.Name)
		}
	}
	if !names["base-files"] || !names["libc6"] {
		t.Errorf("missing expected packages: %v", names)
	}
}

func TestDpkgParser_StatusD_Dedup(t *testing.T) {
	fs := NewFilesystem()
	fs.files["/var/lib/dpkg/status"] = []byte(`Package: libc6
Status: install ok installed
Architecture: amd64
Version: 2.36-9+deb12u9
`)
	fs.files["/var/lib/dpkg/status.d/libc6"] = []byte(`Package: libc6
Status: install ok installed
Architecture: amd64
Version: 2.36-9+deb12u9
`)

	parser := NewDpkgParser()
	pkgs, err := parser.Parse(fs)
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 1 {
		t.Fatalf("expected 1 deduped package, got %d", len(pkgs))
	}
}

func TestDpkgParser_NoFiles(t *testing.T) {
	fs := NewFilesystem()
	parser := NewDpkgParser()
	pkgs, err := parser.Parse(fs)
	if err != nil {
		t.Fatal(err)
	}
	if pkgs != nil {
		t.Fatalf("expected nil for no dpkg files, got %d packages", len(pkgs))
	}
}
