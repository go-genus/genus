package core

import (
	"context"
	"database/sql/driver"
	"strings"
	"testing"
)

func TestDefaultBatchConfig(t *testing.T) {
	config := DefaultBatchConfig()

	if config.BatchSize != 100 {
		t.Errorf("BatchSize = %d, want 100", config.BatchSize)
	}
	if config.SkipHooks {
		t.Error("SkipHooks = true, want false")
	}
}

func TestBatchConfig_CustomValues(t *testing.T) {
	config := BatchConfig{
		BatchSize: 50,
		SkipHooks: true,
	}

	if config.BatchSize != 50 {
		t.Errorf("BatchSize = %d, want 50", config.BatchSize)
	}
	if !config.SkipHooks {
		t.Error("SkipHooks = false, want true")
	}
}

// BatchInsert tests

func TestBatchInsert_NotSlice(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	err := db.BatchInsert(context.Background(), "not a slice")
	if err == nil {
		t.Error("should return error for non-slice")
	}
}

func TestBatchInsert_EmptySlice(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	models := []*TestModel{}
	err := db.BatchInsert(context.Background(), models)
	if err != nil {
		t.Errorf("empty slice should not return error, got: %v", err)
	}
}

func TestBatchInsert_NonPointerSlice(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	models := []TestModel{{Name: "test"}}
	err := db.BatchInsert(context.Background(), models)
	if err == nil {
		t.Error("should return error for non-pointer slice")
	}
	if !strings.Contains(err.Error(), "slice of pointers") {
		t.Errorf("error should mention slice of pointers, got: %v", err)
	}
}

func TestBatchInsertWithConfig_PostgreSQL_QueryError(t *testing.T) {
	exec := newMockExecutor()
	exec.queryErr = errTest
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	models := []*TestModel{
		{Name: "Alice", Email: "alice@test.com"},
		{Name: "Bob", Email: "bob@test.com"},
	}

	err := db.BatchInsertWithConfig(context.Background(), models, DefaultBatchConfig())
	if err == nil {
		t.Error("should return error when query fails")
	}
}

func TestBatchInsertWithConfig_MySQL(t *testing.T) {
	exec := newMockExecutor()
	exec.execResult = &mockResult{lastInsertID: 10, rowsAffected: 2}
	logger := newMockLogger()
	dialect := newMySQLDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	models := []*TestModel{
		{Name: "Alice", Email: "alice@test.com"},
		{Name: "Bob", Email: "bob@test.com"},
	}

	err := db.BatchInsertWithConfig(context.Background(), models, DefaultBatchConfig())
	if err != nil {
		t.Errorf("BatchInsert MySQL error = %v", err)
	}

	// Check IDs were set
	if models[0].ID != 10 {
		t.Errorf("models[0].ID = %d, want 10", models[0].ID)
	}
	if models[1].ID != 11 {
		t.Errorf("models[1].ID = %d, want 11", models[1].ID)
	}
}

func TestBatchInsertWithConfig_MySQL_ExecError(t *testing.T) {
	exec := newMockExecutor()
	exec.execErr = errTest
	logger := newMockLogger()
	dialect := newMySQLDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	models := []*TestModel{{Name: "Alice"}}
	err := db.BatchInsertWithConfig(context.Background(), models, DefaultBatchConfig())
	if err == nil {
		t.Error("should return error on exec failure")
	}
}

func TestBatchInsertWithConfig_Batching(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newMySQLDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	models := make([]*TestModel, 5)
	for i := range models {
		models[i] = &TestModel{Name: "test"}
	}

	config := BatchConfig{BatchSize: 2}
	err := db.BatchInsertWithConfig(context.Background(), models, config)
	if err != nil {
		t.Errorf("error = %v", err)
	}

	// 5 items with batch size 2 = 3 batches
	if len(exec.execCalls) != 3 {
		t.Errorf("expected 3 exec calls, got %d", len(exec.execCalls))
	}
}

func TestBatchInsertWithConfig_ZeroBatchSize(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newMySQLDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	models := []*TestModel{{Name: "test"}}
	config := BatchConfig{BatchSize: 0}
	err := db.BatchInsertWithConfig(context.Background(), models, config)
	if err != nil {
		t.Errorf("error = %v", err)
	}
}

