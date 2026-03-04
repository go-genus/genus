package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/go-genus/genus/core"
)

// Migration representa uma migration com funções Up e Down.
type Migration struct {
	// Version é o número da versão da migration (ex: 1, 2, 3, etc.)
	Version int64
	// Name é o nome descritivo da migration (ex: "create_users_table")
	Name string
	// Up é executado quando aplicando a migration
	Up func(context.Context, *sql.DB, core.Dialect) error
	// Down é executado quando revertendo a migration
	Down func(context.Context, *sql.DB, core.Dialect) error
}

// Migrator gerencia migrations de banco de dados.
type Migrator struct {
	db      *sql.DB
	dialect core.Dialect
	logger  core.Logger
	// migrations é a lista de todas as migrations registradas
	migrations []Migration
	// tableName é o nome da tabela de controle de migrations
	tableName string
}

// Config contém configurações para o Migrator.
type Config struct {
	// TableName é o nome da tabela de controle (default: "schema_migrations")
	TableName string
}

// New cria um novo Migrator.
func New(db *sql.DB, dialect core.Dialect, logger core.Logger, config Config) *Migrator {
	tableName := config.TableName
	if tableName == "" {
		tableName = "schema_migrations"
	}

	return &Migrator{
		db:         db,
		dialect:    dialect,
		logger:     logger,
		migrations: []Migration{},
		tableName:  tableName,
	}
}

// Register registra uma nova migration.
func (m *Migrator) Register(migration Migration) {
	m.migrations = append(m.migrations, migration)
}

// RegisterMultiple registra múltiplas migrations.
func (m *Migrator) RegisterMultiple(migrations []Migration) {
	m.migrations = append(m.migrations, migrations...)
}

// Up aplica todas as migrations pendentes.
func (m *Migrator) Up(ctx context.Context) error {
	// Criar tabela de controle se não existir
	if err := m.createMigrationsTable(ctx); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Obter migrations aplicadas
	applied, err := m.getAppliedMigrations(ctx)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Ordenar migrations por versão
	sort.Slice(m.migrations, func(i, j int) bool {
		return m.migrations[i].Version < m.migrations[j].Version
	})

	// Aplicar migrations pendentes
	for _, migration := range m.migrations {
		if applied[migration.Version] {
			continue
		}

		m.logger.LogQuery(fmt.Sprintf("Applying migration %d: %s", migration.Version, migration.Name), nil, 0)

		if err := m.runMigration(ctx, migration, true); err != nil {
			return fmt.Errorf("failed to apply migration %d (%s): %w", migration.Version, migration.Name, err)
		}

		m.logger.LogQuery(fmt.Sprintf("✅ Applied migration %d: %s", migration.Version, migration.Name), nil, 0)
	}

	return nil
}

// Down reverte a última migration aplicada.
func (m *Migrator) Down(ctx context.Context) error {
	// Obter migrations aplicadas
	applied, err := m.getAppliedMigrations(ctx)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	if len(applied) == 0 {
		m.logger.LogQuery("No migrations to revert", nil, 0)
		return nil
	}

	// Encontrar a última migration aplicada
	var lastMigration *Migration
	maxVersion := int64(0)

	for _, migration := range m.migrations {
		if applied[migration.Version] && migration.Version > maxVersion {
			maxVersion = migration.Version
			lastMigration = &migration
		}
	}

	if lastMigration == nil {
		return fmt.Errorf("no migration found to revert")
	}

	m.logger.LogQuery(fmt.Sprintf("Reverting migration %d: %s", lastMigration.Version, lastMigration.Name), nil, 0)

	if err := m.runMigration(ctx, *lastMigration, false); err != nil {
		return fmt.Errorf("failed to revert migration %d (%s): %w", lastMigration.Version, lastMigration.Name, err)
	}

	m.logger.LogQuery(fmt.Sprintf("✅ Reverted migration %d: %s", lastMigration.Version, lastMigration.Name), nil, 0)

	return nil
}

