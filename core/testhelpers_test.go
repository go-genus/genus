package core

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"sync/atomic"
	"time"
)

// driverCounter ensures unique driver names across all tests
var driverCounter uint64

// ========================================
// Mock Dialect
// ========================================

// mockDialect implements Dialect for testing.
type mockDialect struct {
	placeholderStyle string // "$" for PostgreSQL, "?" for MySQL
}

func newPostgresDialect() *mockDialect {
	return &mockDialect{placeholderStyle: "$"}
}

func newMySQLDialect() *mockDialect {
	return &mockDialect{placeholderStyle: "?"}
}

func (d *mockDialect) Placeholder(n int) string {
	if d.placeholderStyle == "?" {
		return "?"
	}
	return fmt.Sprintf("$%d", n)
}

func (d *mockDialect) QuoteIdentifier(name string) string {
	if d.placeholderStyle == "?" {
		return "`" + name + "`"
	}
	return `"` + name + `"`
}

func (d *mockDialect) GetType(goType string) string {
	switch goType {
	case "int64":
		return "BIGINT"
	case "string":
		return "TEXT"
	case "bool":
		return "BOOLEAN"
	default:
		return "TEXT"
	}
}

// ========================================
// Mock Executor
// ========================================

// mockResult implements sql.Result for testing.
type mockResult struct {
	lastInsertID int64
	rowsAffected int64
	lastIDErr    error
	rowsErr      error
}

func (r *mockResult) LastInsertId() (int64, error) {
	return r.lastInsertID, r.lastIDErr
}

func (r *mockResult) RowsAffected() (int64, error) {
	return r.rowsAffected, r.rowsErr
}

// mockExecutor implements Executor for testing.
type mockExecutor struct {
	execResult sql.Result
	execErr    error
	queryRows  *sql.Rows
	queryErr   error
	queryRow   *sql.Row
	execCalls  []execCall
	queryCalls []queryCall
}

type execCall struct {
	query string
	args  []interface{}
}

type queryCall struct {
	query string
	args  []interface{}
}

func newMockExecutor() *mockExecutor {
	return &mockExecutor{
		execResult: &mockResult{lastInsertID: 1, rowsAffected: 1},
	}
}

func (m *mockExecutor) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	m.execCalls = append(m.execCalls, execCall{query: query, args: args})
	return m.execResult, m.execErr
}

func (m *mockExecutor) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	m.queryCalls = append(m.queryCalls, queryCall{query: query, args: args})
	return m.queryRows, m.queryErr
}

func (m *mockExecutor) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	m.queryCalls = append(m.queryCalls, queryCall{query: query, args: args})
	return m.queryRow
}

// ========================================
// Mock Logger
// ========================================

// mockLogger implements Logger and records calls.
type mockLogger struct {
	queries []loggedQuery
	errors  []loggedError
}

type loggedQuery struct {
	query    string
	args     []interface{}
	duration int64
}

type loggedError struct {
	query string
	args  []interface{}
	err   error
}

func newMockLogger() *mockLogger {
	return &mockLogger{}
}

func (l *mockLogger) LogQuery(query string, args []interface{}, duration int64) {
	l.queries = append(l.queries, loggedQuery{query: query, args: args, duration: duration})
}

func (l *mockLogger) LogError(query string, args []interface{}, err error) {
	l.errors = append(l.errors, loggedError{query: query, args: args, err: err})
}

// ========================================
// Test Models
// ========================================

// TestModel is a basic model for testing.
type TestModel struct {
	Model
	Name  string `db:"name"`
	Email string `db:"email"`
}

// TestModelWithTableName implements TableNamer.
type TestModelWithTableName struct {
	Model
	Name string `db:"name"`
}

func (m *TestModelWithTableName) TableName() string {
	return "custom_table"
}

// TestModelWithHooks implements all lifecycle hooks.
type TestModelWithHooks struct {
	Model
	Name               string `db:"name"`
	beforeSaveCalled   bool
	afterSaveCalled    bool
	beforeCreateCalled bool
	afterCreateCalled  bool
	beforeUpdateCalled bool
	afterUpdateCalled  bool
	beforeDeleteCalled bool
	afterDeleteCalled  bool
	hookErr            error
}

func (m *TestModelWithHooks) BeforeSave() error {
	m.beforeSaveCalled = true
	return m.hookErr
}

func (m *TestModelWithHooks) AfterSave() error {
	m.afterSaveCalled = true
	return m.hookErr
}

func (m *TestModelWithHooks) BeforeCreate() error {
	m.beforeCreateCalled = true
	return m.hookErr
}

