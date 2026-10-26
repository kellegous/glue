// Package migrations provides SQLite schema migrations.
//
// Deprecated: Use github.com/kellegous/glue/sqlite/migrations instead.
package migrations

import (
	"database/sql"

	sqlitemigrations "github.com/kellegous/glue/sqlite/migrations"
)

// Migration describes the SQL needed to move the database schema between two
// consecutive versions.
//
// Deprecated: Use github.com/kellegous/glue/sqlite/migrations.Migration instead.
type Migration = sqlitemigrations.Migration

// Migrator applies and reverses an ordered sequence of database migrations.
//
// Deprecated: Use github.com/kellegous/glue/sqlite/migrations.Migrator instead.
type Migrator = sqlitemigrations.Migrator

// NewMigrator returns a Migrator that manages migrations on db.
//
// Deprecated: Use github.com/kellegous/glue/sqlite/migrations.NewMigrator instead.
func NewMigrator(db *sql.DB, migrations []Migration) *Migrator {
	return sqlitemigrations.NewMigrator(db, migrations)
}
