package matcher

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/fortuna/core/pkg/cve"
	"github.com/fortuna/core/pkg/cve/catalogtest"
	"github.com/fortuna/core/pkg/cve/database"
	"github.com/fortuna/core/pkg/metrics"
	"github.com/fortuna/core/pkg/models"
	"github.com/glebarez/sqlite"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestComparatorEcosystemForBulkBatch(t *testing.T) {
	p, err := ParsePURL("pkg:generic/openssl@1.1.1d-0+deb10u7")
	if err != nil {
		t.Fatal(err)
	}
	if g := comparatorEcosystemForBulkBatch(p, "debian"); g != "debian" {
		t.Fatalf("bulk debian: got %q want debian", g)
	}
	if g := comparatorEcosystemForBulkBatch(p, "generic"); g != "generic" {
		t.Fatalf("bulk generic: got %q want generic", g)
	}
	rp, err := ParsePURL("pkg:rpm/openssl@1.1.1k-8.el8")
	if err != nil {
		t.Fatal(err)
	}
	if g := comparatorEcosystemForBulkBatch(rp, "linux"); g != "rpm" {
		t.Fatalf("bulk linux: got %q want rpm", g)
	}
	if g := comparatorEcosystemForBulkBatch(nil, "generic"); g != "generic" {
		t.Fatalf("nil purl + generic bulk: got %q want generic", g)
	}
	if g := comparatorEcosystemForBulkBatch(nil, ""); g != "generic" {
		t.Fatalf("nil purl + empty bulk: got %q want generic", g)
	}
}

func TestLikelyDistroEcosystemsForGenericSBOM(t *testing.T) {
	if got := strings.Join(likelyDistroEcosystemsForGenericSBOM(""), ","); got != "alpine,debian" {
		t.Fatalf("empty OS: got %q", got)
	}
	if got := likelyDistroEcosystemsForGenericSBOM("Alpine Linux 3.19"); len(got) != 1 || got[0] != "alpine" {
		t.Fatalf("alpine: %#v", got)
	}
	if got := likelyDistroEcosystemsForGenericSBOM("Debian GNU/Linux 12"); len(got) != 1 || got[0] != "debian" {
		t.Fatalf("debian: %#v", got)
	}
	if got := likelyDistroEcosystemsForGenericSBOM("Ubuntu 22.04"); len(got) != 1 || got[0] != "ubuntu" {
		t.Fatalf("ubuntu: %#v", got)
	}
	if got := likelyDistroEcosystemsForGenericSBOM("Red Hat Enterprise Linux 8"); len(got) != 1 || got[0] != "redhat" {
		t.Fatalf("rhel: %#v", got)
	}
}

func TestNormalizeQueryEcosystemWithOS(t *testing.T) {
	tests := []struct {
		name       string
		purl       string
		sbomOSName string
		want       string
	}{
		{
			name:       "generic openssl on Debian -> debian for OSV query",
			purl:       "pkg:generic/openssl@3.0.18-1~deb12u2",
			sbomOSName: "debian",
			want:       "debian",
		},
		{
			name:       "generic on Ubuntu -> ubuntu",
			purl:       "pkg:generic/openssl@1.1.1",
			sbomOSName: "ubuntu",
			want:       "ubuntu",
		},
		{
			name:       "generic on Alpine -> alpine",
			purl:       "pkg:generic/busybox@1.36",
			sbomOSName: "alpine",
			want:       "alpine",
		},
		{
			name:       "generic with unknown OS -> generic",
			purl:       "pkg:generic/coredns@unknown",
			sbomOSName: "",
			want:       "generic",
		},
		{
			name:       "pkg:deb/debian/openssl -> debian",
			purl:       "pkg:deb/debian/openssl@3.0.18-1~deb12u2",
			sbomOSName: "",
			want:       "debian",
		},
		{
			name:       "pkg:deb/ubuntu/libc6 -> ubuntu",
			purl:       "pkg:deb/ubuntu/libc6@2.35",
			sbomOSName: "",
			want:       "ubuntu",
		},
		{
			name:       "rpm without namespace on Rocky -> rocky",
			purl:       "pkg:rpm/openssl@1.1.1k-8.el8_6",
			sbomOSName: "Rocky Linux 8.8",
			want:       "rocky",
		},
		{
			name:       "generic on RHEL -> redhat",
			purl:       "pkg:generic/openssl@1.1.1k",
			sbomOSName: "Red Hat Enterprise Linux 8.6",
			want:       "redhat",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := ParsePURL(tt.purl)
			if err != nil {
				t.Fatalf("ParsePURL: %v", err)
			}
			got := normalizeQueryEcosystemWithOS(p, tt.sbomOSName)
			if got != tt.want {
				t.Errorf("normalizeQueryEcosystemWithOS(%q, %q) = %q, want %q", tt.purl, tt.sbomOSName, got, tt.want)
			}
		})
	}
}

