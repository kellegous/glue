// Package migrations applies transactional PostgreSQL schema migrations using pgx.
package migrations

import (
	"context"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/kellegous/poop"
)

// ErrNewerSchema indicates that the database is ahead of the configured
// migrations. MigrateTo can be used to explicitly downgrade it.
var ErrNewerSchema = poop.New("database schema is newer than configured migrations")

// DB is implemented by both *pgx.Conn and *pgxpool.Pool.
type DB interface {
	BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error)
}

// Migrator applies and reverses an ordered sequence of database migrations.
// Applied versions and their rollback SQL live in public.schema_migrations.
// Migration definitions must be append-only once applied.
type Migrator struct {
	db         DB
	migrations []Migration
}

// NewMigrator returns a Migrator. Slice element i advances the schema from
// version i to i+1. All migrators in a database share one ledger and lock.
func NewMigrator(db DB, migrations []Migration) *Migrator {
	return &Migrator{
		db:         db,
		migrations: slices.Clone(migrations),
	}
}

// Migrate applies pending migrations in one transaction. It returns
// ErrNewerSchema rather than automatically downgrading a newer database.
func (m *Migrator) Migrate(ctx context.Context) error {
	return poop.Chain(m.migrate(ctx, len(m.migrations), false, false))
}

// MigrateTo explicitly migrates up or down to target, which must be between
// zero and the number of configured migrations. Downgrades use the rollback
// SQL recorded in the database, even for versions absent from this binary.
func (m *Migrator) MigrateTo(ctx context.Context, target int) error {
	if target < 0 || target > len(m.migrations) {
		return poop.Newf("migration target %d outside range [0, %d]", target, len(m.migrations))
	}
	return poop.Chain(m.migrate(ctx, target, true, false))
}

// FactoryReset reverses every applied migration and removes the ledger in one
// transaction. It is also safe on a database with no migrations.
func (m *Migrator) FactoryReset(ctx context.Context) error {
	return poop.Chain(m.migrate(ctx, 0, true, true))
}

func (m *Migrator) migrate(ctx context.Context, target int, allowDown, reset bool) error {
	// Read committed ensures that a waiter sees the previous migrator's commit
	// in statements issued after acquiring the lock.
	tx, err := m.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return poop.Chain(err)
	}
	defer func() {
		// A canceled caller context must not prevent releasing the transaction.
		rollbackCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(rollbackCtx)
	}()

	// Stable, database-wide key reserved for glue's public.schema_migrations.
	// Acquire before CREATE TABLE to serialize first-time initialization too.
	if _, err := tx.Exec(
		ctx,
		`SELECT pg_advisory_xact_lock($1)`,
		int64(0x676c75656d696772),
	); err != nil {
		return poop.Chain(err)
	}

	if _, err := tx.Exec(
		ctx, `
		CREATE TABLE IF NOT EXISTS public.schema_migrations (
			version integer PRIMARY KEY CHECK (version > 0),
			down text NOT NULL,
			applied_at timestamptz NOT NULL DEFAULT now()
		)`,
	); err != nil {
		return poop.Chain(err)
	}

	var active, count int
	if err := tx.QueryRow(
		ctx, `
		SELECT COALESCE(MAX(version), 0), COUNT(*)
		FROM public.schema_migrations`,
	).Scan(&active, &count); err != nil {
		return poop.Chain(err)
	}
	if active != count {
		return poop.Newf("migration history is not contiguous: %d rows through version %d", count, active)
	}

	if active > target && !allowDown {
		return poop.ChainWithf(ErrNewerSchema, "database %d, configured %d", active, target)
	}

	for active < target {
		mig := m.migrations[active]
		version := active + 1
		// Migration scripts have no parameters and may contain multiple SQL
		// statements. Explicit simple protocol avoids preparing an entire script.
		if _, err := tx.Exec(ctx, mig.Up, pgx.QueryExecModeSimpleProtocol); err != nil {
			return poop.ChainWithf(err, "apply migration %d", version)
		}

		if _, err := tx.Exec(
			ctx, `
			INSERT INTO public.schema_migrations (version, down)
			VALUES ($1, $2)`,
			version,
			mig.Down,
		); err != nil {
			return poop.ChainWithf(err, "record migration %d", version)
		}
		active = version
	}

	for active > target {
		var down string
		if err := tx.QueryRow(
			ctx,
			`SELECT down FROM public.schema_migrations WHERE version = $1`,
			active,
		).Scan(&down); err != nil {
			return poop.ChainWithf(err, "read rollback for migration %d", active)
		}

		if _, err := tx.Exec(ctx, down, pgx.QueryExecModeSimpleProtocol); err != nil {
			return poop.ChainWithf(err, "reverse migration %d", active)
		}

		if _, err := tx.Exec(
			ctx,
			`DELETE FROM public.schema_migrations WHERE version = $1`,
			active,
		); err != nil {
			return poop.ChainWithf(err, "delete migration %d", active)
		}

		active--
	}

	if reset {
		if _, err := tx.Exec(
			ctx,
			`DROP TABLE public.schema_migrations`,
		); err != nil {
			return poop.Chain(err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return poop.Chain(err)
	}

	return nil
}
