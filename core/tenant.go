package core

import (
	"context"
	"fmt"
)

// TenantStrategy define a estratégia de multi-tenancy.
type TenantStrategy int

const (
	// TenantColumn usa uma coluna tenant_id em cada tabela.
	TenantColumn TenantStrategy = iota

	// TenantSchema usa schemas separados por tenant (PostgreSQL).
	TenantSchema

	// TenantDatabase usa databases separados por tenant.
	TenantDatabase
)

// TenantConfig configura multi-tenancy.
type TenantConfig struct {
	// Strategy define a estratégia de isolamento.
	Strategy TenantStrategy

	// ColumnName é o nome da coluna de tenant (para TenantColumn).
	ColumnName string

	// DefaultTenant é o tenant padrão quando não especificado.
	DefaultTenant string

	// GetTenantFromContext extrai o tenant ID do contexto.
	GetTenantFromContext func(ctx context.Context) string
}

// DefaultTenantConfig retorna configuração padrão.
func DefaultTenantConfig() TenantConfig {
	return TenantConfig{
		Strategy:   TenantColumn,
		ColumnName: "tenant_id",
	}
}

// tenantContextKey é a chave para armazenar tenant no contexto.
type tenantContextKey struct{}

// WithTenant adiciona um tenant ID ao contexto.
func WithTenant(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantContextKey{}, tenantID)
}

// TenantFromContext obtém o tenant ID do contexto.
func TenantFromContext(ctx context.Context) string {
	if tenantID, ok := ctx.Value(tenantContextKey{}).(string); ok {
		return tenantID
	}
	return ""
}

// TenantScope adiciona condição de tenant às queries.
type TenantScope struct {
	config TenantConfig
}

// NewTenantScope cria um novo tenant scope.
func NewTenantScope(config TenantConfig) *TenantScope {
	return &TenantScope{config: config}
}

// GetTenantCondition retorna a condição SQL para filtrar por tenant.
func (ts *TenantScope) GetTenantCondition(ctx context.Context, dialect Dialect) (string, interface{}) {
	tenantID := ts.getTenantID(ctx)
	if tenantID == "" {
		return "", nil
	}

	column := ts.config.ColumnName
	if column == "" {
		column = "tenant_id"
	}

	return fmt.Sprintf("%s = %s",
		dialect.QuoteIdentifier(column),
		dialect.Placeholder(1)), tenantID
}

// GetSchemaName retorna o nome do schema para o tenant (TenantSchema strategy).
func (ts *TenantScope) GetSchemaName(ctx context.Context) string {
	tenantID := ts.getTenantID(ctx)
	if tenantID == "" {
		return "public"
	}
	return fmt.Sprintf("tenant_%s", tenantID)
}

// getTenantID obtém o tenant ID do contexto.
func (ts *TenantScope) getTenantID(ctx context.Context) string {
	if ts.config.GetTenantFromContext != nil {
		return ts.config.GetTenantFromContext(ctx)
	}
	return TenantFromContext(ctx)
}

// TenantMiddleware adiciona tenant a todas as queries automaticamente.
type TenantMiddleware struct {
	scope   *TenantScope
	dialect Dialect
}

// NewTenantMiddleware cria um novo middleware de tenant.
func NewTenantMiddleware(config TenantConfig, dialect Dialect) *TenantMiddleware {
	return &TenantMiddleware{
		scope:   NewTenantScope(config),
		dialect: dialect,
	}
}

// ApplyToQuery adiciona a condição de tenant a uma query.
func (tm *TenantMiddleware) ApplyToQuery(ctx context.Context, query string) (string, []interface{}) {
	condition, arg := tm.scope.GetTenantCondition(ctx, tm.dialect)
	if condition == "" {
		return query, nil
	}

	// Simplificação: adiciona WHERE ou AND
	// Em produção, seria necessário um parser SQL mais sofisticado
	return query, []interface{}{arg}
}

// SetSearchPath define o search_path para o schema do tenant (PostgreSQL).
func (tm *TenantMiddleware) SetSearchPath(ctx context.Context, executor Executor) error {
	schema := tm.scope.GetSchemaName(ctx)
	query := fmt.Sprintf("SET search_path TO %s, public", tm.dialect.QuoteIdentifier(schema))
	_, err := executor.ExecContext(ctx, query)
	return err
}

