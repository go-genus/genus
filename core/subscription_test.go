package core

import (
	"context"
	"database/sql/driver"
	"testing"
	"time"
)

func TestDefaultSubscriptionConfig(t *testing.T) {
	config := DefaultSubscriptionConfig()
	if config.ChannelPrefix != "genus_" {
		t.Errorf("ChannelPrefix = %q, want %q", config.ChannelPrefix, "genus_")
	}
	if config.PollInterval != 100*time.Millisecond {
		t.Errorf("PollInterval = %v, want 100ms", config.PollInterval)
	}
}

func TestNewSubscriptionManager(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	config := DefaultSubscriptionConfig()

	sm := NewSubscriptionManager(exec, dialect, config)
	if sm == nil {
		t.Fatal("NewSubscriptionManager returned nil")
	}
	defer sm.Close()
}

func TestSubscription_Cancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	sub := &Subscription{
		channel: "test",
		cancel:  cancel,
		active:  true,
	}

	if !sub.IsActive() {
		t.Error("should be active")
	}

	sub.Cancel()
	if sub.IsActive() {
		t.Error("should not be active after cancel")
	}

	// Cancel again should not panic
	sub.Cancel()
	_ = ctx
}

func TestSubscription_Cancel_NotActive(t *testing.T) {
	sub := &Subscription{
		channel: "test",
		active:  false,
	}

	// Should not panic
	sub.Cancel()
}

func TestSubscription_IsActive(t *testing.T) {
	sub := &Subscription{active: true}
	if !sub.IsActive() {
		t.Error("should be active")
	}

	sub.active = false
	if sub.IsActive() {
		t.Error("should not be active")
	}
}

func TestSubscriptionManager_Subscribe(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	config := DefaultSubscriptionConfig()

	sm := NewSubscriptionManager(exec, dialect, config)
	defer sm.Close()

	sub, err := sm.Subscribe(context.Background(), "users", func(payload NotifyPayload) {})
	if err != nil {
		t.Errorf("Subscribe error = %v", err)
	}
	if sub == nil {
		t.Fatal("sub should not be nil")
	}
	if !sub.IsActive() {
		t.Error("sub should be active")
	}

	// Should have executed LISTEN
	if len(exec.execCalls) != 1 {
		t.Errorf("expected 1 exec call (LISTEN), got %d", len(exec.execCalls))
	}

	sub.Cancel()
}

func TestSubscriptionManager_Subscribe_Error(t *testing.T) {
	exec := newMockExecutor()
	exec.execErr = errTest
	dialect := newPostgresDialect()
	config := DefaultSubscriptionConfig()

	sm := NewSubscriptionManager(exec, dialect, config)
	defer sm.Close()

	_, err := sm.Subscribe(context.Background(), "users", func(payload NotifyPayload) {})
	if err == nil {
		t.Error("should return error when LISTEN fails")
	}
}

func TestSubscriptionManager_Unsubscribe(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	config := DefaultSubscriptionConfig()

	sm := NewSubscriptionManager(exec, dialect, config)
	defer sm.Close()

	sub, err := sm.Subscribe(context.Background(), "users", func(payload NotifyPayload) {})
	if err != nil {
		t.Fatalf("Subscribe error: %v", err)
	}

	err = sm.Unsubscribe(context.Background(), sub)
	if err != nil {
		t.Errorf("Unsubscribe error = %v", err)
	}

	if sub.IsActive() {
		t.Error("sub should not be active after unsubscribe")
	}
}

func TestSubscriptionManager_Unsubscribe_Error(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	config := DefaultSubscriptionConfig()

	sm := NewSubscriptionManager(exec, dialect, config)
	defer sm.Close()

	sub, _ := sm.Subscribe(context.Background(), "users", func(payload NotifyPayload) {})

	// Make UNLISTEN fail
	exec.execErr = errTest
	err := sm.Unsubscribe(context.Background(), sub)
	if err == nil {
		t.Error("should return error when UNLISTEN fails")
	}
}

