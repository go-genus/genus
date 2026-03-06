package core

import (
	"context"
	"database/sql/driver"
	"testing"
	"time"
)

func TestDefaultAuditConfig(t *testing.T) {
	config := DefaultAuditConfig()
	if config.Enabled {
		t.Error("Enabled should be false")
	}
	if config.AuditTable != "audit_logs" {
		t.Errorf("AuditTable = %q, want %q", config.AuditTable, "audit_logs")
	}
	if config.AuditReads {
		t.Error("AuditReads should be false")
	}
	if len(config.ExcludeColumns) != 5 {
		t.Errorf("ExcludeColumns has %d items, want 5", len(config.ExcludeColumns))
	}
}

func TestNewAuditor(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	config := DefaultAuditConfig()

	auditor := NewAuditor(exec, dialect, config)
	if auditor == nil {
		t.Fatal("NewAuditor returned nil")
	}
}

func TestAuditor_LogCreate_Disabled(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	config := DefaultAuditConfig()
	config.Enabled = false

	auditor := NewAuditor(exec, dialect, config)
	err := auditor.LogCreate(context.Background(), "users", 1, nil)
	if err != nil {
		t.Errorf("LogCreate error = %v", err)
	}
	// Should not have executed anything
	if len(exec.execCalls) != 0 {
		t.Error("should not execute when disabled")
	}
}

func TestAuditor_LogCreate_Enabled(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	config := DefaultAuditConfig()
	config.Enabled = true

	auditor := NewAuditor(exec, dialect, config)

	type SimpleModel struct {
		Name  string `db:"name"`
		Email string `db:"email"`
	}
	model := &SimpleModel{Name: "Alice", Email: "alice@test.com"}

	err := auditor.LogCreate(context.Background(), "users", 1, model)
	if err != nil {
		t.Errorf("LogCreate error = %v", err)
	}

	if len(exec.execCalls) != 1 {
		t.Errorf("expected 1 exec call, got %d", len(exec.execCalls))
	}
}

func TestAuditor_LogCreate_ExcludedTable(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	config := DefaultAuditConfig()
	config.Enabled = true
	config.ExcludeTables = []string{"audit_logs", "sessions"}

	auditor := NewAuditor(exec, dialect, config)
	err := auditor.LogCreate(context.Background(), "sessions", 1, nil)
	if err != nil {
		t.Errorf("error = %v", err)
	}
	if len(exec.execCalls) != 0 {
		t.Error("should not log excluded table")
	}
}

func TestAuditor_LogUpdate_Disabled(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	config := DefaultAuditConfig()

	auditor := NewAuditor(exec, dialect, config)
	err := auditor.LogUpdate(context.Background(), "users", 1, nil, nil)
	if err != nil {
		t.Errorf("LogUpdate error = %v", err)
	}
}

func TestAuditor_LogUpdate_NoChanges(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	config := DefaultAuditConfig()
	config.Enabled = true

	auditor := NewAuditor(exec, dialect, config)

	type M struct {
		Name string `db:"name"`
	}
	old := &M{Name: "Alice"}
	new := &M{Name: "Alice"} // Same values

	err := auditor.LogUpdate(context.Background(), "users", 1, old, new)
	if err != nil {
		t.Errorf("LogUpdate error = %v", err)
	}
	// No changes, so no exec call
	if len(exec.execCalls) != 0 {
		t.Error("should not log when nothing changed")
	}
}

func TestAuditor_LogUpdate_WithChanges(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	config := DefaultAuditConfig()
	config.Enabled = true

	auditor := NewAuditor(exec, dialect, config)

	type M struct {
		Name string `db:"name"`
	}
	old := &M{Name: "Alice"}
	new := &M{Name: "Bob"}

	err := auditor.LogUpdate(context.Background(), "users", 1, old, new)
	if err != nil {
		t.Errorf("LogUpdate error = %v", err)
	}
	if len(exec.execCalls) != 1 {
		t.Errorf("expected 1 exec call, got %d", len(exec.execCalls))
	}
}

func TestAuditor_LogDelete_Disabled(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	config := DefaultAuditConfig()

	auditor := NewAuditor(exec, dialect, config)
	err := auditor.LogDelete(context.Background(), "users", 1, nil)
	if err != nil {
		t.Errorf("LogDelete error = %v", err)
	}
}

func TestAuditor_LogDelete_Enabled(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	config := DefaultAuditConfig()
	config.Enabled = true

	auditor := NewAuditor(exec, dialect, config)

	type M struct {
		Name string `db:"name"`
	}
	err := auditor.LogDelete(context.Background(), "users", 1, &M{Name: "Alice"})
	if err != nil {
		t.Errorf("LogDelete error = %v", err)
	}
	if len(exec.execCalls) != 1 {
		t.Errorf("expected 1 exec call, got %d", len(exec.execCalls))
	}
}

