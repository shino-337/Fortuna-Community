package risk

import (
	"context"
	"log"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// RuntimeCVECorrelation joins a pod's runtime events with its SBOM CVE matches.
// Answers: "This pod had suspicious activity AND has these known vulnerabilities."

// RuntimeCVEEntry is a single correlated record.
type RuntimeCVEEntry struct {
	PodUID          string    `json:"podUid"`
	PodName         string    `json:"podName"`
	Namespace       string    `json:"namespace"`
	NodeName        string    `json:"nodeName,omitempty"`
	EventType       string    `json:"eventType"`
	Signal          string    `json:"signal,omitempty"`
	Mitre           string    `json:"mitreTechnique,omitempty"`
	EventSeverity   string    `json:"eventSeverity"`
	EventObserved   time.Time `json:"eventObserved"`
	Syscall         string    `json:"syscall,omitempty"`
	TargetPath      string    `json:"targetPath,omitempty"`
	CVEID           string    `json:"cveId"`
	CVESeverity     string    `json:"cveSeverity"`
	CVSS            float32   `json:"cvss"`
	PackageName     string    `json:"packageName"`
	PackageVersion  string    `json:"packageVersion"`
	FixedVersion    string    `json:"fixedVersion,omitempty"`
}

// RuntimeCVESummary is the aggregated response.
type RuntimeCVESummary struct {
	TotalCorrelations   int                     `json:"totalCorrelations"`
	PodsWithBothSignals int                     `json:"podsWithBothSignals"`
	CriticalPairCount   int                     `json:"criticalPairCount"`
	TopRiskyPods        []RuntimeCVEPodSummary  `json:"topRiskyPods"`
	Entries             []RuntimeCVEEntry       `json:"entries,omitempty"`
	Insights            []string                `json:"insights"`
}

// RuntimeCVEPodSummary aggregates per pod.
type RuntimeCVEPodSummary struct {
	PodUID          string `json:"podUid"`
	PodName         string `json:"podName"`
	Namespace       string `json:"namespace"`
	RuntimeEvents   int    `json:"runtimeEvents"`
	CVEMatches      int    `json:"cveMatches"`
	CriticalCVEs    int    `json:"criticalCves"`
	HighSevEvents   int    `json:"highSevEvents"`
	CombinedScore   int    `json:"combinedScore"`
}

type runtimeCVERow struct {
	PodUID         string
	PodName        string
	Namespace      string
	NodeName       string
	EventType      string
	Signal         string
	Mitre          string
	EventSeverity  string
	ObservedAt     time.Time
	Syscall        string
	TargetPath     string
	CVEID          string
	CVESeverity    string
	CVSS           float32
	PackageName    string
	PackageVersion string
	FixedVersion   string
}

// GetRuntimeCVECorrelation returns pods where runtime events AND CVE matches co-occur.
func GetRuntimeCVECorrelation(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()

		namespace := c.Query("namespace")
		podUID := c.Query("pod_uid")
		minSeverity := c.DefaultQuery("min_severity", "MEDIUM")
		limitStr := c.DefaultQuery("limit", "200")
		limit, _ := strconv.Atoi(limitStr)
		if limit <= 0 || limit > 2000 {
			limit = 200
		}
		includeEntries := c.DefaultQuery("entries", "false") == "true"

		rows, err := queryRuntimeCVE(ctx, db, namespace, podUID, minSeverity, limit)
		if err != nil {
			log.Printf("[RuntimeCVE] query error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query runtime-CVE correlation"})
			return
		}

		summary := buildRuntimeCVESummary(rows, includeEntries)
		c.JSON(http.StatusOK, summary)
	}
}

