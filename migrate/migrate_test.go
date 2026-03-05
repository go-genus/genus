package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/go-genus/genus/core"
	"github.com/go-genus/genus/dialects/sqlite"
)

// ========================================
// Test Helpers
// ========================================

// mockLogger implements core.Logger for testing.
type mockLogger struct {
	queries []string
	errors  []string
}

func newMockLogger() *mockLogger {
	return &mockLogger{}
}

func (l *mockLogger) LogQuery(query string, args []interface{}, duration int64) {
	l.queries = append(l.queries, query)
}

func (l *mockLogger) LogError(query string, args []interface{}, err error) {
	l.errors = append(l.errors, query)
}

// newTestDB creates a new in-memory SQLite database for testing.
// Uses shared cache mode so that multiple connections share the same
// in-memory database (required for SQLite :memory: with connection pools).
// Uses WAL journal mode and busy_timeout to avoid deadlocks when
// runMigration opens a transaction and the migration function also
// accesses the db directly.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", "file::memory:?cache=shared&_busy_timeout=10000&_journal_mode=WAL")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// newTestMigrator creates a Migrator with default config and test helpers.
func newTestMigrator(t *testing.T) (*Migrator, *sql.DB, core.Dialect, *mockLogger) {
	t.Helper()
	db := newTestDB(t)
	dialect := sqlite.New()
	logger := newMockLogger()
	m := New(db, dialect, logger, Config{})
	return m, db, dialect, logger
}

// sampleMigrations returns a set of migrations for testing.
func sampleMigrations(t *testing.T) []Migration {
	t.Helper()
	return []Migration{
		{
			Version: 1,
			Name:    "create_users",
			Up: func(ctx context.Context, db *sql.DB, dialect core.Dialect) error {
				_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY, name TEXT NOT NULL)`)
				return err
			},
			Down: func(ctx context.Context, db *sql.DB, dialect core.Dialect) error {
				_, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS users`)
				return err
			},
		},
		{
			Version: 2,
			Name:    "create_posts",
			Up: func(ctx context.Context, db *sql.DB, dialect core.Dialect) error {
				_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS posts (id INTEGER PRIMARY KEY, title TEXT NOT NULL, user_id INTEGER)`)
				return err
			},
			Down: func(ctx context.Context, db *sql.DB, dialect core.Dialect) error {
				_, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS posts`)
				return err
			},
		},
	}
}

// ========================================
// Tests for New
// ========================================

func TestNew(t *testing.T) {
	t.Run("default table name", func(t *testing.T) {
		db := newTestDB(t)
		dialect := sqlite.New()
		logger := newMockLogger()
		m := New(db, dialect, logger, Config{})

		if m.tableName != "schema_migrations" {
			t.Errorf("expected table name 'schema_migrations', got '%s'", m.tableName)
		}
		if m.db != db {
			t.Error("expected db to be set")
		}
		if m.dialect != dialect {
			t.Error("expected dialect to be set")
		}
		if m.logger != logger {
			t.Error("expected logger to be set")
		}
		if len(m.migrations) != 0 {
			t.Errorf("expected 0 migrations, got %d", len(m.migrations))
		}
	})

	t.Run("custom table name", func(t *testing.T) {
		db := newTestDB(t)
		dialect := sqlite.New()
		logger := newMockLogger()
		m := New(db, dialect, logger, Config{TableName: "my_migrations"})

		if m.tableName != "my_migrations" {
			t.Errorf("expected table name 'my_migrations', got '%s'", m.tableName)
		}
	})
}

// ========================================
// Tests for Register / RegisterMultiple
// ========================================

func TestRegister(t *testing.T) {
	m, _, _, _ := newTestMigrator(t)

	m.Register(Migration{Version: 1, Name: "first"})
	if len(m.migrations) != 1 {
		t.Fatalf("expected 1 migration, got %d", len(m.migrations))
	}
	if m.migrations[0].Version != 1 {
		t.Errorf("expected version 1, got %d", m.migrations[0].Version)
	}
}

