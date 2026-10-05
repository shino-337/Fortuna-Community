package worker

import (
	"context"
	"testing"

	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/sbom"
	"github.com/stretchr/testify/require"
)

func TestSBOMEventRejectsForeignOwnershipBeforeEffects(t *testing.T) {
	db := newReplayWorkerTestDB(t)
	row := seedFinalizedSBOM(t, db)
	worker := NewCVEMatcherWorker(nil, db, nil)
	for _, field := range []string{"cluster", "pod", "container", "digest", "missing"} {
		t.Run(field, func(t *testing.T) {
			event := sbom.SBOMCreatedEvent{SBOMID: row.ID, ClusterID: row.ClusterID, PodUID: row.PodUID, ContainerName: row.ContainerName, ImageDigest: row.ImageDigest, Timestamp: 12345}
			switch field {
			case "cluster":
				event.ClusterID = "foreign"
			case "pod":
				event.PodUID = "foreign"
			case "container":
				event.ContainerName = "foreign"
			case "digest":
				event.ImageDigest = "sha256:foreign"
			case "missing":
				event.ClusterID = ""
			}
			require.Error(t, worker.ProcessSBOMCreatedEvent(context.Background(), event))
			var count int64
			require.NoError(t, db.Model(&models.SBOMMatchRun{}).Count(&count).Error)
			require.Zero(t, count)
			var stored models.SBOM
			require.NoError(t, db.First(&stored, row.ID).Error)
			require.Equal(t, row.Version, stored.Version)
		})
	}
}

func TestMalwarePersistenceFailureIsReturned(t *testing.T) {
	db := newReplayWorkerTestDB(t)
	worker := NewCVEMatcherWorker(nil, db, nil)
	// No malware_matches table: failure must not be reported as successful matching.
	require.ErrorContains(t, worker.persistMalwareMatches(context.Background(), []*models.MalwareMatch{{ClusterID: "cluster-1", SBOMID: 1, PodUID: "pod-1", PackageName: "evil", PackageVersion: "1"}}), "persist malware match")
}
