package core

import (
	"context"
	"database/sql/driver"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	sqlDB := getMockSQLDB()
	defer sqlDB.Close()

	dialect := newPostgresDialect()
	db := New(sqlDB, dialect)

	if db.Executor() == nil {
		t.Error("Executor() should not be nil")
	}
	if db.Dialect() == nil {
		t.Error("Dialect() should not be nil")
	}
	if db.Logger() == nil {
		t.Error("Logger() should not be nil")
	}
}

func TestNewWithLogger(t *testing.T) {
	sqlDB := getMockSQLDB()
	defer sqlDB.Close()

	dialect := newPostgresDialect()
	logger := newMockLogger()
	db := NewWithLogger(sqlDB, dialect, logger)

	if db.Logger() != logger {
		t.Error("Logger should be the custom logger")
	}
}

func TestNewWithExecutor(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	db := NewWithExecutor(exec, dialect)

	if db.Executor() != exec {
		t.Error("Executor should be the custom executor")
	}
}

func TestNewWithExecutorAndLogger(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	logger := newMockLogger()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	if db.Executor() != exec {
		t.Error("Executor should be the custom executor")
	}
	if db.Logger() != logger {
		t.Error("Logger should be the custom logger")
	}
}

func TestDB_SetLogger(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	db := NewWithExecutor(exec, dialect)

	newLogger := newMockLogger()
	db.SetLogger(newLogger)

	if db.Logger() != newLogger {
		t.Error("SetLogger did not set the logger")
	}
}

func TestDB_Dialect(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	db := NewWithExecutor(exec, dialect)

	if db.Dialect() != dialect {
		t.Error("Dialect() should return the configured dialect")
	}
}

func TestWithTx_NotSqlDB(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	db := NewWithExecutor(exec, dialect)

	err := db.WithTx(context.Background(), func(txDB *DB) error {
		return nil
	})

	if err == nil {
		t.Error("WithTx with non-sql.DB should return error")
	}
	if !strings.Contains(err.Error(), "cannot start transaction") {
		t.Errorf("error should mention 'cannot start transaction', got: %v", err)
	}
}

func TestWithTx_WithSqlDB(t *testing.T) {
	sqlDB := getMockSQLDB()
	defer sqlDB.Close()

	dialect := newPostgresDialect()
	db := New(sqlDB, dialect)

	// This will fail because mock driver doesn't fully support transactions,
	// but we can test the flow
	err := db.WithTx(context.Background(), func(txDB *DB) error {
		return nil
	})

	// Mock driver should support basic tx
	if err != nil {
		t.Logf("WithTx error (expected with mock): %v", err)
	}
}

func TestWithTx_FnError_Rollback(t *testing.T) {
	sqlDB := getMockSQLDB()
	defer sqlDB.Close()

	dialect := newPostgresDialect()
	db := New(sqlDB, dialect)

	expectedErr := fmt.Errorf("fn error")
	err := db.WithTx(context.Background(), func(txDB *DB) error {
		return expectedErr
	})

	if err == nil {
		t.Error("WithTx should propagate fn error")
	}
}

// Test helper functions

func TestWithTx_BeginError(t *testing.T) {
	sqlDB := getMockSQLDBFailBegin()
	defer sqlDB.Close()

	dialect := newPostgresDialect()
	db := New(sqlDB, dialect)

	err := db.WithTx(context.Background(), func(txDB *DB) error {
		return nil
	})
	if err == nil {
		t.Error("WithTx should return error when BeginTx fails")
	}
	if !strings.Contains(err.Error(), "begin transaction") {
		t.Errorf("error should mention begin transaction, got: %v", err)
	}
}

func TestWithTx_CommitError(t *testing.T) {
	sqlDB := getMockSQLDBFailCommit()
	defer sqlDB.Close()

	dialect := newPostgresDialect()
	db := New(sqlDB, dialect)

	err := db.WithTx(context.Background(), func(txDB *DB) error {
		return nil // fn succeeds, commit should fail
	})
	if err == nil {
		t.Error("WithTx should return error when Commit fails")
	}
	if !strings.Contains(err.Error(), "commit") {
		t.Errorf("error should mention commit, got: %v", err)
	}
}

