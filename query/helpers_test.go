package query

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/go-genus/genus/core"
)

// --- Mock Dialect (PostgreSQL-style) ---

type mockDialect struct{}

func (d *mockDialect) Placeholder(n int) string {
	return fmt.Sprintf("$%d", n)
}

func (d *mockDialect) QuoteIdentifier(name string) string {
	return fmt.Sprintf(`"%s"`, name)
}

func (d *mockDialect) GetType(goType string) string {
	return "TEXT"
}

// --- Mock Dialect (MySQL-style) ---

type mockMySQLDialect struct{}

func (d *mockMySQLDialect) Placeholder(n int) string {
	return "?"
}

func (d *mockMySQLDialect) QuoteIdentifier(name string) string {
	return fmt.Sprintf("`%s`", name)
}

func (d *mockMySQLDialect) GetType(goType string) string {
	return "TEXT"
}

// --- Mock Logger ---

type mockLogger struct {
	queries []string
	errors  []error
}

func newMockLogger() *mockLogger {
	return &mockLogger{}
}

func (l *mockLogger) LogQuery(query string, args []interface{}, duration int64) {
	l.queries = append(l.queries, query)
}

func (l *mockLogger) LogError(query string, args []interface{}, err error) {
	l.errors = append(l.errors, err)
}

// --- Mock Executor ---

type mockExecutor struct {
	execFn     func(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	queryFn    func(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	queryRowFn func(ctx context.Context, query string, args ...interface{}) *sql.Row
}

func (e *mockExecutor) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if e.execFn != nil {
		return e.execFn(ctx, query, args...)
	}
	return nil, fmt.Errorf("not implemented")
}

func (e *mockExecutor) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if e.queryFn != nil {
		return e.queryFn(ctx, query, args...)
	}
	return nil, fmt.Errorf("not implemented")
}

func (e *mockExecutor) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if e.queryRowFn != nil {
		return e.queryRowFn(ctx, query, args...)
	}
	return nil
}

// --- Test model types ---

type testUser struct {
	core.Model
	Name  string `db:"name"`
	Email string `db:"email"`
	Age   int    `db:"age"`
}

func (u testUser) TableName() string {
	return "users"
}

type testPost struct {
	ID     int64  `db:"id"`
	Title  string `db:"title"`
	UserID int64  `db:"user_id"`
}

func (p testPost) TableName() string {
	return "posts"
}

// newTestBuilder creates a Builder for testing SQL generation.
func newTestBuilder() *Builder[testUser] {
	return NewBuilder[testUser](
		&mockExecutor{},
		&mockDialect{},
		newMockLogger(),
		"users",
	)
}

func newTestBuilderMySQL() *Builder[testUser] {
	return NewBuilder[testUser](
		&mockExecutor{},
		&mockMySQLDialect{},
		newMockLogger(),
		"users",
	)
}
