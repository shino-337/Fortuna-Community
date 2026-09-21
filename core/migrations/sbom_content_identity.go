package migrations

import (
	"fmt"
	"strings"

	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/sbomcontent"
	"gorm.io/gorm"
)

const sbomOwnedPredicate = "deleted_at IS NULL AND cluster_id <> '' AND pod_uid <> '' AND container_name <> '' AND image_digest <> ''"

// EnsureSBOMContentIdentity preserves ambiguous legacy rows without guessing an
// owner, backfills immutable content, and fails startup on conflicting active
// observations. No historical evidence or association is deleted or reassigned.
func EnsureSBOMContentIdentity(db *gorm.DB) error {
	if !db.Migrator().HasTable(&models.SBOM{}) {
		return fmt.Errorf("SBOM table is missing")
	}
	// Avoid re-running generic AutoMigrate over a populated immutable table;
	// explicit invariants make startup reruns independent of driver introspection.
	if !db.Migrator().HasTable(&models.SBOMImageContent{}) {
		if err := db.Migrator().CreateTable(&models.SBOMImageContent{}); err != nil {
			return err
		}
	}
	if err := db.Exec("SELECT id,content_hash,payload,created_at FROM sbom_image_contents LIMIT 0").Error; err != nil {
		return err
	}
	if err := ensureIndex(db, "idx_sbom_image_contents_content_hash", "sbom_image_contents", "content_hash", true); err != nil {
		return err
	}
	if !db.Migrator().HasColumn(&models.SBOM{}, "ContentID") {
		if err := db.Migrator().AddColumn(&models.SBOM{}, "ContentID"); err != nil {
			return err
		}
	}
	if !db.Migrator().HasConstraint(&models.SBOM{}, "ContentRef") {
		if err := db.Migrator().CreateConstraint(&models.SBOM{}, "ContentRef"); err != nil {
			return err
		}
	}
	var duplicates int64
	if err := db.Raw("SELECT COUNT(*) FROM (SELECT 1 FROM sboms WHERE " + sbomOwnedPredicate + " GROUP BY cluster_id,pod_uid,container_name,image_digest HAVING COUNT(*) > 1) duplicates").Scan(&duplicates).Error; err != nil {
		return err
	}
	if duplicates > 0 {
		return fmt.Errorf("%d duplicate active SBOM ownership groups require explicit reconciliation; no rows were reassigned", duplicates)
	}
	const indexName = "idx_sbom_active_workload_identity"
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS " + indexName + " ON sboms(cluster_id,pod_uid,container_name,image_digest) WHERE " + sbomOwnedPredicate).Error; err != nil {
		return err
	}
	if db.Dialector.Name() == "postgres" {
		def, exists, err := postgresIndexDefinitionForName(db, indexName)
		if err != nil {
			return err
		}
		var predicate string
		if err := db.Raw("SELECT pg_get_expr(i.indpred,i.indrelid) FROM pg_index i JOIN pg_class c ON c.oid=i.indexrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE c.relname=? AND n.nspname=current_schema()", indexName).Scan(&predicate).Error; err != nil {
			return err
		}
		normalize := func(s string) string {
			return strings.ToLower(strings.NewReplacer("(", "", ")", "", " ", "", "::text", "", "\n", "").Replace(s))
		}
		if !exists || !def.Valid || !def.Unique || def.Table != "sboms" || def.Columns != "cluster_id,pod_uid,container_name,image_digest" || normalize(predicate) != normalize(sbomOwnedPredicate) {
			return fmt.Errorf("SBOM ownership index has an invalid definition")
		}
		if err := db.Exec(`CREATE OR REPLACE FUNCTION fortuna_sbom_content_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'SBOM image content is immutable'; END $$`).Error; err != nil {
			return err
		}
		if err := db.Exec("DROP TRIGGER IF EXISTS sbom_content_immutable ON sbom_image_contents").Error; err != nil {
			return err
		}
		if err := db.Exec("CREATE TRIGGER sbom_content_immutable BEFORE UPDATE ON sbom_image_contents FOR EACH ROW EXECUTE FUNCTION fortuna_sbom_content_immutable()").Error; err != nil {
			return err
		}
		if err := db.Exec(`CREATE OR REPLACE FUNCTION fortuna_sbom_owner_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP = 'INSERT' AND (COALESCE(NEW.cluster_id,'')='' OR COALESCE(NEW.pod_uid,'')='' OR COALESCE(NEW.container_name,'')='' OR COALESCE(NEW.image_digest,'')='') THEN
  RAISE EXCEPTION 'SBOM workload identity is required';
 END IF;
 IF NEW.content_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM sbom_image_contents c WHERE c.id=NEW.content_id AND c.payload::jsonb->>'imageDigest'=NEW.image_digest) THEN
  RAISE EXCEPTION 'SBOM content digest mismatch';
 END IF;
 IF TG_OP = 'UPDATE' AND (NEW.cluster_id,NEW.pod_uid,NEW.container_name,NEW.image_digest) IS DISTINCT FROM (OLD.cluster_id,OLD.pod_uid,OLD.container_name,OLD.image_digest) THEN
  RAISE EXCEPTION 'SBOM workload ownership is immutable';
 END IF;
 RETURN NEW;
END $$`).Error; err != nil {
			return err
		}
		if err := db.Exec("DROP TRIGGER IF EXISTS sbom_owner_immutable ON sboms").Error; err != nil {
			return err
		}
		if err := db.Exec("CREATE TRIGGER sbom_owner_immutable BEFORE INSERT OR UPDATE OF cluster_id,pod_uid,container_name,image_digest,content_id ON sboms FOR EACH ROW EXECUTE FUNCTION fortuna_sbom_owner_immutable()").Error; err != nil {
			return err
		}

		if err := db.Exec(`CREATE OR REPLACE FUNCTION fortuna_sbom_association_owner() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.sbom_id IS NULL THEN RETURN NEW; END IF;
 IF NOT EXISTS (SELECT 1 FROM sboms s WHERE s.id=NEW.sbom_id AND s.cluster_id=NEW.cluster_id AND s.pod_uid=NEW.pod_uid
   AND s.deleted_at IS NULL AND s.cluster_id<>'' AND s.pod_uid<>''
   AND (TG_TABLE_NAME='malware_matches' OR s.container_name=to_jsonb(NEW)->>'container_name')) THEN
  RAISE EXCEPTION 'SBOM association ownership mismatch';
 END IF;
 RETURN NEW;
END $$`).Error; err != nil {
			return err
		}
		for _, table := range []string{"pod_image_scans", "cve_matches", "malware_matches"} {
			if !db.Migrator().HasTable(table) {
				continue
			}
			container := " AND s.container_name=a.container_name"
			if table == "malware_matches" {
				container = ""
			}
			var invalid int64
			query := "SELECT COUNT(*) FROM " + table + " a WHERE a.sbom_id IS NOT NULL AND COALESCE(a.cluster_id,'')<>'' AND NOT EXISTS (SELECT 1 FROM sboms s WHERE s.id=a.sbom_id AND s.cluster_id=a.cluster_id AND s.pod_uid=a.pod_uid" + container + ")"
			if err := db.Raw(query).Scan(&invalid).Error; err != nil {
				return err
			}
			if invalid > 0 {
				return fmt.Errorf("%s has %d inconsistent SBOM associations; explicit reconciliation required", table, invalid)
			}
			if err := db.Exec("DROP TRIGGER IF EXISTS sbom_association_owner ON " + table).Error; err != nil {
				return err
			}
			if err := db.Exec("CREATE TRIGGER sbom_association_owner BEFORE INSERT OR UPDATE ON " + table + " FOR EACH ROW EXECUTE FUNCTION fortuna_sbom_association_owner()").Error; err != nil {
				return err
			}
		}
	}
	// Bounded, resumable backfill. Unresolved ownership stays unlinked/quarantined.
	var lastID uint
	for {
		var rows []models.SBOM
		if err := db.Select("id").Where(sbomOwnedPredicate+" AND content_id IS NULL AND id > ?", lastID).Order("id").Limit(200).Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			break
		}
		for _, row := range rows {
			if err := db.Transaction(func(tx *gorm.DB) error { _, err := sbomcontent.Attach(tx, row.ID); return err }); err != nil {
				return fmt.Errorf("backfill SBOM %d: %w", row.ID, err)
			}
			lastID = row.ID
		}
	}
	return nil
}
