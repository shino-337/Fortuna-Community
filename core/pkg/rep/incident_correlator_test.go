package rep

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCorrelateAndPersistRuntimeIncidents_ReconBurst(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.RuntimeEvent{}, &models.RuntimeBehaviorFact{}, &models.RuntimeIncident{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	now := time.Now().UTC()
	podUID := "rep-c-pod-1"

	// Seed recent historical facts to cross threshold.
	for i := 0; i < 5; i++ {
		f := models.RuntimeBehaviorFact{
			FactID:     "seed-fact-" + string(rune('a'+i)),
			EventID:    uint(i + 1),
			PodUID:     podUID,
			Namespace:  "ns1",
			FactType:   "NETWORK_CONNECT",
			Domain:     "network",
			Attributes: "{}",
			SourceRef:  "{}",
			ObservedAt: now.Add(-2 * time.Minute),
			CreatedAt:  now.Add(-2 * time.Minute),
		}
		if err := db.Create(&f).Error; err != nil {
			t.Fatalf("seed fact: %v", err)
		}
	}

	ev := &models.RuntimeEvent{
		ID:        100,
		PodUID:    podUID,
		Namespace: "ns1",
		CreatedAt: now,
	}
	curFacts := []models.RuntimeBehaviorFact{
		{
			FactID:    "100:NETWORK_CONNECT",
			EventID:   100,
			PodUID:    podUID,
			Namespace: "ns1",
			FactType:  "NETWORK_CONNECT",
		},
	}

	// Persist current facts into DB so correlator logic relies on DB as in production.
	for i := range curFacts {
		curFacts[i].Domain = "network"
		curFacts[i].Attributes = "{}"
		curFacts[i].SourceRef = "{}"
		curFacts[i].ObservedAt = now
		curFacts[i].CreatedAt = now
		if err := db.Create(&curFacts[i]).Error; err != nil {
			t.Fatalf("persist cur fact: %v", err)
		}
	}

	if err := correlateAndPersistRuntimeIncidents(context.Background(), db, ev, curFacts); err != nil {
		t.Fatalf("correlate: %v", err)
	}

	var incidents []models.RuntimeIncident
	if err := db.Where("pod_uid = ?", podUID).Find(&incidents).Error; err != nil {
		t.Fatalf("query incidents: %v", err)
	}
	if len(incidents) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(incidents))
	}
	if incidents[0].IncidentType != "RECON_BURST" {
		t.Fatalf("expected RECON_BURST, got %s", incidents[0].IncidentType)
	}
}

