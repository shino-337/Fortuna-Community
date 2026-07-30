package risk

import (
	"context"
	"fmt"
	"log"
	"math"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SupplyChainEntry represents a single CVE → package → container → pod → node path.
type SupplyChainEntry struct {
	CVEID          string  `json:"cveId"`
	Severity       string  `json:"severity"`
	CVSS           float32 `json:"cvss"`
	PackageName    string  `json:"packageName"`
	PackageVersion string  `json:"packageVersion"`
	ImageName      string  `json:"imageName"`
	ImageTag       string  `json:"imageTag"`
	PodName        string  `json:"podName"`
	PodUID         string  `json:"podUid"`
	Namespace      string  `json:"namespace"`
	NodeName       string  `json:"nodeName"`
	FixedVersion   string  `json:"fixedVersion,omitempty"`
	SbomSource     string  `json:"sbomSource,omitempty"`
}

// SupplyChainSummary is the top-level supply chain correlation response.
type SupplyChainSummary struct {
	TotalCVEs         int                   `json:"totalCves"`
	TotalPackages     int                   `json:"totalPackages"`
	TotalImages       int                   `json:"totalImages"`
	TotalPods         int                   `json:"totalPods"`
	TotalNodes        int                   `json:"totalNodes"`
	Scope             string                `json:"scope"` // current | current_and_historical
	SeverityBreakdown map[string]int        `json:"severityBreakdown"`
	TopVulnImages     []ImageVulnSummary    `json:"topVulnImages"`
	NodeExposure      []NodeExposureSummary `json:"nodeExposure"`
	NamespaceExposure []NamespaceExposure   `json:"namespaceExposure"`
	Entries           []SupplyChainEntry    `json:"entries,omitempty"`
}

// ImageVulnSummary aggregates CVEs per image.
type ImageVulnSummary struct {
	Image         string `json:"image"`
	PodCount      int    `json:"podCount"`
	CriticalCount int    `json:"criticalCount"`
	HighCount     int    `json:"highCount"`
	MediumCount   int    `json:"mediumCount"`
	LowCount      int    `json:"lowCount"`
	TotalCVEs     int    `json:"totalCves"`
}

// NodeExposureSummary aggregates CVE exposure per node.
type NodeExposureSummary struct {
	NodeName      string `json:"nodeName"`
	PodCount      int    `json:"podCount"`
	CriticalCount int    `json:"criticalCount"`
	HighCount     int    `json:"highCount"`
	TotalCVEs     int    `json:"totalCves"`
}

// NamespaceExposure aggregates CVE exposure per namespace.
type NamespaceExposure struct {
	Namespace     string  `json:"namespace"`
	PodCount      int     `json:"podCount"`
	CriticalCount int     `json:"criticalCount"`
	HighCount     int     `json:"highCount"`
	TotalCVEs     int     `json:"totalCves"`
	AvgCVSS       float64 `json:"avgCvss"`
}

type supplyChainRow struct {
	CVEID          string
	Severity       string
	CVSS           float32
	PackageName    string
	PackageVersion string
	FixedVersion   string
	ImageName      string
	ImageTag       string
	SbomSource     string
	PodName        string
	PodUID         string
	Namespace      string
	NodeName       string
}

// GetSupplyChainCorrelation returns the full CVE → package → container → pod → node correlation view.
func GetSupplyChainCorrelation(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()

		namespace := c.Query("namespace")
		severity := c.Query("severity")
		nodeName := c.Query("node")
		limitStr := c.DefaultQuery("limit", "500")
		limit, _ := strconv.Atoi(limitStr)
		if limit <= 0 || limit > 5000 {
			limit = 500
		}
		includeEntries := c.DefaultQuery("entries", "false") == "true"
		includeHistorical := c.DefaultQuery("includeHistorical", "false") == "true"

		rows, err := querySupplyChain(ctx, db, namespace, severity, nodeName, limit, includeHistorical)
		if err != nil {
			log.Printf("[SupplyChain] query error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query supply chain data"})
			return
		}

		summary := buildSummary(rows, includeEntries)
		if includeHistorical {
			summary.Scope = "current_and_historical"
		} else {
			summary.Scope = "current"
		}
		c.JSON(http.StatusOK, summary)
	}
}

