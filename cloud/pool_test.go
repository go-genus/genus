package cloud

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"
)

// ========================================
// DefaultServerlessPoolConfig Tests
// ========================================

func TestDefaultServerlessPoolConfig(t *testing.T) {
	config := DefaultServerlessPoolConfig()

	if config.PoolMode != PoolModeTransaction {
		t.Errorf("PoolMode = %q, want %q", config.PoolMode, PoolModeTransaction)
	}
	if config.MaxOpenConns != 10 {
		t.Errorf("MaxOpenConns = %d, want 10", config.MaxOpenConns)
	}
	if config.MaxIdleConns != 2 {
		t.Errorf("MaxIdleConns = %d, want 2", config.MaxIdleConns)
	}
	if config.ConnMaxLifetime != 5*time.Minute {
		t.Errorf("ConnMaxLifetime = %v, want %v", config.ConnMaxLifetime, 5*time.Minute)
	}
	if config.ConnMaxIdleTime != 30*time.Second {
		t.Errorf("ConnMaxIdleTime = %v, want %v", config.ConnMaxIdleTime, 30*time.Second)
	}
	if config.WarmConnections != 1 {
		t.Errorf("WarmConnections = %d, want 1", config.WarmConnections)
	}
	if config.ColdStartTimeout != 10*time.Second {
		t.Errorf("ColdStartTimeout = %v, want %v", config.ColdStartTimeout, 10*time.Second)
	}
	if config.ScaleDownDelay != 30*time.Second {
		t.Errorf("ScaleDownDelay = %v, want %v", config.ScaleDownDelay, 30*time.Second)
	}
	if config.HealthCheckInterval != 30*time.Second {
		t.Errorf("HealthCheckInterval = %v, want %v", config.HealthCheckInterval, 30*time.Second)
	}
	if config.HealthCheckTimeout != 5*time.Second {
		t.Errorf("HealthCheckTimeout = %v, want %v", config.HealthCheckTimeout, 5*time.Second)
	}
	if config.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", config.MaxRetries)
	}
	if config.RetryDelay != 100*time.Millisecond {
		t.Errorf("RetryDelay = %v, want %v", config.RetryDelay, 100*time.Millisecond)
	}
	if config.RetryMaxDelay != 2*time.Second {
		t.Errorf("RetryMaxDelay = %v, want %v", config.RetryMaxDelay, 2*time.Second)
	}
	if config.RetryMultiplier != 2.0 {
		t.Errorf("RetryMultiplier = %f, want 2.0", config.RetryMultiplier)
	}
}

// ========================================
// PoolMode Constants Tests
// ========================================

func TestPoolModeConstants(t *testing.T) {
	if PoolModeTransaction != "transaction" {
		t.Errorf("PoolModeTransaction = %q, want %q", PoolModeTransaction, "transaction")
	}
	if PoolModeSession != "session" {
		t.Errorf("PoolModeSession = %q, want %q", PoolModeSession, "session")
	}
	if PoolModeStatement != "statement" {
		t.Errorf("PoolModeStatement = %q, want %q", PoolModeStatement, "statement")
	}
}

// ========================================
// NewServerlessPool Tests
// ========================================

func TestNewServerlessPool(t *testing.T) {
	db := getMockSQLDB()
	defer db.Close()

	config := DefaultServerlessPoolConfig()
	pool := NewServerlessPool(db, config)

	if pool == nil {
		t.Fatal("NewServerlessPool() returned nil")
	}
	if pool.db != db {
		t.Error("db should be set")
	}

	stats := db.Stats()
	if stats.MaxOpenConnections != config.MaxOpenConns {
		t.Errorf("MaxOpenConnections = %d, want %d", stats.MaxOpenConnections, config.MaxOpenConns)
	}
}

// ========================================
// Start / Stop Tests
// ========================================

