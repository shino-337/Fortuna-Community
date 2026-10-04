package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRedactQueryRemovesTokens(t *testing.T) {
	got := RedactQuery("/api/v1/ws/risks?token=eyJhbGciOi.secret&clusterId=c1")
	if strings.Contains(got, "eyJhbGciOi") || strings.Contains(got, "secret") {
		t.Fatalf("token leaked in %q", got)
	}
	if !strings.Contains(got, "clusterId=c1") || !strings.Contains(got, "token=REDACTED") {
		t.Fatalf("unexpected redaction %q", got)
	}
	if RedactQuery("/api/v1/pods?limit=10") != "/api/v1/pods?limit=10" {
		t.Fatal("non-sensitive query must be unchanged")
	}
	if RedactQuery("/healthz") != "/healthz" {
		t.Fatal("path without query must be unchanged")
	}
}

func TestMaxRequestBodyRejectsOversizedBodies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(MaxRequestBody(16))
	r.POST("/x", func(c *gin.Context) {
		if _, err := io.ReadAll(c.Request.Body); err != nil {
			c.Status(http.StatusRequestEntityTooLarge)
			return
		}
		c.Status(http.StatusOK)
	})

	small := httptest.NewRecorder()
	r.ServeHTTP(small, httptest.NewRequest(http.MethodPost, "/x", strings.NewReader("ok")))
	if small.Code != http.StatusOK {
		t.Fatalf("small body: got %d", small.Code)
	}

	declared := httptest.NewRecorder()
	r.ServeHTTP(declared, httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(strings.Repeat("a", 64))))
	if declared.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("declared oversized body: got %d", declared.Code)
	}

	// Chunked body without Content-Length is cut off while reading.
	req := httptest.NewRequest(http.MethodPost, "/x", io.NopCloser(strings.NewReader(strings.Repeat("a", 64))))
	req.ContentLength = -1
	streamed := httptest.NewRecorder()
	r.ServeHTTP(streamed, req)
	if streamed.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("streamed oversized body: got %d", streamed.Code)
	}
}

func TestTrustedProxiesFromEnv(t *testing.T) {
	t.Setenv("FORTUNA_TRUSTED_PROXIES", "")
	if got := TrustedProxiesFromEnv(); got != nil {
		t.Fatalf("expected no trusted proxies by default, got %v", got)
	}
	t.Setenv("FORTUNA_TRUSTED_PROXIES", " 10.0.0.0/8, ,192.168.0.0/16 ")
	got := TrustedProxiesFromEnv()
	if len(got) != 2 || got[0] != "10.0.0.0/8" || got[1] != "192.168.0.0/16" {
		t.Fatalf("unexpected proxies %v", got)
	}
}

func TestUntrustedForwardedForIsIgnored(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if err := r.SetTrustedProxies(nil); err != nil {
		t.Fatal(err)
	}
	var seen string
	r.GET("/ip", func(c *gin.Context) { seen = c.ClientIP() })
	req := httptest.NewRequest(http.MethodGet, "/ip", nil)
	req.RemoteAddr = "203.0.113.7:4242"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	r.ServeHTTP(httptest.NewRecorder(), req)
	if seen != "203.0.113.7" {
		t.Fatalf("spoofed X-Forwarded-For accepted: ClientIP=%q", seen)
	}
}