func (m *TestModelWithHooks) AfterCreate() error {
	m.afterCreateCalled = true
	return m.hookErr
}

func (m *TestModelWithHooks) BeforeUpdate() error {
	m.beforeUpdateCalled = true
	return m.hookErr
}

func (m *TestModelWithHooks) AfterUpdate() error {
	m.afterUpdateCalled = true
	return m.hookErr
}

func (m *TestModelWithHooks) BeforeDelete() error {
	m.beforeDeleteCalled = true
	return m.hookErr
}

func (m *TestModelWithHooks) AfterDelete() error {
	m.afterDeleteCalled = true
	return m.hookErr
}

// TestSoftDeleteModel is a model with soft delete support.
type TestSoftDeleteModel struct {
	SoftDeleteModel
	Name string `db:"name"`
}

func (m *TestSoftDeleteModel) TableName() string {
	return "soft_delete_models"
}

// ========================================
// Mock Driver for sql.Row tests
// ========================================

// mockDriverConn implements driver.Conn
type mockDriverConn struct{}

func (c *mockDriverConn) Prepare(query string) (driver.Stmt, error) {
	return &mockDriverStmt{}, nil
}
func (c *mockDriverConn) Close() error { return nil }
func (c *mockDriverConn) Begin() (driver.Tx, error) {
	return &mockDriverTx{}, nil
}

// mockDriverStmt implements driver.Stmt
type mockDriverStmt struct {
	rowsToReturn   driver.Rows
	resultToReturn driver.Result
	err            error
}

func (s *mockDriverStmt) Close() error  { return nil }
func (s *mockDriverStmt) NumInput() int { return -1 }
func (s *mockDriverStmt) Exec(args []driver.Value) (driver.Result, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.resultToReturn != nil {
		return s.resultToReturn, nil
	}
	return &mockDriverResult{lastID: 1, affected: 1}, nil
}
func (s *mockDriverStmt) Query(args []driver.Value) (driver.Rows, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.rowsToReturn != nil {
		return s.rowsToReturn, nil
	}
	return &mockDriverRows{closed: true}, nil
}

// mockDriverTx implements driver.Tx
type mockDriverTx struct{}

func (t *mockDriverTx) Commit() error   { return nil }
func (t *mockDriverTx) Rollback() error { return nil }

// mockDriverResult implements driver.Result
type mockDriverResult struct {
	lastID   int64
	affected int64
}

func (r *mockDriverResult) LastInsertId() (int64, error) { return r.lastID, nil }
func (r *mockDriverResult) RowsAffected() (int64, error) { return r.affected, nil }

// mockDriverRows implements driver.Rows
type mockDriverRows struct {
	columns []string
	values  [][]driver.Value
	index   int
	closed  bool
}

func (r *mockDriverRows) Columns() []string { return r.columns }
func (r *mockDriverRows) Close() error {
	r.closed = true
	return nil
}
func (r *mockDriverRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

// mockDriver implements driver.Driver
type mockDriver struct {
	conn *mockDriverConn
}

func (d *mockDriver) Open(name string) (driver.Conn, error) {
	if d.conn != nil {
		return d.conn, nil
	}
	return &mockDriverConn{}, nil
}

// Helper errors
var (
	errTest     = errors.New("test error")
	errHookFail = errors.New("hook failed")
)

// Helper to get a *sql.DB backed by a mock for simple tests
// Not used for query results, only for transaction tests
func getMockSQLDB() *sql.DB {
	// Register a unique driver name to avoid conflicts
	n := atomic.AddUint64(&driverCounter, 1)
	driverName := fmt.Sprintf("mockdriver_%d_%d", time.Now().UnixNano(), n)
	sql.Register(driverName, &mockDriver{})
	db, _ := sql.Open(driverName, "")
	return db
}

// getMockSQLDBWithRows returns a *sql.DB backed by a mock driver that returns specific rows.
func getMockSQLDBWithRows(columns []string, values [][]driver.Value) *sql.DB {
	n := atomic.AddUint64(&driverCounter, 1)
	driverName := fmt.Sprintf("mockdriver_rows_%d_%d", time.Now().UnixNano(), n)
	conn := &mockDriverConn{}
	d := &mockDriverWithRows{
		conn: conn,
		rows: &mockDriverRows{
			columns: columns,
			values:  values,
		},
	}
	sql.Register(driverName, d)
	db, _ := sql.Open(driverName, "")
	return db
}

// mockDriverWithRows is a driver that returns specific rows for queries.
type mockDriverWithRows struct {
	conn *mockDriverConn
	rows *mockDriverRows
}

func (d *mockDriverWithRows) Open(name string) (driver.Conn, error) {
	return &mockDriverConnWithRows{rows: d.rows}, nil
}

// mockDriverConnWithRows returns specific rows for queries.
type mockDriverConnWithRows struct {
	rows *mockDriverRows
}

func (c *mockDriverConnWithRows) Prepare(query string) (driver.Stmt, error) {
	return &mockDriverStmt{rowsToReturn: &mockDriverRows{
		columns: c.rows.columns,
		values:  c.rows.values,
	}}, nil
}
func (c *mockDriverConnWithRows) Close() error { return nil }
func (c *mockDriverConnWithRows) Begin() (driver.Tx, error) {
	return &mockDriverTx{}, nil
}

// mockExecutorWithQueryRows is a mock executor that returns real sql.Rows from a mock sql.DB.
type mockExecutorWithQueryRows struct {
	db         *sql.DB
	execResult sql.Result
	execErr    error
	execCalls  []execCall
	queryCalls []queryCall
}

func newMockExecutorWithQueryRows(columns []string, values [][]driver.Value) *mockExecutorWithQueryRows {
	return &mockExecutorWithQueryRows{
		db:         getMockSQLDBWithRows(columns, values),
		execResult: &mockResult{lastInsertID: 1, rowsAffected: 1},
	}
}

func (m *mockExecutorWithQueryRows) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	m.execCalls = append(m.execCalls, execCall{query: query, args: args})
	return m.execResult, m.execErr
}

