package loader

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/fortuna/core/pkg/cve/cvss"
	"gorm.io/gorm"
)

// CatalogSchemaVersion salts advisory content hashes. Bump it when BuildAdvisory extracts
// something new, so the next load rewrites every advisory instead of skipping unchanged files.
const CatalogSchemaVersion = "vuln-catalog-1"

// ParsedAdvisory is one OSV advisory as stored in the versioned catalog
// (vuln_advisories, vuln_advisory_refs, vuln_affected).
type ParsedAdvisory struct {
	ID             string
	Withdrawn      bool // withdrawn advisories only close the current version
	Source         string
	Kind           string // "vulnerability" or "malware"
	Summary        string
	Details        string
	PublishedAt    *time.Time
	ModifiedAt     *time.Time
	CVSSv3Vector   string
	CVSSv3Score    *float64
	CVSSv4Vector   string
	CVSSv4Score    *float64
	SourceSeverity string // the source's own rating (GitHub, Ubuntu, highest Debian urgency); "" if none
	CWEIDs         []string
	References     string // JSON array
	ContentSHA256  string
	Refs           []AdvisoryRef
	Affected       []AffectedRange
}

// AdvisoryRef links an advisory to another identifier: the CVE it is about, a GHSA, or another
// source's advisory.
type AdvisoryRef struct {
	RefID    string
	Relation string // alias, upstream, related
	RefKind  string // cve, ghsa, advisory
}

// AffectedRange is one affected version interval of a package in one distro release.
type AffectedRange struct {
	Ecosystem      string
	Release        string
	PackageName    string
	RangeType      string // ECOSYSTEM, SEMVER, EXPLICIT
	Introduced     string
	Fixed          string
	LastAffected   string
	VendorSeverity string
}

var cveIDRE = regexp.MustCompile(`^CVE-\d{4}-\d{4,}$`)

// BuildAdvisory converts a parsed OSV advisory; raw is the file content, hashed to detect changes.
func BuildAdvisory(osv *OSVVulnerability, raw []byte) *ParsedAdvisory {
	sum := sha256.New()
	sum.Write([]byte(CatalogSchemaVersion))
	sum.Write([]byte{0})
	sum.Write(raw)

	adv := &ParsedAdvisory{
		ID:            osv.ID,
		Withdrawn:     strings.TrimSpace(osv.Withdrawn) != "",
		Source:        advisorySource(osv.ID),
		Kind:          "vulnerability",
		Summary:       sanitizeUTF8(osv.Summary),
		Details:       sanitizeUTF8(osv.Details),
		PublishedAt:   parseOSVTime(osv.Published),
		ModifiedAt:    parseOSVTime(osv.Modified),
		CWEIDs:        extractCWEIDs(osv.DatabaseSpecific),
		References:    buildReferencesJSON(osv.References),
		ContentSHA256: hex.EncodeToString(sum.Sum(nil)),
	}
	if strings.HasPrefix(osv.ID, "MAL-") {
		adv.Kind = "malware"
	}
	for _, s := range osv.Severity {
		score, version, ok := cvss.BaseScore(s.Score)
		if !ok {
			continue
		}
		switch {
		case strings.HasPrefix(version, "3") && adv.CVSSv3Vector == "":
			adv.CVSSv3Vector, adv.CVSSv3Score = s.Score, &score
		case version == "4.0" && adv.CVSSv4Vector == "":
			adv.CVSSv4Vector, adv.CVSSv4Score = s.Score, &score
		}
	}
	adv.SourceSeverity = AdvisoryRating(osv)
	adv.Refs = advisoryRefs(osv)
	adv.Affected = affectedRanges(osv)
	return adv
}

func parseOSVTime(s string) *time.Time {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	return &t
}

// advisorySource names the database an advisory comes from by its ID prefix.
func advisorySource(id string) string {
	prefix, _, _ := strings.Cut(id, "-")
	switch strings.ToUpper(prefix) {
	case "DEBIAN", "DSA", "DLA", "DTSA":
		return "debian"
	case "RHSA", "RHBA", "RHEA":
		return "redhat"
	case "ALSA", "ALBA", "ALEA":
		return "almalinux"
	case "RLSA", "RLBA", "RLEA", "RXSA":
		return "rocky"
	case "PYSEC":
		return "pypa"
	case "MAL":
		return "ossf-malicious-packages"
	}
	return strings.ToLower(prefix)
}

