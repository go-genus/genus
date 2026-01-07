package genus

import (
	"database/sql"
	"reflect"
	"strings"

	"github.com/GabrielOnRails/genus/core"
	"github.com/GabrielOnRails/genus/dialects/postgres"
	"github.com/GabrielOnRails/genus/query"
)

// Genus é a interface pública principal do ORM.
type Genus struct {
	db *core.DB
}

// Open cria uma nova conexão com o banco de dados.
func Open(driver, dsn string) (*Genus, error) {
	sqlDB, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}

	// Por padrão, usa PostgreSQL
	// TODO: detectar driver automaticamente
	dialect := postgres.New()

	return &Genus{
		db: core.New(sqlDB, dialect),
	}, nil
}

// New cria uma nova instância do Genus com uma conexão existente.
func New(sqlDB *sql.DB, dialect core.Dialect) *Genus {
	return &Genus{
		db: core.New(sqlDB, dialect),
	}
}

// NewWithLogger cria uma nova instância do Genus com um logger customizado.
func NewWithLogger(sqlDB *sql.DB, dialect core.Dialect, logger core.Logger) *Genus {
	return &Genus{
		db: core.NewWithLogger(sqlDB, dialect, logger),
	}
}

// DB retorna o core.DB subjacente para operações avançadas.
func (g *Genus) DB() *core.DB {
	return g.db
}

// Table cria um query builder type-safe para o tipo T.
// Esta é a função mágica que permite: genus.Table[User]().Where(...)
func Table[T any](g *Genus) *query.Builder[T] {
	var model T
	tableName := getTableName(model)
	return query.NewBuilder[T](g.db.Executor(), g.db.Dialect(), g.db.Logger(), tableName)
}

// getTableName obtém o nome da tabela para um modelo.
func getTableName(model interface{}) string {
	if tn, ok := model.(core.TableNamer); ok {
		return tn.TableName()
	}

	t := reflect.TypeOf(model)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return toSnakeCase(t.Name())
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

// RegisterModels registra múltiplos models com seus relacionamentos.
// Deve ser chamado na inicialização da aplicação, antes de usar Preload.
// Exemplo: genus.RegisterModels(&User{}, &Post{}, &Tag{})
func RegisterModels(models ...interface{}) error {
	for _, model := range models {
		if err := core.RegisterModel(model); err != nil {
			return err
		}
	}
	return nil
}
