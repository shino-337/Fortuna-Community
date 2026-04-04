package nvd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

const (
	nvdAPIBaseURL       = "https://services.nvd.nist.gov/rest/json/cves/2.0"
	defaultPageSize     = 2000
	rateLimitNoKey      = 6 * time.Second
	rateLimitWithKey    = 600 * time.Millisecond
	maxRetries          = 3
	mirrorName          = "nvd"
	syncWindowMaxDays   = 120
)

// Syncer downloads NVD CVE data via API 2.0 and upserts into cves + package_vulnerabilities tables.
type Syncer struct {
	db       *gorm.DB
	apiKey   string
	baseURL  string
	client   *http.Client
	logger   *log.Logger
	pageSize int
}

// NewSyncer creates a new NVD mirror syncer.
func NewSyncer(db *gorm.DB, apiKey string) *Syncer {
	return &Syncer{
		db:       db,
		apiKey:   strings.TrimSpace(apiKey),
		baseURL:  nvdAPIBaseURL,
		client:   &http.Client{Timeout: 60 * time.Second},
		logger:   log.New(log.Writer(), "[NVDMirror] ", log.LstdFlags),
		pageSize: defaultPageSize,
	}
}

// Sync performs an incremental NVD mirror sync.
// On first run it downloads all CVEs modified in the last syncWindowMaxDays days.
// Subsequent runs use the stored lastModified timestamp for delta sync.
func (s *Syncer) Sync(ctx context.Context) error {
	lastMod := s.loadLastSyncTime(ctx)
	now := time.Now().UTC()

	if lastMod.IsZero() {
		lastMod = now.AddDate(0, 0, -syncWindowMaxDays)
		s.logger.Printf("Initial sync: fetching CVEs modified since %s", lastMod.Format(time.RFC3339))
	} else {
		s.logger.Printf("Incremental sync: fetching CVEs modified since %s", lastMod.Format(time.RFC3339))
	}

	endDate := now.Add(-1 * time.Minute)

	var totalIngested int64
	windowStart := lastMod
	for windowStart.Before(endDate) {
		windowEnd := windowStart.AddDate(0, 0, syncWindowMaxDays)
		if windowEnd.After(endDate) {
			windowEnd = endDate
		}

		n, err := s.syncWindow(ctx, windowStart, windowEnd)
		if err != nil {
			return fmt.Errorf("sync window %s–%s: %w",
				windowStart.Format(time.RFC3339), windowEnd.Format(time.RFC3339), err)
		}
		totalIngested += n
		windowStart = windowEnd
	}

	s.saveLastSyncTime(ctx, endDate)
	s.logger.Printf("Sync complete: %d CVEs ingested/updated", totalIngested)
	return nil
}

// syncWindow fetches all CVEs modified within [start, end] and upserts them.
func (s *Syncer) syncWindow(ctx context.Context, start, end time.Time) (int64, error) {
	startIdx := 0
	var total int64

	for {
		resp, err := s.fetchPage(ctx, start, end, startIdx)
		if err != nil {
			return total, err
		}

		if len(resp.Vulnerabilities) == 0 {
			break
		}

		ingested, err := s.ingestPage(ctx, resp.Vulnerabilities)
		if err != nil {
			return total, fmt.Errorf("ingest page at offset %d: %w", startIdx, err)
		}
		total += ingested

		s.logger.Printf("  page offset=%d, got=%d, total_results=%d",
			startIdx, len(resp.Vulnerabilities), resp.TotalResults)

		startIdx += len(resp.Vulnerabilities)
		if startIdx >= resp.TotalResults {
			break
		}
	}

	return total, nil
}

// fetchPage calls NVD API 2.0 with pagination and rate-limit handling.
func (s *Syncer) fetchPage(ctx context.Context, start, end time.Time, startIndex int) (*APIResponse, error) {
	url := fmt.Sprintf("%s?lastModStartDate=%s&lastModEndDate=%s&startIndex=%d&resultsPerPage=%d",
		s.baseURL,
		start.Format("2006-01-02T15:04:05.000"),
		end.Format("2006-01-02T15:04:05.000"),
		startIndex,
		s.pageSize,
	)

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt*attempt) * 30 * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		if s.apiKey != "" {
			req.Header.Set("apiKey", s.apiKey)
		}

		resp, err := s.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("HTTP request: %w", err)
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			_ = resp.Body.Close()
			wait := s.rateLimit()
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if sec, err := strconv.Atoi(ra); err == nil && sec > 0 {
					wait = time.Duration(sec) * time.Second
				}
			}
			s.logger.Printf("Rate limited (429), waiting %v", wait)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wait):
			}
			lastErr = fmt.Errorf("rate limited (429)")
			continue
		}

		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("NVD API status %d", resp.StatusCode)
			continue
		}

		var apiResp APIResponse
		if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("decode response: %w", err)
		}
		_ = resp.Body.Close()

		s.respectRateLimit(ctx)
		return &apiResp, nil
	}

	return nil, fmt.Errorf("NVD API failed after %d retries: %w", maxRetries, lastErr)
}