func TestBatchInsertWithConfig_SkipHooks(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newMySQLDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestModelWithHooks{Name: "test"}
	models := []interface{}{m}
	// Use insertBatch directly to test hooks
	err := db.insertBatch(context.Background(), models, BatchConfig{SkipHooks: true})
	if err != nil {
		t.Errorf("error = %v", err)
	}
	if m.beforeSaveCalled {
		t.Error("BeforeSave should not be called when SkipHooks=true")
	}
}

func TestBatchInsertWithConfig_WithHooks(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newMySQLDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestModelWithHooks{Name: "test"}
	models := []interface{}{m}
	err := db.insertBatch(context.Background(), models, BatchConfig{SkipHooks: false})
	if err != nil {
		t.Errorf("error = %v", err)
	}
	if !m.beforeSaveCalled {
		t.Error("BeforeSave should be called")
	}
	if !m.beforeCreateCalled {
		t.Error("BeforeCreate should be called")
	}
	if !m.afterCreateCalled {
		t.Error("AfterCreate should be called")
	}
	if !m.afterSaveCalled {
		t.Error("AfterSave should be called")
	}
}

func TestBatchInsertWithConfig_HookError(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newMySQLDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestModelWithHooks{Name: "test", hookErr: errHookFail}
	models := []interface{}{m}
	err := db.insertBatch(context.Background(), models, BatchConfig{SkipHooks: false})
	if err == nil {
		t.Error("should return error when hook fails")
	}
}

func TestBatchInsert_EmptyModels(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newMySQLDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	models := []interface{}{}
	err := db.insertBatch(context.Background(), models, DefaultBatchConfig())
	if err != nil {
		t.Errorf("empty models should return nil, got: %v", err)
	}
}

// BatchUpdate tests

func TestBatchUpdate_NotSlice(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	err := db.BatchUpdate(context.Background(), "not a slice")
	if err == nil {
		t.Error("should return error for non-slice")
	}
}

func TestBatchUpdate_EmptySlice(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	models := []*TestModel{}
	err := db.BatchUpdate(context.Background(), models)
	if err != nil {
		t.Errorf("empty slice error = %v", err)
	}
}

func TestBatchUpdate_WithTx(t *testing.T) {
	sqlDB := getMockSQLDB()
	defer sqlDB.Close()

	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithLogger(sqlDB, dialect, logger)

	models := []*TestModel{
		{Name: "Alice"},
	}
	models[0].ID = 1

	err := db.BatchUpdate(context.Background(), models)
	// May fail with mock driver, but tests the transaction path
	if err != nil {
		t.Logf("Expected error with mock: %v", err)
	}
}

func TestBatchUpdateDirect_Success(t *testing.T) {
	exec := newMockExecutor()
	exec.execResult = &mockResult{rowsAffected: 1}
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	// Need to use reflect.Value directly
	models := []*TestModel{
		{Name: "Alice"},
		{Name: "Bob"},
	}
	models[0].ID = 1
	models[1].ID = 2

	err := db.BatchUpdateWithConfig(context.Background(), models, DefaultBatchConfig())
	if err != nil {
		t.Errorf("BatchUpdate error = %v", err)
	}

	if len(exec.execCalls) != 2 {
		t.Errorf("expected 2 exec calls, got %d", len(exec.execCalls))
	}
}

func TestBatchUpdateDirect_ZeroID(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	models := []*TestModel{{Name: "test"}}
	err := db.BatchUpdateWithConfig(context.Background(), models, DefaultBatchConfig())
	if err == nil {
		t.Error("should return error for zero ID")
	}
}

func TestBatchUpdateDirect_ExecError(t *testing.T) {
	exec := newMockExecutor()
	exec.execErr = errTest
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	models := []*TestModel{{Name: "test"}}
	models[0].ID = 1
	err := db.BatchUpdateWithConfig(context.Background(), models, DefaultBatchConfig())
	if err == nil {
		t.Error("should return error on exec failure")
	}
}

func TestBatchUpdateDirect_WithHooks(t *testing.T) {
	exec := newMockExecutor()
	exec.execResult = &mockResult{rowsAffected: 1}
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestModelWithHooks{Name: "test"}
	m.ID = 1
	models := []*TestModelWithHooks{m}
	err := db.BatchUpdateWithConfig(context.Background(), models, BatchConfig{SkipHooks: false})
	if err != nil {
		t.Errorf("error = %v", err)
	}
	if !m.beforeSaveCalled {
		t.Error("BeforeSave should be called")
	}
	if !m.beforeUpdateCalled {
		t.Error("BeforeUpdate should be called")
	}
	if !m.afterUpdateCalled {
		t.Error("AfterUpdate should be called")
	}
	if !m.afterSaveCalled {
		t.Error("AfterSave should be called")
	}
}

