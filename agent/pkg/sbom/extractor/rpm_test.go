package extractor

import (
	"testing"
)

func TestParseRPMPackagesList_FourFields(t *testing.T) {
	const data = `openssl	1.1.1k	7.el8	x86_64
# comment
zlib	1.2.11	5.el8	x86_64
`
	pkgs, err := parseRPMPackagesList(data, "rocky")
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 2 {
		t.Fatalf("want 2 packages, got %d", len(pkgs))
	}
	if pkgs[0].Name != "openssl" || pkgs[0].Type != "rpm" {
		t.Fatalf("unexpected first pkg: %+v", pkgs[0])
	}
	if pkgs[0].PURL == "" || pkgs[0].Version != "1.1.1k-7.el8" {
		t.Fatalf("want merged evr, got %+v", pkgs[0])
	}
	if want := "pkg:rpm/rocky/openssl@1.1.1k-7.el8?arch=x86_64"; pkgs[0].PURL != want {
		t.Fatalf("PURL = %q, want %q", pkgs[0].PURL, want)
	}
}

func TestParseRPMPackagesList_FiveFieldsEpoch(t *testing.T) {
	data := "curl\t1\t7.76.1\t4.fc34\tx86_64\n"
	pkgs, err := parseRPMPackagesList(data, "fedora")
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 1 {
		t.Fatalf("want 1 pkg, got %d", len(pkgs))
	}
	if pkgs[0].Version != "1:7.76.1-4.fc34" {
		t.Fatalf("version %q", pkgs[0].Version)
	}
}

func TestRpmParser_Parse_InventoryFile(t *testing.T) {
	fs := NewFilesystem()
	fs.files["/var/lib/fortuna/rpm-packages.list"] = []byte("bash|5.1|8.el9|x86_64\n")
	p := NewRpmParser()
	pkgs, err := p.Parse(fs)
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 1 || pkgs[0].Name != "bash" || pkgs[0].PURL == "" {
		t.Fatalf("unexpected: %+v", pkgs)
	}
}

func TestRpmParser_Parse_MissingInventory(t *testing.T) {
	fs := NewFilesystem()
	pkgs, err := NewRpmParser().Parse(fs)
	if err != nil {
		t.Fatal(err)
	}
	if pkgs != nil {
		t.Fatalf("want nil slice, got %#v", pkgs)
	}
}

func TestRpmParser_Parse_RPMManifest(t *testing.T) {
	fs := NewFilesystem()
	fs.files["/etc/os-release"] = []byte("ID=mariner\nVERSION_ID=2.0\n")
	fs.files["/var/lib/rpmmanifest/container-manifest-2"] = []byte(
		"curl-7.85.0-1.cm2.x86_64\n" +
			"openssl-libs-1.1.1k-21.cm2.x86_64\n" +
			"# comment line\n" +
			"\n" +
			"glibc-2.35-4.cm2.x86_64\n",
	)

	p := NewRpmParser()
	pkgs, err := p.Parse(fs)
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 3 {
		t.Fatalf("want 3 packages from rpmmanifest, got %d", len(pkgs))
	}

	names := map[string]string{}
	for _, pkg := range pkgs {
		names[pkg.Name] = pkg.Version
		if pkg.Type != "rpm" {
			t.Errorf("expected type rpm, got %s", pkg.Type)
		}
		if pkg.Source != "rpmmanifest" {
			t.Errorf("expected source rpmmanifest, got %s", pkg.Source)
		}
	}
	if v, ok := names["curl"]; !ok || v != "7.85.0-1.cm2" {
		t.Errorf("curl: got %q", v)
	}
	if v, ok := names["openssl-libs"]; !ok || v != "1.1.1k-21.cm2" {
		t.Errorf("openssl-libs: got %q", v)
	}
	if v, ok := names["glibc"]; !ok || v != "2.35-4.cm2" {
		t.Errorf("glibc: got %q", v)
	}
}

func TestRpmParser_Parse_InvalidRPMDBFallback_NonFatal(t *testing.T) {
	fs := NewFilesystem()
	// Inventory missing; fallback path exists but contains invalid data.
	fs.files["/var/lib/rpm/rpmdb.sqlite"] = []byte("not-a-real-rpmdb")

	pkgs, err := NewRpmParser().Parse(fs)
	if err != nil {
		t.Fatalf("fallback should be non-fatal, got error: %v", err)
	}
	if pkgs != nil {
		t.Fatalf("expected nil packages on invalid fallback db, got %#v", pkgs)
	}
}
