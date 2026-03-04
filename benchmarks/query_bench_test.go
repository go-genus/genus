package benchmarks

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/go-genus/genus"
	"github.com/go-genus/genus/core"
	"github.com/go-genus/genus/dialects"
	"github.com/go-genus/genus/query"

	_ "github.com/mattn/go-sqlite3"
)

// User model para benchmarks
type User struct {
	core.Model
	Name     string `db:"name"`
	Email    string `db:"email"`
	Age      int    `db:"age"`
	IsActive bool   `db:"is_active"`
	Score    int    `db:"score"`
}

func (u User) TableName() string {
	return "users"
}

// UserFields campos tipados
var UserFields = struct {
	ID       query.IntField
	Name     query.StringField
	Email    query.StringField
	Age      query.IntField
	IsActive query.BoolField
	Score    query.IntField
}{
	ID:       query.NewIntField("id"),
	Name:     query.NewStringField("name"),
	Email:    query.NewStringField("email"),
	Age:      query.NewIntField("age"),
	IsActive: query.NewBoolField("is_active"),
	Score:    query.NewIntField("score"),
}

// setupBenchDB cria banco SQLite em memória para benchmarks (silent logger)
func setupBenchDB(b *testing.B) (*genus.Genus, *sql.DB, func()) {
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		b.Fatalf("failed to open db: %v", err)
	}

	// Cria tabela
	_, err = sqlDB.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			email TEXT NOT NULL,
			age INTEGER NOT NULL,
			is_active BOOLEAN NOT NULL DEFAULT 1,
			score INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		b.Fatalf("failed to create table: %v", err)
	}

	// Cria índices
	sqlDB.Exec("CREATE INDEX idx_users_email ON users(email)")
	sqlDB.Exec("CREATE INDEX idx_users_age ON users(age)")
	sqlDB.Exec("CREATE INDEX idx_users_is_active ON users(is_active)")

	// Insere dados de teste
	stmt, _ := sqlDB.Prepare("INSERT INTO users (name, email, age, is_active, score) VALUES (?, ?, ?, ?, ?)")
	for i := 0; i < 1000; i++ {
		stmt.Exec(
			fmt.Sprintf("User %d", i),
			fmt.Sprintf("user%d@example.com", i),
			20+(i%50),
			i%2 == 0,
			i*10,
		)
	}
	stmt.Close()

	// Usa NoOpLogger para silenciar logs durante benchmarks
	db := genus.NewWithLogger(sqlDB, dialects.DetectDialect("sqlite3"), &core.NoOpLogger{})

	return db, sqlDB, func() {
		sqlDB.Close()
	}
}

// ============================================================================
// Benchmarks: Query Builder
// ============================================================================

// BenchmarkQueryBuilder_Simple query simples com 1 condição
func BenchmarkQueryBuilder_Simple(b *testing.B) {
	db, _, cleanup := setupBenchDB(b)
	defer cleanup()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = genus.Table[User](db).
			Where(UserFields.IsActive.Eq(true)).
			Find(ctx)
	}
}

// BenchmarkQueryBuilder_Complex query complexa com 5 condições
func BenchmarkQueryBuilder_Complex(b *testing.B) {
	db, _, cleanup := setupBenchDB(b)
	defer cleanup()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = genus.Table[User](db).
			Where(UserFields.IsActive.Eq(true)).
			Where(UserFields.Age.Gte(25)).
			Where(UserFields.Age.Lte(40)).
			Where(UserFields.Score.Gt(100)).
			Where(UserFields.Name.Like("%User%")).
			OrderByAsc("score").
			Limit(10).
			Find(ctx)
	}
}

// BenchmarkQueryBuilder_First busca primeiro resultado
func BenchmarkQueryBuilder_First(b *testing.B) {
	db, _, cleanup := setupBenchDB(b)
	defer cleanup()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = genus.Table[User](db).
			Where(UserFields.Email.Eq("user500@example.com")).
			First(ctx)
	}
}

// BenchmarkQueryBuilder_Count contagem
func BenchmarkQueryBuilder_Count(b *testing.B) {
	db, _, cleanup := setupBenchDB(b)
	defer cleanup()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = genus.Table[User](db).
			Where(UserFields.IsActive.Eq(true)).
			Count(ctx)
	}
}

// ============================================================================
// Benchmarks: Raw SQL (baseline)
// ============================================================================

// BenchmarkRawSQL_Simple baseline para comparação
func BenchmarkRawSQL_Simple(b *testing.B) {
	_, sqlDB, cleanup := setupBenchDB(b)
	defer cleanup()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rows, err := sqlDB.QueryContext(ctx, "SELECT id, name, email, age, is_active, score FROM users WHERE is_active = ?", true)
		if err != nil {
			b.Fatal(err)
		}
		var users []User
		for rows.Next() {
			var u User
			rows.Scan(&u.ID, &u.Name, &u.Email, &u.Age, &u.IsActive, &u.Score)
			users = append(users, u)
		}
		rows.Close()
	}
}

