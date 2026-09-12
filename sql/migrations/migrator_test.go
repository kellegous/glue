package migrations

import (
	"database/sql"
	"errors"
	"testing"

	_ "modernc.org/sqlite"
)

// errExpectedAny is used as ExpectedErr when Migrate must fail but the error type is not asserted.
var errExpectedAny = errors.New("expected migrate error")

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func schemaVersion(t *testing.T, db *sql.DB) int {
	t.Helper()
	var v int
	if err := db.QueryRowContext(t.Context(), "PRAGMA user_version").Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func tableExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var n int
	err := db.QueryRowContext(
		t.Context(),
		`SELECT COUNT(*) FROM sqlite_master WHERE type IN ('table','view') AND name = ?`,
		name,
	).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	return n > 0
}

func TestMigrator_Migrate(t *testing.T) {
	ctx := t.Context()

	tests := []struct {
		Name        string
		Migrations  []Migration
		Before      func(t *testing.T, db *sql.DB)
		Assert      func(t *testing.T, db *sql.DB)
		ExpectedErr error
	}{
		{
			Name:       "empty migrations leaves version at zero",
			Migrations: nil,
			Assert: func(t *testing.T, db *sql.DB) {
				if got := schemaVersion(t, db); got != 0 {
					t.Fatalf("user_version = %d, want 0", got)
				}
			},
		},
		{
			Name: "single migration applies up and records down",
			Migrations: []Migration{
				{Up: "CREATE TABLE items (id INTEGER PRIMARY KEY)", Down: "DROP TABLE items"},
			},
			Assert: func(t *testing.T, db *sql.DB) {
				if got := schemaVersion(t, db); got != 1 {
					t.Fatalf("user_version = %d, want 1", got)
				}
				if !tableExists(t, db, "items") {
					t.Fatal("expected items table to exist")
				}
				if !tableExists(t, db, "migrations") {
					t.Fatal("expected migrations metadata table")
				}
				var down string
				err := db.QueryRowContext(ctx, "SELECT down FROM migrations WHERE version = ?", 1).Scan(&down)
				if err != nil {
					t.Fatal(err)
				}
				if down != "DROP TABLE items" {
					t.Fatalf("stored down = %q", down)
				}
			},
		},
		{
			Name: "multiple migrations apply in order",
			Migrations: []Migration{
				{Up: "CREATE TABLE a (x INTEGER)", Down: "DROP TABLE a"},
				{Up: "CREATE TABLE b (y INTEGER)", Down: "DROP TABLE b"},
			},
			Assert: func(t *testing.T, db *sql.DB) {
				if got := schemaVersion(t, db); got != 2 {
					t.Fatalf("user_version = %d, want 2", got)
				}
				if !tableExists(t, db, "a") || !tableExists(t, db, "b") {
					t.Fatal("expected tables a and b")
				}
			},
		},
		{
			Name: "already at latest is a no-op",
			Migrations: []Migration{
				{Up: "CREATE TABLE t (id INTEGER PRIMARY KEY)", Down: "DROP TABLE t"},
			},
			Before: func(t *testing.T, db *sql.DB) {
				if err := NewMigrator(db, []Migration{
					{Up: "CREATE TABLE t (id INTEGER PRIMARY KEY)", Down: "DROP TABLE t"},
				}).Migrate(ctx); err != nil {
					t.Fatal(err)
				}
				if schemaVersion(t, db) != 1 {
					t.Fatalf("setup: user_version = %d, want 1", schemaVersion(t, db))
				}
			},
			Assert: func(t *testing.T, db *sql.DB) {
				if got := schemaVersion(t, db); got != 1 {
					t.Fatalf("user_version = %d, want 1", got)
				}
			},
		},
		{
			Name: "invalid up sql rolls back and returns error",
			Migrations: []Migration{
				{Up: "THIS IS NOT VALID SQL", Down: "-- noop"},
			},
			ExpectedErr: errExpectedAny,
			Assert: func(t *testing.T, db *sql.DB) {
				if tableExists(t, db, "migrations") {
					t.Fatal("did not expect migrations table after failed migrate")
				}
				if got := schemaVersion(t, db); got != 0 {
					t.Fatalf("user_version = %d, want 0", got)
				}
			},
		},
		{
			Name: "migrate down fails when down migration row is missing",
			Migrations: []Migration{
				{Up: "CREATE TABLE only (x INTEGER)", Down: "DROP TABLE only"},
			},
			Before: func(t *testing.T, db *sql.DB) {
				if _, err := db.ExecContext(ctx, `
					CREATE TABLE migrations (
						version INTEGER PRIMARY KEY,
						down TEXT NOT NULL
					)`); err != nil {
					t.Fatal(err)
				}
				if _, err := db.ExecContext(ctx, "PRAGMA user_version = 2"); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedErr: sql.ErrNoRows,
		},
		{
			Name: "migrates down when configured with fewer migrations than schema version",
			Migrations: []Migration{
				{Up: "CREATE TABLE v1tab (x INTEGER)", Down: "DROP TABLE v1tab"},
			},
			Before: func(t *testing.T, db *sql.DB) {
				full := []Migration{
					{Up: "CREATE TABLE v1tab (x INTEGER)", Down: "DROP TABLE v1tab"},
					{Up: "CREATE TABLE v2tab (y INTEGER)", Down: "DROP TABLE v2tab"},
				}
				if err := NewMigrator(db, full).Migrate(ctx); err != nil {
					t.Fatal(err)
				}
				if schemaVersion(t, db) != 2 {
					t.Fatalf("setup: user_version = %d, want 2", schemaVersion(t, db))
				}
			},
			Assert: func(t *testing.T, db *sql.DB) {
				if got := schemaVersion(t, db); got != 1 {
					t.Fatalf("user_version = %d, want 1", got)
				}
				if !tableExists(t, db, "v1tab") {
					t.Fatal("expected v1tab after partial downgrade")
				}
				if tableExists(t, db, "v2tab") {
					t.Fatal("did not expect v2tab after downgrade from v2")
				}
				var cnt int
				if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM migrations WHERE version = 2").Scan(&cnt); err != nil {
					t.Fatal(err)
				}
				if cnt != 0 {
					t.Fatalf("version 2 row should be deleted, count=%d", cnt)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			db := openTestDB(t)
			if tt.Before != nil {
				tt.Before(t, db)
			}

			err := NewMigrator(db, tt.Migrations).Migrate(ctx)

			switch tt.ExpectedErr {
			case nil:
				if err != nil {
					t.Fatalf("Migrate: %v", err)
				}
			case errExpectedAny:
				if err == nil {
					t.Fatal("Migrate: expected error, got nil")
				}
			default:
				if err == nil {
					t.Fatal("Migrate: expected error, got nil")
				}
				if !errors.Is(err, tt.ExpectedErr) {
					t.Fatalf("Migrate: error = %v, want wrap %v", err, tt.ExpectedErr)
				}
			}

			if tt.Assert != nil {
				tt.Assert(t, db)
			}
		})
	}
}

func TestMigrator_FactoryReset(t *testing.T) {
	ctx := t.Context()
	migrations := []Migration{
		{Up: "CREATE TABLE a (x INTEGER)", Down: "DROP TABLE a"},
		{Up: "CREATE TABLE b (y INTEGER)", Down: "DROP TABLE b"},
	}

	t.Run("runs every down migration and removes metadata", func(t *testing.T) {
		db := openTestDB(t)
		migrator := NewMigrator(db, migrations)
		if err := migrator.Migrate(ctx); err != nil {
			t.Fatalf("Migrate: %v", err)
		}

		if err := migrator.FactoryReset(ctx); err != nil {
			t.Fatalf("FactoryReset: %v", err)
		}

		if got := schemaVersion(t, db); got != 0 {
			t.Fatalf("user_version = %d, want 0", got)
		}
		for _, name := range []string{"a", "b", "migrations"} {
			if tableExists(t, db, name) {
				t.Fatalf("did not expect %s table after reset", name)
			}
		}
	})

	t.Run("rolls back when a down migration fails", func(t *testing.T) {
		db := openTestDB(t)
		migrator := NewMigrator(db, migrations)
		if err := migrator.Migrate(ctx); err != nil {
			t.Fatalf("Migrate: %v", err)
		}
		if _, err := db.ExecContext(ctx, "UPDATE migrations SET down = 'THIS IS NOT VALID SQL' WHERE version = 2"); err != nil {
			t.Fatal(err)
		}

		if err := migrator.FactoryReset(ctx); err == nil {
			t.Fatal("FactoryReset: expected error, got nil")
		}

		if got := schemaVersion(t, db); got != 2 {
			t.Fatalf("user_version = %d, want 2", got)
		}
		for _, name := range []string{"a", "b", "migrations"} {
			if !tableExists(t, db, name) {
				t.Fatalf("expected %s table after failed reset", name)
			}
		}
	})

	t.Run("is safe on an empty database", func(t *testing.T) {
		db := openTestDB(t)
		if err := NewMigrator(db, migrations).FactoryReset(ctx); err != nil {
			t.Fatalf("FactoryReset: %v", err)
		}
		if got := schemaVersion(t, db); got != 0 {
			t.Fatalf("user_version = %d, want 0", got)
		}
		if tableExists(t, db, "migrations") {
			t.Fatal("did not expect migrations table after reset")
		}
	})
}
