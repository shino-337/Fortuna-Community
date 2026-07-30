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
	"github.com/fortuna/core/pkg/models"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func openSBOMHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SBOM{}, &models.SBOMComponent{}, &models.Pod{}))
	// Repository upsert uses ON CONFLICT (sbom_id, purl); SQLite requires an explicit UNIQUE index.
	require.NoError(t, db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_sbom_components_unique_sbom_purl_all ON sbom_components(sbom_id, purl);").Error)
	return db
}

func TestCanonicalization_Idempotent(t *testing.T) {
	input := " github.com/gin-gonic/gin/ "
	c1, ok := canonicalizeGoModuleName(input)
	require.True(t, ok)
	c2, ok := canonicalizeGoModuleName(c1)
	require.True(t, ok)
	require.Equal(t, c1, c2)
}

func TestCore_DoesNotOverrideValidPURL(t *testing.T) {
	db := openSBOMHandlerTestDB(t)
	svc := NewSBOMServiceServer(db, nil, nil)

	req := &pb.SBOMFinding{
		PodUid:        "pod-uid-valid-purl",
		PodName:       "pod-valid-purl",
		Namespace:     "default",
		ContainerName: "main",
		ImageName:     "test/image",
		ImageDigest:   "sha256:abc",
		ImageTag:      "latest",
		AgentId:       "agent-1",
		GeneratedAt:   timestamppb.New(time.Now()),
		Packages: []*pb.Package{
			{
				Name:    "github.com/a/b",
				Version: "v1.2.3",
				Type:    pb.PackageType_PACKAGE_TYPE_GO_MOD,
				Purl:    "pkg:go/github.com/a/b@v1.2.3",
			},
		},
	}

	resp, err := svc.SendSBOMFinding(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.True(t, resp.Success)

	sbomID64, err := strconv.ParseUint(resp.SbomId, 10, 64)
	require.NoError(t, err)
	var comp models.SBOMComponent
	require.NoError(t, db.Where("sbom_id = ?", uint(sbomID64)).First(&comp).Error)
	require.Equal(t, "pkg:go/github.com/a/b@v1.2.3", comp.PURL)
}

func TestCore_InvalidPURL_Fallback(t *testing.T) {
	db := openSBOMHandlerTestDB(t)
	svc := NewSBOMServiceServer(db, nil, nil)

	req := &pb.SBOMFinding{
		PodUid:        "pod-uid-invalid-purl",
		PodName:       "pod-invalid-purl",
		Namespace:     "default",
		ContainerName: "main",
		ImageName:     "test/image",
		ImageDigest:   "sha256:abc",
		ImageTag:      "latest",
		AgentId:       "agent-1",
		GeneratedAt:   timestamppb.New(time.Now()),
		Packages: []*pb.Package{
			{
				Name:    "github.com/a/b",
				Version: "v1.2.3",
				Type:    pb.PackageType_PACKAGE_TYPE_GO_MOD,
				Purl:    "invalid",
			},
		},
	}

	resp, err := svc.SendSBOMFinding(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.True(t, resp.Success)

	sbomID64, err := strconv.ParseUint(resp.SbomId, 10, 64)
	require.NoError(t, err)
	var comp models.SBOMComponent
	require.NoError(t, db.Where("sbom_id = ?", uint(sbomID64)).First(&comp).Error)
	require.Contains(t, comp.PURL, "pkg:go/")
}

func TestCore_PURL_EcosystemMismatch_Regenerates(t *testing.T) {
	db := openSBOMHandlerTestDB(t)
	svc := NewSBOMServiceServer(db, nil, nil)

	// Type=GO_MOD but PURL ecosystem is npm -> should regenerate to pkg:go/...
	req := &pb.SBOMFinding{
		PodUid:        "pod-uid-eco-mismatch",
		PodName:       "pod-eco-mismatch",
		Namespace:     "default",
		ContainerName: "main",
		ImageName:     "test/image",
		ImageDigest:   "sha256:abc",
		ImageTag:      "latest",
		AgentId:       "agent-1",
		GeneratedAt:   timestamppb.New(time.Now()),
		Packages: []*pb.Package{
			{
				Name:    "github.com/a/b",
				Version: "v1.2.3",
				Type:    pb.PackageType_PACKAGE_TYPE_GO_MOD,
				Purl:    "pkg:npm/github.com/a/b@v1.2.3",
			},
		},
	}

	resp, err := svc.SendSBOMFinding(context.Background(), req)
	require.NoError(t, err)
	require.True(t, resp.Success)

	sbomID64, err := strconv.ParseUint(resp.SbomId, 10, 64)
	require.NoError(t, err)
	var comp models.SBOMComponent
	require.NoError(t, db.Where("sbom_id = ?", uint(sbomID64)).First(&comp).Error)
	require.Contains(t, comp.PURL, "pkg:go/")
}

func TestCore_GoVersionBuildMetadata_StrippedInPURL(t *testing.T) {
	db := openSBOMHandlerTestDB(t)
	svc := NewSBOMServiceServer(db, nil, nil)

	req := &pb.SBOMFinding{
		PodUid:        "pod-uid-go-buildmeta",
		PodName:       "pod-go-buildmeta",
		Namespace:     "default",
		ContainerName: "main",
		ImageName:     "test/image",
		ImageDigest:   "sha256:abc",
		ImageTag:      "latest",
		AgentId:       "agent-1",
		GeneratedAt:   timestamppb.New(time.Now()),
		Packages: []*pb.Package{
			{
				Name:    "github.com/gin-gonic/gin",
				Version: "v1.8.0+malicious",
				Type:    pb.PackageType_PACKAGE_TYPE_GO_MOD,
				Purl:    "pkg:go/github.com/gin-gonic/gin@v1.8.0+malicious",
			},
		},
	}

	resp, err := svc.SendSBOMFinding(context.Background(), req)
	require.NoError(t, err)
	require.True(t, resp.Success)

	sbomID64, err := strconv.ParseUint(resp.SbomId, 10, 64)
	require.NoError(t, err)
	var comp models.SBOMComponent
	require.NoError(t, db.Where("sbom_id = ?", uint(sbomID64)).First(&comp).Error)
	require.Equal(t, "pkg:go/github.com/gin-gonic/gin@v1.8.0", comp.PURL)
}

func TestCore_GoModuleName_PathTraversalRejected(t *testing.T) {
	db := openSBOMHandlerTestDB(t)
	svc := NewSBOMServiceServer(db, nil, nil)

	req := &pb.SBOMFinding{
		PodUid:        "pod-uid-go-traversal",
		PodName:       "pod-go-traversal",
		Namespace:     "default",
		ContainerName: "main",
		ImageName:     "test/image",
		ImageDigest:   "sha256:abc",
		ImageTag:      "latest",
		AgentId:       "agent-1",
		GeneratedAt:   timestamppb.New(time.Now()),
		Packages: []*pb.Package{
			{
				Name:    "github.com/a/../b",
				Version: "v1.2.3",
				Type:    pb.PackageType_PACKAGE_TYPE_GO_MOD,
				Purl:    "pkg:go/github.com/a/../b@v1.2.3",
			},
		},
	}

	resp, err := svc.SendSBOMFinding(context.Background(), req)
	require.NoError(t, err)
	require.True(t, resp.Success)

	sbomID64, err := strconv.ParseUint(resp.SbomId, 10, 64)
	require.NoError(t, err)
	var count int64
	require.NoError(t, db.Model(&models.SBOMComponent{}).Where("sbom_id = ?", uint(sbomID64)).Count(&count).Error)
	require.Equal(t, int64(0), count, "traversal module name must be dropped, not cleaned")
}

func TestCore_GoModuleName_NonASCII_Rejected(t *testing.T) {
	db := openSBOMHandlerTestDB(t)
	svc := NewSBOMServiceServer(db, nil, nil)

	// Cyrillic 'і' in gіn-gonic
	req := &pb.SBOMFinding{
		PodUid:        "pod-uid-go-nonascii",
		PodName:       "pod-go-nonascii",
		Namespace:     "default",
		ContainerName: "main",
		ImageName:     "test/image",
		ImageDigest:   "sha256:abc",
		ImageTag:      "latest",
		AgentId:       "agent-1",
		GeneratedAt:   timestamppb.New(time.Now()),
		Packages: []*pb.Package{
			{
				Name:    "github.com/gіn-gonic/gin",
				Version: "v1.8.0",
				Type:    pb.PackageType_PACKAGE_TYPE_GO_MOD,
				Purl:    "pkg:go/github.com/gіn-gonic/gin@v1.8.0",
			},
		},
	}

	resp, err := svc.SendSBOMFinding(context.Background(), req)
	require.NoError(t, err)
	require.True(t, resp.Success)

	sbomID64, err := strconv.ParseUint(resp.SbomId, 10, 64)
	require.NoError(t, err)
	var count int64
	require.NoError(t, db.Model(&models.SBOMComponent{}).Where("sbom_id = ?", uint(sbomID64)).Count(&count).Error)
	require.Equal(t, int64(0), count, "non-ASCII module name must be rejected")
}

func TestCore_GoModuleName_TrailingSlash_Normalized(t *testing.T) {
	db := openSBOMHandlerTestDB(t)
	svc := NewSBOMServiceServer(db, nil, nil)

	req := &pb.SBOMFinding{
		PodUid:        "pod-uid-go-trailing-slash",
		PodName:       "pod-go-trailing-slash",
		Namespace:     "default",
		ContainerName: "main",
		ImageName:     "test/image",
		ImageDigest:   "sha256:abc",
		ImageTag:      "latest",
		AgentId:       "agent-1",
		GeneratedAt:   timestamppb.New(time.Now()),
		Packages: []*pb.Package{
			{
				Name:    "github.com/gin-gonic/gin/",
				Version: "v1.8.0",
				Type:    pb.PackageType_PACKAGE_TYPE_GO_MOD,
				Purl:    "pkg:go/github.com/gin-gonic/gin/@v1.8.0",
			},
		},
	}

	resp, err := svc.SendSBOMFinding(context.Background(), req)
	require.NoError(t, err)
	require.True(t, resp.Success)

	sbomID64, err := strconv.ParseUint(resp.SbomId, 10, 64)
	require.NoError(t, err)
	var comp models.SBOMComponent
	require.NoError(t, db.Where("sbom_id = ?", uint(sbomID64)).First(&comp).Error)
	require.Equal(t, "pkg:go/github.com/gin-gonic/gin@v1.8.0", comp.PURL)
}