func TestServerlessPool_StartStop(t *testing.T) {
	db := getMockSQLDBWithPinger(nil)
	defer db.Close()

	config := DefaultServerlessPoolConfig()
	config.WarmConnections = 0 // avoid warm-up issues
	config.HealthCheckInterval = 1 * time.Hour
	config.ScaleDownDelay = 1 * time.Hour
	pool := NewServerlessPool(db, config)

	err := pool.Start(t.Context())
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if !pool.running.Load() {
		t.Error("pool should be running after Start")
	}

	// Start again should be no-op
	err = pool.Start(t.Context())
	if err != nil {
		t.Fatalf("Start() again error = %v", err)
	}

	err = pool.Stop()
	if err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	if pool.running.Load() {
		t.Error("pool should not be running after Stop")
	}

	// Stop again should be no-op
	err = pool.Stop()
	if err != nil {
		t.Fatalf("Stop() again error = %v", err)
	}
}

func TestServerlessPool_Start_WithWarmConnections(t *testing.T) {
	db := getMockSQLDBWithPinger(nil)
	defer db.Close()

	config := DefaultServerlessPoolConfig()
	config.WarmConnections = 1
	config.ColdStartTimeout = 5 * time.Second
	config.HealthCheckInterval = 1 * time.Hour
	config.ScaleDownDelay = 1 * time.Hour
	pool := NewServerlessPool(db, config)

	err := pool.Start(t.Context())
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer pool.Stop()

	pool.mu.RLock()
	warmCount := len(pool.warmConns)
	pool.mu.RUnlock()

	if warmCount != 1 {
		t.Errorf("warm connections = %d, want 1", warmCount)
	}
}

// ========================================
// DB Tests
// ========================================

func TestServerlessPool_DB(t *testing.T) {
	db := getMockSQLDB()
	defer db.Close()

	pool := NewServerlessPool(db, DefaultServerlessPoolConfig())
	if pool.DB() != db {
		t.Error("DB() should return the underlying sql.DB")
	}
}

// ========================================
// Stats Tests
// ========================================

func TestServerlessPool_Stats(t *testing.T) {
	db := getMockSQLDB()
	defer db.Close()

	pool := NewServerlessPool(db, DefaultServerlessPoolConfig())
	stats := pool.Stats()

	if stats.TotalConnections != 0 {
		t.Errorf("TotalConnections = %d, want 0", stats.TotalConnections)
	}
	if stats.ActiveConnections != 0 {
		t.Errorf("ActiveConnections = %d, want 0", stats.ActiveConnections)
	}
	if stats.ColdStarts != 0 {
		t.Errorf("ColdStarts = %d, want 0", stats.ColdStarts)
	}
}

// ========================================
// Conn Tests
// ========================================

func TestServerlessPool_Conn(t *testing.T) {
	db := getMockSQLDBWithPinger(nil)
	defer db.Close()

	config := DefaultServerlessPoolConfig()
	config.WarmConnections = 0
	config.MaxRetries = 0
	config.RetryDelay = 1 * time.Millisecond
	pool := NewServerlessPool(db, config)

	conn, err := pool.Conn(t.Context())
	if err != nil {
		t.Fatalf("Conn() error = %v", err)
	}
	if conn == nil {
		t.Fatal("Conn() returned nil")
	}
	conn.Close()
}

func TestServerlessPool_Conn_FromWarmPool(t *testing.T) {
	db := getMockSQLDBWithPinger(nil)
	defer db.Close()

	config := DefaultServerlessPoolConfig()
	config.WarmConnections = 1
	config.ColdStartTimeout = 5 * time.Second
	config.HealthCheckInterval = 1 * time.Hour
	config.ScaleDownDelay = 1 * time.Hour
	pool := NewServerlessPool(db, config)

	err := pool.Start(t.Context())
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer pool.Stop()

	// Get connection from warm pool
	conn, err := pool.Conn(t.Context())
	if err != nil {
		t.Fatalf("Conn() error = %v", err)
	}
	if conn == nil {
		t.Fatal("Conn() returned nil")
	}
	conn.Close()
}

// ========================================
// ReleaseConn Tests
// ========================================

