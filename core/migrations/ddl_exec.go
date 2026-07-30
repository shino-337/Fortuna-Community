package migrations

import "gorm.io/gorm"

// execDDL runs DDL using database/sql so the statement is sent verbatim.
// GORM's db.Exec can mis-parse some PostgreSQL literals and return "insufficient arguments".
func execDDL(db *gorm.DB, sql string) error {
	sdb, err := db.DB()
	if err != nil {
		return err
	}
	_, err = sdb.Exec(sql)
	return err
}
