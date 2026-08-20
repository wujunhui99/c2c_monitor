package mysql

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
)

const (
	initialSchemaMigration    = "2026040401_initial_schema"
	reliabilityIndexMigration = "2026081301_reliability_indexes"
	alertBenchmarkMigration   = "2026082001_alert_benchmark"
)

type SchemaMigrationDAO struct {
	ID        int64     `gorm:"primaryKey;autoIncrement"`
	Name      string    `gorm:"type:varchar(128);uniqueIndex"`
	AppliedAt time.Time `gorm:"index"`
}

func (SchemaMigrationDAO) TableName() string {
	return "schema_migrations"
}

type Migration struct {
	Name string
	Up   func(tx *gorm.DB) error
}

var migrations = []Migration{
	{
		Name: initialSchemaMigration,
		Up: func(tx *gorm.DB) error {
			return runSchemaAutoMigrate(tx)
		},
	},
	{
		Name: reliabilityIndexMigration,
		Up: func(tx *gorm.DB) error {
			return createReliabilityIndexes(tx)
		},
	},
	{
		Name: alertBenchmarkMigration,
		Up: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&AlertBenchmarkDAO{})
		},
	},
}

func (r *MySQLRepository) RunMigrations(ctx context.Context) error {
	db := r.db.WithContext(ctx)

	if err := db.AutoMigrate(&SchemaMigrationDAO{}); err != nil {
		return fmt.Errorf("auto migrate schema_migrations: %w", err)
	}

	for _, migration := range migrations {
		applied, err := migrationApplied(db, migration.Name)
		if err != nil {
			return err
		}
		if applied {
			continue
		}

		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := migration.Up(tx); err != nil {
				return err
			}

			return tx.Create(&SchemaMigrationDAO{
				Name:      migration.Name,
				AppliedAt: time.Now(),
			}).Error
		}); err != nil {
			return fmt.Errorf("apply migration %s: %w", migration.Name, err)
		}
	}

	return nil
}

func migrationApplied(db *gorm.DB, name string) (bool, error) {
	var count int64
	if err := db.Model(&SchemaMigrationDAO{}).Where("name = ?", name).Count(&count).Error; err != nil {
		return false, fmt.Errorf("query migration %s: %w", name, err)
	}
	return count > 0, nil
}

func createReliabilityIndexes(db *gorm.DB) error {
	indexes := []struct {
		model any
		name  string
	}{
		{model: &PricePointDAO{}, name: "idx_price_history"},
		{model: &ForexRateDAO{}, name: "idx_forex_pair_time"},
	}

	for _, index := range indexes {
		if db.Migrator().HasIndex(index.model, index.name) {
			continue
		}
		if err := db.Migrator().CreateIndex(index.model, index.name); err != nil {
			return fmt.Errorf("create index %s: %w", index.name, err)
		}
	}
	return nil
}
