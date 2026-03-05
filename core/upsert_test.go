package core

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestUpsertConfig_Defaults(t *testing.T) {
	config := UpsertConfig{}
	if len(config.ConflictColumns) != 0 {
		t.Error("ConflictColumns should be empty by default")
	}
	if config.DoNothing {
		t.Error("DoNothing should be false by default")
	}
}

func TestDB_Upsert_PostgreSQL(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestModel{Name: "test", Email: "test@test.com"}
	m.CreatedAt = time.Now()
	m.UpdatedAt = time.Now()

	err := db.Upsert(context.Background(), m)
	if err != nil {
		t.Errorf("Upsert error = %v", err)
	}

	if len(exec.execCalls) != 1 {
		t.Fatalf("expected 1 exec call, got %d", len(exec.execCalls))
	}

	query := exec.execCalls[0].query
	if !strings.Contains(query, "ON CONFLICT") {
		t.Errorf("query should contain ON CONFLICT, got: %s", query)
	}
	if !strings.Contains(query, "DO UPDATE SET") {
		t.Errorf("query should contain DO UPDATE SET, got: %s", query)
	}
}

func TestDB_Upsert_MySQL(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newMySQLDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestModel{Name: "test", Email: "test@test.com"}
	m.CreatedAt = time.Now()
	m.UpdatedAt = time.Now()

	err := db.Upsert(context.Background(), m)
	if err != nil {
		t.Errorf("Upsert error = %v", err)
	}

	query := exec.execCalls[0].query
	if !strings.Contains(query, "ON DUPLICATE KEY UPDATE") {
		t.Errorf("query should contain ON DUPLICATE KEY UPDATE, got: %s", query)
	}
}

func TestDB_UpsertWithConfig_DoNothing_PostgreSQL(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestModel{Name: "test", Email: "test@test.com"}
	m.CreatedAt = time.Now()
	m.UpdatedAt = time.Now()

	err := db.UpsertWithConfig(context.Background(), m, UpsertConfig{
		DoNothing: true,
	})
	if err != nil {
		t.Errorf("Upsert error = %v", err)
	}

	query := exec.execCalls[0].query
	if !strings.Contains(query, "DO NOTHING") {
		t.Errorf("query should contain DO NOTHING, got: %s", query)
	}
}

func TestDB_UpsertWithConfig_DoNothing_MySQL(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newMySQLDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestModel{Name: "test", Email: "test@test.com"}
	m.CreatedAt = time.Now()
	m.UpdatedAt = time.Now()

	err := db.UpsertWithConfig(context.Background(), m, UpsertConfig{
		DoNothing: true,
	})
	if err != nil {
		t.Errorf("Upsert error = %v", err)
	}

	query := exec.execCalls[0].query
	if !strings.Contains(query, "INSERT IGNORE") {
		t.Errorf("query should contain INSERT IGNORE, got: %s", query)
	}
}

func TestDB_UpsertWithConfig_CustomConflictColumns(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestModel{Name: "test", Email: "test@test.com"}
	m.CreatedAt = time.Now()
	m.UpdatedAt = time.Now()

	err := db.UpsertWithConfig(context.Background(), m, UpsertConfig{
		ConflictColumns: []string{"email"},
	})
	if err != nil {
		t.Errorf("Upsert error = %v", err)
	}

	query := exec.execCalls[0].query
	if !strings.Contains(query, `"email"`) {
		t.Errorf("query should contain email conflict column, got: %s", query)
	}
}

func TestDB_UpsertWithConfig_CustomUpdateColumns(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestModel{Name: "test", Email: "test@test.com"}
	m.CreatedAt = time.Now()
	m.UpdatedAt = time.Now()

	err := db.UpsertWithConfig(context.Background(), m, UpsertConfig{
		UpdateColumns: []string{"name"},
	})
	if err != nil {
		t.Errorf("Upsert error = %v", err)
	}

	query := exec.execCalls[0].query
	if !strings.Contains(query, "EXCLUDED") {
		t.Errorf("query should contain EXCLUDED, got: %s", query)
	}
}

func TestDB_UpsertWithConfig_UpdateWhere(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestModel{Name: "test", Email: "test@test.com"}
	m.CreatedAt = time.Now()
	m.UpdatedAt = time.Now()

	err := db.UpsertWithConfig(context.Background(), m, UpsertConfig{
		UpdateWhere:     "version > EXCLUDED.version",
		UpdateWhereArgs: []interface{}{},
	})
	if err != nil {
		t.Errorf("Upsert error = %v", err)
	}

	query := exec.execCalls[0].query
	if !strings.Contains(query, "WHERE") {
		t.Errorf("query should contain WHERE, got: %s", query)
	}
}