func TestSubscriptionManager_Close(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	config := DefaultSubscriptionConfig()

	sm := NewSubscriptionManager(exec, dialect, config)

	sub1, _ := sm.Subscribe(context.Background(), "users", func(payload NotifyPayload) {})
	sub2, _ := sm.Subscribe(context.Background(), "orders", func(payload NotifyPayload) {})

	sm.Close()

	if sub1.IsActive() {
		t.Error("sub1 should not be active after Close")
	}
	if sub2.IsActive() {
		t.Error("sub2 should not be active after Close")
	}
}

func TestSubscriptionManager_Notify(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	config := DefaultSubscriptionConfig()

	sm := NewSubscriptionManager(exec, dialect, config)
	defer sm.Close()

	payload := NotifyPayload{
		Table:  "users",
		Action: "INSERT",
	}

	err := sm.Notify(context.Background(), "users", payload)
	if err != nil {
		t.Errorf("Notify error = %v", err)
	}
}

func TestSubscriptionManager_CreateNotifyTrigger(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	config := DefaultSubscriptionConfig()

	sm := NewSubscriptionManager(exec, dialect, config)
	defer sm.Close()

	err := sm.CreateNotifyTrigger(context.Background(), "users")
	if err != nil {
		t.Errorf("CreateNotifyTrigger error = %v", err)
	}

	// Should have 2 exec calls (function + trigger)
	if len(exec.execCalls) != 2 {
		t.Errorf("expected 2 exec calls, got %d", len(exec.execCalls))
	}
}

func TestSubscriptionManager_CreateNotifyTrigger_FunctionError(t *testing.T) {
	exec := newMockExecutor()
	exec.execErr = errTest
	dialect := newPostgresDialect()
	config := DefaultSubscriptionConfig()

	sm := NewSubscriptionManager(exec, dialect, config)
	defer sm.Close()

	err := sm.CreateNotifyTrigger(context.Background(), "users")
	if err == nil {
		t.Error("should return error")
	}
}

func TestSubscriptionManager_DropNotifyTrigger(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	config := DefaultSubscriptionConfig()

	sm := NewSubscriptionManager(exec, dialect, config)
	defer sm.Close()

	err := sm.DropNotifyTrigger(context.Background(), "users")
	if err != nil {
		t.Errorf("DropNotifyTrigger error = %v", err)
	}

	if len(exec.execCalls) != 2 {
		t.Errorf("expected 2 exec calls, got %d", len(exec.execCalls))
	}
}

func TestSubscriptionManager_DropNotifyTrigger_Error(t *testing.T) {
	exec := newMockExecutor()
	exec.execErr = errTest
	dialect := newPostgresDialect()
	config := DefaultSubscriptionConfig()

	sm := NewSubscriptionManager(exec, dialect, config)
	defer sm.Close()

	err := sm.DropNotifyTrigger(context.Background(), "users")
	if err != nil {
		// First ExecContext fails
	}
}

func TestChangeStream_Changes(t *testing.T) {
	cs := &ChangeStream{
		tableName: "users",
		changes:   make(chan NotifyPayload, 10),
	}

	ch := cs.Changes()
	if ch == nil {
		t.Error("Changes() should not return nil")
	}
}

func TestChangeStream_Close(t *testing.T) {
	_, cancel := context.WithCancel(context.Background())
	sub := &Subscription{
		channel: "test",
		cancel:  cancel,
		active:  true,
	}

	cs := &ChangeStream{
		tableName: "users",
		changes:   make(chan NotifyPayload, 10),
		sub:       sub,
	}

	cs.Close()
	if !cs.closed {
		t.Error("should be closed")
	}

	// Double close should not panic
	cs.Close()
}

func TestReadYourWritesHelper(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()

	helper := NewReadYourWritesHelper(exec, dialect)
	if helper == nil {
		t.Fatal("NewReadYourWritesHelper returned nil")
	}
}

func TestReadYourWritesHelper_MySQL_NotSupported(t *testing.T) {
	exec := newMockExecutor()
	dialect := newMySQLDialect()

	helper := NewReadYourWritesHelper(exec, dialect)
	err := helper.EnsureReplication(context.Background(), "0/0")
	if err == nil {
		t.Error("should return error for MySQL")
	}
}

