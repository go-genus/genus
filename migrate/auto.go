package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/go-genus/genus/core"
)

// AutoMigrate cria automaticamente tabelas a partir de structs.
// Útil para desenvolvimento rápido, mas não recomendado para produção.
func AutoMigrate(ctx context.Context, db *sql.DB, dialect core.Dialect, models ...interface{}) error {
	for _, model := range models {
		if err := createTableFromStruct(ctx, db, dialect, model); err != nil {
			return fmt.Errorf("failed to auto-migrate %T: %w", model, err)
		}
	}
	return nil
}

// CreateTableMigration cria uma migration para criar uma tabela a partir de uma struct.
func CreateTableMigration(version int64, name string, model interface{}) Migration {
	return Migration{
		Version: version,
		Name:    name,
		Up: func(ctx context.Context, db *sql.DB, dialect core.Dialect) error {
			return createTableFromStruct(ctx, db, dialect, model)
		},
		Down: func(ctx context.Context, db *sql.DB, dialect core.Dialect) error {
			return dropTable(ctx, db, dialect, model)
		},
	}
}

// createTableFromStruct cria uma tabela a partir de uma struct.
func createTableFromStruct(ctx context.Context, db *sql.DB, dialect core.Dialect, model interface{}) error {
	// Obter informações da struct
	t := reflect.TypeOf(model)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return fmt.Errorf("model must be a struct, got %v", t.Kind())
	}

	// Obter nome da tabela
	tableName := getTableName(model)

	// Construir colunas
	var columns []string

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Ignorar campos embedded (como core.Model)
		if field.Anonymous {
			// Processar campos do embedded struct
			embeddedType := field.Type
			if embeddedType.Kind() == reflect.Ptr {
				embeddedType = embeddedType.Elem()
			}

			for j := 0; j < embeddedType.NumField(); j++ {
				embeddedField := embeddedType.Field(j)
				if col := buildColumnDefinition(embeddedField, dialect); col != "" {
					columns = append(columns, col)
				}
			}
			continue
		}

		// Processar campo normal
		if col := buildColumnDefinition(field, dialect); col != "" {
			columns = append(columns, col)
		}
	}

	if len(columns) == 0 {
		return fmt.Errorf("no columns found in struct %v", t.Name())
	}

	// Construir query CREATE TABLE
	query := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n  %s\n)",
		dialect.QuoteIdentifier(tableName),
		strings.Join(columns, ",\n  "))

	// Executar query
	if _, err := db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	return nil
}

// dropTable remove uma tabela.
func dropTable(ctx context.Context, db *sql.DB, dialect core.Dialect, model interface{}) error {
	tableName := getTableName(model)

	query := fmt.Sprintf("DROP TABLE IF EXISTS %s", dialect.QuoteIdentifier(tableName))

	if _, err := db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("failed to drop table: %w", err)
	}

	return nil
}

// buildColumnDefinition constrói a definição de uma coluna a partir de um campo.
func buildColumnDefinition(field reflect.StructField, dialect core.Dialect) string {
	// Obter tag db
	dbTag := field.Tag.Get("db")
	if dbTag == "" || dbTag == "-" {
		return ""
	}

	columnName := dbTag

	// Obter tipo SQL
	sqlType := getSQLType(field.Type, dialect)

	// Verificar constraints
	var constraints []string

	// PRIMARY KEY
	if field.Name == "ID" {
		constraints = append(constraints, "PRIMARY KEY")

		// Auto-increment dependendo do dialect
		placeholder := dialect.Placeholder(1)
		quotedIdentifier := dialect.QuoteIdentifier("test")

		if strings.HasPrefix(placeholder, "$") {
			// PostgreSQL usa $1, $2, etc
			sqlType = "SERIAL"
		} else if strings.HasPrefix(quotedIdentifier, "`") {
			// MySQL usa backticks `
			sqlType = "INTEGER"
			constraints = append(constraints, "AUTO_INCREMENT")
		} else {
			// SQLite usa aspas duplas "
			// SQLite: INTEGER PRIMARY KEY é automaticamente AUTOINCREMENT
			sqlType = "INTEGER"
			// Não adiciona AUTO_INCREMENT pois SQLite não suporta
		}
	}

	// NOT NULL
	if !isOptional(field.Type) && field.Name != "ID" {
		constraints = append(constraints, "NOT NULL")
	}

	// DEFAULT para timestamps
	if field.Type == reflect.TypeOf(time.Time{}) {
		if field.Name == "CreatedAt" || field.Name == "UpdatedAt" {
			constraints = append(constraints, "DEFAULT CURRENT_TIMESTAMP")
		}
	}

	// Construir definição
	parts := []string{
		dialect.QuoteIdentifier(columnName),
		sqlType,
	}
	parts = append(parts, constraints...)

	return strings.Join(parts, " ")
}

// getSQLType retorna o tipo SQL para um tipo Go.
func getSQLType(t reflect.Type, dialect core.Dialect) string {
	// Verificar se é Optional[T]
	if t.Kind() == reflect.Struct && t.Name() == "Optional" {
		// Obter tipo interno
		if t.NumField() > 0 {
			innerType := t.Field(0).Type
			return dialect.GetType(innerType.String())
		}
	}

	// Mapear tipos básicos
	switch t.Kind() {
	case reflect.String:
		return dialect.GetType("string")
	case reflect.Int:
		return dialect.GetType("int")
	case reflect.Int64:
		return dialect.GetType("int64")
	case reflect.Bool:
		return dialect.GetType("bool")
	case reflect.Float64:
		return dialect.GetType("float64")
	case reflect.Float32:
		return dialect.GetType("float32")
	case reflect.Struct:
		if t == reflect.TypeOf(time.Time{}) {
			return dialect.GetType("time.Time")
		}
	}

	// Fallback
	return "TEXT"
}

// isOptional verifica se um tipo é Optional[T].
func isOptional(t reflect.Type) bool {
	return t.Kind() == reflect.Struct && t.Name() == "Optional"
}

// getTableName obtém o nome da tabela a partir do modelo.
func getTableName(model interface{}) string {
	// Verificar se implementa TableNamer
	if namer, ok := model.(core.TableNamer); ok {
		return namer.TableName()
	}

	// Usar nome do tipo em snake_case
	t := reflect.TypeOf(model)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	name := t.Name()
	return toSnakeCase(name)
}

// toSnakeCase converte CamelCase para snake_case.
func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}