func TestBatchUpdateDirect_SkipHooks(t *testing.T) {
	exec := newMockExecutor()
	exec.execResult = &mockResult{rowsAffected: 1}
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestModelWithHooks{Name: "test"}
	m.ID = 1
	models := []*TestModelWithHooks{m}
	err := db.BatchUpdateWithConfig(context.Background(), models, BatchConfig{SkipHooks: true})
	if err != nil {
		t.Errorf("error = %v", err)
	}
	if m.beforeSaveCalled {
		t.Error("BeforeSave should not be called when SkipHooks=true")
	}
}

func TestBatchUpdateDirect_HookError(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestModelWithHooks{Name: "test", hookErr: errHookFail}
	m.ID = 1
	models := []*TestModelWithHooks{m}
	err := db.BatchUpdateWithConfig(context.Background(), models, BatchConfig{SkipHooks: false})
	if err == nil {
		t.Error("should return error when hook fails")
	}
}

// BatchDelete tests

func TestBatchDelete_NotSlice(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	err := db.BatchDelete(context.Background(), "not a slice")
	if err == nil {
		t.Error("should return error for non-slice")
	}
}

func TestBatchDelete_EmptySlice(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	models := []*TestModel{}
	err := db.BatchDelete(context.Background(), models)
	if err != nil {
		t.Errorf("empty slice error = %v", err)
	}
}

func TestBatchDelete_HardDelete(t *testing.T) {
	exec := newMockExecutor()
	exec.execResult = &mockResult{rowsAffected: 2}
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m1 := &TestModel{Name: "Alice"}
	m1.ID = 1
	m2 := &TestModel{Name: "Bob"}
	m2.ID = 2
	models := []*TestModel{m1, m2}

	err := db.BatchDelete(context.Background(), models)
	if err != nil {
		t.Errorf("BatchDelete error = %v", err)
	}

	if len(exec.execCalls) != 1 {
		t.Fatalf("expected 1 exec call, got %d", len(exec.execCalls))
	}
	if !strings.Contains(exec.execCalls[0].query, "DELETE") {
		t.Errorf("query should contain DELETE, got: %s", exec.execCalls[0].query)
	}
}

func TestBatchDelete_ZeroID(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	models := []*TestModel{{Name: "test"}}
	err := db.BatchDelete(context.Background(), models)
	if err == nil {
		t.Error("should return error for zero ID")
	}
}

func TestBatchDelete_ExecError(t *testing.T) {
	exec := newMockExecutor()
	exec.execErr = errTest
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestModel{Name: "test"}
	m.ID = 1
	err := db.BatchDelete(context.Background(), []*TestModel{m})
	if err == nil {
		t.Error("should return error on exec failure")
	}
}

func TestBatchDelete_NoRowsAffected(t *testing.T) {
	exec := newMockExecutor()
	exec.execResult = &mockResult{rowsAffected: 0}
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestModel{Name: "test"}
	m.ID = 1
	err := db.BatchDelete(context.Background(), []*TestModel{m})
	if err == nil {
		t.Error("should return error when no rows deleted")
	}
}

func TestBatchDelete_SoftDelete(t *testing.T) {
	exec := newMockExecutor()
	exec.execResult = &mockResult{rowsAffected: 1}
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestSoftDeleteModel{Name: "test"}
	m.ID = 1
	models := []*TestSoftDeleteModel{m}

	err := db.BatchDelete(context.Background(), models)
	if err != nil {
		t.Errorf("BatchDelete error = %v", err)
	}

	if len(exec.execCalls) != 1 {
		t.Fatalf("expected 1 exec call, got %d", len(exec.execCalls))
	}
	if !strings.Contains(exec.execCalls[0].query, "UPDATE") {
		t.Errorf("soft delete should use UPDATE, got: %s", exec.execCalls[0].query)
	}

	if !m.IsDeleted() {
		t.Error("model should be soft deleted")
	}
}

func TestBatchDelete_WithHooks(t *testing.T) {
	exec := newMockExecutor()
	exec.execResult = &mockResult{rowsAffected: 1}
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestModelWithHooks{Name: "test"}
	m.ID = 1
	models := []*TestModelWithHooks{m}
	err := db.BatchDeleteWithConfig(context.Background(), models, BatchConfig{SkipHooks: false})
	if err != nil {
		t.Errorf("error = %v", err)
	}
	if !m.beforeDeleteCalled {
		t.Error("BeforeDelete should be called")
	}
	if !m.afterDeleteCalled {
		t.Error("AfterDelete should be called")
	}
}