// advisoryRefs collects the identifiers an advisory points to. One identifier keeps the first
// relation it appears under: alias, then upstream, then related.
func advisoryRefs(osv *OSVVulnerability) []AdvisoryRef {
	var out []AdvisoryRef
	seen := map[string]bool{osv.ID: true}
	add := func(ids []string, relation string) {
		for _, id := range ids {
			id = strings.TrimSpace(id)
			if id == "" || seen[id] || len(id) > 255 {
				continue
			}
			seen[id] = true
			kind := "advisory"
			switch {
			case cveIDRE.MatchString(id):
				kind = "cve"
			case strings.HasPrefix(id, "GHSA-"):
				kind = "ghsa"
			}
			out = append(out, AdvisoryRef{RefID: id, Relation: relation, RefKind: kind})
		}
	}
	add(osv.Aliases, "alias")
	add(osv.Upstream, "upstream")
	add(osv.Related, "related")
	return out
}

// affectedRanges flattens the affected packages into intervals, dropping the duplicates Red Hat
// produces by listing the same package in several streams of one release. When duplicates carry
// different vendor ratings the highest is kept.
func affectedRanges(osv *OSVVulnerability) []AffectedRange {
	var out []AffectedRange
	index := map[AffectedRange]int{}
	add := func(r AffectedRange) {
		key := r
		key.VendorSeverity = ""
		if i, ok := index[key]; ok {
			if cvss.Rank(r.VendorSeverity) > cvss.Rank(out[i].VendorSeverity) {
				out[i].VendorSeverity = r.VendorSeverity
			}
			return
		}
		index[key] = len(out)
		out = append(out, r)
	}
	for i := range osv.Affected {
		affected := &osv.Affected[i]
		if affected.Package.Name == "" || affected.Package.Ecosystem == "" {
			continue
		}
		base := AffectedRange{
			Ecosystem:      normalizeEcosystem(affected.Package.Ecosystem),
			Release:        OSVEcosystemRelease(affected.Package.Ecosystem),
			PackageName:    affected.Package.Name,
			VendorSeverity: AffectedRating(affected),
		}
		versionRanges := 0
		for _, r := range affected.Ranges {
			rt := strings.ToUpper(strings.TrimSpace(r.Type))
			// GIT ranges carry commit hashes, not package versions.
			if rt != "SEMVER" && rt != "ECOSYSTEM" {
				continue
			}
			versionRanges++
			for _, iv := range OSVRangeIntervals(r.Events) {
				ar := base
				ar.RangeType = rt
				ar.Introduced, ar.Fixed, ar.LastAffected = iv.Introduced, iv.Fixed, iv.LastAffected
				add(ar)
			}
		}
		if versionRanges == 0 {
			for _, v := range uniqueVersions(affected.Versions, maxExplicitVersionsPerPackage) {
				ar := base
				ar.RangeType = "EXPLICIT"
				ar.Introduced, ar.LastAffected = v, v
				add(ar)
			}
		}
	}
	return out
}

// VersionedCatalog writes advisories into the versioned catalog as part of one catalog
// generation. Each advisory keeps one current version (valid_to_gen IS NULL); a changed
// advisory closes its current version at this generation and inserts the new one.
type VersionedCatalog struct {
	db  *gorm.DB
	gen uint
}

// CatalogWriteStats counts what one Write or Close did.
type CatalogWriteStats struct {
	Written   int // new versions inserted
	Unchanged int // same content as the current version
	Closed    int // current versions closed (changed, withdrawn or removed)
}

// NewVersionedCatalog returns a writer for generation gen, or nil when the catalog tables do
// not exist (migration 157 not applied).
func NewVersionedCatalog(db *gorm.DB, gen uint) *VersionedCatalog {
	if db == nil || gen == 0 || !db.Migrator().HasTable("vuln_advisories") {
		return nil
	}
	return &VersionedCatalog{db: db, gen: gen}
}

// VersionedCatalogEmpty reports whether the versioned catalog has no advisories yet, so the
// next load must read every source file instead of only the changed ones.
func VersionedCatalogEmpty(ctx context.Context, db *gorm.DB) (bool, error) {
	if !db.Migrator().HasTable("vuln_advisories") {
		return false, nil
	}
	var n int64
	if err := db.WithContext(ctx).Raw(`SELECT COUNT(*) FROM (SELECT 1 FROM vuln_advisories LIMIT 1) t`).Scan(&n).Error; err != nil {
		return false, err
	}
	return n == 0, nil
}

