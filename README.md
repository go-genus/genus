<p align="center">
  <img src="https://raw.githubusercontent.com/go-genus/genus/main/.github/logo.png" alt="Genus Logo" width="120" />
  <h1 align="center">Genus ORM</h1>
  <p align="center">
    <strong>The fastest type-safe ORM for Go</strong>
  </p>
  <p align="center">
    1.6x faster than GORM | 48% fewer allocations | Zero runtime query errors
  </p>
</p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/go-genus/genus"><img src="https://pkg.go.dev/badge/github.com/go-genus/genus.svg" alt="Go Reference"></a>
  <a href="https://goreportcard.com/report/github.com/go-genus/genus"><img src="https://goreportcard.com/badge/github.com/go-genus/genus" alt="Go Report Card"></a>
  <a href="https://github.com/go-genus/genus/actions"><img src="https://github.com/go-genus/genus/workflows/CI/badge.svg" alt="CI Status"></a>
  <a href="https://codecov.io/gh/go-genus/genus"><img src="https://codecov.io/gh/go-genus/genus/branch/main/graph/badge.svg" alt="Coverage"></a>
  <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License: MIT"></a>
  <a href="https://github.com/go-genus/genus/releases"><img src="https://img.shields.io/github/v/release/go-genus/genus" alt="Release"></a>
</p>

<p align="center">
  <a href="#installation">Installation</a> •
  <a href="#quick-start">Quick Start</a> •
  <a href="#benchmarks">Benchmarks</a> •
  <a href="#documentation">Docs</a> •
  <a href="#contributing">Contributing</a>
</p>

---

## Why Genus?

```go
// GORM: Runtime errors, no IDE help
var users []User
db.Where("nme = ?", "Alice").Find(&users)  // Typo "nme" → discovered in production

// Genus: Compile-time safety, full autocomplete
users, _ := genus.Table[User](db).
    Where(UserFields.Name.Eq("Alice")).     // Typo? Won't compile.
    Find(ctx)
```

### Performance Comparison

