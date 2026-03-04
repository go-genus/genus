package genus

import (
	"database/sql"
	"reflect"
	"strings"

	"github.com/GabrielOnRails/genus/core"
	"github.com/GabrielOnRails/genus/dialects"
	"github.com/GabrielOnRails/genus/query"
	"github.com/GabrielOnRails/genus/sharding"
	"github.com/GabrielOnRails/genus/tracing"
)

// Genus é a interface pública principal do ORM.
type Genus struct {
	db *core.DB
}

// Open cria uma nova conexão com o banco de dados.
// O dialeto é detectado automaticamente baseado no driver:
//   - postgres, pgx → PostgreSQL
//   - mysql → MySQL
//   - sqlite3, sqlite → SQLite
func Open(driver, dsn string) (*Genus, error) {
	sqlDB, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}

	dialect := dialects.DetectDialect(driver)

	return &Genus{
		db: core.New(sqlDB, dialect),
	}, nil
}

// OpenWithConfig cria uma nova conexão com configurações de pool personalizadas.
// O dialeto é detectado automaticamente baseado no driver.
//
// Exemplo:
//
//	db, err := genus.OpenWithConfig("postgres", dsn, core.DefaultPoolConfig())
//	db, err := genus.OpenWithConfig("mysql", dsn, core.HighPerformancePoolConfig())
func OpenWithConfig(driver, dsn string, config core.PoolConfig) (*Genus, error) {
	sqlDB, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}

	config.Apply(sqlDB)
	dialect := dialects.DetectDialect(driver)

	return &Genus{
		db: core.New(sqlDB, dialect),
	}, nil
}

// ReplicaConfig contém configurações para conexão com read replicas.
type ReplicaConfig struct {
	// PrimaryDSN é a string de conexão do primary (escrita + leitura).
	PrimaryDSN string

	// ReplicaDSNs são as strings de conexão das replicas (somente leitura).
	// Reads são distribuídos entre replicas usando round-robin.
	ReplicaDSNs []string

	// PoolConfig é a configuração de pool aplicada a todas as conexões.
	// Se não fornecido, usa DefaultPoolConfig().
	PoolConfig *core.PoolConfig
}

