package nvd

import (
	"context"
	"os"
	"testing"
	"time"
)

// runNVDIntegration skips the test unless RUN_NVD_INTEGRATION=1 or NVD_API_KEY is set.
func runNVDIntegration(t *testing.T) {
	if os.Getenv("RUN_NVD_INTEGRATION") != "1" && os.Getenv("NVD_API_KEY") == "" {
		t.Skip("Skipping NVD integration test (set RUN_NVD_INTEGRATION=1 or NVD_API_KEY to run)")
	}
}

// TestNVDQuery_OpenSSL_DebianVersion calls real NVD API for OpenSSL and logs what we get.
// NVD returns keyword-based CVEs (ID, Severity, CVSS, Description); Constraint and FixedVersion
// are empty because NVD API does not provide version ranges.
//
// Run to see actual NVD data for OpenSSL 3.0.18-1~deb12u2 (Debian 12):
//
//	RUN_NVD_INTEGRATION=1 go test -v -run TestNVDQuery_OpenSSL ./core/pkg/cve/database/nvd/
//	NVD_API_KEY=your-key go test -v -run TestNVDQuery_OpenSSL ./core/pkg/cve/database/nvd/
func TestNVDQuery_OpenSSL_DebianVersion(t *testing.T) {
	runNVDIntegration(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	client := NewClient()
	if key := os.Getenv("NVD_API_KEY"); key != "" {
		client.SetAPIKey(key)
	}

	// Query as we would for OpenSSL 3.0.18-1~deb12u2 (Debian 12)
	ecosystem := "generic"
	name := "openssl"
	version := "3.0.18-1~deb12u2"

	cves, err := client.Query(ctx, ecosystem, name, version)
	if err != nil {
		t.Fatalf("NVD Query: %v", err)
	}

	t.Logf("=== NVD response for %s @ %s (ecosystem=%s) ===", name, version, ecosystem)
	t.Logf("Total CVEs returned: %d", len(cves))
	for i, c := range cves {
		desc := c.Description
		if len(desc) > 300 {
			desc = desc[:300] + "..."
		}
		t.Logf("  [%d] ID=%s Severity=%s CVSS=%.1f Published=%s",
			i+1, c.ID, c.Severity, c.CVSSScore, c.Published.Format("2006-01-02"))
		t.Logf("       Description: %s", desc)
		t.Logf("       Constraint=%q FixedVersion=%q", c.Constraint, c.FixedVersion)
		if len(c.References) > 0 {
			t.Logf("       Ref[0]=%s", c.References[0])
		}
	}
}

// TestNVDQuery_ControlPlane_KubeControllerManager calls real NVD API for kube-controller-manager
// and logs CVE info for control-plane version v1.29.15.
//
// Run: RUN_NVD_INTEGRATION=1 go test -v -run TestNVDQuery_ControlPlane ./core/pkg/cve/database/nvd/
func TestNVDQuery_ControlPlane_KubeControllerManager(t *testing.T) {
	runNVDIntegration(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	client := NewClient()
	if key := os.Getenv("NVD_API_KEY"); key != "" {
		client.SetAPIKey(key)
	}

	ecosystem := "generic"
	name := "kube-controller-manager"
	version := "v1.29.15"

	cves, err := client.Query(ctx, ecosystem, name, version)
	if err != nil {
		t.Fatalf("NVD Query: %v", err)
	}

	t.Logf("=== NVD response for %s @ %s (control-plane) ===", name, version)
	t.Logf("Total CVEs returned: %d", len(cves))
	for i, c := range cves {
		desc := c.Description
		if len(desc) > 350 {
			desc = desc[:350] + "..."
		}
		t.Logf("  [%d] ID=%s Severity=%s CVSS=%.1f",
			i+1, c.ID, c.Severity, c.CVSSScore)
		t.Logf("       Description: %s", desc)
		t.Logf("       Constraint=%q", c.Constraint)
	}
}
