package worker

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/cve/database"
	"github.com/fortuna/core/pkg/cve/matcher"
	"github.com/fortuna/core/pkg/malware"
	"github.com/fortuna/core/pkg/models"
)

// SBOM E2E / integration test inventory (Fortuna Core):
//
//	internal/grpc/
//	  sbom_e2e_go_purl_test.go          — SendSBOMFinding + Go multi-segment PURL + OSV mirror path
//	  sbom_e2e_debian_epoch_arch_test.go — Debian epoch/arch PURL + seeded package_vulnerabilities
//	  handler_sbom_purl_sanitize_test.go — PURL validation / regeneration on ingest
//	  handler_sbom_guard_test.go       — Monotonic SBOM guard (cannot bypass finalized SBOM)
//	  handler_sbom_correlation_test.go — Correlation ID from gRPC context
//	  sbom_publish_retry_test.go       — NATS publish sbom.created with retry/DLQ
//	pkg/worker/
//	  malware_e2e_test.go              — Malware + threat summary + loader (SBOM components)
//	  replay_determinism_e2e_test.go   — CVE worker replay + sbom_processing_state
//	  cve_matcher_worker_integration_test.go — Full distroless→NVD (RUN_NVD_INTEGRATION=1)
//	  sbom_worker_phase_policy_test.go — Pod phase gating for SBOM processing
//	  sbom_dlq_worker_test.go          — DLQ replay max attempts
//	  sbom_coverage_stats_test.go      — Coverage stats invariants
//	  sbom_realistic_pod_e2e_test.go   — THIS FILE: realistic pod metadata + DB-backed CVE/malware
//	pkg/sbom/events_contract_test.go — sbom.created event JSON contract
//	internal/repository/sbom_repository_test.go — Finalized SBOM mutation rules
//	pkg/reconciler/sbom_reconciler_test.go       — Reconciler behaviour
//	internal/api/malware_handlers_e2e_test.go    — REST malware API (uses SBOM pod_uid)
//	pkg/malware/aikido_syncer_e2e_test.go        — Aikido feed sync