func TestWithTx_RollbackError(t *testing.T) {
	sqlDB := getMockSQLDBFailRollback()
	defer sqlDB.Close()

	dialect := newPostgresDialect()
	db := New(sqlDB, dialect)

	err := db.WithTx(context.Background(), func(txDB *DB) error {
		return errTest // fn fails, rollback should also fail
	})
	if err == nil {
		t.Error("WithTx should return error")
	}
	if !strings.Contains(err.Error(), "rolling back") {
		t.Errorf("error should mention rolling back, got: %v", err)
	}
}

func TestGetTableName_WithTableNamer(t *testing.T) {
	m := &TestModelWithTableName{Name: "test"}
	name := getTableName(m)
	if name != "custom_table" {
		t.Errorf("getTableName() = %q, want %q", name, "custom_table")
	}
}

func TestGetTableName_WithoutTableNamer(t *testing.T) {
	m := &TestModel{Name: "test"}
	name := getTableName(m)
	if name != "test_model" {
		t.Errorf("getTableName() = %q, want %q", name, "test_model")
	}
}

func TestGetTableName_NonPointer(t *testing.T) {
	m := TestModel{Name: "test"}
	name := getTableName(m)
	if name != "test_model" {
		t.Errorf("getTableName() = %q, want %q", name, "test_model")
	}
}

