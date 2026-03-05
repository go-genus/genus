package migrate

import (
	"context"
	"database/sql"
	"reflect"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/go-genus/genus/dialects/sqlite"
)

// ========================================
// Test Helpers for diff
// ========================================

// mockExecutor implements core.Executor for testing SchemaDiffer.
type mockExecutor struct {
	db *sql.DB
}

func newMockExecutor(t *testing.T) *mockExecutor {
	t.Helper()
	db := newTestDB(t)
	return &mockExecutor{db: db}
}

func (e *mockExecutor) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return e.db.ExecContext(ctx, query, args...)
}

func (e *mockExecutor) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return e.db.QueryContext(ctx, query, args...)
}

func (e *mockExecutor) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return e.db.QueryRowContext(ctx, query, args...)
}

// Test model for schema diff
type DiffTestModel struct {
	ID   int64  `db:"id"`
	Name string `db:"name"`
}

type DiffTestModelWithTableName struct {
	ID   int64  `db:"id"`
	Name string `db:"name"`
}

func (m *DiffTestModelWithTableName) TableName() string {
	return "custom_diff_table"
}

type DiffEmbeddedBase struct {
	ID int64 `db:"id"`
}

type DiffTestModelEmbedded struct {
	DiffEmbeddedBase
	Name string `db:"name"`
}

type DiffTestModelNoTags struct {
	ID   int64
	Name string
}

type DiffTestModelWithDash struct {
	ID      int64  `db:"id"`
	Ignored string `db:"-"`
	Name    string `db:"name"`
}

type DiffTestModelNullable struct {
	ID   int64   `db:"id"`
	Name *string `db:"name"`
}

// ========================================
// Tests for NewSchemaDiffer
// ========================================

func TestNewSchemaDiffer(t *testing.T) {
	executor := newMockExecutor(t)
	dialect := sqlite.New()

	differ := NewSchemaDiffer(executor, dialect)
	if differ == nil {
		t.Fatal("expected non-nil SchemaDiffer")
	}
	if differ.executor != executor {
		t.Error("expected executor to be set")
	}
	if differ.dialect != dialect {
		t.Error("expected dialect to be set")
	}
}

// ========================================
// Tests for GetSchemaFromModels
// ========================================

