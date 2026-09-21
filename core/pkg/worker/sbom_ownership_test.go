package worker

import (
	"context"
	"encoding/json"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/sbom"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"io"
	"log"
	"testing"
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

func TestSBOMWorkerNeverGuessesOwnerFromDigestOrDefault(t *testing.T) {
	db := newReplayWorkerTestDB(t)
	row := seedFinalizedSBOM(t, db)
	worker := &SBOMWorker{db: db, service: sbom.NewService(db), logger: log.New(io.Discard, "", 0)}
	t.Setenv("DEFAULT_CLUSTER_ID", row.ClusterID)
	for _, cluster := range []string{"", "foreign"} {
		raw, err := json.Marshal(map[string]interface{}{"uid": row.PodUID, "cluster_id": cluster, "raw_json": `{"spec":{"containers":[{"name":"main","image":"test/image@sha256:test"}]},"status":{"phase":"Running"}}`})
		require.NoError(t, err)
		err = worker.Process(context.Background(), &nats.Msg{Data: raw})
		if cluster == "" {
			require.ErrorContains(t, err, "ownership")
		} else {
			require.NoError(t, err)
		}
	}
	// No PodImageScan schema or publisher is needed: foreign snapshots never reach writes/publication.
}
