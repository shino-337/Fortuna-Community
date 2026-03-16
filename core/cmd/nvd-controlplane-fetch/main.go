package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/fortuna/core/pkg/cve/database/nvd"
	"github.com/fortuna/core/pkg/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// controlPlaneComponents are the binaries we want CVEs for when running distroless/control-plane pods.
var controlPlaneComponents = []string{
	"kube-apiserver",
	"kube-controller-manager",
	"kube-scheduler",
	"kube-proxy",
	"coredns",
	"etcd",
	"pause",
}

// map to NVD keyword / product if different from name (can extend later)
func keywordFor(name string) string {
	switch name {
	case "kube-apiserver", "kube-controller-manager", "kube-scheduler", "kube-proxy":
		return "kubernetes " + name
	default:
		return name
	}
}

func severityFrom(score float64) string {
	switch {
	case score >= 9.0:
		return "CRITICAL"
	case score >= 7.0:
		return "HIGH"
	case score >= 4.0:
		return "MEDIUM"
	default:
		return "LOW"
	}
}

func main() {
	logger := log.New(log.Writer(), "[nvd-controlplane-fetch] ", log.LstdFlags)
	dbURL := os.Getenv("DATABASE_URL")
	if strings.TrimSpace(dbURL) == "" {
		logger.Fatalf("DATABASE_URL is required")
	}

	db, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{})
	if err != nil {
		logger.Fatalf("failed to connect database: %v", err)
	}

	client := nvd.NewClient()
	if key := strings.TrimSpace(os.Getenv("NVD_API_KEY")); key != "" {
		client.SetAPIKey(key)
		logger.Printf("Using NVD API key")
	}

	ctx := context.Background()
	totalInserted := 0

	for _, name := range controlPlaneComponents {
		kw := keywordFor(name)
		logger.Printf("Querying NVD for %s (keyword=%q)", name, kw)
		cves, err := client.Query(ctx, "generic", kw, "")
		if err != nil {
			logger.Printf("WARN: NVD query failed for %s: %v", name, err)
			continue
		}
		logger.Printf("NVD returned %d CVEs for %s", len(cves), name)

		for _, item := range cves {
			// Map NVD CVE into normalized CVE model. We don't have full CVSS vector/version here,
			// but we still persist score, severity, description, and references.
			cveModel := models.CVE{
				CVEID:       item.ID,
				Severity:    severityFrom(item.CVSSScore),
				CVSSScore:   item.CVSSScore,
				Description: item.Description,
				References:  models.ToJSONBString(item.References),
				Source:      "nvd-controlplane-fetch",
			}
			db.Where(models.CVE{CVEID: cveModel.CVEID}).Assign(cveModel).FirstOrCreate(&cveModel)

			// PackageVulnerability schema now uses version range fields instead of a single constraint.
			// For control-plane prefetch we don't know detailed ranges, so we leave range fields empty
			// and rely on matcher-side NVD logic when applicable.
			pv := models.PackageVulnerability{
				CVEID:       item.ID,
				PackageName: name,
				Ecosystem:   "generic",
				FixedVersion: item.FixedVersion,
				Product:      name,
				CreatedAt:    time.Now(),
			}
			db.Where(models.PackageVulnerability{
				PackageName: name,
				CVEID:       item.ID,
			}).Assign(pv).FirstOrCreate(&pv)
			totalInserted++
		}
	}

	logger.Printf("Completed. Upserted ~%d package_vulnerabilities rows for control-plane components", totalInserted)
	fmt.Println("OK")
}