func TestGetSchemaFromModels(t *testing.T) {
	executor := newMockExecutor(t)
	dialect := sqlite.New()
	differ := NewSchemaDiffer(executor, dialect)

	t.Run("single model", func(t *testing.T) {
		schemas := differ.GetSchemaFromModels(DiffTestModel{})
		if len(schemas) != 1 {
			t.Fatalf("expected 1 schema, got %d", len(schemas))
		}

		schema, exists := schemas["diff_test_models"]
		if !exists {
			t.Fatalf("expected schema 'diff_test_models', got keys: %v", keys(schemas))
		}
		if len(schema.Columns) != 2 {
			t.Errorf("expected 2 columns, got %d", len(schema.Columns))
		}
	})

	t.Run("model with custom table name", func(t *testing.T) {
		schemas := differ.GetSchemaFromModels(&DiffTestModelWithTableName{})
		if _, exists := schemas["custom_diff_table"]; !exists {
			t.Errorf("expected 'custom_diff_table' schema, got keys: %v", keys(schemas))
		}
	})

	t.Run("multiple models", func(t *testing.T) {
		schemas := differ.GetSchemaFromModels(DiffTestModel{}, &DiffTestModelWithTableName{})
		if len(schemas) != 2 {
			t.Errorf("expected 2 schemas, got %d", len(schemas))
		}
	})

	t.Run("model with embedded struct", func(t *testing.T) {
		schemas := differ.GetSchemaFromModels(DiffTestModelEmbedded{})
		if len(schemas) != 1 {
			t.Fatalf("expected 1 schema, got %d", len(schemas))
		}

		var schema *TableSchema
		for _, s := range schemas {
			schema = s
		}

		// Should have columns from both embedded and own fields
		if len(schema.Columns) < 2 {
			t.Errorf("expected at least 2 columns (from embedded + own), got %d", len(schema.Columns))
		}
	})

	t.Run("model with no db tags", func(t *testing.T) {
		schemas := differ.GetSchemaFromModels(DiffTestModelNoTags{})
		if len(schemas) != 1 {
			t.Fatalf("expected 1 schema, got %d", len(schemas))
		}

		var schema *TableSchema
		for _, s := range schemas {
			schema = s
		}

		if len(schema.Columns) != 0 {
			t.Errorf("expected 0 columns for model with no db tags, got %d", len(schema.Columns))
		}
	})

	t.Run("model with dash tag is skipped", func(t *testing.T) {
		schemas := differ.GetSchemaFromModels(DiffTestModelWithDash{})
		var schema *TableSchema
		for _, s := range schemas {
			schema = s
		}

		for _, col := range schema.Columns {
			if col.Name == "-" || col.Name == "ignored" {
				t.Error("field with db:\"-\" should be skipped")
			}
		}
		if len(schema.Columns) != 2 { // id and name
			t.Errorf("expected 2 columns, got %d", len(schema.Columns))
		}
	})

	t.Run("nullable pointer field", func(t *testing.T) {
		schemas := differ.GetSchemaFromModels(DiffTestModelNullable{})
		var schema *TableSchema
		for _, s := range schemas {
			schema = s
		}

		for _, col := range schema.Columns {
			if col.Name == "name" {
				if !col.Nullable {
					t.Error("expected pointer field to be nullable")
				}
				return
			}
		}
		t.Error("name column not found")
	})

	t.Run("id field is primary key and auto increment", func(t *testing.T) {
		schemas := differ.GetSchemaFromModels(DiffTestModel{})
		var schema *TableSchema
		for _, s := range schemas {
			schema = s
		}

		for _, col := range schema.Columns {
			if col.Name == "id" {
				if !col.PrimaryKey {
					t.Error("expected id to be primary key")
				}
				if !col.AutoIncrement {
					t.Error("expected id to be auto increment")
				}
				return
			}
		}
		t.Error("id column not found")
	})
}

// ========================================
// Tests for modelToSchema
// ========================================

func TestModelToSchema(t *testing.T) {
	executor := newMockExecutor(t)
	dialect := sqlite.New()
	differ := NewSchemaDiffer(executor, dialect)

	t.Run("pointer model", func(t *testing.T) {
		schema := differ.modelToSchema(&DiffTestModel{})
		if schema.Name != "diff_test_models" {
			t.Errorf("expected 'diff_test_models', got '%s'", schema.Name)
		}
	})

	t.Run("non-pointer model", func(t *testing.T) {
		schema := differ.modelToSchema(DiffTestModel{})
		if schema.Name != "diff_test_models" {
			t.Errorf("expected 'diff_test_models', got '%s'", schema.Name)
		}
	})
}

// ========================================
// Tests for goTypeToSQLType
// ========================================

func TestGoTypeToSQLType(t *testing.T) {
	executor := newMockExecutor(t)
	dialect := sqlite.New()
	differ := NewSchemaDiffer(executor, dialect)

	t.Run("pointer type is dereferenced", func(t *testing.T) {
		ptrType := reflect.TypeOf((*string)(nil))
		result := differ.goTypeToSQLType(ptrType)
		if result != "TEXT" {
			t.Errorf("expected TEXT for *string, got '%s'", result)
		}
	})

	t.Run("int64", func(t *testing.T) {
		result := differ.goTypeToSQLType(reflect.TypeOf(int64(0)))
		if result != "INTEGER" {
			t.Errorf("expected INTEGER for int64, got '%s'", result)
		}
	})
}

// ========================================
// Tests for Diff
// ========================================

