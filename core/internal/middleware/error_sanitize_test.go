package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestErrorSanitizeKeepsStatusOfUnmatchedAndEmptyResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(ErrorSanitize())
	r.GET("/ok", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	r.GET("/empty", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	r.GET("/boom", func(c *gin.Context) { c.JSON(http.StatusInternalServerError, gin.H{"error": "pq: secret detail"}) })
	for _, tc := range []struct {
		method, path string
		code         int
		body         string
	}{
		{"GET", "/missing", http.StatusNotFound, "404 page not found"},
		{"POST", "/missing", http.StatusNotFound, "404 page not found"},
		{"GET", "/ok", http.StatusOK, `{"ok":true}`},
		{"GET", "/empty", http.StatusNoContent, ""},
		{"GET", "/boom", http.StatusInternalServerError, `{"error":"Internal server error"}`},
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != tc.code || w.Body.String() != tc.body {
			t.Errorf("%s %s: %d %q", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
}
