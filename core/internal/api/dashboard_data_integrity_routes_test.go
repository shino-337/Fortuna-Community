package api

import (
	"strings"
	"testing"
)

// The data-integrity report lists the endpoints the Dashboard reads; every entry must
// name a route Core actually registers.
func TestDataIntegrityEndpointsAreRegisteredRoutes(t *testing.T) {
	registered := map[string]bool{}
	for _, spec := range FortunaRouteSecurityInventory(RouteVerifyOptions{CertRoutesRegistered: true}) {
		registered[spec.Path] = true
	}
	for _, ep := range endpointInventory() {
		path := strings.TrimSuffix(ep.Path, "/*")
		if strings.HasSuffix(ep.Path, "/*") {
			found := false
			for p := range registered {
				if strings.HasPrefix(p, path+"/") {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("no registered route under %s", ep.Path)
			}
			continue
		}
		if !registered[path] {
			t.Errorf("data-integrity endpoint %s is not a registered route", ep.Path)
		}
	}
}