func TestDiff(t *testing.T) {
	executor := newMockExecutor(t)
	dialect := sqlite.New()
	differ := NewSchemaDiffer(executor, dialect)

	t.Run("no changes", func(t *testing.T) {
		current := map[string]*TableSchema{
			"users": {
				Name: "users",
				Columns: []ColumnSchema{
					{Name: "id", Type: "INTEGER", PrimaryKey: true},
					{Name: "name", Type: "TEXT"},
				},
			},
		}
		target := map[string]*TableSchema{
			"users": {
				Name: "users",
				Columns: []ColumnSchema{
					{Name: "id", Type: "INTEGER", PrimaryKey: true},
					{Name: "name", Type: "TEXT"},
				},
			},
		}

		changes := differ.Diff(current, target)
		if len(changes) != 0 {
			t.Errorf("expected 0 changes, got %d", len(changes))
		}
	})

	t.Run("add table", func(t *testing.T) {
		current := map[string]*TableSchema{}
		target := map[string]*TableSchema{
			"users": {
				Name: "users",
				Columns: []ColumnSchema{
					{Name: "id", Type: "INTEGER", PrimaryKey: true},
				},
			},
		}

		changes := differ.Diff(current, target)
		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		if changes[0].Type != ChangeAddTable {
			t.Errorf("expected ADD_TABLE, got %s", changes[0].Type)
		}
		if changes[0].Table != "users" {
			t.Errorf("expected table 'users', got '%s'", changes[0].Table)
		}
	})

	t.Run("drop table", func(t *testing.T) {
		current := map[string]*TableSchema{
			"users": {
				Name: "users",
				Columns: []ColumnSchema{
					{Name: "id", Type: "INTEGER"},
				},
			},
		}
		target := map[string]*TableSchema{}

		changes := differ.Diff(current, target)
		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		if changes[0].Type != ChangeDropTable {
			t.Errorf("expected DROP_TABLE, got %s", changes[0].Type)
		}
	})

	t.Run("add column", func(t *testing.T) {
		current := map[string]*TableSchema{
			"users": {
				Name: "users",
				Columns: []ColumnSchema{
					{Name: "id", Type: "INTEGER"},
				},
			},
		}
		target := map[string]*TableSchema{
			"users": {
				Name: "users",
				Columns: []ColumnSchema{
					{Name: "id", Type: "INTEGER"},
					{Name: "email", Type: "TEXT"},
				},
			},
		}

		changes := differ.Diff(current, target)
		found := false
		for _, ch := range changes {
			if ch.Type == ChangeAddColumn && ch.Column == "email" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected ADD_COLUMN change for 'email'")
		}
	})

	t.Run("drop column", func(t *testing.T) {
		current := map[string]*TableSchema{
			"users": {
				Name: "users",
				Columns: []ColumnSchema{
					{Name: "id", Type: "INTEGER"},
					{Name: "email", Type: "TEXT"},
				},
			},
		}
		target := map[string]*TableSchema{
			"users": {
				Name: "users",
				Columns: []ColumnSchema{
					{Name: "id", Type: "INTEGER"},
				},
			},
		}

		changes := differ.Diff(current, target)
		found := false
		for _, ch := range changes {
			if ch.Type == ChangeDropColumn && ch.Column == "email" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected DROP_COLUMN change for 'email'")
		}
	})

	t.Run("modify column type", func(t *testing.T) {
		current := map[string]*TableSchema{
			"users": {
				Name: "users",
				Columns: []ColumnSchema{
					{Name: "id", Type: "INTEGER"},
					{Name: "name", Type: "TEXT"},
				},
			},
		}
		target := map[string]*TableSchema{
			"users": {
				Name: "users",
				Columns: []ColumnSchema{
					{Name: "id", Type: "INTEGER"},
					{Name: "name", Type: "VARCHAR(255)"},
				},
			},
		}

		changes := differ.Diff(current, target)
		found := false
		for _, ch := range changes {
			if ch.Type == ChangeModifyColumn && ch.Column == "name" {
				found = true
				if ch.OldType != "TEXT" {
					t.Errorf("expected old type TEXT, got %s", ch.OldType)
				}
				if ch.NewType != "VARCHAR(255)" {
					t.Errorf("expected new type VARCHAR(255), got %s", ch.NewType)
				}
				break
			}
		}
		if !found {
			t.Error("expected MODIFY_COLUMN change for 'name'")
		}
	})

	t.Run("modify column nullable", func(t *testing.T) {
		current := map[string]*TableSchema{
			"users": {
				Name: "users",
				Columns: []ColumnSchema{
					{Name: "name", Type: "TEXT", Nullable: false},
				},
			},
		}
		target := map[string]*TableSchema{
			"users": {
				Name: "users",
				Columns: []ColumnSchema{
					{Name: "name", Type: "TEXT", Nullable: true},
				},
			},
		}

		changes := differ.Diff(current, target)
		found := false
		for _, ch := range changes {
			if ch.Type == ChangeModifyColumn {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected MODIFY_COLUMN change for nullable change")
		}
	})

	t.Run("modify column default", func(t *testing.T) {
		current := map[string]*TableSchema{
			"users": {
				Name: "users",
				Columns: []ColumnSchema{
					{Name: "name", Type: "TEXT", Default: ""},
				},
			},
		}
		target := map[string]*TableSchema{
			"users": {
				Name: "users",
				Columns: []ColumnSchema{
					{Name: "name", Type: "TEXT", Default: "'unknown'"},
				},
			},
		}

		changes := differ.Diff(current, target)
		found := false
		for _, ch := range changes {
			if ch.Type == ChangeModifyColumn {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected MODIFY_COLUMN change for default change")
		}
	})

	t.Run("complex diff - multiple changes", func(t *testing.T) {
		current := map[string]*TableSchema{
			"users": {
				Name: "users",
				Columns: []ColumnSchema{
					{Name: "id", Type: "INTEGER"},
					{Name: "name", Type: "TEXT"},
					{Name: "old_col", Type: "TEXT"},
				},
			},
			"old_table": {
				Name:    "old_table",
				Columns: []ColumnSchema{{Name: "id", Type: "INTEGER"}},
			},
		}
		target := map[string]*TableSchema{
			"users": {
				Name: "users",
				Columns: []ColumnSchema{
					{Name: "id", Type: "INTEGER"},
					{Name: "name", Type: "VARCHAR(100)"},
					{Name: "new_col", Type: "TEXT"},
				},
			},
			"new_table": {
				Name:    "new_table",
				Columns: []ColumnSchema{{Name: "id", Type: "INTEGER"}},
			},
		}

		changes := differ.Diff(current, target)
		// Should have: add_table(new_table), drop_table(old_table), add_column(new_col), drop_column(old_col), modify_column(name)
		if len(changes) < 4 {
			t.Errorf("expected at least 4 changes, got %d", len(changes))
		}
	})

	t.Run("diffIndexes returns nil", func(t *testing.T) {
		current := &TableSchema{Name: "test"}
		target := &TableSchema{Name: "test"}
		changes := differ.diffIndexes(current, target)
		if changes != nil {
			t.Errorf("expected nil, got %v", changes)
		}
	})

	t.Run("diffForeignKeys returns nil", func(t *testing.T) {
		current := &TableSchema{Name: "test"}
		target := &TableSchema{Name: "test"}
		changes := differ.diffForeignKeys(current, target)
		if changes != nil {
			t.Errorf("expected nil, got %v", changes)
		}
	})
}

// ========================================
// Tests for createTableChange
// ========================================

func TestCreateTableChange(t *testing.T) {
	executor := newMockExecutor(t)
	dialect := sqlite.New()
	differ := NewSchemaDiffer(executor, dialect)

	t.Run("generates CREATE TABLE SQL", func(t *testing.T) {
		table := &TableSchema{
			Name: "users",
			Columns: []ColumnSchema{
				{Name: "id", Type: "INTEGER", PrimaryKey: true},
				{Name: "name", Type: "TEXT", Nullable: false},
			},
		}

		change := differ.createTableChange(table)

		if change.Type != ChangeAddTable {
			t.Errorf("expected ADD_TABLE, got %s", change.Type)
		}
		if !strings.Contains(change.SQL, "CREATE TABLE") {
			t.Error("expected CREATE TABLE in SQL")
		}
		if !strings.Contains(change.SQL, "PRIMARY KEY") {
			t.Error("expected PRIMARY KEY in SQL")
		}
		if !strings.Contains(change.SQL, "NOT NULL") {
			t.Error("expected NOT NULL in SQL")
		}
		if !change.Reversible {
			t.Error("expected reversible")
		}
		if !strings.Contains(change.ReverseSQL, "DROP TABLE") {
			t.Error("expected DROP TABLE in reverse SQL")
		}
	})

	t.Run("nullable column omits NOT NULL", func(t *testing.T) {
		table := &TableSchema{
			Name: "users",
			Columns: []ColumnSchema{
				{Name: "bio", Type: "TEXT", Nullable: true},
			},
		}

		change := differ.createTableChange(table)
		// bio line should not have NOT NULL
		lines := strings.Split(change.SQL, "\n")
		for _, line := range lines {
			if strings.Contains(line, "bio") && strings.Contains(line, "NOT NULL") {
				t.Error("nullable column should not have NOT NULL")
			}
		}
	})

	t.Run("column with default", func(t *testing.T) {
		table := &TableSchema{
			Name: "users",
			Columns: []ColumnSchema{
				{Name: "status", Type: "TEXT", Default: "'active'"},
			},
		}

		change := differ.createTableChange(table)
		if !strings.Contains(change.SQL, "DEFAULT 'active'") {
			t.Error("expected DEFAULT in SQL")
		}
	})

	t.Run("auto increment column with SQLite dialect", func(t *testing.T) {
		table := &TableSchema{
			Name: "users",
			Columns: []ColumnSchema{
				{Name: "id", Type: "INTEGER", PrimaryKey: true, AutoIncrement: true},
			},
		}

		change := differ.createTableChange(table)
		// SQLite dialect uses "?" placeholder, which is NOT "$", so no AUTO_INCREMENT
		// The code checks for "?" placeholder == MySQL
		if !strings.Contains(change.SQL, "PRIMARY KEY") {
			t.Error("expected PRIMARY KEY")
		}
	})
}

// ========================================
// Tests for dropTableChange
// ========================================

func TestDropTableChange(t *testing.T) {
	executor := newMockExecutor(t)
	dialect := sqlite.New()
	differ := NewSchemaDiffer(executor, dialect)

	change := differ.dropTableChange(&TableSchema{Name: "users"})

	if change.Type != ChangeDropTable {
		t.Errorf("expected DROP_TABLE, got %s", change.Type)
	}
	if !strings.Contains(change.SQL, "DROP TABLE") {
		t.Error("expected DROP TABLE in SQL")
	}
	if change.Reversible {
		t.Error("expected not reversible")
	}
}

// ========================================
// Tests for addColumnChange
// ========================================

func TestAddColumnChange(t *testing.T) {
	executor := newMockExecutor(t)
	dialect := sqlite.New()
	differ := NewSchemaDiffer(executor, dialect)

	t.Run("basic add column", func(t *testing.T) {
		col := ColumnSchema{Name: "email", Type: "TEXT", Nullable: false}
		change := differ.addColumnChange("users", col)

		if change.Type != ChangeAddColumn {
			t.Errorf("expected ADD_COLUMN, got %s", change.Type)
		}
		if !strings.Contains(change.SQL, "ADD COLUMN") {
			t.Error("expected ADD COLUMN in SQL")
		}
		if !strings.Contains(change.SQL, "NOT NULL") {
			t.Error("expected NOT NULL in SQL")
		}
		if !change.Reversible {
			t.Error("expected reversible")
		}
		if !strings.Contains(change.ReverseSQL, "DROP COLUMN") {
			t.Error("expected DROP COLUMN in reverse SQL")
		}
	})

	t.Run("nullable add column", func(t *testing.T) {
		col := ColumnSchema{Name: "bio", Type: "TEXT", Nullable: true}
		change := differ.addColumnChange("users", col)

		if strings.Contains(change.SQL, "NOT NULL") {
			t.Error("nullable column should not have NOT NULL")
		}
	})

	t.Run("add column with default", func(t *testing.T) {
		col := ColumnSchema{Name: "status", Type: "TEXT", Default: "'active'"}
		change := differ.addColumnChange("users", col)

		if !strings.Contains(change.SQL, "DEFAULT 'active'") {
			t.Error("expected DEFAULT in SQL")
		}
	})
}

// ========================================
// Tests for dropColumnChange
// ========================================

func TestDropColumnChange(t *testing.T) {
	executor := newMockExecutor(t)
	dialect := sqlite.New()
	differ := NewSchemaDiffer(executor, dialect)

	col := ColumnSchema{Name: "email", Type: "TEXT"}
	change := differ.dropColumnChange("users", col)

	if change.Type != ChangeDropColumn {
		t.Errorf("expected DROP_COLUMN, got %s", change.Type)
	}
	if !strings.Contains(change.SQL, "DROP COLUMN") {
		t.Error("expected DROP COLUMN in SQL")
	}
	if change.Reversible {
		t.Error("expected not reversible")
	}
	if change.OldType != "TEXT" {
		t.Errorf("expected old type TEXT, got %s", change.OldType)
	}
}

// ========================================
// Tests for modifyColumnChange
// ========================================

func TestModifyColumnChange(t *testing.T) {
	executor := newMockExecutor(t)
	dialect := sqlite.New()
	differ := NewSchemaDiffer(executor, dialect)

	old := ColumnSchema{Name: "name", Type: "TEXT"}
	new := ColumnSchema{Name: "name", Type: "VARCHAR(255)"}

	change := differ.modifyColumnChange("users", old, new)

	if change.Type != ChangeModifyColumn {
		t.Errorf("expected MODIFY_COLUMN, got %s", change.Type)
	}
	if change.OldType != "TEXT" {
		t.Errorf("expected old type TEXT, got %s", change.OldType)
	}
	if change.NewType != "VARCHAR(255)" {
		t.Errorf("expected new type VARCHAR(255), got %s", change.NewType)
	}
	if !change.Reversible {
		t.Error("expected reversible")
	}
	// SQLite uses "?" placeholder which is like MySQL, so MODIFY COLUMN should be used
	// Actually SQLite uses double quotes, not backticks, and placeholder is "?"
	// The code checks placeholder == "?" for MySQL (but SQLite also returns "?")
	if !strings.Contains(change.SQL, "ALTER TABLE") {
		t.Error("expected ALTER TABLE in SQL")
	}
}

// ========================================
// Tests for columnChanged
// ========================================

func TestColumnChanged(t *testing.T) {
	executor := newMockExecutor(t)
	dialect := sqlite.New()
	differ := NewSchemaDiffer(executor, dialect)

	t.Run("same column", func(t *testing.T) {
		old := ColumnSchema{Name: "name", Type: "TEXT", Nullable: false, Default: ""}
		new := ColumnSchema{Name: "name", Type: "TEXT", Nullable: false, Default: ""}
		if differ.columnChanged(old, new) {
			t.Error("expected no change")
		}
	})

	t.Run("type changed", func(t *testing.T) {
		old := ColumnSchema{Name: "name", Type: "TEXT"}
		new := ColumnSchema{Name: "name", Type: "VARCHAR(255)"}
		if !differ.columnChanged(old, new) {
			t.Error("expected change for type")
		}
	})

	t.Run("nullable changed", func(t *testing.T) {
		old := ColumnSchema{Name: "name", Type: "TEXT", Nullable: false}
		new := ColumnSchema{Name: "name", Type: "TEXT", Nullable: true}
		if !differ.columnChanged(old, new) {
			t.Error("expected change for nullable")
		}
	})

	t.Run("default changed", func(t *testing.T) {
		old := ColumnSchema{Name: "name", Type: "TEXT", Default: ""}
		new := ColumnSchema{Name: "name", Type: "TEXT", Default: "'test'"}
		if !differ.columnChanged(old, new) {
			t.Error("expected change for default")
		}
	})
}

// ========================================
// Tests for GenerateMigration
// ========================================

func TestGenerateMigration(t *testing.T) {
	executor := newMockExecutor(t)
	dialect := sqlite.New()
	differ := NewSchemaDiffer(executor, dialect)

	t.Run("generates migration SQL", func(t *testing.T) {
		changes := []SchemaChange{
			{
				Type:        ChangeAddTable,
				Table:       "users",
				Description: "Create table users",
				SQL:         `CREATE TABLE "users" (id INTEGER PRIMARY KEY)`,
				Reversible:  true,
				ReverseSQL:  `DROP TABLE "users"`,
			},
			{
				Type:        ChangeAddColumn,
				Table:       "users",
				Column:      "email",
				Description: "Add column users.email",
				SQL:         `ALTER TABLE "users" ADD COLUMN "email" TEXT`,
				Reversible:  true,
				ReverseSQL:  `ALTER TABLE "users" DROP COLUMN "email"`,
			},
		}

		result := differ.GenerateMigration(changes)

		if !strings.Contains(result, "-- Migration generated by Genus") {
			t.Error("expected header comment")
		}
		if !strings.Contains(result, "-- Up") {
			t.Error("expected Up section")
		}
		if !strings.Contains(result, "-- Down") {
			t.Error("expected Down section")
		}
		if !strings.Contains(result, "CREATE TABLE") {
			t.Error("expected CREATE TABLE in output")
		}
		if !strings.Contains(result, "DROP TABLE") {
			t.Error("expected DROP TABLE in Down section")
		}
	})

	t.Run("irreversible change generates warning", func(t *testing.T) {
		changes := []SchemaChange{
			{
				Type:        ChangeDropTable,
				Table:       "users",
				Description: "Drop table users",
				SQL:         `DROP TABLE "users"`,
				Reversible:  false,
			},
		}

		result := differ.GenerateMigration(changes)
		if !strings.Contains(result, "WARNING: Cannot reverse") {
			t.Error("expected warning for irreversible change")
		}
	})

	t.Run("empty changes", func(t *testing.T) {
		result := differ.GenerateMigration(nil)
		if !strings.Contains(result, "-- Migration generated by Genus") {
			t.Error("expected header even with no changes")
		}
	})

	t.Run("reverse order in Down section", func(t *testing.T) {
		changes := []SchemaChange{
			{
				Type:        ChangeAddTable,
				Description: "First",
				SQL:         "CREATE TABLE a",
				Reversible:  true,
				ReverseSQL:  "DROP TABLE a",
			},
			{
				Type:        ChangeAddColumn,
				Description: "Second",
				SQL:         "ALTER TABLE a ADD COLUMN b TEXT",
				Reversible:  true,
				ReverseSQL:  "ALTER TABLE a DROP COLUMN b",
			},
		}

		result := differ.GenerateMigration(changes)

		// In Down section, "Second" should come before "First"
		downIdx := strings.Index(result, "-- Down")
		downSection := result[downIdx:]

		secondIdx := strings.Index(downSection, "Reverse: Second")
		firstIdx := strings.Index(downSection, "Reverse: First")

		if secondIdx > firstIdx {
			t.Error("expected reverse order in Down section: Second before First")
		}
	})
}

// ========================================
// Tests for toSnakeCaseMigrate
// ========================================

func TestToSnakeCaseMigrate(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"TestUser", "test_user"},
		{"Simple", "simple"},
		{"CamelCase", "camel_case"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toSnakeCaseMigrate(tt.input)
			if result != tt.expected {
				t.Errorf("toSnakeCaseMigrate(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// ========================================
// Tests for ChangeType constants
// ========================================

func TestChangeTypeConstants(t *testing.T) {
	tests := []struct {
		ct       ChangeType
		expected string
	}{
		{ChangeAddTable, "ADD_TABLE"},
		{ChangeDropTable, "DROP_TABLE"},
		{ChangeAddColumn, "ADD_COLUMN"},
		{ChangeDropColumn, "DROP_COLUMN"},
		{ChangeModifyColumn, "MODIFY_COLUMN"},
		{ChangeAddIndex, "ADD_INDEX"},
		{ChangeDropIndex, "DROP_INDEX"},
		{ChangeAddForeignKey, "ADD_FOREIGN_KEY"},
		{ChangeDropForeignKey, "DROP_FOREIGN_KEY"},
		{ChangeAddConstraint, "ADD_CONSTRAINT"},
		{ChangeDropConstraint, "DROP_CONSTRAINT"},
	}

	for _, tt := range tests {
		if string(tt.ct) != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, tt.ct)
		}
	}
}

// helper
func keys(m map[string]*TableSchema) []string {
	k := make([]string, 0, len(m))
	for key := range m {
		k = append(k, key)
	}
	return k
}