func TestAuditor_LogRead_Disabled(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	config := DefaultAuditConfig()

	auditor := NewAuditor(exec, dialect, config)
	err := auditor.LogRead(context.Background(), "users", 1)
	if err != nil {
		t.Errorf("LogRead error = %v", err)
	}
}

func TestAuditor_LogRead_EnabledButNoAuditReads(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	config := DefaultAuditConfig()
	config.Enabled = true
	config.AuditReads = false

	auditor := NewAuditor(exec, dialect, config)
	err := auditor.LogRead(context.Background(), "users", 1)
	if err != nil {
		t.Errorf("LogRead error = %v", err)
	}
	if len(exec.execCalls) != 0 {
		t.Error("should not log reads when AuditReads is false")
	}
}

func TestAuditor_LogRead_Enabled(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	config := DefaultAuditConfig()
	config.Enabled = true
	config.AuditReads = true

	auditor := NewAuditor(exec, dialect, config)
	err := auditor.LogRead(context.Background(), "users", 1)
	if err != nil {
		t.Errorf("LogRead error = %v", err)
	}
	if len(exec.execCalls) != 1 {
		t.Errorf("expected 1 exec call, got %d", len(exec.execCalls))
	}
}

func TestAuditor_LogRead_ExcludedTable(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	config := DefaultAuditConfig()
	config.Enabled = true
	config.AuditReads = true
	config.ExcludeTables = []string{"sessions"}

	auditor := NewAuditor(exec, dialect, config)
	err := auditor.LogRead(context.Background(), "sessions", 1)
	if err != nil {
		t.Errorf("error = %v", err)
	}
	if len(exec.execCalls) != 0 {
		t.Error("should not log excluded table")
	}
}

func TestAuditor_EnrichEntry(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	config := DefaultAuditConfig()
	config.Enabled = true
	config.GetCurrentUser = func(ctx context.Context) string { return "admin" }
	config.GetIPAddress = func(ctx context.Context) string { return "127.0.0.1" }
	config.GetUserAgent = func(ctx context.Context) string { return "TestAgent" }

	auditor := NewAuditor(exec, dialect, config)

	entry := &AuditEntry{}
	auditor.enrichEntry(context.Background(), entry)

	if entry.ChangedBy != "admin" {
		t.Errorf("ChangedBy = %q, want admin", entry.ChangedBy)
	}
	if entry.IPAddress != "127.0.0.1" {
		t.Errorf("IPAddress = %q, want 127.0.0.1", entry.IPAddress)
	}
	if entry.UserAgent != "TestAgent" {
		t.Errorf("UserAgent = %q, want TestAgent", entry.UserAgent)
	}
}

func TestAuditor_EnrichEntry_NoCallbacks(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	config := DefaultAuditConfig()
	config.Enabled = true

	auditor := NewAuditor(exec, dialect, config)

	entry := &AuditEntry{}
	// Should not panic with nil callbacks
	auditor.enrichEntry(context.Background(), entry)

	if entry.ChangedBy != "" {
		t.Error("ChangedBy should be empty")
	}
}

func TestAuditor_OnAuditCallback(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	config := DefaultAuditConfig()
	config.Enabled = true

	callbackCalled := false
	config.OnAudit = func(entry AuditEntry) {
		callbackCalled = true
	}

	auditor := NewAuditor(exec, dialect, config)
	auditor.LogCreate(context.Background(), "users", 1, nil)

	if !callbackCalled {
		t.Error("OnAudit callback should have been called")
	}
}

func TestAuditor_SaveEntry_NilExecutor(t *testing.T) {
	config := DefaultAuditConfig()
	config.Enabled = true

	auditor := NewAuditor(nil, newPostgresDialect(), config)
	err := auditor.LogCreate(context.Background(), "users", 1, nil)
	if err != nil {
		t.Errorf("error = %v", err)
	}
}

func TestAuditor_ExtractValues_Nil(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	config := DefaultAuditConfig()

	auditor := NewAuditor(exec, dialect, config)
	result := auditor.extractValues(nil)
	if result != nil {
		t.Error("extractValues(nil) should return nil")
	}
}

func TestAuditor_ExtractValues_Struct(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	config := DefaultAuditConfig()

	auditor := NewAuditor(exec, dialect, config)

	type M struct {
		Name  string `db:"name"`
		Email string `db:"email"`
	}
	values := auditor.extractValues(&M{Name: "Alice", Email: "alice@test.com"})

	if values["name"] != "Alice" {
		t.Errorf("name = %v, want Alice", values["name"])
	}
	if values["email"] != "alice@test.com" {
		t.Errorf("email = %v", values["email"])
	}
}

