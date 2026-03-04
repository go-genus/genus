package benchmarks

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/go-genus/genus"
	"github.com/go-genus/genus/core"
	"github.com/go-genus/genus/dialects"

	_ "github.com/mattn/go-sqlite3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestVerifyRowCounts(t *testing.T) {
	// Setup GORM
	gormDB, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Silent),
		SkipDefaultTransaction: true,
	})
	gormSQL, _ := gormDB.DB()

	// Setup Genus
	genusSQL, _ := sql.Open("sqlite3", ":memory:")
	genusDB := genus.NewWithLogger(genusSQL, dialects.DetectDialect("sqlite3"), &core.NoOpLogger{})

	// Create identical tables
	for _, db := range []*sql.DB{gormSQL, genusSQL} {
		db.Exec(`CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT, email TEXT, age INTEGER,
			is_active BOOLEAN, score INTEGER,
			created_at DATETIME, updated_at DATETIME
		)`)
		stmt, _ := db.Prepare("INSERT INTO users (name, email, age, is_active, score) VALUES (?, ?, ?, ?, ?)")
		for i := 0; i < 1000; i++ {
			stmt.Exec(fmt.Sprintf("User %d", i), fmt.Sprintf("user%d@example.com", i), 20+(i%50), i%2 == 0, i*10)
		}
		stmt.Close()
	}

	ctx := context.Background()

	// Test GORM count
	var gormUsers []BenchUser
	gormDB.Where("is_active = ?", true).Find(&gormUsers)
	t.Logf("GORM returned %d users", len(gormUsers))

	// Test Genus count (using BenchUser which doesn't have core.Model)
	genusUsers, err := genus.Table[BenchUser](genusDB).
		Where(BenchUserFields.IsActive.Eq(true)).
		Find(ctx)
	if err != nil {
		t.Logf("Genus error: %v", err)
	}
	t.Logf("Genus returned %d users", len(genusUsers))

	// Test Raw SQL count
	rows, _ := genusSQL.QueryContext(ctx, "SELECT COUNT(*) FROM users WHERE is_active = ?", true)
	var count int
	if rows.Next() {
		rows.Scan(&count)
	}
	rows.Close()
	t.Logf("Raw SQL count: %d", count)

	// Verify
	if len(gormUsers) != 500 {
		t.Errorf("GORM should return 500, got %d", len(gormUsers))
	}
	if len(genusUsers) != 500 {
		t.Errorf("Genus should return 500, got %d", len(genusUsers))
	}
}
