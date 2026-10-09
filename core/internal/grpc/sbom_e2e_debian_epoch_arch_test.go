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
	"github.com/fortuna/core/pkg/cve/catalogtest"
	"github.com/fortuna/core/pkg/cve/database"
	"github.com/fortuna/core/pkg/cve/matcher"
	"github.com/fortuna/core/pkg/models"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// The epoch qualifier of an arch-qualified Debian PURL takes part in the version comparison
// against a catalog range bounded by epoch versions.
func TestE2E_Core_SendSBOMFinding_DebianEpochQualifier_AffectsMatch(t *testing.T) {
	t.Run("epoch present => match", func(t *testing.T) {
		db := openSBOMFindingTestDB(t)
		seedDebianEpochCase(t, db, "CVE-EPOCH-ARCH-TEST", "debian", "openssl", "2:3.0.8-1", "2:3.0.9-1")

		svc := NewSBOMServiceServer(db, nil, nil)
		req := &pb.SBOMFinding{
			PodUid:        "pod-uid-deb-epoch-arch-match",
			PodName:       "pod-deb-epoch-arch-match",
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
					Name:    "openssl",
					Version: "3.0.8-1",
					Type:    pb.PackageType_PACKAGE_TYPE_DEB,
					Purl:    "pkg:deb/debian/openssl@3.0.8-1?epoch=2&arch=amd64",
				},
			},
		}

		require.NoError(t, db.AutoMigrate(&models.Pod{}))
		require.NoError(t, db.Create(&models.Pod{ClusterID: "cluster-a", UID: req.PodUid, Name: req.PodName, Namespace: req.Namespace}).Error)
		resp, err := svc.SendSBOMFinding(context.Background(), req)
		require.NoError(t, err)
		require.True(t, resp.Success)

		var sbomRow models.SBOM
		sbomID64, err := strconv.ParseUint(resp.SbomId, 10, 64)
		require.NoError(t, err)
		require.NoError(t, db.Where("id = ?", uint(sbomID64)).First(&sbomRow).Error)

		mgr := database.NewPostgresManager(db)
		m := matcher.NewMatcher(mgr, db)
		matches, err := m.MatchSBOM(context.Background(), &sbomRow, nil)
		require.NoError(t, err)

		var found bool
		for _, match := range matches {
			if match.CVEID == "CVE-EPOCH-ARCH-TEST" && match.PackageName == "openssl" {
				found = true
				break
			}
		}
		require.True(t, found, "expected the epoch-qualified PURL to fall in the seeded Debian range")
	})

	t.Run("epoch missing => no match", func(t *testing.T) {
		db := openSBOMFindingTestDB(t)
		seedDebianEpochCase(t, db, "CVE-EPOCH-ARCH-TEST", "debian", "openssl", "2:3.0.8-1", "2:3.0.9-1")

		svc := NewSBOMServiceServer(db, nil, nil)
		req := &pb.SBOMFinding{
			PodUid:        "pod-uid-deb-epoch-arch-epoch-missing",
			PodName:       "pod-deb-epoch-arch-epoch-missing",
			Namespace:     "default",
			ContainerName: "main",
			ImageName:     "test/image",
			ImageDigest:   "sha256:def",
			ImageTag:      "latest",
			AgentId:       "agent-1",
			NodeId:        "node-1",
			GeneratedAt:   timestamppb.New(time.Now()),
			Packages: []*pb.Package{
				{
					Name:    "openssl",
					Version: "3.0.8-1",
					Type:    pb.PackageType_PACKAGE_TYPE_DEB,
					// Missing epoch qualifier: installedVersion becomes 0:3.0.8-1, so constraint >= 2:... must not match.
					Purl: "pkg:deb/debian/openssl@3.0.8-1?arch=amd64",
				},
			},
		}

		require.NoError(t, db.AutoMigrate(&models.Pod{}))
		require.NoError(t, db.Create(&models.Pod{ClusterID: "cluster-a", UID: req.PodUid, Name: req.PodName, Namespace: req.Namespace}).Error)
		resp, err := svc.SendSBOMFinding(context.Background(), req)
		require.NoError(t, err)
		require.True(t, resp.Success)

		var sbomRow models.SBOM
		sbomID64, err := strconv.ParseUint(resp.SbomId, 10, 64)
		require.NoError(t, err)
		require.NoError(t, db.Where("id = ?", uint(sbomID64)).First(&sbomRow).Error)

		mgr := database.NewPostgresManager(db)
		m := matcher.NewMatcher(mgr, db)
		matches, err := m.MatchSBOM(context.Background(), &sbomRow, nil)
		require.NoError(t, err)

		for _, match := range matches {
			require.NotEqual(t, "CVE-EPOCH-ARCH-TEST", match.CVEID, "must not match when epoch qualifier is missing")
		}
	})
}

func openSBOMFindingTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(
		&models.SBOM{},
		&models.SBOMComponent{},
	))

	// Repository upsert uses ON CONFLICT (sbom_id, purl); SQLite requires an explicit UNIQUE index.
	require.NoError(t, db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_sbom_components_unique_sbom_purl_all ON sbom_components(sbom_id, purl);").Error)

	return db
}

// seedDebianEpochCase seeds an advisory affecting eco/pkgName from introduced up to fixed.
func seedDebianEpochCase(t *testing.T, db *gorm.DB, cveID, eco, pkgName, introduced, fixed string) {
	t.Helper()

	catalogtest.Seed(t, db, catalogtest.Advisory{
		ID:       cveID,
		Severity: "HIGH",
		Details:  "test",
		Affected: []catalogtest.Range{{Ecosystem: eco, Package: pkgName, Introduced: introduced, Fixed: fixed}},
	})
}
