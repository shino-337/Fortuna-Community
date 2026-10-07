package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/authorization"
	"github.com/fortuna/core/pkg/models"
)

func sendUserJSON(t *testing.T, r *gin.Engine, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func storedScope(t *testing.T, db *gorm.DB, username string) string {
	t.Helper()
	var u models.User
	if err := db.Where("username = ?", username).First(&u).Error; err != nil {
		t.Fatalf("lookup %s: %v", username, err)
	}
	return u.ScopeJSON
}

func TestRegisterWithoutScopeGrantsNoCluster(t *testing.T) {
	db := setupUserLifecycleDB(t)
	r := routerUserLifecycleV1(t, db)
	tok := loginToken(t, db, userLifecycleSecret, "admin1")

	w := sendUserJSON(t, r, http.MethodPost, "/api/v1/auth/register", tok, map[string]string{
		"username": "noscope", "email": "noscope@test.local", "password": "AnotherPass12!", "role": "viewer",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("register: want 201 got %d %s", w.Code, w.Body.String())
	}
	scope := storedScope(t, db, "noscope")
	if scope != authorization.ScopeNoClusters {
		t.Fatalf("an account created without a scope must see no cluster, got %q", scope)
	}
	if doc := authorization.ParseScopeDocument(scope); doc.ClusterAllowed("c1") {
		t.Fatal("default scope must not allow any cluster")
	}

	// Every cluster stays available, but only when asked for.
	w = sendUserJSON(t, r, http.MethodPost, "/api/v1/auth/register", tok, map[string]string{
		"username": "everycluster", "email": "every@test.local", "password": "AnotherPass12!", "role": "viewer", "scopeJson": "{}",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("register all clusters: want 201 got %d %s", w.Code, w.Body.String())
	}
	if scope := storedScope(t, db, "everycluster"); authorization.ParseScopeDocument(scope).RestrictsClusters() {
		t.Fatalf("explicit {} must stay unrestricted, got %q", scope)
	}
}

func TestClusterAdminNeedsANamedCluster(t *testing.T) {
	db := setupUserLifecycleDB(t)
	r := routerUserLifecycleV1(t, db)
	tok := loginToken(t, db, userLifecycleSecret, "admin1")

	for _, scope := range []string{"", "{}", authorization.ScopeNoClusters} {
		w := sendUserJSON(t, r, http.MethodPost, "/api/v1/auth/register", tok, map[string]string{
			"username": "ca-nocluster", "email": "ca@test.local", "password": "AnotherPass12!", "role": "cluster_admin", "scopeJson": scope,
		})
		if w.Code != http.StatusBadRequest {
			t.Fatalf("cluster_admin with scope %q: want 400 got %d %s", scope, w.Code, w.Body.String())
		}
	}

	opID := strconv.FormatUint(uint64(userIDByUsername(t, db, "op1")), 10)
	// op1 has the default "{}" scope, so promoting it alone would give a cluster admin every cluster.
	if w := sendUserJSON(t, r, http.MethodPatch, "/api/v1/users/"+opID, tok, map[string]string{"role": "cluster_admin"}); w.Code != http.StatusBadRequest {
		t.Fatalf("promote unscoped operator to cluster_admin: want 400 got %d %s", w.Code, w.Body.String())
	}
	if w := sendUserJSON(t, r, http.MethodPatch, "/api/v1/users/"+opID, tok, map[string]string{
		"role": "cluster_admin", "scopeJson": `{"clusters":["c1"]}`,
	}); w.Code != http.StatusOK {
		t.Fatalf("promote with a cluster: want 200 got %d %s", w.Code, w.Body.String())
	}
	if w := sendUserJSON(t, r, http.MethodPatch, "/api/v1/users/"+opID, tok, map[string]string{"scopeJson": "{}"}); w.Code != http.StatusBadRequest {
		t.Fatalf("widen cluster_admin to every cluster: want 400 got %d %s", w.Code, w.Body.String())
	}
}

func TestPatchBlankScopeGrantsNoCluster(t *testing.T) {
	db := setupUserLifecycleDB(t)
	r := routerUserLifecycleV1(t, db)
	tok := loginToken(t, db, userLifecycleSecret, "admin1")
	viewerID := strconv.FormatUint(uint64(userIDByUsername(t, db, "viewer1")), 10)

	if w := sendUserJSON(t, r, http.MethodPatch, "/api/v1/users/"+viewerID, tok, map[string]string{"scopeJson": ""}); w.Code != http.StatusOK {
		t.Fatalf("clear scope: want 200 got %d %s", w.Code, w.Body.String())
	}
	if scope := storedScope(t, db, "viewer1"); scope != authorization.ScopeNoClusters {
		t.Fatalf("clearing a scope must leave no cluster, got %q", scope)
	}
}