func (s *Syncer) rateLimit() time.Duration {
	if s.apiKey != "" {
		return rateLimitWithKey
	}
	return rateLimitNoKey
}

func (s *Syncer) respectRateLimit(ctx context.Context) {
	wait := s.rateLimit()
	select {
	case <-ctx.Done():
	case <-time.After(wait):
	}
}

// ingestPage converts NVD vulnerability wrappers into DB rows.
func (s *Syncer) ingestPage(ctx context.Context, vulns []VulnWrapper) (int64, error) {
	if len(vulns) == 0 {
		return 0, nil
	}

	var count int64

	for _, vw := range vulns {
		item := vw.CVE
		if item.ID == "" {
			continue
		}

		cveRow, err := s.convertCVE(item)
		if err != nil {
			s.logger.Printf("skip %s: %v", item.ID, err)
			continue
		}

		if err := s.upsertCVE(ctx, cveRow); err != nil {
			return count, fmt.Errorf("upsert CVE %s: %w", item.ID, err)
		}

		pvRows := s.extractPackageVulns(item)
		for _, pv := range pvRows {
			if err := s.upsertPackageVuln(ctx, pv); err != nil {
				s.logger.Printf("skip PV for %s/%s: %v", pv.CVEID, pv.PackageName, err)
			}
		}

		count++
	}

	return count, nil
}

func (s *Syncer) convertCVE(item CVEItem) (*models.CVE, error) {
	desc := extractEnglish(item.Descriptions)
	score, vector, version, severity := extractCVSS(item.Metrics)
	cweIDs := extractCWEIDs(item.Weaknesses)
	refs := buildRefsJSON(item.References)

	published := parseTime(item.Published)
	modified := parseTime(item.LastModified)

	title := desc
	if len(title) > 200 {
		title = title[:200] + "..."
	}

	return &models.CVE{
		CVEID:            item.ID,
		CVSSScore:        score,
		CVSSVector:       vector,
		CVSSVersion:      version,
		Severity:         severity,
		Title:            title,
		Description:      desc,
		Source:           "nvd",
		References:       refs,
		CWEIDs:          cweIDs,
		PublishedDate:    published,
		LastModifiedDate: modified,
	}, nil
}

func (s *Syncer) upsertCVE(ctx context.Context, row *models.CVE) error {
	return s.db.WithContext(ctx).
		Where("cve_id = ?", row.CVEID).
		Assign(models.CVE{
			CVSSScore:        row.CVSSScore,
			CVSSVector:       row.CVSSVector,
			CVSSVersion:      row.CVSSVersion,
			Severity:         row.Severity,
			Title:            row.Title,
			Description:      row.Description,
			Source:           row.Source,
			References:       row.References,
			CWEIDs:          row.CWEIDs,
			PublishedDate:    row.PublishedDate,
			LastModifiedDate: row.LastModifiedDate,
		}).
		FirstOrCreate(row).Error
}

// extractPackageVulns derives package_vulnerabilities rows from NVD configurations (CPE matches).
func (s *Syncer) extractPackageVulns(item CVEItem) []*models.PackageVulnerability {
	var result []*models.PackageVulnerability
	seen := make(map[string]bool)

	for _, cfg := range item.Configurations {
		for _, node := range cfg.Nodes {
			for _, m := range node.CPEMatch {
				if !m.Vulnerable {
					continue
				}
				vendor, product, ecosystem := parseCPE(m.Criteria)
				if product == "" {
					continue
				}

				key := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s",
					item.ID, product, ecosystem,
					m.VersionStartIncluding, m.VersionStartExcluding,
					m.VersionEndIncluding, m.VersionEndExcluding)
				if seen[key] {
					continue
				}
				seen[key] = true

				fixed := m.VersionEndExcluding

				result = append(result, &models.PackageVulnerability{
					CVEID:                 item.ID,
					PackageName:           product,
					Ecosystem:             ecosystem,
					Vendor:                vendor,
					Product:               product,
					VersionStartIncluding: m.VersionStartIncluding,
					VersionStartExcluding: m.VersionStartExcluding,
					VersionEndIncluding:   m.VersionEndIncluding,
					VersionEndExcluding:   m.VersionEndExcluding,
					FixedVersion:          fixed,
				})
			}
		}
	}
	return result
}

func (s *Syncer) upsertPackageVuln(ctx context.Context, pv *models.PackageVulnerability) error {
	return s.db.WithContext(ctx).
		Where("cve_id = ? AND package_name = ? AND ecosystem = ? AND COALESCE(version_start_including,'') = ? AND COALESCE(version_end_excluding,'') = ?",
			pv.CVEID, pv.PackageName, pv.Ecosystem,
			pv.VersionStartIncluding, pv.VersionEndExcluding).
		Assign(models.PackageVulnerability{
			Vendor:                pv.Vendor,
			Product:               pv.Product,
			VersionStartIncluding: pv.VersionStartIncluding,
			VersionStartExcluding: pv.VersionStartExcluding,
			VersionEndIncluding:   pv.VersionEndIncluding,
			VersionEndExcluding:   pv.VersionEndExcluding,
			FixedVersion:          pv.FixedVersion,
		}).
		FirstOrCreate(pv).Error
}

