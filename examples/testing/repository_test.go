package testing

import (
	"context"
	"database/sql"
	"testing"

	"github.com/go-genus/genus"
	"github.com/go-genus/genus/core"
	"github.com/go-genus/genus/dialects/postgres"
	"github.com/go-genus/genus/query"
)

// User model para testes
type User struct {
	core.Model
	Name     string `db:"name"`
	Email    string `db:"email"`
	Age      int    `db:"age"`
	IsActive bool   `db:"is_active"`
}

// UserFields para queries type-safe
var UserFields = struct {
	ID       query.Int64Field
	Name     query.StringField
	Email    query.StringField
	Age      query.IntField
	IsActive query.BoolField
}{
	ID:       query.NewInt64Field("id"),
	Name:     query.NewStringField("name"),
	Email:    query.NewStringField("email"),
	Age:      query.NewIntField("age"),
	IsActive: query.NewBoolField("is_active"),
}

// UserRepository encapsula operações de usuário
type UserRepository struct {
	db *genus.Genus
}

func NewUserRepository(db *genus.Genus) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user *User) error {
	return r.db.DB().Create(ctx, user)
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*User, error) {
	user, err := genus.Table[User](r.db).
		Where(UserFields.Email.Eq(email)).
		First(ctx)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) FindActive(ctx context.Context) ([]User, error) {
	return genus.Table[User](r.db).
		Where(UserFields.IsActive.Eq(true)).
		Find(ctx)
}

func (r *UserRepository) FindByAgeRange(ctx context.Context, minAge, maxAge int) ([]User, error) {
	return genus.Table[User](r.db).
		Where(UserFields.Age.Between(minAge, maxAge)).
		Find(ctx)
}

func (r *UserRepository) Update(ctx context.Context, user *User) error {
	return r.db.DB().Update(ctx, user)
}

func (r *UserRepository) Delete(ctx context.Context, user *User) error {
	return r.db.DB().Delete(ctx, user)
}

// setupTestDB cria um banco de dados de teste em memória (SQLite)
// Nota: Para usar SQLite, você precisaria implementar o dialeto SQLite
func setupTestDB() (*genus.Genus, func()) {
	// Este é um exemplo. Em produção, você usaria um banco de teste real
	// ou um mock do Executor interface

	// Exemplo com PostgreSQL de teste:
	// db, err := sql.Open("postgres", "postgresql://test:test@localhost/test_db")
	// if err != nil {
	//     log.Fatalf("Failed to open test database: %v", err)
	// }

	// Para este exemplo, vamos criar um stub
	// Em testes reais, você usaria um banco de dados de teste
	var sqlDB *sql.DB // Normalmente você criaria uma conexão real aqui

	genusDB := genus.New(sqlDB, postgres.New())

	cleanup := func() {
		// Cleanup do banco de teste
		// sqlDB.Close()
	}

	return genusDB, cleanup
}

// Exemplo de teste com mock
type MockExecutor struct {
	createFunc func(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	queryFunc  func(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
}

func (m *MockExecutor) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, query, args...)
	}
	return nil, nil
}

func (m *MockExecutor) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if m.queryFunc != nil {
		return m.queryFunc(ctx, query, args...)
	}
	return nil, nil
}

func (m *MockExecutor) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return nil
}

// TestUserRepository_Create é um exemplo de teste
func TestUserRepository_Create(t *testing.T) {
	// Este é um exemplo conceitual
	// Em testes reais, você configuraria um banco de teste real

	t.Skip("Exemplo conceitual - requer banco de dados de teste configurado")

	db, cleanup := setupTestDB()
	defer cleanup()

	repo := NewUserRepository(db)
	ctx := context.Background()

	user := &User{
		Name:     "Test User",
		Email:    "test@example.com",
		Age:      25,
		IsActive: true,
	}

	err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	if user.ID == 0 {
		t.Error("Expected user ID to be set after creation")
	}
}