func queryRuntimeCVE(ctx context.Context, db *gorm.DB, namespace, podUID, minSeverity string, limit int) ([]runtimeCVERow, error) {
	sevFilter := sevMinFilter(minSeverity)

	q := db.WithContext(ctx).Raw(`
SELECT
  re.pod_uid,
  COALESCE(re.pod_name, '') AS pod_name,
  re.namespace,
  COALESCE(re.node_name, '') AS node_name,
  COALESCE(re.event_type, '') AS event_type,
  COALESCE(re.signal, '') AS signal,
  COALESCE(re.mitre_technique, '') AS mitre,
  COALESCE(re.severity, '') AS event_severity,
  COALESCE(re.observed_at, re.created_at) AS observed_at,
  COALESCE(re.syscall, '') AS syscall,
  COALESCE(re.target_path, '') AS target_path,
  cm.cve_id,
  cm.severity AS cve_severity,
  cm.cvss,
  cm.package_name,
  cm.package_version,
  COALESCE(cm.fixed_version, '') AS fixed_version
FROM runtime_events re
JOIN sboms s ON s.pod_uid = re.pod_uid AND s.deleted_at IS NULL
JOIN cve_matches cm ON cm.sbom_id = s.id AND cm.deleted_at IS NULL
WHERE ($1 = '' OR re.namespace = $1)
  AND ($2 = '' OR re.pod_uid = $2)
  AND cm.severity IN `+sevFilter+`
ORDER BY cm.cvss DESC, re.observed_at DESC
LIMIT $3
`, namespace, podUID, limit)

	var rows []runtimeCVERow
	if err := q.Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func sevMinFilter(minSev string) string {
	switch minSev {
	case "CRITICAL":
		return "('CRITICAL')"
	case "HIGH":
		return "('CRITICAL','HIGH')"
	case "LOW":
		return "('CRITICAL','HIGH','MEDIUM','LOW')"
	default:
		return "('CRITICAL','HIGH','MEDIUM')"
	}
}

func buildRuntimeCVESummary(rows []runtimeCVERow, includeEntries bool) RuntimeCVESummary {
	podAgg := map[string]*RuntimeCVEPodSummary{}
	critPairs := 0
	var entries []RuntimeCVEEntry

	for _, r := range rows {
		if _, ok := podAgg[r.PodUID]; !ok {
			podAgg[r.PodUID] = &RuntimeCVEPodSummary{
				PodUID:    r.PodUID,
				PodName:   r.PodName,
				Namespace: r.Namespace,
			}
		}
		pa := podAgg[r.PodUID]
		pa.RuntimeEvents++
		pa.CVEMatches++
		if r.CVESeverity == "CRITICAL" {
			pa.CriticalCVEs++
			critPairs++
		}
		if r.EventSeverity == "high" || r.EventSeverity == "critical" {
			pa.HighSevEvents++
		}

		if includeEntries {
			entries = append(entries, RuntimeCVEEntry{
				PodUID:         r.PodUID,
				PodName:        r.PodName,
				Namespace:      r.Namespace,
				NodeName:       r.NodeName,
				EventType:      r.EventType,
				Signal:         r.Signal,
				Mitre:          r.Mitre,
				EventSeverity:  r.EventSeverity,
				EventObserved:  r.ObservedAt,
				Syscall:        r.Syscall,
				TargetPath:     r.TargetPath,
				CVEID:          r.CVEID,
				CVESeverity:    r.CVESeverity,
				CVSS:           r.CVSS,
				PackageName:    r.PackageName,
				PackageVersion: r.PackageVersion,
				FixedVersion:   r.FixedVersion,
			})
		}
	}

	// Score and sort pods
	topPods := make([]RuntimeCVEPodSummary, 0, len(podAgg))
	for _, pa := range podAgg {
		pa.CombinedScore = pa.CriticalCVEs*10 + pa.HighSevEvents*5 + pa.CVEMatches
		topPods = append(topPods, *pa)
	}
	sort.Slice(topPods, func(i, j int) bool {
		return topPods[i].CombinedScore > topPods[j].CombinedScore
	})
	if len(topPods) > 20 {
		topPods = topPods[:20]
	}

	insights := generateRuntimeCVEInsights(topPods, critPairs, len(podAgg))

	return RuntimeCVESummary{
		TotalCorrelations:   len(rows),
		PodsWithBothSignals: len(podAgg),
		CriticalPairCount:   critPairs,
		TopRiskyPods:        topPods,
		Entries:             entries,
		Insights:            insights,
	}
}

func generateRuntimeCVEInsights(pods []RuntimeCVEPodSummary, critPairs, totalPods int) []string {
	var insights []string
	if totalPods == 0 {
		insights = append(insights, "No pods found with both runtime events and CVE matches")
		return insights
	}

	insights = append(insights, "Pods below have both active runtime signals AND known CVE vulnerabilities — prioritize remediation")

	if critPairs > 0 {
		insights = append(insights, "CRITICAL: "+strconv.Itoa(critPairs)+
			" runtime-event × critical-CVE pairs found — immediate action recommended")
	}

	if len(pods) > 0 && pods[0].CombinedScore > 15 {
		insights = append(insights, "Highest-risk pod: "+pods[0].PodName+
			" (ns: "+pods[0].Namespace+") with combined score "+strconv.Itoa(pods[0].CombinedScore))
	}

	return insights
}