func TestDB_Upsert_ExecError(t *testing.T) {
	exec := newMockExecutor()
	exec.execErr = errTest
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestModel{Name: "test"}
	m.CreatedAt = time.Now()
	m.UpdatedAt = time.Now()

	err := db.Upsert(context.Background(), m)
	if err == nil {
		t.Error("should return error")
	}
}

func TestDB_BatchUpsert_EmptySlice(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	models := []*TestModel{}
	err := db.BatchUpsert(context.Background(), models)
	if err != nil {
		t.Errorf("BatchUpsert error = %v", err)
	}
}

func TestDB_BatchUpsert_NotSlice(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	err := db.BatchUpsert(context.Background(), "not a slice")
	if err == nil {
		t.Error("should return error for non-slice")
	}
}

func TestDB_BatchUpsertWithConfig(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	now := time.Now()
	models := []*TestModel{
		{Model: Model{ID: 1, CreatedAt: now, UpdatedAt: now}, Name: "a", Email: "a@test.com"},
		{Model: Model{ID: 2, CreatedAt: now, UpdatedAt: now}, Name: "b", Email: "b@test.com"},
	}

	err := db.BatchUpsertWithConfig(context.Background(), models, UpsertConfig{}, BatchConfig{BatchSize: 10})
	if err != nil {
		t.Errorf("BatchUpsertWithConfig error = %v", err)
	}
}

func TestDB_BatchUpsertWithConfig_Batching(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	now := time.Now()
	models := make([]*TestModel, 5)
	for i := range models {
		models[i] = &TestModel{
			Model: Model{ID: int64(i + 1), CreatedAt: now, UpdatedAt: now},
			Name:  "test",
			Email: "test@test.com",
		}
	}

	err := db.BatchUpsertWithConfig(context.Background(), models, UpsertConfig{}, BatchConfig{BatchSize: 2})
	if err != nil {
		t.Errorf("error = %v", err)
	}

	// With batch size 2 and 5 items, we should have 3 batches
	if len(exec.execCalls) != 3 {
		t.Errorf("expected 3 exec calls (batches), got %d", len(exec.execCalls))
	}
}

func TestDB_BatchUpsertWithConfig_DefaultBatchSize(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	now := time.Now()
	models := []*TestModel{
		{Model: Model{ID: 1, CreatedAt: now, UpdatedAt: now}, Name: "a"},
	}

	err := db.BatchUpsertWithConfig(context.Background(), models, UpsertConfig{}, BatchConfig{BatchSize: 0})
	if err != nil {
		t.Errorf("error = %v", err)
	}
}

func TestDB_BatchUpsertWithConfig_MySQL(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newMySQLDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	now := time.Now()
	models := []*TestModel{
		{Model: Model{ID: 1, CreatedAt: now, UpdatedAt: now}, Name: "a", Email: "a@test.com"},
	}

	err := db.BatchUpsertWithConfig(context.Background(), models, UpsertConfig{}, BatchConfig{BatchSize: 10})
	if err != nil {
		t.Errorf("error = %v", err)
	}

	if len(exec.execCalls) != 1 {
		t.Fatalf("expected 1 exec call, got %d", len(exec.execCalls))
	}

	query := exec.execCalls[0].query
	if !strings.Contains(query, "ON DUPLICATE KEY UPDATE") {
		t.Errorf("MySQL batch upsert should use ON DUPLICATE KEY UPDATE, got: %s", query)
	}
}

func TestDB_BatchUpsertWithConfig_DoNothing_MySQL(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newMySQLDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	now := time.Now()
	models := []*TestModel{
		{Model: Model{ID: 1, CreatedAt: now, UpdatedAt: now}, Name: "a"},
	}

	err := db.BatchUpsertWithConfig(context.Background(), models, UpsertConfig{DoNothing: true}, BatchConfig{BatchSize: 10})
	if err != nil {
		t.Errorf("error = %v", err)
	}

	query := exec.execCalls[0].query
	if !strings.Contains(query, "INSERT IGNORE") {
		t.Errorf("should use INSERT IGNORE, got: %s", query)
	}
}