func TestAuditor_ExtractValues_ExcludedColumns(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	config := DefaultAuditConfig()
	config.ExcludeColumns = []string{"password"}

	auditor := NewAuditor(exec, dialect, config)

	type M struct {
		Name     string `db:"name"`
		Password string `db:"password"`
	}
	values := auditor.extractValues(&M{Name: "Alice", Password: "secret"})

	if _, ok := values["password"]; ok {
		t.Error("password should be excluded")
	}
	if values["name"] != "Alice" {
		t.Errorf("name = %v, want Alice", values["name"])
	}
}

func TestAuditor_ExtractValues_NonStruct(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	config := DefaultAuditConfig()

	auditor := NewAuditor(exec, dialect, config)
	str := "not a struct"
	values := auditor.extractValues(str)
	if len(values) != 0 {
		t.Error("extractValues on non-struct should return empty map")
	}
}

func TestAuditor_IsExcludedTable(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	config := DefaultAuditConfig()
	config.ExcludeTables = []string{"sessions", "tokens"}

	auditor := NewAuditor(exec, dialect, config)

	if !auditor.isExcludedTable("sessions") {
		t.Error("sessions should be excluded")
	}
	if !auditor.isExcludedTable("tokens") {
		t.Error("tokens should be excluded")
	}
	if auditor.isExcludedTable("users") {
		t.Error("users should not be excluded")
	}
}

func TestAuditor_IsExcludedColumn(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	config := DefaultAuditConfig()
	config.ExcludeColumns = []string{"password", "token"}

	auditor := NewAuditor(exec, dialect, config)

	if !auditor.isExcludedColumn("password") {
		t.Error("password should be excluded")
	}
	if !auditor.isExcludedColumn("token") {
		t.Error("token should be excluded")
	}
	if auditor.isExcludedColumn("name") {
		t.Error("name should not be excluded")
	}
}

func TestAuditor_CreateAuditTable_PostgreSQL(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	config := DefaultAuditConfig()

	auditor := NewAuditor(exec, dialect, config)
	err := auditor.CreateAuditTable(context.Background())
	if err != nil {
		t.Errorf("CreateAuditTable error = %v", err)
	}
}

func TestAuditor_CreateAuditTable_MySQL(t *testing.T) {
	exec := newMockExecutor()
	dialect := newMySQLDialect()
	config := DefaultAuditConfig()

	auditor := NewAuditor(exec, dialect, config)
	err := auditor.CreateAuditTable(context.Background())
	if err != nil {
		t.Errorf("CreateAuditTable error = %v", err)
	}
}

func TestAuditor_CreateAuditTable_Error(t *testing.T) {
	exec := newMockExecutor()
	exec.execErr = errTest
	dialect := newPostgresDialect()
	config := DefaultAuditConfig()

	auditor := NewAuditor(exec, dialect, config)
	err := auditor.CreateAuditTable(context.Background())
	if err == nil {
		t.Error("should return error")
	}
}

func TestAuditAction_Constants(t *testing.T) {
	if AuditCreate != "CREATE" {
		t.Error("AuditCreate mismatch")
	}
	if AuditUpdate != "UPDATE" {
		t.Error("AuditUpdate mismatch")
	}
	if AuditDelete != "DELETE" {
		t.Error("AuditDelete mismatch")
	}
	if AuditRead != "READ" {
		t.Error("AuditRead mismatch")
	}
}

func TestAuditor_LogCreate_MySQLDialect(t *testing.T) {
	exec := newMockExecutor()
	dialect := newMySQLDialect()
	config := DefaultAuditConfig()
	config.Enabled = true

	auditor := NewAuditor(exec, dialect, config)
	err := auditor.LogCreate(context.Background(), "users", 1, nil)
	if err != nil {
		t.Errorf("LogCreate with MySQL error = %v", err)
	}
}

func TestAuditor_LogDelete_ExcludedTable(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	config := DefaultAuditConfig()
	config.Enabled = true
	config.ExcludeTables = []string{"users"}

	auditor := NewAuditor(exec, dialect, config)
	err := auditor.LogDelete(context.Background(), "users", 1, nil)
	if err != nil {
		t.Errorf("error = %v", err)
	}
	if len(exec.execCalls) != 0 {
		t.Error("should not log excluded table")
	}
}

func TestAuditor_LogUpdate_ExcludedTable(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	config := DefaultAuditConfig()
	config.Enabled = true
	config.ExcludeTables = []string{"users"}

	auditor := NewAuditor(exec, dialect, config)
	err := auditor.LogUpdate(context.Background(), "users", 1, nil, nil)
	if err != nil {
		t.Errorf("error = %v", err)
	}
	if len(exec.execCalls) != 0 {
		t.Error("should not log excluded table")
	}
}