// BenchmarkRawSQL_Complex baseline complexo
func BenchmarkRawSQL_Complex(b *testing.B) {
	_, sqlDB, cleanup := setupBenchDB(b)
	defer cleanup()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rows, err := sqlDB.QueryContext(ctx,
			`SELECT id, name, email, age, is_active, score FROM users
			 WHERE is_active = ? AND age >= ? AND age <= ? AND score > ? AND name LIKE ?
			 ORDER BY score LIMIT 10`,
			true, 25, 40, 100, "%User%")
		if err != nil {
			b.Fatal(err)
		}
		var users []User
		for rows.Next() {
			var u User
			rows.Scan(&u.ID, &u.Name, &u.Email, &u.Age, &u.IsActive, &u.Score)
			users = append(users, u)
		}
		rows.Close()
	}
}

// ============================================================================
// Benchmarks: Insert
// ============================================================================

// BenchmarkInsert_Single insert único
func BenchmarkInsert_Single(b *testing.B) {
	db, _, cleanup := setupBenchDB(b)
	defer cleanup()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		user := &User{
			Name:     fmt.Sprintf("Bench User %d", i),
			Email:    fmt.Sprintf("bench%d@example.com", i),
			Age:      30,
			IsActive: true,
			Score:    100,
		}
		_ = db.DB().Create(ctx, user)
	}
}

// BenchmarkRawSQL_Insert baseline insert
func BenchmarkRawSQL_Insert(b *testing.B) {
	_, sqlDB, cleanup := setupBenchDB(b)
	defer cleanup()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = sqlDB.ExecContext(ctx,
			"INSERT INTO users (name, email, age, is_active, score) VALUES (?, ?, ?, ?, ?)",
			fmt.Sprintf("Bench User %d", i),
			fmt.Sprintf("bench%d@example.com", i),
			30, true, 100)
	}
}

// ============================================================================
// Benchmarks: Update
// ============================================================================

// BenchmarkUpdate_Single update único
func BenchmarkUpdate_Single(b *testing.B) {
	db, _, cleanup := setupBenchDB(b)
	defer cleanup()

	ctx := context.Background()

	// Busca um usuário para atualizar (precisa ser ponteiro para Update)
	user, err := genus.Table[User](db).First(ctx)
	if err != nil {
		b.Fatalf("failed to get user: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		user.Score = i
		_ = db.DB().Update(ctx, &user)
	}
}

// BenchmarkRawSQL_Update baseline update
func BenchmarkRawSQL_Update(b *testing.B) {
	_, sqlDB, cleanup := setupBenchDB(b)
	defer cleanup()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = sqlDB.ExecContext(ctx,
			"UPDATE users SET score = ? WHERE id = ?",
			i, 1)
	}
}

// ============================================================================
// Benchmarks: Query Building (sem execução)
// ============================================================================

// BenchmarkQueryBuild_Only apenas construção da query (sem DB)
func BenchmarkQueryBuild_Only(b *testing.B) {
	db, _, cleanup := setupBenchDB(b)
	defer cleanup()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = genus.Table[User](db).
			Where(UserFields.IsActive.Eq(true)).
			Where(UserFields.Age.Gte(25)).
			Where(UserFields.Age.Lte(40)).
			Where(UserFields.Score.Gt(100)).
			OrderByAsc("score").
			Limit(10)
	}
}

// ============================================================================
// Benchmarks: Aggregations
// ============================================================================

// BenchmarkAggregate_Count aggregação count
func BenchmarkAggregate_Count(b *testing.B) {
	db, _, cleanup := setupBenchDB(b)
	defer cleanup()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = genus.Table[User](db).
			Where(UserFields.IsActive.Eq(true)).
			Aggregate().
			CountAll().
			One(ctx)
	}
}

// BenchmarkAggregate_Sum aggregação sum
func BenchmarkAggregate_Sum(b *testing.B) {
	db, _, cleanup := setupBenchDB(b)
	defer cleanup()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = genus.Table[User](db).
			Where(UserFields.IsActive.Eq(true)).
			Aggregate().
			Sum("score").
			One(ctx)
	}
}

// ============================================================================
// Benchmarks: Memory allocation
// ============================================================================

// BenchmarkAllocation_Query verifica alocações de memória
func BenchmarkAllocation_Query(b *testing.B) {
	db, _, cleanup := setupBenchDB(b)
	defer cleanup()

	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = genus.Table[User](db).
			Where(UserFields.IsActive.Eq(true)).
			Limit(10).
			Find(ctx)
	}
}
