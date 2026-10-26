package migrations

// Migration describes the SQL needed to move the database schema between two
// consecutive versions. SQL must be safe to execute inside a transaction and
// must not contain transaction control statements.
type Migration struct {
	// Up is the SQL that applies this migration.
	Up string

	// Down is the SQL that reverses this migration.
	Down string
}