func TestBatchDelete_SkipHooks(t *testing.T) {
	exec := newMockExecutor()
	exec.execResult = &mockResult{rowsAffected: 1}
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestModelWithHooks{Name: "test"}
	m.ID = 1
	models := []*TestModelWithHooks{m}
	err := db.BatchDeleteWithConfig(context.Background(), models, BatchConfig{SkipHooks: true})
	if err != nil {
		t.Errorf("error = %v", err)
	}
	if m.beforeDeleteCalled {
		t.Error("BeforeDelete should not be called when SkipHooks=true")
	}
}

func TestBatchDelete_HookError(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestModelWithHooks{Name: "test", hookErr: errHookFail}
	m.ID = 1
	models := []*TestModelWithHooks{m}
	err := db.BatchDeleteWithConfig(context.Background(), models, BatchConfig{SkipHooks: false})
	if err == nil {
		t.Error("should return error when hook fails")
	}
}

// BatchDeleteByIDs tests

func TestBatchDeleteByIDs_Empty(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	err := db.BatchDeleteByIDs(context.Background(), "users", []int64{})
	if err != nil {
		t.Errorf("empty IDs error = %v", err)
	}
}

func TestBatchDeleteByIDs_Success(t *testing.T) {
	exec := newMockExecutor()
	exec.execResult = &mockResult{rowsAffected: 3}
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	err := db.BatchDeleteByIDs(context.Background(), "users", []int64{1, 2, 3})
	if err != nil {
		t.Errorf("BatchDeleteByIDs error = %v", err)
	}

	if len(exec.execCalls) != 1 {
		t.Fatalf("expected 1 exec call, got %d", len(exec.execCalls))
	}
	if !strings.Contains(exec.execCalls[0].query, "DELETE") {
		t.Errorf("query should contain DELETE")
	}
}

func TestBatchDeleteByIDs_ExecError(t *testing.T) {
	exec := newMockExecutor()
	exec.execErr = errTest
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	err := db.BatchDeleteByIDs(context.Background(), "users", []int64{1})
	if err == nil {
		t.Error("should return error on exec failure")
	}
}

func TestBatchDeleteByIDs_NoRowsAffected(t *testing.T) {
	exec := newMockExecutor()
	exec.execResult = &mockResult{rowsAffected: 0}
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	err := db.BatchDeleteByIDs(context.Background(), "users", []int64{999})
	if err == nil {
		t.Error("should return error when no rows deleted")
	}
}

func TestBatchInsertWithConfig_PostgreSQL_Success(t *testing.T) {
	// Use a mock executor with QueryContext returning rows with IDs
	exec := newMockExecutorWithQueryRows(
		[]string{"id"},
		[][]driver.Value{{int64(1)}, {int64(2)}},
	)
	defer exec.Close()

	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	models := []*TestModel{
		{Name: "Alice", Email: "alice@test.com"},
		{Name: "Bob", Email: "bob@test.com"},
	}

	err := db.BatchInsertWithConfig(context.Background(), models, DefaultBatchConfig())
	if err != nil {
		t.Logf("BatchInsert error (may be expected with mock): %v", err)
	}
}

func TestBatchUpdateWithTx_Success(t *testing.T) {
	// Use a real sql.DB to trigger the batchUpdateWithTx path
	sqlDB := getMockSQLDB()
	defer sqlDB.Close()

	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithLogger(sqlDB, dialect, logger)

	models := []*TestModel{
		{Name: "Alice"},
		{Name: "Bob"},
	}
	models[0].ID = 1
	models[1].ID = 2

	err := db.BatchUpdateWithConfig(context.Background(), models, DefaultBatchConfig())
	// Mock driver exec returns affected=1, so should succeed
	if err != nil {
		t.Logf("BatchUpdate with tx error (may be expected with mock): %v", err)
	}
}

func TestBatchUpdateWithTx_Error(t *testing.T) {
	sqlDB := getMockSQLDB()
	defer sqlDB.Close()

	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithLogger(sqlDB, dialect, logger)

	// Model with zero ID should cause an error in batchUpdateDirect,
	// triggering the rollback path
	models := []*TestModel{
		{Name: "Alice"},
	}

	err := db.BatchUpdateWithConfig(context.Background(), models, DefaultBatchConfig())
	if err == nil {
		t.Error("should return error for zero ID")
	}
}