func openRealisticPodE2EDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(
		&models.SBOM{},
		&models.SBOMComponent{},
		&models.CVE{},
		&models.PackageVulnerability{},
		&models.CVEMatch{},
		&models.MalwarePackage{},
		&models.MalwareMatch{},
	))
	require.NoError(t, db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_cve_matches_unique_sbom_pkg_cve ON cve_matches(sbom_id, package_name, cve_id)").Error)
	require.NoError(t, db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_malware_pkg_ver ON malware_packages(package_name, version)").Error)
	return db
}

func seedCVEWithPackageVuln(t *testing.T, db *gorm.DB, cveID, eco, pkg string, endExcl, fixed string) {
	t.Helper()
	now := time.Now()
	require.NoError(t, db.Create(&models.CVE{
		CVEID:            cveID,
		Severity:         "HIGH",
		CVSSScore:        7.5,
		Description:      "realistic pod e2e fixture",
		Source:           "osv",
		PublishedDate:    &now,
		LastModifiedDate: &now,
		ExploitSources:   pq.StringArray{},
		CWEIDs:           pq.StringArray{},
	}).Error)
	require.NoError(t, db.Create(&models.PackageVulnerability{
		CVEID:                 cveID,
		PackageName:           pkg,
		Ecosystem:             eco,
		PackageType:           eco,
		VersionEndExcluding:   endExcl,
		FixedVersion:          fixed,
		FixedInVersions:       pq.StringArray{},
	}).Error)
}

// TC-REAL-001: Pod giống CoreDNS trên Debian (kube-system) — openssl trong DB có CVE, matcher phải match.
func TestE2E_RealisticPod_KubeSystem_CoreDNSStyle_DebianOpenSSL_FromDB(t *testing.T) {
	db := openRealisticPodE2EDB(t)
	seedCVEWithPackageVuln(t, db, "CVE-REAL-DEB-OPENSSL-001", "debian", "openssl", "2.0.0", "2.0.0")

	sbomRow := models.SBOM{
		ImageName:     "registry.k8s.io/coredns/coredns",
		ImageTag:      "v1.11.1",
		ImageDigest:   "sha256:2169b3b96af988cf69d7dd69efbcc59433eb027320eb185c6110e0850b997870",
		PodUID:        "85114b90-9118-44ab-886e-e7c4072dbab6",
		PodName:       "coredns-76f75df574-wrd7g",
		Namespace:     "kube-system",
		ContainerName: "coredns",
		OSName:        "debian",
		OSVersion:     "12",
		PackageCount:  2,
		SbomSource:    "parsers",
		Confidence:    "high",
		Status:        "complete",
		GeneratedAt:   time.Now(),
	}
	require.NoError(t, db.Create(&sbomRow).Error)

	comps := []models.SBOMComponent{
		{
			SBOMID: sbomRow.ID, ComponentType: "os-package", ComponentName: "openssl", ComponentVersion: "1.1.1w-0+deb12u1",
			PURL: "pkg:deb/debian/openssl@1.1.1w-0+deb12u1?arch=amd64", TrustLevel: "high", PURLValidated: true, Source: "apk-or-dpkg",
		},
		{
			SBOMID: sbomRow.ID, ComponentType: "language-package", ComponentName: "github.com/coredns/coredns", ComponentVersion: "v1.11.1",
			PURL: "pkg:go/github.com/coredns/coredns@v1.11.1", TrustLevel: "high", PURLValidated: true, Source: "gobinary-main",
		},
	}
	for i := range comps {
		require.NoError(t, db.Create(&comps[i]).Error)
	}

	var loaded []models.SBOMComponent
	require.NoError(t, db.Where("sbom_id = ?", sbomRow.ID).Find(&loaded).Error)
	override := make([]*models.SBOMComponent, len(loaded))
	for i := range loaded {
		override[i] = &loaded[i]
	}

	mgr := database.NewPostgresManager(db)
	m := matcher.NewMatcher(mgr, db)
	matches, err := m.MatchSBOM(context.Background(), &sbomRow, override)
	require.NoError(t, err)

	var hitOpenSSL bool
	for _, x := range matches {
		if x.CVEID == "CVE-REAL-DEB-OPENSSL-001" && x.PackageName == "openssl" {
			hitOpenSSL = true
			break
		}
	}
	require.True(t, hitOpenSSL, "expected openssl CVE from seeded package_vulnerabilities (debian)")
	t.Log("TC-REAL-001 PASSED: kube-system coredns-style pod → openssl CVE from DB")
}

// TC-REAL-002: Pod Alpine (nginx ingress style) — busybox trong alpine ecosystem, CVE trong DB.
func TestE2E_RealisticPod_AlpineIngressStyle_Busybox_FromDB(t *testing.T) {
	db := openRealisticPodE2EDB(t)
	seedCVEWithPackageVuln(t, db, "CVE-REAL-ALP-BUSY-001", "alpine", "busybox", "1.37.0", "1.37.0")

	sbomRow := models.SBOM{
		ImageName:     "registry.k8s.io/ingress-nginx/controller",
		ImageTag:      "v1.10.0",
		ImageDigest:   "sha256:abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		PodUID:        "a1b2c3d4-e5f6-7890-abcd-111111111111",
		PodName:       "ingress-nginx-controller-7d4f8b9c-xk2zp",
		Namespace:     "ingress-nginx",
		ContainerName: "controller",
		OSName:        "alpine",
		OSVersion:     "3.20",
		PackageCount:  1,
		SbomSource:    "parsers",
		Confidence:    "high",
		Status:        "complete",
		GeneratedAt:   time.Now(),
	}
	require.NoError(t, db.Create(&sbomRow).Error)

	comp := models.SBOMComponent{
		SBOMID: sbomRow.ID, ComponentType: "os-package", ComponentName: "busybox", ComponentVersion: "1.36.1-r19",
		PURL:          "pkg:apk/alpine/busybox@1.36.1-r19?arch=x86_64",
		TrustLevel:    "high", PURLValidated: true, Source: "apk",
	}
	require.NoError(t, db.Create(&comp).Error)

	var loaded []models.SBOMComponent
	require.NoError(t, db.Where("sbom_id = ?", sbomRow.ID).Find(&loaded).Error)
	override := make([]*models.SBOMComponent, len(loaded))
	for i := range loaded {
		override[i] = &loaded[i]
	}

	mgr := database.NewPostgresManager(db)
	m := matcher.NewMatcher(mgr, db)
	matches, err := m.MatchSBOM(context.Background(), &sbomRow, override)
	require.NoError(t, err)

	var hit bool
	for _, x := range matches {
		if x.CVEID == "CVE-REAL-ALP-BUSY-001" && x.PackageName == "busybox" {
			hit = true
			break
		}
	}
	require.True(t, hit, "expected busybox CVE from alpine package_vulnerabilities")
	t.Log("TC-REAL-002 PASSED: alpine ingress-style pod → busybox CVE from DB")
}

// TC-REAL-003: Pod risk-center / npm supply-chain — package trùng bản ghi malware trong DB (tên thật từ feed kiểu Aikido).
func TestE2E_RealisticPod_NPM_SupplyChain_Malware_FromDB(t *testing.T) {
	db := openRealisticPodE2EDB(t)
	require.NoError(t, db.Create(&models.MalwarePackage{
		PackageName: "axios-hehe", Version: "1.10.10", Reason: "MALWARE", Confidence: 0.95, Source: "aikido-predictions",
	}).Error)

	sbomRow := models.SBOM{
		ImageName:     "risk-center-test/npm-app",
		ImageTag:      "1.0.0",
		ImageDigest:   "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		PodUID:        "deadbeef-dead-beef-dead-beefdeadbeef",
		PodName:       "npm-vuln-demo-5f8c9d-x7wz",
		Namespace:     "risk-center-test",
		ContainerName: "main",
		OSName:        "debian",
		OSVersion:     "12",
		PackageCount:  2,
		SbomSource:    "parsers",
		Confidence:    "high",
		Status:        "complete",
		GeneratedAt:   time.Now(),
	}
	require.NoError(t, db.Create(&sbomRow).Error)

	require.NoError(t, db.Create(&models.SBOMComponent{
		SBOMID: sbomRow.ID, ComponentType: "language-package", ComponentName: "axios-hehe", ComponentVersion: "1.10.10",
		PURL: "pkg:npm/axios-hehe@1.10.10", TrustLevel: "high", PURLValidated: true, Source: "npm",
	}).Error)
	require.NoError(t, db.Create(&models.SBOMComponent{
		SBOMID: sbomRow.ID, ComponentType: "language-package", ComponentName: "axios", ComponentVersion: "1.6.8",
		PURL: "pkg:npm/axios@1.6.8", TrustLevel: "high", PURLValidated: true, Source: "npm",
	}).Error)

	var loaded []models.SBOMComponent
	require.NoError(t, db.Where("sbom_id = ?", sbomRow.ID).Find(&loaded).Error)

	mgr := malware.NewManager(db)
	require.True(t, mgr.Enabled())
	m := matcher.NewMatcher(database.NewPostgresManager(db), db)
	m.SetMalwareChecker(mgr)
	matches := m.MatchMalware(context.Background(), &sbomRow, loaded)
	require.Len(t, matches, 1)
	require.Equal(t, "axios-hehe", matches[0].PackageName)
	require.Equal(t, "1.10.10", matches[0].PackageVersion)
	require.Equal(t, "MALWARE", matches[0].Reason)
	t.Log("TC-REAL-003 PASSED: npm typosquat package → malware match from DB")
}

// TC-REAL-004: Cùng một SBOM: CVE từ Postgres (zlib debian) + malware (evil-pkg) — cả hai luồng đều hit.
func TestE2E_RealisticPod_Combined_CVE_InDB_And_Malware_InDB(t *testing.T) {
	db := openRealisticPodE2EDB(t)
	// Phải cùng epoch với bản cài (1:…); nếu chỉ "1.3.0" thì so sánh là 0:1.3.0 và 1:1.2.13… không còn < hạn mức.
	seedCVEWithPackageVuln(t, db, "CVE-REAL-DEB-ZLIB-001", "debian", "zlib1g", "1:3.0", "1:3.0")
	require.NoError(t, db.Create(&models.MalwarePackage{
		PackageName: "evil-pkg", Version: "1.0.0", Reason: "MALWARE", Confidence: 0.95, Source: "test-feed",
	}).Error)

	sbomRow := models.SBOM{
		ImageName:     "quay.io/fortuna/demo-combined",
		ImageTag:      "v2",
		ImageDigest:   "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		PodUID:        "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		PodName:       "demo-combined-7f9c8d-abcde",
		Namespace:     "default",
		ContainerName: "app",
		OSName:        "debian",
		OSVersion:     "12",
		PackageCount:  2,
		SbomSource:    "parsers",
		Confidence:    "high",
		Status:        "complete",
		GeneratedAt:   time.Now(),
	}
	require.NoError(t, db.Create(&sbomRow).Error)

	require.NoError(t, db.Create(&models.SBOMComponent{
		SBOMID: sbomRow.ID, ComponentType: "os-package", ComponentName: "zlib1g", ComponentVersion: "1:1.2.13.dfsg-1",
		PURL: "pkg:deb/debian/zlib1g@1.2.13.dfsg-1?arch=amd64", TrustLevel: "high", PURLValidated: true, Source: "dpkg",
	}).Error)
	require.NoError(t, db.Create(&models.SBOMComponent{
		SBOMID: sbomRow.ID, ComponentType: "library", ComponentName: "evil-pkg", ComponentVersion: "1.0.0",
		PURL: "", TrustLevel: "high", PURLValidated: true, Source: "gomod",
	}).Error)

	var loaded []models.SBOMComponent
	require.NoError(t, db.Where("sbom_id = ?", sbomRow.ID).Find(&loaded).Error)

	mgr := database.NewPostgresManager(db)
	m := matcher.NewMatcher(mgr, db)
	m.SetMalwareChecker(malware.NewManager(db))

	ov := make([]*models.SBOMComponent, len(loaded))
	for i := range loaded {
		ov[i] = &loaded[i]
	}
	cveMatches, err := m.MatchSBOM(context.Background(), &sbomRow, ov)
	require.NoError(t, err)
	var zlibCVE bool
	for _, x := range cveMatches {
		if x.CVEID == "CVE-REAL-DEB-ZLIB-001" {
			zlibCVE = true
			break
		}
	}
	require.True(t, zlibCVE, "expected zlib1g CVE")

	malMatches := m.MatchMalware(context.Background(), &sbomRow, loaded)
	require.Len(t, malMatches, 1)
	require.Equal(t, "evil-pkg", malMatches[0].PackageName)

	t.Log("TC-REAL-004 PASSED: same pod SBOM → CVE (postgres) + malware (DB)")
}

// TC-REAL-005: generic busybox (risk-center minimal image) + fallback ecosystem — seed debian busybox, SBOM os unknown/generic PURL.
func TestE2E_RealisticPod_GenericBusybox_CVEViaDistroFallback_FromDB(t *testing.T) {
	db := openRealisticPodE2EDB(t)
	seedCVEWithPackageVuln(t, db, "CVE-REAL-GEN-BUSY-001", "debian", "busybox", "1.37.0", "1.37.0")

	sbomRow := models.SBOM{
		ImageName:     "docker.io/library/busybox",
		ImageTag:      "1.36.1",
		ImageDigest:   "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
		PodUID:        "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
		PodName:       "rc-pss-no-limits",
		Namespace:     "risk-center-test",
		ContainerName: "main",
		OSName:        "unknown",
		OSVersion:     "",
		PackageCount:  1,
		SbomSource:    "parsers",
		Confidence:    "high",
		Status:        "complete",
		GeneratedAt:   time.Now(),
	}
	require.NoError(t, db.Create(&sbomRow).Error)

	require.NoError(t, db.Create(&models.SBOMComponent{
		SBOMID: sbomRow.ID, ComponentType: "os-package", ComponentName: "busybox", ComponentVersion: "1.36.1",
		PURL: "pkg:generic/busybox@1.36.1", TrustLevel: "high", PURLValidated: true, Source: "syft",
	}).Error)

	var loaded []models.SBOMComponent
	require.NoError(t, db.Where("sbom_id = ?", sbomRow.ID).Find(&loaded).Error)
	override := make([]*models.SBOMComponent, len(loaded))
	for i := range loaded {
		override[i] = &loaded[i]
	}

	mgr := database.NewPostgresManager(db)
	m := matcher.NewMatcher(mgr, db)
	matches, err := m.MatchSBOM(context.Background(), &sbomRow, override)
	require.NoError(t, err)

	var hit bool
	for _, x := range matches {
		if x.CVEID == "CVE-REAL-GEN-BUSY-001" && x.PackageName == "busybox" {
			hit = true
			break
		}
	}
	require.True(t, hit, "expected generic→debian bulk fallback to find debian:busybox CVE")
	t.Log("TC-REAL-005 PASSED: generic busybox + debian seed → CVE via ecosystem fallback")
}
