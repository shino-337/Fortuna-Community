package rep

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/resourceidentity"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func runtimeIdempotencySchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.AutoMigrate(
		&models.Pod{},
		&models.RuntimeEvent{},
		&models.RuntimeSignal{},
		&models.RuntimeBehaviorFact{},
		&models.RuntimeIncident{},
		&models.PodRiskProfile{},
	))
	require.NoError(t, db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_runtime_event_agent_source_record_identity
		ON runtime_events(cluster_id, agent_id, source_record_id)
		WHERE source_record_id IS NOT NULL AND source_record_id <> ''
	`).Error)
}

func runtimeReplayInput(sourceRecordID, eventID string, ts time.Time) RuntimeEventInput {
	observed := ts
	ingested := ts.Add(time.Second)
	return RuntimeEventInput{
		AgentID:         "agent-a",
		PodUID:          "pod-runtime-idempotency",
		Namespace:       "ns",
		Syscall:         "connect",
		TargetPath:      "dst=8.8.8.8:53 proto=udp dport=53",
		Timestamp:       &observed,
		EventID:         eventID,
		SourceRecordID:  sourceRecordID,
		ObservedAt:      &observed,
		IngestedAt:      &ingested,
		ResolutionState: "resolved",
		SourceKind:      "falco",
		SourceSensorID:  "node-a",
		SourceRule:      "Network connection",
		PayloadJSON:     `{"kind":"connect","target":"8.8.8.8:53"}`,
		PayloadHash:     "payload-hash-connect",
		Runtime:         "falco",
		EventType:       "runtime.falco.alert",
		Signal:          "Network connection",
		Severity:        "medium",
		Confidence:      0.9,
	}
}

func TestRuntimeSourceRecordExactReplaySkipsDownstreamEffects(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:runtime-idempotency?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	runtimeIdempotencySchema(t, db)

	require.NoError(t, db.Create(&models.Pod{
		ClusterID: "cluster-a", UID: "pod-runtime-idempotency", Namespace: "ns", Name: "pod",
		ServiceAccount: "default",
		Containers:     "[]", ImageDigests: "[]", PodSecurityContext: "{}",
		ContainerSecurityContexts: "{}", VolumeMounts: "[]", Volumes: "[]",
		Tolerations: "[]", Affinity: "{}",
	}).Error)
	id, err := resourceidentity.New("cluster-a", "pod-runtime-idempotency")
	require.NoError(t, err)

	ts := time.Unix(1700000000, 0).UTC()
	first := runtimeReplayInput(strings.Repeat("a", 64), "legacy-second-id", ts)
	result, err := ProcessRuntimeEventForIdentity(context.Background(), db, id, first)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.Duplicate)

	replay := first
	replay.EventID = "different-transport-event-id"
	laterIngest := ts.Add(10 * time.Second)
	replay.IngestedAt = &laterIngest
	result, err = ProcessRuntimeEventForIdentity(context.Background(), db, id, replay)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Duplicate)

	var eventCount int64
	require.NoError(t, db.Model(&models.RuntimeEvent{}).Count(&eventCount).Error)
	require.EqualValues(t, 1, eventCount)

	var signal models.RuntimeSignal
	require.NoError(t, db.Where("cluster_id = ? AND pod_uid = ?", "cluster-a", "pod-runtime-idempotency").First(&signal).Error)
	require.Equal(t, 1, signal.Count, "exact replay must not increment semantic signal count")

	var profile models.PodRiskProfile
	require.NoError(t, db.Where("cluster_id = ? AND pod_uid = ?", "cluster-a", "pod-runtime-idempotency").First(&profile).Error)
	require.Equal(t, 35, profile.RuntimeScore, "exact replay must not increment runtime risk")

	changed := first
	changed.PayloadHash = "changed-payload"
	_, err = ProcessRuntimeEventForIdentity(context.Background(), db, id, changed)
	require.Error(t, err)
	require.Contains(t, err.Error(), "source-record identity reused")
	require.NoError(t, db.Where("cluster_id = ? AND pod_uid = ?", "cluster-a", "pod-runtime-idempotency").First(&profile).Error)
	require.Equal(t, 35, profile.RuntimeScore)
}

func TestRuntimeSameSecondIdenticalObservationsRemainDistinct(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:runtime-same-second?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	runtimeIdempotencySchema(t, db)
	require.NoError(t, db.Create(&models.Cluster{ID: "cluster-a", Name: "cluster-a"}).Error)
	require.NoError(t, db.Create(&models.Pod{
		ClusterID: "cluster-a", UID: "pod-runtime-idempotency", Namespace: "ns", Name: "pod",
		ServiceAccount: "default",
		Containers:     "[]", ImageDigests: "[]", PodSecurityContext: "{}",
		ContainerSecurityContexts: "{}", VolumeMounts: "[]", Volumes: "[]",
		Tolerations: "[]", Affinity: "{}",
	}).Error)
	id, err := resourceidentity.New("cluster-a", "pod-runtime-idempotency")
	require.NoError(t, err)

	ts := time.Unix(1700000000, 0).UTC()
	base := RuntimeEventInput{
		AgentID:         "agent-a",
		PodUID:          "pod-runtime-idempotency",
		Namespace:       "ns",
		Syscall:         "noop",
		TargetPath:      "same",
		Timestamp:       &ts,
		EventID:         "same-second-event-id",
		ObservedAt:      &ts,
		IngestedAt:      &ts,
		ResolutionState: "resolved",
		SourceKind:      "file",
		SourceSensorID:  "node-a",
		PayloadJSON:     `{"same":true}`,
		PayloadHash:     "same-payload",
		Runtime:         "agent",
		EventType:       "runtime.test",
		Signal:          "same",
		Severity:        "low",
		Confidence:      0.9,
	}
	first := base
	first.SourceRecordID = strings.Repeat("b", 64)
	second := base
	second.SourceRecordID = strings.Repeat("c", 64)

	r1, err := ProcessRuntimeEventForIdentity(context.Background(), db, id, first)
	require.NoError(t, err)
	require.False(t, r1.Duplicate)
	r2, err := ProcessRuntimeEventForIdentity(context.Background(), db, id, second)
	require.NoError(t, err)
	require.False(t, r2.Duplicate)

	third := base
	third.AgentID = "agent-b"
	third.SourceRecordID = first.SourceRecordID
	r3, err := ProcessRuntimeEventForIdentity(context.Background(), db, id, third)
	require.NoError(t, err)
	require.False(t, r3.Duplicate, "different authenticated agents must have separate replay namespaces")

	var eventCount int64
	require.NoError(t, db.Model(&models.RuntimeEvent{}).Count(&eventCount).Error)
	require.EqualValues(t, 3, eventCount, "physical records and agent namespaces must remain distinct")

	var signals []models.RuntimeSignal
	require.NoError(t, db.Where("cluster_id = ? AND pod_uid = ? AND signal_type = ?", "cluster-a", "pod-runtime-idempotency", "UNKNOWN").Find(&signals).Error)
	totalSignalEffects := 0
	for i := range signals {
		totalSignalEffects += signals[i].Count
	}
	require.Equal(t, 3, totalSignalEffects, "distinct physical records and Agent namespaces must each reach downstream effects")
}

func TestRuntimeSourceRecordConcurrentDuplicatePostgres(t *testing.T) {
	dsn := os.Getenv("FORTUNA_TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("FORTUNA_TEST_POSTGRES_URL is not configured")
	}

	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	adminSQL, err := admin.DB()
	require.NoError(t, err)
	defer adminSQL.Close()

	schema := fmt.Sprintf("runtime_event_idempotency_%d", time.Now().UnixNano())
	require.NoError(t, admin.Exec("CREATE SCHEMA "+schema).Error)
	defer admin.Exec("DROP SCHEMA " + schema + " CASCADE")

	if strings.Contains(dsn, "://") {
		u, err := url.Parse(dsn)
		require.NoError(t, err)
		q := u.Query()
		q.Set("search_path", schema)
		u.RawQuery = q.Encode()
		dsn = u.String()
	} else {
		dsn += " search_path=" + schema
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	defer pool.Close()
	pool.SetMaxOpenConns(8)

	runtimeIdempotencySchema(t, db)
	require.NoError(t, db.Create(&models.Cluster{ID: "cluster-a", Name: "cluster-a"}).Error)
	require.NoError(t, db.Create(&models.Pod{
		ClusterID: "cluster-a", UID: "pod-runtime-idempotency", Namespace: "ns", Name: "pod",
		ServiceAccount: "default",
		Containers:     "[]", ImageDigests: "[]", PodSecurityContext: "{}",
		ContainerSecurityContexts: "{}", VolumeMounts: "[]", Volumes: "[]",
		Tolerations: "[]", Affinity: "{}",
	}).Error)
	id, err := resourceidentity.New("cluster-a", "pod-runtime-idempotency")
	require.NoError(t, err)

	input := runtimeReplayInput(strings.Repeat("d", 64), "same-second-event-id", time.Unix(1700000000, 0).UTC())
	start := make(chan struct{})
	results := make(chan *ProcessResult, 2)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			result, err := ProcessRuntimeEventForIdentity(context.Background(), db, id, input)
			results <- result
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)

	duplicates := 0
	processed := 0
	for result := range results {
		require.NotNil(t, result)
		if result.Duplicate {
			duplicates++
		} else {
			processed++
		}
	}
	for err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, 1, processed)
	require.Equal(t, 1, duplicates)

	var eventCount int64
	require.NoError(t, db.Model(&models.RuntimeEvent{}).Count(&eventCount).Error)
	require.EqualValues(t, 1, eventCount)

	var signal models.RuntimeSignal
	require.NoError(t, db.Where("cluster_id = ? AND pod_uid = ?", "cluster-a", "pod-runtime-idempotency").First(&signal).Error)
	require.Equal(t, 1, signal.Count)

	var profile models.PodRiskProfile
	require.NoError(t, db.Where("cluster_id = ? AND pod_uid = ?", "cluster-a", "pod-runtime-idempotency").First(&profile).Error)
	require.Equal(t, 35, profile.RuntimeScore, "concurrent duplicate must create one risk effect")
}
