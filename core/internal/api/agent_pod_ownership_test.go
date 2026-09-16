package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fortuna/core/pkg/agentidentity"
	"github.com/fortuna/core/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func podOwnershipDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Pod{}); err != nil {
		t.Fatal(err)
	}
	for _, pod := range []models.Pod{
		{UID: "pod-a", ClusterID: "cluster-a", Namespace: "team-a", Name: "a", ServiceAccount: "default"},
		{UID: "pod-b", ClusterID: "cluster-b", Namespace: "team-b", Name: "b", ServiceAccount: "default"},
	} {
		if err := db.Create(&pod).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func runPodOwnershipGuard(t *testing.T, db *gorm.DB, eventBatch bool, scoped bool, body string) (*httptest.ResponseRecorder, bool) {
	t.Helper()
	called := false
	r := gin.New()
	if scoped {
		r.Use(func(c *gin.Context) {
			c.Set("fortuna.agent.principal", agentidentity.Principal{CredentialID: "cred-a", ClusterID: "cluster-a", AgentID: "agent-a"})
			c.Next()
		})
	}
	r.POST("/ingest", requireScopedPodEvidenceOwnership(db, eventBatch), func(c *gin.Context) {
		called = true
		c.Status(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodPost, "/ingest", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w, called
}

func TestScopedPodEvidenceOwnership(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := podOwnershipDB(t)

	valid := `{"podUid":"pod-a","clusterId":"cluster-a","namespace":"team-a","connections":[]}`
	if w, called := runPodOwnershipGuard(t, db, false, true, valid); w.Code != http.StatusNoContent || !called {
		t.Fatalf("valid scoped pod rejected: %d %s", w.Code, w.Body.String())
	}

	for name, body := range map[string]string{
		"foreign-cluster-claim": `{"podUid":"pod-a","clusterId":"cluster-b","namespace":"team-a"}`,
		"foreign-pod":           `{"podUid":"pod-b","clusterId":"cluster-a","namespace":"team-b"}`,
		"unknown-pod":           `{"podUid":"missing","clusterId":"cluster-a","namespace":"team-a"}`,
		"wrong-namespace":       `{"podUid":"pod-a","clusterId":"cluster-a","namespace":"other"}`,
	} {
		t.Run(name, func(t *testing.T) {
			w, called := runPodOwnershipGuard(t, db, false, true, body)
			if w.Code != http.StatusForbidden || called {
				t.Fatalf("ownership violation reached handler: %d called=%v body=%s", w.Code, called, w.Body.String())
			}
		})
	}

	if w, called := runPodOwnershipGuard(t, db, false, false, valid); w.Code != http.StatusNoContent || !called {
		t.Fatalf("legacy migration mode unexpectedly blocked: %d %s", w.Code, w.Body.String())
	}
}

func TestScopedPodEventBatchValidatesWholeBatchBeforeHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := podOwnershipDB(t)
	body := `{"clusterId":"cluster-a","events":[` +
		`{"involvedKind":"Pod","involvedUid":"pod-a","namespace":"team-a"},` +
		`{"involvedKind":"Pod","involvedUid":"pod-b","namespace":"team-b"}]}`
	w, called := runPodOwnershipGuard(t, db, true, true, body)
	if w.Code != http.StatusForbidden || called {
		t.Fatalf("mixed ownership batch must fail before handler: %d called=%v body=%s", w.Code, called, w.Body.String())
	}
}

func TestScopedPodOwnershipStorageFailureIsUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	body := `{"podUid":"pod-a","clusterId":"cluster-a","namespace":"team-a"}`
	w, called := runPodOwnershipGuard(t, db, false, true, body)
	if w.Code != http.StatusServiceUnavailable || called {
		t.Fatalf("storage failure must fail closed as unavailable: %d called=%v body=%s", w.Code, called, w.Body.String())
	}
}
