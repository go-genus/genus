package genus

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-genus/genus/core"
	"github.com/go-genus/genus/dialects/mysql"
	"github.com/go-genus/genus/dialects/postgres"
	"github.com/go-genus/genus/dialects/sqlite"
	"github.com/go-genus/genus/sharding"
)

// --- Fake driver para testes ---

var fakeDriverCounter uint64

type fakeDriver struct{}

func (d *fakeDriver) Open(name string) (driver.Conn, error) {
	return &fakeConn{}, nil
}

// fakeDriverFailAfterN falha no N-ésimo Open
type fakeDriverFailAfterN struct {
	count  int
	failAt int
}

func (d *fakeDriverFailAfterN) Open(name string) (driver.Conn, error) {
	d.count++
	if d.count >= d.failAt {
		return nil, fmt.Errorf("connection failed at attempt %d", d.count)
	}
	return &fakeConn{}, nil
}

type fakeConn struct{}

func (c *fakeConn) Prepare(query string) (driver.Stmt, error) {
	return &fakeStmt{}, nil
}
func (c *fakeConn) Close() error              { return nil }
func (c *fakeConn) Begin() (driver.Tx, error) { return &fakeTx{}, nil }

type fakeStmt struct{}

func (s *fakeStmt) Close() error  { return nil }
func (s *fakeStmt) NumInput() int { return 0 }
func (s *fakeStmt) Exec(args []driver.Value) (driver.Result, error) {
	return &fakeResult{}, nil
}
func (s *fakeStmt) Query(args []driver.Value) (driver.Rows, error) {
	return &fakeRows{}, nil
}

type fakeTx struct{}

func (t *fakeTx) Commit() error   { return nil }
func (t *fakeTx) Rollback() error { return nil }

type fakeResult struct{}

func (r *fakeResult) LastInsertId() (int64, error) { return 0, nil }
func (r *fakeResult) RowsAffected() (int64, error) { return 0, nil }

type fakeRows struct{ closed bool }

func (r *fakeRows) Columns() []string              { return []string{} }
func (r *fakeRows) Close() error                   { r.closed = true; return nil }
func (r *fakeRows) Next(dest []driver.Value) error { return fmt.Errorf("no rows") }

// fakeFailDriver sempre falha no Open
type fakeFailDriver struct{}

func (d *fakeFailDriver) Open(name string) (driver.Conn, error) {
	return nil, fmt.Errorf("connection failed")
}

// fakeFailOnDSNDriver implementa DriverContext para falhar em DSNs específicas.
// sql.Open chama OpenConnector se disponível, que pode retornar erro.
type fakeFailOnDSNDriver struct {
	failDSN string
}

func (d *fakeFailOnDSNDriver) Open(name string) (driver.Conn, error) {
	return &fakeConn{}, nil
}

func (d *fakeFailOnDSNDriver) OpenConnector(dsn string) (driver.Connector, error) {
	if dsn == d.failDSN {
		return nil, fmt.Errorf("connector failed for DSN: %s", dsn)
	}
	return &fakeConnector{driver: d}, nil
}

type fakeConnector struct {
	driver driver.Driver
}

func (c *fakeConnector) Connect(_ context.Context) (driver.Conn, error) {
	return &fakeConn{}, nil
}

func (c *fakeConnector) Driver() driver.Driver {
	return c.driver
}

// registerFakeFailOnDSNDriver registra um driver que falha no sql.Open para DSN específica
func registerFakeFailOnDSNDriver(failDSN string) string {
	n := atomic.AddUint64(&fakeDriverCounter, 1)
	name := fmt.Sprintf("genus_faildsn_%d_%d", time.Now().UnixNano(), n)
	sql.Register(name, &fakeFailOnDSNDriver{failDSN: failDSN})
	return name
}

// registerFakeDriver registra um driver fake com nome unico
func registerFakeDriver() string {
	n := atomic.AddUint64(&fakeDriverCounter, 1)
	name := fmt.Sprintf("genus_fake_%d_%d", time.Now().UnixNano(), n)
	sql.Register(name, &fakeDriver{})
	return name
}

