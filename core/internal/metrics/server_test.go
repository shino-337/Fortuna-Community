package metrics

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServerExposesRegisteredMetrics(t *testing.T) {
	HTTPRequestsTotal.WithLabelValues("GET", "/test", "200").Inc()

	srv := httptest.NewServer(NewServer("").Handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "fortuna_http_requests_total") {
		t.Fatalf("metrics endpoint: status %d, body missing fortuna_http_requests_total", resp.StatusCode)
	}

	other, err := http.Get(srv.URL + "/api/v1/me")
	if err != nil {
		t.Fatal(err)
	}
	other.Body.Close()
	if other.StatusCode != http.StatusNotFound {
		t.Fatalf("metrics listener must serve only /metrics, got %d", other.StatusCode)
	}
}

func TestAddrFromEnvDisabledByDefault(t *testing.T) {
	t.Setenv("FORTUNA_METRICS_ADDR", "")
	if AddrFromEnv() != "" {
		t.Fatal("metrics endpoint must be disabled when FORTUNA_METRICS_ADDR is unset")
	}
}