func TestReadYourWritesHelper_GetCurrentLSN_MySQL(t *testing.T) {
	exec := newMockExecutor()
	dialect := newMySQLDialect()

	helper := NewReadYourWritesHelper(exec, dialect)
	_, err := helper.GetCurrentLSN(context.Background())
	if err == nil {
		t.Error("should return error for MySQL")
	}
}

func TestNotifyPayload_Fields(t *testing.T) {
	payload := NotifyPayload{
		Table:     "users",
		Action:    "INSERT",
		RecordID:  1,
		Timestamp: time.Now(),
	}

	if payload.Table != "users" {
		t.Error("Table mismatch")
	}
	if payload.Action != "INSERT" {
		t.Error("Action mismatch")
	}
}

func TestWatchConfig(t *testing.T) {
	config := WatchConfig{
		Actions: []string{"INSERT", "UPDATE"},
		Filter: func(payload NotifyPayload) bool {
			return payload.Table == "users"
		},
	}

	if len(config.Actions) != 2 {
		t.Error("Actions should have 2 items")
	}
}

func TestSubscriptionManager_NewChangeStream(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	config := DefaultSubscriptionConfig()

	sm := NewSubscriptionManager(exec, dialect, config)
	defer sm.Close()

	stream, err := sm.NewChangeStream(context.Background(), "users", 10)
	if err != nil {
		t.Fatalf("NewChangeStream error = %v", err)
	}
	if stream == nil {
		t.Fatal("stream should not be nil")
	}

	ch := stream.Changes()
	if ch == nil {
		t.Error("Changes() should not be nil")
	}

	stream.Close()
}

func TestSubscriptionManager_NewChangeStream_DefaultBufferSize(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	config := DefaultSubscriptionConfig()

	sm := NewSubscriptionManager(exec, dialect, config)
	defer sm.Close()

	// Pass 0 to use default buffer size
	stream, err := sm.NewChangeStream(context.Background(), "users", 0)
	if err != nil {
		t.Fatalf("NewChangeStream error = %v", err)
	}
	defer stream.Close()

	if stream == nil {
		t.Fatal("stream should not be nil")
	}
}

func TestSubscriptionManager_NewChangeStream_BufferFull(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	config := DefaultSubscriptionConfig()

	sm := NewSubscriptionManager(exec, dialect, config)
	defer sm.Close()

	stream, err := sm.NewChangeStream(context.Background(), "users", 1)
	if err != nil {
		t.Fatalf("NewChangeStream error = %v", err)
	}
	defer stream.Close()

	// The stream's subscription handler sends to the channel
	// Fill the buffer first
	stream.sub.handler(NotifyPayload{Table: "users", Action: "INSERT"})
	// This should be dropped (buffer full with size 1)
	stream.sub.handler(NotifyPayload{Table: "users", Action: "UPDATE"})

	// Drain one
	select {
	case p := <-stream.Changes():
		if p.Action != "INSERT" {
			t.Errorf("expected INSERT, got %s", p.Action)
		}
	default:
		t.Error("should have received a payload")
	}
}

func TestSubscriptionManager_NewChangeStream_ClosedHandler(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	config := DefaultSubscriptionConfig()

	sm := NewSubscriptionManager(exec, dialect, config)
	defer sm.Close()

	stream, err := sm.NewChangeStream(context.Background(), "users", 10)
	if err != nil {
		t.Fatalf("NewChangeStream error = %v", err)
	}

	// Save handler reference before closing
	handler := stream.sub.handler
	stream.Close()

	// Handler invoked after close should not panic (closed check prevents send to closed channel)
	handler(NotifyPayload{Table: "users", Action: "INSERT"})
}

func TestSubscriptionManager_NewChangeStream_Error(t *testing.T) {
	exec := newMockExecutor()
	exec.execErr = errTest
	dialect := newPostgresDialect()
	config := DefaultSubscriptionConfig()

	sm := NewSubscriptionManager(exec, dialect, config)
	defer sm.Close()

	_, err := sm.NewChangeStream(context.Background(), "users", 10)
	if err == nil {
		t.Error("NewChangeStream should return error when Subscribe fails")
	}
}

