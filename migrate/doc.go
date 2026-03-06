// Package migrate provides database migration management for the Genus ORM.
//
// [Migrator] handles registering, running, and rolling back migrations.
// Each [Migration] has a version, description, and Up/Down functions.
// Migration state is tracked in a schema_migrations table.
//
// The package also includes schema diffing, auto-migration from struct
// definitions, and migration visualization.
package migrate
