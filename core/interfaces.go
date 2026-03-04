package core

import (
	"context"
	"database/sql"
	"errors"
)

// ErrValidation é retornado quando a validação de um modelo falha.
var ErrValidation = errors.New("validation failed")

// ErrNotFound é retornado quando um registro não é encontrado.
var ErrNotFound = errors.New("record not found")

// Dialect define a interface para diferentes dialetos de banco de dados.
// Cada dialeto (PostgreSQL, MySQL, SQLite) implementa esta interface.
type Dialect interface {
	// Placeholder retorna o placeholder para a posição n (ex: $1 para PostgreSQL, ? para MySQL)
	Placeholder(n int) string

	// QuoteIdentifier adiciona aspas ao identificador (ex: "users" para PostgreSQL)
	QuoteIdentifier(name string) string

	// GetType retorna o tipo SQL para um tipo Go
	GetType(goType string) string
}

// Executor é a interface que pode executar queries.
// Implementada por *sql.DB e *sql.Tx.
type Executor interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

// Scanner é a interface para tipos que podem escanear valores do banco.
type Scanner interface {
	Scan(dest ...interface{}) error
}

// Logger é a interface para logging de queries SQL.
// Implementações podem enviar logs para stdout, arquivo, ou serviços externos.
type Logger interface {
	LogQuery(query string, args []interface{}, duration int64)
	LogError(query string, args []interface{}, err error)
}
