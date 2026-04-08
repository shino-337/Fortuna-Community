package api

import (
	"testing"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/networkbucket"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newR5TestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.PodNetworkConnection{}, &models.RuntimeEvent{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestBuildNetworkQueueSpikeEvents_EmitsSpikeEvent(t *testing.T) {
	db := newR5TestDB(t)
	podUID := "pod-net-r5"
	ns := "fortuna"
	now := time.Now().UTC()

	// Baseline around ~500 bytes queue for same destination, enough samples.
	for i := 0; i < 6; i++ {
		obs := now.Add(-10 * time.Minute)
		row := models.PodNetworkConnection{
			PodUID:        podUID,
			ClusterID:     "c1",
			Namespace:     ns,
			ContainerName: "app",
			DestIP:        "1.2.3.4",
			DestPort:      443,
			Protocol:      "tcp",
			BytesSent:     300,
			BytesRecv:     200,
			ObservedAt:    obs,
			CreatedAt:     obs,
			Bucket5m:      networkbucket.FloorBucket5MUTC(obs),
		}
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("seed baseline: %v", err)
		}
	}

	current := []models.PodNetworkConnection{
		{
			ContainerName: "app",
			DestIP:        "1.2.3.4",
			DestPort:      443,
			Protocol:      "tcp",
			BytesSent:     6000,
			BytesRecv:     2000,
		},
	}
	events, err := buildNetworkQueueSpikeEvents(db, podUID, ns, now, current)
	if err != nil {
		t.Fatalf("buildNetworkQueueSpikeEvents error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 spike event, got %d", len(events))
	}
	if events[0].Capability != "NETWORK_TXRX_QUEUE_SPIKE" || events[0].Syscall != "connect" {
		t.Fatalf("unexpected event markers: %+v", events[0])
	}
}

func TestBuildNetworkQueueSpikeEvents_NoBaselineNoEvent(t *testing.T) {
	db := newR5TestDB(t)
	now := time.Now().UTC()
	current := []models.PodNetworkConnection{
		{
			ContainerName: "app",
			DestIP:        "8.8.8.8",
			DestPort:      53,
			Protocol:      "udp",
			BytesSent:     9000,
			BytesRecv:     1000,
		},
	}
	events, err := buildNetworkQueueSpikeEvents(db, "pod-empty", "ns", now, current)
	if err != nil {
		t.Fatalf("buildNetworkQueueSpikeEvents error: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected no event without baseline, got %d", len(events))
	}
}

func TestBuildNetworkQueueSpikeEvents_CooldownSuppressesRepeatedSpike(t *testing.T) {
	db := newR5TestDB(t)
	podUID := "pod-net-r5-cooldown"
	ns := "fortuna"
	now := time.Now().UTC()
	for i := 0; i < 6; i++ {
		obs := now.Add(-8 * time.Minute)
		row := models.PodNetworkConnection{
			PodUID:        podUID,
			ClusterID:     "c1",
			Namespace:     ns,
			ContainerName: "app",
			DestIP:        "10.0.0.8",
			DestPort:      443,
			Protocol:      "tcp",
			BytesSent:     300,
			BytesRecv:     200,
			ObservedAt:    obs,
			CreatedAt:     obs,
			Bucket5m:      networkbucket.FloorBucket5MUTC(obs),
		}
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("seed baseline: %v", err)
		}
	}
	// Existing spike event in cooldown window.
	existing := models.RuntimeEvent{
		PodUID:     podUID,
		Namespace:  ns,
		Syscall:    "connect",
		Capability: "NETWORK_TXRX_QUEUE_SPIKE",
		TargetPath: "key=app|10.0.0.8|443|tcp dst=10.0.0.8:443 proto=tcp q=9000 avg=500 ratio=18.00 samples=6",
		CreatedAt:  now.Add(-2 * time.Minute),
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("seed existing event: %v", err)
	}
	current := []models.PodNetworkConnection{
		{
			ContainerName: "app",
			DestIP:        "10.0.0.8",
			DestPort:      443,
			Protocol:      "tcp",
			BytesSent:     7000,
			BytesRecv:     3000,
		},
	}
	t.Setenv("POD_DETAIL_NET_SPIKE_COOLDOWN_MINUTES", "10")
	events, err := buildNetworkQueueSpikeEvents(db, podUID, ns, now, current)
	if err != nil {
		t.Fatalf("buildNetworkQueueSpikeEvents error: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected suppression by cooldown, got %d events", len(events))
	}
}
