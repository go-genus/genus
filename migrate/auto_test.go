package migrate

import (
	"context"
	"reflect"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/go-genus/genus/core"
	"github.com/go-genus/genus/dialects/sqlite"
)

// ========================================
// Test Models
// ========================================

type TestUser struct {
	core.Model
	Name  string `db:"name"`
	Email string `db:"email"`
}

type TestUserWithTableName struct {
	core.Model
	Name string `db:"name"`
}

func (u *TestUserWithTableName) TableName() string {
	return "custom_users"
}

type TestEmptyModel struct{}

type TestModelNoColumns struct {
	hidden string
}

type TestModelWithPointer struct {
	core.Model
	Name *string `db:"name"`
}

// ========================================
// Tests for AutoMigrate
// ========================================

func TestAutoMigrate(t *testing.T) {
	t.Run("creates table from struct", func(t *testing.T) {
		db := newTestDB(t)
		dialect := sqlite.New()
		ctx := context.Background()

		err := AutoMigrate(ctx, db, dialect, TestUser{})
		if err != nil {
			t.Fatalf("AutoMigrate failed: %v", err)
		}

		// Verify table was created by inserting data
		_, err = db.ExecContext(ctx, `INSERT INTO "test_user" ("name", "email") VALUES ('test', 'test@test.com')`)
		if err != nil {
			t.Fatalf("table not created correctly: %v", err)
		}
	})

	t.Run("creates table with custom name", func(t *testing.T) {
		db := newTestDB(t)
		dialect := sqlite.New()
		ctx := context.Background()

		err := AutoMigrate(ctx, db, dialect, &TestUserWithTableName{})
		if err != nil {
			t.Fatalf("AutoMigrate failed: %v", err)
		}

		// Verify custom table name
		_, err = db.ExecContext(ctx, `INSERT INTO "custom_users" ("name") VALUES ('test')`)
		if err != nil {
			t.Fatalf("custom table not created: %v", err)
		}
	})

	t.Run("multiple models", func(t *testing.T) {
		db := newTestDB(t)
		dialect := sqlite.New()
		ctx := context.Background()

		err := AutoMigrate(ctx, db, dialect, TestUser{}, &TestUserWithTableName{})
		if err != nil {
			t.Fatalf("AutoMigrate failed: %v", err)
		}
	})

	t.Run("non-struct model returns error", func(t *testing.T) {
		db := newTestDB(t)
		dialect := sqlite.New()
		ctx := context.Background()

		err := AutoMigrate(ctx, db, dialect, "not a struct")
		if err == nil {
			t.Fatal("expected error for non-struct model")
		}
	})

	t.Run("idempotent creation", func(t *testing.T) {
		db := newTestDB(t)
		dialect := sqlite.New()
		ctx := context.Background()

		err := AutoMigrate(ctx, db, dialect, TestUser{})
		if err != nil {
			t.Fatalf("first AutoMigrate failed: %v", err)
		}

		err = AutoMigrate(ctx, db, dialect, TestUser{})
		if err != nil {
			t.Fatalf("second AutoMigrate (idempotent) failed: %v", err)
		}
	})
}

// ========================================
// Tests for CreateTableMigration
// ========================================

func TestCreateTableMigration(t *testing.T) {
	t.Run("creates migration with Up and Down", func(t *testing.T) {
		migration := CreateTableMigration(1, "create_users", TestUser{})

		if migration.Version != 1 {
			t.Errorf("expected version 1, got %d", migration.Version)
		}
		if migration.Name != "create_users" {
			t.Errorf("expected name 'create_users', got '%s'", migration.Name)
		}
		if migration.Up == nil {
			t.Error("expected Up function to be set")
		}
		if migration.Down == nil {
			t.Error("expected Down function to be set")
		}
	})

	t.Run("Up creates table", func(t *testing.T) {
		db := newTestDB(t)
		dialect := sqlite.New()
		ctx := context.Background()

		migration := CreateTableMigration(1, "create_users", TestUser{})

		err := migration.Up(ctx, db, dialect)
		if err != nil {
			t.Fatalf("Up failed: %v", err)
		}

		// Verify table exists
		_, err = db.ExecContext(ctx, `SELECT 1 FROM "test_user"`)
		if err != nil {
			t.Fatalf("table not created: %v", err)
		}
	})

	t.Run("Down drops table", func(t *testing.T) {
		db := newTestDB(t)
		dialect := sqlite.New()
		ctx := context.Background()

		migration := CreateTableMigration(1, "create_users", TestUser{})

		// Create first
		migration.Up(ctx, db, dialect)

		// Drop
		err := migration.Down(ctx, db, dialect)
		if err != nil {
			t.Fatalf("Down failed: %v", err)
		}

		// Verify table doesn't exist
		_, err = db.ExecContext(ctx, `SELECT 1 FROM "test_user"`)
		if err == nil {
			t.Fatal("expected table to be dropped")
		}
	})

	t.Run("integration with Migrator", func(t *testing.T) {
		m, _, _, _ := newTestMigrator(t)
		ctx := context.Background()

		migration := CreateTableMigration(1, "create_users", TestUser{})
		m.Register(migration)

		if err := m.Up(ctx); err != nil {
			t.Fatalf("Up failed: %v", err)
		}

		if err := m.Down(ctx); err != nil {
			t.Fatalf("Down failed: %v", err)
		}
	})
}