var catalogTables = []string{"vuln_advisories", "vuln_advisory_refs", "vuln_affected"}

// Write stores a batch of advisories. Advisories whose content hash matches their current
// version are skipped.
func (c *VersionedCatalog) Write(ctx context.Context, advs []*ParsedAdvisory) (CatalogWriteStats, error) {
	var stats CatalogWriteStats
	if c == nil || len(advs) == 0 {
		return stats, nil
	}
	// Last one wins if an ID appears twice in a batch.
	byID := make(map[string]*ParsedAdvisory, len(advs))
	ids := make([]string, 0, len(advs))
	for _, a := range advs {
		if a == nil || a.ID == "" {
			continue
		}
		if _, ok := byID[a.ID]; !ok {
			ids = append(ids, a.ID)
		}
		byID[a.ID] = a
	}
	err := c.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		current, err := currentHashes(tx, ids)
		if err != nil {
			return err
		}
		var toClose []string
		var toInsert []*ParsedAdvisory
		for _, id := range ids {
			a := byID[id]
			sha, exists := current[id]
			switch {
			case a.Withdrawn:
				if exists {
					toClose = append(toClose, id)
				}
			case exists && sha == a.ContentSHA256:
				stats.Unchanged++
			default:
				if exists {
					toClose = append(toClose, id)
				}
				toInsert = append(toInsert, a)
			}
		}
		closed, err := closeAdvisories(tx, c.gen, toClose)
		if err != nil {
			return err
		}
		stats.Closed = closed
		if err := insertAdvisories(tx, c.gen, toInsert); err != nil {
			return err
		}
		stats.Written = len(toInsert)
		return nil
	})
	return stats, err
}

// Close closes the current version of advisories removed from the source.
func (c *VersionedCatalog) Close(ctx context.Context, ids []string) (int, error) {
	if c == nil || len(ids) == 0 {
		return 0, nil
	}
	closed := 0
	err := c.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for start := 0; start < len(ids); start += catalogIDChunk {
			n, err := closeAdvisories(tx, c.gen, ids[start:min(start+catalogIDChunk, len(ids))])
			if err != nil {
				return err
			}
			closed += n
		}
		return nil
	})
	return closed, err
}

// CloseMissing closes every current advisory loaded by an earlier generation whose ID is not
// in seen. A full load calls it so advisories gone from the source stop matching.
func (c *VersionedCatalog) CloseMissing(ctx context.Context, seen []string) (int, error) {
	if c == nil {
		return 0, nil
	}
	var missing []string
	err := c.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`CREATE TEMP TABLE IF NOT EXISTS fortuna_seen_advisories (id VARCHAR(255) PRIMARY KEY) ON COMMIT DROP`).Error; err != nil {
			return err
		}
		for start := 0; start < len(seen); start += catalogIDChunk {
			chunk := seen[start:min(start+catalogIDChunk, len(seen))]
			if err := tx.Exec(`INSERT INTO fortuna_seen_advisories (id) SELECT DISTINCT unnest(?::text[]) ON CONFLICT DO NOTHING`, pgTextArray(chunk)).Error; err != nil {
				return fmt.Errorf("record seen advisories: %w", err)
			}
		}
		return tx.Raw(`SELECT advisory_id FROM vuln_advisories
WHERE valid_to_gen IS NULL AND valid_from_gen < ?
  AND advisory_id NOT IN (SELECT id FROM fortuna_seen_advisories)`, c.gen).Scan(&missing).Error
	})
	if err != nil {
		return 0, err
	}
	return c.Close(ctx, missing)
}

// RollbackCatalogGeneration undoes what generation gen wrote: its new versions are deleted and
// the versions it closed are current again.
func RollbackCatalogGeneration(ctx context.Context, db *gorm.DB, gen uint) error {
	if db == nil || gen == 0 || !db.Migrator().HasTable("vuln_advisories") {
		return nil
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return rollbackGenerations(tx, `SELECT ?::bigint`, gen)
	})
}

