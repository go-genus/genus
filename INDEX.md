# Genus - Índice de Documentação

Bem-vindo ao **Genus**, o ORM type-safe para Go que usa Generics extensivamente!

## 📚 Começar Aqui

Se você é novo no Genus, leia nesta ordem:

1. **[README.md](./README.md)** - Visão geral e quick start
2. **[GENERICS_EXPLAINED.md](./GENERICS_EXPLAINED.md)** - Como funcionam os generics no Genus (didático)
3. **[USAGE.md](./USAGE.md)** - Guia completo de uso
4. **[ARCHITECTURE.md](./ARCHITECTURE.md)** - Arquitetura técnica detalhada

## 📖 Documentação

### Visão Geral

- **[README.md](./README.md)** - Introdução, instalação, características principais
  - O que é o Genus?
  - Por que usar Genus?
  - Comparação com outros ORMs
  - Quick start

### Tutoriais

- **[GENERICS_EXPLAINED.md](./GENERICS_EXPLAINED.md)** - Tutorial didático sobre generics
  - O problema dos ORMs tradicionais
  - Como Genus resolve com generics
  - Exemplos práticos
  - Comparação lado a lado
  - **Recomendado para iniciantes em Go Generics!**

- **[USAGE.md](./USAGE.md)** - Guia completo de uso
  - Instalação e configuração
  - Definindo modelos
  - Criando campos tipados
  - Operações CRUD
  - Queries type-safe
  - Transações
  - Exemplos avançados
  - Best practices

### Arquitetura

- **[ARCHITECTURE.md](./ARCHITECTURE.md)** - Documentação técnica
  - Problema que resolvemos
  - Design com generics
  - Mecanismo interno
  - Trade-offs
  - Onde ainda usamos reflection
  - Comparação com GORM

### Contribuindo

- **[CONTRIBUTING.md](./CONTRIBUTING.md)** - Como contribuir
  - Reportar bugs
  - Sugerir features
  - Pull requests
  - Diretrizes de código
  - Roadmap
  - Setup do ambiente

### Legal

- **[LICENSE](./LICENSE)** - Licença MIT

## 🚀 Exemplos

### Exemplos de Código

- **[examples/basic/main.go](./examples/basic/main.go)** - Exemplo completo
  - Demonstra TODOS os recursos
  - Find, Where, Create, Update, Delete
  - Queries complexas (AND/OR)
  - Operadores type-safe
  - 12+ exemplos diferentes

- **[examples/basic/schema.sql](./examples/basic/schema.sql)** - Schema SQL
  - Schema PostgreSQL para testar
  - Dados de exemplo

- **[examples/testing/repository_test.go](./examples/testing/repository_test.go)** - Testes
  - Exemplo de repository pattern
  - Como testar código com Genus
  - Exemplo de transações em testes

## 🏗️ Estrutura do Código

### Pacotes Principais

- **[genus.go](./genus.go)** - Interface pública
  - `Open()` - Conectar ao banco
  - `Table[T]()` - Criar query builder

- **[core/](./core/)** - Core do ORM
  - `model.go` - Model base, hooks
  - `db.go` - DB, CRUD operations, transações
  - `interfaces.go` - Interfaces principais

- **[query/](./query/)** - Query builder
  - `field.go` - Campos tipados (StringField, IntField, etc)
  - `builder.go` - Query builder genérico
  - `condition.go` - Condições WHERE
  - `scanner.go` - Scanner de structs

- **[dialects/](./dialects/)** - Dialetos de banco
  - `postgres/postgres.go` - PostgreSQL

## 📊 Quick Reference

### Criar Modelo

```go
type User struct {
    core.Model
    Name  string `db:"name"`
    Email string `db:"email"`
    Age   int    `db:"age"`
}
```

### Criar Campos Tipados

```go
var UserFields = struct {
    Name  query.StringField
    Email query.StringField
    Age   query.IntField
}{
    Name:  query.NewStringField("name"),
    Email: query.NewStringField("email"),
    Age:   query.NewIntField("age"),
}
```

### Query Type-Safe

```go
users, err := genus.Table[User](db).
    Where(UserFields.Age.Gt(18)).
    Where(UserFields.IsActive.Eq(true)).
    OrderByDesc("created_at").
    Limit(10).
    Find(ctx)
```

## 🎯 Para Diferentes Perfis

### Iniciante em Go

1. Leia [GENERICS_EXPLAINED.md](./GENERICS_EXPLAINED.md) primeiro
2. Depois [USAGE.md](./USAGE.md)
3. Execute [examples/basic/main.go](./examples/basic/main.go)

### Desenvolvedor Experiente

1. Leia [README.md](./README.md)
2. Vá direto para [ARCHITECTURE.md](./ARCHITECTURE.md)
3. Explore o código em [core/](./core/) e [query/](./query/)

### Quer Contribuir

1. Leia [CONTRIBUTING.md](./CONTRIBUTING.md)
2. Veja o [Roadmap](./CONTRIBUTING.md#7-roadmap)
3. Escolha uma issue `good first issue`

### Migrar de Outro ORM

1. Leia [comparação no README](./README.md#comparação-com-outros-orms)
2. Veja [exemplos práticos](./examples/basic/main.go)
3. Consulte [USAGE.md](./USAGE.md) para detalhes

## ❓ FAQ

### Por que Genus ao invés de GORM?

**Type-safety.** Genus detecta erros em compile-time, GORM em runtime.

Veja: [README.md - Comparação](./README.md#comparação-com-outros-orms)

### Por que Genus ao invés de Ent?

**Simplicidade.** Genus não requer code generation (por enquanto), Ent sim.

Ent é excelente mas mais complexo. Genus foca em simplicidade.

### Preciso definir campos manualmente?

Não! Desde a versão 1.x, o Genus possui CLI de code generation.

Use: `genus generate ./models` para gerar campos automaticamente.

Veja: [README.md - Code Generation CLI](./README.md#4-code-generation-cli)

### Funciona com MySQL/SQLite?

Sim! Genus suporta PostgreSQL, MySQL e SQLite desde a versão 1.x.

Veja: [README.md - Suporte Multi-Database](./README.md#1-suporte-multi-database)

### Tem suporte a relações?

Sim! Desde a versão 2.0, Genus suporta HasMany, BelongsTo e ManyToMany.

Veja: [README.md - Relacionamentos](./README.md#6-relacionamentos-hasmany-belongsto-manytomany)

## 🔗 Links Úteis

- **GitHub**: (adicione o link quando publicar)
- **Issues**: (adicione o link)
- **Discussões**: (adicione o link)

## 📈 Roadmap

Veja o roadmap completo em [CONTRIBUTING.md](./CONTRIBUTING.md#7-roadmap) e [README.md - Roadmap](./README.md#roadmap).

**Versão 2.0 ✅ Implementado:**
- [x] MySQL e SQLite dialects
- [x] Code generation para campos
- [x] Relações (HasMany, BelongsTo, ManyToMany)
- [x] Migrations (AutoMigrate + Manual)
- [x] Eager loading / Preload
- [x] JOINs type-safe
- [x] Soft deletes
- [x] Hooks avançados

**Próximos passos (v3.0):**
- [ ] Query caching
- [ ] Connection pooling configuration
- [ ] Relacionamentos polimórficos
- [ ] Agregações type-safe

## 🙏 Contribuidores

(Adicione aqui quando tiver contribuidores)

## 📝 Changelog

Veja o changelog completo em [CHANGELOG.md](./CHANGELOG.md).

---

**Genus** - Type-Safe ORM para Go 🚀

Feito com ❤️ usando Go Generics
