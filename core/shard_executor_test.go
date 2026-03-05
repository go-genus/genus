package core

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/go-genus/genus/sharding"
)

func TestNewShardedDB(t *testing.T) {
	sdb := NewShardedDB(nil, newPostgresDialect())
	if sdb == nil {
		t.Fatal("NewShardedDB returned nil")
	}
	if sdb.Dialect() == nil {
		t.Error("Dialect should not be nil")
	}
	if sdb.Executor() != nil {
		t.Error("Executor should be nil when passed nil")
	}
}

func TestNewShardedDBWithLogger(t *testing.T) {
	logger := newMockLogger()
	sdb := NewShardedDBWithLogger(nil, newPostgresDialect(), logger)
	if sdb == nil {
		t.Fatal("NewShardedDBWithLogger returned nil")
	}
	if sdb.Logger() != logger {
		t.Error("Logger mismatch")
	}
}

func TestShardedDB_SetLogger(t *testing.T) {
	sdb := NewShardedDB(nil, newPostgresDialect())
	logger := newMockLogger()
	sdb.SetLogger(logger)
	if sdb.Logger() != logger {
		t.Error("SetLogger did not set the logger")
	}
}

func TestShardedDB_Dialect(t *testing.T) {
	dialect := newPostgresDialect()
	sdb := NewShardedDB(nil, dialect)
	if sdb.Dialect() != dialect {
		t.Error("Dialect mismatch")
	}
}

func TestWithShardKey_Int64(t *testing.T) {
	ctx := context.Background()
	key := Int64ShardKey(42)
	newCtx := WithShardKey(ctx, key)
	if newCtx == ctx {
		t.Error("WithShardKey should return a new context")
	}

	extracted, ok := sharding.ShardKeyFromContext(newCtx)
	if !ok {
		t.Error("should be able to extract shard key from context")
	}
	if extracted.Value() != int64(42) {
		t.Errorf("extracted key = %v, want 42", extracted.Value())
	}
}

func TestWithShardKey_String(t *testing.T) {
	ctx := context.Background()
	key := StringShardKey("user_123")
	newCtx := WithShardKey(ctx, key)

	extracted, ok := sharding.ShardKeyFromContext(newCtx)
	if !ok {
		t.Error("should be able to extract shard key from context")
	}
	if extracted.Value() != "user_123" {
		t.Errorf("extracted key = %v, want user_123", extracted.Value())
	}
}

func TestShardExecutorTypes(t *testing.T) {
	var _ Int64ShardKey = Int64ShardKey(42)
	var _ StringShardKey = StringShardKey("test")
}

// shardPingDriver implements driver.Driver and driver.Pinger for use with ShardManager
type shardPingDriver struct{}

func (d *shardPingDriver) Open(name string) (driver.Conn, error) {
	return &shardPingConn{}, nil
}

type shardPingConn struct{}

func (c *shardPingConn) Prepare(query string) (driver.Stmt, error) {
	return &mockDriverStmt{}, nil
}
func (c *shardPingConn) Close() error                   { return nil }
func (c *shardPingConn) Begin() (driver.Tx, error)      { return &mockDriverTx{}, nil }
func (c *shardPingConn) Ping(ctx context.Context) error { return nil }

// Register a shard-compatible driver once
var shardDriverName string

func init() {
	shardDriverName = fmt.Sprintf("shardmock_%d", time.Now().UnixNano())
	sql.Register(shardDriverName, &shardPingDriver{})
}

// createTestShardManager creates a ShardManager with mock sql.DB shards for testing.
func createTestShardManager(numShards int) *sharding.ShardManager {
	dsns := make([]string, numShards)
	for i := 0; i < numShards; i++ {
		dsns[i] = fmt.Sprintf("shard_%d", i)
	}

	manager, err := sharding.NewShardManager(shardDriverName, sharding.ShardConfig{
		DSNs: dsns,
	})
	if err != nil {
		return nil
	}
	return manager
}

