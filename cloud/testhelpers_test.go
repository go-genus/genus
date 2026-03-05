package cloud

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"sync/atomic"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

// driverCounter ensures unique driver names across all tests
var driverCounter uint64

// errTest is a generic test error
var errTest = errors.New("test error")

// ========================================
// Mock Driver
// ========================================

// mockDriverConn implements driver.Conn
type mockDriverConn struct {
	pingErr  error
	beginErr error
	tx       *mockDriverTx
}

func (c *mockDriverConn) Prepare(query string) (driver.Stmt, error) {
	return &mockDriverStmt{}, nil
}
func (c *mockDriverConn) Close() error { return nil }
func (c *mockDriverConn) Begin() (driver.Tx, error) {
	if c.beginErr != nil {
		return nil, c.beginErr
	}
	if c.tx != nil {
		return c.tx, nil
	}
	return &mockDriverTx{}, nil
}

// mockDriverConnPinger implements driver.Conn and driver.Pinger
type mockDriverConnPinger struct {
	mockDriverConn
	pingErr error
}

func (c *mockDriverConnPinger) Ping(ctx context.Context) error {
	return c.pingErr
}

// mockDriverStmt implements driver.Stmt
type mockDriverStmt struct {
	rowsToReturn driver.Rows
	err          error
}

func (s *mockDriverStmt) Close() error  { return nil }
func (s *mockDriverStmt) NumInput() int { return -1 }
func (s *mockDriverStmt) Exec(args []driver.Value) (driver.Result, error) {
	if s.err != nil {
		return nil, s.err
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
type mockDriverTx struct {
	commitErr   error
	rollbackErr error
}

func (t *mockDriverTx) Commit() error   { return t.commitErr }
func (t *mockDriverTx) Rollback() error { return t.rollbackErr }

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
	conn driver.Conn
}

func (d *mockDriver) Open(name string) (driver.Conn, error) {
	if d.conn != nil {
		return d.conn, nil
	}
	return &mockDriverConn{}, nil
}

// getMockSQLDB returns a *sql.DB backed by a mock driver.
func getMockSQLDB() *sql.DB {
	n := atomic.AddUint64(&driverCounter, 1)
	driverName := fmt.Sprintf("cloud_mock_%d_%d", time.Now().UnixNano(), n)
	sql.Register(driverName, &mockDriver{})
	db, _ := sql.Open(driverName, "")
	return db
}

// getMockSQLDBWithPinger returns a *sql.DB backed by a mock driver with Ping support.
func getMockSQLDBWithPinger(pingErr error) *sql.DB {
	n := atomic.AddUint64(&driverCounter, 1)
	driverName := fmt.Sprintf("cloud_mock_pinger_%d_%d", time.Now().UnixNano(), n)
	sql.Register(driverName, &mockDriver{
		conn: &mockDriverConnPinger{pingErr: pingErr},
	})
	db, _ := sql.Open(driverName, "")
	return db
}

// getMockSQLDBFailBegin returns a *sql.DB that fails on Begin.
func getMockSQLDBFailBegin() *sql.DB {
	n := atomic.AddUint64(&driverCounter, 1)
	driverName := fmt.Sprintf("cloud_failbegin_%d_%d", time.Now().UnixNano(), n)
	sql.Register(driverName, &mockDriver{
		conn: &mockDriverConn{beginErr: errTest},
	})
	db, _ := sql.Open(driverName, "")
	return db
}

// getMockSQLDBFailCommit returns a *sql.DB that fails on Commit.
func getMockSQLDBFailCommit() *sql.DB {
	n := atomic.AddUint64(&driverCounter, 1)
	driverName := fmt.Sprintf("cloud_failcommit_%d_%d", time.Now().UnixNano(), n)
	sql.Register(driverName, &mockDriver{
		conn: &mockDriverConn{tx: &mockDriverTx{commitErr: errTest}},
	})
	db, _ := sql.Open(driverName, "")
	return db
}

// retryErrorDriver is a driver that returns serialization errors on Begin.
type retryErrorDriver struct {
	callCount int
	maxFails  int
}

type retryErrorConn struct {
	driver    *retryErrorDriver
	commitErr error
}

func (c *retryErrorConn) Prepare(query string) (driver.Stmt, error) {
	return &mockDriverStmt{}, nil
}
func (c *retryErrorConn) Close() error { return nil }
func (c *retryErrorConn) Begin() (driver.Tx, error) {
	return &retryErrorTx{conn: c}, nil
}

type retryErrorTx struct {
	conn *retryErrorConn
}

func (t *retryErrorTx) Commit() error {
	t.conn.driver.callCount++
	if t.conn.driver.callCount <= t.conn.driver.maxFails {
		return fmt.Errorf("restart transaction: TransactionRetryError: 40001")
	}
	return nil
}

func (t *retryErrorTx) Rollback() error { return nil }

func (d *retryErrorDriver) Open(name string) (driver.Conn, error) {
	return &retryErrorConn{driver: d, commitErr: nil}, nil
}

// getMockSQLDBRetryError returns a *sql.DB that fails N times with retry errors.
func getMockSQLDBRetryError(maxFails int) *sql.DB {
	n := atomic.AddUint64(&driverCounter, 1)
	driverName := fmt.Sprintf("cloud_retry_%d_%d", time.Now().UnixNano(), n)
	d := &retryErrorDriver{maxFails: maxFails}
	sql.Register(driverName, d)
	db, _ := sql.Open(driverName, "")
	return db
}

// failingCloseConn implements driver.Conn but fails on Close
type failingCloseConn struct {
	mockDriverConn
}

func (c *failingCloseConn) Close() error {
	return fmt.Errorf("close failed")
}

func (c *failingCloseConn) Ping(ctx context.Context) error {
	return fmt.Errorf("close failed")
}

// getMockSQLDBFailClose returns a *sql.DB that fails when its underlying connections close.
func getMockSQLDBFailClose() *sql.DB {
	n := atomic.AddUint64(&driverCounter, 1)
	driverName := fmt.Sprintf("cloud_failclose_%d_%d", time.Now().UnixNano(), n)
	sql.Register(driverName, &mockDriver{
		conn: &failingCloseConn{},
	})
	db, _ := sql.Open(driverName, "")
	// Force a connection to be created
	db.Ping()
	return db
}

// getMockSQLDBFailRollback returns a *sql.DB that fails on Rollback.
func getMockSQLDBFailRollback() *sql.DB {
	n := atomic.AddUint64(&driverCounter, 1)
	driverName := fmt.Sprintf("cloud_failrollback_%d_%d", time.Now().UnixNano(), n)
	sql.Register(driverName, &mockDriver{
		conn: &mockDriverConn{tx: &mockDriverTx{rollbackErr: fmt.Errorf("rollback failed")}},
	})
	db, _ := sql.Open(driverName, "")
	return db
}
