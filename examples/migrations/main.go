package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/go-genus/genus"
	"github.com/go-genus/genus/core"
	"github.com/go-genus/genus/dialects/postgres"
	"github.com/go-genus/genus/migrate"
	"github.com/go-genus/genus/query"

	_ "github.com/lib/pq"
)

// User model
type User struct {
	core.Model
	Name     string                 `db:"name"`
	Email    core.Optional[string]  `db:"email"`
	Username string                 `db:"username"`
	IsActive bool                   `db:"is_active"`
}

// Product model
type Product struct {
	core.Model
	Name        string                 `db:"name"`
	Description core.Optional[string]  `db:"description"`
	Price       float64                `db:"price"`
	Stock       int                    `db:"stock"`
}

// UserFields
var UserFields = struct {
	ID       query.Int64Field
	Name     query.StringField
	Email    query.OptionalStringField
	Username query.StringField
	IsActive query.BoolField
}{
	ID:       query.NewInt64Field("id"),
	Name:     query.NewStringField("name"),
	Email:    query.NewOptionalStringField("email"),
	Username: query.NewStringField("username"),
	IsActive: query.NewBoolField("is_active"),
}

func main() {
	fmt.Println("=== Genus Migrations Example ===")
	fmt.Println()

	ctx := context.Background()

	// Conectar ao banco de dados
	dsn := "postgres://postgres:postgres@localhost:5432/genus_migrations?sslmode=disable"
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Printf("[SKIP] Database not available: %v", err)
		log.Println("\nThis example requires a PostgreSQL database.")
		log.Println("Run: docker run -d -p 5432:5432 -e POSTGRES_PASSWORD=postgres postgres")
		log.Println("Then: createdb genus_migrations")
		return
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Printf("[SKIP] Database not available: %v", err)
		return
	}

	dialect := postgres.New()
	logger := core.NewDefaultLogger(true)

	// Demonstração 1: AutoMigrate (desenvolvimento rápido)
	fmt.Println("1. AutoMigrate (Development)")
	fmt.Println("   Creating tables from structs...")

	if err := migrate.AutoMigrate(ctx, db, dialect, User{}, Product{}); err != nil {
		log.Fatalf("AutoMigrate failed: %v", err)
	}

	fmt.Println("   ✅ Tables created successfully!")
	fmt.Println()

	// Testar inserção
	g := genus.NewWithLogger(db, dialect, logger)

	user := &User{
		Name:     "Alice",
		Email:    core.Some("alice@example.com"),
		Username: "alice",
		IsActive: true,
	}

	if err := g.DB().Create(ctx, user); err != nil {
		log.Fatalf("Create failed: %v", err)
	}

	fmt.Printf("   Created user with ID: %d\n\n", user.ID)

	// Demonstração 2: Manual Migrations (produção)
	fmt.Println("2. Manual Migrations (Production)")
	fmt.Println("   Setting up migrator...")

	migrator := migrate.New(db, dialect, logger, migrate.Config{
		TableName: "schema_migrations",
	})

	// Registrar migrations
	migrations := []migrate.Migration{
		{
			Version: 1,
			Name:    "create_users_table",
			Up: func(ctx context.Context, db *sql.DB, dialect core.Dialect) error {
				query := `
					CREATE TABLE IF NOT EXISTS users_v2 (
						id SERIAL PRIMARY KEY,
						name VARCHAR(255) NOT NULL,
						email VARCHAR(255),
						username VARCHAR(255) UNIQUE NOT NULL,
						is_active BOOLEAN NOT NULL DEFAULT true,
						created_at TIMESTAMP NOT NULL DEFAULT NOW(),
						updated_at TIMESTAMP NOT NULL DEFAULT NOW()
					);
				`
				_, err := db.ExecContext(ctx, query)
				return err
			},
			Down: func(ctx context.Context, db *sql.DB, dialect core.Dialect) error {
				_, err := db.ExecContext(ctx, "DROP TABLE IF EXISTS users_v2")
				return err
			},
		},
		{
			Version: 2,
			Name:    "add_users_indexes",
			Up: func(ctx context.Context, db *sql.DB, dialect core.Dialect) error {
				queries := []string{
					"CREATE INDEX IF NOT EXISTS idx_users_v2_email ON users_v2(email)",
					"CREATE INDEX IF NOT EXISTS idx_users_v2_username ON users_v2(username)",
				}
				for _, query := range queries {
					if _, err := db.ExecContext(ctx, query); err != nil {
						return err
					}
				}
				return nil
			},
			Down: func(ctx context.Context, db *sql.DB, dialect core.Dialect) error {
				queries := []string{
					"DROP INDEX IF EXISTS idx_users_v2_email",
					"DROP INDEX IF EXISTS idx_users_v2_username",
				}
				for _, query := range queries {
					if _, err := db.ExecContext(ctx, query); err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			Version: 3,
			Name:    "create_products_table",
			Up: func(ctx context.Context, db *sql.DB, dialect core.Dialect) error {
				query := `
					CREATE TABLE IF NOT EXISTS products_v2 (
						id SERIAL PRIMARY KEY,
						name VARCHAR(255) NOT NULL,
						description TEXT,
						price DECIMAL(10, 2) NOT NULL,
						stock INTEGER NOT NULL DEFAULT 0,
						created_at TIMESTAMP NOT NULL DEFAULT NOW(),
						updated_at TIMESTAMP NOT NULL DEFAULT NOW()
					);
				`
				_, err := db.ExecContext(ctx, query)
				return err
			},
			Down: func(ctx context.Context, db *sql.DB, dialect core.Dialect) error {
				_, err := db.ExecContext(ctx, "DROP TABLE IF EXISTS products_v2")
				return err
			},
		},
	}

	migrator.RegisterMultiple(migrations)

	// Mostrar status antes
	fmt.Println("\n   Migration Status (before):")
	statuses, err := migrator.Status(ctx)
	if err != nil {
		log.Fatalf("Failed to get status: %v", err)
	}
	printStatus(statuses)

	// Aplicar migrations
	fmt.Println("\n   Applying migrations...")
	if err := migrator.Up(ctx); err != nil {
		log.Fatalf("Failed to apply migrations: %v", err)
	}

	// Mostrar status depois
	fmt.Println("\n   Migration Status (after):")
	statuses, err = migrator.Status(ctx)
	if err != nil {
		log.Fatalf("Failed to get status: %v", err)
	}
	printStatus(statuses)

	// Demonstração 3: Rollback
	fmt.Println("\n3. Rollback Demo")
	fmt.Println("   Reverting last migration...")

	if err := migrator.Down(ctx); err != nil {
		log.Fatalf("Failed to revert migration: %v", err)
	}

	fmt.Println("\n   Migration Status (after rollback):")
	statuses, err = migrator.Status(ctx)
	if err != nil {
		log.Fatalf("Failed to get status: %v", err)
	}
	printStatus(statuses)

	// Reaplica a migration para deixar o banco consistente
	fmt.Println("\n   Re-applying migration...")
	if err := migrator.Up(ctx); err != nil {
		log.Fatalf("Failed to re-apply migration: %v", err)
	}

	// Demonstração 4: CreateTableMigration helper
	fmt.Println("\n4. CreateTableMigration Helper")
	fmt.Println("   Creating migration from struct...")

	// Este helper facilita criar migrations a partir de structs
	migrator2 := migrate.New(db, dialect, logger, migrate.Config{
		TableName: "schema_migrations_2",
	})

	type Category struct {
		core.Model
		Name        string                `db:"name"`
		Description core.Optional[string] `db:"description"`
	}

	categoryMigration := migrate.CreateTableMigration(4, "create_categories_table", Category{})
	migrator2.Register(categoryMigration)

	if err := migrator2.Up(ctx); err != nil {
		log.Fatalf("Failed to create categories table: %v", err)
	}

	fmt.Println("   ✅ Categories table created from struct!")

	fmt.Println("\n=== Example completed successfully! ===")
	fmt.Println("\nKey Takeaways:")
	fmt.Println("1. AutoMigrate - Quick for development")
	fmt.Println("2. Manual Migrations - Full control for production")
	fmt.Println("3. Version control - Track and rollback migrations")
	fmt.Println("4. CreateTableMigration - Generate migrations from structs")
}

func printStatus(statuses []migrate.MigrationStatus) {
	if len(statuses) == 0 {
		fmt.Println("   No migrations found.")
		return
	}

	for _, status := range statuses {
		symbol := "[ ]"
		if status.Applied {
			symbol = "[✓]"
		}

		fmt.Printf("   %s %d: %s\n", symbol, status.Version, status.Name)
	}
}
