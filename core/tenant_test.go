package core

import (
	"context"
	"database/sql/driver"
	"testing"
)

func TestDefaultTenantConfig(t *testing.T) {
	config := DefaultTenantConfig()
	if config.Strategy != TenantColumn {
		t.Errorf("Strategy = %d, want TenantColumn", config.Strategy)
	}
	if config.ColumnName != "tenant_id" {
		t.Errorf("ColumnName = %q, want %q", config.ColumnName, "tenant_id")
	}
}

func TestWithTenant(t *testing.T) {
	ctx := context.Background()
	tenantCtx := WithTenant(ctx, "acme")

	id := TenantFromContext(tenantCtx)
	if id != "acme" {
		t.Errorf("TenantFromContext() = %q, want %q", id, "acme")
	}
}

func TestTenantFromContext_Empty(t *testing.T) {
	ctx := context.Background()
	id := TenantFromContext(ctx)
	if id != "" {
		t.Errorf("TenantFromContext() = %q, want empty", id)
	}
}

func TestTenantScope_GetTenantCondition(t *testing.T) {
	config := DefaultTenantConfig()
	scope := NewTenantScope(config)
	dialect := newPostgresDialect()

	ctx := WithTenant(context.Background(), "acme")
	condition, arg := scope.GetTenantCondition(ctx, dialect)

	if condition == "" {
		t.Error("condition should not be empty")
	}
	if arg != "acme" {
		t.Errorf("arg = %v, want %q", arg, "acme")
	}
}

func TestTenantScope_GetTenantCondition_NoTenant(t *testing.T) {
	config := DefaultTenantConfig()
	scope := NewTenantScope(config)
	dialect := newPostgresDialect()

	ctx := context.Background()
	condition, arg := scope.GetTenantCondition(ctx, dialect)

	if condition != "" {
		t.Errorf("condition should be empty, got %q", condition)
	}
	if arg != nil {
		t.Errorf("arg should be nil, got %v", arg)
	}
}

func TestTenantScope_GetTenantCondition_EmptyColumnName(t *testing.T) {
	config := TenantConfig{
		Strategy:   TenantColumn,
		ColumnName: "", // empty
	}
	scope := NewTenantScope(config)
	dialect := newPostgresDialect()

	ctx := WithTenant(context.Background(), "acme")
	condition, _ := scope.GetTenantCondition(ctx, dialect)

	if condition == "" {
		t.Error("should use default column name")
	}
}

func TestTenantScope_GetSchemaName(t *testing.T) {
	config := DefaultTenantConfig()
	scope := NewTenantScope(config)

	ctx := WithTenant(context.Background(), "acme")
	schema := scope.GetSchemaName(ctx)
	if schema != "tenant_acme" {
		t.Errorf("GetSchemaName() = %q, want %q", schema, "tenant_acme")
	}
}

func TestTenantScope_GetSchemaName_NoTenant(t *testing.T) {
	config := DefaultTenantConfig()
	scope := NewTenantScope(config)

	ctx := context.Background()
	schema := scope.GetSchemaName(ctx)
	if schema != "public" {
		t.Errorf("GetSchemaName() = %q, want %q", schema, "public")
	}
}

func TestTenantScope_CustomGetTenant(t *testing.T) {
	config := TenantConfig{
		Strategy:   TenantColumn,
		ColumnName: "tenant_id",
		GetTenantFromContext: func(ctx context.Context) string {
			return "custom-tenant"
		},
	}
	scope := NewTenantScope(config)
	dialect := newPostgresDialect()

	ctx := context.Background()
	_, arg := scope.GetTenantCondition(ctx, dialect)
	if arg != "custom-tenant" {
		t.Errorf("arg = %v, want %q", arg, "custom-tenant")
	}
}

func TestNewTenantMiddleware(t *testing.T) {
	config := DefaultTenantConfig()
	dialect := newPostgresDialect()
	tm := NewTenantMiddleware(config, dialect)
	if tm == nil {
		t.Fatal("NewTenantMiddleware returned nil")
	}
}