// OpenWithReplicas cria uma conexão com suporte a read replicas.
// Writes sempre vão para o primary, reads vão para replicas em round-robin.
// Se não houver replicas configuradas, todas as operações vão para o primary.
//
// Use core.WithPrimary(ctx) para forçar leitura do primary quando necessário
// (ex: read-after-write consistency).
//
// Exemplo:
//
//	config := genus.ReplicaConfig{
//	    PrimaryDSN:  "postgres://user:pass@primary:5432/db",
//	    ReplicaDSNs: []string{
//	        "postgres://user:pass@replica1:5432/db",
//	        "postgres://user:pass@replica2:5432/db",
//	    },
//	}
//	db, err := genus.OpenWithReplicas("postgres", config)
//
//	// Leituras vão para replicas automaticamente
//	users, _ := genus.Table[User](db).Find(ctx)
//
//	// Forçar leitura do primary
//	users, _ := genus.Table[User](db).Find(core.WithPrimary(ctx))
func OpenWithReplicas(driver string, config ReplicaConfig) (*Genus, error) {
	// Abre conexão primary
	primaryDB, err := sql.Open(driver, config.PrimaryDSN)
	if err != nil {
		return nil, err
	}

	// Aplica configuração de pool
	poolConfig := config.PoolConfig
	if poolConfig == nil {
		defaultConfig := core.DefaultPoolConfig()
		poolConfig = &defaultConfig
	}
	poolConfig.Apply(primaryDB)

	// Abre conexões das replicas
	replicas := make([]*sql.DB, len(config.ReplicaDSNs))
	for i, dsn := range config.ReplicaDSNs {
		replicaDB, err := sql.Open(driver, dsn)
		if err != nil {
			// Fecha conexões já abertas em caso de erro
			primaryDB.Close()
			for j := 0; j < i; j++ {
				replicas[j].Close()
			}
			return nil, err
		}
		poolConfig.Apply(replicaDB)
		replicas[i] = replicaDB
	}

	// Cria MultiExecutor
	executor := core.NewMultiExecutor(primaryDB, replicas...)
	dialect := dialects.DetectDialect(driver)

	return &Genus{
		db: core.NewWithExecutor(executor, dialect),
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

// FastTable cria um query builder otimizado para alta performance.
// Use quando performance é crítica. Tem menos features que Table mas é mais rápido.
//
// Otimizações:
//   - Cache de prepared statements
//   - Zero-copy string operations
//   - Pre-alocação de slices
//   - Cache de field maps
//
// Exemplo:
//
//	users, _ := genus.FastTable[User](db).
//	    Where(UserFields.IsActive.Eq(true)).
//	    Find(ctx)
func FastTable[T any](g *Genus) *query.FastBuilder[T] {
	var model T
	tableName := getTableName(model)
	return query.NewFastBuilder[T](g.db.Executor(), g.db.Dialect(), tableName)
}

// UltraFastTable creates a zero-reflection query builder for maximum performance.
// For best results, register a scan function with query.RegisterScanFunc[T]() at init time.
// Without a registered scan function, it falls back to reflection-based scanning.
//
// Example with generated scanner:
//
//	// In init or main:
//	query.RegisterScanFunc[User](ScanUser)
//
//	// Then use:
//	users, _ := genus.UltraFastTable[User](db).
//	    Where(UserFields.IsActive.Eq(true)).
//	    Find(ctx)
func UltraFastTable[T any](g *Genus) *query.UltraFastBuilder[T] {
	var model T
	tableName := getTableName(model)
	return query.NewUltraFastBuilder[T](g.db.Executor(), g.db.Dialect(), tableName)
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

// ================================
// Sharding Support (v4.0)
// ================================

// ShardConfig contém configurações para sharding.
type ShardConfig struct {
	// DSNs é a lista de strings de conexão para cada shard.
	// Os shards são numerados na ordem em que aparecem (0, 1, 2, ...).
	DSNs []string

	// Strategy define como as chaves são mapeadas para shards.
	// Se nil, usa ModuloStrategy (distribuição por módulo).
	// Para consistent hashing, use sharding.NewConsistentHashStrategy(replicas).
	Strategy sharding.ShardStrategy

	// PoolConfig é a configuração de pool aplicada a todos os shards.
	// Se nil, usa DefaultPoolConfig().
	PoolConfig *core.PoolConfig
}

// ShardedGenus é a interface para operações sharded.
type ShardedGenus struct {
	shardedDB *core.ShardedDB
	dialect   core.Dialect
}

// OpenWithShards cria uma conexão com suporte a database sharding.
// Cada shard é uma instância de banco de dados separada.
// Queries são roteadas para o shard correto baseado na shard key.
//
// Use sharding.WithShardKey(ctx, key) para especificar o shard:
//
//	// Usando shard key int64
//	ctx := sharding.WithShardKey(ctx, sharding.Int64ShardKey(userID))
//
//	// Usando shard key string
//	ctx := sharding.WithShardKey(ctx, sharding.StringShardKey(tenantID))
//
// Exemplo:
//
//	config := genus.ShardConfig{
//	    DSNs: []string{
//	        "postgres://user:pass@shard1:5432/db",
//	        "postgres://user:pass@shard2:5432/db",
//	        "postgres://user:pass@shard3:5432/db",
//	    },
//	    Strategy: sharding.NewConsistentHashStrategy(100),
//	}
//	db, err := genus.OpenWithShards("postgres", config)
//
//	// Query em shard específico
//	ctx := sharding.WithShardKey(ctx, sharding.Int64ShardKey(userID))
//	user, _ := genus.ShardedTable[User](db).First(ctx)
func OpenWithShards(driver string, config ShardConfig) (*ShardedGenus, error) {
	shardConfig := sharding.ShardConfig{
		DSNs:     config.DSNs,
		Strategy: config.Strategy,
	}

	manager, err := sharding.NewShardManager(driver, shardConfig)
	if err != nil {
		return nil, err
	}

	// Aplica configuração de pool a todos os shards
	poolConfig := config.PoolConfig
	if poolConfig == nil {
		defaultConfig := core.DefaultPoolConfig()
		poolConfig = &defaultConfig
	}
	for _, shard := range manager.AllShards() {
		poolConfig.Apply(shard.DB)
	}

	dialect := dialects.DetectDialect(driver)
	executor := core.NewShardExecutor(manager, dialect)
	shardedDB := core.NewShardedDB(executor, dialect)

	return &ShardedGenus{
		shardedDB: shardedDB,
		dialect:   dialect,
	}, nil
}

// ShardedDB retorna o core.ShardedDB subjacente para operações avançadas.
func (sg *ShardedGenus) ShardedDB() *core.ShardedDB {
	return sg.shardedDB
}

// Executor retorna o executor sharded.
func (sg *ShardedGenus) Executor() *core.ShardExecutor {
	return sg.shardedDB.Executor()
}

// NumShards retorna o número de shards configurados.
func (sg *ShardedGenus) NumShards() int {
	return sg.shardedDB.Executor().NumShards()
}

// Close fecha todas as conexões de shard.
func (sg *ShardedGenus) Close() error {
	return sg.shardedDB.Close()
}

// ShardedTable cria um query builder type-safe para tabelas sharded.
// Queries são roteadas para o shard baseado na shard key no contexto.
//
// Exemplo:
//
//	ctx := sharding.WithShardKey(ctx, sharding.Int64ShardKey(userID))
//	user, _ := genus.ShardedTable[User](db).First(ctx)
func ShardedTable[T any](sg *ShardedGenus) *query.Builder[T] {
	var model T
	tableName := getTableName(model)
	return query.NewBuilder[T](
		sg.shardedDB.Executor(),
		sg.shardedDB.Dialect(),
		sg.shardedDB.Logger(),
		tableName,
	)
}

// Re-exports para facilitar uso.
var (
	// WithShardKey adiciona uma shard key ao contexto.
	WithShardKey = sharding.WithShardKey

	// ShardKeyFromContext extrai a shard key do contexto.
	ShardKeyFromContext = sharding.ShardKeyFromContext

	// NewConsistentHashStrategy cria uma estratégia de consistent hashing.
	NewConsistentHashStrategy = sharding.NewConsistentHashStrategy
)

// Int64ShardKey é uma chave de sharding baseada em int64.
type Int64ShardKey = sharding.Int64ShardKey

// StringShardKey é uma chave de sharding baseada em string.
type StringShardKey = sharding.StringShardKey

// ShardStrategy define como as chaves são mapeadas para shards.
type ShardStrategy = sharding.ShardStrategy

// ModuloStrategy usa módulo simples para distribuir chaves.
type ModuloStrategy = sharding.ModuloStrategy

// ================================
// OpenTelemetry Integration (v4.0)
// ================================

// TracingConfig contém configurações de tracing.
type TracingConfig struct {
	// Tracer é o tracer a ser usado.
	// Se nil, tracing é desabilitado.
	Tracer tracing.Tracer

	// DBSystem é o sistema de banco de dados (ex: "postgresql", "mysql", "sqlite").
	// Se vazio, é detectado automaticamente.
	DBSystem string

	// DBName é o nome do banco de dados.
	DBName string

	// ServerAddr é o endereço do servidor (host:port).
	ServerAddr string

	// PoolConfig é a configuração de pool de conexões.
	// Se nil, usa DefaultPoolConfig().
	PoolConfig *core.PoolConfig
}

// OpenWithTracing cria uma conexão com suporte a distributed tracing.
// Todas as queries são instrumentadas automaticamente com spans.
//
// Exemplo com OpenTelemetry:
//
//	import "go.opentelemetry.io/otel"
//
//	otelTracer := otel.Tracer("genus")
//
//	adapter := tracing.NewOTelAdapter(tracing.OTelAdapterConfig{
//	    StartFunc: func(ctx context.Context, name string) (context.Context, interface{}) {
//	        return otelTracer.Start(ctx, name)
//	    },
//	    SetAttributeFunc: func(span interface{}, k string, v interface{}) {
//	        span.(trace.Span).SetAttributes(attribute.String(k, fmt.Sprintf("%v", v)))
//	    },
//	    RecordErrorFunc: func(span interface{}, err error) {
//	        span.(trace.Span).RecordError(err)
//	    },
//	    SetStatusFunc: func(span interface{}, ok bool, msg string) {
//	        if ok {
//	            span.(trace.Span).SetStatus(codes.Ok, "")
//	        } else {
//	            span.(trace.Span).SetStatus(codes.Error, msg)
//	        }
//	    },
//	    EndFunc: func(span interface{}) {
//	        span.(trace.Span).End()
//	    },
//	})
//
//	db, err := genus.OpenWithTracing("postgres", dsn, genus.TracingConfig{
//	    Tracer: adapter,
//	    DBName: "mydb",
//	})
//
// Exemplo com SimpleTracer para debugging:
//
//	simpleTracer := tracing.NewSimpleTracer(tracing.SimpleTracerConfig{
//	    OnStart: func(ctx context.Context, name string) context.Context {
//	        log.Printf("Starting: %s", name)
//	        return ctx
//	    },
//	    OnEnd: func(name string, durationMs int64, err error) {
//	        if err != nil {
//	            log.Printf("Finished: %s (error: %v) [%dms]", name, err, durationMs)
//	        } else {
//	            log.Printf("Finished: %s [%dms]", name, durationMs)
//	        }
//	    },
//	})
//
//	db, err := genus.OpenWithTracing("postgres", dsn, genus.TracingConfig{
//	    Tracer: simpleTracer,
//	})
func OpenWithTracing(driver, dsn string, config TracingConfig) (*Genus, error) {
	sqlDB, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}

	// Aplica configuração de pool
	poolConfig := config.PoolConfig
	if poolConfig == nil {
		defaultConfig := core.DefaultPoolConfig()
		poolConfig = &defaultConfig
	}
	poolConfig.Apply(sqlDB)

	dialect := dialects.DetectDialect(driver)

	// Detecta DBSystem se não fornecido
	dbSystem := config.DBSystem
	if dbSystem == "" {
		switch driver {
		case "postgres", "pgx":
			dbSystem = "postgresql"
		case "mysql":
			dbSystem = "mysql"
		case "sqlite3", "sqlite":
			dbSystem = "sqlite"
		default:
			dbSystem = driver
		}
	}

	// Cria executor com tracing
	tracedExecutor := tracing.NewTracedExecutor(sqlDB, tracing.TracedExecutorConfig{
		Tracer:     config.Tracer,
		DBSystem:   dbSystem,
		DBName:     config.DBName,
		ServerAddr: config.ServerAddr,
	})

	return &Genus{
		db: core.NewWithExecutor(tracedExecutor, dialect),
	}, nil
}

// Re-exports de tipos de tracing.

// Tracer é a interface para criar spans de tracing.
type Tracer = tracing.Tracer

// Span representa um span de tracing.
type Span = tracing.Span

// NoopTracer é um tracer que não faz nada.
type NoopTracer = tracing.NoopTracer

// OTelAdapter adapta a interface do OpenTelemetry para o Genus.
type OTelAdapter = tracing.OTelAdapter

// OTelAdapterConfig configura o adapter OpenTelemetry.
type OTelAdapterConfig = tracing.OTelAdapterConfig

// SimpleTracer é um tracer simples que usa callbacks.
type SimpleTracer = tracing.SimpleTracer

// SimpleTracerConfig configura o SimpleTracer.
type SimpleTracerConfig = tracing.SimpleTracerConfig

// NewOTelAdapter cria um novo adapter para OpenTelemetry.
var NewOTelAdapter = tracing.NewOTelAdapter

// NewSimpleTracer cria um tracer simples com callbacks.
var NewSimpleTracer = tracing.NewSimpleTracer

// NewTracedExecutor cria um executor com tracing.
var NewTracedExecutor = tracing.NewTracedExecutor

// TracedExecutorConfig configura o TracedExecutor.
type TracedExecutorConfig = tracing.TracedExecutorConfig
