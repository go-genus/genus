package dialects

import (
	"testing"

	"github.com/GabrielOnRails/genus/dialects/mysql"
	"github.com/GabrielOnRails/genus/dialects/postgres"
	"github.com/GabrielOnRails/genus/dialects/sqlite"
)

func TestDetectDialect(t *testing.T) {
	tests := []struct {
		name     string
		driver   string
		expected string
	}{
		{"postgres lowercase", "postgres", "$1"},
		{"postgres uppercase", "POSTGRES", "$1"},
		{"pgx driver", "pgx", "$1"},
		{"mysql lowercase", "mysql", "?"},
		{"mysql uppercase", "MYSQL", "?"},
		{"sqlite3", "sqlite3", "?"},
		{"sqlite", "sqlite", "?"},
		{"unknown defaults to postgres", "unknown", "$1"},
		{"empty defaults to postgres", "", "$1"},
		{"with spaces", "  postgres  ", "$1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dialect := DetectDialect(tt.driver)
			placeholder := dialect.Placeholder(1)
			if placeholder != tt.expected {
				t.Errorf("DetectDialect(%q) placeholder = %q, want %q", tt.driver, placeholder, tt.expected)
			}
		})
	}
}

func TestDetectDialect_QuoteIdentifier(t *testing.T) {
	tests := []struct {
		name     string
		driver   string
		expected string
	}{
		{"postgres uses double quotes", "postgres", "\"users\""},
		{"mysql uses backticks", "mysql", "`users`"},
		{"sqlite uses double quotes", "sqlite3", "\"users\""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dialect := DetectDialect(tt.driver)
			quoted := dialect.QuoteIdentifier("users")
			if quoted != tt.expected {
				t.Errorf("DetectDialect(%q).QuoteIdentifier = %q, want %q", tt.driver, quoted, tt.expected)
			}
		})
	}
}

func TestDetectDialectFromDSN(t *testing.T) {
	tests := []struct {
		name     string
		dsn      string
		expected string
	}{
		// PostgreSQL patterns
		{"postgres url", "postgres://user:pass@localhost:5432/db", "$1"},
		{"postgresql url", "postgresql://user:pass@localhost:5432/db", "$1"},
		{"postgres keyword style", "host=localhost dbname=test", "$1"},

		// MySQL patterns
		{"mysql url", "mysql://user:pass@localhost:3306/db", "?"},
		{"mariadb url", "mariadb://user:pass@localhost:3306/db", "?"},
		{"mysql tcp", "user:pass@tcp(localhost:3306)/db", "?"},
		{"mysql unix", "user:pass@unix(/var/run/mysql.sock)/db", "?"},

		// SQLite patterns
		{"sqlite file prefix", "file:test.db", "?"},
		{"sqlite memory", ":memory:", "?"},
		{"sqlite db extension", "test.db", "?"},
		{"sqlite sqlite extension", "test.sqlite", "?"},
		{"sqlite sqlite3 extension", "test.sqlite3", "?"},

		// Unknown defaults to postgres
		{"unknown", "some-random-string", "$1"},
		{"empty", "", "$1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dialect := DetectDialectFromDSN(tt.dsn)
			placeholder := dialect.Placeholder(1)
			if placeholder != tt.expected {
				t.Errorf("DetectDialectFromDSN(%q) placeholder = %q, want %q", tt.dsn, placeholder, tt.expected)
			}
		})
	}
}

func TestDetectDriverFromDSN(t *testing.T) {
	tests := []struct {
		name     string
		dsn      string
		expected string
	}{
		// PostgreSQL
		{"postgres url", "postgres://user:pass@localhost:5432/db", "postgres"},
		{"postgresql url", "postgresql://user:pass@localhost:5432/db", "postgres"},
		{"postgres keyword style", "host=localhost dbname=test", "postgres"},

		// MySQL
		{"mysql url", "mysql://user:pass@localhost:3306/db", "mysql"},
		{"mariadb url", "mariadb://user:pass@localhost:3306/db", "mysql"},
		{"mysql tcp", "user:pass@tcp(localhost:3306)/db", "mysql"},

		// SQLite
		{"sqlite file prefix", "file:test.db", "sqlite3"},
		{"sqlite memory", ":memory:", "sqlite3"},
		{"sqlite db extension", "test.db", "sqlite3"},

		// Unknown defaults to postgres
		{"unknown", "some-random-string", "postgres"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driver := DetectDriverFromDSN(tt.dsn)
			if driver != tt.expected {
				t.Errorf("DetectDriverFromDSN(%q) = %q, want %q", tt.dsn, driver, tt.expected)
			}
		})
	}
}

func TestDetectDialect_TypeConsistency(t *testing.T) {
	// Verifica que os dialetos retornados são do tipo correto
	pgDialect := DetectDialect("postgres")
	if _, ok := pgDialect.(*postgres.Dialect); !ok {
		t.Errorf("Expected *postgres.Dialect for postgres driver")
	}

	myDialect := DetectDialect("mysql")
	if _, ok := myDialect.(*mysql.Dialect); !ok {
		t.Errorf("Expected *mysql.Dialect for mysql driver")
	}

	sqliteDialect := DetectDialect("sqlite3")
	if _, ok := sqliteDialect.(*sqlite.Dialect); !ok {
		t.Errorf("Expected *sqlite.Dialect for sqlite3 driver")
	}
}