// registerFakeFailDriver registra um driver que falha ao conectar
func registerFakeFailDriver() string {
	n := atomic.AddUint64(&fakeDriverCounter, 1)
	name := fmt.Sprintf("genus_fakefail_%d_%d", time.Now().UnixNano(), n)
	sql.Register(name, &fakeFailDriver{})
	return name
}

// safeRegister tenta registrar um driver, ignorando panic se já registrado
func safeRegister(name string, d driver.Driver) {
	defer func() { recover() }()
	sql.Register(name, d)
}

func init() {
	// Registra drivers com nomes conhecidos para testar branches de detecção
	safeRegister("postgres", &fakeDriver{})
	safeRegister("pgx", &fakeDriver{})
	safeRegister("mysql", &fakeDriver{})
	safeRegister("sqlite3", &fakeDriver{})
	safeRegister("sqlite", &fakeDriver{})
}

// --- Mock Dialect ---

type mockDialect struct{}

func (d *mockDialect) Placeholder(n int) string           { return "?" }
func (d *mockDialect) QuoteIdentifier(name string) string { return `"` + name + `"` }
func (d *mockDialect) GetType(goType string) string       { return "TEXT" }

// --- Mock Logger ---

type mockLogger struct {
	queries []string
	errors  []string
}

func (l *mockLogger) LogQuery(query string, args []interface{}, duration int64) {
	l.queries = append(l.queries, query)
}
func (l *mockLogger) LogError(query string, args []interface{}, err error) {
	l.errors = append(l.errors, query)
}

// --- Models para teste ---

type TestUser struct {
	core.Model
	Name  string `db:"name"`
	Email string `db:"email"`
}

type CustomTableModel struct {
	core.Model
	Title string `db:"title"`
}

func (m CustomTableModel) TableName() string {
	return "custom_models"
}

type CamelCaseModel struct {
	core.Model
}

type SimpleModel struct {
	ID int64 `db:"id"`
}

// =========================================
// Tests: New, NewWithLogger, DB()
// =========================================

func TestNew(t *testing.T) {
	dialect := &mockDialect{}
	g := New(nil, dialect)
	if g == nil {
		t.Fatal("New() returned nil")
	}
}

func TestNew_WithRealDB(t *testing.T) {
	driverName := registerFakeDriver()
	sqlDB, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

	dialect := postgres.New()
	g := New(sqlDB, dialect)
	if g == nil {
		t.Fatal("New() returned nil")
	}
}

func TestNewWithLogger(t *testing.T) {
	dialect := &mockDialect{}
	logger := &mockLogger{}
	g := NewWithLogger(nil, dialect, logger)
	if g == nil {
		t.Fatal("NewWithLogger() returned nil")
	}
}

func TestGenus_DB(t *testing.T) {
	dialect := &mockDialect{}
	g := New(nil, dialect)
	db := g.DB()
	if db == nil {
		t.Fatal("DB() returned nil")
	}
}

// =========================================
// Tests: Open
// =========================================

func TestOpen_Success(t *testing.T) {
	driverName := registerFakeDriver()
	g, err := Open(driverName, "test_dsn")
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	if g == nil {
		t.Fatal("Open() returned nil")
	}
	if g.DB() == nil {
		t.Fatal("Open().DB() returned nil")
	}
}

func TestOpen_InvalidDriver(t *testing.T) {
	_, err := Open("invalid_driver_that_does_not_exist", "some_dsn")
	if err == nil {
		t.Error("Open() with invalid driver should return error")
	}
}

// =========================================
// Tests: OpenWithConfig
// =========================================

func TestOpenWithConfig_Success(t *testing.T) {
	driverName := registerFakeDriver()
	config := core.DefaultPoolConfig()
	g, err := OpenWithConfig(driverName, "test_dsn", config)
	if err != nil {
		t.Fatalf("OpenWithConfig() error: %v", err)
	}
	if g == nil {
		t.Fatal("OpenWithConfig() returned nil")
	}
}

