# Changelog

Todas as mudanças notáveis neste projeto serão documentadas neste arquivo.

O formato é baseado em [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
e este projeto adere ao [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [6.0.0] - 2026-03-04

### Adicionado - Versão 6.0 (Query Intelligence & Advanced Patterns)

#### 1. Automatic Query Optimization

**Motivação:** Analisar queries e sugerir índices automaticamente para melhorar performance.

**Funcionalidades:**

- `QueryOptimizer` - Analisador de queries
- `QueryAnalysis` - Resultado com custo estimado, seq scans, recomendações
- `IndexSuggestion` - Sugestões de índices com SQL de criação
- `GetTableStats()` - Estatísticas de tabelas (PostgreSQL/MySQL)
- `ListUnusedIndexes()` / `ListDuplicateIndexes()` - Manutenção de índices
- `AutoIndex()` - Criação automática de índices sugeridos

**Exemplo:**

```go
optimizer := query.NewQueryOptimizer(db, dialect)

// Analisar query
analysis, _ := optimizer.Analyze(ctx, "SELECT * FROM users WHERE email = $1", "test@example.com")

fmt.Println("Estimated cost:", analysis.EstimatedCost)
fmt.Println("Seq scans:", analysis.SeqScans)
fmt.Println("Recommendations:", analysis.Recommendations)

// Sugestões de índices
for _, idx := range analysis.MissingIndexes {
    fmt.Printf("Suggested: %s (impact: %s)\n", idx.CreateSQL, idx.Impact)
}

// Criar índices automaticamente
optimizer.AutoIndex(ctx, analysis, false)

// Via Builder
analysis, _ = genus.Table[User](db).
    Where(UserFields.Email.Eq("test@example.com")).
    Optimize(ctx)
```

**Arquivos:**
- `query/optimizer.go` - Query optimizer completo

---

#### 2. N+1 Query Detection

**Motivação:** Detectar automaticamente problemas de N+1 queries em tempo de execução.

**Funcionalidades:**

- `N1Detector` - Detector de queries N+1
- `N1DetectorConfig` - Configuração (threshold, time window)
- `N1Detection` - Detecção com pattern, count, suggestion
- `N1DetectorExecutor` - Executor com detecção automática
- `GenerateReport()` / `PrintReport()` - Relatórios de N+1

**Exemplo:**

```go
detector := query.NewN1Detector(query.N1DetectorConfig{
    Enabled:    true,
    Threshold:  5,
    TimeWindow: 100 * time.Millisecond,
    OnDetection: func(det query.N1Detection) {
        log.Printf("N+1 DETECTED: %s (count: %d)\n%s",
            det.Pattern, det.Count, det.Suggestion)
    },
})

// Usar executor com detecção
executor := query.NewN1DetectorExecutor(db, detector)

// Gerar relatório
fmt.Println(detector.PrintReport())
```

**Arquivos:**
- `query/n1detector.go` - N+1 detector

---

#### 3. GraphQL Schema Generation

**Motivação:** Gerar schemas GraphQL automaticamente a partir de models Go.

**Funcionalidades:**

- `SchemaGenerator` - Gerador de schema GraphQL
- `RegisterType()` - Registra models para geração
- `GenerateSchema()` - Gera schema completo
- Suporte a: Types, Inputs, Enums, Connections (Relay)
- Geração automática de: Query type, Mutation type, Filters, OrderBy

**Exemplo:**

```go
gen := graphql.NewSchemaGenerator()

// Registrar models
gen.RegisterType(User{})
gen.RegisterType(Post{})

// Adicionar enums customizados
gen.AddEnum("UserStatus", []string{"ACTIVE", "INACTIVE", "PENDING"})

// Gerar schema
schema := gen.GenerateSchema()
fmt.Println(schema)

// Output:
// type User {
//   id: ID!
//   name: String!
//   email: String!
//   posts: [Post!]!
// }
//
// type Query {
//   user(id: ID!): User
//   users(first: Int, after: String, filter: UserFilter): UserConnection!
// }
//
// type Mutation {
//   createUser(input: UserInput!): User!
//   updateUser(id: ID!, input: UserInput!): User!
//   deleteUser(id: ID!): Boolean!
// }
```

**Arquivos:**
- `graphql/schema.go` - GraphQL schema generator

---

#### 4. gRPC/Protobuf Support

**Motivação:** Gerar arquivos .proto automaticamente para serviços gRPC.

**Funcionalidades:**

- `ProtoGenerator` - Gerador de arquivos .proto
- `RegisterMessage()` - Registra models como messages
- `GenerateProto()` - Gera arquivo .proto completo
- Geração automática de: Messages, Services CRUD, Streaming
- Suporte a: timestamps, field masks, pagination

**Exemplo:**

```go
gen := grpc.NewProtoGenerator("myapp", "github.com/myapp/proto")

// Registrar models
gen.RegisterMessage(User{})
gen.RegisterMessage(Post{})

// Gerar .proto
proto := gen.GenerateProto()
fmt.Println(proto)

// Output:
// syntax = "proto3";
// package myapp;
//
// import "google/protobuf/timestamp.proto";
//
// message User {
//   int64 id = 1;
//   string name = 2;
//   string email = 3;
//   google.protobuf.Timestamp created_at = 4;
// }
//
// service UserService {
//   rpc GetUser(GetUserRequest) returns (GetUserResponse);
//   rpc ListUsers(ListUsersRequest) returns (ListUsersResponse);
//   rpc CreateUser(CreateUserRequest) returns (CreateUserResponse);
//   rpc UpdateUser(UpdateUserRequest) returns (UpdateUserResponse);
//   rpc DeleteUser(DeleteUserRequest) returns (DeleteUserResponse);
//   rpc WatchUsers(ListUsersRequest) returns (stream User);
// }
```

**Arquivos:**
- `grpc/proto.go` - Protobuf/gRPC generator

---

#### 5. Schema Diff & Migration Generation

**Motivação:** Comparar schemas e gerar migrações automaticamente.

**Funcionalidades:**

- `SchemaDiffer` - Compara schemas de banco
- `GetCurrentSchema()` - Obtém schema atual do banco
- `GetSchemaFromModels()` - Gera schema a partir de structs
- `Diff()` - Compara e retorna mudanças
- `GenerateMigration()` - Gera SQL de migração
- Suporte a: ADD/DROP TABLE, ADD/DROP/MODIFY COLUMN

**Exemplo:**

```go
differ := migrate.NewSchemaDiffer(db, dialect)

// Obter schema atual
current, _ := differ.GetCurrentSchema(ctx)

// Schema desejado (a partir dos models)
target := differ.GetSchemaFromModels(User{}, Post{}, Comment{})

// Comparar
changes := differ.Diff(current, target)

for _, change := range changes {
    fmt.Printf("%s: %s\n", change.Type, change.Description)
    fmt.Println(change.SQL)
}

// Gerar arquivo de migração
migration := differ.GenerateMigration(changes)
fmt.Println(migration)
```

**Arquivos:**
- `migrate/diff.go` - Schema diff e migration generation

---

#### 6. Event Sourcing

**Motivação:** Suporte completo a event sourcing para sistemas baseados em eventos.

**Funcionalidades:**

- `EventStore` - Armazenamento de eventos
- `Event` - Estrutura de evento com aggregate, version, data
- `Aggregate` / `BaseAggregate` - Interface e implementação base
- `AggregateRepository` - Repository para aggregates
- `SnapshotStore` - Armazenamento de snapshots
- `EventBus` - Publicação/subscrição de eventos

**Exemplo:**

```go
// Event Store
store := eventsourcing.NewEventStore(db, dialect, eventsourcing.DefaultEventStoreConfig())
store.CreateTable(ctx)

// Definir aggregate
type UserAggregate struct {
    eventsourcing.BaseAggregate
    Name  string
    Email string
}

func (u *UserAggregate) Apply(event eventsourcing.Event) error {
    switch event.EventType {
    case "UserCreated":
        u.Name = event.Data["name"].(string)
        u.Email = event.Data["email"].(string)
    case "UserNameChanged":
        u.Name = event.Data["name"].(string)
    }
    return nil
}

// Criar aggregate
user := &UserAggregate{
    BaseAggregate: eventsourcing.BaseAggregate{ID: "user-1", Type: "User"},
}
user.RaiseEvent("UserCreated", map[string]interface{}{
    "name":  "John",
    "email": "john@example.com",
})

// Salvar eventos
store.Append(ctx, user.GetUncommittedEvents()...)

// Event Bus
bus := eventsourcing.NewEventBus()
bus.Subscribe("UserCreated", func(ctx context.Context, event eventsourcing.Event) error {
    fmt.Printf("User created: %v\n", event.Data)
    return nil
})
```

**Arquivos:**
- `eventsourcing/event.go` - Event sourcing completo

---

#### 7. CQRS Helpers

**Motivação:** Implementação do padrão Command Query Responsibility Segregation.

**Funcionalidades:**

- `CommandBus` - Despacho de comandos
- `QueryBus` - Despacho de queries
- `Mediator` - Centraliza command e query bus
- Middleware: Logging, Validation, Caching
- `ReadModelProjection` - Projeção de eventos em read models

**Exemplo:**

```go
// Definir comando
type CreateUserCommand struct {
    cqrs.BaseCommand
    Name  string
    Email string
}

func (c CreateUserCommand) CommandName() string { return "CreateUser" }

// Registrar handler
bus := cqrs.NewCommandBus()
cqrs.RegisterFunc(bus, func(ctx context.Context, cmd CreateUserCommand) error {
    // Criar usuário...
    return nil
})

// Middleware
bus.Use(cqrs.LoggingMiddleware(log.Printf))
bus.Use(cqrs.AutoValidationMiddleware())

// Executar comando
cqrs.Dispatch(ctx, bus, CreateUserCommand{Name: "John", Email: "john@example.com"})

// Definir query
type GetUserQuery struct {
    cqrs.BaseQuery
    ID string
}

func (q GetUserQuery) QueryName() string { return "GetUser" }

// Registrar handler de query
queryBus := cqrs.NewQueryBus()
cqrs.RegisterQueryFunc(queryBus, func(ctx context.Context, q GetUserQuery) (*User, error) {
    // Buscar usuário...
    return &User{}, nil
})

// Executar query
user, _ := cqrs.Ask[GetUserQuery, *User](ctx, queryBus, GetUserQuery{ID: "123"})

// Mediator (combina command + query)
mediator := cqrs.NewMediator()
cqrs.Send(ctx, mediator, CreateUserCommand{})
user, _ = cqrs.Request[GetUserQuery, *User](ctx, mediator, GetUserQuery{ID: "123"})
```

**Arquivos:**
- `cqrs/cqrs.go` - CQRS helpers completos

---

### Testes Adicionados

- Todos os arquivos compilam sem erros
- Testes existentes continuam passando

---

## [5.0.0] - 2026-03-04

### Adicionado - Versão 5.0 (Production-Ready Features)

#### 1. Cursor-Based Pagination

**Motivação:** Paginação eficiente e estável para grandes datasets, compatível com GraphQL Relay spec.

**Funcionalidades:**

- `Cursor` - Tipo opaco para cursores de paginação
- `CursorPage[T]` - Resultado de paginação com Items, HasNextPage, HasPreviousPage
- `CursorConfig` - Configuração com OrderBy, First/After, Last/Before
- `EncodeCursor()` / `DecodeCursor()` - Serialização segura de cursores
- Suporte a GraphQL Relay: `Connection[T]`, `Edge[T]`, `PageInfo`

**Exemplo:**

```go
page, _ := genus.Table[User](db).
    Where(UserFields.Active.Eq(true)).
    Paginate(query.CursorConfig{
        OrderBy:   "created_at",
        OrderDesc: true,
        First:     10,
    }).
    Fetch(ctx)

for _, user := range page.Items {
    fmt.Println(user.Name)
}

// Próxima página
if page.HasNextPage {
    nextPage, _ := genus.Table[User](db).
        Paginate(query.CursorConfig{
            OrderBy:   "created_at",
            OrderDesc: true,
            First:     10,
            After:     page.EndCursor,
        }).
        Fetch(ctx)
}

// Converter para formato GraphQL Relay
connection := page.ToConnection(func(u User) query.Cursor {
    return query.EncodeCursor("created_at", u.CreatedAt, u.ID)
})
```

**Arquivos:**
- `query/cursor.go` - Cursor pagination completo

---

#### 2. UPSERT / ON CONFLICT

**Motivação:** Operações de insert-or-update atômicas para PostgreSQL e MySQL.

**Funcionalidades:**

- `UpsertConfig` - Configuração de conflito e colunas a atualizar
- `Upsert()` / `UpsertWithConfig()` - Upsert de registro único
- `BatchUpsert()` / `BatchUpsertWithConfig()` - Upsert em lote
- Suporte a PostgreSQL (`ON CONFLICT DO UPDATE`) e MySQL (`ON DUPLICATE KEY UPDATE`)
- Opção `DoNothing` para ignorar conflitos
- `UpdateWhere` para updates condicionais

**Exemplo:**

```go
// Upsert simples (usa id como coluna de conflito)
user := &User{Email: "test@example.com", Name: "Test"}
db.DB().Upsert(ctx, user)

// Upsert com configuração
db.DB().UpsertWithConfig(ctx, user, core.UpsertConfig{
    ConflictColumns: []string{"email"},
    UpdateColumns:   []string{"name", "updated_at"},
})

// Batch upsert
users := []*User{
    {Email: "a@test.com", Name: "A"},
    {Email: "b@test.com", Name: "B"},
}
db.DB().BatchUpsert(ctx, users)

// Ignorar conflitos
db.DB().UpsertWithConfig(ctx, user, core.UpsertConfig{
    DoNothing: true,
})
```

**Arquivos:**
- `core/upsert.go` - Implementação completa de upsert

---

#### 3. Dry Run Mode / Query Preview

**Motivação:** Visualizar SQL gerado sem executar, útil para debug e logging.

**Funcionalidades:**

- `DryRunBuilder[T]` - Builder para preview de queries
- `DryRunResult` - Resultado com SQL, Args, FormattedSQL, Operation, Table
- `ToSQL()` - Retorna SQL e argumentos diretamente
- `Explain()` / `ExplainAnalyze()` - Preview e execução de EXPLAIN

**Exemplo:**

```go
// Preview de SELECT
result := genus.Table[User](db).
    Where(UserFields.Age.Gt(18)).
    OrderByDesc("created_at").
    Limit(10).
    DryRun().
    Select()

fmt.Println(result.SQL)
// SELECT * FROM "users" WHERE age > $1 ORDER BY created_at DESC LIMIT 10
fmt.Println(result.Args)
// [18]
fmt.Println(result.FormattedSQL)
// SELECT * FROM "users" WHERE age > 18 ORDER BY created_at DESC LIMIT 10

// Obter SQL diretamente
sql, args := genus.Table[User](db).
    Where(UserFields.Active.Eq(true)).
    ToSQL()

// Executar EXPLAIN
results, _ := genus.Table[User](db).
    Where(UserFields.Age.Gt(18)).
    Explain(ctx)
for _, r := range results {
    fmt.Println(r.Plan)
}
```

**Arquivos:**
- `query/dryrun.go` - Dry run e EXPLAIN

---

#### 4. Query Timeout Helpers

**Motivação:** Configurar timeouts de forma simples por query.

**Funcionalidades:**

- `WithTimeout(duration)` - Adiciona timeout a qualquer query
- Métodos `Find()`, `First()`, `Count()` com timeout
- `DefaultQueryTimeout` - Timeout padrão global

**Exemplo:**

```go
users, err := genus.Table[User](db).
    Where(UserFields.Active.Eq(true)).
    WithTimeout(5 * time.Second).
    Find(ctx)

// Timeout no count
count, err := genus.Table[User](db).
    WithTimeout(2 * time.Second).
    Count(ctx)
```

**Arquivos:**
- `query/timeout.go` - Timeout helpers

---

#### 5. JSON/JSONB Field Support

**Motivação:** Suporte nativo a campos JSON com queries tipadas.

**Funcionalidades:**

- `JSON[T]` - Tipo genérico para campos JSON com Scan/Value
- `JSONField` - Builder de queries para campos JSON
- `JSONPath` - Acesso a caminhos dentro do JSON
- Operadores PostgreSQL: `@>`, `<@`, `?`, `?|`, `?&`
- `JSONRaw` - JSON bruto como string

**Exemplo:**

```go
type User struct {
    ID       int64              `db:"id"`
    Metadata query.JSON[map[string]interface{}] `db:"metadata"`
}

// Criar JSON
user := &User{
    Metadata: query.NewJSON(map[string]interface{}{
        "preferences": map[string]interface{}{
            "theme": "dark",
        },
    }),
}

// Query em campo JSON
metaField := query.NewJSONField("metadata")

// Acessar caminho
users, _ := genus.Table[User](db).
    Where(metaField.Path("preferences", "theme").EqText("dark")).
    Find(ctx)

// JSON contains (PostgreSQL)
users, _ := genus.Table[User](db).
    WhereRaw(metaField.Contains(map[string]string{"role": "admin"})).
    Find(ctx)
```

**Arquivos:**
- `query/json.go` - JSON types e queries

---

#### 6. Full-Text Search

**Motivação:** Busca full-text nativa para PostgreSQL e MySQL.

**Funcionalidades:**

- `SimpleSearch()` - Busca simples em uma coluna
- `MultiColumnSearch()` - Busca em múltiplas colunas
- `WeightedSearch()` - Busca com pesos (PostgreSQL)
- `SimpleSearchMySQL()` / `BooleanSearchMySQL()` - MySQL MATCH AGAINST
- `LikeSearch()` / `ILikeSearch()` - Fallback com LIKE

**Exemplo:**

```go
// PostgreSQL full-text search
users, _ := genus.Table[User](db).
    WhereRaw(query.SimpleSearch("name", "john")).
    Find(ctx)

// Múltiplas colunas
users, _ := genus.Table[User](db).
    WhereRaw(query.MultiColumnSearch(
        []string{"name", "bio", "email"},
        "developer",
        "english",
    )).
    Find(ctx)

// MySQL MATCH AGAINST
users, _ := genus.Table[User](db).
    WhereRaw(query.SimpleSearchMySQL([]string{"name", "bio"}, "john")).
    Find(ctx)

// Busca booleana MySQL
users, _ := genus.Table[User](db).
    WhereRaw(query.BooleanSearchMySQL([]string{"name"}, "+john -doe")).
    Find(ctx)
```

**Arquivos:**
- `query/fulltext.go` - Full-text search

---

#### 7. Query Profiler / Slow Query Detection

**Motivação:** Monitorar performance de queries e detectar queries lentas.

**Funcionalidades:**

- `Profiler` - Coleta estatísticas de queries
- `ProfilerConfig` - Configuração (threshold, callbacks)
- `QueryStats` - Estatísticas por query (duration, rows, etc)
- `ProfilerSummary` - Resumo agregado
- `ProfiledExecutor` - Executor com profiling automático

**Exemplo:**

```go
// Criar profiler
config := core.DefaultProfilerConfig()
config.Enabled = true
config.SlowQueryThreshold = 100 * time.Millisecond
config.OnSlowQuery = func(stats core.QueryStats) {
    log.Printf("SLOW QUERY: %s (%v)", stats.SQL, stats.Duration)
}
profiler := core.NewProfiler(config)

// Usar executor com profiling
executor := core.NewProfiledExecutor(db, profiler)

// Ver estatísticas
summary := profiler.Summary()
fmt.Printf("Total: %d queries, Slow: %d, Avg: %v\n",
    summary.TotalQueries, summary.SlowQueries, summary.AverageDuration)

// Listar queries lentas
for _, q := range profiler.GetSlowQueries() {
    fmt.Printf("%v: %s\n", q.Duration, q.SQL)
}
```

**Arquivos:**
- `core/profiler.go` - Query profiler

---

#### 8. Automatic Audit Logging

**Motivação:** Rastrear todas as mudanças em dados para compliance e debugging.

**Funcionalidades:**

- `Auditor` - Gerenciador de auditoria
- `AuditEntry` - Registro de auditoria (table, action, old/new values, user, timestamp)
- `AuditConfig` - Configuração (tabelas excluídas, colunas excluídas, callbacks)
- Métodos `LogCreate()`, `LogUpdate()`, `LogDelete()`, `LogRead()`
- `CreateAuditTable()` - Cria tabela de auditoria
- `GetAuditHistory()` - Consulta histórico

**Exemplo:**

```go
// Configurar auditor
config := core.AuditConfig{
    Enabled:    true,
    AuditTable: "audit_logs",
    ExcludeColumns: []string{"password", "token"},
    GetCurrentUser: func(ctx context.Context) string {
        return ctx.Value("user_id").(string)
    },
}
auditor := core.NewAuditor(db, dialect, config)

// Criar tabela de auditoria
auditor.CreateAuditTable(ctx)

// Registrar operações
auditor.LogCreate(ctx, "users", user.ID, user)
auditor.LogUpdate(ctx, "users", user.ID, oldUser, newUser)
auditor.LogDelete(ctx, "users", user.ID, user)

// Consultar histórico
history, _ := auditor.GetAuditHistory(ctx, "users", userID)
```

**Arquivos:**
- `core/audit.go` - Audit logging

---

#### 9. Circuit Breaker / Connection Retry

**Motivação:** Resiliência em caso de falhas de conexão.

**Funcionalidades:**

- `CircuitBreaker` - Implementação do padrão circuit breaker
- Estados: `Closed`, `Open`, `HalfOpen`
- `CircuitBreakerConfig` - Configuração (thresholds, timeout)
- `CircuitBreakerExecutor` - Executor com circuit breaker
- `RetryConfig` - Configuração de retry com backoff exponencial
- `WithRetry[T]()` - Função genérica para retry

**Exemplo:**

```go
// Circuit breaker
cb := core.NewCircuitBreaker(core.CircuitBreakerConfig{
    FailureThreshold: 5,
    SuccessThreshold: 2,
    Timeout:          30 * time.Second,
    OnStateChange: func(from, to core.CircuitState) {
        log.Printf("Circuit breaker: %s -> %s", from, to)
    },
})

executor := core.NewCircuitBreakerExecutor(db, cb)

// Retry com backoff
result, err := core.WithRetry(ctx, core.DefaultRetryConfig(), func() (*User, error) {
    return genus.Table[User](db).First(ctx)
})
```

**Arquivos:**
- `core/circuit_breaker.go` - Circuit breaker e retry

---

#### 10. Multi-Tenancy Support

**Motivação:** Isolar dados por tenant em aplicações SaaS.

**Funcionalidades:**

- Estratégias: `TenantColumn`, `TenantSchema`, `TenantDatabase`
- `WithTenant(ctx, tenantID)` - Adiciona tenant ao contexto
- `TenantScope` - Escopo de tenant para queries
- `TenantMiddleware` - Middleware para aplicar tenant automaticamente
- `Tenant` - Mixin para models
- Row-Level Security (PostgreSQL): `CreateRLSPolicy()`, `SetTenantSession()`

**Exemplo:**

```go
// Adicionar tenant ao contexto
ctx = core.WithTenant(ctx, "acme-corp")

// Modelo com tenant
type User struct {
    core.Model
    core.Tenant  // Embedded - adiciona tenant_id
    Name string `db:"name"`
}

// Middleware
middleware := core.NewTenantMiddleware(core.TenantConfig{
    Strategy:   core.TenantColumn,
    ColumnName: "tenant_id",
}, dialect)

// Row-Level Security (PostgreSQL)
core.CreateRLSPolicy(ctx, executor, dialect, "users", core.DefaultRowLevelSecurityConfig())
core.SetTenantSession(ctx, executor, "acme-corp")
```

**Arquivos:**
- `core/tenant.go` - Multi-tenancy

---

#### 11. Real-Time Subscriptions (PostgreSQL LISTEN/NOTIFY)

**Motivação:** Notificações em tempo real de mudanças no banco de dados.

**Funcionalidades:**

- `SubscriptionManager` - Gerenciador de subscriptions
- `Subscribe()` - Inscrever-se para notificações
- `NotifyPayload` - Payload de notificação (table, action, old/new values)
- `ChangeStream` - Stream de mudanças com channel
- `CreateNotifyTrigger()` - Cria trigger automático
- `ReadYourWritesHelper` - Helper para consistência read-your-writes

**Exemplo:**

```go
// Criar gerenciador
manager := core.NewSubscriptionManager(db, dialect, core.DefaultSubscriptionConfig())

// Criar trigger para notificações automáticas
manager.CreateNotifyTrigger(ctx, "users")

// Inscrever-se para notificações
sub, _ := manager.Subscribe(ctx, "users", func(payload core.NotifyPayload) {
    fmt.Printf("User %s: ID=%v\n", payload.Action, payload.RecordID)
})
defer sub.Cancel()

// Usar ChangeStream
stream, _ := manager.NewChangeStream(ctx, "orders", 100)
defer stream.Close()

for change := range stream.Changes() {
    fmt.Printf("Order %s: %v\n", change.Action, change.NewValues)
}

// Watch com filtros
sub, _ = manager.Watch(ctx, "users", core.WatchConfig{
    Actions: []string{"INSERT", "UPDATE"},
    Filter: func(p core.NotifyPayload) bool {
        return p.NewValues["status"] == "active"
    },
}, handler)
```

**Arquivos:**
- `core/subscription.go` - Real-time subscriptions

---

### Testes Adicionados

- Todos os arquivos compilam sem erros
- Testes existentes continuam passando

---

## [4.0.0] - 2026-03-04

### Adicionado - Versão 4.0 (Enterprise Features)

#### 1. Query Caching

**Motivação:** Reduzir latência e carga no banco de dados cacheando resultados de queries frequentes.

**Funcionalidades:**

- Interface `Cache` para backends plugáveis
- `InMemoryCache` com LRU eviction e TTL
- `NoOpCache` para desabilitar cache sem modificar código
- `CachedBuilder[T]` - wrapper do builder com cache integrado
- Invalidação automática por tabela (prefixos)
- Geração de chaves consistente baseada em query + args
- Estatísticas de cache (hits, misses, evictions, hit rate)

**Exemplo:**

```go
// Criar cache
cache := cache.NewInMemoryCache(10000)  // 10k entries

// Usar cache em queries
users, _ := genus.Table[User](db).
    WithCache(cache, cache.DefaultCacheConfig()).
    Where(UserFields.Active.Eq(true)).
    Find(ctx)

// Invalidar cache de uma tabela
cache.DeleteByPrefix(ctx, "genus:users:")

// Verificar estatísticas
stats := cache.Stats()
fmt.Printf("Hit rate: %.2f%%\n", stats.HitRate * 100)
```

**Arquivos:**
- `cache/cache.go` - Interface Cache e InMemoryCache
- `query/cached_builder.go` - CachedBuilder com integração

---

#### 2. Polymorphic Relationships

**Motivação:** Suportar relacionamentos onde um model pode pertencer a múltiplos tipos de models (ex: Comments que podem pertencer a Posts ou Articles).

**Funcionalidades:**

- Novo tipo de relacionamento `Polymorphic`
- Tags `polymorphic`, `polymorphic_type`, `polymorphic_id`
- Suporte em Preload para eager loading polimórfico
- Defaults automáticos para nomes de colunas (`{base}_type`, `{base}_id`)
- Validação de relacionamentos polimórficos no registro

**Exemplo:**

```go
type Comment struct {
    core.Model
    Body              string `db:"body"`
    CommentableType   string `db:"commentable_type"`  // "Post" ou "Article"
    CommentableID     int64  `db:"commentable_id"`
}

type Post struct {
    core.Model
    Title    string    `db:"title"`
    Comments []Comment `db:"-" relation:"polymorphic,polymorphic=commentable"`
}

type Article struct {
    core.Model
    Content  string    `db:"content"`
    Comments []Comment `db:"-" relation:"polymorphic,polymorphic=commentable"`
}

// Query com eager loading
posts, _ := genus.Table[Post](db).
    Preload("Comments").  // Carrega comentários automaticamente
    Find(ctx)
```

**Arquivos:**
- `core/relationship.go` - RelationType `Polymorphic` e campos relacionados
- `query/preload.go` - `preloadPolymorphic()` para eager loading

---

#### 3. Type-Safe Subqueries

**Motivação:** Permitir subqueries complexas mantendo type-safety e compatibilidade com o query builder.

**Funcionalidades:**

- `SubqueryBuilder[T]` - Builder para subqueries
- Condições `IN (subquery)` e `NOT IN (subquery)`
- Condições `EXISTS (subquery)` e `NOT EXISTS (subquery)`
- `ScalarSubquery` para comparações escalares (ex: `price > (SELECT AVG(price)...)`)
- `CorrelatedSubquery` para subqueries correlacionadas
- `RawSubquery()` para SQL raw quando necessário
- Reescrita automática de placeholders para diferentes dialetos

**Exemplo:**

```go
// Subquery IN
subquery := genus.Table[Order](db).
    Where(OrderFields.Status.Eq("paid")).
    Subquery().
    Column("user_id").
    ToSubquery()

users, _ := genus.Table[User](db).
    WhereInSubquery("id", subquery).
    Find(ctx)
// SELECT * FROM users WHERE id IN (SELECT user_id FROM orders WHERE status = 'paid')

// Subquery EXISTS
postSubquery := genus.Table[Post](db).
    CorrelatedSubquery("id").
    Correlate("users.id = posts.user_id").
    ToSubquery()

usersWithPosts, _ := genus.Table[User](db).
    WhereExists(postSubquery).
    Find(ctx)
// SELECT * FROM users WHERE EXISTS (SELECT id FROM posts WHERE users.id = posts.user_id)

// Scalar subquery
avgPrice := genus.Table[Product](db).
    ScalarSubquery("AVG(price)").
    ToScalar()

expensiveProducts, _ := genus.Table[Product](db).
    Where(ProductFields.Price.Gt(avgPrice)).
    Find(ctx)
```

**Arquivos:**
- `query/subquery.go` - SubqueryBuilder, ScalarSubquery, CorrelatedSubquery
- `query/builder.go` - Integração no buildWhereClause

---

#### 4. Database Sharding

**Motivação:** Suportar particionamento horizontal de dados em múltiplas instâncias de banco.

**Funcionalidades:**

- `ShardManager` - Gerencia conexões para múltiplos shards
- Estratégias de sharding:
  - `ModuloStrategy` - Distribuição por módulo (padrão)
  - `ConsistentHashStrategy` - Consistent hashing para redistribuição mínima
- `ShardExecutor` - Implementa Executor com routing automático
- Context-based shard routing via `WithShardKey(ctx, key)`
- Suporte a operações em todos os shards (`ExecOnAllShards`, `QueryAllShards`)
- `MergeResults[T]()` para combinar resultados de múltiplos shards

**Exemplo:**

```go
// Configurar sharding
config := genus.ShardConfig{
    DSNs: []string{
        "postgres://host1:5432/db",
        "postgres://host2:5432/db",
        "postgres://host3:5432/db",
    },
    Strategy: sharding.NewConsistentHashStrategy(100),
}

db, _ := genus.OpenWithShards("postgres", config)

// Query em shard específico (baseado na shard key)
ctx := sharding.WithShardKey(ctx, sharding.Int64ShardKey(userID))
user, _ := genus.ShardedTable[User](db).First(ctx)

// Query em todos os shards
results := make([]sharding.ShardedResult[User], db.NumShards())
for i := 0; i < db.NumShards(); i++ {
    ctx := sharding.WithShardKey(ctx, sharding.Int64ShardKey(i))
    users, err := genus.ShardedTable[User](db).Find(ctx)
    results[i] = sharding.ShardedResult[User]{Results: users, Error: err}
}
allUsers, _ := sharding.MergeResults(results)
```

**Arquivos:**
- `sharding/sharding.go` - ShardManager, estratégias, tipos de chave
- `core/shard_executor.go` - ShardExecutor e ShardedDB
- `genus.go` - ShardConfig, OpenWithShards, ShardedTable

---

#### 5. OpenTelemetry Integration

**Motivação:** Integrar distributed tracing para observabilidade de queries SQL em sistemas distribuídos.

**Funcionalidades:**

- `TracedExecutor` - Wrapper de executor com spans automáticos
- Interface `Tracer` e `Span` abstratas (não requer importar OTel diretamente)
- `OTelAdapter` - Adapter para integrar com go.opentelemetry.io/otel
- `SimpleTracer` - Tracer simples com callbacks (para debugging)
- `NoopTracer` - Tracer que não faz nada (default)
- Semantic conventions para database spans (db.system, db.name, db.statement)
- `MetricsCollector` para coletar métricas de queries

**Exemplo:**

```go
// Com OpenTelemetry SDK
import "go.opentelemetry.io/otel"

otelTracer := otel.Tracer("genus")

adapter := tracing.NewOTelAdapter(tracing.OTelAdapterConfig{
    StartFunc: func(ctx context.Context, name string) (context.Context, interface{}) {
        return otelTracer.Start(ctx, name)
    },
    SetAttributeFunc: func(span interface{}, k string, v interface{}) {
        span.(trace.Span).SetAttributes(attribute.String(k, fmt.Sprintf("%v", v)))
    },
    RecordErrorFunc: func(span interface{}, err error) {
        span.(trace.Span).RecordError(err)
    },
    SetStatusFunc: func(span interface{}, ok bool, msg string) {
        if ok {
            span.(trace.Span).SetStatus(codes.Ok, "")
        } else {
            span.(trace.Span).SetStatus(codes.Error, msg)
        }
    },
    EndFunc: func(span interface{}) {
        span.(trace.Span).End()
    },
})

db, _ := genus.OpenWithTracing("postgres", dsn, genus.TracingConfig{
    Tracer:   adapter,
    DBSystem: "postgresql",
    DBName:   "mydb",
})

// Com SimpleTracer para debugging
simpleTracer := tracing.NewSimpleTracer(tracing.SimpleTracerConfig{
    OnStart: func(ctx context.Context, name string) context.Context {
        log.Printf("Starting: %s", name)
        return ctx
    },
    OnEnd: func(name string, durationMs int64, err error) {
        log.Printf("Finished: %s [%dms]", name, durationMs)
    },
})

db, _ := genus.OpenWithTracing("postgres", dsn, genus.TracingConfig{
    Tracer: simpleTracer,
})
```

**Arquivos:**
- `tracing/otel.go` - Interfaces, TracedExecutor, MetricsCollector
- `tracing/otel_adapter.go` - OTelAdapter, SimpleTracer
- `genus.go` - TracingConfig, OpenWithTracing

---

### Mudanças Técnicas

#### Arquitetura

- Novo pacote `cache` para caching de queries
- Novo pacote `sharding` para database sharding
- Novo pacote `tracing` para observabilidade
- Subquery conditions integradas ao query builder
- Polymorphic relationships no sistema de preload

#### Performance

- Query caching reduz chamadas ao banco para queries repetidas
- Sharding permite escalar horizontalmente
- Tracing overhead mínimo com NoopTracer padrão

#### Compatibilidade

- Todas as features são opt-in
- API existente permanece 100% compatível
- Sem breaking changes

---

### Testes Adicionados

- `cache/cache_test.go` - Testes para InMemoryCache, NoOpCache
- `sharding/sharding_test.go` - Testes para estratégias de sharding
- `tracing/otel_test.go` - Testes para tracers e adapters

---



### Adicionado - Versão 3.0 (Performance & Scaling Features)

#### 1. Auto-detect Dialect

**Motivação:** Simplificar a configuração removendo a necessidade de especificar o dialeto manualmente.

**Funcionalidades:**

- Função `DetectDialect(driver)` que detecta o dialeto baseado no nome do driver
- Função `DetectDialectFromDSN(dsn)` que detecta o dialeto baseado na string de conexão
- Função `DetectDriverFromDSN(dsn)` que detecta o driver baseado na string de conexão
- Suporte a padrões de URL (postgres://, mysql://, file:)
- Mapeamento automático:
  - `postgres`, `pgx` → PostgreSQL dialect
  - `mysql` → MySQL dialect
  - `sqlite3`, `sqlite` → SQLite dialect

**Exemplo:**

```go
// Antes (v2.0)
db, _ := sql.Open("postgres", dsn)
g := genus.New(db, postgres.New())

// Agora (v3.0) - detecção automática
g, _ := genus.Open("postgres", dsn) // Dialeto detectado automaticamente!

// Ou mesmo sem especificar o driver
driver := dialects.DetectDriverFromDSN("postgres://localhost/mydb")
dialect := dialects.DetectDialectFromDSN("postgres://localhost/mydb")
```

**Arquivos:**
- `dialects/detect.go` - Funções de detecção automática

---

#### 2. Connection Pooling Configuration

**Motivação:** Expor configurações de pool de conexões de forma organizada e com defaults sensatos.

**Funcionalidades:**

- Struct `PoolConfig` com configurações de pool
- `DefaultPoolConfig()` - Configuração padrão (25 open, 10 idle, 30min lifetime)
- `HighPerformancePoolConfig()` - Para alta carga (100 open, 50 idle, 1h lifetime)
- `MinimalPoolConfig()` - Para desenvolvimento (5 open, 2 idle, 15min lifetime)
- Métodos fluentes: `WithMaxOpenConns()`, `WithMaxIdleConns()`, etc.
- Função `OpenWithConfig()` que aplica configurações de pool automaticamente

**Exemplo:**

```go
// Configuração padrão
db, _ := genus.OpenWithConfig("postgres", dsn, core.DefaultPoolConfig())

// Alta performance
db, _ := genus.OpenWithConfig("mysql", dsn, core.HighPerformancePoolConfig())

// Customizado com fluent API
config := core.DefaultPoolConfig().
    WithMaxOpenConns(50).
    WithMaxIdleConns(25).
    WithConnMaxLifetime(time.Hour)
db, _ := genus.OpenWithConfig("postgres", dsn, config)
```

**Arquivos:**
- `core/pool.go` - Estruturas e funções de pool configuration
- `genus.go` - Função `OpenWithConfig()`

---

#### 3. Type-Safe Aggregations

**Motivação:** Permitir operações de agregação (COUNT, SUM, AVG, MAX, MIN) com type-safety e suporte a GROUP BY/HAVING.

**Funcionalidades:**

- `AggregateBuilder[T]` - Builder dedicado para agregações
- Funções de agregação: `CountAll()`, `Count()`, `Sum()`, `Avg()`, `Max()`, `Min()`
- Agrupamento: `GroupBy(columns...)`
- Filtragem de grupos: `Having(condition)`
- Ordenação e paginação
- `AggregateResult` com métodos tipados: `Int64()`, `Float64()`, `String()`, `Value()`

**Exemplo:**

```go
// Contagem simples
result, _ := genus.Table[Order](db).
    Where(OrderFields.Status.Eq("paid")).
    Aggregate().
    CountAll().
    One(ctx)
fmt.Println(result.Int64("count")) // 42

// Agregações múltiplas com GROUP BY
results, _ := genus.Table[Order](db).
    Aggregate().
    Sum("total").
    Avg("total").
    CountAll().
    GroupBy("user_id").
    Having(query.Condition{Field: "COUNT(*)", Operator: query.OpGt, Value: 5}).
    All(ctx)

for _, r := range results {
    fmt.Printf("User %s: sum=%f, avg=%f, count=%d\n",
        r.String("user_id"),
        r.Float64("sum_total"),
        r.Float64("avg_total"),
        r.Int64("count"))
}
```

**Arquivos:**
- `query/aggregation.go` - AggregateBuilder e AggregateResult

---

#### 4. Batch Operations

**Motivação:** Permitir inserção, atualização e deleção em lote com performance otimizada (1 query ao invés de N).

**Funcionalidades:**

- `BatchInsert(ctx, models)` - Insert múltiplo com uma única query
- `BatchInsertWithConfig(ctx, models, config)` - Insert com configuração personalizada
- `BatchUpdate(ctx, models)` - Update múltiplo em transação
- `BatchDelete(ctx, models)` - Delete múltiplo com `WHERE id IN (...)`
- `BatchDeleteByIDs(ctx, tableName, ids)` - Delete direto por IDs
- `BatchConfig` com `BatchSize` (default: 100) e `SkipHooks`
- Suporte a soft delete em batch operations
- Hooks opcionais (BeforeSave, AfterCreate, etc.)

**Exemplo:**

```go
// Batch Insert
users := []*User{
    {Name: "Alice"},
    {Name: "Bob"},
    {Name: "Charlie"},
}
err := db.DB().BatchInsert(ctx, users) // 1 query!
// INSERT INTO users (name) VALUES ('Alice'), ('Bob'), ('Charlie')

// Batch com configuração
config := core.BatchConfig{BatchSize: 50, SkipHooks: true}
err = db.DB().BatchInsertWithConfig(ctx, users, config)

// Batch Update (usa transação)
for _, u := range users {
    u.Name = u.Name + " Updated"
}
err = db.DB().BatchUpdate(ctx, users)

// Batch Delete
err = db.DB().BatchDelete(ctx, users)
// DELETE FROM users WHERE id IN (1, 2, 3)

// Delete por IDs
err = db.DB().BatchDeleteByIDs(ctx, "users", []int64{1, 2, 3})
```

**Arquivos:**
- `core/batch.go` - Operações em lote

---

#### 5. Read Replicas Support

**Motivação:** Suportar arquiteturas com read replicas para escalar leituras.

**Funcionalidades:**

- `MultiExecutor` - Implementa `Executor` com suporte a replicas
- Escrita sempre vai para o primary
- Leitura usa round-robin entre replicas
- `WithPrimary(ctx)` - Força leitura do primary (read-after-write consistency)
- `OpenWithReplicas()` - Cria conexão com replicas configuradas
- `NewWithExecutor()` - Cria DB com executor customizado
- Métodos de diagnóstico: `Stats()`, `AllStats()`, `Ping()`

**Exemplo:**

```go
// Configuração de replicas
config := genus.ReplicaConfig{
    PrimaryDSN: "postgres://user:pass@primary:5432/db",
    ReplicaDSNs: []string{
        "postgres://user:pass@replica1:5432/db",
        "postgres://user:pass@replica2:5432/db",
    },
    PoolConfig: &core.HighPerformancePoolConfig(),
}

db, _ := genus.OpenWithReplicas("postgres", config)

// Leituras vão automaticamente para replicas (round-robin)
users, _ := genus.Table[User](db).Find(ctx) // → replica

// Forçar leitura do primary (para consistência read-after-write)
users, _ := genus.Table[User](db).Find(core.WithPrimary(ctx)) // → primary

// Escritas sempre vão para o primary
db.DB().Create(ctx, &user) // → primary
db.DB().Update(ctx, &user) // → primary
db.DB().Delete(ctx, &user) // → primary
```

**Arquivos:**
- `core/context.go` - `WithPrimary()` e `UsePrimary()`
- `core/multi_executor.go` - `MultiExecutor` com round-robin
- `core/db.go` - `NewWithExecutor()` e `NewWithExecutorAndLogger()`
- `genus.go` - `ReplicaConfig` e `OpenWithReplicas()`

---

### Mudanças Técnicas

#### Arquitetura

- Novo sistema de detecção automática de dialetos
- Pool configuration como estrutura imutável com métodos fluentes
- AggregateBuilder separado do Builder principal (single responsibility)
- MultiExecutor implementa Executor interface (strategy pattern)
- Context-based routing para primary/replica selection

#### Performance

- Batch operations reduzem N queries para 1 (ou N/BatchSize)
- Read replicas distribuem carga de leitura
- Pool configuration permite tuning fino de conexões
- Round-robin atômico para distribuição de carga entre replicas

#### Compatibilidade

- Todas as features são opt-in
- API existente permanece 100% compatível
- Funções `Open()` e `New()` continuam funcionando normalmente
- Novos métodos adicionados sem breaking changes

---

### Testes Adicionados

- `dialects/detect_test.go` - Testes para detecção de dialeto
- `core/pool_test.go` - Testes para pool configuration
- `core/context_test.go` - Testes para WithPrimary
- `core/multi_executor_test.go` - Testes para MultiExecutor
- `core/batch_test.go` - Testes para batch config
- `query/aggregation_test.go` - Testes para AggregateResult

---

## [2.0.0] - 2026-01-06

### Adicionado - Versão 2.0 (Relational Features)

#### 1. Sistema de Relacionamentos (HasMany, BelongsTo, ManyToMany)

**Motivação:** Permitir definição de relacionamentos entre models usando tags, eliminando boilerplate e mantendo type-safety.

**Funcionalidades:**

- Relacionamento **HasMany**: Um para muitos (User has many Posts)
- Relacionamento **BelongsTo**: Pertence a (Post belongs to User)
- Relacionamento **ManyToMany**: Muitos para muitos via tabela de junção (Post has many Tags)
- Registro de relacionamentos via `genus.RegisterModels()`
- Tags em struct fields para definir relacionamentos
- Suporte a foreign keys customizadas
- Suporte a join tables para ManyToMany

**Exemplo:**

```go
type User struct {
    core.Model
    Posts []Post `db:"-" relation:"has_many,foreign_key=user_id"`
}

type Post struct {
    core.Model
    UserID int64 `db:"user_id"`
    User   *User `db:"-" relation:"belongs_to,foreign_key=user_id"`
    Tags   []Tag `db:"-" relation:"many_to_many,join_table=post_tags,foreign_key=post_id,association_foreign_key=tag_id"`
}

genus.RegisterModels(&User{}, &Post{}, &Tag{})
```

**Arquivos:**
- `core/relationship.go` - Sistema de registro e metadata de relacionamentos
- `genus.go` - Helper `RegisterModels()`

---

#### 2. Eager Loading (Preload)

**Motivação:** Resolver o problema N+1 queries, carregando relacionamentos de forma eficiente.

**Funcionalidades:**

- Método `.Preload(relation)` para eager loading
- Suporte a preload aninhado (`"Posts.Comments"`)
- Implementações otimizadas para cada tipo de relacionamento
- Query única por relacionamento (evita N+1)

**Exemplo:**

```go
// Sem Preload - N+1 queries
users, _ := genus.Table[User](db).Find(ctx)
for _, user := range users {
    posts, _ := genus.Table[Post](db).Where(PostFields.UserID.Eq(user.ID)).Find(ctx)
}

// Com Preload - Apenas 2 queries
users, _ := genus.Table[User](db).Preload("Posts").Find(ctx)
```

**Arquivos:**
- `query/preload.go` - Engine completo de eager loading
- `query/builder.go` - Método `Preload()` e integração

---

#### 3. JOINs Type-Safe

**Motivação:** Permitir queries complexas com JOINs mantendo type-safety através de generics.

**Funcionalidades:**

- Método genérico `Join[T](condition)` para INNER JOINs
- Método genérico `LeftJoin[T](condition)` para LEFT JOINs
- Método genérico `RightJoin[T](condition)` para RIGHT JOINs
- Helper `On(leftCol, rightCol)` para condições de join
- Suporte a múltiplos JOINs na mesma query

**Exemplo:**

```go
users, _ := genus.Table[User](db).
    Join[Post](query.On("users.id", "posts.user_id")).
    Where(PostFields.Title.Like("%Go%")).
    Find(ctx)
```

**Arquivos:**
- `query/join.go` - Estruturas e lógica de JOINs
- `query/builder.go` - Métodos `Join[T]`, `LeftJoin[T]`, `RightJoin[T]`

---

#### 4. Soft Deletes

**Motivação:** Permitir deleção suave com recuperação fácil, usando global scopes para filtrar automaticamente.

**Funcionalidades:**

- Interface `SoftDeletable` com `GetDeletedAt()`, `SetDeletedAt()`, `IsDeleted()`
- Struct base `SoftDeleteModel` com campo `DeletedAt`
- Global scope automático filtra soft-deleted por padrão
- Métodos `WithTrashed()` e `OnlyTrashed()` para controle
- Método `ForceDelete()` para deleção permanente
- Integração com hooks (BeforeDelete chama soft delete automaticamente)

**Exemplo:**

```go
type User struct {
    core.SoftDeleteModel
    Name string `db:"name"`
}

db.DB().Delete(ctx, user)  // Soft delete

genus.Table[User](db).Find(ctx)  // Exclui soft-deleted
genus.Table[User](db).WithTrashed().Find(ctx)  // Inclui soft-deleted

db.DB().ForceDelete(ctx, user)  // Delete permanente
```

**Arquivos:**
- `core/softdelete.go` - Interface e implementação
- `query/scope.go` - Sistema de global scopes
- `query/builder.go` - Métodos `WithTrashed()`, `OnlyTrashed()`
- `core/db.go` - Integração em `Delete()` e novo `ForceDelete()`

---

#### 5. Hooks Avançados

**Motivação:** Expandir sistema de hooks para cobrir todo o ciclo de vida de models.

**Funcionalidades:**

- **Novos hooks:**
  - `AfterCreater` - Após criar registro
  - `BeforeUpdater` - Antes de atualizar
  - `AfterUpdater` - Após atualizar
  - `BeforeDeleter` - Antes de deletar
  - `AfterDeleter` - Após deletar
  - `BeforeSaver` - Antes de Create ou Update
  - `AfterSaver` - Após Create ou Update
- Integração completa em operações CRUD
- Rollback automático em caso de erro em hooks
- Ordem de execução definida

**Exemplo:**

```go
func (u *User) BeforeCreate() error {
    // Validação
}

func (u *User) AfterCreate() error {
    // Auditoria
}

func (u *User) BeforeSave() error {
    // Normalização de dados
}
```

**Arquivos:**
- `core/hooks.go` - Novas interfaces de hooks
- `core/db.go` - Integração em `Create()`, `Update()`, `Delete()`
- `query/builder.go` - Hook `AfterFind()` corrigido

---

### Mudanças Técnicas

#### Arquitetura

- Novo sistema de registry global para relacionamentos
- Sistema de global scopes para filtros automáticos
- Suporte a reflection mínimo apenas para scanning e parsing de tags
- Estrutura de preload otimizada para evitar N+1

#### Performance

- Eager loading reduz queries de N+1 para 2-3
- JOINs nativos do SQL ao invés de múltiplas queries
- Preload agrupa registros por parent_id em memória
- Scanning otimizado com fieldPath para structs embedded

#### Compatibilidade

- Todas as features são opt-in
- Não há breaking changes na API existente
- Models sem relacionamentos continuam funcionando normalmente

---

### Exemplos e Documentação

#### Documentação Atualizada

- `README.md` - Seções completas para v2.0: Relacionamentos, Preload, JOINs, Soft Deletes, Hooks
- `README.md` - Tabela de comparação atualizada com features v2.0
- `README.md` - Roadmap atualizado mostrando v2.0 como implementado

---

## [1.0.1] - 2025-11-25

### Corrigido

#### 1. **Bug Crítico: Scanning com core.Model Embedded**

- **Problema:** Ao fazer scan de resultados SQL para structs com `core.Model` embedded, ocorria erro pois o código tentava escanear para o campo embedded inteiro ao invés dos campos individuais (ID, CreatedAt, UpdatedAt)
- **Solução:** Implementado sistema de `fieldPath` em `query/scanner.go` que navega corretamente através de structs embedded usando um caminho de índices
- **Impacto:** Agora todas as queries com modelos que usam `core.Model` funcionam corretamente
- **Arquivo:** `query/scanner.go`

#### 2. **Bug: AutoMigrate com SQLite (Sintaxe MySQL Incorreta)**

- **Problema:** `migrate.AutoMigrate()` gerava SQL com sintaxe `AUTO_INCREMENT` do MySQL, que SQLite não suporta
- **Solução:** Implementado detecção de dialeto baseada em `Placeholder()` e `QuoteIdentifier()`:
  - PostgreSQL: usa `SERIAL`
  - MySQL: usa `INTEGER AUTO_INCREMENT`
  - SQLite: usa `INTEGER` (auto-increment automático para INTEGER PRIMARY KEY)
- **Impacto:** AutoMigrate agora funciona corretamente em SQLite
- **Arquivo:** `migrate/auto.go`

#### 3. **Bug: createMigrationsTable Usando Tipos Genéricos**

- **Problema:** Tabela de migrations usava tipos genéricos (VARCHAR, TIMESTAMP) que podem não funcionar em todos os dialetos
- **Solução:** Implementado detecção de dialeto e uso de tipos específicos:
  - PostgreSQL: `BIGINT`, `VARCHAR(255)`, `TIMESTAMP`
  - MySQL: `BIGINT`, `VARCHAR(255)`, `DATETIME`
  - SQLite: `INTEGER`, `TEXT`, `DATETIME`
- **Impacto:** Sistema de migrations funciona corretamente em todos os dialetos suportados
- **Arquivo:** `migrate/migrate.go`

### Adicionado

#### 1. **core.ErrValidation**

- Adicionado erro `ErrValidation` que estava faltando e era usado nos exemplos
- **Arquivo:** `core/interfaces.go`

#### 2. **Float64Field**

- Adicionado tipo `Float64Field` para campos float64 não-nullable
- Já existia `OptionalFloat64Field` mas faltava a versão não-opcional
- Suporta todos os operadores: Eq, Ne, Gt, Gte, Lt, Lte, In, NotIn, Between, IsNull, IsNotNull
- **Arquivo:** `query/field.go`

#### 3. **Dependências SQL Drivers**

- Adicionadas dependências dos drivers SQL oficiais:
  - `github.com/lib/pq` (PostgreSQL)
  - `github.com/go-sql-driver/mysql` (MySQL)
  - `github.com/mattn/go-sqlite3` (SQLite)
- **Arquivo:** `go.mod`

### Corrigido - Exemplos

#### 1. **Atualizados Exemplos para API Correta**

- Corrigido uso de `genus.New()` para `genus.NewWithLogger()`
- Corrigido chamadas de CRUD: `g.Create()` → `g.DB().Create()`
- Corrigido Table builder: `g.Table[T]()` → `genus.Table[T](g)`
- **Arquivos:** `examples/optional/main.go`, `examples/migrations/main.go`, `examples/multi-database/main.go`

#### 2. **Corrigidos fmt.Println com Newlines Redundantes**

- Substituído `fmt.Println("text\n")` por `fmt.Println("text")` seguido de `fmt.Println()`
- Corrige warning do linter: "fmt.Println arg list ends with redundant newline"
- **Arquivos:** Todos os exemplos

## [1.0.0] - 2025-11-03

### Adicionado - Versão 1.x (Usabilidade - Performance e Composição)

#### 1. Tipos Opcionais Genéricos (`Optional[T]`)

**Motivação:** Resolver a dor da manipulação de `sql.Null*` e ponteiros em JSON com uma API limpa e unificada.

**Funcionalidades:**

- Tipo genérico `core.Optional[T]` para valores nullable
- Suporte completo a JSON marshaling/unmarshaling (serializa como `null` quando vazio)
- Implementa `sql.Scanner` e `driver.Valuer` para integração com database/sql
- API funcional: `Map`, `FlatMap`, `Filter`, `IfPresent`, `IfPresentOrElse`
- Funções helper: `Some()`, `None()`, `FromPtr()`
- Métodos de acesso: `Get()`, `GetOrDefault()`, `GetOrZero()`, `Ptr()`
- Conversões automáticas para tipos primitivos (string, int, int64, bool, float64)

**Exemplo:**

```go
type User struct {
    core.Model
    Name  string                `db:"name"`
    Email core.Optional[string] `db:"email"`  // Pode ser NULL
    Age   core.Optional[int]    `db:"age"`    // Pode ser NULL
}

// Criar valores
email := core.Some("user@example.com")
age := core.None[int]()

// Usar
if email.IsPresent() {
    fmt.Println(email.Get())
}
userAge := age.GetOrDefault(18)
```

**Arquivos:**

- `core/optional.go` - Implementação completa do tipo Optional[T]

**Supera:** Todos os ORMs Go existentes - primeira implementação completa de Optional[T] genérico para Go

---

#### 2. Campos Opcionais Tipados

**Motivação:** Permitir queries type-safe em campos nullable.

**Funcionalidades:**

- `OptionalStringField` - Campo string opcional
- `OptionalIntField` - Campo int opcional
- `OptionalInt64Field` - Campo int64 opcional
- `OptionalBoolField` - Campo bool opcional
- `OptionalFloat64Field` - Campo float64 opcional
- Todos os campos suportam operadores apropriados (`Eq`, `Ne`, `Gt`, `Like`, `IsNull`, `IsNotNull`, etc.)

**Exemplo:**

```go
var UserFields = struct {
    Name  query.StringField
    Email query.OptionalStringField  // Campo opcional
    Age   query.OptionalIntField     // Campo opcional
}{
    Name:  query.NewStringField("name"),
    Email: query.NewOptionalStringField("email"),
    Age:   query.NewOptionalIntField("age"),
}

// Query em campos opcionais
users, _ := genus.Table[User]().
    Where(UserFields.Email.IsNotNull()).
    Where(UserFields.Age.Gt(18)).
    Find(ctx)
```

**Arquivos:**

- `query/field.go` - Adicionados campos opcionais (linhas 362-794)

**Supera:** GORM, Squirrel - campos opcionais totalmente tipados

---

#### 3. Query Builder Imutável

**Motivação:** Permitir composição segura de queries dinâmicas sem efeitos colaterais.

**Funcionalidades:**

- Todos os métodos do Builder retornam uma nova instância
- Método `clone()` interno para cópia profunda do estado
- Thread-safe por design
- Permite reutilização de queries base sem mutação

**Exemplo:**

```go
// Base query não é modificada
baseQuery := genus.Table[User]().Where(UserFields.Active.Eq(true))

// Composição segura
adults := baseQuery.Where(UserFields.Age.Gte(18))
minors := baseQuery.Where(UserFields.Age.Lt(18))

// baseQuery permanece inalterada!
// Cada query é completamente independente
```

**Impacto:**

- Antes: `baseQuery.Where()` modificava o objeto original
- Depois: `baseQuery.Where()` retorna uma nova query, original intocado

**Arquivos:**

- `query/builder.go` - Adicionado método `clone()` e modificados todos os métodos de building (linhas 42-132)

**Supera:** Squirrel - composição type-safe e imutável

---

#### 4. CLI de Code Generation (`genus generate`)

**Motivação:** Eliminar boilerplate manual e garantir sincronização automática entre structs e campos tipados.

**Funcionalidades:**

- CLI completo com comandos `generate`, `version`, `help`
- Parser de AST Go para extrair structs e tags `db`
- Geração automática de arquivos `*_fields.gen.go`
- Detecção automática de tipos `Optional[T]`
- Mapeamento inteligente de tipos Go para tipos de campo query
- Flags: `-o` (output dir), `-p` (package name), `-h` (help)

**Uso:**

```bash
# Instalar
go install github.com/GabrielOnRails/genus/cmd/genus@latest

# Gerar campos
genus generate ./models

# Com flags
genus generate -o ./generated -p mypackage ./models
```

**Entrada (struct):**

```go
type User struct {
    core.Model
    Name  string                `db:"name"`
    Email core.Optional[string] `db:"email"`
    Age   core.Optional[int]    `db:"age"`
}
```

**Saída (gerada automaticamente):**

```go
// user_fields.gen.go
var UserFields = struct {
    ID    query.Int64Field
    Name  query.StringField
    Email query.OptionalStringField
    Age   query.OptionalIntField
}{
    ID:    query.NewInt64Field("id"),
    Name:  query.NewStringField("name"),
    Email: query.NewOptionalStringField("email"),
    Age:   query.NewOptionalIntField("age"),
}
```

**Arquivos:**

- `cmd/genus/main.go` - CLI principal com comandos
- `codegen/generator.go` - Lógica de geração (parser AST, extração de structs, mapeamento de tipos)
- `codegen/template.go` - Template de código gerado

**Supera:** GORM - remove dependência excessiva de runtime reflection ao gerar metadados de coluna em compile-time

---

### Mudanças Técnicas

#### Performance

- **Zero reflection em queries:** Campos tipados gerados eliminam necessidade de reflection para descobrir metadados de coluna
- **Builder imutável:** Clone otimizado com cópia profunda apenas de slices necessários
- **Optional[T]:** Implementação eficiente com conversões diretas para tipos primitivos

#### Arquitetura

- Novo pacote `codegen` para geração de código
- Novo pacote `cmd/genus` para CLI
- Expansão de `core` com tipo `Optional[T]`
- Expansão de `query` com campos opcionais

#### Compatibilidade

- **Breaking change:** Query builder agora é imutável
  - Migração: Nenhuma mudança necessária no código do usuário (API permanece a mesma)
  - Impacto: Queries agora são thread-safe e podem ser reutilizadas

---

### Exemplos e Documentação

#### Novos Exemplos

- `examples/optional/main.go` - Demonstração completa de Optional[T]
- `examples/codegen/models/user.go` - Modelos para code generation
- `examples/codegen/README.md` - Tutorial de code generation

#### Documentação Atualizada

- `README.md` - Adicionadas seções sobre Optional[T], Code Generation e Query Builder Imutável
- `README.md` - Tabela de comparação expandida (GORM, Ent, sqlboiler, Squirrel)
- `examples/codegen/README.md` - Guia completo de uso do CLI

---

### Comparação de Performance (vs Competidores)

| Métrica | GORM | sqlboiler | Squirrel | **Genus 1.x** |
|---------|------|-----------|----------|---------------|
| Reflection em queries | Alto | Zero | N/A | Zero (após codegen) |
| Type-safety | Baixo | Alto | Baixo | Alto |
| Imutabilidade | Não | Não | Não | Sim |
| Tipos opcionais | `sql.Null*` | `null.*` | Manual | `Optional[T]` |
| Code generation | Não | Sim (schemas) | Não | Sim (fields) |

---

## [0.1.0] - 2025-11-01

### Adicionado - Versão Inicial

#### Core Features

- Query builder genérico com suporte a Go Generics
- Campos tipados (StringField, IntField, Int64Field, BoolField)
- Operadores type-safe (Eq, Ne, Gt, Like, etc.)
- Suporte a condições complexas (AND/OR)
- CRUD operations (Create, Update, Delete)
- SQL logging automático com performance monitoring
- Suporte a PostgreSQL (dialect)
- Transaction support
- Hook system (BeforeCreate, AfterFind)
- Context-aware operations
- Direct slice returns (`[]T`)

#### Packages

- `core` - Tipos base, DB, Logger, Interfaces
- `query` - Query builder, Fields, Conditions
- `dialects/postgres` - PostgreSQL dialect

#### Examples

- `examples/basic` - Exemplo completo de todas as features
- `examples/logging` - Configuração de logging customizado
- `examples/testing` - Padrões de teste

---

## Próximas Versões

### [5.0.0] - Planejado

#### Performance & Developer Experience

- Cursor-based pagination (mais eficiente que OFFSET para grandes datasets)
- UPSERT/ON CONFLICT support (INSERT ... ON CONFLICT DO UPDATE)
- Query profiling / slow query detection (detecção automática de queries lentas)
- Dry run mode (visualizar SQL sem executar)
- Automatic query optimization

#### Data Types

- JSON/JSONB field support com queries type-safe
- Full-text search (PostgreSQL tsvector/tsquery, MySQL FULLTEXT)

#### Security & Compliance

- Automatic audit logging (tracking de mudanças: quem, quando, o quê)
- Row-level security para multi-tenant apps

#### Resilience

- Connection retry com circuit breaker e exponential backoff
- Query timeout helpers por operação

#### Integration

- GraphQL integration
- Schema diff and migration generation
- Multi-tenancy support
- Real-time subscriptions (PostgreSQL LISTEN/NOTIFY)
- Read-your-writes consistency helpers

---

[4.0.0]: https://github.com/GabrielOnRails/genus/releases/tag/v4.0.0
[3.0.0]: https://github.com/GabrielOnRails/genus/releases/tag/v3.0.0
[2.0.0]: https://github.com/GabrielOnRails/genus/releases/tag/v2.0.0
[1.0.0]: https://github.com/GabrielOnRails/genus/releases/tag/v1.0.0
[0.1.0]: https://github.com/GabrielOnRails/genus/releases/tag/v0.1.0
