package mysql

import (
	"context"
	"fmt"
	"testing"

	"c2c_monitor/internal/domain"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRunMigrationsCreatesSchemaAndTracksVersion(t *testing.T) {
	db := openMigrationTestDB(t)

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

func TestAlertBenchmarkPersistence(t *testing.T) {
	db := openMigrationTestDB(t)

	repo := NewMySQLRepository(db)
	if err := repo.RunMigrations(context.Background()); err != nil {
		t.Fatalf("RunMigrations returned error: %v", err)
	}

	ctx := context.Background()
	if err := repo.UpsertAlertBenchmark(ctx, &domain.AlertBenchmark{
		Pair:  "USDCNY",
		Price: 7.1,
	}); err != nil {
		t.Fatalf("UpsertAlertBenchmark returned error: %v", err)
	}

	benchmark, err := repo.GetAlertBenchmark(ctx, "USDCNY")
	if err != nil {
		t.Fatalf("GetAlertBenchmark returned error: %v", err)
	}
	if benchmark == nil || benchmark.Price != 7.1 {
		t.Fatalf("expected persisted benchmark 7.1, got %#v", benchmark)
	}

	if err := repo.UpsertAlertBenchmark(ctx, &domain.AlertBenchmark{
		Pair:  "USDCNY",
		Price: 7.05,
	}); err != nil {
		t.Fatalf("second UpsertAlertBenchmark returned error: %v", err)
	}

	benchmark, err = repo.GetAlertBenchmark(ctx, "USDCNY")
	if err != nil {
		t.Fatalf("second GetAlertBenchmark returned error: %v", err)
	}
	if benchmark == nil || benchmark.Price != 7.05 {
		t.Fatalf("expected updated benchmark 7.05, got %#v", benchmark)
	}

	if err := repo.UpsertAlertBenchmarkOverride(ctx, &domain.AlertBenchmarkOverride{
		Pair:         "USDCNY",
		TargetAmount: 1000,
		Price:        6.70,
	}); err != nil {
		t.Fatalf("UpsertAlertBenchmarkOverride returned error: %v", err)
	}

	overrides, err := repo.GetAlertBenchmarkOverrides(ctx, "USDCNY")
	if err != nil {
		t.Fatalf("GetAlertBenchmarkOverrides returned error: %v", err)
	}
	if len(overrides) != 1 || overrides[0].TargetAmount != 1000 || overrides[0].Price != 6.70 {
		t.Fatalf("expected 1000 CNY override at 6.70, got %#v", overrides)
	}

	if err := repo.UpsertAlertBenchmarkOverride(ctx, &domain.AlertBenchmarkOverride{
		Pair:         "USDCNY",
		TargetAmount: 1000,
		Price:        6.66,
	}); err != nil {
		t.Fatalf("second UpsertAlertBenchmarkOverride returned error: %v", err)
	}
	overrides, err = repo.GetAlertBenchmarkOverrides(ctx, "USDCNY")
	if err != nil {
		t.Fatalf("second GetAlertBenchmarkOverrides returned error: %v", err)
	}
	if len(overrides) != 1 || overrides[0].Price != 6.66 {
		t.Fatalf("expected updated 1000 CNY override at 6.66, got %#v", overrides)
	}
}

func openMigrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite database: %v", err)
	}
	return db
}