// RollbackUnfinishedCatalogGenerations undoes the catalog writes of cve generations that never
// became active (a load that failed or was killed) and marks the killed ones failed. It runs
// before every load so a crashed load cannot leave half-written versions behind.
func RollbackUnfinishedCatalogGenerations(ctx context.Context, db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable("vuln_advisories") || !db.Migrator().HasTable("catalog_generations") {
		return nil
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := rollbackGenerations(tx, `SELECT id FROM catalog_generations WHERE catalog_type = 'cve' AND status IN ('running', 'failed')`); err != nil {
			return err
		}
		return tx.Exec(`UPDATE catalog_generations SET status = 'failed', error_summary = 'load did not finish',
			validation_status = 'failed', updated_at = NOW()
			WHERE catalog_type = 'cve' AND status = 'running'`).Error
	})
}

// rollbackGenerations deletes the versions inserted by the generations genQuery selects and
// reopens the versions they closed. Deleting first keeps one current version per advisory.
func rollbackGenerations(tx *gorm.DB, genQuery string, args ...interface{}) error {
	for _, t := range catalogTables {
		if err := tx.Exec(`DELETE FROM `+t+` WHERE valid_from_gen IN (`+genQuery+`)`, args...).Error; err != nil {
			return fmt.Errorf("roll back %s: %w", t, err)
		}
	}
	for _, t := range catalogTables {
		if err := tx.Exec(`UPDATE `+t+` SET valid_to_gen = NULL WHERE valid_to_gen IN (`+genQuery+`)`, args...).Error; err != nil {
			return fmt.Errorf("reopen %s: %w", t, err)
		}
	}
	return nil
}

// PruneVersionedCatalog deletes the versions no kept generation can read: those closed at or
// before oldestKept, the oldest generation still kept.
func PruneVersionedCatalog(ctx context.Context, db *gorm.DB, oldestKept uint) (int64, error) {
	if db == nil || oldestKept == 0 || !db.Migrator().HasTable("vuln_advisories") {
		return 0, nil
	}
	var deleted int64
	for _, t := range catalogTables {
		res := db.WithContext(ctx).Exec(`DELETE FROM `+t+` WHERE valid_to_gen IS NOT NULL AND valid_to_gen <= ?`, oldestKept)
		if res.Error != nil {
			return deleted, fmt.Errorf("prune %s: %w", t, res.Error)
		}
		deleted += res.RowsAffected
	}
	return deleted, nil
}

// catalogLoadLockKey is the PostgreSQL advisory lock held for the whole of a catalog load.
const catalogLoadLockKey = 0x46435645 // "FCVE"

// LockCatalogLoad takes the catalog load lock on a dedicated connection and returns the
// function that releases it. It fails at once when another load holds the lock; the lock is
// also released if the process dies, because it belongs to the connection.
func LockCatalogLoad(ctx context.Context, db *gorm.DB) (func(), error) {
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("take catalog load lock: %w", err)
	}
	var ok bool
	if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, catalogLoadLockKey).Scan(&ok); err != nil {
		conn.Close()
		return nil, fmt.Errorf("take catalog load lock: %w", err)
	}
	if !ok {
		conn.Close()
		return nil, fmt.Errorf("another vulnerability catalog load is running")
	}
	return func() {
		_, _ = conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, catalogLoadLockKey)
		conn.Close()
	}, nil
}

// catalogIDChunk bounds the IDs sent in one statement.
const catalogIDChunk = 1000

func currentHashes(tx *gorm.DB, ids []string) (map[string]string, error) {
	out := make(map[string]string, len(ids))
	for start := 0; start < len(ids); start += catalogIDChunk {
		var rows []struct {
			AdvisoryID    string
			ContentSHA256 string `gorm:"column:content_sha256"`
		}
		if err := tx.Raw(`SELECT advisory_id, content_sha256 FROM vuln_advisories
WHERE valid_to_gen IS NULL AND advisory_id = ANY(?::text[])`, pgTextArray(ids[start:min(start+catalogIDChunk, len(ids))])).Scan(&rows).Error; err != nil {
			return nil, fmt.Errorf("read current advisories: %w", err)
		}
		for _, r := range rows {
			out[r.AdvisoryID] = r.ContentSHA256
		}
	}
	return out, nil
}