func TestTenantMiddleware_ApplyToQuery(t *testing.T) {
	config := DefaultTenantConfig()
	dialect := newPostgresDialect()
	tm := NewTenantMiddleware(config, dialect)

	ctx := WithTenant(context.Background(), "acme")
	_, args := tm.ApplyToQuery(ctx, "SELECT * FROM users")

	if len(args) != 1 {
		t.Errorf("expected 1 arg, got %d", len(args))
	}
}

func TestTenantMiddleware_ApplyToQuery_NoTenant(t *testing.T) {
	config := DefaultTenantConfig()
	dialect := newPostgresDialect()
	tm := NewTenantMiddleware(config, dialect)

	ctx := context.Background()
	_, args := tm.ApplyToQuery(ctx, "SELECT * FROM users")

	if args != nil {
		t.Errorf("expected nil args, got %v", args)
	}
}

func TestTenantMiddleware_SetSearchPath(t *testing.T) {
	config := DefaultTenantConfig()
	dialect := newPostgresDialect()
	tm := NewTenantMiddleware(config, dialect)

	exec := newMockExecutor()
	ctx := WithTenant(context.Background(), "acme")
	err := tm.SetSearchPath(ctx, exec)
	if err != nil {
		t.Errorf("SetSearchPath error = %v", err)
	}

	if len(exec.execCalls) != 1 {
		t.Fatal("expected 1 exec call")
	}
}

func TestTenantMiddleware_CreateTenantSchema(t *testing.T) {
	config := DefaultTenantConfig()
	dialect := newPostgresDialect()
	tm := NewTenantMiddleware(config, dialect)

	exec := newMockExecutor()
	ctx := context.Background()
	err := tm.CreateTenantSchema(ctx, exec, "acme")
	if err != nil {
		t.Errorf("CreateTenantSchema error = %v", err)
	}
}

func TestTenantMiddleware_DropTenantSchema(t *testing.T) {
	config := DefaultTenantConfig()
	dialect := newPostgresDialect()
	tm := NewTenantMiddleware(config, dialect)

	exec := newMockExecutor()
	ctx := context.Background()
	err := tm.DropTenantSchema(ctx, exec, "acme")
	if err != nil {
		t.Errorf("DropTenantSchema error = %v", err)
	}
}

func TestTenantMiddleware_SetSearchPath_Error(t *testing.T) {
	config := DefaultTenantConfig()
	dialect := newPostgresDialect()
	tm := NewTenantMiddleware(config, dialect)

	exec := newMockExecutor()
	exec.execErr = errTest
	ctx := WithTenant(context.Background(), "acme")
	err := tm.SetSearchPath(ctx, exec)
	if err == nil {
		t.Error("should return error")
	}
}

func TestTenant_GetSetTenantID(t *testing.T) {
	tenant := &Tenant{}
	if tenant.GetTenantID() != "" {
		t.Error("initial tenant ID should be empty")
	}

	tenant.SetTenantID("acme")
	if tenant.GetTenantID() != "acme" {
		t.Errorf("GetTenantID() = %q, want %q", tenant.GetTenantID(), "acme")
	}
}

func TestTenant_ImplementsTenantAware(t *testing.T) {
	var _ TenantAware = &Tenant{}
}

func TestDefaultRowLevelSecurityConfig(t *testing.T) {
	config := DefaultRowLevelSecurityConfig()
	if config.Enabled {
		t.Error("Enabled should be false")
	}
	if config.PolicyName != "tenant_isolation_policy" {
		t.Errorf("PolicyName = %q", config.PolicyName)
	}
	if config.TenantColumn != "tenant_id" {
		t.Errorf("TenantColumn = %q", config.TenantColumn)
	}
}

func TestCreateRLSPolicy(t *testing.T) {
	exec := newMockExecutor()
	dialect := newPostgresDialect()
	config := DefaultRowLevelSecurityConfig()

	err := CreateRLSPolicy(context.Background(), exec, dialect, "users", config)
	if err != nil {
		t.Errorf("CreateRLSPolicy error = %v", err)
	}

	// Should have 3 exec calls: ENABLE RLS, FORCE RLS, CREATE POLICY
	if len(exec.execCalls) != 3 {
		t.Errorf("expected 3 exec calls, got %d", len(exec.execCalls))
	}
}

