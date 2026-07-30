package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fortuna/core/internal/middleware"
	"github.com/gin-gonic/gin"
)

func TestRequireIngestToken_emptyFailsClosed(t *testing.T) {
	t.Setenv("FORTUNA_ALLOW_UNAUTHED_INGEST", "")
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/ingest", middleware.RequireIngestToken(""), func(c *gin.Context) { c.Status(http.StatusOK) })
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/ingest", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503 got %d body=%s", w.Code, w.Body.String())
	}
}

func TestRequireIngestToken_emptyAllowsWithExplicitDevOptIn(t *testing.T) {
	t.Setenv("FORTUNA_ALLOW_UNAUTHED_INGEST", "1")
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/ingest", middleware.RequireIngestToken(""), func(c *gin.Context) { c.Status(http.StatusOK) })
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/ingest", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d body=%s", w.Code, w.Body.String())
	}
}

func TestRequireIngestToken_header(t *testing.T) {
	t.Setenv("FORTUNA_ALLOW_UNAUTHED_INGEST", "")
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/ingest", middleware.RequireIngestToken("secret-token-xyz"), func(c *gin.Context) { c.Status(http.StatusOK) })
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/ingest", nil)
	req.Header.Set("X-Fortuna-Ingest-Token", "secret-token-xyz")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d body=%s", w.Code, w.Body.String())
	}
}

func TestRequireIngestToken_bearer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/ingest", middleware.RequireIngestToken("abc"), func(c *gin.Context) { c.Status(http.StatusOK) })
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/ingest", nil)
	req.Header.Set("Authorization", "Bearer abc")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", w.Code)
	}
}

func TestRequireIngestToken_rejects(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/ingest", middleware.RequireIngestToken("good"), func(c *gin.Context) { c.Status(http.StatusOK) })
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/ingest", nil)
	req.Header.Set("X-Fortuna-Ingest-Token", "wrong")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d", w.Code)
	}
}