// ========================================
// Tests for createTableFromStruct
// ========================================

func TestCreateTableFromStruct(t *testing.T) {
	t.Run("non-struct returns error", func(t *testing.T) {
		db := newTestDB(t)
		dialect := sqlite.New()
		ctx := context.Background()

		err := createTableFromStruct(ctx, db, dialect, 42)
		if err == nil {
			t.Fatal("expected error for non-struct")
		}
	})

	t.Run("pointer to struct works", func(t *testing.T) {
		db := newTestDB(t)
		dialect := sqlite.New()
		ctx := context.Background()

		err := createTableFromStruct(ctx, db, dialect, &TestUser{})
		if err != nil {
			t.Fatalf("failed with pointer: %v", err)
		}
	})

	t.Run("model with no db tags returns error", func(t *testing.T) {
		db := newTestDB(t)
		dialect := sqlite.New()
		ctx := context.Background()

		err := createTableFromStruct(ctx, db, dialect, TestModelNoColumns{})
		if err == nil {
			t.Fatal("expected error for model with no columns")
		}
	})
}

// ========================================
// Tests for dropTable
// ========================================

func TestDropTable(t *testing.T) {
	t.Run("drops existing table", func(t *testing.T) {
		db := newTestDB(t)
		dialect := sqlite.New()
		ctx := context.Background()

		// Create table first
		db.ExecContext(ctx, `CREATE TABLE "test_user" (id INTEGER PRIMARY KEY)`)

		err := dropTable(ctx, db, dialect, TestUser{})
		if err != nil {
			t.Fatalf("dropTable failed: %v", err)
		}

		// Verify table was dropped
		_, err = db.ExecContext(ctx, `SELECT 1 FROM "test_user"`)
		if err == nil {
			t.Fatal("expected table to be dropped")
		}
	})

	t.Run("drops non-existing table without error", func(t *testing.T) {
		db := newTestDB(t)
		dialect := sqlite.New()
		ctx := context.Background()

		err := dropTable(ctx, db, dialect, TestUser{})
		if err != nil {
			t.Fatalf("dropTable should not error for non-existing table: %v", err)
		}
	})

	t.Run("drops table with custom name", func(t *testing.T) {
		db := newTestDB(t)
		dialect := sqlite.New()
		ctx := context.Background()

		db.ExecContext(ctx, `CREATE TABLE "custom_users" (id INTEGER PRIMARY KEY)`)

		err := dropTable(ctx, db, dialect, &TestUserWithTableName{})
		if err != nil {
			t.Fatalf("dropTable failed: %v", err)
		}
	})
}

// ========================================
// Tests for buildColumnDefinition
// ========================================