func TestOpenWithConfig_HighPerformance(t *testing.T) {
	driverName := registerFakeDriver()
	config := core.HighPerformancePoolConfig()
	g, err := OpenWithConfig(driverName, "test_dsn", config)
	if err != nil {
		t.Fatalf("OpenWithConfig() error: %v", err)
	}
	if g == nil {
		t.Fatal("OpenWithConfig() returned nil")
	}
}

func TestOpenWithConfig_InvalidDriver(t *testing.T) {
	config := core.DefaultPoolConfig()
	_, err := OpenWithConfig("invalid_driver_that_does_not_exist", "some_dsn", config)
	if err == nil {
		t.Error("OpenWithConfig() with invalid driver should return error")
	}
}

// =========================================
// Tests: OpenWithReplicas
// =========================================

func TestOpenWithReplicas_Success(t *testing.T) {
	driverName := registerFakeDriver()
	config := ReplicaConfig{
		PrimaryDSN:  "primary_dsn",
		ReplicaDSNs: []string{"replica1_dsn", "replica2_dsn"},
	}
	g, err := OpenWithReplicas(driverName, config)
	if err != nil {
		t.Fatalf("OpenWithReplicas() error: %v", err)
	}
	if g == nil {
		t.Fatal("OpenWithReplicas() returned nil")
	}
}

func TestOpenWithReplicas_WithPoolConfig(t *testing.T) {
	driverName := registerFakeDriver()
	poolConfig := core.DefaultPoolConfig()
	config := ReplicaConfig{
		PrimaryDSN:  "primary_dsn",
		ReplicaDSNs: []string{"replica1_dsn"},
		PoolConfig:  &poolConfig,
	}
	g, err := OpenWithReplicas(driverName, config)
	if err != nil {
		t.Fatalf("OpenWithReplicas() error: %v", err)
	}
	if g == nil {
		t.Fatal("OpenWithReplicas() returned nil")
	}
}

func TestOpenWithReplicas_NoReplicas(t *testing.T) {
	driverName := registerFakeDriver()
	config := ReplicaConfig{
		PrimaryDSN:  "primary_dsn",
		ReplicaDSNs: []string{},
	}
	g, err := OpenWithReplicas(driverName, config)
	if err != nil {
		t.Fatalf("OpenWithReplicas() error: %v", err)
	}
	if g == nil {
		t.Fatal("OpenWithReplicas() returned nil")
	}
}

func TestOpenWithReplicas_InvalidDriver(t *testing.T) {
	config := ReplicaConfig{
		PrimaryDSN:  "some_dsn",
		ReplicaDSNs: []string{"replica1", "replica2"},
	}
	_, err := OpenWithReplicas("invalid_driver_that_does_not_exist", config)
	if err == nil {
		t.Error("OpenWithReplicas() with invalid driver should return error")
	}
}

func TestOpenWithReplicas_ReplicaOpenFails(t *testing.T) {
	// Usa driver que falha no sql.Open para DSN "fail_replica"
	driverName := registerFakeFailOnDSNDriver("fail_replica")
	config := ReplicaConfig{
		PrimaryDSN:  "primary_dsn",
		ReplicaDSNs: []string{"good_replica", "fail_replica"},
	}
	_, err := OpenWithReplicas(driverName, config)
	if err == nil {
		t.Error("OpenWithReplicas() should fail when replica Open fails")
	}
}

func TestOpenWithReplicas_FirstReplicaFails(t *testing.T) {
	// Usa driver que falha para a primeira replica
	driverName := registerFakeFailOnDSNDriver("fail_first")
	config := ReplicaConfig{
		PrimaryDSN:  "primary_dsn",
		ReplicaDSNs: []string{"fail_first"},
	}
	_, err := OpenWithReplicas(driverName, config)
	if err == nil {
		t.Error("OpenWithReplicas() should fail when first replica Open fails")
	}
}