func (s *Syncer) loadLastSyncTime(ctx context.Context) time.Time {
	if s.db == nil {
		return time.Time{}
	}
	if !s.db.Migrator().HasTable("mirror_state") {
		return time.Time{}
	}
	var row models.MirrorState
	if err := s.db.WithContext(ctx).Where("name = ?", mirrorName).First(&row).Error; err != nil {
		return time.Time{}
	}
	return row.UpdatedAt
}

func (s *Syncer) saveLastSyncTime(ctx context.Context, t time.Time) {
	if s.db == nil {
		return
	}
	if !s.db.Migrator().HasTable("mirror_state") {
		return
	}
	var row models.MirrorState
	err := s.db.WithContext(ctx).Where("name = ?", mirrorName).First(&row).Error
	if err != nil {
		s.db.WithContext(ctx).Create(&models.MirrorState{Name: mirrorName, Version: 1})
		return
	}
	row.Version++
	s.db.WithContext(ctx).Save(&row)
}

// --- helpers ---

func extractEnglish(descs []LangString) string {
	for _, d := range descs {
		if d.Lang == "en" {
			return strings.ToValidUTF8(d.Value, "\uFFFD")
		}
	}
	if len(descs) > 0 {
		return strings.ToValidUTF8(descs[0].Value, "\uFFFD")
	}
	return ""
}

func extractCVSS(m *Metrics) (score float64, vector, version, severity string) {
	if m == nil {
		return 0, "", "", "MEDIUM"
	}

	var metric *CVSSMetric
	if len(m.CvssMetricV31) > 0 {
		metric = &m.CvssMetricV31[0]
	} else if len(m.CvssMetricV30) > 0 {
		metric = &m.CvssMetricV30[0]
	} else if len(m.CvssMetricV2) > 0 {
		metric = &m.CvssMetricV2[0]
	}

	if metric == nil {
		return 0, "", "", "MEDIUM"
	}

	score = metric.CVSSData.BaseScore
	vector = metric.CVSSData.VectorString
	version = metric.CVSSData.Version
	severity = strings.ToUpper(metric.CVSSData.BaseSeverity)
	if severity == "" {
		severity = scoreToSeverity(score)
	}
	return
}

func scoreToSeverity(s float64) string {
	switch {
	case s >= 9.0:
		return "CRITICAL"
	case s >= 7.0:
		return "HIGH"
	case s >= 4.0:
		return "MEDIUM"
	default:
		return "LOW"
	}
}

func extractCWEIDs(ws []Weakness) pq.StringArray {
	if len(ws) == 0 {
		return pq.StringArray{}
	}
	var ids []string
	seen := make(map[string]bool)
	for _, w := range ws {
		for _, d := range w.Description {
			id := strings.TrimSpace(d.Value)
			if id != "" && !seen[id] && strings.HasPrefix(id, "CWE-") {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	return pq.StringArray(ids)
}

func buildRefsJSON(refs []Reference) string {
	if len(refs) == 0 {
		return "[]"
	}
	type r struct {
		URL  string   `json:"url"`
		Tags []string `json:"tags,omitempty"`
	}
	out := make([]r, 0, len(refs))
	for _, ref := range refs {
		if ref.URL != "" {
			out = append(out, r{URL: ref.URL, Tags: ref.Tags})
		}
	}
	b, _ := json.Marshal(out)
	return string(b)
}

// parseCPE extracts vendor, product and a heuristic ecosystem from a CPE 2.3 URI.
// cpe:2.3:a:vendor:product:version:...
func parseCPE(criteria string) (vendor, product, ecosystem string) {
	parts := strings.Split(criteria, ":")
	if len(parts) < 5 {
		return "", "", ""
	}
	vendor = parts[3]
	product = parts[4]

	ecosystem = guessCPEEcosystem(vendor, product)
	return
}

func guessCPEEcosystem(vendor, product string) string {
	v := strings.ToLower(vendor)
	p := strings.ToLower(product)

	switch {
	case v == "debian" || strings.Contains(p, "debian"):
		return "debian"
	case v == "ubuntu" || v == "canonical":
		return "ubuntu"
	case v == "alpinelinux" || v == "alpine":
		return "alpine"
	case v == "redhat" || v == "centos" || v == "fedoraproject":
		return "rpm"
	case v == "golang" || strings.HasPrefix(p, "go"):
		return "go"
	case v == "python" || v == "pypi" || v == "djangoproject":
		return "pypi"
	case v == "npmjs" || v == "nodejs":
		return "npm"
	case strings.Contains(v, "apache") || v == "maven":
		return "maven"
	case v == "microsoft" && strings.Contains(p, "nuget"):
		return "nuget"
	case v == "rust-lang" || v == "crates":
		return "cargo"
	case v == "ruby-lang" || v == "rubygems":
		return "rubygems"
	default:
		return "nvd"
	}
}

var timeLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05.000Z07:00",
	"2006-01-02T15:04:05.000",
	"2006-01-02T15:04:05Z07:00",
	"2006-01-02T15:04:05",
}

func parseTime(s string) *time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	for _, layout := range timeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return &t
		}
	}
	return nil
}