func (m *mockExecutorWithQueryRows) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	m.queryCalls = append(m.queryCalls, queryCall{query: query, args: args})
	return m.db.QueryContext(ctx, query, args...)
}

func (m *mockExecutorWithQueryRows) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	m.queryCalls = append(m.queryCalls, queryCall{query: query, args: args})
	return m.db.QueryRowContext(ctx, query, args...)
}

func (m *mockExecutorWithQueryRows) Close() error {
	return m.db.Close()
}

// TestBeforeCreateOnlyModel only fails on BeforeCreate, not BeforeSave
type TestBeforeCreateOnlyModel struct {
	Model
	Name string `db:"name"`
}

func (m *TestBeforeCreateOnlyModel) BeforeCreate() error {
	return errHookFail
}

// TestAfterCreateOnlyModel only fails on AfterCreate
type TestAfterCreateOnlyModel struct {
	Model
	Name string `db:"name"`
}

func (m *TestAfterCreateOnlyModel) AfterCreate() error {
	return errHookFail
}

// TestAfterSaveOnlyModel only fails on AfterSave
type TestAfterSaveOnlyModel struct {
	Model
	Name string `db:"name"`
}

func (m *TestAfterSaveOnlyModel) AfterSave() error {
	return errHookFail
}

// TestBeforeUpdateOnlyModel only fails on BeforeUpdate
type TestBeforeUpdateOnlyModel struct {
	Model
	Name string `db:"name"`
}

func (m *TestBeforeUpdateOnlyModel) BeforeUpdate() error {
	return errHookFail
}

// TestAfterUpdateOnlyModel only fails on AfterUpdate
type TestAfterUpdateOnlyModel struct {
	Model
	Name string `db:"name"`
}

func (m *TestAfterUpdateOnlyModel) AfterUpdate() error {
	return errHookFail
}

// TestAfterDeleteOnlyModel only fails on AfterDelete
type TestAfterDeleteOnlyModel struct {
	Model
	Name string `db:"name"`
}

func (m *TestAfterDeleteOnlyModel) AfterDelete() error {
	return errHookFail
}

// TestSoftDeleteWithHooksModel combines soft delete with hook errors
type TestSoftDeleteWithAfterDeleteModel struct {
	SoftDeleteModel
	Name string `db:"name"`
}

func (m *TestSoftDeleteWithAfterDeleteModel) TableName() string {
	return "soft_with_hooks"
}

func (m *TestSoftDeleteWithAfterDeleteModel) AfterDelete() error {
	return errHookFail
}

// failingCommitDriver is a driver whose transactions fail on Commit
type failingCommitDriver struct{}

func (d *failingCommitDriver) Open(name string) (driver.Conn, error) {
	return &failingCommitConn{}, nil
}

type failingCommitConn struct{}

func (c *failingCommitConn) Prepare(query string) (driver.Stmt, error) {
	return &mockDriverStmt{}, nil
}
func (c *failingCommitConn) Close() error { return nil }
func (c *failingCommitConn) Begin() (driver.Tx, error) {
	return &failingCommitTx{}, nil
}