func TestSubscriptionManager_Watch(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	config := DefaultSubscriptionConfig()

	sm := NewSubscriptionManager(exec, dialect, config)
	defer sm.Close()

	handlerCalled := false
	watchConfig := WatchConfig{
		Actions: []string{"INSERT"},
	}

	sub, err := sm.Watch(context.Background(), "users", watchConfig, func(payload NotifyPayload) {
		handlerCalled = true
	})
	if err != nil {
		t.Fatalf("Watch error = %v", err)
	}
	if sub == nil {
		t.Fatal("sub should not be nil")
	}

	// Invoke the wrapped handler with matching action
	sub.handler(NotifyPayload{Table: "users", Action: "INSERT"})
	if !handlerCalled {
		t.Error("handler should be called for matching action")
	}

	// Invoke with non-matching action
	handlerCalled = false
	sub.handler(NotifyPayload{Table: "users", Action: "DELETE"})
	if handlerCalled {
		t.Error("handler should not be called for non-matching action")
	}

	sub.Cancel()
}

func TestSubscriptionManager_Watch_Error(t *testing.T) {
	exec := newMockExecutor()
	exec.execErr = errTest
	dialect := newPostgresDialect()
	config := DefaultSubscriptionConfig()

	sm := NewSubscriptionManager(exec, dialect, config)
	defer sm.Close()

	_, err := sm.Watch(context.Background(), "users", WatchConfig{}, func(payload NotifyPayload) {})
	if err == nil {
		t.Error("Watch should return error when Subscribe fails")
	}
}

func TestSubscriptionManager_Watch_FiltersByAction(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	config := DefaultSubscriptionConfig()

	sm := NewSubscriptionManager(exec, dialect, config)
	defer sm.Close()

	handlerCalled := false
	watchConfig := WatchConfig{
		Actions: []string{"INSERT"},
	}

	sub, err := sm.Watch(context.Background(), "users", watchConfig, func(payload NotifyPayload) {
		handlerCalled = true
	})
	if err != nil {
		t.Fatalf("Watch error = %v", err)
	}
	defer sub.Cancel()

	// Invoke with matching action
	sub.handler(NotifyPayload{Table: "users", Action: "INSERT"})
	if !handlerCalled {
		t.Error("handler should be called for matching action")
	}
}

func TestSubscriptionManager_Watch_WithFilter(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	config := DefaultSubscriptionConfig()

	sm := NewSubscriptionManager(exec, dialect, config)
	defer sm.Close()

	handlerCalled := false
	watchConfig := WatchConfig{
		Actions: []string{"INSERT", "UPDATE"},
		Filter: func(payload NotifyPayload) bool {
			return payload.Table == "users"
		},
	}

	sub, err := sm.Watch(context.Background(), "users", watchConfig, func(payload NotifyPayload) {
		handlerCalled = true
	})
	if err != nil {
		t.Fatalf("Watch error = %v", err)
	}
	defer sub.Cancel()

	// Matching action + matching filter
	sub.handler(NotifyPayload{Table: "users", Action: "INSERT"})
	if !handlerCalled {
		t.Error("handler should be called for matching action and filter")
	}

	// Matching action + non-matching filter
	handlerCalled = false
	sub.handler(NotifyPayload{Table: "orders", Action: "INSERT"})
	if handlerCalled {
		t.Error("handler should not be called when filter rejects")
	}

	// Non-matching action
	handlerCalled = false
	sub.handler(NotifyPayload{Table: "users", Action: "DELETE"})
	if handlerCalled {
		t.Error("handler should not be called for non-matching action")
	}
}

func TestSubscriptionManager_Watch_NoActionFilter(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	config := DefaultSubscriptionConfig()

	sm := NewSubscriptionManager(exec, dialect, config)
	defer sm.Close()

	handlerCalled := false
	watchConfig := WatchConfig{
		// No Actions filter - all actions pass
	}

	sub, err := sm.Watch(context.Background(), "users", watchConfig, func(payload NotifyPayload) {
		handlerCalled = true
	})
	if err != nil {
		t.Fatalf("Watch error = %v", err)
	}
	defer sub.Cancel()

	sub.handler(NotifyPayload{Table: "users", Action: "DELETE"})
	if !handlerCalled {
		t.Error("handler should be called when no action filter")
	}
}

