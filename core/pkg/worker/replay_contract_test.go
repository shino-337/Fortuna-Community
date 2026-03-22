package worker

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/fortuna/core/migrations"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/sbom"
	"github.com/glebarez/sqlite"
	"github.com/nats-io/nats.go"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newReplayWorkerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	// Minimum schema for worker Process path.
	if err := db.AutoMigrate(
		&models.SBOM{},
		&models.SBOMComponent{},
		&models.SBOMMatchRun{},
		&models.OSVVulnerability{},
		&models.OSVPackage{},
		&models.OSVRange{},
		&models.CVEMatch{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := migrations.Migration086_AddSBOMProcessingState(db); err != nil {
		t.Fatalf("migration 086: %v", err)
	}
	return db
}

func seedFinalizedSBOM(t *testing.T, db *gorm.DB) models.SBOM {
	t.Helper()
	sb := models.SBOM{
		ImageName:     "test/image",
		ImageTag:      "latest",
		ImageDigest:   "sha256:test",
		PodUID:        "pod-1",
		PodName:       "pod-1",
		Namespace:     "default",
		ContainerName: "main",
		Status:        "finalized",
		Version:       1,
		GeneratedAt:   time.Now(),
	}
	if err := db.Create(&sb).Error; err != nil {
		t.Fatalf("create sbom: %v", err)
	}
	return sb
}

func runWorkerEvent(t *testing.T, w *CVEMatcherWorker, ev sbom.SBOMCreatedEvent) {
	t.Helper()
	raw, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	msg := &nats.Msg{Data: raw}
	if err := w.Process(context.Background(), msg); err != nil {
		t.Fatalf("process: %v", err)
	}
}

func TestWorker_SchemaMismatch_SkipsProcessing(t *testing.T) {
	db := newReplayWorkerTestDB(t)
	sb := seedFinalizedSBOM(t, db)
	w := NewCVEMatcherWorker(nil, db, nil)

	runWorkerEvent(t, w, sbom.SBOMCreatedEvent{
		Type:          "sbom.created",
		Timestamp:     time.Now().Unix(),
		EventID:       "ev-schema-warn",
		SchemaVersion: "legacy-v0", // intentionally mismatched
		SBOMID:        sb.ID,
		ImageDigest:   sb.ImageDigest,
	})

	var runs int64
	if err := db.Model(&models.SBOMMatchRun{}).Where("sbom_id = ?", sb.ID).Count(&runs).Error; err != nil {
		t.Fatalf("count runs: %v", err)
	}
	if runs != 0 {
		t.Fatalf("expected no match run when schema mismatched, got %d", runs)
	}
}

func TestWorker_Replay_SameTimestampSameEventID_Idempotent(t *testing.T) {
	db := newReplayWorkerTestDB(t)
	sb := seedFinalizedSBOM(t, db)
	w := NewCVEMatcherWorker(nil, db, nil)

	ts := time.Now().Unix()
	ev := sbom.SBOMCreatedEvent{
		Type:          "sbom.created",
		Timestamp:     ts,
		EventID:       "ev-same",
		SchemaVersion: sbom.SBOMCreatedEventSchemaVersion,
		SBOMID:        sb.ID,
		ImageDigest:   sb.ImageDigest,
	}
	runWorkerEvent(t, w, ev)
	runWorkerEvent(t, w, ev)

	var runs int64
	if err := db.Model(&models.SBOMMatchRun{}).Where("sbom_id = ?", sb.ID).Count(&runs).Error; err != nil {
		t.Fatalf("count runs: %v", err)
	}
	if runs != 1 {
		t.Fatalf("expected 1 match run after idempotent replay, got %d", runs)
	}
}

func TestWorker_Replay_SameTimestampDifferentEventID_UpdatesWatermark(t *testing.T) {
	db := newReplayWorkerTestDB(t)
	sb := seedFinalizedSBOM(t, db)
	w := NewCVEMatcherWorker(nil, db, nil)

	ts := time.Now().Unix()
	runWorkerEvent(t, w, sbom.SBOMCreatedEvent{
		Type:          "sbom.created",
		Timestamp:     ts,
		EventID:       "ev-A",
		SchemaVersion: sbom.SBOMCreatedEventSchemaVersion,
		SBOMID:        sb.ID,
		ImageDigest:   sb.ImageDigest,
	})
	runWorkerEvent(t, w, sbom.SBOMCreatedEvent{
		Type:          "sbom.created",
		Timestamp:     ts,
		EventID:       "ev-B",
		SchemaVersion: sbom.SBOMCreatedEventSchemaVersion,
		SBOMID:        sb.ID,
		ImageDigest:   sb.ImageDigest,
	})

	var latestID string
	if err := db.Raw("SELECT latest_event_id FROM sbom_processing_state WHERE sbom_id = ?", sb.ID).Scan(&latestID).Error; err != nil {
		t.Fatalf("query watermark: %v", err)
	}
	if latestID != "ev-B" {
		t.Fatalf("expected latest_event_id=ev-B, got %q", latestID)
	}
}

func TestWorker_Replay_OlderTimestampSkipped_AfterNewerProcessed(t *testing.T) {
	db := newReplayWorkerTestDB(t)
	sb := seedFinalizedSBOM(t, db)
	w := NewCVEMatcherWorker(nil, db, nil)

	runWorkerEvent(t, w, sbom.SBOMCreatedEvent{
		Type:          "sbom.created",
		Timestamp:     200,
		EventID:       "ev-new",
		SchemaVersion: sbom.SBOMCreatedEventSchemaVersion,
		SBOMID:        sb.ID,
		ImageDigest:   sb.ImageDigest,
	})
	runWorkerEvent(t, w, sbom.SBOMCreatedEvent{
		Type:          "sbom.created",
		Timestamp:     100,
		EventID:       "ev-old",
		SchemaVersion: sbom.SBOMCreatedEventSchemaVersion,
		SBOMID:        sb.ID,
		ImageDigest:   sb.ImageDigest,
	})

	var latestTS int64
	var latestID string
	if err := db.Raw("SELECT latest_event_ts, latest_event_id FROM sbom_processing_state WHERE sbom_id = ?", sb.ID).Row().Scan(&latestTS, &latestID); err != nil {
		t.Fatalf("query watermark: %v", err)
	}
	if latestTS != 200 || latestID != "ev-new" {
		t.Fatalf("expected watermark to remain newest (200, ev-new), got (%d, %q)", latestTS, latestID)
	}
}