type failingCommitTx struct{}

func (t *failingCommitTx) Commit() error   { return errTest }
func (t *failingCommitTx) Rollback() error { return nil }

func getMockSQLDBFailCommit() *sql.DB {
	n := atomic.AddUint64(&driverCounter, 1)
	driverName := fmt.Sprintf("failcommit_%d_%d", time.Now().UnixNano(), n)
	sql.Register(driverName, &failingCommitDriver{})
	db, _ := sql.Open(driverName, "")
	return db
}

// failingRollbackDriver is a driver whose transactions fail on Rollback
type failingRollbackDriver struct{}

func (d *failingRollbackDriver) Open(name string) (driver.Conn, error) {
	return &failingRollbackConn{}, nil
}

type failingRollbackConn struct{}

func (c *failingRollbackConn) Prepare(query string) (driver.Stmt, error) {
	return &mockDriverStmt{}, nil
}
func (c *failingRollbackConn) Close() error { return nil }
func (c *failingRollbackConn) Begin() (driver.Tx, error) {
	return &failingRollbackTx{}, nil
}

type failingRollbackTx struct{}

func (t *failingRollbackTx) Commit() error   { return nil }
func (t *failingRollbackTx) Rollback() error { return errTest }

func getMockSQLDBFailRollback() *sql.DB {
	n := atomic.AddUint64(&driverCounter, 1)
	driverName := fmt.Sprintf("failrollback_%d_%d", time.Now().UnixNano(), n)
	sql.Register(driverName, &failingRollbackDriver{})
	db, _ := sql.Open(driverName, "")
	return db
}

// failingBeginDriver is a driver that fails on Begin
type failingBeginDriver struct{}

func (d *failingBeginDriver) Open(name string) (driver.Conn, error) {
	return &failingBeginConn{}, nil
}

type failingBeginConn struct{}

func (c *failingBeginConn) Prepare(query string) (driver.Stmt, error) {
	return &mockDriverStmt{}, nil
}
func (c *failingBeginConn) Close() error { return nil }
func (c *failingBeginConn) Begin() (driver.Tx, error) {
	return nil, errTest
}

func getMockSQLDBFailBegin() *sql.DB {
	n := atomic.AddUint64(&driverCounter, 1)
	driverName := fmt.Sprintf("failbegin_%d_%d", time.Now().UnixNano(), n)
	sql.Register(driverName, &failingBeginDriver{})
	db, _ := sql.Open(driverName, "")
	return db
}

// failingCloseDriver is a driver whose connections fail on Close
type failingCloseDriver struct{}

func (d *failingCloseDriver) Open(name string) (driver.Conn, error) {
	return &failingCloseConn{}, nil
}

type failingCloseConn struct {
	mockDriverConn
}

func (c *failingCloseConn) Close() error {
	return errTest
}

func (c *failingCloseConn) Ping(ctx context.Context) error {
	return errTest
}

func getMockSQLDBFailClose() *sql.DB {
	n := atomic.AddUint64(&driverCounter, 1)
	driverName := fmt.Sprintf("failclose_%d_%d", time.Now().UnixNano(), n)
	sql.Register(driverName, &failingCloseDriver{})
	db, _ := sql.Open(driverName, "")
	// Force a connection to be created
	db.Ping()
	return db
}

// conditionalMockExecutor fails on the Nth exec call
type conditionalMockExecutor struct {
	execResult  sql.Result
	execCallNum int
	failOnCall  int // 1-based: fail on this call number
	execCalls   []execCall
	queryCalls  []queryCall
	queryRows   *sql.Rows
	queryErr    error
	queryRow    *sql.Row
}

func newConditionalMockExecutor(failOnCall int) *conditionalMockExecutor {
	return &conditionalMockExecutor{
		execResult: &mockResult{lastInsertID: 1, rowsAffected: 1},
		failOnCall: failOnCall,
	}
}

func (m *conditionalMockExecutor) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	m.execCallNum++
	m.execCalls = append(m.execCalls, execCall{query: query, args: args})
	if m.execCallNum == m.failOnCall {
		return nil, errTest
	}
	return m.execResult, nil
}

func (m *conditionalMockExecutor) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	m.queryCalls = append(m.queryCalls, queryCall{query: query, args: args})
	return m.queryRows, m.queryErr
}

func (m *conditionalMockExecutor) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	m.queryCalls = append(m.queryCalls, queryCall{query: query, args: args})
	return m.queryRow
}
