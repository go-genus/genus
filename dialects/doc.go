// Package dialects provides SQL dialect detection and abstraction for
// PostgreSQL, MySQL, and SQLite.
//
// Use [DetectDialect] to automatically identify the correct dialect from
// a database driver name or DSN. Each dialect handles SQL-specific differences
// like placeholder syntax, quoting, and type mapping.
package dialects