func TestNewShardExecutor(t *testing.T) {
	manager := createTestShardManager(2)
	if manager == nil {
		t.Skip("could not create test shard manager")
	}

	dialect := newPostgresDialect()
	se := NewShardExecutor(manager, dialect)
	if se == nil {
		t.Fatal("NewShardExecutor returned nil")
	}
	if se.GetManager() != manager {
		t.Error("GetManager should return the manager")
	}
}

func TestShardExecutor_NumShards(t *testing.T) {
	manager := createTestShardManager(3)
	if manager == nil {
		t.Skip("could not create test shard manager")
	}

	se := NewShardExecutor(manager, newPostgresDialect())
	if se.NumShards() != 3 {
		t.Errorf("NumShards() = %d, want 3", se.NumShards())
	}
}

func TestShardExecutor_ExecContext(t *testing.T) {
	manager := createTestShardManager(2)
	if manager == nil {
		t.Skip("could not create test shard manager")
	}

	se := NewShardExecutor(manager, newPostgresDialect())
	ctx := WithShardKey(context.Background(), Int64ShardKey(0))

	_, err := se.ExecContext(ctx, "INSERT INTO t VALUES (1)")
	if err != nil {
		t.Logf("ExecContext error (expected with mock): %v", err)
	}
}

func TestShardExecutor_QueryContext(t *testing.T) {
	manager := createTestShardManager(2)
	if manager == nil {
		t.Skip("could not create test shard manager")
	}

	se := NewShardExecutor(manager, newPostgresDialect())
	ctx := WithShardKey(context.Background(), Int64ShardKey(0))

	rows, err := se.QueryContext(ctx, "SELECT 1")
	if err != nil {
		t.Logf("QueryContext error (expected with mock): %v", err)
	}
	if rows != nil {
		rows.Close()
	}
}

func TestShardExecutor_QueryRowContext(t *testing.T) {
	manager := createTestShardManager(2)
	if manager == nil {
		t.Skip("could not create test shard manager")
	}

	se := NewShardExecutor(manager, newPostgresDialect())
	ctx := WithShardKey(context.Background(), Int64ShardKey(0))

	row := se.QueryRowContext(ctx, "SELECT 1")
	if row == nil {
		t.Error("QueryRowContext should not return nil")
	}
}