// Status mostra o status de todas as migrations.
func (m *Migrator) Status(ctx context.Context) ([]MigrationStatus, error) {
	// Criar tabela de controle se não existir
	if err := m.createMigrationsTable(ctx); err != nil {
		return nil, fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Obter migrations aplicadas
	applied, err := m.getAppliedMigrations(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Ordenar migrations por versão
	sort.Slice(m.migrations, func(i, j int) bool {
		return m.migrations[i].Version < m.migrations[j].Version
	})

	// Criar lista de status
	statuses := make([]MigrationStatus, len(m.migrations))
	for i, migration := range m.migrations {
		statuses[i] = MigrationStatus{
			Version: migration.Version,
			Name:    migration.Name,
			Applied: applied[migration.Version],
		}
	}

	return statuses, nil
}

// MigrationStatus representa o status de uma migration.
type MigrationStatus struct {
	Version int64
	Name    string
	Applied bool
}

// runMigration executa uma migration (Up ou Down).
func (m *Migrator) runMigration(ctx context.Context, migration Migration, up bool) error {
	// Iniciar transação
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Executar função Up ou Down
	if up {
		if err := migration.Up(ctx, m.db, m.dialect); err != nil {
			return err
		}
	} else {
		if migration.Down == nil {
			return fmt.Errorf("migration %d has no Down function", migration.Version)
		}
		if err := migration.Down(ctx, m.db, m.dialect); err != nil {
			return err
		}
	}

	// Atualizar tabela de controle
	if up {
		query := fmt.Sprintf("INSERT INTO %s (version, name, applied_at) VALUES (%s, %s, %s)",
			m.dialect.QuoteIdentifier(m.tableName),
			m.dialect.Placeholder(1),
			m.dialect.Placeholder(2),
			m.dialect.Placeholder(3))

		if _, err := tx.ExecContext(ctx, query, migration.Version, migration.Name, time.Now()); err != nil {
			return fmt.Errorf("failed to record migration: %w", err)
		}
	} else {
		query := fmt.Sprintf("DELETE FROM %s WHERE version = %s",
			m.dialect.QuoteIdentifier(m.tableName),
			m.dialect.Placeholder(1))

		if _, err := tx.ExecContext(ctx, query, migration.Version); err != nil {
			return fmt.Errorf("failed to remove migration record: %w", err)
		}
	}

	// Commit transação
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// createMigrationsTable cria a tabela de controle de migrations.
func (m *Migrator) createMigrationsTable(ctx context.Context) error {
	// Detectar o tipo de dialeto pelos métodos
	placeholder := m.dialect.Placeholder(1)
	quotedIdentifier := m.dialect.QuoteIdentifier("test")

	var versionType, nameType, timestampType string

	if strings.HasPrefix(placeholder, "$") {
		// PostgreSQL
		versionType = "BIGINT"
		nameType = "VARCHAR(255)"
		timestampType = "TIMESTAMP"
	} else if strings.HasPrefix(quotedIdentifier, "`") {
		// MySQL
		versionType = "BIGINT"
		nameType = "VARCHAR(255)"
		timestampType = "DATETIME"
	} else {
		// SQLite
		versionType = "INTEGER"
		nameType = "TEXT"
		timestampType = "DATETIME"
	}

	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			version %s PRIMARY KEY,
			name %s NOT NULL,
			applied_at %s NOT NULL
		)
	`, m.dialect.QuoteIdentifier(m.tableName), versionType, nameType, timestampType)

	if _, err := m.db.ExecContext(ctx, query); err != nil {
		return err
	}

	return nil
}

// getAppliedMigrations retorna um mapa de versões aplicadas.
func (m *Migrator) getAppliedMigrations(ctx context.Context) (map[int64]bool, error) {
	query := fmt.Sprintf("SELECT version FROM %s", m.dialect.QuoteIdentifier(m.tableName))

	rows, err := m.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[int64]bool)
	for rows.Next() {
		var version int64
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return applied, nil
}