func TestRegisterMultiple(t *testing.T) {
	m, _, _, _ := newTestMigrator(t)

	migrations := []Migration{
		{Version: 1, Name: "first"},
		{Version: 2, Name: "second"},
		{Version: 3, Name: "third"},
	}
	m.RegisterMultiple(migrations)

	if len(m.migrations) != 3 {
		t.Fatalf("expected 3 migrations, got %d", len(m.migrations))
	}
	if m.migrations[2].Name != "third" {
		t.Errorf("expected 'third', got '%s'", m.migrations[2].Name)
	}
}

// ========================================
// Tests for Up
// ========================================

func TestUp(t *testing.T) {
	t.Run("applies all pending migrations", func(t *testing.T) {
		m, db, _, _ := newTestMigrator(t)
		ctx := context.Background()

		m.RegisterMultiple(sampleMigrations(t))

		if err := m.Up(ctx); err != nil {
			t.Fatalf("Up failed: %v", err)
		}

		// Verify tables were created
		var count int
		err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
		if err != nil {
			t.Fatalf("users table not created: %v", err)
		}

		err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM posts").Scan(&count)
		if err != nil {
			t.Fatalf("posts table not created: %v", err)
		}

		// Verify migrations were recorded
		rows, err := db.QueryContext(ctx, `SELECT version FROM "schema_migrations" ORDER BY version`)
		if err != nil {
			t.Fatalf("failed to query schema_migrations: %v", err)
		}
		defer rows.Close()

		var versions []int64
		for rows.Next() {
			var v int64
			if err := rows.Scan(&v); err != nil {
				t.Fatalf("failed to scan version: %v", err)
			}
			versions = append(versions, v)
		}

		if len(versions) != 2 {
			t.Fatalf("expected 2 applied versions, got %d", len(versions))
		}
		if versions[0] != 1 || versions[1] != 2 {
			t.Errorf("expected versions [1, 2], got %v", versions)
		}
	})

	t.Run("skips already applied migrations", func(t *testing.T) {
		m, _, _, _ := newTestMigrator(t)
		ctx := context.Background()

		m.RegisterMultiple(sampleMigrations(t))

		// Apply once
		if err := m.Up(ctx); err != nil {
			t.Fatalf("first Up failed: %v", err)
		}

		// Apply again - should be idempotent
		if err := m.Up(ctx); err != nil {
			t.Fatalf("second Up failed: %v", err)
		}
	})

	t.Run("applies in version order", func(t *testing.T) {
		m, _, _, logger := newTestMigrator(t)
		ctx := context.Background()

		var order []int64
		// Register in reverse order
		m.Register(Migration{
			Version: 3,
			Name:    "third",
			Up: func(ctx context.Context, db *sql.DB, d core.Dialect) error {
				order = append(order, 3)
				return nil
			},
		})
		m.Register(Migration{
			Version: 1,
			Name:    "first",
			Up: func(ctx context.Context, db *sql.DB, d core.Dialect) error {
				order = append(order, 1)
				return nil
			},
		})
		m.Register(Migration{
			Version: 2,
			Name:    "second",
			Up: func(ctx context.Context, db *sql.DB, d core.Dialect) error {
				order = append(order, 2)
				return nil
			},
		})

		if err := m.Up(ctx); err != nil {
			t.Fatalf("Up failed: %v", err)
		}

		if len(order) != 3 || order[0] != 1 || order[1] != 2 || order[2] != 3 {
			t.Errorf("expected execution order [1, 2, 3], got %v", order)
		}

		// Verify logger was called
		if len(logger.queries) == 0 {
			t.Error("expected logger to have logged queries")
		}
	})

	t.Run("returns error on migration failure", func(t *testing.T) {
		m, _, _, _ := newTestMigrator(t)
		ctx := context.Background()

		m.Register(Migration{
			Version: 1,
			Name:    "failing_migration",
			Up: func(ctx context.Context, db *sql.DB, d core.Dialect) error {
				return fmt.Errorf("intentional error")
			},
		})

		err := m.Up(ctx)
		if err == nil {
			t.Fatal("expected error from Up, got nil")
		}
		if got := err.Error(); got != "failed to apply migration 1 (failing_migration): intentional error" {
			t.Errorf("unexpected error message: %s", got)
		}
	})

	t.Run("no migrations registered", func(t *testing.T) {
		m, _, _, _ := newTestMigrator(t)
		ctx := context.Background()

		if err := m.Up(ctx); err != nil {
			t.Fatalf("Up with no migrations should succeed, got: %v", err)
		}
	})
}