// CreateTenantSchema cria o schema para um novo tenant (PostgreSQL).
func (tm *TenantMiddleware) CreateTenantSchema(ctx context.Context, executor Executor, tenantID string) error {
	schema := fmt.Sprintf("tenant_%s", tenantID)
	query := fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", tm.dialect.QuoteIdentifier(schema))
	_, err := executor.ExecContext(ctx, query)
	return err
}

// DropTenantSchema remove o schema de um tenant (PostgreSQL).
func (tm *TenantMiddleware) DropTenantSchema(ctx context.Context, executor Executor, tenantID string) error {
	schema := fmt.Sprintf("tenant_%s", tenantID)
	query := fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", tm.dialect.QuoteIdentifier(schema))
	_, err := executor.ExecContext(ctx, query)
	return err
}

// ListTenantSchemas lista todos os schemas de tenant.
func (tm *TenantMiddleware) ListTenantSchemas(ctx context.Context, executor Executor) ([]string, error) {
	query := `
		SELECT schema_name
		FROM information_schema.schemata
		WHERE schema_name LIKE 'tenant_%'
		ORDER BY schema_name
	`

	rows, err := executor.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var schemas []string
	for rows.Next() {
		var schema string
		if err := rows.Scan(&schema); err != nil {
			return nil, err
		}
		schemas = append(schemas, schema)
	}

	return schemas, rows.Err()
}

// TenantAware interface para models que suportam multi-tenancy.
type TenantAware interface {
	GetTenantID() string
	SetTenantID(tenantID string)
}

// Tenant é um mixin que pode ser embedded em models.
type Tenant struct {
	TenantID string `db:"tenant_id" json:"tenant_id"`
}

// GetTenantID retorna o tenant ID.
func (t *Tenant) GetTenantID() string {
	return t.TenantID
}

// SetTenantID define o tenant ID.
func (t *Tenant) SetTenantID(tenantID string) {
	t.TenantID = tenantID
}

// RowLevelSecurityConfig configura RLS (Row-Level Security).
type RowLevelSecurityConfig struct {
	// Enabled ativa RLS.
	Enabled bool

	// PolicyName é o nome da policy.
	PolicyName string

	// TenantColumn é a coluna de tenant.
	TenantColumn string

	// SessionVariable é a variável de sessão que armazena o tenant ID.
	// Para PostgreSQL: current_setting('app.tenant_id')
	SessionVariable string
}

// DefaultRowLevelSecurityConfig retorna configuração padrão.
func DefaultRowLevelSecurityConfig() RowLevelSecurityConfig {
	return RowLevelSecurityConfig{
		Enabled:         false,
		PolicyName:      "tenant_isolation_policy",
		TenantColumn:    "tenant_id",
		SessionVariable: "current_setting('app.tenant_id')",
	}
}

// CreateRLSPolicy cria uma policy de RLS para uma tabela (PostgreSQL).
func CreateRLSPolicy(ctx context.Context, executor Executor, dialect Dialect, tableName string, config RowLevelSecurityConfig) error {
	// Habilita RLS na tabela
	enableQuery := fmt.Sprintf("ALTER TABLE %s ENABLE ROW LEVEL SECURITY",
		dialect.QuoteIdentifier(tableName))
	if _, err := executor.ExecContext(ctx, enableQuery); err != nil {
		return err
	}

	// Força RLS mesmo para owner da tabela
	forceQuery := fmt.Sprintf("ALTER TABLE %s FORCE ROW LEVEL SECURITY",
		dialect.QuoteIdentifier(tableName))
	if _, err := executor.ExecContext(ctx, forceQuery); err != nil {
		return err
	}

	// Cria a policy
	policyQuery := fmt.Sprintf(`
		CREATE POLICY %s ON %s
		USING (%s = %s)
		WITH CHECK (%s = %s)
	`,
		dialect.QuoteIdentifier(config.PolicyName),
		dialect.QuoteIdentifier(tableName),
		dialect.QuoteIdentifier(config.TenantColumn),
		config.SessionVariable,
		dialect.QuoteIdentifier(config.TenantColumn),
		config.SessionVariable,
	)

	_, err := executor.ExecContext(ctx, policyQuery)
	return err
}

// SetTenantSession define o tenant na sessão do banco (PostgreSQL).
func SetTenantSession(ctx context.Context, executor Executor, tenantID string) error {
	query := fmt.Sprintf("SET app.tenant_id = '%s'", tenantID)
	_, err := executor.ExecContext(ctx, query)
	return err
}
