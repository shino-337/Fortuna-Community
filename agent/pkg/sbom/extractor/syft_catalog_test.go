package extractor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Trimmed from `syft scan docker-archive:... -o syft-json` (v1.52) on debian, tomcat,
// node and coredns images.
const syftFixture = `{
  "distro": {"id": "debian", "versionID": "12.15"},
  "artifacts": [
    {"name": "libssl3", "version": "3.0.15-1~deb12u1", "type": "deb", "foundBy": "dpkg-db-cataloger",
     "purl": "pkg:deb/debian/libssl3@3.0.15-1~deb12u1?arch=amd64&distro=debian-12.15&upstream=openssl",
     "licenses": [{"value": "Apache-2.0", "spdxExpression": "Apache-2.0"}, {"value": "sha256:abc"}],
     "cpes": [{"cpe": "cpe:2.3:a:libssl3:libssl3:3.0.15-1~deb12u1:*:*:*:*:*:*:*"}]},
    {"name": "zlib1g", "version": "1:1.2.13.dfsg-1", "type": "deb", "foundBy": "dpkg-db-cataloger",
     "purl": "pkg:deb/debian/zlib1g@1%3A1.2.13.dfsg-1?arch=amd64&distro=debian-12.15&upstream=zlib"},
    {"name": "catalina", "version": "10.1.60", "type": "java-archive", "foundBy": "java-archive-cataloger",
     "purl": "pkg:maven/org.apache.tomcat/catalina@10.1.60"},
    {"name": "@npmcli/agent", "version": "2.2.2", "type": "npm", "foundBy": "javascript-package-cataloger",
     "purl": "pkg:npm/%40npmcli/agent@2.2.2"},
    {"name": "github.com/miekg/dns", "version": "v1.1.55", "type": "go-module", "foundBy": "go-module-binary-cataloger",
     "purl": "pkg:golang/github.com/miekg/dns@v1.1.55"},
    {"name": "stdlib", "version": "go1.21.3", "type": "go-module", "foundBy": "go-module-binary-cataloger", "purl": "pkg:golang/stdlib@1.21.3"},
    {"name": "stdlib", "version": "go1.20.7", "type": "go-module", "foundBy": "go-module-binary-cataloger", "purl": "pkg:golang/stdlib@1.20.7"},
    {"name": "node", "version": "20.20.2", "type": "binary", "foundBy": "binary-classifier-cataloger", "purl": "pkg:generic/node@20.20.2"},
    {"name": "Simple Launcher", "version": "1.1.0.14", "type": "binary", "foundBy": "pe-binary-package-cataloger"},
    {"name": "libssl3", "version": "3.0.15-1~deb12u1", "type": "deb", "foundBy": "dpkg-db-cataloger",
     "purl": "pkg:deb/debian/libssl3@3.0.15-1~deb12u1?arch=amd64&distro=debian-12.15&upstream=openssl"}
  ]
}`

func TestParseSyftCatalog(t *testing.T) {
	cat, err := parseSyftCatalog([]byte(syftFixture), 0)
	if err != nil {
		t.Fatal(err)
	}
	if cat.OS.Name != "debian" || cat.OS.Version != "12.15" || cat.OS.Distro != "debian" {
		t.Errorf("OS = %+v, want debian 12.15", cat.OS)
	}
	if cat.GoVersion != "go1.20.7" {
		t.Errorf("GoVersion = %q, want the oldest toolchain go1.20.7", cat.GoVersion)
	}
	byName := map[string]Package{}
	for _, p := range cat.Packages {
		if _, dup := byName[p.Name]; dup {
			t.Errorf("duplicate package %s", p.Name)
		}
		byName[p.Name] = p
	}
	if len(cat.Packages) != 6 {
		t.Errorf("got %d packages, want 6 (stdlib folded into GoVersion, no-PURL artifact and duplicate dropped): %v", len(cat.Packages), byName)
	}

	ssl := byName["libssl3"]
	if ssl.Type != "deb" || ssl.SourcePackage != "openssl" || ssl.Arch != "amd64" || ssl.Source != "dpkg" || ssl.Confidence != "high" {
		t.Errorf("libssl3 = %+v", ssl)
	}
	if len(ssl.Licenses) != 1 || ssl.Licenses[0] != "Apache-2.0" || len(ssl.CPEs) != 1 {
		t.Errorf("libssl3 licenses %v cpes %v", ssl.Licenses, ssl.CPEs)
	}
	if z := byName["zlib1g"]; z.Version != "1:1.2.13.dfsg-1" || z.SourcePackage != "zlib" {
		t.Errorf("zlib1g = %+v", z)
	}
	if m, ok := byName["org.apache.tomcat:catalina"]; !ok || m.Type != "maven" {
		t.Errorf("maven package must be named group:artifact, got %v", byName)
	}
	if n := byName["@npmcli/agent"]; n.Type != "npm" {
		t.Errorf("npm scoped package = %+v", n)
	}
	if g := byName["github.com/miekg/dns"]; g.Type != "go" || g.Source != "gobinary" {
		t.Errorf("go module = %+v", g)
	}
	if b := byName["node"]; b.Type != "generic" || b.Confidence != "medium" || b.Source != "binary-classifier" {
		t.Errorf("binary classifier = %+v", b)
	}
}

