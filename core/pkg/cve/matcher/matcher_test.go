package matcher

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/fortuna/core/pkg/cve/database"
	"github.com/fortuna/core/pkg/cve/database/nvd"
	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

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

func TestNormalizeComponentNameForNVD(t *testing.T) {
	tests := []struct{ in, want string }{
		{"coredns", "coredns"},
		{"coredns/coredns", "coredns"},
		{"registry.k8s.io/coredns/coredns", "coredns"},
		{"registry.k8s.io/coredns", "coredns"}, // no slash: use registryCanonicalName map
		{"kube-apiserver", "kube-apiserver"},
		{"openssl", "openssl"},
		{"  coredns  ", "coredns"},
	}
	for _, tt := range tests {
		got := normalizeComponentNameForNVD(tt.in)
		if got != tt.want {
			t.Errorf("normalizeComponentNameForNVD(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestIsNVDFallbackWhitelisted(t *testing.T) {
	allowed := []string{
		"kube-apiserver", "kube-controller-manager", "kube-scheduler", "kube-proxy",
		"coredns", "etcd", "pause", "openssl", "libc.so.6", "libssl.so.3",
		"containerd-shim", "runc", "glibc", "libfoo.so.1",
		"registry.k8s.io/coredns", "coredns/coredns", // normalized to coredns → whitelisted
	}
	for _, name := range allowed {
		if !isNVDFallbackWhitelisted(name) {
			t.Errorf("isNVDFallbackWhitelisted(%q) = false, want true (NVD query allowed)", name)
		}
	}
	rejected := []string{"Extend.pl", "ISO-IR-197.so", "Y.pl", "unknown-image", "nginx"}
	for _, name := range rejected {
		if isNVDFallbackWhitelisted(name) {
			t.Errorf("isNVDFallbackWhitelisted(%q) = true, want false (no NVD spam)", name)
		}
	}
}

func TestIsDistrolessHeuristicJunk(t *testing.T) {
	// Junk: version=unknown, source=distroless-heuristic, name not whitelisted -> skip
	if !isDistrolessHeuristicJunk(&models.SBOMComponent{
		ComponentName: "Extend.pl", ComponentVersion: "unknown", Source: "distroless-heuristic",
	}) {
		t.Error("Extend.pl@unknown distroless-heuristic should be junk (skipped)")
	}
	if !isDistrolessHeuristicJunk(&models.SBOMComponent{
		ComponentName: "ISO-IR-197.so", ComponentVersion: "unknown", Source: "distroless-heuristic",
	}) {
		t.Error("ISO-IR-197.so@unknown distroless-heuristic should be junk (skipped)")
	}
	// Not junk: whitelisted names even with unknown -> keep (will query Postgres/NVD)
	if isDistrolessHeuristicJunk(&models.SBOMComponent{
		ComponentName: "coredns", ComponentVersion: "unknown", Source: "distroless-heuristic",
	}) {
		t.Error("coredns@unknown should not be junk (NVD fallback allowed)")
	}
	if isDistrolessHeuristicJunk(&models.SBOMComponent{
		ComponentName: "kube-apiserver", ComponentVersion: "unknown", Source: "distroless-heuristic",
	}) {
		t.Error("kube-apiserver@unknown should not be junk (NVD fallback allowed)")
	}
	// Not junk: has version -> keep
	if isDistrolessHeuristicJunk(&models.SBOMComponent{
		ComponentName: "Extend.pl", ComponentVersion: "1.0", Source: "distroless-heuristic",
	}) {
		t.Error("Extend.pl@1.0 should not be junk (has version)")
	}
	// Not junk: source not distroless-heuristic -> keep
	if isDistrolessHeuristicJunk(&models.SBOMComponent{
		ComponentName: "openssl", ComponentVersion: "unknown", Source: "parsers",
	}) {
		t.Error("openssl@unknown from parsers should not be junk")
	}
}

// TestNVDQueryEcosystem ensures generic components on Debian use ecosystem "debian"
// so that Postgres OSV data is queried first; NVD is then used only for whitelisted names.
func TestNVDQueryEcosystem(t *testing.T) {
	// Simulate: SBOM from Debian image, component openssl generic (e.g. from heuristic).
	// Query ecosystem must be "debian" to hit OSV loader data.
	p, _ := ParsePURL("pkg:generic/openssl@3.0.18-1~deb12u2")
	eco := normalizeQueryEcosystemWithOS(p, "debian")
	if eco != "debian" {
		t.Errorf("generic openssl on Debian should query ecosystem debian for OSV; got %q", eco)
	}
	// Whitelisted name -> NVD fallback allowed when Postgres returns 0
	if !isNVDFallbackWhitelisted("openssl") {
		t.Error("openssl must be whitelisted for NVD fallback")
	}
	// Non-whitelisted -> no NVD call
	if isNVDFallbackWhitelisted("Extend.pl") {
		t.Error("Extend.pl must not trigger NVD fallback")
	}
}

// TestCVEFromNVDFlow documents that TryNVDFallback is true only for whitelisted names,
// so CVE lookup will query NVD API only for control-plane/runtime (coredns, kube-*, openssl, etc.).
func TestCVEFromNVDFlow(t *testing.T) {
	type row struct {
		componentName string
		tryNVD        bool
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
		got := isNVDFallbackWhitelisted(c.componentName)
		if got != c.tryNVD {
			t.Errorf("component %q: TryNVDFallback = %v, want %v (CVE from NVD when true)", c.componentName, got, c.tryNVD)
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
	if err := db.AutoMigrate(&models.SBOM{}, &models.SBOMComponent{}, &models.CVE{}, &models.PackageVulnerability{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Seed CVE and package_vulnerabilities (as OSV loader would)
	now := time.Now()
	cveRow := models.CVE{
		CVEID:            "CVE-2025-15467",
		Severity:         "HIGH",
		CVSSScore:        7.5,
		Description:      "Stack buffer overflow in OpenSSL (Debian 12 bookworm).",
		PublishedDate:     &now,
		LastModifiedDate: &now,
	}
	if err := db.Create(&cveRow).Error; err != nil {
		t.Fatalf("create CVE: %v", err)
	}
	pv := models.PackageVulnerability{
		CVEID:                 "CVE-2025-15467",
		Ecosystem:             "debian",
		PackageName:           "openssl",
		VersionEndIncluding:   "3.0.18-1~deb12u2", // affected up to and including this version
		VersionEndExcluding:   "",
		VersionStartIncluding: "",
		FixedVersion:          "3.0.19-1",
	}
	if err := db.Create(&pv).Error; err != nil {
		t.Fatalf("create PackageVulnerability: %v", err)
	}

	// SBOM for Debian 12 with openssl 3.0.18-1~deb12u2
	sbom := &models.SBOM{
		OSName:    "debian",
		OSVersion: "12",
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
	if err := db.AutoMigrate(&models.SBOM{}, &models.SBOMComponent{}, &models.CVE{}, &models.PackageVulnerability{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	now := time.Now()
	if err := db.Create(&models.CVE{
		CVEID: "CVE-2025-15467", Severity: "HIGH", CVSSScore: 7.5,
		PublishedDate: &now, LastModifiedDate: &now,
	}).Error; err != nil {
		t.Fatalf("create CVE: %v", err)
	}
	if err := db.Create(&models.PackageVulnerability{
		CVEID:               "CVE-2025-15467",
		Ecosystem:           "debian",
		PackageName:         "openssl",
		VersionEndIncluding: "3.0.18-1~deb12u2",
		FixedVersion:        "3.0.19-1",
	}).Error; err != nil {
		t.Fatalf("create PackageVulnerability: %v", err)
	}

	sbom := &models.SBOM{OSName: "debian", OSVersion: "12"}
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
	if err := db.AutoMigrate(&models.SBOM{}, &models.SBOMComponent{}, &models.CVE{}, &models.PackageVulnerability{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	now := time.Now()
	// Example CVE for Kubernetes controller-manager (version range that includes v1.29.15)
	if err := db.Create(&models.CVE{
		CVEID: "CVE-2024-12345", Severity: "HIGH", CVSSScore: 8.1,
		Description: "Example Kubernetes CVE for controller-manager.",
		PublishedDate: &now, LastModifiedDate: &now,
	}).Error; err != nil {
		t.Fatalf("create CVE: %v", err)
	}
	if err := db.Create(&models.PackageVulnerability{
		CVEID:                 "CVE-2024-12345",
		Ecosystem:             "go",
		PackageName:           "k8s.io/kubernetes",
		VersionStartIncluding: "v1.29.0",
		VersionEndExcluding:   "v1.29.16",
		FixedVersion:          "v1.29.16",
	}).Error; err != nil {
		t.Fatalf("create PackageVulnerability: %v", err)
	}

	sbom := &models.SBOM{
		OSName:      "distroless",
		OSVersion:   "",
		SbomSource:  "distroless-heuristic",
		Confidence:  "medium",
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

// TestControlPlane_KubeControllerManager_NVDIntegration runs full CVE flow with real NVD
// for kube-controller-manager:v1.29.15 when RUN_NVD_INTEGRATION=1 or NVD_API_KEY is set.
// It logs how many CVEs NVD returns and how many matches the matcher produces.
//
// Run: RUN_NVD_INTEGRATION=1 go test -v -run TestControlPlane_KubeControllerManager_NVDIntegration ./core/pkg/cve/matcher/
func TestControlPlane_KubeControllerManager_NVDIntegration(t *testing.T) {
	if os.Getenv("RUN_NVD_INTEGRATION") != "1" && os.Getenv("NVD_API_KEY") == "" {
		t.Skip("Skipping NVD integration (set RUN_NVD_INTEGRATION=1 or NVD_API_KEY to run)")
	}
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.SBOM{}, &models.SBOMComponent{}, &models.CVE{}, &models.PackageVulnerability{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	sbom := &models.SBOM{
		OSName: "distroless", SbomSource: "distroless-heuristic", Confidence: "medium",
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

	nvdClient := nvd.NewClient()
	if key := os.Getenv("NVD_API_KEY"); key != "" {
		nvdClient.SetAPIKey(key)
	}
	mgr := database.NewPostgresManagerWithNVD(db, nvdClient)
	matcher := NewMatcher(mgr, db)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	matches, err := matcher.MatchSBOM(ctx, sbom, nil)
	if err != nil {
		t.Fatalf("MatchSBOM: %v", err)
	}

	t.Logf("=== Control-plane kube-controller-manager:v1.29.15 CVE flow (with NVD) ===")
	t.Logf("Match count after MatchSBOM: %d", len(matches))
	for i, m := range matches {
		t.Logf("  [%d] CVEID=%s Package=%s@%s Severity=%s CVSS=%.1f",
			i+1, m.CVEID, m.PackageName, m.PackageVersion, m.Severity, m.CVSS)
	}
}
