package worker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/sbom"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestCVECatalogRematcherQueuesOncePerGeneration(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&models.CatalogGeneration{}, &models.MirrorState{}, &models.SBOM{}, &models.Pod{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	newGen := func() uint {
		at := time.Now()
		g := models.CatalogGeneration{CatalogType: "cve", SourceName: "osv", Status: "active", StartedAt: at, ActivatedAt: &at, RecordCounts: "{}"}
		if err := db.Create(&g).Error; err != nil {
			t.Fatal(err)
		}
		return g.ID
	}
	if err := db.Create(&models.Pod{ClusterID: "c1", Name: "web", Namespace: "default", ServiceAccount: "default", UID: "pod-1"}).Error; err != nil {
		t.Fatal(err)
	}
	for _, s := range []models.SBOM{
		{ClusterID: "c1", PodUID: "pod-1", PodName: "web", Namespace: "default", ContainerName: "app", ImageName: "nginx", ImageTag: "1.27", ImageDigest: "sha256:a", Status: "complete"},
		{ClusterID: "c1", PodUID: "gone", PodName: "old", Namespace: "default", ContainerName: "app", ImageName: "nginx", ImageTag: "1.25", ImageDigest: "sha256:b", Status: "complete"},
	} {
		s := s
		if err := db.Create(&s).Error; err != nil {
			t.Fatal(err)
		}
	}

	var published []sbom.SBOMCreatedEvent
	fail := false
	r := &CVECatalogRematcher{DB: db, Now: func() time.Time { return now }, Publish: func(subject string, data []byte) error {
		if fail {
			return errors.New("nats down")
		}
		if subject != "fortuna.sbom.created" {
			t.Fatalf("subject %q", subject)
		}
		var ev sbom.SBOMCreatedEvent
		if err := json.Unmarshal(data, &ev); err != nil {
			t.Fatal(err)
		}
		published = append(published, ev)
		return nil
	}}
	ctx := context.Background()

	// No catalog yet: nothing to do.
	if n, err := r.CheckOnce(ctx); err != nil || n != 0 {
		t.Fatalf("no catalog: n=%d err=%v", n, err)
	}

	g1 := newGen()
	if n, err := r.CheckOnce(ctx); err != nil || n != 1 {
		t.Fatalf("first check: n=%d err=%v", n, err)
	}
	if ev := published[0]; ev.PodUID != "pod-1" || ev.ContainerName != "app" || ev.ImageDigest != "sha256:a" || ev.SchemaVersion != sbom.SBOMCreatedEventSchemaVersion {
		t.Fatalf("event = %+v", ev)
	}
	// Same generation (another replica, or the next tick): not queued again.
	if n, _ := r.CheckOnce(ctx); n != 0 {
		t.Fatalf("same generation re-queued %d SBOMs", n)
	}

	// A publish failure releases the claim so the next check retries.
	newGen()
	fail = true
	if _, err := r.CheckOnce(ctx); err == nil {
		t.Fatal("expected publish error")
	}
	fail = false
	if n, err := r.CheckOnce(ctx); err != nil || n != 1 {
		t.Fatalf("retry: n=%d err=%v", n, err)
	}
	var st models.MirrorState
	db.Where("name = ?", cveRematchStateName).First(&st)
	if st.Version != int64(g1+1) {
		t.Fatalf("rematch marker = %d, want %d", st.Version, g1+1)
	}
}

func TestCanonicalCVEID(t *testing.T) {
	for in, want := range map[string]string{
		"DEBIAN-CVE-2024-3094":  "CVE-2024-3094",
		"ubuntu-cve-2021-44228": "CVE-2021-44228",
		"ALPINE-CVE-2023-12345": "CVE-2023-12345",
		"CVE-2024-3094":         "CVE-2024-3094",
		"GHSA-xxxx-yyyy-zzzz":   "GHSA-XXXX-YYYY-ZZZZ",
		"RHSA-2026:76130":       "RHSA-2026:76130",
	} {
		if got := canonicalCVEID(in); got != want {
			t.Errorf("canonicalCVEID(%q) = %q, want %q", in, got, want)
		}
	}
}
