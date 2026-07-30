package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fortuna/core/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestGetSBOMDetail_DefaultScopeExcludesStalePodSBOM(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Pod{}, &models.SBOM{}, &models.SBOMComponent{}, &models.CVEMatch{}, &models.CVE{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Create(&models.SBOM{
		PodUID:        "pod-stale",
		PodName:       "old-api",
		Namespace:     "default",
		ContainerName: "api",
		ImageName:     "api",
		ImageTag:      "1.0.0",
		Status:        "complete",
		Version:       1,
	}).Error; err != nil {
		t.Fatalf("seed stale sbom: %v", err)
	}

	router := gin.New()
	router.GET("/sbom/:uid", GetSBOMDetail(db))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sbom/pod-stale", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("default stale status=%d body=%s", w.Code, w.Body.String())
	}
	var miss map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &miss); err != nil {
		t.Fatalf("decode stale miss: %v", err)
	}
	if miss["state"] != "stale" {
		t.Fatalf("state=%v", miss["state"])
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/sbom/pod-stale?includeStale=true", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("includeStale status=%d body=%s", w.Code, w.Body.String())
	}
	var detail SBOMDetailDTO
	if err := json.Unmarshal(w.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if detail.PodID != "pod-stale" {
		t.Fatalf("pod id=%q", detail.PodID)
	}
	if detail.ActivePod {
		t.Fatalf("stale detail should not be linked to active pod")
	}
	if detail.LifecycleState != "stale" {
		t.Fatalf("lifecycle=%q", detail.LifecycleState)
	}
}

func TestGetSBOMDetail_DefaultScopeAllowsActivePodSBOM(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("FORTUNA_TRUSTED_REGISTRIES", "registry.k8s.io")
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Pod{}, &models.SBOM{}, &models.SBOMComponent{}, &models.CVEMatch{}, &models.CVE{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Create(&models.Pod{UID: "pod-active", ClusterID: "c1", Name: "api", Namespace: "default", ServiceAccount: "default"}).Error; err != nil {
		t.Fatalf("seed pod: %v", err)
	}
	if err := db.Create(&models.SBOM{
		PodUID:        "pod-active",
		PodName:       "api",
		Namespace:     "default",
		ContainerName: "api",
		ImageName:     "registry.k8s.io/api",
		ImageTag:      "1.0.0",
		ImageDigest:   "sha256:abc123",
		Status:        "complete",
		Version:       1,
	}).Error; err != nil {
		t.Fatalf("seed sbom: %v", err)
	}

	router := gin.New()
	router.GET("/sbom/:uid", GetSBOMDetail(db))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sbom/pod-active", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("active status=%d body=%s", w.Code, w.Body.String())
	}
	var detail SBOMDetailDTO
	if err := json.Unmarshal(w.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if !detail.ActivePod || detail.LifecycleState != "current" {
		t.Fatalf("active/lifecycle=%v/%q", detail.ActivePod, detail.LifecycleState)
	}
	if detail.ImageDigest != "sha256:abc123" {
		t.Fatalf("image digest=%q", detail.ImageDigest)
	}
	if detail.ImageTrust.Status != "strong" || detail.ImageTrust.RegistryClass != "trusted" || !detail.ImageTrust.DigestAvailable || detail.ImageTrust.MutableTag {
		t.Fatalf("image trust=%#v", detail.ImageTrust)
	}
}

func TestBuildImageTrust_FlagsMutableTagAndMissingDigest(t *testing.T) {
	trust := buildImageTrust("nginx", "latest", "")
	if trust.Status != "unknown" {
		t.Fatalf("status=%q", trust.Status)
	}
	if trust.Registry != "docker.io" || trust.RegistryClass != "public" {
		t.Fatalf("registry=%q class=%q", trust.Registry, trust.RegistryClass)
	}
	if trust.DigestAvailable || !trust.MutableTag {
		t.Fatalf("digest/mutable=%v/%v", trust.DigestAvailable, trust.MutableTag)
	}
}

func TestBuildImageTrust_UsesRegistryPolicy(t *testing.T) {
	t.Setenv("FORTUNA_TRUSTED_REGISTRIES", "registry.example.com,*.trusted.local")
	t.Setenv("FORTUNA_BLOCKED_REGISTRIES", "bad.example.com")

	trusted := buildImageTrust("registry.example.com/app/api", "1.0.0", "sha256:abc123")
	if trusted.RegistryClass != "trusted" || trusted.Status != "strong" {
		t.Fatalf("trusted registry trust=%#v", trusted)
	}

	wildcard := buildImageTrust("team.trusted.local/app/api", "1.0.0", "sha256:abc123")
	if wildcard.RegistryClass != "trusted" || wildcard.Status != "strong" {
		t.Fatalf("wildcard trusted registry trust=%#v", wildcard)
	}

	blocked := buildImageTrust("bad.example.com/app/api", "1.0.0", "sha256:abc123")
	if blocked.RegistryClass != "blocked" || blocked.Status != "blocked" {
		t.Fatalf("blocked registry trust=%#v", blocked)
	}
}