func TestParseSyftCatalogNoDistroAndCap(t *testing.T) {
	cat, err := parseSyftCatalog([]byte(`{"artifacts":[
		{"name":"a","version":"1","type":"npm","purl":"pkg:npm/a@1"},
		{"name":"b","version":"1","type":"npm","purl":"pkg:npm/b@1"}]}`), 1)
	if err != nil {
		t.Fatal(err)
	}
	if cat.OS.Name != "unknown" || len(cat.Packages) != 1 {
		t.Fatalf("got OS %+v and %d packages, want unknown and the cap of 1", cat.OS, len(cat.Packages))
	}
}

// Syft must scan the local archive, offline, without the Agent's environment.
func TestCatalogArchiveRunsOfflineWithCleanEnv(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "syft")
	argsFile := filepath.Join(dir, "args")
	script := `#!/bin/sh
printf '%s\n' "$@" > ` + argsFile + `
[ -z "$FORTUNA_TEST_SECRET" ] || { echo "agent env leaked" >&2; exit 7; }
[ "$SYFT_CHECK_FOR_APP_UPDATE" = false ] || exit 8
[ "$SYFT_JAVA_USE_NETWORK" = false ] || exit 9
for a in "$@"; do case "$a" in syft-json=*) out="${a#syft-json=}";; esac; done
printf '{"distro":{"id":"alpine","versionID":"3.20.3"},"artifacts":[{"name":"libcrypto3","version":"3.3.2-r0","type":"apk","foundBy":"apk-db-cataloger","purl":"pkg:apk/alpine/libcrypto3@3.3.2-r0?arch=x86_64&distro=alpine-3.20.3&upstream=openssl"}]}' > "$out"
`
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SBOM_SYFT_BIN", bin)
	t.Setenv("FORTUNA_TEST_SECRET", "do-not-pass")

	cat, err := NewSyftAdapter(nil, time.Minute, 0).CatalogArchive(context.Background(), "/tmp/image.tar")
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Packages) != 1 || cat.Packages[0].SourcePackage != "openssl" || cat.OS.Name != "alpine" {
		t.Fatalf("catalog = %+v", cat)
	}
	args, _ := os.ReadFile(argsFile)
	got := strings.Fields(string(args))
	if len(got) < 2 || got[0] != "scan" || got[1] != "docker-archive:/tmp/image.tar" {
		t.Fatalf("syft args = %v, want scan docker-archive:/tmp/image.tar ...", got)
	}
}

func TestCatalogArchiveFailureIsAnError(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "syft")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho boom >&2\nexit 3\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SBOM_SYFT_BIN", bin)
	if _, err := NewSyftAdapter(nil, time.Minute, 0).CatalogArchive(context.Background(), "/tmp/x.tar"); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v, want the syft failure with its stderr", err)
	}
}

func TestApkParserReadsOriginAndLicense(t *testing.T) {
	fs := NewFilesystem()
	fs.files["/lib/apk/db/installed"] = []byte("P:libcrypto3\nV:3.3.2-r0\nA:x86_64\nL:Apache-2.0\no:openssl\n\nP:musl\nV:1.2.5-r0\nA:x86_64\no:musl\n")
	pkgs, err := NewApkParser().Parse(fs)
	if err != nil {
		t.Fatal(err)
	}
	pkgs = setOSPackagePURLs(pkgs, OSInfo{Name: "alpine", Version: "3.20.3", Distro: "alpine"})
	if want := "pkg:apk/alpine/libcrypto3@3.3.2-r0?arch=x86_64&distro=alpine-3.20.3&upstream=openssl"; pkgs[0].PURL != want {
		t.Errorf("libcrypto3 PURL = %q, want %q", pkgs[0].PURL, want)
	}
	if len(pkgs[0].Licenses) != 1 || pkgs[0].Licenses[0] != "Apache-2.0" {
		t.Errorf("licenses = %v", pkgs[0].Licenses)
	}
	if want := "pkg:apk/alpine/musl@1.2.5-r0?arch=x86_64&distro=alpine-3.20.3"; pkgs[1].PURL != want {
		t.Errorf("musl PURL = %q, want %q (no upstream when it equals the name)", pkgs[1].PURL, want)
	}
}
