# Changelog

Todas as mudanças notáveis neste projeto serão documentadas neste arquivo.

O formato é baseado em [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
e este projeto adere ao [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

## [0.1.0] - 2024-01-XX

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

### [3.0.0] - Planejado

#### Advanced Features

- Query caching
- Connection pooling configuration
- Relacionamentos polimórficos
- Agregações type-safe (Count, Sum, Avg, Max, Min, GroupBy)
- Subqueries type-safe
- Raw SQL builder com type-safety

---

[2.0.0]: https://github.com/GabrielOnRails/genus/releases/tag/v2.0.0
[1.0.0]: https://github.com/GabrielOnRails/genus/releases/tag/v1.0.0
[0.1.0]: https://github.com/GabrielOnRails/genus/releases/tag/v0.1.0
