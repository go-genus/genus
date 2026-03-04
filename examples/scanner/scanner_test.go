package scanner

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/GabrielOnRails/genus"
	"github.com/GabrielOnRails/genus/core"
	"github.com/GabrielOnRails/genus/dialects"

	_ "github.com/mattn/go-sqlite3"
)

func TestRowCounts(t *testing.T) {
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

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
		t.Fatal(err)
	}

	// Insert 1000 rows
	stmt, _ := sqlDB.Prepare("INSERT INTO users (name, email, age, is_active, score) VALUES (?, ?, ?, ?, ?)")
	for i := 0; i < 1000; i++ {
		stmt.Exec(fmt.Sprintf("User %d", i), fmt.Sprintf("user%d@example.com", i), 20+(i%50), i%2 == 0, i*10)
	}
	stmt.Close()

	ctx := context.Background()
	db := genus.NewWithLogger(sqlDB, dialects.DetectDialect("sqlite3"), &core.NoOpLogger{})

	// Test ORM Find
	users, err := genus.Table[User](db).Where(UserFields.IsActive.Eq(true)).Find(ctx)
	if err != nil {
		t.Fatalf("ORM Find error: %v", err)
	}
	t.Logf("ORM Find returned %d users", len(users))

	// Test Generated Scanner
	rows, err := sqlDB.QueryContext(ctx, "SELECT "+UserColumnsString()+" FROM users WHERE is_active = ?", true)
	if err != nil {
		t.Fatal(err)
	}
	genUsers, err := ScanUsers(rows)
	rows.Close()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Generated Scanner returned %d users", len(genUsers))

	// Test Raw SQL
	rows2, _ := sqlDB.QueryContext(ctx, "SELECT id, created_at, updated_at, name, email, age, is_active, score FROM users WHERE is_active = ?", true)
	var rawCount int
	for rows2.Next() {
		rawCount++
	}
	rows2.Close()
	t.Logf("Raw SQL returned %d rows", rawCount)

	if len(users) != 500 {
		t.Errorf("Expected 500 users from ORM, got %d", len(users))
	}
	if len(genUsers) != 500 {
		t.Errorf("Expected 500 users from Generated, got %d", len(genUsers))
	}
}
