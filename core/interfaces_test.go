package core

import (
	"testing"
)

func TestErrValidation(t *testing.T) {
	if ErrValidation == nil {
		t.Error("ErrValidation should not be nil")
	}
	if ErrValidation.Error() != "validation failed" {
		t.Errorf("ErrValidation.Error() = %q, want %q", ErrValidation.Error(), "validation failed")
	}
}

func TestErrNotFound(t *testing.T) {
	if ErrNotFound == nil {
		t.Error("ErrNotFound should not be nil")
	}
	if ErrNotFound.Error() != "record not found" {
		t.Errorf("ErrNotFound.Error() = %q, want %q", ErrNotFound.Error(), "record not found")
	}
}

func TestDialect_MockImplementation(t *testing.T) {
	pgDialect := newPostgresDialect()
	mysqlDialect := newMySQLDialect()

	// PostgreSQL placeholders
	if got := pgDialect.Placeholder(1); got != "$1" {
		t.Errorf("PostgreSQL Placeholder(1) = %q, want $1", got)
	}
	if got := pgDialect.Placeholder(5); got != "$5" {
		t.Errorf("PostgreSQL Placeholder(5) = %q, want $5", got)
	}

	// MySQL placeholders
	if got := mysqlDialect.Placeholder(1); got != "?" {
		t.Errorf("MySQL Placeholder(1) = %q, want ?", got)
	}

	// QuoteIdentifier
	if got := pgDialect.QuoteIdentifier("users"); got != `"users"` {
		t.Errorf("PostgreSQL QuoteIdentifier = %q, want %q", got, `"users"`)
	}
	if got := mysqlDialect.QuoteIdentifier("users"); got != "`users`" {
		t.Errorf("MySQL QuoteIdentifier = %q, want %q", got, "`users`")
	}

	// GetType
	if got := pgDialect.GetType("int64"); got != "BIGINT" {
		t.Errorf("GetType(int64) = %q, want BIGINT", got)
	}
	if got := pgDialect.GetType("string"); got != "TEXT" {
		t.Errorf("GetType(string) = %q, want TEXT", got)
	}
}

func TestExecutor_MockImplementation(t *testing.T) {
	exec := newMockExecutor()

	// Verify it implements Executor
	var _ Executor = exec
}

func TestLogger_MockImplementation(t *testing.T) {
	logger := newMockLogger()

	// Verify it implements Logger
	var _ Logger = logger

	logger.LogQuery("SELECT 1", nil, 100)
	if len(logger.queries) != 1 {
		t.Errorf("expected 1 query logged, got %d", len(logger.queries))
	}

	logger.LogError("SELECT err", nil, errTest)
	if len(logger.errors) != 1 {
		t.Errorf("expected 1 error logged, got %d", len(logger.errors))
	}
}

func TestScanner_Interface(t *testing.T) {
	// Scanner interface should be defined
	type impl struct{}
	// Just verify the interface is accessible
	var _ Scanner = (*scannerImpl)(nil)
}

type scannerImpl struct{}

func (s *scannerImpl) Scan(dest ...interface{}) error { return nil }