// ========================================
// Tests for Down
// ========================================

func TestDown(t *testing.T) {
	t.Run("reverts last applied migration", func(t *testing.T) {
		m, db, _, _ := newTestMigrator(t)
		ctx := context.Background()

		m.RegisterMultiple(sampleMigrations(t))

		// Apply all
		if err := m.Up(ctx); err != nil {
			t.Fatalf("Up failed: %v", err)
		}

		// Revert last
		if err := m.Down(ctx); err != nil {
			t.Fatalf("Down failed: %v", err)
		}

		// posts table should be dropped
		_, err := db.ExecContext(ctx, "SELECT 1 FROM posts")
		if err == nil {
			t.Error("expected posts table to be dropped")
		}

		// users table should still exist
		_, err = db.ExecContext(ctx, "SELECT 1 FROM users")
		if err != nil {
			t.Error("expected users table to still exist")
		}

		// Only version 1 should remain in schema_migrations
		var count int
		db.QueryRowContext(ctx, `SELECT COUNT(*) FROM "schema_migrations"`).Scan(&count)
		if count != 1 {
			t.Errorf("expected 1 applied migration, got %d", count)
		}
	})

	t.Run("no migrations applied", func(t *testing.T) {
		m, _, _, logger := newTestMigrator(t)
		ctx := context.Background()

		// Need to create the table first
		if err := m.Up(ctx); err != nil {
			t.Fatalf("Up failed: %v", err)
		}

		if err := m.Down(ctx); err != nil {
			t.Fatalf("Down with no migrations should succeed, got: %v", err)
		}

		// Logger should have logged "No migrations to revert"
		found := false
		for _, q := range logger.queries {
			if q == "No migrations to revert" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected 'No migrations to revert' log message")
		}
	})

	t.Run("migration without Down function", func(t *testing.T) {
		m, _, _, _ := newTestMigrator(t)
		ctx := context.Background()

		m.Register(Migration{
			Version: 1,
			Name:    "no_down",
			Up: func(ctx context.Context, db *sql.DB, d core.Dialect) error {
				return nil
			},
			Down: nil,
		})

		if err := m.Up(ctx); err != nil {
			t.Fatalf("Up failed: %v", err)
		}

		err := m.Down(ctx)
		if err == nil {
			t.Fatal("expected error for migration without Down, got nil")
		}
		if got := err.Error(); got != "failed to revert migration 1 (no_down): migration 1 has no Down function" {
			t.Errorf("unexpected error: %s", got)
		}
	})

	t.Run("down with failing migration", func(t *testing.T) {
		m, _, _, _ := newTestMigrator(t)
		ctx := context.Background()

		m.Register(Migration{
			Version: 1,
			Name:    "fail_down",
			Up: func(ctx context.Context, db *sql.DB, d core.Dialect) error {
				return nil
			},
			Down: func(ctx context.Context, db *sql.DB, d core.Dialect) error {
				return fmt.Errorf("down error")
			},
		})

		if err := m.Up(ctx); err != nil {
			t.Fatalf("Up failed: %v", err)
		}

		err := m.Down(ctx)
		if err == nil {
			t.Fatal("expected error from Down")
		}
	})

	t.Run("reverts only the latest migration", func(t *testing.T) {
		m, _, _, _ := newTestMigrator(t)
		ctx := context.Background()

		m.RegisterMultiple(sampleMigrations(t))

		if err := m.Up(ctx); err != nil {
			t.Fatalf("Up failed: %v", err)
		}

		// Down once
		if err := m.Down(ctx); err != nil {
			t.Fatalf("first Down failed: %v", err)
		}

		// Down again
		if err := m.Down(ctx); err != nil {
			t.Fatalf("second Down failed: %v", err)
		}

		// Down with nothing left
		if err := m.Down(ctx); err != nil {
			t.Fatalf("third Down should succeed, got: %v", err)
		}
	})
}

// ========================================
// Tests for Status
// ========================================