func querySupplyChain(ctx context.Context, db *gorm.DB, namespace, severity, nodeName string, limit int, includeHistorical bool) ([]supplyChainRow, error) {
	query := db.WithContext(ctx).Raw(`
SELECT
  cm.cve_id,
  cm.severity,
  cm.cvss,
  cm.package_name,
  cm.package_version,
  COALESCE(cm.fixed_version, '') AS fixed_version,
  s.image_name,
  s.image_tag,
  COALESCE(s.sbom_source, '') AS sbom_source,
  COALESCE(s.pod_name, '') AS pod_name,
  COALESCE(s.pod_uid, '') AS pod_uid,
  COALESCE(s.namespace, '') AS namespace,
  COALESCE(p.node_name, '') AS node_name
FROM cve_matches cm
JOIN sboms s ON s.id = cm.sbom_id AND s.deleted_at IS NULL
LEFT JOIN pods p ON p.uid = s.pod_uid AND p.deleted_at IS NULL
WHERE cm.deleted_at IS NULL
  AND ($1 = '' OR s.namespace = $1)
  AND ($2 = '' OR cm.severity = $2)
  AND ($3 = '' OR p.node_name = $3)
  AND ($4 = true OR p.id IS NOT NULL)
ORDER BY cm.cvss DESC, cm.severity ASC
LIMIT $5
`, namespace, severity, nodeName, includeHistorical, limit)

	var rows []supplyChainRow
	if err := query.Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("supply chain query: %w", err)
	}
	return rows, nil
}

