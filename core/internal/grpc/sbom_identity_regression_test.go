package grpc

import (
	pb "github.com/fortuna/api/proto/agent"
	"github.com/fortuna/core/pkg/models"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
	"strconv"
	"testing"
	"time"
)

func TestSBOMIngestPreservesTrustedClusterIdentity(t *testing.T) {
	db := openSBOMHandlerTestDB(t)
	svc := NewSBOMServiceServer(db, nil, nil)
	var ids []string
	for _, cluster := range []string{"a", "b"} {
		require.NoError(t, db.Create(&models.Pod{ClusterID: cluster, UID: "same", Namespace: "ns", Name: "app"}).Error)
		ctx := scopedGRPCTestContext(scopedGRPCTestPrincipal(cluster, "agent"), nil)
		req := &pb.SBOMFinding{PodUid: "same", PodName: "app", Namespace: "ns", ContainerName: "app", ImageName: "image", ImageDigest: "sha256:same", AgentId: "agent", GeneratedAt: timestamppb.New(time.Now()), Packages: []*pb.Package{{Name: "lib", Version: "1", Type: pb.PackageType_PACKAGE_TYPE_NPM}}}
		resp, err := svc.SendSBOMFinding(ctx, req)
		require.NoError(t, err)
		require.True(t, resp.Success)
		ids = append(ids, resp.SbomId)
		n, err := strconv.ParseUint(resp.SbomId, 10, 64)
		require.NoError(t, err)
		var stored models.SBOM
		require.NoError(t, db.First(&stored, n).Error)
		require.Equal(t, cluster, stored.ClusterID)
		var visible int64
		require.NoError(t, db.Table("sboms s").Joins("JOIN pods p ON p.cluster_id=s.cluster_id AND p.uid=s.pod_uid").Where("s.id = ? AND p.cluster_id = ?", n, cluster).Count(&visible).Error)
		require.EqualValues(t, 1, visible)
	}
	require.NotEqual(t, ids[0], ids[1], "same UID/digest cannot reuse another cluster's SBOM")
}
