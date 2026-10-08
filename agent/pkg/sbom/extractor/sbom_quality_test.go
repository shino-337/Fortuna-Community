package extractor

import (
	"sort"
	"testing"
)

// Debian and Ubuntu ship /etc/os-release as a symlink to ../usr/lib/os-release. The layer
// filesystem keeps regular files only, so detection must also read the target; otherwise
// Ubuntu falls through to /etc/debian_version ("bookworm/sid") and is reported as Debian.
func TestDetectOS_UsrLibOSReleaseWhenEtcIsSymlink(t *testing.T) {
	e := NewExtractor()
	fs := NewFilesystem()
	fs.files["/usr/lib/os-release"] = []byte("NAME=\"Ubuntu\"\nID=ubuntu\nVERSION_ID=\"24.04\"\n")
	fs.files["/etc/debian_version"] = []byte("trixie/sid\n")

	got := e.detectOS(fs, nil)
	if got.Name != "ubuntu" || got.Version != "24.04" {
		t.Fatalf("detectOS = %+v, want ubuntu 24.04", got)
	}
}

// Google distroless images say PRETTY_NAME="Distroless" but are Debian; their dpkg PURLs
// must use the debian namespace so Core queries the Debian advisories.
func TestSetOSPackagePURLs_DistrolessUsesBaseDistro(t *testing.T) {
	e := NewExtractor()
	fs := NewFilesystem()
	fs.files["/etc/os-release"] = []byte("PRETTY_NAME=\"Distroless\"\nNAME=\"Debian GNU/Linux\"\nID=\"debian\"\nVERSION_ID=\"12\"\n")
	osInfo := e.detectOS(fs, nil)
	if osInfo.Name != "distroless" {
		t.Fatalf("Name = %q, want distroless (parser selection relies on it)", osInfo.Name)
	}

	pkgs := setOSPackagePURLs([]Package{{Name: "libssl3", SourcePackage: "openssl", Version: "3.0.15-1~deb12u1", Arch: "amd64", Type: "deb"}}, osInfo)
	if want := "pkg:deb/debian/openssl@3.0.15-1?arch=amd64"; pkgs[0].PURL != want {
		t.Fatalf("PURL = %q, want %q", pkgs[0].PURL, want)
	}

	// Distroless detected from OCI labels only (no os-release): still a Debian namespace.
	pkgs = setOSPackagePURLs([]Package{{Name: "tzdata", Version: "2024a-0+deb12u1", Type: "deb"}}, OSInfo{Name: "distroless", Version: "unknown"})
	if want := "pkg:deb/debian/tzdata@2024a-0"; pkgs[0].PURL != want {
		t.Fatalf("PURL = %q, want %q", pkgs[0].PURL, want)
	}
}

func TestParseDpkgStatus_InstalledStateAndNoFieldLeak(t *testing.T) {
	status := `Package: removed-lib
Status: deinstall ok config-files
Architecture: amd64
Source: openssl
Version: 1.0-1

Package: purged
Status: purge ok not-installed
Architecture: amd64
Version: 2.0-1

Package: broken
Status: install ok half-installed
Architecture: amd64
Version: 3.0-1

Package: zlib1g
Status: install ok installed
Architecture: amd64
Version: 1:1.2.13.dfsg-1
Description: compression library
 Version: 9.9 appears in the description and is not a field

Package: marked-for-removal
Status: deinstall ok installed
Architecture: all
Version: 4.0-1
`
	pkgs := parseDpkgStatus(status)
	names := make([]string, 0, len(pkgs))
	for _, p := range pkgs {
		names = append(names, p.Name)
	}
	sort.Strings(names)
	if len(names) != 2 || names[0] != "marked-for-removal" || names[1] != "zlib1g" {
		t.Fatalf("installed packages = %v, want [marked-for-removal zlib1g]", names)
	}
	for _, p := range pkgs {
		if p.Name == "zlib1g" {
			if p.SourcePackage != "" {
				t.Errorf("zlib1g SourcePackage = %q leaked from a previous stanza", p.SourcePackage)
			}
			if p.Version != "1.2.13.dfsg-1" || p.Epoch != "1" {
				t.Errorf("zlib1g version = %q epoch %q, want 1.2.13.dfsg-1 epoch 1", p.Version, p.Epoch)
			}
		}
	}
}

func TestNpmParser_LockV2ScopedNestedAndRoot(t *testing.T) {
	p := NewNpmParser()
	lock := `{
  "lockfileVersion": 3,
  "packages": {
    "": { "name": "my-app", "version": "1.0.0" },
    "node_modules/@babel/core": { "version": "7.24.0" },
    "node_modules/express/node_modules/debug": { "version": "2.6.9" },
    "node_modules/aliased": { "name": "real-name", "version": "3.1.0" },
    "node_modules/workspace-pkg": { "resolved": "packages/ws", "link": true },
    "packages/ws": { "name": "workspace-pkg", "version": "0.1.0" }
  }
}`
	pkgs, err := p.parsePackageLock([]byte(lock))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, pkg := range pkgs {
		got[pkg.Name] = pkg.Version
	}
	want := map[string]string{"@babel/core": "7.24.0", "debug": "2.6.9", "real-name": "3.1.0", "workspace-pkg": "0.1.0"}
	for n, v := range want {
		if got[n] != v {
			t.Errorf("%s = %q, want %q (all: %v)", n, got[n], v, got)
		}
	}
	for _, bad := range []string{"root", "core", "my-app", "aliased"} {
		if _, ok := got[bad]; ok {
			t.Errorf("unexpected package %q (all: %v)", bad, got)
		}
	}
}

func TestNpmParser_InstalledScopedAndGlobalModules(t *testing.T) {
	fs := NewFilesystem()
	fs.files["/app/node_modules/@nestjs/core/package.json"] = []byte(`{"name":"@nestjs/core","version":"10.3.0"}`)
	fs.files["/app/node_modules/lodash/package.json"] = []byte(`{"name":"lodash","version":"4.17.21"}`)
	fs.files["/usr/local/lib/node_modules/npm/package.json"] = []byte(`{"name":"npm","version":"10.5.0"}`)

	pkgs, err := NewNpmParser().Parse(fs)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, pkg := range pkgs {
		got[pkg.Name] = pkg.Version
	}
	for n, v := range map[string]string{"@nestjs/core": "10.3.0", "lodash": "4.17.21", "npm": "10.5.0"} {
		if got[n] != v {
			t.Errorf("%s = %q, want %q (all: %v)", n, got[n], v, got)
		}
	}
}

func TestPipParser_MetadataStopsAtBody(t *testing.T) {
	meta := "Metadata-Version: 2.1\r\nName: requests\r\nVersion: 2.32.3\r\n\r\nRelease notes\nVersion: 0.1 was the first release\nName: not-a-package\n"
	pkg, err := NewPipParser().parseMetadata([]byte(meta))
	if err != nil {
		t.Fatal(err)
	}
	if pkg.Name != "requests" || pkg.Version != "2.32.3" {
		t.Fatalf("got %s@%s, want requests@2.32.3", pkg.Name, pkg.Version)
	}
}
