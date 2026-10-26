package migrations

// Migration describes the SQL needed to move the database schema between two
// consecutive versions.
type Migration struct {
	// Up is the SQL that applies this migration.
	Up string

	// Down is the SQL that reverses this migration.
	Down string
}