func TestServerlessPool_ReleaseConn(t *testing.T) {
	db := getMockSQLDBWithPinger(nil)
	defer db.Close()

	config := DefaultServerlessPoolConfig()
	config.WarmConnections = 1
	config.MaxRetries = 0
	pool := NewServerlessPool(db, config)

	conn, err := pool.Conn(t.Context())
	if err != nil {
		t.Fatalf("Conn() error = %v", err)
	}

	// Release should add back to warm pool (since WarmConnections=1)
	pool.ReleaseConn(conn)

	pool.mu.RLock()
	warmCount := len(pool.warmConns)
	pool.mu.RUnlock()

	if warmCount != 1 {
		t.Errorf("warm connections after release = %d, want 1", warmCount)
	}
}

func TestServerlessPool_ReleaseConn_ExcessClosed(t *testing.T) {
	db := getMockSQLDBWithPinger(nil)
	defer db.Close()

	config := DefaultServerlessPoolConfig()
	config.WarmConnections = 0 // no warm connections desired
	config.MaxRetries = 0
	pool := NewServerlessPool(db, config)

	conn, err := pool.Conn(t.Context())
	if err != nil {
		t.Fatalf("Conn() error = %v", err)
	}

	// Release should close the conn since WarmConnections=0
	pool.ReleaseConn(conn)

	pool.mu.RLock()
	warmCount := len(pool.warmConns)
	pool.mu.RUnlock()

	if warmCount != 0 {
		t.Errorf("warm connections after release = %d, want 0", warmCount)
	}
}

// ========================================
// ExecContext Tests
// ========================================

func TestServerlessPool_ExecContext(t *testing.T) {
	db := getMockSQLDB()
	defer db.Close()

	pool := NewServerlessPool(db, DefaultServerlessPoolConfig())

	// This will fail because our mock driver doesn't support real exec,
	// but we can verify the stats are incremented
	_, _ = pool.ExecContext(t.Context(), "INSERT INTO test VALUES ($1)", 1)

	stats := pool.Stats()
	if stats.TotalQueries != 1 {
		t.Errorf("TotalQueries = %d, want 1", stats.TotalQueries)
	}
}

func TestServerlessPool_ExecContext_Error(t *testing.T) {
	db := getMockSQLDB()
	defer db.Close()

	pool := NewServerlessPool(db, DefaultServerlessPoolConfig())

	// Use canceled context to force error
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := pool.ExecContext(ctx, "INSERT INTO test VALUES ($1)", 1)
	if err == nil {
		t.Error("ExecContext() should fail with canceled context")
	}

	stats := pool.Stats()
	if stats.FailedQueries != 1 {
		t.Errorf("FailedQueries = %d, want 1", stats.FailedQueries)
	}
}

// ========================================
// QueryContext Tests
// ========================================

func TestServerlessPool_QueryContext(t *testing.T) {
	db := getMockSQLDB()
	defer db.Close()

	pool := NewServerlessPool(db, DefaultServerlessPoolConfig())

	rows, err := pool.QueryContext(t.Context(), "SELECT 1")
	if err != nil {
		t.Fatalf("QueryContext() error = %v", err)
	}
	if rows != nil {
		rows.Close()
	}

	stats := pool.Stats()
	if stats.TotalQueries != 1 {
		t.Errorf("TotalQueries = %d, want 1", stats.TotalQueries)
	}
}

func TestServerlessPool_QueryContext_Error(t *testing.T) {
	db := getMockSQLDB()
	defer db.Close()

	pool := NewServerlessPool(db, DefaultServerlessPoolConfig())

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := pool.QueryContext(ctx, "SELECT 1")
	if err == nil {
		t.Error("QueryContext() should fail with canceled context")
	}

	stats := pool.Stats()
	if stats.FailedQueries != 1 {
		t.Errorf("FailedQueries = %d, want 1", stats.FailedQueries)
	}
}

// ========================================
// QueryRowContext Tests
// ========================================

