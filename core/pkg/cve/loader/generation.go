package loader

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

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