// closeAdvisories ends the current version of ids at generation gen. A version inserted by gen
// itself (the same advisory loaded twice in one run) is deleted instead.
func closeAdvisories(tx *gorm.DB, gen uint, ids []string) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	arr := pgTextArray(ids)
	closed := 0
	for _, t := range catalogTables {
		if err := tx.Exec(`DELETE FROM `+t+` WHERE advisory_id = ANY(?::text[]) AND valid_from_gen = ?`, arr, gen).Error; err != nil {
			return 0, fmt.Errorf("replace %s: %w", t, err)
		}
		res := tx.Exec(`UPDATE `+t+` SET valid_to_gen = ? WHERE advisory_id = ANY(?::text[]) AND valid_to_gen IS NULL`, gen, arr)
		if res.Error != nil {
			return 0, fmt.Errorf("close %s: %w", t, res.Error)
		}
		if t == "vuln_advisories" {
			closed = int(res.RowsAffected)
		}
	}
	return closed, nil
}

func insertAdvisories(tx *gorm.DB, gen uint, advs []*ParsedAdvisory) error {
	if len(advs) == 0 {
		return nil
	}
	const advCols = 16
	if err := insertRows(tx, `INSERT INTO vuln_advisories (advisory_id, valid_from_gen, source, kind, summary, details,
		published_at, modified_at, cvss_v3_vector, cvss_v3_score, cvss_v4_vector, cvss_v4_score,
		source_severity, cwe_ids, reference_urls, content_sha256) VALUES `,
		"(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?::text[], ?::jsonb, ?)", advCols, len(advs), func(i int) []interface{} {
			a := advs[i]
			return []interface{}{a.ID, gen, a.Source, a.Kind, a.Summary, a.Details, a.PublishedAt, a.ModifiedAt,
				a.CVSSv3Vector, a.CVSSv3Score, a.CVSSv4Vector, a.CVSSv4Score, a.SourceSeverity,
				pgTextArray(a.CWEIDs), a.References, a.ContentSHA256}
		}); err != nil {
		return fmt.Errorf("insert vuln_advisories: %w", err)
	}

	type ref struct {
		advisory string
		AdvisoryRef
	}
	var refs []ref
	type aff struct {
		advisory string
		AffectedRange
	}
	var affs []aff
	for _, a := range advs {
		for _, r := range a.Refs {
			refs = append(refs, ref{a.ID, r})
		}
		for _, r := range a.Affected {
			affs = append(affs, aff{a.ID, r})
		}
	}
	if err := insertRows(tx, `INSERT INTO vuln_advisory_refs (advisory_id, ref_id, relation, ref_kind, valid_from_gen) VALUES `,
		"(?, ?, ?, ?, ?)", 5, len(refs), func(i int) []interface{} {
			r := refs[i]
			return []interface{}{r.advisory, r.RefID, r.Relation, r.RefKind, gen}
		}); err != nil {
		return fmt.Errorf("insert vuln_advisory_refs: %w", err)
	}
	if err := insertRows(tx, `INSERT INTO vuln_affected (advisory_id, ecosystem, release, package_name, range_type,
		introduced, fixed, last_affected, vendor_severity, valid_from_gen) VALUES `,
		"(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", 10, len(affs), func(i int) []interface{} {
			r := affs[i]
			return []interface{}{r.advisory, r.Ecosystem, r.Release, r.PackageName, r.RangeType,
				r.Introduced, r.Fixed, r.LastAffected, r.VendorSeverity, gen}
		}); err != nil {
		return fmt.Errorf("insert vuln_affected: %w", err)
	}
	return nil
}

// insertRows runs a multi-row INSERT in chunks that stay under PostgreSQL's 65535 parameters.
func insertRows(tx *gorm.DB, head, rowTemplate string, cols, n int, row func(int) []interface{}) error {
	chunk := 60000 / cols
	for start := 0; start < n; start += chunk {
		end := min(start+chunk, n)
		placeholders := make([]string, 0, end-start)
		args := make([]interface{}, 0, (end-start)*cols)
		for i := start; i < end; i++ {
			placeholders = append(placeholders, rowTemplate)
			args = append(args, row(i)...)
		}
		if err := tx.Exec(head+strings.Join(placeholders, ","), args...).Error; err != nil {
			return err
		}
	}
	return nil
}

// pgTextArray renders a PostgreSQL text[] literal.
func pgTextArray(values []string) string {
	var b strings.Builder
	b.WriteByte('{')
	for i, v := range values {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('"')
		for _, r := range v {
			if r == '"' || r == '\\' {
				b.WriteByte('\\')
			}
			b.WriteRune(r)
		}
		b.WriteByte('"')
	}
	b.WriteByte('}')
	return b.String()
}
