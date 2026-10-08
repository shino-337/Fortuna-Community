package loader

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

// packageVulnColumns are copied when an incremental load carries rows into a new generation.
const packageVulnColumns = `cve_id, package_name, package_type, ecosystem, ecosystem_release, affected_range,
	version_start_including, version_start_excluding, version_end_including, version_end_excluding,
	fixed_version, fixed_in_versions, vendor, product`

// ActiveCVEGeneration returns the generation the matcher reads (latest activated "active" cve
// generation), or nil when there is none.
func ActiveCVEGeneration(ctx context.Context, db *gorm.DB) (*models.CatalogGeneration, error) {
	var gen models.CatalogGeneration
	err := db.WithContext(ctx).
		Where("catalog_type = ? AND status = ?", "cve", "active").
		Order("activated_at DESC, id DESC").
		Limit(1).
		Find(&gen).Error
	if err != nil {
		return nil, err
	}
	if gen.ID == 0 {
		return nil, nil
	}
	return &gen, nil
}

// CarryForwardPackageVulns copies the package_vulnerabilities rows of generation from into
// generation to, except rows of the advisories in excludedIDs (reloaded or removed in this run).
//
// The matcher reads only the active generation, so an incremental load that holds just the
// changed advisories must carry the unchanged ones forward before it is activated; otherwise
// activating it hides every other advisory.
func CarryForwardPackageVulns(ctx context.Context, db *gorm.DB, from, to uint, excludedIDs []string) (int64, error) {
	if from == 0 || to == 0 || from == to {
		return 0, nil
	}
	var copied int64
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`CREATE TEMP TABLE IF NOT EXISTS fortuna_excluded_advisories (id VARCHAR(255) PRIMARY KEY)`).Error; err != nil {
			return fmt.Errorf("create excluded advisories table: %w", err)
		}
		if err := tx.Exec(`DELETE FROM fortuna_excluded_advisories`).Error; err != nil {
			return err
		}
		const chunk = 1000
		seen := make(map[string]bool, len(excludedIDs))
		batch := make([]string, 0, chunk)
		flush := func() error {
			if len(batch) == 0 {
				return nil
			}
			placeholders := strings.TrimSuffix(strings.Repeat("(?),", len(batch)), ",")
			args := make([]interface{}, len(batch))
			for i, id := range batch {
				args[i] = id
			}
			batch = batch[:0]
			return tx.Exec(`INSERT INTO fortuna_excluded_advisories (id) VALUES `+placeholders, args...).Error
		}
		for _, id := range excludedIDs {
			id = strings.TrimSpace(id)
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			batch = append(batch, id)
			if len(batch) == chunk {
				if err := flush(); err != nil {
					return fmt.Errorf("record excluded advisories: %w", err)
				}
			}
		}
		if err := flush(); err != nil {
			return fmt.Errorf("record excluded advisories: %w", err)
		}
		res := tx.Exec(`INSERT INTO package_vulnerabilities (`+packageVulnColumns+`, catalog_generation_id, created_at, updated_at)
SELECT `+packageVulnColumns+`, ?, created_at, CURRENT_TIMESTAMP
FROM package_vulnerabilities
WHERE catalog_generation_id = ? AND deleted_at IS NULL
  AND cve_id NOT IN (SELECT id FROM fortuna_excluded_advisories)`, to, from)
		if res.Error != nil {
			return fmt.Errorf("carry forward package vulnerabilities: %w", res.Error)
		}
		copied = res.RowsAffected
		return tx.Exec(`DROP TABLE fortuna_excluded_advisories`).Error
	})
	return copied, err
}

// PruneCVEGenerations keeps the newest keep active cve generations and retires the older ones,
// deleting their package_vulnerabilities rows and the versioned catalog rows none of the kept
// generations can read. Rows of generation 0 (loaded before generations existed) are left alone.
func PruneCVEGenerations(ctx context.Context, db *gorm.DB, keep int) (retired []uint, deleted int64, err error) {
	if keep < 1 {
		keep = 1
	}
	var gens []models.CatalogGeneration
	if err := db.WithContext(ctx).
		Where("catalog_type = ? AND status = ?", "cve", "active").
		Order("activated_at DESC, id DESC").
		Find(&gens).Error; err != nil {
		return nil, 0, err
	}
	if len(gens) == 0 {
		return nil, 0, nil
	}
	// Versions closed at or before the oldest kept generation are invisible to every kept one.
	// Generations are ordered by activation, so a re-activated older one may be kept.
	oldestKept := gens[0].ID
	for _, g := range gens[:min(keep, len(gens))] {
		if g.ID < oldestKept {
			oldestKept = g.ID
		}
	}
	pruned, err := PruneVersionedCatalog(ctx, db, oldestKept)
	if err != nil {
		return nil, 0, err
	}
	deleted += pruned
	if len(gens) <= keep {
		return nil, deleted, nil
	}
	for _, g := range gens[keep:] {
		res := db.WithContext(ctx).Unscoped().
			Where("catalog_generation_id = ?", g.ID).
			Delete(&models.PackageVulnerability{})
		if res.Error != nil {
			return retired, deleted, fmt.Errorf("delete rows of generation %d: %w", g.ID, res.Error)
		}
		deleted += res.RowsAffected
		if err := db.WithContext(ctx).Model(&models.CatalogGeneration{}).
			Where("id = ?", g.ID).
			Update("status", "retired").Error; err != nil {
			return retired, deleted, fmt.Errorf("retire generation %d: %w", g.ID, err)
		}
		retired = append(retired, g.ID)
	}
	return retired, deleted, nil
}

// RemovedCVEIDs returns the advisory IDs whose source file under this tracker's directory was
// tracked before and no longer exists. Files tracked from other directories are not counted,
// so a catalog loaded from another source is not treated as removed.
func (t *IncrementalTracker) RemovedCVEIDs(ctx context.Context) ([]string, error) {
	var metas []FileMetadata
	if err := t.db.WithContext(ctx).Select("file_path", "cve_id").Find(&metas).Error; err != nil {
		return nil, err
	}
	current, err := t.scanDirectory()
	if err != nil {
		return nil, err
	}
	dir := filepath.Clean(t.sourceDir) + string(filepath.Separator)
	var out []string
	for _, m := range metas {
		if !strings.HasPrefix(filepath.Clean(m.FilePath), dir) {
			continue
		}
		if _, ok := current[m.FilePath]; ok {
			continue
		}
		out = append(out, m.CVEID)
	}
	return out, nil
}

// CVEIDsOfFiles returns the advisory IDs of source files (<ID>.json).
func CVEIDsOfFiles(files []string) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, extractCVEIDFromPath(f))
	}
	return out
}
