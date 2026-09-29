// migration-rehearsal runs startup migrations on an isolated restored database.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"

	"github.com/fortuna/core/internal/storage"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func run() error {
	dsn := os.Getenv("FORTUNA_REHEARSAL_POSTGRES_URL")
	u, err := url.Parse(dsn)
	if err != nil || u.Scheme != "postgres" || u.Hostname() != "127.0.0.1" || !strings.HasPrefix(u.Path, "/fortuna_rehearsal_") {
		return fmt.Errorf("dedicated loopback fortuna_rehearsal_ database required")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return err
	}
	pool, err := db.DB()
	if err != nil {
		return err
	}
	defer pool.Close()
	tables := []string{"pods", "insights", "risk_scores", "runtime_events", "sboms", "sbom_components"}
	before := map[string]int64{}
	after := map[string]int64{}
	for _, table := range tables {
		if db.Migrator().HasTable(table) {
			var count int64
			if err = db.Table(table).Count(&count).Error; err != nil {
				return err
			}
			before[table] = count
		}
	}
	if len(before) == 0 {
		return fmt.Errorf("populated backup required")
	}
	for i := 0; i < 2; i++ {
		if err = storage.Migrate(db); err != nil {
			return err
		}
	}
	for table, count := range before {
		var current int64
		if err = db.Table(table).Count(&current).Error; err != nil {
			return err
		}
		after[table] = current
		if current < count {
			return fmt.Errorf("migration lost rows in %s", table)
		}
	}
	var quarantined int64
	if err = db.Table("risk_score_ownership_quarantines").Count(&quarantined).Error; err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"passed": true, "before": before, "after": after, "quarantinedRiskScores": quarantined, "migrationReruns": 2})
}
func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
