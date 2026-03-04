package scanner

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/go-genus/genus"
	"github.com/go-genus/genus/core"
	"github.com/go-genus/genus/dialects"
	"github.com/go-genus/genus/query"

	_ "github.com/mattn/go-sqlite3"
)

// UserFields for type-safe queries
var UserFields = struct {
	IsActive query.BoolField
}{
	IsActive: query.NewBoolField("is_active"),
}

// setupDB creates an in-memory SQLite database for benchmarks.
func setupDB(b *testing.B) (*sql.DB, func()) {
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		b.Fatalf("failed to open db: %v", err)
	}

	// Create table
	_, err = sqlDB.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			email TEXT NOT NULL,
			age INTEGER NOT NULL,
			is_active BOOLEAN NOT NULL DEFAULT 1,
			score INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		b.Fatalf("failed to create table: %v", err)
	}

	// Insert test data
	stmt, _ := sqlDB.Prepare("INSERT INTO users (name, email, age, is_active, score) VALUES (?, ?, ?, ?, ?)")
	for i := 0; i < 1000; i++ {
		stmt.Exec(
			fmt.Sprintf("User %d", i),
			fmt.Sprintf("user%d@example.com", i),
			20+(i%50),
			i%2 == 0,
			i*10,
		)
	}
	stmt.Close()

	return sqlDB, func() { sqlDB.Close() }
}

// BenchmarkScan_Reflection uses the ORM's reflection-based scanning.
func BenchmarkScan_Reflection(b *testing.B) {
	sqlDB, cleanup := setupDB(b)
	defer cleanup()

	db := genus.NewWithLogger(sqlDB, dialects.DetectDialect("sqlite3"), &core.NoOpLogger{})
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = genus.Table[User](db).
			Where(UserFields.IsActive.Eq(true)).
			Find(ctx)
	}
}

// BenchmarkScan_Generated uses the generated, reflection-free scanner.
func BenchmarkScan_Generated(b *testing.B) {
	sqlDB, cleanup := setupDB(b)
	defer cleanup()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rows, err := sqlDB.QueryContext(ctx,
			"SELECT "+UserColumnsString()+" FROM users WHERE is_active = ?", true)
		if err != nil {
			b.Fatal(err)
		}
		_, _ = ScanUsers(rows)
		rows.Close()
	}
}

// BenchmarkScan_GeneratedWithCap uses generated scanner with pre-allocated capacity.
func BenchmarkScan_GeneratedWithCap(b *testing.B) {
	sqlDB, cleanup := setupDB(b)
	defer cleanup()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rows, err := sqlDB.QueryContext(ctx,
			"SELECT "+UserColumnsString()+" FROM users WHERE is_active = ?", true)
		if err != nil {
			b.Fatal(err)
		}
		_, _ = ScanUsersWithCap(rows, 500) // Pre-allocate for ~500 results
		rows.Close()
	}
}

// BenchmarkScan_RawSQL is the baseline with manual scanning.
func BenchmarkScan_RawSQL(b *testing.B) {
	sqlDB, cleanup := setupDB(b)
	defer cleanup()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rows, err := sqlDB.QueryContext(ctx,
			"SELECT id, created_at, updated_at, name, email, age, is_active, score FROM users WHERE is_active = ?", true)
		if err != nil {
			b.Fatal(err)
		}
		var users []User
		for rows.Next() {
			var u User
			rows.Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt, &u.Name, &u.Email, &u.Age, &u.IsActive, &u.Score)
			users = append(users, u)
		}
		rows.Close()
	}
}
