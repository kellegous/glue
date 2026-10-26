package migrations_test

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/kellegous/glue/sql/migrations"
	sqlitemigrations "github.com/kellegous/glue/sqlite/migrations"
)

func TestCompatibility(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	// Both import paths share type identity, including slices and method sets.
	legacy := []migrations.Migration{{
		Up:   "CREATE TABLE example (id INTEGER PRIMARY KEY)",
		Down: "DROP TABLE example",
	}}
	var (
		current     []sqlitemigrations.Migration
		migrator    *sqlitemigrations.Migrator
		oldMigrator *migrations.Migrator
	)
	current = legacy
	migrator = migrations.NewMigrator(db, current)
	oldMigrator = migrator

	if err := oldMigrator.Migrate(t.Context()); err != nil {
		t.Fatal(err)
	}
	var version int
	if err := db.QueryRowContext(t.Context(), "PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 1 {
		t.Fatalf("user_version = %d, want 1", version)
	}
	if err := oldMigrator.FactoryReset(t.Context()); err != nil {
		t.Fatal(err)
	}
}