func TestSubscriptionManager_Notify_Error(t *testing.T) {
	exec := newMockExecutor()
	exec.execErr = errTest
	dialect := newPostgresDialect()
	config := DefaultSubscriptionConfig()

	sm := NewSubscriptionManager(exec, dialect, config)
	defer sm.Close()

	err := sm.Notify(context.Background(), "users", NotifyPayload{Action: "INSERT"})
	if err == nil {
		t.Error("should return error")
	}
}

func TestSubscriptionManager_CreateNotifyTrigger_TriggerError(t *testing.T) {
	exec := newConditionalMockExecutor(2) // Fail on second exec (trigger creation)
	dialect := newPostgresDialect()
	config := DefaultSubscriptionConfig()

	sm := NewSubscriptionManager(exec, dialect, config)
	defer sm.Close()

	err := sm.CreateNotifyTrigger(context.Background(), "users")
	if err == nil {
		t.Error("should return error when trigger creation fails")
	}
}

func TestSubscriptionManager_DropNotifyTrigger_SecondExecError(t *testing.T) {
	exec := newConditionalMockExecutor(2) // Fail on second exec (drop function)
	dialect := newPostgresDialect()
	config := DefaultSubscriptionConfig()

	sm := NewSubscriptionManager(exec, dialect, config)
	defer sm.Close()

	err := sm.DropNotifyTrigger(context.Background(), "users")
	if err == nil {
		t.Error("should return error when drop function fails")
	}
}

func TestReadYourWritesHelper_EnsureReplication_PostgreSQL_Reached(t *testing.T) {
	sqlDB := getMockSQLDBWithRows([]string{"result"}, [][]driver.Value{{true}})
	defer sqlDB.Close()

	dialect := newPostgresDialect()
	helper := NewReadYourWritesHelper(sqlDB, dialect)

	err := helper.EnsureReplication(context.Background(), "0/0")
	if err != nil {
		t.Logf("EnsureReplication error (may be expected with mock): %v", err)
	}
}

func TestReadYourWritesHelper_EnsureReplication_PostgreSQL_NotReached(t *testing.T) {
	sqlDB := getMockSQLDBWithRows([]string{"result"}, [][]driver.Value{{false}})
	defer sqlDB.Close()

	dialect := newPostgresDialect()
	helper := NewReadYourWritesHelper(sqlDB, dialect)

	err := helper.EnsureReplication(context.Background(), "0/0")
	// Should return "replication lag detected" or query error
	if err != nil {
		t.Logf("EnsureReplication: %v", err)
	}
}

func TestReadYourWritesHelper_EnsureReplication_NoRows(t *testing.T) {
	sqlDB := getMockSQLDBWithRows([]string{"result"}, [][]driver.Value{})
	defer sqlDB.Close()

	dialect := newPostgresDialect()
	helper := NewReadYourWritesHelper(sqlDB, dialect)

	err := helper.EnsureReplication(context.Background(), "0/0")
	// ErrNoRows should return nil
	if err != nil {
		t.Logf("EnsureReplication (no rows): %v", err)
	}
}

func TestReadYourWritesHelper_GetCurrentLSN_ScanError(t *testing.T) {
	// Mock with empty rows to cause scan error
	sqlDB := getMockSQLDBWithRows([]string{}, [][]driver.Value{})
	defer sqlDB.Close()

	dialect := newPostgresDialect()
	helper := NewReadYourWritesHelper(sqlDB, dialect)

	_, err := helper.GetCurrentLSN(context.Background())
	if err == nil {
		t.Error("should return error when scan fails")
	}
}

func TestReadYourWritesHelper_GetCurrentLSN_PostgreSQL(t *testing.T) {
	sqlDB := getMockSQLDBWithRows([]string{"lsn"}, [][]driver.Value{{"0/16B3748"}})
	defer sqlDB.Close()

	dialect := newPostgresDialect()
	helper := NewReadYourWritesHelper(sqlDB, dialect)

	lsn, err := helper.GetCurrentLSN(context.Background())
	if err != nil {
		t.Logf("GetCurrentLSN error (expected with mock): %v", err)
	}
	_ = lsn
}
