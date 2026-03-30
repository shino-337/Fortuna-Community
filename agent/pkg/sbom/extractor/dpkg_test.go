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

func TestParseDpkgStatus_DebianEpoch(t *testing.T) {
	content := `Package: zlib1g
Status: install ok installed
Architecture: amd64
Version: 1:1.2.13.dfsg-1
`
	pkgs := parseDpkgStatus(content)
	if len(pkgs) != 1 {
		t.Fatalf("expected 1 package, got %d", len(pkgs))
	}
	if pkgs[0].Epoch != "1" {
		t.Fatalf("Epoch = %q, want 1", pkgs[0].Epoch)
	}
	if pkgs[0].Version != "1.2.13.dfsg-1" {
		t.Fatalf("Version = %q, want 1.2.13.dfsg-1", pkgs[0].Version)
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

func TestParseDebianDependencies(t *testing.T) {
	deps := parseDebianDependencies("libc6 (>= 2.34), libssl3 | libssl1.1, zlib1g:any")
	if len(deps) < 4 {
		t.Fatalf("expected >= 4 deps, got %d", len(deps))
	}
	if deps[0].Package != "libc6" || deps[0].Or {
		t.Fatalf("dep0 = %+v", deps[0])
	}
	if deps[1].Package != "libssl3" || deps[1].Or {
		t.Fatalf("dep1 = %+v", deps[1])
	}
	if deps[2].Package != "libssl1.1" || !deps[2].Or {
		t.Fatalf("dep2 = %+v", deps[2])
	}
	if deps[3].Package != "zlib1g" {
		t.Fatalf("dep3 = %+v", deps[3])
	}
}

func TestParseDpkgStatusDetails(t *testing.T) {
	content := `Package: nginx
Status: install ok installed
Version: 1.25.0-1
Depends: libc6 (>= 2.34), libssl3 | libssl1.1
Pre-Depends: adduser

Package: not-installed
Status: deinstall ok config-files
Version: 1.0
Depends: foo
`
	items := parseDpkgStatusDetails(content)
	if len(items) != 1 {
		t.Fatalf("expected 1 installed item, got %d", len(items))
	}
	if items[0].Name != "nginx" {
		t.Fatalf("unexpected name: %+v", items[0])
	}
	if len(items[0].Depends) == 0 || len(items[0].PreDepends) == 0 {
		t.Fatalf("expected dependencies to be parsed: %+v", items[0])
	}
}