// =========================================
// Tests: OpenWithTracing
// =========================================

func TestOpenWithTracing_Success(t *testing.T) {
	driverName := registerFakeDriver()
	config := TracingConfig{
		DBSystem:   "postgresql",
		DBName:     "testdb",
		ServerAddr: "localhost:5432",
	}
	g, err := OpenWithTracing(driverName, "test_dsn", config)
	if err != nil {
		t.Fatalf("OpenWithTracing() error: %v", err)
	}
	if g == nil {
		t.Fatal("OpenWithTracing() returned nil")
	}
}

func TestOpenWithTracing_WithPoolConfig(t *testing.T) {
	driverName := registerFakeDriver()
	poolConfig := core.DefaultPoolConfig()
	config := TracingConfig{
		PoolConfig: &poolConfig,
	}
	g, err := OpenWithTracing(driverName, "test_dsn", config)
	if err != nil {
		t.Fatalf("OpenWithTracing() error: %v", err)
	}
	if g == nil {
		t.Fatal("OpenWithTracing() returned nil")
	}
}

func TestOpenWithTracing_AutoDetectDBSystem_Postgres(t *testing.T) {
	driverName := registerFakeDriver()
	// Registra com nome que contenha "postgres" para detecção
	// Mas na verdade a detecção é pelo parâmetro driver, não pelo nome do registro.
	// Precisamos testar passando um driver com nome real.
	// Vamos testar indiretamente - o sucesso sem DBSystem já cobre o default.
	config := TracingConfig{}
	g, err := OpenWithTracing(driverName, "test_dsn", config)
	if err != nil {
		t.Fatalf("OpenWithTracing() error: %v", err)
	}
	if g == nil {
		t.Fatal("OpenWithTracing() returned nil")
	}
}

func TestOpenWithTracing_InvalidDriver(t *testing.T) {
	config := TracingConfig{}
	_, err := OpenWithTracing("invalid_driver_that_does_not_exist", "some_dsn", config)
	if err == nil {
		t.Error("OpenWithTracing() with invalid driver should return error")
	}
}

// Teste de detecção de DBSystem para diferentes drivers
func TestOpenWithTracing_DBSystemDetection(t *testing.T) {
	// Testamos os branches de detecção de dbSystem indiretamente
	// criando drivers com nomes que mapeiam para drivers conhecidos.
	// Porém sql.Open usa o nome registrado, não um nome semântico.
	// A detecção usa o parâmetro `driver` passado para a função.

	// Para cobrir os branches, precisamos que o driver exista.
	// Criamos drivers com nomes específicos.
	tests := []struct {
		name     string
		dbSystem string
	}{
		{"with explicit dbSystem", "custom_system"},
		{"empty dbSystem auto-detect", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driverName := registerFakeDriver()
			config := TracingConfig{
				DBSystem: tt.dbSystem,
			}
			g, err := OpenWithTracing(driverName, "test_dsn", config)
			if err != nil {
				t.Fatalf("OpenWithTracing() error: %v", err)
			}
			if g == nil {
				t.Fatal("returned nil")
			}
		})
	}
}

// =========================================
// Tests: OpenWithShards
// =========================================

func TestOpenWithShards_Success(t *testing.T) {
	driverName := registerFakeDriver()
	config := ShardConfig{
		DSNs: []string{"shard1_dsn", "shard2_dsn"},
	}
	sg, err := OpenWithShards(driverName, config)
	if err != nil {
		t.Fatalf("OpenWithShards() error: %v", err)
	}
	if sg == nil {
		t.Fatal("OpenWithShards() returned nil")
	}
}

func TestOpenWithShards_WithPoolConfig(t *testing.T) {
	driverName := registerFakeDriver()
	poolConfig := core.DefaultPoolConfig()
	config := ShardConfig{
		DSNs:       []string{"shard1_dsn", "shard2_dsn"},
		PoolConfig: &poolConfig,
	}
	sg, err := OpenWithShards(driverName, config)
	if err != nil {
		t.Fatalf("OpenWithShards() error: %v", err)
	}
	if sg == nil {
		t.Fatal("OpenWithShards() returned nil")
	}
}