func TestCorrelateAndPersistRuntimeIncidents_PostExploitExecChain(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.RuntimeEvent{}, &models.RuntimeBehaviorFact{}, &models.RuntimeIncident{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	now := time.Now().UTC()
	podUID := "rep-c-pod-2"
	seedTypes := []string{"INTERACTIVE_SHELL", "REMOTE_TOOL_EXEC", "TMP_BINARY_EXEC"}
	for i, ft := range seedTypes {
		f := models.RuntimeBehaviorFact{
			FactID:     "seed-chain-" + ft,
			EventID:    uint(i + 20),
			PodUID:     podUID,
			Namespace:  "ns2",
			FactType:   ft,
			Domain:     "execution",
			Attributes: "{}",
			SourceRef:  "{}",
			ObservedAt: now.Add(-3 * time.Minute),
			CreatedAt:  now.Add(-3 * time.Minute),
		}
		if err := db.Create(&f).Error; err != nil {
			t.Fatalf("seed fact: %v", err)
		}
	}

	ev := &models.RuntimeEvent{
		ID:        200,
		PodUID:    podUID,
		Namespace: "ns2",
		CreatedAt: now,
	}
	curFacts := []models.RuntimeBehaviorFact{
		{FactID: "200:INTERACTIVE_SHELL", FactType: "INTERACTIVE_SHELL", PodUID: podUID, Namespace: "ns2"},
	}

	if err := correlateAndPersistRuntimeIncidents(context.Background(), db, ev, curFacts); err != nil {
		t.Fatalf("correlate: %v", err)
	}

	var incidents []models.RuntimeIncident
	if err := db.Where("pod_uid = ? AND incident_type = ?", podUID, "POST_EXPLOIT_EXEC_CHAIN").Find(&incidents).Error; err != nil {
		t.Fatalf("query incidents: %v", err)
	}
	if len(incidents) != 1 {
		t.Fatalf("expected 1 post-exploit incident, got %d", len(incidents))
	}
}

func TestCorrelateAndPersistRuntimeIncidents_ExfilLikeSequence(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.RuntimeEvent{}, &models.RuntimeBehaviorFact{}, &models.RuntimeIncident{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	now := time.Now().UTC()
	podUID := "rep-c-pod-3"
	seed := []models.RuntimeBehaviorFact{
		{
			FactID:     "token-read",
			EventID:    301,
			PodUID:     podUID,
			Namespace:  "ns3",
			FactType:   "SERVICEACCOUNT_TOKEN_READ",
			Domain:     "credentials",
			Attributes: "{}",
			SourceRef:  "{}",
			ObservedAt: now.Add(-2 * time.Minute),
			CreatedAt:  now.Add(-2 * time.Minute),
		},
		{
			FactID:     "external-connect",
			EventID:    302,
			PodUID:     podUID,
			Namespace:  "ns3",
			FactType:   "EXTERNAL_CONNECT",
			Domain:     "network",
			Attributes: "{}",
			SourceRef:  "{}",
			ObservedAt: now.Add(-1 * time.Minute),
			CreatedAt:  now.Add(-1 * time.Minute),
		},
	}
	for i := range seed {
		if err := db.Create(&seed[i]).Error; err != nil {
			t.Fatalf("seed fact: %v", err)
		}
	}

	ev := &models.RuntimeEvent{
		ID:        303,
		PodUID:    podUID,
		Namespace: "ns3",
		CreatedAt: now,
	}
	curFacts := []models.RuntimeBehaviorFact{
		{FactID: "303:EXTERNAL_CONNECT", FactType: "EXTERNAL_CONNECT", PodUID: podUID, Namespace: "ns3"},
	}
	if err := correlateAndPersistRuntimeIncidents(context.Background(), db, ev, curFacts); err != nil {
		t.Fatalf("correlate: %v", err)
	}

	var incidents []models.RuntimeIncident
	if err := db.Where("pod_uid = ? AND incident_type = ?", podUID, "EXFIL_LIKE_SEQUENCE").Find(&incidents).Error; err != nil {
		t.Fatalf("query incidents: %v", err)
	}
	if len(incidents) != 1 {
		t.Fatalf("expected 1 exfil-like incident, got %d", len(incidents))
	}
}

func TestCorrelateAndPersistRuntimeIncidents_ReconBurst_NoEmitBelowThreshold(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.RuntimeEvent{}, &models.RuntimeBehaviorFact{}, &models.RuntimeIncident{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	now := time.Now().UTC()
	podUID := "rep-c-neg-1"
	// total NETWORK/EXTERNAL facts in window = 5 (< threshold 6)
	for i := 0; i < 4; i++ {
		f := models.RuntimeBehaviorFact{
			FactID:     "neg-recon-" + string(rune('a'+i)),
			EventID:    uint(i + 1),
			PodUID:     podUID,
			Namespace:  "nsn",
			FactType:   "NETWORK_CONNECT",
			Domain:     "network",
			Attributes: "{}",
			SourceRef:  "{}",
			ObservedAt: now.Add(-2 * time.Minute),
			CreatedAt:  now.Add(-2 * time.Minute),
		}
		if err := db.Create(&f).Error; err != nil {
			t.Fatalf("seed fact: %v", err)
		}
	}
	ev := &models.RuntimeEvent{ID: 401, PodUID: podUID, Namespace: "nsn", CreatedAt: now}
	curFacts := []models.RuntimeBehaviorFact{{FactID: "401:NETWORK_CONNECT", FactType: "NETWORK_CONNECT", PodUID: podUID, Namespace: "nsn"}}
	if err := correlateAndPersistRuntimeIncidents(context.Background(), db, ev, curFacts); err != nil {
		t.Fatalf("correlate: %v", err)
	}

	var count int64
	if err := db.Model(&models.RuntimeIncident{}).Where("pod_uid = ? AND incident_type = ?", podUID, "RECON_BURST").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected 0 RECON_BURST incidents, got %d", count)
	}
}

func TestCorrelateAndPersistRuntimeIncidents_ReconBurst_SuppressedWithinCooldown(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.RuntimeEvent{}, &models.RuntimeBehaviorFact{}, &models.RuntimeIncident{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	now := time.Now().UTC()
	podUID := "rep-c-suppress-1"

	// Seed an existing incident within cooldown.
	// Use different 5m bucket to ensure we'd create a new row without suppression.
	d := DetectorReconBurst
	existingAt := now.Add(-25 * time.Minute)
	existingBucket := d.Bucket(existingAt)
	existingIncidentID := fmt.Sprintf("%s:%s:%d", podUID, d.IncidentType, existingBucket)
	existing := models.RuntimeIncident{
		IncidentID:   existingIncidentID,
		PodUID:       podUID,
		Namespace:    "ns",
		IncidentType: d.IncidentType,
		SeverityHint: d.SeverityHint,
		Confidence:   0.50,
		FirstSeenAt:  existingAt,
		LastSeenAt:   existingAt,
		Window:       "5m",
		EvidenceRefs: "[]",
		Metadata:     "{}",
		CreatedAt:    existingAt,
		UpdatedAt:    existingAt,
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("seed incident: %v", err)
	}

	// Trigger event within cooldown at a different 5m bucket.
	eventAt := now.Add(-15 * time.Minute)
	ev := &models.RuntimeEvent{ID: 600, PodUID: podUID, Namespace: "ns", CreatedAt: eventAt}

	// Seed enough distinct NETWORK_CONNECT/EXTERNAL_CONNECT facts in the event's 5m window.
	// correlator counts DISTINCT fact_id from DB.
	since := eventAt.Add(-d.Window)
	factsToSeed := make([]models.RuntimeBehaviorFact, 0, 6)
	for i := 0; i < 6; i++ {
		ft := "NETWORK_CONNECT"
		if i%2 == 1 {
			ft = "EXTERNAL_CONNECT"
		}
		f := models.RuntimeBehaviorFact{
			FactID:     fmt.Sprintf("seed-%d", i),
			EventID:    uint(i + 1000),
			PodUID:     podUID,
			Namespace:  "ns",
			FactType:   ft,
			Domain:     "network",
			Attributes: "{}",
			SourceRef:  "{}",
			ObservedAt: since.Add(time.Duration(i) * time.Minute),
			CreatedAt:  since.Add(time.Duration(i) * time.Minute),
		}
		factsToSeed = append(factsToSeed, f)
	}
	for i := range factsToSeed {
		if err := db.Create(&factsToSeed[i]).Error; err != nil {
			t.Fatalf("seed fact: %v", err)
		}
	}

	// Provide a minimal facts slice to satisfy trigger condition.
	curFacts := []models.RuntimeBehaviorFact{factsToSeed[0]}
	if err := correlateAndPersistRuntimeIncidents(context.Background(), db, ev, curFacts); err != nil {
		t.Fatalf("correlate: %v", err)
	}

	var count int64
	if err := db.Model(&models.RuntimeIncident{}).
		Where("pod_uid = ? AND incident_type = ?", podUID, d.IncidentType).
		Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	// Suppression should prevent creating an additional bucket incident row.
	if count != 1 {
		t.Fatalf("expected 1 incident (suppressed), got %d", count)
	}
}

func TestCorrelateAndPersistRuntimeIncidents_PostExploit_NoEmitWhenChainIncomplete(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.RuntimeEvent{}, &models.RuntimeBehaviorFact{}, &models.RuntimeIncident{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	now := time.Now().UTC()
	podUID := "rep-c-neg-2"
	seedTypes := []string{"INTERACTIVE_SHELL", "REMOTE_TOOL_EXEC"} // missing TMP_BINARY_EXEC
	for i, ft := range seedTypes {
		f := models.RuntimeBehaviorFact{
			FactID:     "neg-chain-" + ft,
			EventID:    uint(i + 10),
			PodUID:     podUID,
			Namespace:  "nsn2",
			FactType:   ft,
			Domain:     "execution",
			Attributes: "{}",
			SourceRef:  "{}",
			ObservedAt: now.Add(-2 * time.Minute),
			CreatedAt:  now.Add(-2 * time.Minute),
		}
		if err := db.Create(&f).Error; err != nil {
			t.Fatalf("seed fact: %v", err)
		}
	}
	ev := &models.RuntimeEvent{ID: 402, PodUID: podUID, Namespace: "nsn2", CreatedAt: now}
	curFacts := []models.RuntimeBehaviorFact{{FactID: "402:INTERACTIVE_SHELL", FactType: "INTERACTIVE_SHELL", PodUID: podUID, Namespace: "nsn2"}}
	if err := correlateAndPersistRuntimeIncidents(context.Background(), db, ev, curFacts); err != nil {
		t.Fatalf("correlate: %v", err)
	}
	var count int64
	if err := db.Model(&models.RuntimeIncident{}).Where("pod_uid = ? AND incident_type = ?", podUID, "POST_EXPLOIT_EXEC_CHAIN").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected 0 POST_EXPLOIT_EXEC_CHAIN incidents, got %d", count)
	}
}

func TestCorrelateAndPersistRuntimeIncidents_Exfil_NoEmitWhenOrderInvalid(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.RuntimeEvent{}, &models.RuntimeBehaviorFact{}, &models.RuntimeIncident{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	now := time.Now().UTC()
	podUID := "rep-c-neg-3"
	// external connect happens before token read -> should not emit exfil-like
	seed := []models.RuntimeBehaviorFact{
		{
			FactID:     "neg-ext-first",
			EventID:    501,
			PodUID:     podUID,
			Namespace:  "nsn3",
			FactType:   "EXTERNAL_CONNECT",
			Domain:     "network",
			Attributes: "{}",
			SourceRef:  "{}",
			ObservedAt: now.Add(-2 * time.Minute),
			CreatedAt:  now.Add(-2 * time.Minute),
		},
		{
			FactID:     "neg-token-later",
			EventID:    502,
			PodUID:     podUID,
			Namespace:  "nsn3",
			FactType:   "SERVICEACCOUNT_TOKEN_READ",
			Domain:     "credentials",
			Attributes: "{}",
			SourceRef:  "{}",
			ObservedAt: now.Add(-1 * time.Minute),
			CreatedAt:  now.Add(-1 * time.Minute),
		},
	}
	for i := range seed {
		if err := db.Create(&seed[i]).Error; err != nil {
			t.Fatalf("seed fact: %v", err)
		}
	}
	ev := &models.RuntimeEvent{ID: 503, PodUID: podUID, Namespace: "nsn3", CreatedAt: now}
	curFacts := []models.RuntimeBehaviorFact{{FactID: "503:SERVICEACCOUNT_TOKEN_READ", FactType: "SERVICEACCOUNT_TOKEN_READ", PodUID: podUID, Namespace: "nsn3"}}
	if err := correlateAndPersistRuntimeIncidents(context.Background(), db, ev, curFacts); err != nil {
		t.Fatalf("correlate: %v", err)
	}
	var count int64
	if err := db.Model(&models.RuntimeIncident{}).Where("pod_uid = ? AND incident_type = ?", podUID, "EXFIL_LIKE_SEQUENCE").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected 0 EXFIL_LIKE_SEQUENCE incidents, got %d", count)
	}
}

// G-REP-GOV-01 / ADR-004: duplicate replay — second correlate with same evidence must not create extra RECON_BURST rows.
func TestCorrelateAndPersistRuntimeIncidents_ReconBurst_DuplicateReplayStable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.RuntimeEvent{}, &models.RuntimeBehaviorFact{}, &models.RuntimeIncident{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	now := time.Now().UTC()
	podUID := "rep-c-replay-1"
	for i := 0; i < 6; i++ {
		f := models.RuntimeBehaviorFact{
			FactID:     fmt.Sprintf("replay-fact-%d", i),
			EventID:    uint(i + 1),
			PodUID:     podUID,
			Namespace:  "ns",
			FactType:   "NETWORK_CONNECT",
			Domain:     "network",
			Attributes: "{}",
			SourceRef:  "{}",
			ObservedAt: now.Add(-1 * time.Minute),
			CreatedAt:  now.Add(-1 * time.Minute),
		}
		if err := db.Create(&f).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	ev := &models.RuntimeEvent{ID: 99, PodUID: podUID, Namespace: "ns", CreatedAt: now}
	cur := []models.RuntimeBehaviorFact{{FactID: "99:NETWORK_CONNECT", FactType: "NETWORK_CONNECT", PodUID: podUID, Namespace: "ns", Domain: "network", Attributes: "{}", SourceRef: "{}", ObservedAt: now, CreatedAt: now}}
	if err := db.Create(&cur[0]).Error; err != nil {
		t.Fatal(err)
	}

	for pass := 0; pass < 2; pass++ {
		if err := correlateAndPersistRuntimeIncidents(context.Background(), db, ev, cur); err != nil {
			t.Fatalf("pass %d: %v", pass, err)
		}
	}
	var count int64
	if err := db.Model(&models.RuntimeIncident{}).Where("pod_uid = ? AND incident_type = ?", podUID, DetectorReconBurst.IncidentType).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 incident after duplicate replay, got %d", count)
	}
}

// G-REP-GOV-01: facts inserted in reverse time order but all inside window — same outcome as ordered insert.
func TestCorrelateAndPersistRuntimeIncidents_ReconBurst_OutOfOrderInsertStillEmits(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.RuntimeEvent{}, &models.RuntimeBehaviorFact{}, &models.RuntimeIncident{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	now := time.Now().UTC()
	podUID := "rep-c-oo-1"
	base := now.Add(-3 * time.Minute)
	// Insert from newest to oldest (reverse order).
	for i := 5; i >= 0; i-- {
		f := models.RuntimeBehaviorFact{
			FactID:     fmt.Sprintf("oo-fact-%d", i),
			EventID:    uint(200 + i),
			PodUID:     podUID,
			Namespace:  "ns",
			FactType:   "EXTERNAL_CONNECT",
			Domain:     "network",
			Attributes: "{}",
			SourceRef:  "{}",
			ObservedAt: base.Add(time.Duration(i) * time.Second),
			CreatedAt:  base.Add(time.Duration(i) * time.Second),
		}
		if err := db.Create(&f).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	ev := &models.RuntimeEvent{ID: 300, PodUID: podUID, Namespace: "ns", CreatedAt: now}
	cur := []models.RuntimeBehaviorFact{{FactID: "300:EXTERNAL_CONNECT", FactType: "EXTERNAL_CONNECT", PodUID: podUID, Namespace: "ns", Domain: "network", Attributes: "{}", SourceRef: "{}", ObservedAt: now, CreatedAt: now}}
	if err := db.Create(&cur[0]).Error; err != nil {
		t.Fatal(err)
	}

	if err := correlateAndPersistRuntimeIncidents(context.Background(), db, ev, cur); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&models.RuntimeIncident{}).Where("pod_uid = ? AND incident_type = ?", podUID, DetectorReconBurst.IncidentType).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 RECON_BURST with out-of-order fact inserts, got %d", count)
	}
}