func TestDB_BatchUpsertWithConfig_DoNothing_PostgreSQL(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	now := time.Now()
	models := []*TestModel{
		{Model: Model{ID: 1, CreatedAt: now, UpdatedAt: now}, Name: "a"},
	}

	err := db.BatchUpsertWithConfig(context.Background(), models, UpsertConfig{DoNothing: true}, BatchConfig{BatchSize: 10})
	if err != nil {
		t.Errorf("error = %v", err)
	}

	query := exec.execCalls[0].query
	if !strings.Contains(query, "DO NOTHING") {
		t.Errorf("should use DO NOTHING, got: %s", query)
	}
}

func TestDB_GetUpdateColumns_Explicit(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	db := NewWithExecutor(exec, dialect)

	cols := db.getUpdateColumns([]string{"id", "name", "email"}, UpsertConfig{
		UpdateColumns: []string{"name"},
	})
	if len(cols) != 1 || cols[0] != "name" {
		t.Errorf("getUpdateColumns = %v, want [name]", cols)
	}
}

func TestDB_GetUpdateColumns_Auto(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	db := NewWithExecutor(exec, dialect)

	cols := db.getUpdateColumns(
		[]string{"id", "name", "email", "created_at", "updated_at"},
		UpsertConfig{ConflictColumns: []string{"email"}},
	)

	// Should exclude: id, created_at, email (conflict column)
	for _, col := range cols {
		if col == "id" || col == "created_at" || col == "email" {
			t.Errorf("should not include %q", col)
		}
	}
}

func TestDB_GetTableNameFromValue(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	db := NewWithExecutor(exec, dialect)

	m := TestModel{Name: "test"}
	name := db.getTableNameFromValue(reflect.ValueOf(m))
	if name != "test_model" {
		t.Errorf("getTableNameFromValue = %q, want %q", name, "test_model")
	}
}

func TestDB_GetTableNameFromValue_WithTableNamer(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	db := NewWithExecutor(exec, dialect)

	m := &TestModelWithTableName{Name: "test"}
	name := db.getTableNameFromValue(reflect.ValueOf(m))
	if name != "custom_table" {
		t.Errorf("getTableNameFromValue = %q, want %q", name, "custom_table")
	}
}

func TestGetColumnsAndValuesFromValue(t *testing.T) {
	m := TestModel{
		Model: Model{ID: 1, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		Name:  "Alice",
		Email: "alice@test.com",
	}

	cols, vals, err := getColumnsAndValuesFromValue(reflect.ValueOf(m))
	if err != nil {
		t.Fatalf("error = %v", err)
	}

	if len(cols) != len(vals) {
		t.Errorf("cols/vals mismatch: %d vs %d", len(cols), len(vals))
	}
}

func TestGetColumnsAndValuesFromValue_ZeroID(t *testing.T) {
	m := TestModel{Name: "Alice"}
	m.CreatedAt = time.Now()
	m.UpdatedAt = time.Now()

	cols, _, _ := getColumnsAndValuesFromValue(reflect.ValueOf(m))

	for _, col := range cols {
		if col == "id" {
			t.Error("zero ID should be skipped")
		}
	}
}

func TestDB_GetColumnNames(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	db := NewWithExecutor(exec, dialect)

	m := TestModel{Name: "test", Email: "test@test.com"}
	cols := db.getColumnNames(reflect.ValueOf(m))

	if len(cols) == 0 {
		t.Error("should return columns")
	}

	// Should contain db-tagged columns
	found := map[string]bool{}
	for _, col := range cols {
		found[col] = true
	}

	if !found["name"] {
		t.Error("should contain 'name'")
	}
	if !found["email"] {
		t.Error("should contain 'email'")
	}
	if !found["id"] {
		t.Error("should contain 'id'")
	}
}

func TestDB_BatchUpsert_ExecError(t *testing.T) {
	exec := newMockExecutor()
	exec.execErr = errTest
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	now := time.Now()
	models := []*TestModel{
		{Model: Model{ID: 1, CreatedAt: now, UpdatedAt: now}, Name: "a"},
	}

	err := db.BatchUpsert(context.Background(), models)
	if err == nil {
		t.Error("should return error on exec failure")
	}
}

func TestDB_BatchUpsertWithConfig_PointerSlice(t *testing.T) {
	exec := newMockExecutor()
	logger := newMockLogger()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	// Test with pointer to slice
	now := time.Now()
	models := &[]*TestModel{
		{Model: Model{ID: 1, CreatedAt: now, UpdatedAt: now}, Name: "a"},
	}

	err := db.BatchUpsertWithConfig(context.Background(), models, UpsertConfig{}, BatchConfig{})
	if err != nil {
		t.Errorf("error = %v", err)
	}
}

func TestDB_GetColumnNames_WithEmbedded(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, newMockLogger())

	m := TestModel{Name: "test"}
	val := reflect.ValueOf(m)
	cols := db.getColumnNames(val)

	// Should include columns from embedded Model and from TestModel
	if len(cols) == 0 {
		t.Error("should have columns")
	}

	// Should include id, created_at, updated_at, name, email
	found := make(map[string]bool)
	for _, c := range cols {
		found[c] = true
	}
	if !found["name"] {
		t.Error("should contain name")
	}
}

