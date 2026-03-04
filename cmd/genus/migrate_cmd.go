package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/go-genus/genus/core"
	"github.com/go-genus/genus/dialects"
	"github.com/go-genus/genus/migrate"
)

func runMigrate() error {
	if len(os.Args) < 3 {
		printMigrateUsage()
		return nil
	}

	subcommand := os.Args[2]

	switch subcommand {
	case "up":
		return runMigrateUp()
	case "down":
		return runMigrateDown()
	case "status":
		return runMigrateStatus()
	case "create":
		return runMigrateCreate()
	case "help", "--help", "-h":
		printMigrateUsage()
		return nil
	default:
		return fmt.Errorf("unknown migrate subcommand: %s", subcommand)
	}
}

func runMigrateUp() error {
	ctx := context.Background()

	// Conectar ao banco de dados
	db, dialect, err := connectDB()
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Criar migrator
	logger := core.NewDefaultLogger(true)
	migrator := migrate.New(db, dialect, logger, migrate.Config{})

	// Carregar migrations do diretório
	migrations, err := loadMigrations()
	if err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	migrator.RegisterMultiple(migrations)

	// Executar migrations
	fmt.Println("Running migrations...")
	if err := migrator.Up(ctx); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	fmt.Println("\n✅ All migrations applied successfully!")
	return nil
}

func runMigrateDown() error {
	ctx := context.Background()

	// Conectar ao banco de dados
	db, dialect, err := connectDB()
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Criar migrator
	logger := core.NewDefaultLogger(true)
	migrator := migrate.New(db, dialect, logger, migrate.Config{})

	// Carregar migrations do diretório
	migrations, err := loadMigrations()
	if err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	migrator.RegisterMultiple(migrations)

	// Reverter última migration
	fmt.Println("Reverting last migration...")
	if err := migrator.Down(ctx); err != nil {
		return fmt.Errorf("failed to revert migration: %w", err)
	}

	fmt.Println("\n✅ Migration reverted successfully!")
	return nil
}

func runMigrateStatus() error {
	ctx := context.Background()

	// Conectar ao banco de dados
	db, dialect, err := connectDB()
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Criar migrator
	logger := core.NewDefaultLogger(false)
	migrator := migrate.New(db, dialect, logger, migrate.Config{})

	// Carregar migrations do diretório
	migrations, err := loadMigrations()
	if err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	migrator.RegisterMultiple(migrations)

	// Obter status
	statuses, err := migrator.Status(ctx)
	if err != nil {
		return fmt.Errorf("failed to get migration status: %w", err)
	}

	// Imprimir status
	fmt.Println("\nMigration Status:")
	fmt.Println("================")

	if len(statuses) == 0 {
		fmt.Println("No migrations found.")
		return nil
	}

	for _, status := range statuses {
		symbol := "[ ]"
		if status.Applied {
			symbol = "[✓]"
		}

		fmt.Printf("%s %d: %s\n", symbol, status.Version, status.Name)
	}

	return nil
}

func runMigrateCreate() error {
	args := os.Args[3:]

	if len(args) == 0 {
		return fmt.Errorf("migration name required")
	}

	name := strings.Join(args, "_")
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "_")

	// Criar diretório migrations se não existir
	migrationsDir := "./migrations"
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		return fmt.Errorf("failed to create migrations directory: %w", err)
	}

	// Gerar timestamp
	timestamp := time.Now().Unix()

	// Nome do arquivo
	filename := fmt.Sprintf("%d_%s.go", timestamp, name)
	filepath := filepath.Join(migrationsDir, filename)

	// Template da migration
	tmpl := template.Must(template.New("migration").Parse(migrationTemplate))

	data := struct {
		Version int64
		Name    string
	}{
		Version: timestamp,
		Name:    name,
	}

	// Criar arquivo
	file, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("failed to create migration file: %w", err)
	}
	defer file.Close()

	// Escrever template
	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to write migration file: %w", err)
	}

	fmt.Printf("✅ Created migration: %s\n", filepath)
	fmt.Println("\nNext steps:")
	fmt.Println("1. Edit the migration file and implement Up() and Down()")
	fmt.Println("2. Run 'genus migrate up' to apply the migration")

	return nil
}

// connectDB conecta ao banco de dados usando variáveis de ambiente.
// O driver e dialeto são detectados automaticamente baseado no DSN.
func connectDB() (*sql.DB, core.Dialect, error) {
	// Obter DSN de variável de ambiente
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/genus_dev?sslmode=disable"
		fmt.Printf("DATABASE_URL not set, using default: %s\n", dsn)
	}

	// Detectar driver e dialeto automaticamente do DSN
	driver := dialects.DetectDriverFromDSN(dsn)
	dialect := dialects.DetectDialectFromDSN(dsn)

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, nil, err
	}

	return db, dialect, nil
}

// loadMigrations carrega migrations do diretório ./migrations.
// Nota: Em produção, isso seria mais sofisticado, provavelmente usando
// um arquivo de registro ou importando migrations como pacotes.
func loadMigrations() ([]migrate.Migration, error) {
	// Por enquanto, retorna lista vazia
	// O usuário precisa registrar migrations manualmente
	// ou usar AutoMigrate para desenvolvimento
	fmt.Println("\n⚠️  Note: Migration files must be registered in your code.")
	fmt.Println("   Use migrate.Register() or migrate.AutoMigrate() for development.")
	fmt.Println()

	return []migrate.Migration{}, nil
}

const migrationTemplate = `package migrations

import (
	"context"
	"database/sql"

	"github.com/go-genus/genus/core"
)

// Migration{{.Version}} - {{.Name}}
var Migration{{.Version}} = migrate.Migration{
	Version: {{.Version}},
	Name:    "{{.Name}}",
	Up: func(ctx context.Context, db *sql.DB, dialect core.Dialect) error {
		query := ` + "`" + `
			-- Write your migration SQL here
			-- Example:
			-- CREATE TABLE users (
			--     id SERIAL PRIMARY KEY,
			--     name VARCHAR(255) NOT NULL,
			--     email VARCHAR(255) UNIQUE NOT NULL
			-- );
		` + "`" + `

		_, err := db.ExecContext(ctx, query)
		return err
	},
	Down: func(ctx context.Context, db *sql.DB, dialect core.Dialect) error {
		query := ` + "`" + `
			-- Write your rollback SQL here
			-- Example:
			-- DROP TABLE IF EXISTS users;
		` + "`" + `

		_, err := db.ExecContext(ctx, query)
		return err
	},
}
`

func printMigrateUsage() {
	fmt.Println(`Manage database migrations

Usage:
  genus migrate <subcommand> [arguments]

Subcommands:
  up        Apply all pending migrations
  down      Revert the last applied migration
  status    Show migration status
  create    Create a new migration file
  help      Show this help message

Environment Variables:
  DATABASE_URL    Database connection string (default: postgres://postgres:postgres@localhost:5432/genus_dev?sslmode=disable)

Examples:
  genus migrate up                           # Apply all pending migrations
  genus migrate down                         # Revert last migration
  genus migrate status                       # Show migration status
  genus migrate create add_users_table       # Create new migration
  genus migrate create "add email to users"  # Create migration with spaces in name

Migration Files:
  Migrations are created in ./migrations/ directory
  Each migration has a timestamp-based version number
  Implement Up() and Down() functions for each migration`)
}