func TestOpenWithShards_WithStrategy(t *testing.T) {
	driverName := registerFakeDriver()
	config := ShardConfig{
		DSNs:     []string{"shard1_dsn", "shard2_dsn", "shard3_dsn"},
		Strategy: sharding.NewConsistentHashStrategy(100),
	}
	sg, err := OpenWithShards(driverName, config)
	if err != nil {
		t.Fatalf("OpenWithShards() error: %v", err)
	}
	if sg == nil {
		t.Fatal("OpenWithShards() returned nil")
	}
}

func TestOpenWithShards_InvalidDriver(t *testing.T) {
	config := ShardConfig{
		DSNs: []string{"shard1_dsn"},
	}
	_, err := OpenWithShards("invalid_driver_that_does_not_exist", config)
	if err == nil {
		t.Error("OpenWithShards() with invalid driver should return error")
	}
}

// =========================================
// Tests: ShardedGenus methods
// =========================================

func TestShardedGenus_ShardedDB(t *testing.T) {
	driverName := registerFakeDriver()
	config := ShardConfig{
		DSNs: []string{"shard1_dsn", "shard2_dsn"},
	}
	sg, err := OpenWithShards(driverName, config)
	if err != nil {
		t.Fatalf("OpenWithShards() error: %v", err)
	}

	sdb := sg.ShardedDB()
	if sdb == nil {
		t.Fatal("ShardedDB() returned nil")
	}
}

func TestShardedGenus_Executor(t *testing.T) {
	driverName := registerFakeDriver()
	config := ShardConfig{
		DSNs: []string{"shard1_dsn", "shard2_dsn"},
	}
	sg, err := OpenWithShards(driverName, config)
	if err != nil {
		t.Fatalf("OpenWithShards() error: %v", err)
	}

	exec := sg.Executor()
	if exec == nil {
		t.Fatal("Executor() returned nil")
	}
}

func TestShardedGenus_NumShards(t *testing.T) {
	driverName := registerFakeDriver()
	config := ShardConfig{
		DSNs: []string{"shard1_dsn", "shard2_dsn", "shard3_dsn"},
	}
	sg, err := OpenWithShards(driverName, config)
	if err != nil {
		t.Fatalf("OpenWithShards() error: %v", err)
	}

	numShards := sg.NumShards()
	if numShards != 3 {
		t.Errorf("NumShards() = %d, want 3", numShards)
	}
}

func TestShardedGenus_Close(t *testing.T) {
	driverName := registerFakeDriver()
	config := ShardConfig{
		DSNs: []string{"shard1_dsn", "shard2_dsn"},
	}
	sg, err := OpenWithShards(driverName, config)
	if err != nil {
		t.Fatalf("OpenWithShards() error: %v", err)
	}

	err = sg.Close()
	if err != nil {
		t.Errorf("Close() error: %v", err)
	}
}

func TestShardedTable(t *testing.T) {
	driverName := registerFakeDriver()
	config := ShardConfig{
		DSNs: []string{"shard1_dsn", "shard2_dsn"},
	}
	sg, err := OpenWithShards(driverName, config)
	if err != nil {
		t.Fatalf("OpenWithShards() error: %v", err)
	}

	builder := ShardedTable[TestUser](sg)
	if builder == nil {
		t.Fatal("ShardedTable[TestUser]() returned nil")
	}
}

// =========================================
// Tests: getTableName e toSnakeCase
// =========================================

func TestGetTableName_WithTableNamer(t *testing.T) {
	model := CustomTableModel{}
	name := getTableName(model)
	if name != "custom_models" {
		t.Errorf("getTableName(CustomTableModel) = %q, want %q", name, "custom_models")
	}
}

