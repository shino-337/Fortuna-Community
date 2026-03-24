package api

import (
	"strings"
	"testing"
	"time"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newR7TestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.PodProcess{}, &models.RuntimeEvent{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestBuildProcessDiffEvents_FirstSnapshotNoBaseline(t *testing.T) {
	db := newR7TestDB(t)
	now := time.Now().UTC()

	current := []models.PodProcess{
		{ContainerName: "app", PID: 1, BinaryPath: "/app/start"},
	}
	events, err := buildProcessDiffEvents(db, "pod-a", "ns-a", now, current)
	if err != nil {
		t.Fatalf("buildProcessDiffEvents error: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected 0 events for first snapshot, got %d", len(events))
	}
}

func TestBuildProcessDiffEvents_OnlyNewProcessesAreEmitted(t *testing.T) {
	db := newR7TestDB(t)
	podUID := "pod-r7"
	ns := "fortuna"

	prevTs := time.Now().UTC().Add(-30 * time.Second)
	prev := []models.PodProcess{
		{
			PodUID:        podUID,
			ClusterID:     "c1",
			Namespace:     ns,
			ContainerName: "app",
			PID:           100,
			ObservedAt:    prevTs,
			CreatedAt:     prevTs,
		},
	}
	if err := db.Create(&prev).Error; err != nil {
		t.Fatalf("seed previous snapshot: %v", err)
	}

	now := prevTs.Add(30 * time.Second)
	current := []models.PodProcess{
		{ContainerName: "app", PID: 100, BinaryPath: "/usr/bin/existing"},
		{ContainerName: "app", PID: 200, BinaryPath: "/usr/bin/newproc"},
	}
	events, err := buildProcessDiffEvents(db, podUID, ns, now, current)
	if err != nil {
		t.Fatalf("buildProcessDiffEvents error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].PodUID != podUID || events[0].Namespace != ns {
		t.Fatalf("unexpected identity fields: %+v", events[0])
	}
	if events[0].Syscall != "execve" || events[0].Capability != "PROCESS_SNAPSHOT_DIFF" {
		t.Fatalf("unexpected event markers: %+v", events[0])
	}
	if events[0].TargetPath != "/usr/bin/newproc" {
		t.Fatalf("unexpected target path: %q", events[0].TargetPath)
	}
}

func TestBuildProcessDiffEvents_TargetFallbackAndTruncate(t *testing.T) {
	db := newR7TestDB(t)
	podUID := "pod-truncate"
	ns := "ns"

	prevTs := time.Now().UTC().Add(-1 * time.Minute)
	prev := models.PodProcess{
		PodUID:        podUID,
		ClusterID:     "c1",
		Namespace:     ns,
		ContainerName: "app",
		PID:           1,
		ObservedAt:    prevTs,
		CreatedAt:     prevTs,
	}
	if err := db.Create(&prev).Error; err != nil {
		t.Fatalf("seed previous snapshot: %v", err)
	}

	longCmd := strings.Repeat("x", 700)
	now := prevTs.Add(1 * time.Minute)
	current := []models.PodProcess{
		{ContainerName: "app", PID: 2, Command: longCmd},
	}
	events, err := buildProcessDiffEvents(db, podUID, ns, now, current)
	if err != nil {
		t.Fatalf("buildProcessDiffEvents error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if len(events[0].TargetPath) != 500 {
		t.Fatalf("expected truncated target len=500, got %d", len(events[0].TargetPath))
	}
}
