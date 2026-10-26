package migrations

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestMigrator(t *testing.T) {
	ctx := t.Context()
	container, err := postgres.Run(ctx, "postgres:17-alpine",
		postgres.WithDatabase("migrations"),
		postgres.WithUsername("migrations"),
		postgres.WithPassword("migrations"),
		postgres.BasicWaitStrategies(),
	)

	testcontainers.CleanupContainer(t, container)
	if err != nil {
		t.Fatal(err)
	}

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}

	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)

	// Each subtest gets its own database, including an initially absent ledger.
	databaseID := 0
	openDB := func(t *testing.T) *pgxpool.Pool {
		t.Helper()
		databaseID++
		name := fmt.Sprintf("migrations_%d", databaseID)
		if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
			t.Fatal(err)
		}
		config, err := pgxpool.ParseConfig(dsn)
		if err != nil {
			t.Fatal(err)
		}
		config.ConnConfig.Database = name
		config.MaxConns = 10
		db, err := pgxpool.NewWithConfig(t.Context(), config)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(db.Close)
		return db
	}

	migrations := []Migration{
		{Up: "CREATE TABLE items (id integer PRIMARY KEY); INSERT INTO items VALUES (1)", Down: "DELETE FROM items; DROP TABLE items"},
		{Up: "ALTER TABLE items ADD COLUMN name text; UPDATE items SET name = 'first'", Down: "ALTER TABLE items DROP COLUMN name"},
	}

	t.Run("applies scripts and records rollback SQL", func(t *testing.T) {
		db := openDB(t)
		m := NewMigrator(db, migrations)
		mustMigrate(t, m.Migrate(t.Context()))
		assertVersion(t, db, 2)
		var name, down string
		var appliedAt time.Time
		if err := db.QueryRow(t.Context(), "SELECT name FROM items WHERE id = 1").Scan(&name); err != nil || name != "first" {
			t.Fatalf("name = %q, error = %v", name, err)
		}
		if err := db.QueryRow(t.Context(), "SELECT down, applied_at FROM public.schema_migrations WHERE version = 2").Scan(&down, &appliedAt); err != nil {
			t.Fatal(err)
		}
		if down != migrations[1].Down || appliedAt.IsZero() {
			t.Fatalf("unexpected ledger entry: %q, %v", down, appliedAt)
		}
		// Re-executing either script would fail or duplicate the inserted row.
		mustMigrate(t, m.Migrate(t.Context()))
		assertVersion(t, db, 2)
	})

	t.Run("empty migrations and direct connection", func(t *testing.T) {
		db := openDB(t)
		conn, err := db.Acquire(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		mustMigrate(t, NewMigrator(conn.Conn(), nil).Migrate(t.Context()))
		conn.Release()
		assertVersion(t, db, 0)
	})

	t.Run("failed up rolls back entire batch including ledger creation", func(t *testing.T) {
		db := openDB(t)
		m := NewMigrator(db, []Migration{migrations[0], {Up: "INVALID SQL"}})
		if err := m.Migrate(t.Context()); err == nil {
			t.Fatal("expected migration failure")
		}
		assertTable(t, db, "items", false)
		assertTable(t, db, "schema_migrations", false)
		mustMigrate(t, NewMigrator(db, migrations).Migrate(t.Context()))
	})

	t.Run("failed upgrade preserves previously committed version", func(t *testing.T) {
		db := openDB(t)
		mustMigrate(t, NewMigrator(db, migrations[:1]).Migrate(t.Context()))
		m := NewMigrator(db, []Migration{migrations[0], migrations[1], {Up: "INVALID SQL"}})
		if err := m.Migrate(t.Context()); err == nil {
			t.Fatal("expected migration failure")
		}
		assertVersion(t, db, 1)
		var columns int
		if err := db.QueryRow(t.Context(), `SELECT COUNT(*) FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'items' AND column_name = 'name'`).Scan(&columns); err != nil {
			t.Fatal(err)
		}
		if columns != 0 {
			t.Fatal("failed upgrade retained name column")
		}
	})

	t.Run("newer schema requires explicit downgrade and uses stored SQL", func(t *testing.T) {
		db := openDB(t)
		mustMigrate(t, NewMigrator(db, migrations).Migrate(t.Context()))
		older := NewMigrator(db, migrations[:1])
		if err := older.Migrate(t.Context()); !errors.Is(err, ErrNewerSchema) {
			t.Fatalf("Migrate = %v, want ErrNewerSchema", err)
		}
		assertVersion(t, db, 2)
		mustMigrate(t, older.MigrateTo(t.Context(), 1))
		assertVersion(t, db, 1)
		var columns int
		if err := db.QueryRow(t.Context(), `SELECT COUNT(*) FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'items' AND column_name = 'name'`).Scan(&columns); err != nil {
			t.Fatal(err)
		}
		if columns != 0 {
			t.Fatal("downgrade did not remove name column")
		}
		mustMigrate(t, older.MigrateTo(t.Context(), 0))
		assertVersion(t, db, 0)
		assertTable(t, db, "items", false)
		mustMigrate(t, NewMigrator(db, migrations).MigrateTo(t.Context(), 1))
		assertVersion(t, db, 1)
	})

	t.Run("invalid targets leave database untouched", func(t *testing.T) {
		db := openDB(t)
		for _, target := range []int{-1, 3} {
			if err := NewMigrator(db, migrations).MigrateTo(t.Context(), target); err == nil {
				t.Fatalf("expected invalid target %d to fail", target)
			}
		}
		assertTable(t, db, "schema_migrations", false)
	})

	t.Run("rejects gaps before making schema changes", func(t *testing.T) {
		db := openDB(t)
		mustMigrate(t, NewMigrator(db, migrations).Migrate(t.Context()))
		if _, err := db.Exec(t.Context(), "DELETE FROM public.schema_migrations WHERE version = 1"); err != nil {
			t.Fatal(err)
		}
		m := NewMigrator(db, migrations)
		for _, operation := range []func(context.Context) error{m.Migrate, m.FactoryReset} {
			if err := operation(t.Context()); err == nil {
				t.Fatal("expected non-contiguous history error")
			}
		}
		assertTable(t, db, "items", true)
	})

	t.Run("failed rollback restores earlier down operations", func(t *testing.T) {
		db := openDB(t)
		mustMigrate(t, NewMigrator(db, migrations).Migrate(t.Context()))
		if _, err := db.Exec(t.Context(), "UPDATE public.schema_migrations SET down = 'INVALID SQL' WHERE version = 1"); err != nil {
			t.Fatal(err)
		}
		m := NewMigrator(db, nil)
		for _, operation := range []func(context.Context) error{
			func(ctx context.Context) error { return m.MigrateTo(ctx, 0) }, m.FactoryReset,
		} {
			if err := operation(t.Context()); err == nil {
				t.Fatal("expected rollback failure")
			}
			assertVersion(t, db, 2)
			var name string
			if err := db.QueryRow(t.Context(), "SELECT name FROM items WHERE id = 1").Scan(&name); err != nil {
				t.Fatal(err)
			}
		}
	})

	t.Run("factory reset removes ledger and permits fresh migration", func(t *testing.T) {
		db := openDB(t)
		reset := NewMigrator(db, nil)
		mustMigrate(t, reset.FactoryReset(t.Context()))
		assertTable(t, db, "schema_migrations", false)
		mustMigrate(t, NewMigrator(db, migrations).Migrate(t.Context()))
		mustMigrate(t, reset.FactoryReset(t.Context()))
		assertTable(t, db, "items", false)
		assertTable(t, db, "schema_migrations", false)
		mustMigrate(t, NewMigrator(db, migrations).Migrate(t.Context()))
		assertVersion(t, db, 2)
	})

	t.Run("concurrent startup serializes ledger creation and migrations", func(t *testing.T) {
		db := openDB(t)
		m := NewMigrator(db, []Migration{{
			Up:   "SELECT pg_sleep(0.1); CREATE TABLE once_only (id integer); INSERT INTO once_only VALUES (1)",
			Down: "DROP TABLE once_only",
		}})
		var wg sync.WaitGroup
		start := make(chan struct{})
		errs := make(chan error, 8)
		for range 8 {
			wg.Go(func() {
				<-start
				errs <- m.Migrate(t.Context())
			})
		}
		close(start)
		wg.Wait()
		close(errs)
		for err := range errs {
			mustMigrate(t, err)
		}
		assertVersion(t, db, 1)
		var count int
		if err := db.QueryRow(t.Context(), "SELECT COUNT(*) FROM once_only").Scan(&count); err != nil || count != 1 {
			t.Fatalf("row count = %d, error = %v", count, err)
		}
	})

	t.Run("lock wait respects cancellation and releases transaction", func(t *testing.T) {
		db := openDB(t)
		tx, err := db.Begin(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(context.Background()) }()
		if _, err := tx.Exec(t.Context(), "SELECT pg_advisory_xact_lock($1)", int64(0x676c75656d696772)); err != nil {
			t.Fatal(err)
		}
		waitCtx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
		defer cancel()
		m := NewMigrator(db, migrations)
		if err := m.Migrate(waitCtx); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Migrate = %v, want deadline exceeded", err)
		}
		mustMigrate(t, tx.Rollback(t.Context()))
		mustMigrate(t, m.Migrate(t.Context()))
		assertVersion(t, db, 2)
	})
}

func mustMigrate(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func assertVersion(t *testing.T, db *pgxpool.Pool, want int) {
	t.Helper()
	var version, count int
	if err := db.QueryRow(t.Context(), "SELECT COALESCE(MAX(version), 0), COUNT(*) FROM public.schema_migrations").Scan(&version, &count); err != nil {
		t.Fatal(err)
	}
	if version != want || count != want {
		t.Fatalf("version = %d, count = %d, want %d", version, count, want)
	}
}

func assertTable(t *testing.T, db *pgxpool.Pool, name string, want bool) {
	t.Helper()
	var exists bool
	if err := db.QueryRow(t.Context(), "SELECT to_regclass($1) IS NOT NULL", "public."+name).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists != want {
		t.Fatalf("table %s exists = %v, want %v", name, exists, want)
	}
}
