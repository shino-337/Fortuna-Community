package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	pb "github.com/fortuna/api/proto/agent"
	"github.com/fortuna/core/internal/contextkeys"
	"github.com/fortuna/core/internal/repository"
	"github.com/fortuna/core/pkg/models"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TestService_CannotBypassSBOMGuard verifies that the handler uses the guarded path (with mutation flag).
// When an existing finalized SBOM exists for a pod, a second SendSBOMFinding for the same pod must succeed
// because the handler passes WithSBOMMutationAllowed. If the handler bypassed the repo or forgot the flag,
// this call would return an error (finalized and immutable).
func TestService_CannotBypassSBOMGuard(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SBOM{}, &models.SBOMComponent{}, &models.Pod{}))

	// Create finalized SBOM for pod via repo (with flag)
	repo := repository.NewSBOMRepository(db)
	ctxAllow := contextkeys.WithSBOMMutationAllowed(context.Background())
	sbom := &models.SBOM{
		PodUID:        "pod-uid-finalized",
		ImageName:     "test/image",
		ImageTag:      "latest",
		ImageDigest:   "sha256:abc",
		PodName:       "test-pod",
		Namespace:     "default",
		ContainerName: "main",
		PackageCount:  0,
		LastUsedAt:    time.Now(),
		UseCount:      1,
		Status:        "finalized",
		Version:       1,
	}
	_, _, err = repo.UpsertSBOMWithComponents(ctxAllow, sbom, nil)
	require.NoError(t, err)

	// Handler must use repo with flag so this second "update" for same pod succeeds
	svc := NewSBOMServiceServer(db, nil, nil)
	req := &pb.SBOMFinding{
		PodUid:        "pod-uid-finalized",
		PodName:       "test-pod",
		Namespace:     "default",
		ContainerName: "main",
		ImageName:     "test/image",
		ImageDigest:   "sha256:abc",
		ImageTag:      "latest",
		AgentId:       "agent-1",
		GeneratedAt:   timestamppb.New(time.Now()),
		Packages:      []*pb.Package{},
	}

	resp, err := svc.SendSBOMFinding(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.True(t, resp.Success)
}