// TestUserRepository_FindByEmail é um exemplo de teste de query
func TestUserRepository_FindByEmail(t *testing.T) {
	t.Skip("Exemplo conceitual - requer banco de dados de teste configurado")

	db, cleanup := setupTestDB()
	defer cleanup()

	repo := NewUserRepository(db)
	ctx := context.Background()

	// Criar usuário de teste
	user := &User{
		Name:     "Alice",
		Email:    "alice@example.com",
		Age:      28,
		IsActive: true,
	}
	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Buscar por email
	found, err := repo.FindByEmail(ctx, "alice@example.com")
	if err != nil {
		t.Fatalf("Failed to find user: %v", err)
	}

	if found.Email != user.Email {
		t.Errorf("Expected email %s, got %s", user.Email, found.Email)
	}

	if found.Name != user.Name {
		t.Errorf("Expected name %s, got %s", user.Name, found.Name)
	}
}

// TestUserRepository_FindActive testa busca de usuários ativos
func TestUserRepository_FindActive(t *testing.T) {
	t.Skip("Exemplo conceitual - requer banco de dados de teste configurado")

	db, cleanup := setupTestDB()
	defer cleanup()

	repo := NewUserRepository(db)
	ctx := context.Background()

	// Criar usuários de teste
	users := []*User{
		{Name: "Alice", Email: "alice@example.com", Age: 28, IsActive: true},
		{Name: "Bob", Email: "bob@example.com", Age: 32, IsActive: false},
		{Name: "Charlie", Email: "charlie@example.com", Age: 24, IsActive: true},
	}

	for _, u := range users {
		if err := repo.Create(ctx, u); err != nil {
			t.Fatalf("Failed to create test user: %v", err)
		}
	}

	// Buscar apenas ativos
	active, err := repo.FindActive(ctx)
	if err != nil {
		t.Fatalf("Failed to find active users: %v", err)
	}

	if len(active) != 2 {
		t.Errorf("Expected 2 active users, got %d", len(active))
	}

	for _, u := range active {
		if !u.IsActive {
			t.Errorf("Found inactive user in active results: %s", u.Name)
		}
	}
}

// TestUserRepository_Transaction testa operações em transação
func TestUserRepository_Transaction(t *testing.T) {
	t.Skip("Exemplo conceitual - requer banco de dados de teste configurado")

	db, cleanup := setupTestDB()
	defer cleanup()

	repo := NewUserRepository(db)
	ctx := context.Background()

	// Testar transação bem-sucedida
	err := db.DB().WithTx(ctx, func(txDB *core.DB) error {
		user := &User{
			Name:     "Transaction Test",
			Email:    "tx@example.com",
			Age:      30,
			IsActive: true,
		}
		return txDB.Create(ctx, user)
	})

	if err != nil {
		t.Fatalf("Transaction failed: %v", err)
	}

	// Verificar se o usuário foi criado
	found, err := repo.FindByEmail(ctx, "tx@example.com")
	if err != nil {
		t.Fatalf("Failed to find user after transaction: %v", err)
	}

	if found.Email != "tx@example.com" {
		t.Error("User was not created in transaction")
	}

	// Testar rollback em caso de erro
	initialCount, _ := genus.Table[User](db).Count(ctx)

	err = db.DB().WithTx(ctx, func(txDB *core.DB) error {
		user := &User{
			Name:     "Rollback Test",
			Email:    "rollback@example.com",
			Age:      25,
			IsActive: true,
		}
		if err := txDB.Create(ctx, user); err != nil {
			return err
		}

		// Forçar erro para testar rollback
		return sql.ErrTxDone
	})

	if err == nil {
		t.Error("Expected transaction to fail")
	}

	// Verificar se o rollback funcionou
	finalCount, _ := genus.Table[User](db).Count(ctx)
	if finalCount != initialCount {
		t.Errorf("Expected count to remain %d after rollback, got %d", initialCount, finalCount)
	}
}

// BenchmarkUserRepository_FindActive benchmark para queries
func BenchmarkUserRepository_FindActive(b *testing.B) {
	b.Skip("Exemplo conceitual - requer banco de dados de teste configurado")

	db, cleanup := setupTestDB()
	defer cleanup()

	repo := NewUserRepository(db)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = repo.FindActive(ctx)
	}
}
