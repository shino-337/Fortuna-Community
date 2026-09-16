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

func runtimeOwnershipDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Pod{}); err != nil {
		t.Fatal(err)
	}
	for _, pod := range []models.Pod{
		{UID: "pod-a", ClusterID: "cluster-a", Namespace: "team-a", Name: "pod-a", ServiceAccount: "default"},
		{UID: "pod-b", ClusterID: "cluster-b", Namespace: "team-b", Name: "pod-b", ServiceAccount: "default"},
	} {
		if err := db.Create(&pod).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func runRuntimeOwnershipGuard(t *testing.T, db *gorm.DB, scoped bool, body string) (*httptest.ResponseRecorder, bool) {
	t.Helper()
	called := false
	r := gin.New()
	if scoped {
		r.Use(func(c *gin.Context) {
			c.Set("fortuna.agent.principal", agentidentity.Principal{CredentialID: "cred-a", ClusterID: "cluster-a", AgentID: "agent-a"})
			c.Next()
		})
	}
	r.POST("/runtime", requireScopedRuntimeOwnership(db), func(c *gin.Context) {
		called = true
		c.Status(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodPost, "/runtime", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w, called
}

func TestScopedRuntimeOwnershipValidatesWholeBatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := runtimeOwnershipDB(t)

	valid := `[{"pod":{"uid":"pod-a","namespace":"team-a"},"syscall":"execve"}]`
	if w, called := runRuntimeOwnershipGuard(t, db, true, valid); w.Code != http.StatusNoContent || !called {
		t.Fatalf("valid runtime batch rejected: %d called=%v body=%s", w.Code, called, w.Body.String())
	}

	mixed := `[` +
		`{"pod":{"uid":"pod-a","namespace":"team-a"},"syscall":"execve"},` +
		`{"pod":{"uid":"pod-b","namespace":"team-b"},"syscall":"connect"}]`
	if w, called := runRuntimeOwnershipGuard(t, db, true, mixed); w.Code != http.StatusForbidden || called {
		t.Fatalf("mixed cluster runtime batch reached handler: %d called=%v body=%s", w.Code, called, w.Body.String())
	}
}

func TestScopedRuntimeOwnershipSupportsLegacyPodUIDAliases(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := runtimeOwnershipDB(t)
	for _, body := range []string{
		`{"pod_uid":"pod-a","namespace":"team-a","syscall":"execve"}`,
		`{"podUid":"pod-a","namespace":"team-a","syscall":"execve"}`,
	} {
		if w, called := runRuntimeOwnershipGuard(t, db, true, body); w.Code != http.StatusNoContent || !called {
			t.Fatalf("legacy UID alias rejected: %d called=%v body=%s", w.Code, called, w.Body.String())
		}
	}
}

func TestScopedRuntimeOwnershipRejectsUnknownAndNamespaceMismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := runtimeOwnershipDB(t)
	for name, body := range map[string]string{
		"unknown-pod":     `{"pod":{"uid":"missing","namespace":"team-a"}}`,
		"wrong-namespace": `{"pod":{"uid":"pod-a","namespace":"other"}}`,
		"missing-pod":     `{"syscall":"execve"}`,
	} {
		t.Run(name, func(t *testing.T) {
			w, called := runRuntimeOwnershipGuard(t, db, true, body)
			if w.Code == http.StatusNoContent || called {
				t.Fatalf("invalid runtime ownership reached handler: %d called=%v body=%s", w.Code, called, w.Body.String())
			}
		})
	}
}

func TestScopedRuntimeOwnershipStorageFailureIsUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	body := `{"pod":{"uid":"pod-a","namespace":"team-a"}}`
	w, called := runRuntimeOwnershipGuard(t, db, true, body)
	if w.Code != http.StatusServiceUnavailable || called {
		t.Fatalf("storage failure must be unavailable: %d called=%v body=%s", w.Code, called, w.Body.String())
	}
}

func TestRuntimeOwnershipLegacyModeRemainsCompatibleUntilRouteCutover(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := runtimeOwnershipDB(t)
	body := `{"pod":{"uid":"pod-b","namespace":"team-b"}}`
	w, called := runRuntimeOwnershipGuard(t, db, false, body)
	if w.Code != http.StatusNoContent || !called {
		t.Fatalf("legacy migration mode unexpectedly blocked: %d called=%v body=%s", w.Code, called, w.Body.String())
	}
}