func TestBuildColumnDefinition(t *testing.T) {
	dialect := sqlite.New()

	t.Run("skips field without db tag", func(t *testing.T) {
		field := reflect.StructField{
			Name: "Hidden",
			Type: reflect.TypeOf(""),
			Tag:  "",
		}
		result := buildColumnDefinition(field, dialect)
		if result != "" {
			t.Errorf("expected empty string, got '%s'", result)
		}
	})

	t.Run("skips field with db tag '-'", func(t *testing.T) {
		field := reflect.StructField{
			Name: "Ignored",
			Type: reflect.TypeOf(""),
			Tag:  `db:"-"`,
		}
		result := buildColumnDefinition(field, dialect)
		if result != "" {
			t.Errorf("expected empty string, got '%s'", result)
		}
	})

	t.Run("ID field gets PRIMARY KEY", func(t *testing.T) {
		field := reflect.StructField{
			Name: "ID",
			Type: reflect.TypeOf(int64(0)),
			Tag:  `db:"id"`,
		}
		result := buildColumnDefinition(field, dialect)
		if result == "" {
			t.Fatal("expected column definition, got empty")
		}
		// SQLite: INTEGER PRIMARY KEY
		if !contains(result, "PRIMARY KEY") {
			t.Errorf("expected PRIMARY KEY in '%s'", result)
		}
	})

	t.Run("string field gets NOT NULL", func(t *testing.T) {
		field := reflect.StructField{
			Name: "Name",
			Type: reflect.TypeOf(""),
			Tag:  `db:"name"`,
		}
		result := buildColumnDefinition(field, dialect)
		if !contains(result, "NOT NULL") {
			t.Errorf("expected NOT NULL in '%s'", result)
		}
	})

	t.Run("time.Time field gets DEFAULT CURRENT_TIMESTAMP for CreatedAt", func(t *testing.T) {
		field := reflect.StructField{
			Name: "CreatedAt",
			Type: reflect.TypeOf(time.Time{}),
			Tag:  `db:"created_at"`,
		}
		result := buildColumnDefinition(field, dialect)
		if !contains(result, "DEFAULT CURRENT_TIMESTAMP") {
			t.Errorf("expected DEFAULT CURRENT_TIMESTAMP in '%s'", result)
		}
	})

	t.Run("time.Time field gets DEFAULT CURRENT_TIMESTAMP for UpdatedAt", func(t *testing.T) {
		field := reflect.StructField{
			Name: "UpdatedAt",
			Type: reflect.TypeOf(time.Time{}),
			Tag:  `db:"updated_at"`,
		}
		result := buildColumnDefinition(field, dialect)
		if !contains(result, "DEFAULT CURRENT_TIMESTAMP") {
			t.Errorf("expected DEFAULT CURRENT_TIMESTAMP in '%s'", result)
		}
	})
}

// ========================================
// Tests for getSQLType
// ========================================

func TestGetSQLType(t *testing.T) {
	dialect := sqlite.New()

	tests := []struct {
		name     string
		goType   reflect.Type
		expected string
	}{
		{"string", reflect.TypeOf(""), "TEXT"},
		{"int", reflect.TypeOf(0), "INTEGER"},
		{"int64", reflect.TypeOf(int64(0)), "INTEGER"},
		{"bool", reflect.TypeOf(true), "INTEGER"}, // SQLite
		{"float64", reflect.TypeOf(0.0), "REAL"},
		{"float32", reflect.TypeOf(float32(0)), "REAL"},
		{"time.Time", reflect.TypeOf(time.Time{}), "DATETIME"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getSQLType(tt.goType, dialect)
			if result != tt.expected {
				t.Errorf("getSQLType(%s) = '%s', want '%s'", tt.name, result, tt.expected)
			}
		})
	}

	t.Run("unknown type returns TEXT", func(t *testing.T) {
		// A slice type should fallback to TEXT
		result := getSQLType(reflect.TypeOf([]byte{}), dialect)
		if result != "TEXT" {
			t.Errorf("expected TEXT for unknown type, got '%s'", result)
		}
	})
}

// ========================================
// Tests for isOptional
// ========================================

func TestIsOptional(t *testing.T) {
	t.Run("non-struct is not optional", func(t *testing.T) {
		if isOptional(reflect.TypeOf("")) {
			t.Error("string should not be optional")
		}
	})

	t.Run("regular struct is not optional", func(t *testing.T) {
		type NotOptional struct{ Value string }
		if isOptional(reflect.TypeOf(NotOptional{})) {
			t.Error("NotOptional should not be optional")
		}
	})
}

// ========================================
// Tests for getTableName
// ========================================

func TestGetTableName(t *testing.T) {
	t.Run("uses TableNamer interface", func(t *testing.T) {
		name := getTableName(&TestUserWithTableName{})
		if name != "custom_users" {
			t.Errorf("expected 'custom_users', got '%s'", name)
		}
	})

	t.Run("uses snake_case of type name", func(t *testing.T) {
		name := getTableName(TestUser{})
		if name != "test_user" {
			t.Errorf("expected 'test_user', got '%s'", name)
		}
	})

	t.Run("pointer type works", func(t *testing.T) {
		name := getTableName(&TestUser{})
		// Pointer dereference still uses type name
		if name != "test_user" {
			t.Errorf("expected 'test_user', got '%s'", name)
		}
	})
}

// ========================================
// Tests for toSnakeCase
// ========================================

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"TestUser", "test_user"},
		{"ID", "i_d"},
		{"Simple", "simple"},
		{"HTTPServer", "h_t_t_p_server"},
		{"CamelCase", "camel_case"},
		{"", ""},
		{"a", "a"},
		{"A", "a"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toSnakeCase(tt.input)
			if result != tt.expected {
				t.Errorf("toSnakeCase(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// helper
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