func TestNormalizeDistrolessComponentName(t *testing.T) {
	tests := []struct{ in, want string }{
		{"coredns", "coredns"},
		{"coredns/coredns", "coredns"},
		{"registry.k8s.io/coredns/coredns", "coredns"},
		{"registry.k8s.io/coredns", "coredns"}, // no slash: use registryCanonicalName map
		{"kube-apiserver", "kube-apiserver"},
		{"kubernetes-apiserver", "kube-apiserver"},
		{"k8s-apiserver", "kube-apiserver"},
		{"openssl", "openssl"},
		{"  coredns  ", "coredns"},
	}
	for _, tt := range tests {
		got := normalizeDistrolessComponentName(tt.in)
		if got != tt.want {
			t.Errorf("normalizeDistrolessComponentName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestEffectiveVersionForComparison_DebEpochQualifier(t *testing.T) {
	p := &PURL{
		Ecosystem: "deb",
		Qualifiers: map[string]string{
			"epoch": "2",
		},
	}
	got := effectiveVersionForComparison("3.0.8-1", p)
	if got != "2:3.0.8-1" {
		t.Fatalf("effectiveVersionForComparison = %q, want %q", got, "2:3.0.8-1")
	}
}

func TestIsCVEApplicableToPackageArch(t *testing.T) {
	p := &PURL{
		Ecosystem: "deb",
		Qualifiers: map[string]string{
			"arch": "amd64",
		},
	}
	if !isCVEApplicableToPackageArch(&cve.CVE{Constraint: ">= 1.0, < 2.0"}, p) {
		t.Fatal("constraint without arch metadata should be applicable")
	}
	if !isCVEApplicableToPackageArch(&cve.CVE{Constraint: ">= 1.0, arch=amd64"}, p) {
		t.Fatal("matching arch qualifier should be applicable")
	}
	if isCVEApplicableToPackageArch(&cve.CVE{Constraint: ">= 1.0, arch=arm64"}, p) {
		t.Fatal("mismatched arch qualifier should not be applicable")
	}
}

func TestIsDistrolessHeuristicAllowlisted(t *testing.T) {
	allowed := []string{
		"kube-apiserver", "kube-controller-manager", "kube-scheduler", "kube-proxy",
		"coredns", "etcd", "pause", "openssl", "libc.so.6", "libssl.so.3",
		"containerd-shim", "runc", "glibc", "libfoo.so.1",
		"registry.k8s.io/coredns", "coredns/coredns", // normalized to coredns → whitelisted
		"golang.org/x/crypto", "Golang.org/x/Net", // x/* expansion path (normalize → short token; raw prefix whitelists)
	}
	for _, name := range allowed {
		if !isDistrolessHeuristicAllowlisted(name) {
			t.Errorf("isDistrolessHeuristicAllowlisted(%q) = false, want true", name)
		}
	}
	rejected := []string{"Extend.pl", "ISO-IR-197.so", "Y.pl", "unknown-image", "nginx"}
	for _, name := range rejected {
		if isDistrolessHeuristicAllowlisted(name) {
			t.Errorf("isDistrolessHeuristicAllowlisted(%q) = true, want false", name)
		}
	}
}

func TestIsDistrolessHeuristicJunk(t *testing.T) {
	// C0.3: version=unknown, source=distroless-heuristic should NOT be skipped entirely
	// (controlled matching instead of hard skip).
	if isDistrolessHeuristicJunk(&models.SBOMComponent{
		ComponentName: "Extend.pl", ComponentVersion: "unknown", Source: "distroless-heuristic",
	}) {
		t.Error("Extend.pl@unknown distroless-heuristic should NOT be junk (controlled matching)")
	}
	if isDistrolessHeuristicJunk(&models.SBOMComponent{
		ComponentName: "ISO-IR-197.so", ComponentVersion: "unknown", Source: "distroless-heuristic",
	}) {
		t.Error("ISO-IR-197.so@unknown distroless-heuristic should NOT be junk (controlled matching)")
	}
	// Not junk: allowlisted names even with unknown -> keep
	if isDistrolessHeuristicJunk(&models.SBOMComponent{
		ComponentName: "coredns", ComponentVersion: "unknown", Source: "distroless-heuristic",
	}) {
		t.Error("coredns@unknown should not be junk (allowlisted control-plane name)")
	}
	if isDistrolessHeuristicJunk(&models.SBOMComponent{
		ComponentName: "kube-apiserver", ComponentVersion: "unknown", Source: "distroless-heuristic",
	}) {
		t.Error("kube-apiserver@unknown should not be junk (allowlisted control-plane name)")
	}
	// Junk: non-whitelisted names with known version (version is available to match).
	if !isDistrolessHeuristicJunk(&models.SBOMComponent{
		ComponentName: "Extend.pl", ComponentVersion: "1.0", Source: "distroless-heuristic",
	}) {
		t.Error("Extend.pl@1.0 distroless-heuristic should be junk (non-whitelisted)")
	}
	// Not junk: source not distroless-heuristic -> keep
	if isDistrolessHeuristicJunk(&models.SBOMComponent{
		ComponentName: "openssl", ComponentVersion: "unknown", Source: "parsers",
	}) {
		t.Error("openssl@unknown from parsers should not be junk")
	}
}

// TestGenericOpenSSLOnDebianUsesDebianEcosystem verifies generic components on Debian use ecosystem "debian"
// so that Postgres OSV data is queried.
func TestGenericOpenSSLOnDebianUsesDebianEcosystem(t *testing.T) {
	// Simulate: SBOM from Debian image, component openssl generic (e.g. from heuristic).
	// Query ecosystem must be "debian" to hit OSV loader data.
	p, _ := ParsePURL("pkg:generic/openssl@3.0.18-1~deb12u2")
	eco := normalizeQueryEcosystemWithOS(p, "debian")
	if eco != "debian" {
		t.Errorf("generic openssl on Debian should query ecosystem debian for OSV; got %q", eco)
	}
	if !isDistrolessHeuristicAllowlisted("openssl") {
		t.Error("openssl must be allowlisted for distroless-heuristic noise control")
	}
	if isDistrolessHeuristicAllowlisted("Extend.pl") {
		t.Error("Extend.pl must not be allowlisted as a control-plane/runtime name")
	}
}

// TestDistrolessAllowlistCoversControlPlane documents which component names stay in the
// distroless-heuristic allowlist for junk filtering (not live CVE API calls).
func TestDistrolessAllowlistCoversControlPlane(t *testing.T) {
	type row struct {
		componentName string
		allowlisted   bool
	}
	cases := []row{
		{"coredns", true},
		{"kube-apiserver", true},
		{"openssl", true},
		{"etcd", true},
		{"pause", true},
		{"libc.so.6", true},
		{"Extend.pl", false},
		{"ISO-IR-197.so", false},
		{"Y.pl", false},
	}
	for _, c := range cases {
		got := isDistrolessHeuristicAllowlisted(c.componentName)
		if got != c.allowlisted {
			t.Errorf("component %q: allowlisted = %v, want %v", c.componentName, got, c.allowlisted)
		}
	}
}

// TestOpenSSL_Debian12_CVEResults verifies that OpenSSL 3.0.18-1~deb12u2 for Debian 12
// is queried with ecosystem=debian and returns CVE matches when OSV data is present.
func TestOpenSSL_Debian12_CVEResults(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.SBOM{}, &models.SBOMComponent{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Seed the advisory in the versioned catalog (as the OSV loader would)
	catalogtest.Seed(t, db, catalogtest.Advisory{
		ID:       "CVE-2025-15467",
		Severity: "HIGH",
		Details:  "Stack buffer overflow in OpenSSL (Debian 12 bookworm).",
		Affected: []catalogtest.Range{
			// affected up to and including 3.0.18-1~deb12u2
			{Ecosystem: "debian", Package: "openssl", LastAffected: "3.0.18-1~deb12u2"},
		},
	})

	// SBOM for Debian 12 with openssl 3.0.18-1~deb12u2
	sbom := &models.SBOM{
		OSName:    "debian",
		OSVersion: "12",
		Status:    "finalized",
	}
	if err := db.Create(sbom).Error; err != nil {
		t.Fatalf("create SBOM: %v", err)
	}
	comp := models.SBOMComponent{
		SBOMID:           sbom.ID,
		ComponentName:    "openssl",
		ComponentVersion: "3.0.18-1~deb12u2",
		PURL:             "pkg:deb/debian/openssl@3.0.18-1~deb12u2",
		ComponentType:    "os-package",
	}
	if err := db.Create(&comp).Error; err != nil {
		t.Fatalf("create SBOMComponent: %v", err)
	}

	// Match CVEs
	mgr := database.NewPostgresManager(db)
	matcher := NewMatcher(mgr, db)
	ctx := context.Background()
	matches, err := matcher.MatchSBOM(ctx, sbom, nil)
	if err != nil {
		t.Fatalf("MatchSBOM: %v", err)
	}

	// Assert CVE result for OpenSSL 3.0.18-1~deb12u2
	if len(matches) == 0 {
		t.Fatal("expected at least one CVE match for openssl 3.0.18-1~deb12u2 (Debian 12), got 0")
	}
	var found bool
	for _, m := range matches {
		if m.PackageName == "openssl" && m.PackageVersion == "3.0.18-1~deb12u2" && m.CVEID == "CVE-2025-15467" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a CVE match for openssl@3.0.18-1~deb12u2 with CVE-2025-15467; got %d matches: %+v",
			len(matches), matches)
	}
}

// TestOpenSSL_Debian12_GenericPURL_CVEResults verifies that when the component has
// generic PURL (e.g. from distroless/heuristic) but SBOM OS is Debian 12, the matcher
// still queries with ecosystem=debian and returns CVE results.
func TestOpenSSL_Debian12_GenericPURL_CVEResults(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.SBOM{}, &models.SBOMComponent{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	catalogtest.Seed(t, db, catalogtest.Advisory{ID: "CVE-2025-15467", Severity: "HIGH", Affected: []catalogtest.Range{
		{Ecosystem: "debian", Package: "openssl", LastAffected: "3.0.18-1~deb12u2"},
	}})

	sbom := &models.SBOM{OSName: "debian", OSVersion: "12", Status: "finalized"}
	if err := db.Create(sbom).Error; err != nil {
		t.Fatalf("create SBOM: %v", err)
	}
	// Simulate heuristic SBOM: generic PURL instead of pkg:deb/debian/...
	comp := models.SBOMComponent{
		SBOMID:           sbom.ID,
		ComponentName:    "openssl",
		ComponentVersion: "3.0.18-1~deb12u2",
		PURL:             "pkg:generic/openssl@3.0.18-1~deb12u2",
		ComponentType:    "application",
		Source:           "distroless-heuristic",
	}
	if err := db.Create(&comp).Error; err != nil {
		t.Fatalf("create SBOMComponent: %v", err)
	}

	mgr := database.NewPostgresManager(db)
	matcher := NewMatcher(mgr, db)
	matches, err := matcher.MatchSBOM(context.Background(), sbom, nil)
	if err != nil {
		t.Fatalf("MatchSBOM: %v", err)
	}

	if len(matches) == 0 {
		t.Fatal("generic PURL + OS debian should query ecosystem debian and return CVE; got 0 matches")
	}
	var found bool
	for _, m := range matches {
		if m.CVEID == "CVE-2025-15467" && m.PackageVersion == "3.0.18-1~deb12u2" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected CVE-2025-15467 for openssl@3.0.18-1~deb12u2 (generic PURL); got %+v", matches)
	}
}

// TestControlPlane_KubeControllerManager_V12915 verifies CVE lookup for control-plane
// component kube-controller-manager:v1.29.15. Uses seeded Postgres CVE data (as OSV or
// custom loader might provide) so we get version-based match.
func TestControlPlane_KubeControllerManager_V12915(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.SBOM{}, &models.SBOMComponent{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Example CVE for Kubernetes controller-manager (version range that includes v1.29.15)
	catalogtest.Seed(t, db, catalogtest.Advisory{
		ID:       "CVE-2024-12345",
		Severity: "HIGH",
		Details:  "Example Kubernetes CVE for controller-manager.",
		Affected: []catalogtest.Range{
			{Ecosystem: "go", Package: "k8s.io/kubernetes", Introduced: "v1.29.0", Fixed: "v1.29.16"},
		},
	})

	sbom := &models.SBOM{
		OSName:     "distroless",
		OSVersion:  "",
		SbomSource: "distroless-heuristic",
		Confidence: "medium",
		Status:     "finalized",
	}
	if err := db.Create(sbom).Error; err != nil {
		t.Fatalf("create SBOM: %v", err)
	}
	comp := models.SBOMComponent{
		SBOMID:           sbom.ID,
		ComponentName:    "kube-controller-manager",
		ComponentVersion: "v1.29.15",
		PURL:             "pkg:generic/kube-controller-manager@v1.29.15",
		ComponentType:    "application",
		Source:           "distroless-heuristic",
	}
	if err := db.Create(&comp).Error; err != nil {
		t.Fatalf("create SBOMComponent: %v", err)
	}

	mgr := database.NewPostgresManager(db)
	matcher := NewMatcher(mgr, db)
	matches, err := matcher.MatchSBOM(context.Background(), sbom, nil)
	if err != nil {
		t.Fatalf("MatchSBOM: %v", err)
	}

	// Expect at least one CVE match for kube-controller-manager v1.29.15 (within range)
	if len(matches) == 0 {
		t.Fatal("expected at least one CVE match for kube-controller-manager@v1.29.15, got 0")
	}
	var found bool
	for _, m := range matches {
		if m.PackageName == "kube-controller-manager" && m.PackageVersion == "v1.29.15" && m.CVEID == "CVE-2024-12345" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected CVE-2024-12345 for kube-controller-manager@v1.29.15; got %+v", matches)
	}
}

func TestGoStdlibMatcher_VulnerableAndPatched(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.SBOM{}, &models.SBOMComponent{}, &models.CVEMatch{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Seed stdlib vuln: GO-STDLIB-TEST, introduced 0, fixed 1.18.3
	catalogtest.Seed(t, db, catalogtest.Advisory{
		ID: "GO-STDLIB-TEST", Summary: "stdlib vuln", Details: "details", Severity: "HIGH",
		Affected: []catalogtest.Range{{Ecosystem: "Go", Package: "stdlib", Type: "SEMVER", Fixed: "1.18.3"}},
	})

	mgr := database.NewPostgresManager(db)
	matcher := NewMatcher(mgr, db)

	// Vulnerable GoVersion
	sbomVuln := &models.SBOM{GoVersion: "go1.18.1", Status: "finalized"}
	if err := db.Create(sbomVuln).Error; err != nil {
		t.Fatalf("create sbom: %v", err)
	}
	matches, err := matcher.MatchSBOM(context.Background(), sbomVuln, nil)
	if err != nil {
		t.Fatalf("MatchSBOM (vulnerable): %v", err)
	}
	var found bool
	for _, m := range matches {
		if m.PackageName == "stdlib" && m.CVEID == "GO-STDLIB-TEST" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected GO-STDLIB-TEST match for stdlib@1.18.1, got %+v", matches)
	}

	// Patched GoVersion
	sbomPatched := &models.SBOM{GoVersion: "go1.19.0", Status: "finalized"}
	if err := db.Create(sbomPatched).Error; err != nil {
		t.Fatalf("create sbom patched: %v", err)
	}
	matchesPatched, err := matcher.MatchSBOM(context.Background(), sbomPatched, nil)
	if err != nil {
		t.Fatalf("MatchSBOM (patched): %v", err)
	}
	for _, m := range matchesPatched {
		if m.PackageName == "stdlib" && m.CVEID == "GO-STDLIB-TEST" {
			t.Fatalf("did not expect GO-STDLIB-TEST for stdlib@1.19.0, got %+v", matchesPatched)
		}
	}
}

// TestGoModuleAlias_EtcdMatch verifies that when SBOM has github.com/coreos/etcd/client/v3 and
// the catalog has vulns only for go.etcd.io/etcd, alias resolution still produces a CVE match.
func TestGoModuleAlias_EtcdMatch(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.GoModuleAlias{},
		&models.SBOM{}, &models.SBOMComponent{}, &models.CVEMatch{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Alias: old path -> canonical (OSV uses canonical)
	if err := db.Create(&models.GoModuleAlias{Alias: "github.com/coreos/etcd", Canonical: "go.etcd.io/etcd"}).Error; err != nil {
		t.Fatalf("seed alias: %v", err)
	}

	// Advisory only for canonical go.etcd.io/etcd; version range includes v3.3.0
	catalogtest.Seed(t, db, catalogtest.Advisory{
		ID: "GO-ETCD-ALIAS-TEST", Summary: "etcd vuln", Details: "details", Severity: "HIGH",
		Affected: []catalogtest.Range{{Ecosystem: "Go", Package: "go.etcd.io/etcd", Type: "SEMVER", Fixed: "3.3.99"}},
	})

	sbom := &models.SBOM{Status: "finalized"}
	if err := db.Create(sbom).Error; err != nil {
		t.Fatalf("create SBOM: %v", err)
	}
	// SBOM component uses old path (alias); no k8s map, so matcher uses prefix + alias resolution
	comp := models.SBOMComponent{
		SBOMID:           sbom.ID,
		ComponentName:    "github.com/coreos/etcd/client/v3",
		ComponentVersion: "v3.3.0",
		PURL:             "pkg:go/github.com/coreos/etcd/client/v3@v3.3.0",
		ComponentType:    "library",
		Source:           "go-binary",
	}
	if err := db.Create(&comp).Error; err != nil {
		t.Fatalf("create SBOMComponent: %v", err)
	}

	mgr := database.NewPostgresManager(db)
	matcher := NewMatcher(mgr, db)
	matches, err := matcher.MatchSBOM(context.Background(), sbom, nil)
	if err != nil {
		t.Fatalf("MatchSBOM: %v", err)
	}
	var found bool
	for _, m := range matches {
		if m.PackageName == "github.com/coreos/etcd/client/v3" && m.CVEID == "GO-ETCD-ALIAS-TEST" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected GO-ETCD-ALIAS-TEST match for github.com/coreos/etcd/client/v3@v3.3.0 via alias go.etcd.io/etcd; got %+v", matches)
	}
}

func TestNormalizeGoModulePrefixes(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{
			in:   "k8s.io/kubernetes/cmd/kube-apiserver",
			want: []string{"k8s.io/kubernetes/cmd/kube-apiserver", "k8s.io/kubernetes/cmd", "k8s.io/kubernetes"},
		},
		{
			in:   "github.com/labstack/echo/v4/middleware",
			want: []string{"github.com/labstack/echo/v4/middleware", "github.com/labstack/echo/v4"},
		},
		{
			in:   "github.com/org/repo",
			want: []string{"github.com/org/repo"},
		},
		{
			in:   "k8s.io",
			want: []string{"k8s.io"},
		},
	}
	for _, tt := range tests {
		got := normalizeGoModulePrefixes(tt.in)
		if len(got) != len(tt.want) {
			t.Fatalf("normalizeGoModulePrefixes(%q) len=%d, want %d (%v)", tt.in, len(got), len(tt.want), got)
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Fatalf("normalizeGoModulePrefixes(%q)[%d]=%q, want %q", tt.in, i, got[i], tt.want[i])
			}
		}
	}
}

func TestCompareRPMVersion(t *testing.T) {
	vc := NewVersionComparator()

	// Vulnerable: 1.1.1k-4.el8 < 1.1.1k-5.el8
	vuln, err := vc.IsVulnerable("1.1.1k-4.el8", "< 1.1.1k-5.el8", "rpm")
	if err != nil {
		t.Fatalf("IsVulnerable rpm: %v", err)
	}
	if !vuln {
		t.Error("expected 1.1.1k-4.el8 to be vulnerable to < 1.1.1k-5.el8")
	}

	// Not vulnerable: 1.1.1k-6.el8 >= 1.1.1k-5.el8
	notVuln, err := vc.IsVulnerable("1.1.1k-6.el8", "< 1.1.1k-5.el8", "rpm")
	if err != nil {
		t.Fatalf("IsVulnerable rpm: %v", err)
	}
	if notVuln {
		t.Error("expected 1.1.1k-6.el8 not to be vulnerable to < 1.1.1k-5.el8")
	}
}

func TestVersionComparator_UnknownEcosystem_Fallback(t *testing.T) {
	vc := NewVersionComparator()

	// Semver fallback path for unknown ecosystems.
	vuln, err := vc.IsVulnerable("1.2.0", "< 2.0.0", "custom-eco")
	if err != nil {
		t.Fatalf("IsVulnerable unknown ecosystem: %v", err)
	}
	if !vuln {
		t.Fatalf("expected semver fallback to mark 1.2.0 vulnerable for <2.0.0")
	}

	// Exact equality fallback path (when semver parsing might fail).
	vulnEq, err := vc.IsVulnerable("v1", "== v1", "totally-unknown")
	if err != nil {
		t.Fatalf("IsVulnerable unknown ecosystem eq fallback: %v", err)
	}
	if !vulnEq {
		t.Fatalf("expected equality fallback to mark v1 == v1 as vulnerable")
	}
}

func TestPURLParser_GolangAlias(t *testing.T) {
	p, err := ParsePURL("pkg:golang/github.com/a/b/c@v1.2.3")
	if err != nil {
		t.Fatalf("ParsePURL: %v", err)
	}
	if p.Ecosystem != "go" {
		t.Fatalf("ecosystem=%q, want go", p.Ecosystem)
	}
	if p.Name != "github.com/a/b/c" {
		t.Fatalf("name=%q, want github.com/a/b/c", p.Name)
	}
	if p.Version != "v1.2.3" {
		t.Fatalf("version=%q, want v1.2.3", p.Version)
	}
}

func TestMatcher_FullModulePathPreserved(t *testing.T) {
	p, err := ParsePURL("pkg:go/github.com/a/b/c/d@v1.0.0")
	if err != nil {
		t.Fatalf("ParsePURL: %v", err)
	}
	if p.Ecosystem != "go" {
		t.Fatalf("ecosystem=%q, want go", p.Ecosystem)
	}
	if p.Name != "github.com/a/b/c/d" {
		t.Fatalf("name=%q, want github.com/a/b/c/d", p.Name)
	}
}

func TestMatcher_ConflictResolution_PrefersGobinaryOverGomod(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.SBOM{}, &models.SBOMComponent{}, &models.CVEMatch{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// Seed vuln for module github.com/a/b; range includes v1.1.0 but not v1.2.0
	catalogtest.Seed(t, db, catalogtest.Advisory{
		ID: "GO-CONFLICT-TEST", Summary: "test", Details: "d", Severity: "HIGH",
		Affected: []catalogtest.Range{{Ecosystem: "Go", Package: "github.com/a/b", Type: "SEMVER", Fixed: "1.1.99"}},
	})

	sbom := &models.SBOM{Status: "finalized"}
	if err := db.Create(sbom).Error; err != nil {
		t.Fatalf("create sbom: %v", err)
	}

	// Same canonical component (pkg:go/github.com/a/b@...), different sources and versions:
	// - gobinary says v1.1.0 (vulnerable)
	// - gomod says v1.2.0 (NOT vulnerable)
	// We expect resolver prefers gobinary -> produces a match.
	override := []*models.SBOMComponent{
		{
			SBOMID:           sbom.ID,
			ComponentName:    "github.com/a/b",
			ComponentVersion: "v1.2.0",
			PURL:             "pkg:go/github.com/a/b@v1.2.0",
			Source:           "gomod",
		},
		{
			SBOMID:           sbom.ID,
			ComponentName:    "github.com/a/b",
			ComponentVersion: "v1.1.0",
			PURL:             "pkg:go/github.com/a/b@v1.1.0",
			Source:           "gobinary",
		},
	}

	mgr := database.NewPostgresManager(db)
	m := NewMatcher(mgr, db)
	matches, err := m.MatchSBOM(context.Background(), sbom, override)
	if err != nil {
		t.Fatalf("MatchSBOM: %v", err)
	}
	var found bool
	for _, mm := range matches {
		if mm.CVEID == "GO-CONFLICT-TEST" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected GO-CONFLICT-TEST match after resolver; got %+v", matches)
	}
}

func TestMatcher_TrustLevel_GracefulDegradation_AllLowAllowed(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.SBOM{}, &models.SBOMComponent{}, &models.CVEMatch{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	catalogtest.Seed(t, db, catalogtest.Advisory{
		ID: "GO-LOW-ONLY", Summary: "test", Details: "d", Severity: "HIGH",
		Affected: []catalogtest.Range{{Ecosystem: "Go", Package: "github.com/a/b", Type: "SEMVER", Fixed: "1.1.99"}},
	})

	sbom := &models.SBOM{Status: "finalized"}
	_ = db.Create(sbom).Error

	override := []*models.SBOMComponent{
		{
			SBOMID:           sbom.ID,
			ComponentName:    "github.com/a/b",
			ComponentVersion: "v1.1.0",
			PURL:             "pkg:go/github.com/a/b@v1.1.0",
			Source:           "distroless-heuristic",
			TrustLevel:       "low",
		},
	}

	mgr := database.NewPostgresManager(db)
	m := NewMatcher(mgr, db)
	matches, err := m.MatchSBOM(context.Background(), sbom, override)
	if err != nil {
		t.Fatalf("MatchSBOM: %v", err)
	}
	if len(matches) == 0 {
		t.Fatalf("expected match in fallback mode when all components are LOW trust")
	}
}

func TestMatcher_TrustLevel_LowSkippedWhenNonLowExists(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.SBOM{}, &models.SBOMComponent{}, &models.CVEMatch{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// Vulnerability only for github.com/a/b (low trust component).
	catalogtest.Seed(t, db, catalogtest.Advisory{
		ID: "GO-LOW-SKIP", Summary: "test", Details: "d", Severity: "HIGH",
		Affected: []catalogtest.Range{{Ecosystem: "Go", Package: "github.com/a/b", Type: "SEMVER", Fixed: "1.1.99"}},
	})

	sbom := &models.SBOM{Status: "finalized"}
	_ = db.Create(sbom).Error

	override := []*models.SBOMComponent{
		// High trust unrelated component (ensures hasNonLow=true).
		{
			SBOMID:           sbom.ID,
			ComponentName:    "github.com/x/y",
			ComponentVersion: "v9.9.9",
			PURL:             "pkg:go/github.com/x/y@v9.9.9",
			Source:           "gobinary",
			TrustLevel:       "high",
		},
		// Low trust vulnerable component should be skipped -> no matches.
		{
			SBOMID:           sbom.ID,
			ComponentName:    "github.com/a/b",
			ComponentVersion: "v1.1.0",
			PURL:             "pkg:go/github.com/a/b@v1.1.0",
			Source:           "distroless-heuristic",
			TrustLevel:       "low",
		},
	}

	mgr := database.NewPostgresManager(db)
	m := NewMatcher(mgr, db)
	matches, err := m.MatchSBOM(context.Background(), sbom, override)
	if err != nil {
		t.Fatalf("MatchSBOM: %v", err)
	}
	for _, mm := range matches {
		if mm.CVEID == "GO-LOW-SKIP" {
			t.Fatalf("expected low-trust component to be skipped when non-low exists; got %+v", matches)
		}
	}
}

func TestMatcher_ResolveAfterTrustFilter(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.SBOM{}, &models.SBOMComponent{}, &models.CVEMatch{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// Vulnerability affects github.com/a/b in v1.1.x.
	catalogtest.Seed(t, db, catalogtest.Advisory{
		ID: "GO-TRUST-ORDER", Summary: "test", Details: "d", Severity: "HIGH",
		Affected: []catalogtest.Range{{Ecosystem: "Go", Package: "github.com/a/b", Type: "SEMVER", Fixed: "1.1.99"}},
	})

	sbom := &models.SBOM{Status: "finalized"}
	_ = db.Create(sbom).Error

	override := []*models.SBOMComponent{
		// LOW trust gobinary (would win by priority if resolver ran before trust filter).
		{
			SBOMID:           sbom.ID,
			ComponentName:    "github.com/a/b",
			ComponentVersion: "v1.1.0",
			PURL:             "pkg:go/github.com/a/b@v1.1.0",
			Source:           "gobinary",
			TrustLevel:       "low",
		},
		// HIGH trust gomod (should be chosen and LOW dropped before resolution).
		{
			SBOMID:           sbom.ID,
			ComponentName:    "github.com/a/b",
			ComponentVersion: "v1.2.0",
			PURL:             "pkg:go/github.com/a/b@v1.2.0",
			Source:           "gomod",
			TrustLevel:       "high",
		},
	}

	mgr := database.NewPostgresManager(db)
	m := NewMatcher(mgr, db)
	matches, err := m.MatchSBOM(context.Background(), sbom, override)
	if err != nil {
		t.Fatalf("MatchSBOM: %v", err)
	}
	for _, mm := range matches {
		if mm.CVEID == "GO-TRUST-ORDER" {
			t.Fatalf("expected LOW-trust gobinary to be dropped before resolution, so no match; got %+v", matches)
		}
	}
}

func TestMatcher_FallbackMode_ComponentLimit(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.SBOM{}, &models.SBOMComponent{}, &models.CVEMatch{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sbom := &models.SBOM{Status: "finalized"}
	_ = db.Create(sbom).Error

	// 100 low-trust components (no vulns needed; we just ensure it doesn't error).
	override := make([]*models.SBOMComponent, 0, 100)
	for i := 0; i < 100; i++ {
		override = append(override, &models.SBOMComponent{
			SBOMID:           sbom.ID,
			ComponentName:    fmt.Sprintf("github.com/a/b%d", i),
			ComponentVersion: "v1.0.0",
			PURL:             fmt.Sprintf("pkg:go/github.com/a/b%d@v1.0.0", i),
			Source:           "distroless-heuristic",
			TrustLevel:       "low",
		})
	}

	mgr := database.NewPostgresManager(db)
	m := NewMatcher(mgr, db)
	_, err = m.MatchSBOM(context.Background(), sbom, override)
	if err != nil {
		t.Fatalf("MatchSBOM: %v", err)
	}
}

func TestMatcher_CVECap_PrioritizesSeverityBeforeLimit(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.SBOM{}, &models.SBOMComponent{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Seed 5 CRITICAL + 25 LOW for same package (total 30 > cap 25).
	var advs []catalogtest.Advisory
	for i := 1; i <= 5; i++ {
		advs = append(advs, catalogtest.Advisory{
			ID:       fmt.Sprintf("CVE-D3-CRIT-%02d", i),
			Severity: "CRITICAL",
			CVSSv3:   "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H", // 9.8
			Details:  "critical test",
			Affected: []catalogtest.Range{{Ecosystem: "debian", Package: "openssl", LastAffected: "9.9.9"}},
		})
	}
	for i := 1; i <= 25; i++ {
		advs = append(advs, catalogtest.Advisory{
			ID:       fmt.Sprintf("CVE-D3-LOW-%02d", i),
			Severity: "LOW",
			CVSSv3:   "CVSS:3.1/AV:N/AC:H/PR:H/UI:R/S:U/C:L/I:N/A:N", // 2.0
			Details:  "low test",
			Affected: []catalogtest.Range{{Ecosystem: "debian", Package: "openssl", LastAffected: "9.9.9"}},
		})
	}
	catalogtest.Seed(t, db, advs...)

	sb := &models.SBOM{OSName: "debian", OSVersion: "12", Status: "finalized"}
	if err := db.Create(sb).Error; err != nil {
		t.Fatalf("create sbom: %v", err)
	}
	comp := []*models.SBOMComponent{
		{
			SBOMID:           sb.ID,
			ComponentName:    "openssl",
			ComponentVersion: "1.0.0",
			PURL:             "pkg:generic/openssl@1.0.0",
			Source:           "rpmdb-fallback",
			TrustLevel:       "high",
		},
	}

	mgr := database.NewPostgresManager(db)
	m := NewMatcher(mgr, db)
	matches, err := m.MatchSBOM(context.Background(), sb, comp)
	if err != nil {
		t.Fatalf("MatchSBOM: %v", err)
	}
	if len(matches) != 25 {
		t.Fatalf("expected cap 25 matches, got %d", len(matches))
	}

	criticalCount := 0
	for _, mm := range matches {
		if strings.HasPrefix(mm.CVEID, "CVE-D3-CRIT-") {
			criticalCount++
		}
	}
	if criticalCount != 5 {
		t.Fatalf("expected all 5 critical CVEs retained before cap, got %d/%d", criticalCount, 5)
	}
}

func TestMatcher_OSNamespaceAndArch_NotCollapsed(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	mgr := database.NewPostgresManager(db)
	m := NewMatcher(mgr, db)

	sbom := &models.SBOM{ID: 1, Status: "finalized", OSName: "debian"}
	in := []models.SBOMComponent{
		{SBOMID: sbom.ID, ComponentName: "openssl", ComponentVersion: "1.1.1", PURL: "pkg:deb/debian/openssl@1.1.1?arch=amd64", Source: "os", TrustLevel: "high"},
		{SBOMID: sbom.ID, ComponentName: "openssl", ComponentVersion: "1.1.1", PURL: "pkg:deb/ubuntu/openssl@1.1.1?arch=amd64", Source: "os", TrustLevel: "high"},
		{SBOMID: sbom.ID, ComponentName: "openssl", ComponentVersion: "1.1.1", PURL: "pkg:deb/debian/openssl@1.1.1?arch=arm64", Source: "os", TrustLevel: "high"},
	}
	out := m.resolveComponentsForMatching(context.Background(), sbom, in)
	// All three must survive: different namespace and arch are distinct identities.
	if len(out) != 3 {
		t.Fatalf("expected 3 components, got %d", len(out))
	}
}

func TestMatchSBOM_MultipleArchComponentsSamePackage_AllEvaluated(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.SBOM{}, &models.SBOMComponent{}, &models.CVEMatch{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	catalogtest.Seed(t, db, catalogtest.Advisory{
		ID:       "CVE-ARCH-0001",
		Severity: "HIGH",
		Details:  "arch regression test",
		Affected: []catalogtest.Range{{Ecosystem: "debian", Package: "openssl", Introduced: "1.0.0", Fixed: "2.0.0"}},
	})

	sbom := &models.SBOM{Status: "finalized", OSName: "debian"}
	if err := db.Create(sbom).Error; err != nil {
		t.Fatalf("create sbom: %v", err)
	}
	override := []*models.SBOMComponent{
		{SBOMID: sbom.ID, ComponentName: "openssl", ComponentVersion: "1.1.1", PURL: "pkg:deb/debian/openssl@1.1.1?arch=amd64", Source: "os", TrustLevel: "high"},
		{SBOMID: sbom.ID, ComponentName: "openssl", ComponentVersion: "1.1.1", PURL: "pkg:deb/debian/openssl@1.1.1?arch=arm64", Source: "os", TrustLevel: "high"},
	}

	m := NewMatcher(database.NewPostgresManager(db), db)
	matches, err := m.MatchSBOM(context.Background(), sbom, override)
	if err != nil {
		t.Fatalf("MatchSBOM: %v", err)
	}
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches (both arch components evaluated), got %d", len(matches))
	}
}

func TestMatcher_ShadowMetrics_InvalidPURL_Increments(t *testing.T) {
	before := testutil.ToFloat64(metrics.MatcherComponentsShadowedTotal.WithLabelValues("invalid"))

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	mgr := database.NewPostgresManager(db)
	m := NewMatcher(mgr, db)
	sbom := &models.SBOM{ID: 1, Status: "finalized"}

	in := []models.SBOMComponent{
		{SBOMID: sbom.ID, ComponentName: "x", ComponentVersion: "1", PURL: "not-a-purl", Source: "os", TrustLevel: "high"},
	}
	_ = m.resolveComponentsForMatching(context.Background(), sbom, in)

	after := testutil.ToFloat64(metrics.MatcherComponentsShadowedTotal.WithLabelValues("invalid"))
	if after != before+1 {
		t.Fatalf("expected invalid shadow metric to increment by 1 (before=%v after=%v)", before, after)
	}
}

func TestMatcher_VulnerabilityDecisionMetrics(t *testing.T) {
	candidateBefore := testutil.ToFloat64(metrics.MatcherVulnerabilityCandidatesTotal.WithLabelValues("pypi", ResolverVersion))
	versionMatchBefore := testutil.ToFloat64(metrics.MatcherVulnerabilityVersionMatchesTotal.WithLabelValues("pypi", ResolverVersion))
	notVulnerableBefore := testutil.ToFloat64(metrics.MatcherVulnerabilitySkipsTotal.WithLabelValues("not_vulnerable", "pypi", ResolverVersion))

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SBOM{}, &models.SBOMComponent{}))

	catalogtest.Seed(t, db,
		catalogtest.Advisory{ID: "CVE-2026-0001", Severity: "HIGH", Details: "matched", Affected: []catalogtest.Range{
			{Ecosystem: "PyPI", Package: "django", Fixed: "2.0.0"},
		}},
		catalogtest.Advisory{ID: "CVE-2026-0002", Severity: "HIGH", Details: "not affected", Affected: []catalogtest.Range{
			{Ecosystem: "PyPI", Package: "django", Introduced: "3.0.0", Fixed: "3.0.5"},
		}},
	)

	sbom := &models.SBOM{Status: "finalized"}
	component := &models.SBOMComponent{
		SBOMID:           sbom.ID,
		ComponentName:    "django",
		ComponentVersion: "1.11.0",
		PURL:             "pkg:pypi/django@1.11.0",
		Source:           "language",
		TrustLevel:       "high",
	}

	m := NewMatcher(database.NewPostgresManager(db), db)
	matches, err := m.MatchSBOM(context.Background(), sbom, []*models.SBOMComponent{component})
	require.NoError(t, err)
	require.Len(t, matches, 1)

	require.Equal(t, candidateBefore+2, testutil.ToFloat64(metrics.MatcherVulnerabilityCandidatesTotal.WithLabelValues("pypi", ResolverVersion)))
	require.Equal(t, versionMatchBefore+1, testutil.ToFloat64(metrics.MatcherVulnerabilityVersionMatchesTotal.WithLabelValues("pypi", ResolverVersion)))
	require.Equal(t, notVulnerableBefore+1, testutil.ToFloat64(metrics.MatcherVulnerabilitySkipsTotal.WithLabelValues("not_vulnerable", "pypi", ResolverVersion)))
}

func TestMatcher_GobinaryMain_KeptForMatching(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	mgr := database.NewPostgresManager(db)
	m := NewMatcher(mgr, db)
	sbom := &models.SBOM{ID: 1, Status: "finalized"}

	in := []models.SBOMComponent{
		{SBOMID: sbom.ID, ComponentName: "k8s.io/kubernetes", ComponentVersion: "v1.29.15", PURL: "pkg:golang/k8s.io/kubernetes@v1.29.15", Source: "gobinary-main", TrustLevel: "high"},
		{SBOMID: sbom.ID, ComponentName: "k8s.io/apiserver", ComponentVersion: "v0.29.15", PURL: "pkg:go/k8s.io/apiserver@v0.29.15", Source: "gobinary", TrustLevel: "high"},
	}
	out := m.resolveComponentsForMatching(context.Background(), sbom, in)
	if len(out) != 2 {
		t.Fatalf("expected gobinary-main to be kept (got %d components, want 2)", len(out))
	}
	found := false
	for _, c := range out {
		if c.Source == "gobinary-main" {
			found = true
		}
	}
	if !found {
		t.Fatal("gobinary-main component was filtered out but should be kept for CVE matching")
	}
}

func TestMatcher_DeterministicOutput_SameInputSameResult(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.SBOM{}, &models.SBOMComponent{}, &models.CVEMatch{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// Two vulns for same module so output order matters.
	catalogtest.Seed(t, db,
		catalogtest.Advisory{ID: "GO-DET-1", Summary: "t", Details: "d", Severity: "HIGH",
			Affected: []catalogtest.Range{{Ecosystem: "Go", Package: "github.com/a/b", Type: "SEMVER", Fixed: "9.9.9"}}},
		catalogtest.Advisory{ID: "GO-DET-2", Summary: "t", Details: "d", Severity: "MEDIUM",
			Affected: []catalogtest.Range{{Ecosystem: "Go", Package: "github.com/a/b", Type: "SEMVER", Fixed: "9.9.9"}}},
	)

	sbom := &models.SBOM{Status: "finalized"}
	_ = db.Create(sbom).Error
	override := []*models.SBOMComponent{
		{
			SBOMID:           sbom.ID,
			ComponentName:    "github.com/a/b",
			ComponentVersion: "v1.0.0",
			PURL:             "pkg:go/github.com/a/b@v1.0.0",
			Source:           "gobinary",
			TrustLevel:       "high",
		},
	}

	mgr := database.NewPostgresManager(db)
	m := NewMatcher(mgr, db)

	var baseline string
	for i := 0; i < 100; i++ {
		matches, err := m.MatchSBOM(context.Background(), sbom, override)
		if err != nil {
			t.Fatalf("MatchSBOM: %v", err)
		}
		// Serialize deterministically by the sorted order in MatchSBOM.
		var b strings.Builder
		for _, mm := range matches {
			b.WriteString(mm.PackageName)
			b.WriteString("|")
			b.WriteString(mm.PackageVersion)
			b.WriteString("|")
			b.WriteString(mm.CVEID)
			b.WriteString("|")
			b.WriteString(mm.PURL)
			b.WriteString("\n")
		}
		if i == 0 {
			baseline = b.String()
		} else if b.String() != baseline {
			t.Fatalf("nondeterministic output at iter=%d\nbaseline:\n%s\ngot:\n%s", i, baseline, b.String())
		}
	}
}

// An Alpine advisory aliasing a CVE is reported under the CVE.
func TestMatchSBOM_AlpineBusyboxMatchesAdvisory(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SBOM{}, &models.SBOMComponent{}, &models.CVEMatch{}))
	catalogtest.Seed(t, db, catalogtest.Advisory{
		ID: "ALPINE-CVE-2023-42363", Summary: "busybox", Details: "x", Severity: "HIGH",
		Aliases:  []string{"CVE-2023-42363"},
		Affected: []catalogtest.Range{{Ecosystem: "Alpine", Package: "busybox", Fixed: "1.99.0"}},
	})

	sb := &models.SBOM{PodUID: "pod-1", OSName: "alpine", Status: "complete", PackageCount: 1}
	require.NoError(t, db.Create(sb).Error)
	comp := []*models.SBOMComponent{{
		SBOMID: sb.ID, ComponentName: "busybox", ComponentVersion: "1.36.1-r15",
		PURL: "pkg:apk/alpine/busybox@1.36.1-r15?arch=x86_64", TrustLevel: "high",
	}}

	mgr := database.NewPostgresManager(db)
	m := NewMatcher(mgr, db)
	matches, err := m.MatchSBOM(context.Background(), sb, comp)
	require.NoError(t, err)
	require.Len(t, matches, 1)
	require.Equal(t, "CVE-2023-42363", matches[0].CVEID)
	require.Equal(t, "fortuna-core-cve-matcher", matches[0].MatchedBy)
}

func TestMatchSBOM_SkipsFailedSBOMStatus(t *testing.T) {
	m := NewMatcher(nil, nil)
	sbom := &models.SBOM{ID: 1, Status: "failed"}
	matches, err := m.MatchSBOM(context.Background(), sbom, nil)
	require.NoError(t, err)
	require.Nil(t, matches)
}

func TestMatchSBOM_SkipsPendingSBOMStatus(t *testing.T) {
	m := NewMatcher(nil, nil)
	sbom := &models.SBOM{ID: 1, Status: "pending"}
	matches, err := m.MatchSBOM(context.Background(), sbom, nil)
	require.NoError(t, err)
	require.Nil(t, matches)
}