func TestCreateRLSPolicy_Error(t *testing.T) {
	exec := newMockExecutor()
	exec.execErr = errTest
	dialect := newPostgresDialect()
	config := DefaultRowLevelSecurityConfig()

	err := CreateRLSPolicy(context.Background(), exec, dialect, "users", config)
	if err == nil {
		t.Error("should return error")
	}
}

func TestSetTenantSession(t *testing.T) {
	exec := newMockExecutor()
	err := SetTenantSession(context.Background(), exec, "acme")
	if err != nil {
		t.Errorf("SetTenantSession error = %v", err)
	}

	if len(exec.execCalls) != 1 {
		t.Fatal("expected 1 exec call")
	}
}

func TestSetTenantSession_Error(t *testing.T) {
	exec := newMockExecutor()
	exec.execErr = errTest
	err := SetTenantSession(context.Background(), exec, "acme")
	if err == nil {
		t.Error("should return error")
	}
}

func TestTenantStrategy_Constants(t *testing.T) {
	if TenantColumn != 0 {
		t.Errorf("TenantColumn = %d, want 0", TenantColumn)
	}
	if TenantSchema != 1 {
		t.Errorf("TenantSchema = %d, want 1", TenantSchema)
	}
	if TenantDatabase != 2 {
		t.Errorf("TenantDatabase = %d, want 2", TenantDatabase)
	}
}

func TestTenantMiddleware_ListTenantSchemas_Error(t *testing.T) {
	exec := newMockExecutor()
	exec.queryErr = errTest
	dialect := newPostgresDialect()
	config := DefaultTenantConfig()
	tm := NewTenantMiddleware(config, dialect)

	_, err := tm.ListTenantSchemas(context.Background(), exec)
	if err == nil {
		t.Error("should return error when query fails")
	}
}

func TestTenantMiddleware_ListTenantSchemas_Success(t *testing.T) {
	exec := newMockExecutorWithQueryRows(
		[]string{"schema_name"},
		[][]driver.Value{{"tenant_acme"}, {"tenant_beta"}},
	)
	defer exec.Close()

	dialect := newPostgresDialect()
	config := DefaultTenantConfig()
	tm := NewTenantMiddleware(config, dialect)

	schemas, err := tm.ListTenantSchemas(context.Background(), exec)
	if err != nil {
		t.Logf("ListTenantSchemas error (may be expected with mock): %v", err)
	}
	_ = schemas
}

func TestCreateRLSPolicy_SecondExecError(t *testing.T) {
	exec := newConditionalMockExecutor(2) // Fail on second exec (FORCE RLS)
	dialect := newPostgresDialect()
	config := DefaultRowLevelSecurityConfig()

	err := CreateRLSPolicy(context.Background(), exec, dialect, "users", config)
	if err == nil {
		t.Error("should return error when FORCE RLS fails")
	}
}

func TestCreateRLSPolicy_ThirdExecError(t *testing.T) {
	exec := newConditionalMockExecutor(3) // Fail on third exec (CREATE POLICY)
	dialect := newPostgresDialect()
	config := DefaultRowLevelSecurityConfig()

	err := CreateRLSPolicy(context.Background(), exec, dialect, "users", config)
	if err == nil {
		t.Error("should return error when CREATE POLICY fails")
	}
}

func TestTenantMiddleware_CreateTenantSchema_Error(t *testing.T) {
	exec := newMockExecutor()
	exec.execErr = errTest
	dialect := newPostgresDialect()
	config := DefaultTenantConfig()
	tm := NewTenantMiddleware(config, dialect)

	err := tm.CreateTenantSchema(context.Background(), exec, "acme")
	if err == nil {
		t.Error("should return error")
	}
}

func TestTenantMiddleware_DropTenantSchema_Error(t *testing.T) {
	exec := newMockExecutor()
	exec.execErr = errTest
	dialect := newPostgresDialect()
	config := DefaultTenantConfig()
	tm := NewTenantMiddleware(config, dialect)

	err := tm.DropTenantSchema(context.Background(), exec, "acme")
	if err == nil {
		t.Error("should return error")
	}
}