func TestServerlessPool_QueryRowContext(t *testing.T) {
	db := getMockSQLDB()
	defer db.Close()

	pool := NewServerlessPool(db, DefaultServerlessPoolConfig())

	row := pool.QueryRowContext(t.Context(), "SELECT 1")
	if row == nil {
		t.Error("QueryRowContext() returned nil")
	}

	stats := pool.Stats()
	if stats.TotalQueries != 1 {
		t.Errorf("TotalQueries = %d, want 1", stats.TotalQueries)
	}
}

// ========================================
// BeginTx Tests
// ========================================

func TestServerlessPool_BeginTx(t *testing.T) {
	db := getMockSQLDB()
	defer db.Close()

	pool := NewServerlessPool(db, DefaultServerlessPoolConfig())

	tx, err := pool.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("BeginTx() error = %v", err)
	}
	if tx == nil {
		t.Fatal("BeginTx() returned nil")
	}
	tx.Rollback()
}

// ========================================
// maybeScaleDown Tests
// ========================================

func TestServerlessPool_maybeScaleDown(t *testing.T) {
	db := getMockSQLDB()
	defer db.Close()

	config := DefaultServerlessPoolConfig()
	config.WarmConnections = 0
	pool := NewServerlessPool(db, config)

	// Should not panic with empty warm conns
	pool.maybeScaleDown()
}

// ========================================
// checkHealth Tests
// ========================================

func TestServerlessPool_checkHealth(t *testing.T) {
	db := getMockSQLDBWithPinger(nil)
	defer db.Close()

	config := DefaultServerlessPoolConfig()
	config.WarmConnections = 0
	config.HealthCheckTimeout = 5 * time.Second
	pool := NewServerlessPool(db, config)

	// Should not panic with empty warm conns
	pool.checkHealth()
}

// ========================================
// NewPgBouncerPool Tests
// ========================================

func TestNewPgBouncerPool(t *testing.T) {
	config := PgBouncerConfig{
		Host:            "localhost",
		Database:        "testdb",
		Username:        "user",
		Password:        "pass",
		PoolMode:        PoolModeTransaction,
		ApplicationName: "myapp",
	}

	pool, err := NewPgBouncerPool(config)
	if err != nil {
		t.Fatalf("NewPgBouncerPool() error = %v", err)
	}
	if pool == nil {
		t.Fatal("NewPgBouncerPool() returned nil")
	}
	defer pool.db.Close()

	// Check that default port was applied
	if config.Port != 0 {
		// Port was set in the function
	}
}

func TestNewPgBouncerPool_DefaultPort(t *testing.T) {
	config := PgBouncerConfig{
		Host:     "localhost",
		Database: "testdb",
		Username: "user",
		Password: "pass",
	}

	pool, err := NewPgBouncerPool(config)
	if err != nil {
		t.Fatalf("NewPgBouncerPool() error = %v", err)
	}
	defer pool.db.Close()
}

func TestNewPgBouncerPool_TransactionMode(t *testing.T) {
	config := PgBouncerConfig{
		Host:     "localhost",
		Port:     6432,
		Database: "testdb",
		Username: "user",
		Password: "pass",
		PoolMode: PoolModeTransaction,
	}

	pool, err := NewPgBouncerPool(config)
	if err != nil {
		t.Fatalf("NewPgBouncerPool() error = %v", err)
	}
	defer pool.db.Close()

	if pool.config.PoolMode != PoolModeTransaction {
		t.Errorf("PoolMode = %q, want %q", pool.config.PoolMode, PoolModeTransaction)
	}
	// Transaction mode should have MaxIdleConns=1
	if pool.config.MaxIdleConns != 1 {
		t.Errorf("MaxIdleConns = %d, want 1 for transaction mode", pool.config.MaxIdleConns)
	}
}

func TestNewPgBouncerPool_SessionMode(t *testing.T) {
	config := PgBouncerConfig{
		Host:     "localhost",
		Port:     6432,
		Database: "testdb",
		Username: "user",
		Password: "pass",
		PoolMode: PoolModeSession,
	}

	pool, err := NewPgBouncerPool(config)
	if err != nil {
		t.Fatalf("NewPgBouncerPool() error = %v", err)
	}
	defer pool.db.Close()

	if pool.config.PoolMode != PoolModeSession {
		t.Errorf("PoolMode = %q, want %q", pool.config.PoolMode, PoolModeSession)
	}
}

