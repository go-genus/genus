package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/go-genus/genus"
	"github.com/go-genus/genus/core"
	"github.com/go-genus/genus/dialects/mysql"
	"github.com/go-genus/genus/dialects/postgres"
	"github.com/go-genus/genus/dialects/sqlite"
	"github.com/go-genus/genus/query"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

// User is the example struct for all databases.
type User struct {
	core.Model
	Name     string                `db:"name"`
	Email    core.Optional[string] `db:"email"`
	Age      core.Optional[int]    `db:"age"`
	IsActive bool                  `db:"is_active"`
}

// UserFields - typed fields (can be generated with genus generate)
var UserFields = struct {
	ID        query.Int64Field
	Name      query.StringField
	Email     query.OptionalStringField
	Age       query.OptionalIntField
	IsActive  query.BoolField
	CreatedAt query.StringField
	UpdatedAt query.StringField
}{
	ID:        query.NewInt64Field("id"),
	Name:      query.NewStringField("name"),
	Email:     query.NewOptionalStringField("email"),
	Age:       query.NewOptionalIntField("age"),
	IsActive:  query.NewBoolField("is_active"),
	CreatedAt: query.NewStringField("created_at"),
	UpdatedAt: query.NewStringField("updated_at"),
}

func main() {
	fmt.Println("=== Genus Multi-Database Example ===")
	fmt.Println()

	ctx := context.Background()

	// 1. PostgreSQL
	fmt.Println("1. PostgreSQL")
	if err := runPostgreSQL(ctx); err != nil {
		fmt.Printf("   [SKIP] PostgreSQL not available: %v\n", err)
	} else {
		fmt.Println("   ✅ PostgreSQL working!")
	}

	fmt.Println()

	// 2. MySQL
	fmt.Println("2. MySQL")
	if err := runMySQL(ctx); err != nil {
		fmt.Printf("   [SKIP] MySQL not available: %v\n", err)
	} else {
		fmt.Println("   ✅ MySQL working!")
	}

	fmt.Println()

	// 3. SQLite
	fmt.Println("3. SQLite")
	if err := runSQLite(ctx); err != nil {
		fmt.Printf("   [ERROR] SQLite failed: %v\n", err)
	} else {
		fmt.Println("   ✅ SQLite working!")
	}

	fmt.Println("\n=== Example completed! ===")
}

func runPostgreSQL(ctx context.Context) error {
	// Connect to PostgreSQL
	dsn := "user=postgres password=postgres dbname=genus_test sslmode=disable"
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return err
	}

	// Create Genus with PostgreSQL dialect
	g := genus.NewWithLogger(db, postgres.New(), core.NewDefaultLogger(true))

	// Create table
	createTablePostgreSQL(db)

	// Test CRUD
	return testCRUD(ctx, g, "PostgreSQL")
}

func runMySQL(ctx context.Context) error {
	// Connect to MySQL
	dsn := "root:password@tcp(localhost:3306)/genus_test?parseTime=true"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return err
	}

	// Create Genus with MySQL dialect
	g := genus.NewWithLogger(db, mysql.New(), core.NewDefaultLogger(true))

	// Create table
	createTableMySQL(db)

	// Test CRUD
	return testCRUD(ctx, g, "MySQL")
}

func runSQLite(ctx context.Context) error {
	// Connect to SQLite (in-memory)
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return err
	}
	defer db.Close()

	// Create Genus with SQLite dialect
	g := genus.NewWithLogger(db, sqlite.New(), core.NewDefaultLogger(true))

	// Create table
	createTableSQLite(db)

	// Test CRUD
	return testCRUD(ctx, g, "SQLite")
}

func testCRUD(ctx context.Context, g *genus.Genus, dbName string) error {
	fmt.Printf("   Testing CRUD operations on %s...\n", dbName)

	// Create
	user := &User{
		Name:     "Alice",
		Email:    core.Some("alice@example.com"),
		Age:      core.Some(25),
		IsActive: true,
	}

	if err := g.DB().Create(ctx, user); err != nil {
		return fmt.Errorf("create failed: %w", err)
	}
	fmt.Printf("   - Created user with ID: %d\n", user.ID)

	// Find
	users, err := genus.Table[User](g).
		Where(UserFields.Name.Eq("Alice")).
		Find(ctx)

	if err != nil {
		return fmt.Errorf("find failed: %w", err)
	}

	if len(users) == 0 {
		return fmt.Errorf("no users found")
	}

	fmt.Printf("   - Found %d user(s)\n", len(users))
	fmt.Printf("   - User: %s (email: %s, age: %d)\n",
		users[0].Name,
		users[0].Email.GetOrDefault("N/A"),
		users[0].Age.GetOrDefault(0))

	// Update
	user.Age = core.Some(26)
	if err := g.DB().Update(ctx, user); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}
	fmt.Printf("   - Updated user age to 26\n")

	// Count
	count, err := genus.Table[User](g).
		Where(UserFields.IsActive.Eq(true)).
		Count(ctx)

	if err != nil {
		return fmt.Errorf("count failed: %w", err)
	}
	fmt.Printf("   - Active users count: %d\n", count)

	// Delete
	if err := g.DB().Delete(ctx, user); err != nil {
		return fmt.Errorf("delete failed: %w", err)
	}
	fmt.Printf("   - Deleted user\n")

	return nil
}

func createTablePostgreSQL(db *sql.DB) {
	schema := `
	DROP TABLE IF EXISTS users;
	CREATE TABLE users (
		id SERIAL PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		email VARCHAR(255),
		age INTEGER,
		is_active BOOLEAN NOT NULL DEFAULT true,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP NOT NULL DEFAULT NOW()
	);
	`
	if _, err := db.Exec(schema); err != nil {
		log.Printf("Warning: failed to create table: %v", err)
	}
}

func createTableMySQL(db *sql.DB) {
	schema := `
	DROP TABLE IF EXISTS users;
	CREATE TABLE users (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		email VARCHAR(255),
		age INT,
		is_active BOOLEAN NOT NULL DEFAULT true,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
	);
	`
	if _, err := db.Exec(schema); err != nil {
		log.Printf("Warning: failed to create table: %v", err)
	}
}

func createTableSQLite(db *sql.DB) {
	schema := `
	DROP TABLE IF EXISTS users;
	CREATE TABLE users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		email TEXT,
		age INTEGER,
		is_active INTEGER NOT NULL DEFAULT 1,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	`
	if _, err := db.Exec(schema); err != nil {
		log.Printf("Warning: failed to create table: %v", err)
	}
}