func TestGetTableName_WithoutTableNamer(t *testing.T) {
	model := TestUser{}
	name := getTableName(model)
	if name != "test_user" {
		t.Errorf("getTableName(TestUser) = %q, want %q", name, "test_user")
	}
}

func TestGetTableName_Pointer(t *testing.T) {
	model := &SimpleModel{}
	name := getTableName(model)
	if name != "simple_model" {
		t.Errorf("getTableName(&SimpleModel{}) = %q, want %q", name, "simple_model")
	}
}

func TestGetTableName_CamelCase(t *testing.T) {
	model := CamelCaseModel{}
	name := getTableName(model)
	if name != "camel_case_model" {
		t.Errorf("getTableName(CamelCaseModel) = %q, want %q", name, "camel_case_model")
	}
}

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple lowercase", "user", "user"},
		{"camel case", "UserName", "user_name"},
		{"multiple words", "MyLongModelName", "my_long_model_name"},
		{"single char", "A", "a"},
		{"empty string", "", ""},
		{"all uppercase", "ABC", "a_b_c"},
		{"already snake", "user_name", "user_name"},
		{"starts lowercase", "camelCase", "camel_case"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toSnakeCase(tt.input)
			if got != tt.want {
				t.Errorf("toSnakeCase(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// =========================================
// Tests: Table, FastTable, UltraFastTable
// =========================================

func TestTable(t *testing.T) {
	dialect := postgres.New()
	g := New(nil, dialect)

	builder := Table[TestUser](g)
	if builder == nil {
		t.Fatal("Table[TestUser]() returned nil")
	}
}

func TestTable_WithCustomTableName(t *testing.T) {
	dialect := mysql.New()
	g := New(nil, dialect)

	builder := Table[CustomTableModel](g)
	if builder == nil {
		t.Fatal("Table[CustomTableModel]() returned nil")
	}
}

func TestFastTable(t *testing.T) {
	dialect := sqlite.New()
	g := New(nil, dialect)

	builder := FastTable[TestUser](g)
	if builder == nil {
		t.Fatal("FastTable[TestUser]() returned nil")
	}
}

func TestUltraFastTable(t *testing.T) {
	dialect := postgres.New()
	g := New(nil, dialect)

	builder := UltraFastTable[TestUser](g)
	if builder == nil {
		t.Fatal("UltraFastTable[TestUser]() returned nil")
	}
}

func TestTable_DifferentDialects(t *testing.T) {
	dialects := []struct {
		name    string
		dialect core.Dialect
	}{
		{"postgres", postgres.New()},
		{"mysql", mysql.New()},
		{"sqlite", sqlite.New()},
	}

	for _, dd := range dialects {
		t.Run(dd.name, func(t *testing.T) {
			g := New(nil, dd.dialect)
			builder := Table[TestUser](g)
			if builder == nil {
				t.Fatalf("Table[TestUser]() with %s dialect returned nil", dd.name)
			}
		})
	}
}

func TestFastTable_DifferentDialects(t *testing.T) {
	dialects := []struct {
		name    string
		dialect core.Dialect
	}{
		{"postgres", postgres.New()},
		{"mysql", mysql.New()},
		{"sqlite", sqlite.New()},
	}

	for _, dd := range dialects {
		t.Run(dd.name, func(t *testing.T) {
			g := New(nil, dd.dialect)
			builder := FastTable[TestUser](g)
			if builder == nil {
				t.Fatalf("FastTable[TestUser]() with %s dialect returned nil", dd.name)
			}
		})
	}
}

func TestUltraFastTable_DifferentDialects(t *testing.T) {
	dialects := []struct {
		name    string
		dialect core.Dialect
	}{
		{"postgres", postgres.New()},
		{"mysql", mysql.New()},
		{"sqlite", sqlite.New()},
	}

	for _, dd := range dialects {
		t.Run(dd.name, func(t *testing.T) {
			g := New(nil, dd.dialect)
			builder := UltraFastTable[TestUser](g)
			if builder == nil {
				t.Fatalf("UltraFastTable[TestUser]() with %s dialect returned nil", dd.name)
			}
		})
	}
}

// =========================================
// Tests: RegisterModels
// =========================================

func TestRegisterModels_EmptySlice(t *testing.T) {
	err := RegisterModels()
	if err != nil {
		t.Errorf("RegisterModels() with no models should not error, got: %v", err)
	}
}

func TestRegisterModels_SimpleModel(t *testing.T) {
	err := RegisterModels(&TestUser{})
	if err != nil {
		t.Errorf("RegisterModels(&TestUser{}) should not error, got: %v", err)
	}
}

func TestRegisterModels_MultipleModels(t *testing.T) {
	err := RegisterModels(&TestUser{}, &CustomTableModel{})
	if err != nil {
		t.Errorf("RegisterModels() with multiple models should not error, got: %v", err)
	}
}

// =========================================
// Tests: Config structs
// =========================================

func TestReplicaConfig_Struct(t *testing.T) {
	config := ReplicaConfig{
		PrimaryDSN:  "postgres://primary:5432/db",
		ReplicaDSNs: []string{"postgres://replica1:5432/db"},
		PoolConfig:  nil,
	}

	if config.PrimaryDSN != "postgres://primary:5432/db" {
		t.Error("PrimaryDSN not set correctly")
	}
	if len(config.ReplicaDSNs) != 1 {
		t.Error("ReplicaDSNs not set correctly")
	}
}

func TestShardConfig_Struct(t *testing.T) {
	config := ShardConfig{
		DSNs:       []string{"shard1", "shard2"},
		Strategy:   nil,
		PoolConfig: nil,
	}

	if len(config.DSNs) != 2 {
		t.Error("DSNs not set correctly")
	}
}

func TestTracingConfig_Struct(t *testing.T) {
	config := TracingConfig{
		Tracer:     nil,
		DBSystem:   "postgresql",
		DBName:     "testdb",
		ServerAddr: "localhost:5432",
		PoolConfig: nil,
	}

	if config.DBSystem != "postgresql" {
		t.Error("DBSystem not set correctly")
	}
	if config.DBName != "testdb" {
		t.Error("DBName not set correctly")
	}
	if config.ServerAddr != "localhost:5432" {
		t.Error("ServerAddr not set correctly")
	}
}

// =========================================
// Tests: Re-exported vars e types
// =========================================

func TestReExportedVars(t *testing.T) {
	if WithShardKey == nil {
		t.Error("WithShardKey should not be nil")
	}
	if ShardKeyFromContext == nil {
		t.Error("ShardKeyFromContext should not be nil")
	}
	if NewConsistentHashStrategy == nil {
		t.Error("NewConsistentHashStrategy should not be nil")
	}
	if NewOTelAdapter == nil {
		t.Error("NewOTelAdapter should not be nil")
	}
	if NewSimpleTracer == nil {
		t.Error("NewSimpleTracer should not be nil")
	}
	if NewTracedExecutor == nil {
		t.Error("NewTracedExecutor should not be nil")
	}
}

func TestTypeAliases(t *testing.T) {
	// Verifica que os type aliases são acessíveis
	var _ Int64ShardKey = sharding.Int64ShardKey(42)
	var _ StringShardKey = sharding.StringShardKey("tenant1")

	// NoopTracer deve ser instanciável
	var tracer NoopTracer
	_ = tracer
}

// =========================================
// Tests: OpenWithTracing DBSystem detection branches
// =========================================

func TestOpenWithTracing_DBSystem_Postgres(t *testing.T) {
	g, err := OpenWithTracing("postgres", "test_dsn", TracingConfig{})
	if err != nil {
		t.Fatalf("OpenWithTracing(postgres) error: %v", err)
	}
	if g == nil {
		t.Fatal("returned nil")
	}
}

func TestOpenWithTracing_DBSystem_Pgx(t *testing.T) {
	g, err := OpenWithTracing("pgx", "test_dsn", TracingConfig{})
	if err != nil {
		t.Fatalf("OpenWithTracing(pgx) error: %v", err)
	}
	if g == nil {
		t.Fatal("returned nil")
	}
}

func TestOpenWithTracing_DBSystem_MySQL(t *testing.T) {
	g, err := OpenWithTracing("mysql", "test_dsn", TracingConfig{})
	if err != nil {
		t.Fatalf("OpenWithTracing(mysql) error: %v", err)
	}
	if g == nil {
		t.Fatal("returned nil")
	}
}

func TestOpenWithTracing_DBSystem_SQLite3(t *testing.T) {
	g, err := OpenWithTracing("sqlite3", "test_dsn", TracingConfig{})
	if err != nil {
		t.Fatalf("OpenWithTracing(sqlite3) error: %v", err)
	}
	if g == nil {
		t.Fatal("returned nil")
	}
}

func TestOpenWithTracing_DBSystem_SQLite(t *testing.T) {
	g, err := OpenWithTracing("sqlite", "test_dsn", TracingConfig{})
	if err != nil {
		t.Fatalf("OpenWithTracing(sqlite) error: %v", err)
	}
	if g == nil {
		t.Fatal("returned nil")
	}
}

func TestOpenWithTracing_DBSystem_Default(t *testing.T) {
	// Usa um driver genérico que cai no default do switch
	driverName := registerFakeDriver()
	g, err := OpenWithTracing(driverName, "test_dsn", TracingConfig{})
	if err != nil {
		t.Fatalf("OpenWithTracing(custom) error: %v", err)
	}
	if g == nil {
		t.Fatal("returned nil")
	}
}

func TestOpenWithTracing_DBSystem_ExplicitOverride(t *testing.T) {
	g, err := OpenWithTracing("postgres", "test_dsn", TracingConfig{
		DBSystem: "custom_system",
	})
	if err != nil {
		t.Fatalf("OpenWithTracing() error: %v", err)
	}
	if g == nil {
		t.Fatal("returned nil")
	}
}

// =========================================
// Tests: RegisterModels error branch
// =========================================

// ModelWithBadRelation tem uma tag relation inválida para causar erro
type ModelWithBadRelation struct {
	core.Model
	BadField string `relation:","`
}

func TestRegisterModels_Error(t *testing.T) {
	err := RegisterModels(&ModelWithBadRelation{})
	if err == nil {
		t.Error("RegisterModels() with bad relation tag should return error")
	}
}

// =========================================
// Tests: OpenWithReplicas com drivers conhecidos
// =========================================

func TestOpenWithReplicas_WithKnownDriver(t *testing.T) {
	config := ReplicaConfig{
		PrimaryDSN:  "primary_dsn",
		ReplicaDSNs: []string{"replica1_dsn", "replica2_dsn"},
	}
	g, err := OpenWithReplicas("postgres", config)
	if err != nil {
		t.Fatalf("OpenWithReplicas(postgres) error: %v", err)
	}
	if g == nil {
		t.Fatal("returned nil")
	}
}

// =========================================
// Tests: Open/OpenWithConfig com drivers conhecidos
// =========================================

func TestOpen_Postgres(t *testing.T) {
	g, err := Open("postgres", "test_dsn")
	if err != nil {
		t.Fatalf("Open(postgres) error: %v", err)
	}
	if g == nil {
		t.Fatal("returned nil")
	}
}

func TestOpen_MySQL(t *testing.T) {
	g, err := Open("mysql", "test_dsn")
	if err != nil {
		t.Fatalf("Open(mysql) error: %v", err)
	}
	if g == nil {
		t.Fatal("returned nil")
	}
}

func TestOpen_SQLite(t *testing.T) {
	g, err := Open("sqlite3", "test_dsn")
	if err != nil {
		t.Fatalf("Open(sqlite3) error: %v", err)
	}
	if g == nil {
		t.Fatal("returned nil")
	}
}