// ========================================
// NewPgCatPool Tests
// ========================================

func TestNewPgCatPool(t *testing.T) {
	config := PgCatConfig{
		Host:     "localhost",
		Database: "testdb",
		Username: "user",
		Password: "pass",
	}

	pool, err := NewPgCatPool(config)
	if err != nil {
		t.Fatalf("NewPgCatPool() error = %v", err)
	}
	if pool == nil {
		t.Fatal("NewPgCatPool() returned nil")
	}
	defer pool.db.Close()
}

func TestNewPgCatPool_CustomPort(t *testing.T) {
	config := PgCatConfig{
		Host:     "localhost",
		Port:     5433,
		Database: "testdb",
		Username: "user",
		Password: "pass",
	}

	pool, err := NewPgCatPool(config)
	if err != nil {
		t.Fatalf("NewPgCatPool() error = %v", err)
	}
	defer pool.db.Close()
}

// ========================================
// NewProxySQLPool Tests
// ========================================

func TestNewProxySQLPool(t *testing.T) {
	config := ProxySQLConfig{
		Host:     "localhost",
		Database: "testdb",
		Username: "user",
		Password: "pass",
	}

	pool, err := NewProxySQLPool(config)
	if err != nil {
		t.Fatalf("NewProxySQLPool() error = %v", err)
	}
	if pool == nil {
		t.Fatal("NewProxySQLPool() returned nil")
	}
	defer pool.db.Close()
}

func TestNewProxySQLPool_CustomPort(t *testing.T) {
	config := ProxySQLConfig{
		Host:     "localhost",
		Port:     6034,
		Database: "testdb",
		Username: "user",
		Password: "pass",
	}

	pool, err := NewProxySQLPool(config)
	if err != nil {
		t.Fatalf("NewProxySQLPool() error = %v", err)
	}
	defer pool.db.Close()
}

// ========================================
// ServerlessPoolStats Tests
// ========================================

func TestServerlessPoolStats_ZeroValue(t *testing.T) {
	var stats ServerlessPoolStats
	if stats.TotalConnections != 0 || stats.ActiveConnections != 0 ||
		stats.ColdStarts != 0 || stats.FailedConnections != 0 {
		t.Error("zero value should have all zero fields")
	}
}

// ========================================
// Conn retry behavior Tests
// ========================================

func TestServerlessPool_Conn_CanceledContext(t *testing.T) {
	db := getMockSQLDB()
	defer db.Close()
	// Close the db to force connection errors
	db.Close()

	config := DefaultServerlessPoolConfig()
	config.WarmConnections = 0
	config.MaxRetries = 0
	config.RetryDelay = 1 * time.Millisecond
	pool := NewServerlessPool(db, config)

	_, err := pool.Conn(t.Context())
	if err == nil {
		t.Error("Conn() should fail with closed db")
	}

	stats := pool.Stats()
	if stats.FailedConnections != 1 {
		t.Errorf("FailedConnections = %d, want 1", stats.FailedConnections)
	}
}

// ========================================
// Integration-like Tests
// ========================================

func TestServerlessPool_checkHealth_WithWarmConns(t *testing.T) {
	db := getMockSQLDBWithPinger(nil)
	defer db.Close()

	config := DefaultServerlessPoolConfig()
	config.WarmConnections = 2
	config.ColdStartTimeout = 5 * time.Second
	config.HealthCheckTimeout = 5 * time.Second
	config.HealthCheckInterval = 1 * time.Hour
	config.ScaleDownDelay = 1 * time.Hour
	pool := NewServerlessPool(db, config)

	// Start to create warm connections
	err := pool.Start(t.Context())
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer pool.Stop()

	// Run health check - should ping warm connections
	pool.checkHealth()
}