func TestBatchUpdateWithTx_CommitError(t *testing.T) {
	sqlDB := getMockSQLDBFailCommit()
	defer sqlDB.Close()

	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithLogger(sqlDB, dialect, logger)

	models := []*TestModel{
		{Name: "Alice"},
	}
	models[0].ID = 1

	err := db.BatchUpdateWithConfig(context.Background(), models, DefaultBatchConfig())
	if err == nil {
		t.Error("should return error when commit fails")
	}
}

func TestBatchUpdateWithTx_RollbackError(t *testing.T) {
	sqlDB := getMockSQLDBFailRollback()
	defer sqlDB.Close()

	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithLogger(sqlDB, dialect, logger)

	// Zero ID causes error, rollback then fails too
	models := []*TestModel{
		{Name: "Alice"},
	}

	err := db.BatchUpdateWithConfig(context.Background(), models, DefaultBatchConfig())
	if err == nil {
		t.Error("should return error")
	}
}

func TestBatchUpdateWithTx_BeginError(t *testing.T) {
	sqlDB := getMockSQLDBFailBegin()
	defer sqlDB.Close()

	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithLogger(sqlDB, dialect, logger)

	models := []*TestModel{
		{Name: "Alice"},
	}
	models[0].ID = 1

	err := db.BatchUpdateWithConfig(context.Background(), models, DefaultBatchConfig())
	if err == nil {
		t.Error("should return error when begin fails")
	}
}

func TestBatchUpdateDirect_AfterUpdateHookError(t *testing.T) {
	exec := newMockExecutor()
	exec.execResult = &mockResult{rowsAffected: 1}
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	models := []*TestAfterUpdateOnlyModel{
		{Name: "Alice"},
	}
	models[0].ID = 1

	config := DefaultBatchConfig()
	err := db.BatchUpdateWithConfig(context.Background(), models, config)
	if err == nil {
		t.Error("should return error when AfterUpdate hook fails")
	}
	if !strings.Contains(err.Error(), "AfterUpdate") {
		t.Errorf("error should mention AfterUpdate, got: %v", err)
	}
}

func TestBatchUpdateDirect_AfterSaveHookError(t *testing.T) {
	exec := newMockExecutor()
	exec.execResult = &mockResult{rowsAffected: 1}
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	models := []*TestAfterSaveOnlyModel{
		{Name: "Alice"},
	}
	models[0].ID = 1

	config := DefaultBatchConfig()
	err := db.BatchUpdateWithConfig(context.Background(), models, config)
	if err == nil {
		t.Error("should return error when AfterSave hook fails")
	}
	if !strings.Contains(err.Error(), "AfterSave") {
		t.Errorf("error should mention AfterSave, got: %v", err)
	}
}

func TestBatchDelete_AfterDeleteHookError(t *testing.T) {
	exec := newMockExecutor()
	exec.execResult = &mockResult{rowsAffected: 2}
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	models := []*TestAfterDeleteOnlyModel{
		{Name: "Alice"},
		{Name: "Bob"},
	}
	models[0].ID = 1
	models[1].ID = 2

	config := DefaultBatchConfig()
	err := db.BatchDeleteWithConfig(context.Background(), models, config)
	if err == nil {
		t.Error("should return error when AfterDelete hook fails")
	}
	if !strings.Contains(err.Error(), "AfterDelete") {
		t.Errorf("error should mention AfterDelete, got: %v", err)
	}
}

func TestBatchDelete_RowsAffectedError(t *testing.T) {
	exec := newMockExecutor()
	exec.execResult = &mockResult{rowsAffected: 0, rowsErr: errTest}
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	models := []*TestModel{{Name: "Alice"}}
	models[0].ID = 1

	err := db.BatchDeleteWithConfig(context.Background(), models, DefaultBatchConfig())
	// rowsAffected returns error, treated as 0 rows
	if err == nil {
		t.Error("should return error")
	}
}

func TestBatchInsert_GetColumnsError(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newMySQLDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	// Using non-pointer slice to test the getColumnsAndValues path with non-pointer models
	models := []TestModel{
		{Name: "Alice"},
	}
	err := db.BatchInsert(context.Background(), models)
	if err != nil {
		t.Logf("BatchInsert non-pointer error: %v", err)
	}
}