func TestStatus(t *testing.T) {
	t.Run("shows all migration statuses", func(t *testing.T) {
		m, _, _, _ := newTestMigrator(t)
		ctx := context.Background()

		m.RegisterMultiple(sampleMigrations(t))

		// Apply first migration only
		firstOnly := Migration{
			Version: 1,
			Name:    "create_users",
			Up: func(ctx context.Context, db *sql.DB, d core.Dialect) error {
				_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY, name TEXT NOT NULL)`)
				return err
			},
		}
		temp := New(m.db, m.dialect, m.logger, Config{})
		temp.Register(firstOnly)
		if err := temp.Up(ctx); err != nil {
			t.Fatalf("Up failed: %v", err)
		}

		statuses, err := m.Status(ctx)
		if err != nil {
			t.Fatalf("Status failed: %v", err)
		}

		if len(statuses) != 2 {
			t.Fatalf("expected 2 statuses, got %d", len(statuses))
		}

		// First should be applied
		if !statuses[0].Applied {
			t.Error("expected first migration to be applied")
		}
		if statuses[0].Version != 1 {
			t.Errorf("expected version 1, got %d", statuses[0].Version)
		}

		// Second should be pending
		if statuses[1].Applied {
			t.Error("expected second migration to be pending")
		}
		if statuses[1].Version != 2 {
			t.Errorf("expected version 2, got %d", statuses[1].Version)
		}
	})

	t.Run("no migrations registered", func(t *testing.T) {
		m, _, _, _ := newTestMigrator(t)
		ctx := context.Background()

		statuses, err := m.Status(ctx)
		if err != nil {
			t.Fatalf("Status failed: %v", err)
		}
		if len(statuses) != 0 {
			t.Errorf("expected 0 statuses, got %d", len(statuses))
		}
	})

	t.Run("status is sorted by version", func(t *testing.T) {
		m, _, _, _ := newTestMigrator(t)
		ctx := context.Background()

		// Register in reverse order
		m.Register(Migration{Version: 3, Name: "third"})
		m.Register(Migration{Version: 1, Name: "first"})
		m.Register(Migration{Version: 2, Name: "second"})

		statuses, err := m.Status(ctx)
		if err != nil {
			t.Fatalf("Status failed: %v", err)
		}

		if statuses[0].Version != 1 || statuses[1].Version != 2 || statuses[2].Version != 3 {
			t.Errorf("expected sorted versions, got %d, %d, %d",
				statuses[0].Version, statuses[1].Version, statuses[2].Version)
		}
	})
}

// ========================================
// Tests for createMigrationsTable
// ========================================

func TestCreateMigrationsTable(t *testing.T) {
	t.Run("creates table with SQLite types", func(t *testing.T) {
		m, db, _, _ := newTestMigrator(t)
		ctx := context.Background()

		if err := m.createMigrationsTable(ctx); err != nil {
			t.Fatalf("createMigrationsTable failed: %v", err)
		}

		// Verify table exists by inserting data
		_, err := db.ExecContext(ctx, `INSERT INTO "schema_migrations" (version, name, applied_at) VALUES (1, 'test', '2024-01-01')`)
		if err != nil {
			t.Fatalf("failed to insert into schema_migrations: %v", err)
		}
	})

	t.Run("idempotent creation", func(t *testing.T) {
		m, _, _, _ := newTestMigrator(t)
		ctx := context.Background()

		if err := m.createMigrationsTable(ctx); err != nil {
			t.Fatalf("first call failed: %v", err)
		}
		if err := m.createMigrationsTable(ctx); err != nil {
			t.Fatalf("second call failed: %v", err)
		}
	})

	t.Run("custom table name", func(t *testing.T) {
		db := newTestDB(t)
		dialect := sqlite.New()
		logger := newMockLogger()
		m := New(db, dialect, logger, Config{TableName: "custom_migrations"})
		ctx := context.Background()

		if err := m.createMigrationsTable(ctx); err != nil {
			t.Fatalf("createMigrationsTable failed: %v", err)
		}

		// Verify custom table exists
		_, err := db.ExecContext(ctx, `INSERT INTO "custom_migrations" (version, name, applied_at) VALUES (1, 'test', '2024-01-01')`)
		if err != nil {
			t.Fatalf("custom table not created: %v", err)
		}
	})
}

// ========================================
// Tests for getAppliedMigrations
// ========================================

func TestGetAppliedMigrations(t *testing.T) {
	t.Run("returns applied versions", func(t *testing.T) {
		m, db, _, _ := newTestMigrator(t)
		ctx := context.Background()

		// Create table and insert some versions
		m.createMigrationsTable(ctx)
		db.ExecContext(ctx, `INSERT INTO "schema_migrations" (version, name, applied_at) VALUES (1, 'first', '2024-01-01')`)
		db.ExecContext(ctx, `INSERT INTO "schema_migrations" (version, name, applied_at) VALUES (3, 'third', '2024-01-03')`)

		applied, err := m.getAppliedMigrations(ctx)
		if err != nil {
			t.Fatalf("getAppliedMigrations failed: %v", err)
		}

		if !applied[1] {
			t.Error("expected version 1 to be applied")
		}
		if applied[2] {
			t.Error("expected version 2 to NOT be applied")
		}
		if !applied[3] {
			t.Error("expected version 3 to be applied")
		}
	})

	t.Run("empty table returns empty map", func(t *testing.T) {
		m, _, _, _ := newTestMigrator(t)
		ctx := context.Background()

		m.createMigrationsTable(ctx)

		applied, err := m.getAppliedMigrations(ctx)
		if err != nil {
			t.Fatalf("getAppliedMigrations failed: %v", err)
		}

		if len(applied) != 0 {
			t.Errorf("expected empty map, got %d entries", len(applied))
		}
	})
}

// ========================================
// Tests for runMigration
// ========================================

func TestRunMigration(t *testing.T) {
	t.Run("up records migration in control table", func(t *testing.T) {
		m, db, dialect, _ := newTestMigrator(t)
		ctx := context.Background()

		m.createMigrationsTable(ctx)

		migration := Migration{
			Version: 1,
			Name:    "test_migration",
			Up: func(ctx context.Context, db *sql.DB, d core.Dialect) error {
				return nil
			},
		}

		if err := m.runMigration(ctx, migration, true); err != nil {
			t.Fatalf("runMigration Up failed: %v", err)
		}

		// Verify recorded
		var name string
		err := db.QueryRowContext(ctx, fmt.Sprintf(`SELECT name FROM %s WHERE version = ?`, dialect.QuoteIdentifier(m.tableName)), 1).Scan(&name)
		if err != nil {
			t.Fatalf("migration not recorded: %v", err)
		}
		if name != "test_migration" {
			t.Errorf("expected name 'test_migration', got '%s'", name)
		}
	})

	t.Run("down removes migration from control table", func(t *testing.T) {
		m, db, _, _ := newTestMigrator(t)
		ctx := context.Background()

		m.createMigrationsTable(ctx)

		migration := Migration{
			Version: 1,
			Name:    "test_migration",
			Up: func(ctx context.Context, db *sql.DB, d core.Dialect) error {
				return nil
			},
			Down: func(ctx context.Context, db *sql.DB, d core.Dialect) error {
				return nil
			},
		}

		// Apply
		m.runMigration(ctx, migration, true)

		// Revert
		if err := m.runMigration(ctx, migration, false); err != nil {
			t.Fatalf("runMigration Down failed: %v", err)
		}

		// Verify removed
		var count int
		db.QueryRowContext(ctx, `SELECT COUNT(*) FROM "schema_migrations" WHERE version = ?`, 1).Scan(&count)
		if count != 0 {
			t.Errorf("expected 0 records, got %d", count)
		}
	})

	t.Run("down without Down function returns error", func(t *testing.T) {
		m, _, _, _ := newTestMigrator(t)
		ctx := context.Background()

		m.createMigrationsTable(ctx)

		migration := Migration{
			Version: 1,
			Name:    "no_down",
			Up: func(ctx context.Context, db *sql.DB, d core.Dialect) error {
				return nil
			},
			Down: nil,
		}

		err := m.runMigration(ctx, migration, false)
		if err == nil {
			t.Fatal("expected error for nil Down")
		}
	})
}

// ========================================
// Tests for MigrationStatus
// ========================================

func TestMigrationStatus(t *testing.T) {
	s := MigrationStatus{
		Version: 1,
		Name:    "test",
		Applied: true,
	}

	if s.Version != 1 {
		t.Errorf("expected version 1, got %d", s.Version)
	}
	if s.Name != "test" {
		t.Errorf("expected name 'test', got '%s'", s.Name)
	}
	if !s.Applied {
		t.Error("expected applied to be true")
	}
}