func TestDB_GetColumnNames_WithDBTagDash(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, newMockLogger())

	type ModelWithIgnored struct {
		Name    string `db:"name"`
		Ignored string `db:"-"`
	}

	m := ModelWithIgnored{Name: "test", Ignored: "skip"}
	val := reflect.ValueOf(m)
	cols := db.getColumnNames(val)

	for _, c := range cols {
		if c == "-" || c == "ignored" {
			t.Error("should not contain ignored column")
		}
	}
	if len(cols) != 1 {
		t.Errorf("expected 1 column, got %d: %v", len(cols), cols)
	}
}

func TestDB_GetColumnNames_Pointer(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, newMockLogger())

	m := &TestModel{Name: "test"}
	val := reflect.ValueOf(m) // pointer
	cols := db.getColumnNames(val)

	if len(cols) == 0 {
		t.Error("should have columns from pointer model")
	}
}

func TestDB_GetColumnNames_NoDBTag(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	db := NewWithExecutorAndLogger(exec, dialect, newMockLogger())

	type ModelNoTag struct {
		Name string
	}

	m := ModelNoTag{Name: "test"}
	val := reflect.ValueOf(m)
	cols := db.getColumnNames(val)

	// Field without db tag should be skipped (no dbTag means empty string)
	if len(cols) != 0 {
		t.Errorf("expected 0 columns for fields without db tag, got %d: %v", len(cols), cols)
	}
}

func TestGetColumnsAndValuesFromValue_EmbeddedStruct(t *testing.T) {
	m := TestModel{Name: "Alice", Email: "alice@test.com"}
	m.ID = 1

	val := reflect.ValueOf(m)
	cols, vals, err := getColumnsAndValuesFromValue(val)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(cols) != len(vals) {
		t.Errorf("cols length (%d) != vals length (%d)", len(cols), len(vals))
	}
}

func TestGetColumnsAndValuesFromValue_Pointer(t *testing.T) {
	m := &TestModel{Name: "Alice"}
	m.ID = 1

	val := reflect.ValueOf(m) // pointer
	cols, vals, err := getColumnsAndValuesFromValue(val)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(cols) != len(vals) {
		t.Errorf("cols length (%d) != vals length (%d)", len(cols), len(vals))
	}
}

func TestGetColumnsAndValuesFromValue_NonExported(t *testing.T) {
	type modelWithUnexported struct {
		Name    string `db:"name"`
		private string `db:"private"`
	}

	m := modelWithUnexported{Name: "Alice", private: "secret"}
	val := reflect.ValueOf(m)
	cols, _, err := getColumnsAndValuesFromValue(val)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	// Should not include unexported field
	for _, c := range cols {
		if c == "private" {
			t.Error("should not include unexported field")
		}
	}
	_ = m.private
}

func TestDB_UpsertWithConfig_LastInsertID(t *testing.T) {
	exec := newMockExecutor()
	exec.execResult = &mockResult{lastInsertID: 42, rowsAffected: 1}
	logger := newMockLogger()
	dialect := newMySQLDialect()
	db := NewWithExecutorAndLogger(exec, dialect, logger)

	m := &TestModel{Name: "test"}
	err := db.Upsert(context.Background(), m)
	if err != nil {
		t.Errorf("Upsert error = %v", err)
	}
	// Should set ID from LastInsertId
	if m.ID != 42 {
		t.Errorf("ID = %d, want 42", m.ID)
	}
}

func TestDB_UpsertWithConfig_NilLogger(t *testing.T) {
	exec := newMockExecutor()
	exec.execErr = errTest
	dialect := newPostgresDialect()
	// DB with nil logger
	db := &DB{executor: exec, dialect: dialect, logger: nil}

	m := &TestModel{Name: "test"}
	err := db.UpsertWithConfig(context.Background(), m, UpsertConfig{})
	if err == nil {
		t.Error("should return error")
	}
}
