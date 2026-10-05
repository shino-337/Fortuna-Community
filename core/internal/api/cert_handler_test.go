package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCertificateInfoWithoutTLS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/info", NewCertHandler(nil).GetCertificateInfo)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/info", nil))
	if w.Code != http.StatusOK || w.Body.String() != `{"certificates":[],"tlsEnabled":false}` {
		t.Fatalf("got %d %s", w.Code, w.Body)
	}
}