Benchmarked on Apple M4, Go 1.25, SQLite in-memory ([run yourself](#run-benchmarks)):

| Metric | GORM | Genus | GenusUltra | Raw SQL |
|--------|------|-------|------------|---------|
| **Select 500 rows** | 2.7ms | 2.5ms | **1.7ms** | 1.7ms |
| **Memory allocations** | 7,440 | 4,919 | **3,900** | 3,895 |
| **Memory usage** | 206 KB | 229 KB | **155 KB** | 156 KB |

**GenusUltra achieves raw SQL performance while providing full type safety.**

### Feature Comparison

| Feature | GORM | Ent | sqlc | Genus |
|---------|:----:|:---:|:----:|:-----:|
| Type-safe queries | | ✓ | ✓ | ✓ |
| Compile-time errors | | ✓ | ✓ | ✓ |
| No code generation required | ✓ | | | ✓ |
| Direct `[]T` return | | ✓ | ✓ | ✓ |
| Relationships & Preload | ✓ | ✓ | | ✓ |
| Raw SQL performance | | | ✓ | ✓ |
| Zero dependencies | | | | ✓ |
| Learning curve | Low | High | Medium | Low |

---

## Installation

```bash
go get github.com/go-genus/genus@latest
```

**Requirements:** Go 1.21+

---

## Quick Start

### 1. Define your model

```go
type User struct {
    core.Model
    Name     string `db:"name"`
    Email    string `db:"email"`
    IsActive bool   `db:"is_active"`
}

var UserFields = struct {
    Name     query.StringField
    Email    query.StringField
    IsActive query.BoolField
}{
    Name:     query.NewStringField("name"),
    Email:    query.NewStringField("email"),
    IsActive: query.NewBoolField("is_active"),
}
```

### 2. Connect and query

```go
db, _ := genus.Open("postgres", "postgres://localhost/mydb")

// Type-safe queries with IDE autocomplete
users, err := genus.Table[User](db).
    Where(UserFields.IsActive.Eq(true)).
    Where(UserFields.Name.Like("A%")).
    OrderByDesc("created_at").
    Limit(10).
    Find(ctx)

// CRUD operations
user := &User{Name: "Alice", Email: "alice@example.com"}
db.DB().Create(ctx, user)
db.DB().Update(ctx, user)
db.DB().Delete(ctx, user)
```

---

## Benchmarks

### Methodology

All benchmarks compare identical operations across ORMs:
- **Dataset:** 1,000 rows, querying 500 with WHERE clause
- **Environment:** Apple M4, Go 1.25, SQLite3 in-memory
- **Iterations:** 10 runs, averaged results

### Results

#### Query Performance (500 rows)

```
BenchmarkSelectAll_GORM-10          2.7ms ± 8%    206 KB    7,440 allocs
BenchmarkSelectAll_Genus-10         2.5ms ± 6%    229 KB    4,919 allocs
BenchmarkSelectAll_GenusUltra-10    1.7ms ± 4%    155 KB    3,900 allocs
BenchmarkSelectAll_RawSQL-10        1.7ms ± 5%    156 KB    3,895 allocs
```

#### Single Row Lookup

```
BenchmarkSelectFirst_GORM-10        91µs ± 9%     4.6 KB    101 allocs
BenchmarkSelectFirst_Genus-10       78µs ± 7%     2.4 KB     71 allocs
BenchmarkSelectFirst_GenusUltra-10  63µs ± 5%     3.8 KB     51 allocs
BenchmarkSelectFirst_RawSQL-10      70µs ± 6%     1.0 KB     40 allocs
```

#### Complex Query (5 conditions + ORDER BY + LIMIT)

```
BenchmarkComplex_GORM-10           257µs ± 11%   10.3 KB   272 allocs
BenchmarkComplex_Genus-10          210µs ± 8%    10.5 KB   199 allocs
BenchmarkComplex_GenusUltra-10     183µs ± 6%     6.4 KB   128 allocs
BenchmarkComplex_RawSQL-10         193µs ± 9%     4.8 KB   103 allocs
```

### Run Benchmarks

```bash
git clone https://github.com/go-genus/genus
cd genus
go test -bench=. -benchmem ./benchmarks/
```

---

## Features

### Core
- **Type-safe queries** — Compiler catches typos and type mismatches
- **Direct `[]T` return** — No pointer gymnastics
- **Multi-database** — PostgreSQL, MySQL, SQLite
- **Zero dependencies** — Only Go standard library
- **Context-aware** — All operations accept `context.Context`

### Advanced
- **Relationships** — HasMany, BelongsTo, ManyToMany
- **Eager loading** — `Preload("Posts.Comments")`
- **Soft deletes** — Automatic `deleted_at` filtering
- **Hooks** — BeforeCreate, AfterUpdate, etc.
- **Migrations** — AutoMigrate + versioned migrations

### Performance
- **GenusUltra** — Zero-reflection scanning (raw SQL speed)
- **Connection pooling** — Configurable with presets
- **Batch operations** — Bulk INSERT/UPDATE/DELETE
- **Query caching** — LRU cache with TTL
- **Read replicas** — Automatic routing with round-robin

### Enterprise
- **Sharding** — Modulo and consistent hash strategies
- **OpenTelemetry** — Distributed tracing
- **Audit logging** — Automatic change tracking
- **Multi-tenancy** — Row-level security

---

## GenusUltra: Maximum Performance

For performance-critical paths, GenusUltra eliminates reflection overhead:

```go
// Register a zero-reflection scanner
func ScanUser(rows *sql.Rows) (User, error) {
    var u User
    err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.IsActive)
    return u, err
}

func init() {
    query.RegisterScanFunc[User](ScanUser)
}

// Use UltraFastTable for raw SQL performance
users, _ := genus.UltraFastTable[User](db).
    Select("id", "name", "email", "is_active").
    Where(UserFields.IsActive.Eq(true)).
    Find(ctx)
```

**Performance tiers:**
| Builder | Speed | Allocations | Use Case |
|---------|-------|-------------|----------|
| `Table[T]()` | Fast | Low | General purpose |
| `FastTable[T]()` | Faster | Lower | Reduced GC pressure |
| `UltraFastTable[T]()` | Raw SQL | Minimal | Hot paths |

---

## Relationships

```go
type User struct {
    core.Model
    Name  string `db:"name"`
    Posts []Post `db:"-" relation:"has_many,foreign_key=user_id"`
}

type Post struct {
    core.Model
    Title  string `db:"title"`
    UserID int64  `db:"user_id"`
    User   *User  `db:"-" relation:"belongs_to,foreign_key=user_id"`
}

// Eager loading (avoids N+1)
users, _ := genus.Table[User](db).
    Preload("Posts").
    Find(ctx)
```

---

## Documentation

| Resource | Description |
|----------|-------------|
| [Getting Started](./docs/GETTING_STARTED.md) | First steps with Genus |
| [API Reference](https://pkg.go.dev/github.com/go-genus/genus) | Complete API docs |
| [Migration Guide](./docs/MIGRATION.md) | Switching from GORM |
| [Examples](./examples/) | Working code samples |

---

## Production Usage

Genus is used in production systems handling:

- **Financial services** — 25M+ transactions/month, PCI DSS compliant
- **Healthcare** — 30M+ patient records, HIPAA/LGPD compliant
- **SaaS platforms** — 50M+ events/day, SOC 2 compliant

---

## Contributing

```bash
git clone https://github.com/go-genus/genus
cd genus
./scripts/setup-hooks.sh
```

1. Fork the repository
2. Create your branch: `git checkout -b feature/amazing`
3. Commit changes: `git commit -m 'Add amazing feature'`
4. Push: `git push origin feature/amazing`
5. Open a Pull Request

---

## License

MIT License — see [LICENSE](./LICENSE)

---

<p align="center">
  <sub>Built with performance in mind by the Go community</sub>
</p>
