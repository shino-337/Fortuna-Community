package grpc

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	pb "github.com/fortuna/api/proto/agent"
	"github.com/fortuna/core/pkg/cve/database"
	"github.com/fortuna/core/pkg/cve/matcher"
	"github.com/fortuna/core/pkg/models"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Core-level E2E: Agent-style SBOMFinding -> Core stores components -> matcher resolves pkg:go multi-segment + alias -> OSV mirror match.
func TestE2E_Core_SendSBOMFinding_GoMultiSegmentPURL_AliasMatch(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)

	// Schema needed for handler write + matcher read.
	require.NoError(t, db.AutoMigrate(
		&models.SBOM{},
		&models.SBOMComponent{},
		&models.CVEMatch{},
		&models.OSVVulnerability{},
		&models.OSVPackage{},
		&models.OSVRange{},
		&models.GoModuleAlias{},
	))
	// Repository upsert uses ON CONFLICT (sbom_id, purl); SQLite requires an explicit UNIQUE index.
	require.NoError(t, db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_sbom_components_unique_sbom_purl_all ON sbom_components(sbom_id, purl);").Error)

	// Seed alias: old module prefix -> canonical (OSV mirror stores canonical).
	require.NoError(t, db.Create(&models.GoModuleAlias{
		Alias:     "github.com/coreos/etcd",
		Canonical: "go.etcd.io/etcd",
	}).Error)

	// Seed OSV mirror rows: vuln affects canonical module; range includes v3.3.0.
	v := models.OSVVulnerability{ID: "GO-ETCD-ALIAS-TEST", Summary: "etcd vuln", Details: "details", Severity: "HIGH", CVSSScore: 8.0}
	require.NoError(t, db.Create(&v).Error)
	p := models.OSVPackage{VulnID: "GO-ETCD-ALIAS-TEST", Ecosystem: "go", PackageName: "go.etcd.io/etcd"}
	require.NoError(t, db.Create(&p).Error)
	r := models.OSVRange{PackageID: p.ID, RangeType: "SEMVER", Introduced: "0", Fixed: "3.3.99"}
	require.NoError(t, db.Create(&r).Error)

	// Send SBOMFinding with multi-segment go module PURL (Agent should emit this; Core must preserve it).
	svc := NewSBOMServiceServer(db, nil, nil)
	req := &pb.SBOMFinding{
		PodUid:        "pod-uid-go-etcd",
		PodName:       "pod-go-etcd",
		Namespace:     "default",
		ContainerName: "main",
		ImageName:     "test/image",
		ImageDigest:   "sha256:abc",
		ImageTag:      "latest",
		AgentId:       "agent-1",
		NodeId:        "node-1",
		GeneratedAt:   timestamppb.New(time.Now()),
		Packages: []*pb.Package{
			{
				Name:    "github.com/coreos/etcd/client/v3",
				Version: "v3.3.0",
				Type:    pb.PackageType_PACKAGE_TYPE_GO_MOD,
				Purl:    "pkg:go/github.com/coreos/etcd/client/v3@v3.3.0",
			},
		},
	}

	resp, err := svc.SendSBOMFinding(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.True(t, resp.Success)

	sbomID64, err := strconv.ParseUint(resp.SbomId, 10, 64)
	require.NoError(t, err)

	var sbomRow models.SBOM
	require.NoError(t, db.Where("id = ?", uint(sbomID64)).First(&sbomRow).Error)

	mgr := database.NewPostgresManager(db)
	m := matcher.NewMatcher(mgr, db)
	matches, err := m.MatchSBOM(context.Background(), &sbomRow, nil)
	require.NoError(t, err)

	var found bool
	for _, match := range matches {
		if match.CVEID == "GO-ETCD-ALIAS-TEST" && match.PackageName == "github.com/coreos/etcd/client/v3" {
			found = true
			break
		}
	}
	require.True(t, found, "expected alias-resolved match for github.com/coreos/etcd/client/v3@v3.3.0")
}