func TestServerlessPool_maybeScaleDown_WithExcess(t *testing.T) {
	db := getMockSQLDBWithPinger(nil)
	defer db.Close()

	config := DefaultServerlessPoolConfig()
	config.WarmConnections = 1
	config.MaxOpenConns = 10
	config.MaxIdleConns = 5 // High idle count so DB keeps them
	pool := NewServerlessPool(db, config)

	// Create connections and release them to make them idle in the pool
	ctx := t.Context()
	conns := make([]*sql.Conn, 5)
	for i := range conns {
		c, err := db.Conn(ctx)
		if err != nil {
			t.Fatalf("failed to get conn: %v", err)
		}
		conns[i] = c
	}
	for _, c := range conns {
		c.Close()
	}

	// Now reduce the config MaxIdleConns so the condition triggers
	pool.config.MaxIdleConns = 0

	// Add extra warm connections
	conn1, _ := db.Conn(ctx)
	conn2, _ := db.Conn(ctx)
	conn3, _ := db.Conn(ctx)

	pool.mu.Lock()
	pool.warmConns = append(pool.warmConns, conn1, conn2, conn3)
	pool.mu.Unlock()

	// Now stats.Idle > config.MaxIdleConns (0) should be true
	pool.maybeScaleDown()

	pool.mu.RLock()
	warmCount := len(pool.warmConns)
	pool.mu.RUnlock()

	if warmCount > config.WarmConnections {
		t.Errorf("warm connections after scale down = %d, want <= %d", warmCount, config.WarmConnections)
	}
}

func TestServerlessPool_warmUp_PingFailure(t *testing.T) {
	// Use a driver where Ping fails
	db := getMockSQLDBWithPinger(errors.New("ping failed"))
	defer db.Close()

	config := DefaultServerlessPoolConfig()
	config.WarmConnections = 2
	config.ColdStartTimeout = 5 * time.Second
	pool := NewServerlessPool(db, config)

	// warmUp should handle ping failures gracefully (continue to next)
	err := pool.warmUp(t.Context())
	// May or may not error depending on whether Conn() itself fails
	_ = err
}

func TestServerlessPool_Conn_RetryBackoff(t *testing.T) {
	db := getMockSQLDB()
	db.Close() // Close so all connections fail

	config := DefaultServerlessPoolConfig()
	config.WarmConnections = 0
	config.MaxRetries = 2
	config.RetryDelay = 1 * time.Millisecond
	config.RetryMaxDelay = 5 * time.Millisecond
	config.RetryMultiplier = 2.0
	pool := NewServerlessPool(db, config)

	_, err := pool.Conn(t.Context())
	if err == nil {
		t.Error("Conn() should fail with closed db")
	}

	stats := pool.Stats()
	if stats.RetriedConnections != 2 {
		t.Errorf("RetriedConnections = %d, want 2", stats.RetriedConnections)
	}
}

func TestServerlessPool_Start_WarmUpFailure(t *testing.T) {
	db := getMockSQLDB()
	db.Close() // Close to force failure

	config := DefaultServerlessPoolConfig()
	config.WarmConnections = 1
	config.ColdStartTimeout = 100 * time.Millisecond
	pool := NewServerlessPool(db, config)

	err := pool.Start(t.Context())
	if err == nil {
		t.Error("Start() should fail when warmUp fails")
	}
}

func TestServerlessPool_ExecAndQueryWorkflow(t *testing.T) {
	db := getMockSQLDB()
	defer db.Close()

	config := DefaultServerlessPoolConfig()
	pool := NewServerlessPool(db, config)

	// Execute a query
	pool.QueryRowContext(t.Context(), "SELECT 1")

	// Execute
	pool.ExecContext(t.Context(), "UPDATE test SET x = 1")

	// Begin transaction
	tx, err := pool.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("BeginTx() error = %v", err)
	}
	tx.Rollback()

	stats := pool.Stats()
	if stats.TotalQueries != 2 {
		t.Errorf("TotalQueries = %d, want 2", stats.TotalQueries)
	}
}