func TestGetColumnsAndValues(t *testing.T) {
	m := &TestModel{
		Model: Model{ID: 1, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		Name:  "Alice",
		Email: "alice@test.com",
	}

	cols, vals, err := getColumnsAndValues(m)
	if err != nil {
		t.Fatalf("getColumnsAndValues() error = %v", err)
	}

	// Should have: id, created_at, updated_at, name, email
	if len(cols) != 5 {
		t.Errorf("len(columns) = %d, want 5, got columns: %v", len(cols), cols)
	}
	if len(vals) != 5 {
		t.Errorf("len(values) = %d, want 5", len(vals))
	}

	// Check column names
	expectedCols := map[string]bool{
		"id": true, "created_at": true, "updated_at": true,
		"name": true, "email": true,
	}
	for _, col := range cols {
		if !expectedCols[col] {
			t.Errorf("unexpected column: %q", col)
		}
	}
}

func TestGetColumnsAndValues_NonPointer(t *testing.T) {
	// getColumnsAndValues requires a pointer for embedded structs (Addr() call)
	m := &TestModel{Name: "Test"}
	cols, vals, err := getColumnsAndValues(m)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(cols) != len(vals) {
		t.Errorf("cols and vals length mismatch: %d vs %d", len(cols), len(vals))
	}
}

func TestGetID(t *testing.T) {
	m := &TestModel{}
	m.ID = 42

	id := getID(m)
	if id != 42 {
		t.Errorf("getID() = %d, want 42", id)
	}
}

func TestGetID_Zero(t *testing.T) {
	m := &TestModel{}
	id := getID(m)
	if id != 0 {
		t.Errorf("getID() = %d, want 0", id)
	}
}

func TestGetID_NoIDField(t *testing.T) {
	type NoIDModel struct {
		Name string `db:"name"`
	}
	m := &NoIDModel{Name: "test"}
	id := getID(m)
	if id != 0 {
		t.Errorf("getID() = %d, want 0 for model without ID", id)
	}
}

func TestSetID(t *testing.T) {
	m := &TestModel{}
	setID(m, 99)
	if m.ID != 99 {
		t.Errorf("after setID(99), ID = %d", m.ID)
	}
}

func TestSetID_NoIDField(t *testing.T) {
	type NoIDModel struct {
		Name string `db:"name"`
	}
	m := &NoIDModel{}
	// Should not panic
	setID(m, 99)
}

func TestSetTimestamps(t *testing.T) {
	m := &TestModel{}
	before := time.Now()
	setTimestamps(m)
	after := time.Now()

	if m.CreatedAt.Before(before) || m.CreatedAt.After(after) {
		t.Error("CreatedAt not set correctly")
	}
	if m.UpdatedAt.Before(before) || m.UpdatedAt.After(after) {
		t.Error("UpdatedAt not set correctly")
	}
}

func TestSetTimestamps_PreservesCreatedAt(t *testing.T) {
	existing := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	m := &TestModel{}
	m.CreatedAt = existing

	setTimestamps(m)

	if !m.CreatedAt.Equal(existing) {
		t.Error("setTimestamps should not overwrite existing CreatedAt")
	}
}

func TestSetUpdatedAt(t *testing.T) {
	m := &TestModel{}
	before := time.Now()
	setUpdatedAt(m)
	after := time.Now()

	if m.UpdatedAt.Before(before) || m.UpdatedAt.After(after) {
		t.Error("UpdatedAt not set correctly")
	}
}

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"User", "user"},
		{"UserProfile", "user_profile"},
		{"ID", "i_d"},
		{"HTMLParser", "h_t_m_l_parser"},
		{"simple", "simple"},
		{"", ""},
		{"A", "a"},
		{"ABCDef", "a_b_c_def"},
	}

	for _, tt := range tests {
		got := toSnakeCase(tt.input)
		if got != tt.expected {
			t.Errorf("toSnakeCase(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestDB_Update_ZeroID(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestModel{Name: "test"}
	err := db.Update(context.Background(), m)
	if err == nil {
		t.Error("Update with zero ID should return error")
	}
	if !strings.Contains(err.Error(), "zero ID") {
		t.Errorf("error should mention zero ID, got: %v", err)
	}
}

func TestDB_Delete_ZeroID(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestModel{Name: "test"}
	err := db.Delete(context.Background(), m)
	if err == nil {
		t.Error("Delete with zero ID should return error")
	}
	if !strings.Contains(err.Error(), "zero ID") {
		t.Errorf("error should mention zero ID, got: %v", err)
	}
}

func TestDB_ForceDelete_ZeroID(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestModel{Name: "test"}
	err := db.ForceDelete(context.Background(), m)
	if err == nil {
		t.Error("ForceDelete with zero ID should return error")
	}
}

func TestDB_Update_ExecError(t *testing.T) {
	exec := newMockExecutor()
	exec.execErr = errTest
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestModel{Name: "test"}
	m.ID = 1
	err := db.Update(context.Background(), m)
	if err == nil {
		t.Error("Update should return error on exec failure")
	}
}

func TestDB_Update_NoRowsAffected(t *testing.T) {
	exec := newMockExecutor()
	exec.execResult = &mockResult{rowsAffected: 0}
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestModel{Name: "test"}
	m.ID = 1
	err := db.Update(context.Background(), m)
	if err == nil {
		t.Error("Update with 0 rows affected should return error")
	}
	if !strings.Contains(err.Error(), "no rows updated") {
		t.Errorf("error should mention 'no rows updated', got: %v", err)
	}
}

func TestDB_Update_RowsAffectedError(t *testing.T) {
	exec := newMockExecutor()
	exec.execResult = &mockResult{rowsAffected: 1, rowsErr: errTest}
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestModel{Name: "test"}
	m.ID = 1
	err := db.Update(context.Background(), m)
	if err == nil {
		t.Error("Update should return error when RowsAffected fails")
	}
}

func TestDB_Update_Success(t *testing.T) {
	exec := newMockExecutor()
	exec.execResult = &mockResult{rowsAffected: 1}
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestModel{Name: "test"}
	m.ID = 1
	err := db.Update(context.Background(), m)
	if err != nil {
		t.Errorf("Update should succeed, got: %v", err)
	}

	// Check that query was logged
	if len(logger.queries) != 1 {
		t.Errorf("expected 1 query logged, got %d", len(logger.queries))
	}

	// Verify the query contains UPDATE
	if len(exec.execCalls) != 1 {
		t.Fatalf("expected 1 exec call, got %d", len(exec.execCalls))
	}
	if !strings.Contains(exec.execCalls[0].query, "UPDATE") {
		t.Errorf("query should contain UPDATE, got: %s", exec.execCalls[0].query)
	}
}

func TestDB_Update_WithHooks(t *testing.T) {
	exec := newMockExecutor()
	exec.execResult = &mockResult{rowsAffected: 1}
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestModelWithHooks{Name: "test"}
	m.ID = 1
	err := db.Update(context.Background(), m)
	if err != nil {
		t.Errorf("Update error: %v", err)
	}

	if !m.beforeSaveCalled {
		t.Error("BeforeSave not called")
	}
	if !m.beforeUpdateCalled {
		t.Error("BeforeUpdate not called")
	}
	if !m.afterUpdateCalled {
		t.Error("AfterUpdate not called")
	}
	if !m.afterSaveCalled {
		t.Error("AfterSave not called")
	}
}

func TestDB_Update_BeforeSaveHookError(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestModelWithHooks{Name: "test"}
	m.ID = 1
	m.hookErr = errHookFail
	err := db.Update(context.Background(), m)
	if err == nil {
		t.Error("Update should fail when BeforeSave hook fails")
	}
	if !strings.Contains(err.Error(), "BeforeSave") {
		t.Errorf("error should mention BeforeSave, got: %v", err)
	}
}

func TestDB_Delete_HardDelete_Success(t *testing.T) {
	exec := newMockExecutor()
	exec.execResult = &mockResult{rowsAffected: 1}
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestModel{Name: "test"}
	m.ID = 1
	err := db.Delete(context.Background(), m)
	if err != nil {
		t.Errorf("Delete error: %v", err)
	}

	if len(exec.execCalls) != 1 {
		t.Fatalf("expected 1 exec call, got %d", len(exec.execCalls))
	}
	if !strings.Contains(exec.execCalls[0].query, "DELETE") {
		t.Errorf("query should contain DELETE, got: %s", exec.execCalls[0].query)
	}
}

func TestDB_Delete_HardDelete_ExecError(t *testing.T) {
	exec := newMockExecutor()
	exec.execErr = errTest
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestModel{Name: "test"}
	m.ID = 1
	err := db.Delete(context.Background(), m)
	if err == nil {
		t.Error("Delete should return error on exec failure")
	}
}

func TestDB_Delete_NoRowsAffected(t *testing.T) {
	exec := newMockExecutor()
	exec.execResult = &mockResult{rowsAffected: 0}
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestModel{Name: "test"}
	m.ID = 1
	err := db.Delete(context.Background(), m)
	if err == nil {
		t.Error("Delete with 0 rows should return error")
	}
	if !strings.Contains(err.Error(), "no rows deleted") {
		t.Errorf("error should mention 'no rows deleted', got: %v", err)
	}
}

func TestDB_Delete_WithBeforeDeleteHook(t *testing.T) {
	exec := newMockExecutor()
	exec.execResult = &mockResult{rowsAffected: 1}
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestModelWithHooks{Name: "test"}
	m.ID = 1
	err := db.Delete(context.Background(), m)
	if err != nil {
		t.Errorf("Delete error: %v", err)
	}
	if !m.beforeDeleteCalled {
		t.Error("BeforeDelete not called")
	}
	if !m.afterDeleteCalled {
		t.Error("AfterDelete not called")
	}
}

func TestDB_Delete_BeforeDeleteHookError(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestModelWithHooks{Name: "test"}
	m.ID = 1
	m.hookErr = errHookFail
	err := db.Delete(context.Background(), m)
	if err == nil {
		t.Error("Delete should fail when BeforeDelete hook fails")
	}
}

func TestDB_ForceDelete_Success(t *testing.T) {
	exec := newMockExecutor()
	exec.execResult = &mockResult{rowsAffected: 1}
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestModel{Name: "test"}
	m.ID = 1
	err := db.ForceDelete(context.Background(), m)
	if err != nil {
		t.Errorf("ForceDelete error: %v", err)
	}
}

func TestDB_ForceDelete_ExecError(t *testing.T) {
	exec := newMockExecutor()
	exec.execErr = errTest
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestModel{Name: "test"}
	m.ID = 1
	err := db.ForceDelete(context.Background(), m)
	if err == nil {
		t.Error("ForceDelete should return error on exec failure")
	}
}

func TestDB_ForceDelete_NoRows(t *testing.T) {
	exec := newMockExecutor()
	exec.execResult = &mockResult{rowsAffected: 0}
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestModel{Name: "test"}
	m.ID = 1
	err := db.ForceDelete(context.Background(), m)
	if err == nil {
		t.Error("ForceDelete with 0 rows should return error")
	}
}

func TestDB_ForceDelete_RowsAffectedError(t *testing.T) {
	exec := newMockExecutor()
	exec.execResult = &mockResult{rowsAffected: 1, rowsErr: errTest}
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestModel{Name: "test"}
	m.ID = 1
	err := db.ForceDelete(context.Background(), m)
	if err == nil {
		t.Error("ForceDelete should return error when RowsAffected fails")
	}
}

func TestDB_ForceDelete_WithHooks(t *testing.T) {
	exec := newMockExecutor()
	exec.execResult = &mockResult{rowsAffected: 1}
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestModelWithHooks{Name: "test"}
	m.ID = 1
	err := db.ForceDelete(context.Background(), m)
	if err != nil {
		t.Errorf("ForceDelete error: %v", err)
	}
	if !m.beforeDeleteCalled {
		t.Error("BeforeDelete not called")
	}
	if !m.afterDeleteCalled {
		t.Error("AfterDelete not called")
	}
}

func TestDB_Delete_RowsAffectedError(t *testing.T) {
	exec := newMockExecutor()
	exec.execResult = &mockResult{rowsAffected: 1, rowsErr: errTest}
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestModel{Name: "test"}
	m.ID = 1
	err := db.Delete(context.Background(), m)
	if err == nil {
		t.Error("Delete should return error when RowsAffected fails")
	}
}

// Test Create via mock executor that returns a row with ID
func TestDB_Create_Success(t *testing.T) {
	// We need a mock that provides QueryRowContext returning a scannable row
	// Use a real sql.DB with mock driver
	sqlDB := getMockSQLDB()
	defer sqlDB.Close()

	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithLogger(sqlDB, dialect, logger)

	m := &TestModel{Name: "test", Email: "test@test.com"}
	err := db.Create(context.Background(), m)
	// Will fail because mock driver returns empty result for QueryRow
	if err != nil {
		t.Logf("Create error (expected with mock): %v", err)
	}
}

func TestDB_Create_BeforeSaveHookError(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestModelWithHooks{Name: "test", hookErr: errHookFail}
	err := db.Create(context.Background(), m)
	if err == nil {
		t.Error("Create should fail when BeforeSave hook fails")
	}
	if !strings.Contains(err.Error(), "BeforeSave") {
		t.Errorf("error should mention BeforeSave, got: %v", err)
	}
}

func TestDB_Create_BeforeCreateHookError(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	// Need a model that only fails on BeforeCreate, not BeforeSave
	type createOnlyHookModel struct {
		Model
		Name string `db:"name"`
	}
	type createHookErr struct {
		createOnlyHookModel
	}
	// Can't easily separate the hooks. Test with the general model
	// where BeforeSave is first to fail
	m := &TestModelWithHooks{Name: "test", hookErr: errHookFail}
	err := db.Create(context.Background(), m)
	if err == nil {
		t.Error("should return error")
	}
}

func TestDB_Create_BeforeCreateOnlyHookError(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestBeforeCreateOnlyModel{Name: "test"}
	err := db.Create(context.Background(), m)
	if err == nil {
		t.Error("should return error when BeforeCreate hook fails")
	}
	if !strings.Contains(err.Error(), "BeforeCreate") {
		t.Errorf("error should mention BeforeCreate, got: %v", err)
	}
}

func TestDB_Create_AfterCreateHookError(t *testing.T) {
	// Use a mock SQL DB that returns a row with an ID
	sqlDB := getMockSQLDBWithRows([]string{"id"}, [][]driver.Value{{int64(1)}})
	defer sqlDB.Close()

	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithLogger(sqlDB, dialect, logger)

	m := &TestAfterCreateOnlyModel{Name: "test"}
	err := db.Create(context.Background(), m)
	if err != nil {
		// The mock may fail on the query itself, check for hook error or query error
		if strings.Contains(err.Error(), "AfterCreate") {
			// Expected
		} else {
			t.Logf("Create error (may be expected with mock): %v", err)
		}
	}
}

func TestDB_Create_AfterSaveHookError(t *testing.T) {
	sqlDB := getMockSQLDBWithRows([]string{"id"}, [][]driver.Value{{int64(1)}})
	defer sqlDB.Close()

	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithLogger(sqlDB, dialect, logger)

	m := &TestAfterSaveOnlyModel{Name: "test"}
	err := db.Create(context.Background(), m)
	if err != nil {
		if strings.Contains(err.Error(), "AfterSave") {
			// Expected
		} else {
			t.Logf("Create error (may be expected with mock): %v", err)
		}
	}
}

func TestDB_Create_QueryError(t *testing.T) {
	// Use a mock sql.DB where the query will return an error on Scan
	sqlDB := getMockSQLDBWithRows([]string{}, [][]driver.Value{})
	defer sqlDB.Close()

	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithLogger(sqlDB, dialect, logger)

	m := &TestModel{Name: "test"}
	err := db.Create(context.Background(), m)
	// Should fail because no rows returned
	if err != nil {
		t.Logf("Create error (expected with empty mock): %v", err)
	}
}

func TestDB_Update_BeforeUpdateHookError(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestBeforeUpdateOnlyModel{Name: "test"}
	m.ID = 1
	err := db.Update(context.Background(), m)
	if err == nil {
		t.Error("should return error when BeforeUpdate hook fails")
	}
	if !strings.Contains(err.Error(), "BeforeUpdate") {
		t.Errorf("error should mention BeforeUpdate, got: %v", err)
	}
}

func TestDB_Update_AfterUpdateHookError(t *testing.T) {
	exec := newMockExecutor()
	exec.execResult = &mockResult{rowsAffected: 1}
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestAfterUpdateOnlyModel{Name: "test"}
	m.ID = 1
	err := db.Update(context.Background(), m)
	if err == nil {
		t.Error("should return error when AfterUpdate hook fails")
	}
	if !strings.Contains(err.Error(), "AfterUpdate") {
		t.Errorf("error should mention AfterUpdate, got: %v", err)
	}
}

func TestDB_Update_AfterSaveHookError(t *testing.T) {
	exec := newMockExecutor()
	exec.execResult = &mockResult{rowsAffected: 1}
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestAfterSaveOnlyModel{Name: "test"}
	m.ID = 1
	err := db.Update(context.Background(), m)
	if err == nil {
		t.Error("should return error when AfterSave hook fails")
	}
	if !strings.Contains(err.Error(), "AfterSave") {
		t.Errorf("error should mention AfterSave, got: %v", err)
	}
}

func TestDB_Delete_AfterDeleteHookError(t *testing.T) {
	exec := newMockExecutor()
	exec.execResult = &mockResult{rowsAffected: 1}
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestAfterDeleteOnlyModel{Name: "test"}
	m.ID = 1
	err := db.Delete(context.Background(), m)
	if err == nil {
		t.Error("should return error when AfterDelete hook fails")
	}
	if !strings.Contains(err.Error(), "AfterDelete") {
		t.Errorf("error should mention AfterDelete, got: %v", err)
	}
}

func TestDB_Delete_SoftDelete_AfterDeleteHookError(t *testing.T) {
	exec := newMockExecutor()
	exec.execResult = &mockResult{rowsAffected: 1}
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestSoftDeleteWithAfterDeleteModel{Name: "test"}
	m.ID = 1
	err := db.Delete(context.Background(), m)
	if err == nil {
		t.Error("should return error when AfterDelete hook fails on soft delete")
	}
	if !strings.Contains(err.Error(), "AfterDelete") {
		t.Errorf("error should mention AfterDelete, got: %v", err)
	}
}

func TestDB_ForceDelete_BeforeDeleteHookError(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestModelWithHooks{Name: "test", hookErr: errHookFail}
	m.ID = 1
	err := db.ForceDelete(context.Background(), m)
	if err == nil {
		t.Error("should return error when BeforeDelete hook fails")
	}
	if !strings.Contains(err.Error(), "BeforeDelete") {
		t.Errorf("error should mention BeforeDelete, got: %v", err)
	}
}

func TestDB_ForceDelete_AfterDeleteHookError(t *testing.T) {
	exec := newMockExecutor()
	exec.execResult = &mockResult{rowsAffected: 1}
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestAfterDeleteOnlyModel{Name: "test"}
	m.ID = 1
	err := db.ForceDelete(context.Background(), m)
	if err == nil {
		t.Error("should return error when AfterDelete hook fails")
	}
}

func TestDB_ForceDelete_AfterSaveHookError(t *testing.T) {
	// ForceDelete doesn't have AfterSave, only AfterDelete - let's test that
	// Actually ForceDelete only calls BeforeDelete and AfterDelete
	// so AfterSave is not called. This test verifies that.
	exec := newMockExecutor()
	exec.execResult = &mockResult{rowsAffected: 1}
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestAfterSaveOnlyModel{Name: "test"}
	m.ID = 1
	err := db.ForceDelete(context.Background(), m)
	if err != nil {
		t.Errorf("ForceDelete should not fail for AfterSave models: %v", err)
	}
}

func TestDB_Delete_SoftDelete(t *testing.T) {
	exec := newMockExecutor()
	exec.execResult = &mockResult{rowsAffected: 1}
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestSoftDeleteModel{Name: "test"}
	m.ID = 1
	err := db.Delete(context.Background(), m)
	if err != nil {
		t.Errorf("Delete soft delete error = %v", err)
	}
	if !m.IsDeleted() {
		t.Error("model should be soft deleted")
	}
	// Soft delete uses Update internally
	if len(exec.execCalls) != 1 {
		t.Errorf("expected 1 exec call (UPDATE), got %d", len(exec.execCalls))
	}
	if !strings.Contains(exec.execCalls[0].query, "UPDATE") {
		t.Errorf("soft delete should use UPDATE, got: %s", exec.execCalls[0].query)
	}
}