func buildSummary(rows []supplyChainRow, includeEntries bool) SupplyChainSummary {
	sevBreakdown := map[string]int{}
	cveSet := map[string]bool{}
	pkgSet := map[string]bool{}
	imageSet := map[string]bool{}
	podSet := map[string]bool{}
	nodeSet := map[string]bool{}

	// Per-image aggregation
	imageAgg := map[string]*ImageVulnSummary{}
	// Per-node aggregation
	nodeAgg := map[string]*NodeExposureSummary{}
	nodePods := map[string]map[string]bool{}
	// Per-namespace aggregation
	nsAgg := map[string]*nsAggData{}

	var entries []SupplyChainEntry

	for _, r := range rows {
		cveSet[r.CVEID] = true
		pkgSet[r.PackageName+"@"+r.PackageVersion] = true

		img := r.ImageName + ":" + r.ImageTag
		imageSet[img] = true
		if r.PodUID != "" {
			podSet[r.PodUID] = true
		}
		if r.NodeName != "" {
			nodeSet[r.NodeName] = true
		}

		sev := normalizeSev(r.Severity)
		sevBreakdown[sev]++

		// image agg
		if _, ok := imageAgg[img]; !ok {
			imageAgg[img] = &ImageVulnSummary{Image: img}
		}
		ia := imageAgg[img]
		ia.TotalCVEs++
		addSevCount(sev, &ia.CriticalCount, &ia.HighCount, &ia.MediumCount, &ia.LowCount)
		if r.PodUID != "" {
			ia.PodCount++ // approximate (may overcount if same pod, multiple CVEs)
		}

		// node agg
		if r.NodeName != "" {
			if _, ok := nodeAgg[r.NodeName]; !ok {
				nodeAgg[r.NodeName] = &NodeExposureSummary{NodeName: r.NodeName}
				nodePods[r.NodeName] = map[string]bool{}
			}
			na := nodeAgg[r.NodeName]
			na.TotalCVEs++
			if sev == "CRITICAL" {
				na.CriticalCount++
			} else if sev == "HIGH" {
				na.HighCount++
			}
			if r.PodUID != "" {
				nodePods[r.NodeName][r.PodUID] = true
			}
		}

		// namespace agg
		if r.Namespace != "" {
			if _, ok := nsAgg[r.Namespace]; !ok {
				nsAgg[r.Namespace] = &nsAggData{ns: r.Namespace}
			}
			na := nsAgg[r.Namespace]
			na.totalCVEs++
			na.cvssSum += float64(r.CVSS)
			if sev == "CRITICAL" {
				na.critical++
			} else if sev == "HIGH" {
				na.high++
			}
			if r.PodUID != "" {
				if na.pods == nil {
					na.pods = map[string]bool{}
				}
				na.pods[r.PodUID] = true
			}
		}

		if includeEntries {
			entries = append(entries, SupplyChainEntry{
				CVEID:          r.CVEID,
				Severity:       r.Severity,
				CVSS:           r.CVSS,
				PackageName:    r.PackageName,
				PackageVersion: r.PackageVersion,
				ImageName:      r.ImageName,
				ImageTag:       r.ImageTag,
				PodName:        r.PodName,
				PodUID:         r.PodUID,
				Namespace:      r.Namespace,
				NodeName:       r.NodeName,
				FixedVersion:   r.FixedVersion,
				SbomSource:     r.SbomSource,
			})
		}
	}

	// Build sorted top vuln images
	topImages := make([]ImageVulnSummary, 0, len(imageAgg))
	for _, v := range imageAgg {
		topImages = append(topImages, *v)
	}
	sort.Slice(topImages, func(i, j int) bool {
		return topImages[i].CriticalCount > topImages[j].CriticalCount ||
			(topImages[i].CriticalCount == topImages[j].CriticalCount && topImages[i].TotalCVEs > topImages[j].TotalCVEs)
	})
	if len(topImages) > 20 {
		topImages = topImages[:20]
	}

	// Build node exposure
	nodeExposure := make([]NodeExposureSummary, 0, len(nodeAgg))
	for name, v := range nodeAgg {
		v.PodCount = len(nodePods[name])
		nodeExposure = append(nodeExposure, *v)
	}
	sort.Slice(nodeExposure, func(i, j int) bool {
		return nodeExposure[i].CriticalCount > nodeExposure[j].CriticalCount
	})

	// Build namespace exposure
	namespaceExposure := make([]NamespaceExposure, 0, len(nsAgg))
	for _, v := range nsAgg {
		avg := 0.0
		if v.totalCVEs > 0 {
			avg = v.cvssSum / float64(v.totalCVEs)
		}
		namespaceExposure = append(namespaceExposure, NamespaceExposure{
			Namespace:     v.ns,
			PodCount:      len(v.pods),
			CriticalCount: v.critical,
			HighCount:     v.high,
			TotalCVEs:     v.totalCVEs,
			AvgCVSS:       math.Round(avg*100) / 100,
		})
	}
	sort.Slice(namespaceExposure, func(i, j int) bool {
		return namespaceExposure[i].CriticalCount > namespaceExposure[j].CriticalCount
	})

	return SupplyChainSummary{
		TotalCVEs:         len(cveSet),
		TotalPackages:     len(pkgSet),
		TotalImages:       len(imageSet),
		TotalPods:         len(podSet),
		TotalNodes:        len(nodeSet),
		SeverityBreakdown: sevBreakdown,
		TopVulnImages:     topImages,
		NodeExposure:      nodeExposure,
		NamespaceExposure: namespaceExposure,
		Entries:           entries,
	}
}

type nsAggData struct {
	ns        string
	totalCVEs int
	critical  int
	high      int
	cvssSum   float64
	pods      map[string]bool
}

func normalizeSev(s string) string {
	switch s {
	case "CRITICAL", "critical":
		return "CRITICAL"
	case "HIGH", "high":
		return "HIGH"
	case "MEDIUM", "medium":
		return "MEDIUM"
	case "LOW", "low":
		return "LOW"
	default:
		return "UNKNOWN"
	}
}

func addSevCount(sev string, crit, high, med, low *int) {
	switch sev {
	case "CRITICAL":
		*crit++
	case "HIGH":
		*high++
	case "MEDIUM":
		*med++
	case "LOW":
		*low++
	}
}
