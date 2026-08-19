package mysql

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRunMigrationsCreatesSchemaAndTracksVersion(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite database: %v", err)
	}

	repo := NewMySQLRepository(db)
	if err := repo.RunMigrations(context.Background()); err != nil {
		t.Fatalf("RunMigrations returned error: %v", err)
	}

	for _, model := range schemaModels() {
		if !db.Migrator().HasTable(model) {
			t.Fatalf("expected table for %T to exist", model)
		}
	}

	if !db.Migrator().HasTable(&SchemaMigrationDAO{}) {
		t.Fatal("expected schema_migrations table to exist")
	}

	var count int64
	if err := db.Model(&SchemaMigrationDAO{}).Count(&count).Error; err != nil {
		t.Fatalf("failed to count schema_migrations rows: %v", err)
	}
	if count != int64(len(migrations)) {
		t.Fatalf("expected %d applied migrations, got %d", len(migrations), count)
	}

	if err := repo.RunMigrations(context.Background()); err != nil {
		t.Fatalf("second RunMigrations returned error: %v", err)
	}
	if err := db.Model(&SchemaMigrationDAO{}).Count(&count).Error; err != nil {
		t.Fatalf("failed to count schema_migrations rows after rerun: %v", err)
	}
	if count != int64(len(migrations)) {
		t.Fatalf("expected migration rerun to stay idempotent with %d rows, got %d", len(migrations), count)
	}

	for _, index := range []struct {
		model any
		name  string
	}{
		{model: &PricePointDAO{}, name: "idx_price_history"},
		{model: &ForexRateDAO{}, name: "idx_forex_pair_time"},
	} {
		if !db.Migrator().HasIndex(index.model, index.name) {
			t.Fatalf("expected index %s to exist", index.name)
		}
	}
}