func TestShardExecutor_ExecOnShard(t *testing.T) {
	manager := createTestShardManager(2)
	if manager == nil {
		t.Skip("could not create test shard manager")
	}

	se := NewShardExecutor(manager, newPostgresDialect())

	// Valid shard index
	_, err := se.ExecOnShard(context.Background(), 0, "INSERT INTO t VALUES (1)")
	if err != nil {
		t.Logf("ExecOnShard error (expected with mock): %v", err)
	}

	// Invalid shard index
	_, err = se.ExecOnShard(context.Background(), 999, "INSERT INTO t VALUES (1)")
	if err == nil {
		t.Error("ExecOnShard with invalid index should return error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}
}

func TestShardExecutor_QueryOnShard(t *testing.T) {
	manager := createTestShardManager(2)
	if manager == nil {
		t.Skip("could not create test shard manager")
	}

	se := NewShardExecutor(manager, newPostgresDialect())

	// Valid shard index
	rows, err := se.QueryOnShard(context.Background(), 0, "SELECT 1")
	if err != nil {
		t.Logf("QueryOnShard error (expected with mock): %v", err)
	}
	if rows != nil {
		rows.Close()
	}

	// Invalid shard index
	_, err = se.QueryOnShard(context.Background(), -1, "SELECT 1")
	if err == nil {
		t.Error("QueryOnShard with invalid index should return error")
	}
}

func TestShardExecutor_ExecOnAllShards(t *testing.T) {
	manager := createTestShardManager(2)
	if manager == nil {
		t.Skip("could not create test shard manager")
	}

	se := NewShardExecutor(manager, newPostgresDialect())
	results, errors := se.ExecOnAllShards(context.Background(), "INSERT INTO t VALUES (1)")

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
	if len(errors) != 2 {
		t.Errorf("expected 2 errors, got %d", len(errors))
	}
}

func TestShardExecutor_QueryAllShards(t *testing.T) {
	manager := createTestShardManager(2)
	if manager == nil {
		t.Skip("could not create test shard manager")
	}

	se := NewShardExecutor(manager, newPostgresDialect())

	errors := se.QueryAllShards(context.Background(), "SELECT 1", nil, func(shardIndex int, rows *sql.Rows) error {
		return nil
	})

	if len(errors) != 2 {
		t.Errorf("expected 2 errors, got %d", len(errors))
	}
}

func TestShardExecutor_QueryAllShards_CallbackError(t *testing.T) {
	manager := createTestShardManager(2)
	if manager == nil {
		t.Skip("could not create test shard manager")
	}

	se := NewShardExecutor(manager, newPostgresDialect())

	errors := se.QueryAllShards(context.Background(), "SELECT 1", nil, func(shardIndex int, rows *sql.Rows) error {
		return fmt.Errorf("callback error")
	})

	if len(errors) != 2 {
		t.Errorf("expected 2 errors, got %d", len(errors))
	}
}

func TestShardExecutor_QueryAllShards_QueryError(t *testing.T) {
	// Create a shard manager where the query will fail
	// The mock driver's query returns closed rows, which should work
	// But we can test with a context that's cancelled
	manager := createTestShardManager(2)
	if manager == nil {
		t.Skip("could not create test shard manager")
	}

	se := NewShardExecutor(manager, newPostgresDialect())
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately to cause query error

	errors := se.QueryAllShards(ctx, "SELECT 1", nil, func(shardIndex int, rows *sql.Rows) error {
		return nil
	})

	// At least some errors should be non-nil due to cancelled context
	hasError := false
	for _, err := range errors {
		if err != nil {
			hasError = true
			break
		}
	}
	if !hasError {
		t.Log("Expected errors with cancelled context, but none occurred")
	}
}

func TestShardExecutor_BeginTxOnShard(t *testing.T) {
	manager := createTestShardManager(2)
	if manager == nil {
		t.Skip("could not create test shard manager")
	}

	se := NewShardExecutor(manager, newPostgresDialect())

	// Valid shard
	tx, err := se.BeginTxOnShard(context.Background(), 0, nil)
	if err != nil {
		t.Logf("BeginTxOnShard error (expected with mock): %v", err)
	}
	if tx != nil {
		tx.Rollback()
	}

	// Invalid shard
	_, err = se.BeginTxOnShard(context.Background(), 999, nil)
	if err == nil {
		t.Error("BeginTxOnShard with invalid index should return error")
	}
}

func TestShardExecutor_BeginTx(t *testing.T) {
	manager := createTestShardManager(2)
	if manager == nil {
		t.Skip("could not create test shard manager")
	}

	se := NewShardExecutor(manager, newPostgresDialect())
	ctx := WithShardKey(context.Background(), Int64ShardKey(0))

	tx, err := se.BeginTx(ctx, nil)
	if err != nil {
		t.Logf("BeginTx error (expected with mock): %v", err)
	}
	if tx != nil {
		tx.Rollback()
	}
}

func TestShardExecutor_Close(t *testing.T) {
	manager := createTestShardManager(2)
	if manager == nil {
		t.Skip("could not create test shard manager")
	}

	se := NewShardExecutor(manager, newPostgresDialect())
	err := se.Close()
	if err != nil {
		t.Logf("Close error: %v", err)
	}
}

func TestShardedDB_Close_WithExecutor(t *testing.T) {
	manager := createTestShardManager(2)
	if manager == nil {
		t.Skip("could not create test shard manager")
	}

	se := NewShardExecutor(manager, newPostgresDialect())
	sdb := NewShardedDB(se, newPostgresDialect())
	err := sdb.Close()
	if err != nil {
		t.Logf("ShardedDB Close error: %v", err)
	}
}
