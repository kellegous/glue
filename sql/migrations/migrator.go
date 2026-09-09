package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/kellegous/poop"

	"github.com/kellegous/glue/fn"
)

type Migrator struct {
	db         *sql.DB
	migrations []Migration
}

type dbOrTx interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func NewMigrator(db *sql.DB, migrations []Migration) *Migrator {
	return &Migrator{db: db, migrations: migrations}
}

func ensureMigrationsTable(ctx context.Context, tx dbOrTx) error {
	if _, err := tx.ExecContext(ctx, `
	  CREATE TABLE IF NOT EXISTS migrations (
		version INTEGER PRIMARY KEY,
		down TEXT NOT NULL
	  )`); err != nil {
		return poop.Chain(err)
	}
	return nil
}

func getSchemaVersion(ctx context.Context, db dbOrTx) (int, error) {
	var version int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return 0, poop.Chain(err)
	}
	return version, nil
}

func setSchemaVersion(ctx context.Context, db dbOrTx, version int) error {
	_, err := db.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", version))
	return poop.Chain(err)
}

// migrateUp migrates up FROM version.
func (m *Migrator) migrateUp(ctx context.Context, tx *sql.Tx, version int) (int, error) {
	mig := m.migrations[version]

	to := version + 1
	if _, err := tx.ExecContext(ctx, mig.Up); err != nil {
		return 0, poop.Chain(err)
	}

	if err := setSchemaVersion(ctx, tx, to); err != nil {
		return 0, poop.Chain(err)
	}

	if _, err := tx.ExecContext(
		ctx,
		"INSERT INTO migrations (version, down) VALUES (?, ?)",
		to,
		mig.Down,
	); err != nil {
		return 0, poop.Chain(err)
	}

	return to, nil
}

// migrateDown migrates down FROM version.
func (m *Migrator) migrateDown(ctx context.Context, tx *sql.Tx, version int) (int, error) {
	var down string
	if err := tx.QueryRowContext(
		ctx,
		"SELECT down FROM migrations WHERE version = ?",
		version,
	).Scan(&down); err != nil {
		return 0, poop.Chain(err)
	}

	if _, err := tx.ExecContext(ctx, down); err != nil {
		return 0, poop.Chain(err)
	}

	to := version - 1

	if err := setSchemaVersion(ctx, tx, to); err != nil {
		return 0, poop.Chain(err)
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM migrations WHERE version = ?", version); err != nil {
		return 0, poop.Chain(err)
	}

	return to, nil
}

func (m *Migrator) Migrate(ctx context.Context) error {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer fn.WithAbandon(tx.Rollback)

	if err := ensureMigrationsTable(ctx, tx); err != nil {
		return poop.Chain(err)
	}

	active, err := getSchemaVersion(ctx, tx)
	if err != nil {
		return err
	}

	latest := len(m.migrations)

	for active < latest {
		active, err = m.migrateUp(ctx, tx, active)
		if err != nil {
			return poop.Chain(err)
		}
	}

	for active > latest {
		active, err = m.migrateDown(ctx, tx, active)
		if err != nil {
			return poop.Chain(err)
		}
	}

	return tx.Commit()
}