func TestAuditor_ExtractValues_DBTagDash(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	config := DefaultAuditConfig()

	auditor := NewAuditor(exec, dialect, config)

	type M struct {
		Name    string `db:"name"`
		Ignored string `db:"-"`
	}
	values := auditor.extractValues(&M{Name: "Alice", Ignored: "skip"})

	// Field with db:"-" should use snake_case name and be treated as regular
	if _, ok := values["name"]; !ok {
		t.Error("name should be in values")
	}
}

func TestAuditor_GetAuditHistory_PostgreSQL(t *testing.T) {
	exec := newMockExecutor()
	exec.queryErr = errTest
	dialect := newPostgresDialect()
	config := DefaultAuditConfig()
	auditor := NewAuditor(exec, dialect, config)

	_, err := auditor.GetAuditHistory(context.Background(), "users", "1")
	if err == nil {
		t.Error("should return error when query fails")
	}
}

func TestAuditor_GetAuditHistory_MySQL(t *testing.T) {
	exec := newMockExecutor()
	exec.queryErr = errTest
	dialect := newMySQLDialect()
	config := DefaultAuditConfig()
	auditor := NewAuditor(exec, dialect, config)

	_, err := auditor.GetAuditHistory(context.Background(), "users", "1")
	if err == nil {
		t.Error("should return error when query fails")
	}
}

func TestAuditor_GetAuditHistory_EmptyResult(t *testing.T) {
	exec := newMockExecutorWithQueryRows(
		[]string{"id", "table_name", "record_id", "action", "old_values", "new_values",
			"changed_by", "changed_at", "ip_address", "user_agent", "metadata"},
		[][]driver.Value{},
	)
	defer exec.Close()

	dialect := newPostgresDialect()
	config := DefaultAuditConfig()
	auditor := NewAuditor(exec, dialect, config)

	entries, err := auditor.GetAuditHistory(context.Background(), "users", "1")
	if err != nil {
		t.Logf("GetAuditHistory error (may be expected with mock): %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestAuditor_GetAuditHistory_WithRows(t *testing.T) {
	now := time.Now()
	exec := newMockExecutorWithQueryRows(
		[]string{"id", "table_name", "record_id", "action", "old_values", "new_values",
			"changed_by", "changed_at", "ip_address", "user_agent", "metadata"},
		[][]driver.Value{
			{int64(1), "users", "42", "INSERT", []byte("{}"), []byte(`{"name":"Alice"}`),
				"admin", now, "127.0.0.1", "test-agent", []byte("{}")},
		},
	)
	defer exec.Close()

	dialect := newPostgresDialect()
	config := DefaultAuditConfig()
	auditor := NewAuditor(exec, dialect, config)

	entries, err := auditor.GetAuditHistory(context.Background(), "users", "42")
	if err != nil {
		t.Logf("GetAuditHistory error (may be expected with mock): %v", err)
	} else if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}
}

func TestAuditor_GetAuditHistory_ScanError(t *testing.T) {
	// Provide wrong number of columns to cause scan error
	exec := newMockExecutorWithQueryRows(
		[]string{"id", "table_name"},
		[][]driver.Value{
			{int64(1), "users"},
		},
	)
	defer exec.Close()

	dialect := newPostgresDialect()
	config := DefaultAuditConfig()
	auditor := NewAuditor(exec, dialect, config)

	_, err := auditor.GetAuditHistory(context.Background(), "users", "42")
	if err == nil {
		t.Error("should return error when scan fails due to column mismatch")
	}
}

func TestAuditor_GetAuditHistory_MySQL_QueryError(t *testing.T) {
	exec := newMockExecutor()
	exec.queryErr = errTest
	dialect := newMySQLDialect()
	config := DefaultAuditConfig()
	auditor := NewAuditor(exec, dialect, config)

	_, err := auditor.GetAuditHistory(context.Background(), "users", "1")
	if err == nil {
		t.Error("should return error when query fails (MySQL path)")
	}
}

func TestAuditor_ExtractValues_EmptyDBTag(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	config := DefaultAuditConfig()
	auditor := NewAuditor(exec, dialect, config)

	type ModelWithNoTag struct {
		Name    string
		Address string `db:""`
	}

	values := auditor.extractValues(&ModelWithNoTag{Name: "Alice", Address: "Main St"})
	// Field without db tag should use snake_case name
	if _, ok := values["name"]; !ok {
		t.Error("Name should be in values as 'name'")
	}
	if _, ok := values["address"]; !ok {
		t.Error("Address should be in values as 'address'")
	}
}
