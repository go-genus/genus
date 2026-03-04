package dialects

import (
	"strings"

	"github.com/go-genus/genus/core"
	"github.com/go-genus/genus/dialects/mysql"
	"github.com/go-genus/genus/dialects/postgres"
	"github.com/go-genus/genus/dialects/sqlite"
)

// DetectDialect returns the appropriate dialect based on the driver name.
// Supported drivers:
//   - postgres, pgx → PostgreSQL dialect
//   - mysql → MySQL dialect
//   - sqlite3, sqlite → SQLite dialect
//
// Returns PostgreSQL dialect as default if driver is not recognized.
func DetectDialect(driver string) core.Dialect {
	driver = strings.ToLower(strings.TrimSpace(driver))

	switch driver {
	case "postgres", "pgx":
		return postgres.New()
	case "mysql":
		return mysql.New()
	case "sqlite3", "sqlite":
		return sqlite.New()
	default:
		// Default to PostgreSQL for unknown drivers
		return postgres.New()
	}
}

// DetectDialectFromDSN attempts to detect the dialect from the DSN string.
// This is useful when the driver name is not available directly.
// It looks for common patterns in connection strings:
//   - postgres://, postgresql:// → PostgreSQL
//   - mysql://, mariadb:// → MySQL
//   - file:, :memory:, .db → SQLite
//
// Returns PostgreSQL dialect as default if pattern is not recognized.
func DetectDialectFromDSN(dsn string) core.Dialect {
	dsn = strings.ToLower(dsn)

	// PostgreSQL patterns
	if strings.HasPrefix(dsn, "postgres://") ||
		strings.HasPrefix(dsn, "postgresql://") ||
		strings.Contains(dsn, "host=") && strings.Contains(dsn, "dbname=") {
		return postgres.New()
	}

	// MySQL patterns
	if strings.HasPrefix(dsn, "mysql://") ||
		strings.HasPrefix(dsn, "mariadb://") ||
		strings.Contains(dsn, "@tcp(") ||
		strings.Contains(dsn, "@unix(") {
		return mysql.New()
	}

	// SQLite patterns
	if strings.HasPrefix(dsn, "file:") ||
		dsn == ":memory:" ||
		strings.HasSuffix(dsn, ".db") ||
		strings.HasSuffix(dsn, ".sqlite") ||
		strings.HasSuffix(dsn, ".sqlite3") {
		return sqlite.New()
	}

	// Default to PostgreSQL
	return postgres.New()
}

// DetectDriverFromDSN attempts to detect the driver name from the DSN string.
// This is useful for sql.Open() when only DSN is provided.
// Returns the driver name or "postgres" as default.
func DetectDriverFromDSN(dsn string) string {
	dsn = strings.ToLower(dsn)

	// PostgreSQL patterns
	if strings.HasPrefix(dsn, "postgres://") ||
		strings.HasPrefix(dsn, "postgresql://") ||
		strings.Contains(dsn, "host=") && strings.Contains(dsn, "dbname=") {
		return "postgres"
	}

	// MySQL patterns
	if strings.HasPrefix(dsn, "mysql://") ||
		strings.HasPrefix(dsn, "mariadb://") ||
		strings.Contains(dsn, "@tcp(") ||
		strings.Contains(dsn, "@unix(") {
		return "mysql"
	}

	// SQLite patterns
	if strings.HasPrefix(dsn, "file:") ||
		dsn == ":memory:" ||
		strings.HasSuffix(dsn, ".db") ||
		strings.HasSuffix(dsn, ".sqlite") ||
		strings.HasSuffix(dsn, ".sqlite3") {
		return "sqlite3"
	}

	// Default to postgres
	return "postgres"
}
