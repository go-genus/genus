package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/go-genus/genus"
	"github.com/go-genus/genus/core"
	"github.com/go-genus/genus/dialects/postgres"
	"github.com/go-genus/genus/query"
)

// User é o modelo de usuário.
type User struct {
	core.Model
	Name     string `db:"name"`
	Email    string `db:"email"`
	Age      int    `db:"age"`
	IsActive bool   `db:"is_active"`
}

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

func main() {
	ctx := context.Background()

	fmt.Println("=== Genus ORM - SQL Logging Demo ===")
	fmt.Println()

	// --- Exemplo 1: Logging Padrão (não-verbose) ---
	fmt.Println("1. Logging Padrão (mostra SQL e tempo de execução):")
	fmt.Println("   Conectando ao banco...")

	db, err := genus.Open("postgres", "host=localhost user=postgres password=postgres dbname=testdb sslmode=disable")
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	fmt.Println("\n   Executando query SELECT...")
	users, err := genus.Table[User](db).
		Where(UserFields.Age.Gt(25)).
		Limit(5).
		Find(ctx)

	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("\n   Retornou %d usuários\n", len(users))
	}

	// --- Exemplo 2: Logging Verbose (mostra arguments) ---
	fmt.Println("\n\n2. Logging Verbose (mostra SQL, arguments e tempo):")

	// Cria uma nova conexão com logger verbose
	sqlDB, _ := sql.Open("postgres", "host=localhost user=postgres password=postgres dbname=testdb sslmode=disable")
	verboseDB := genus.NewWithLogger(sqlDB, postgres.New(), core.NewDefaultLogger(true))

	fmt.Println("\n   Executando query complexa...")
	complexUsers, err := genus.Table[User](verboseDB).
		Where(query.And(
			UserFields.Age.Between(20, 40),
			UserFields.IsActive.Eq(true),
		)).
		OrderByDesc("age").
		Limit(3).
		Find(ctx)

	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("\n   Retornou %d usuários\n", len(complexUsers))
	}

	// --- Exemplo 3: Logging de CRUD Operations ---
	fmt.Println("\n\n3. Logging de operações CRUD:")

	fmt.Println("\n   Criando novo usuário...")
	newUser := &User{
		Name:     "Test User",
		Email:    "test@example.com",
		Age:      30,
		IsActive: true,
	}

	err = verboseDB.DB().Create(ctx, newUser)
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("\n   Usuário criado com ID: %d\n", newUser.ID)
	}

	if newUser.ID > 0 {
		fmt.Println("\n   Atualizando usuário...")
		newUser.Age = 31
		err = verboseDB.DB().Update(ctx, newUser)
		if err != nil {
			log.Printf("Error: %v\n", err)
		}

		fmt.Println("\n   Deletando usuário...")
		err = verboseDB.DB().Delete(ctx, newUser)
		if err != nil {
			log.Printf("Error: %v\n", err)
		}
	}

	// --- Exemplo 4: Logging de Erros ---
	fmt.Println("\n\n4. Logging de erros SQL:")

	fmt.Println("\n   Executando query inválida (tabela inexistente)...")

	// Forçar erro tentando consultar uma tabela que não existe
	type FakeModel struct {
		core.Model
		Name string `db:"name"`
	}

	_, err = genus.Table[FakeModel](verboseDB).Find(ctx)
	if err != nil {
		fmt.Printf("\n   Erro capturado (veja o log acima)\n")
	}

	// --- Exemplo 5: Sem Logging (NoOpLogger) ---
	fmt.Println("\n\n5. Sem logging (NoOpLogger):")

	silentDB := genus.NewWithLogger(sqlDB, postgres.New(), &core.NoOpLogger{})
	genus.Table[User](silentDB).
		Where(UserFields.Name.Eq("Alice")).
		Find(ctx)

	fmt.Println("   (Nenhum log SQL foi exibido)")

	// --- Exemplo 6: Count e Queries Agregadas ---
	fmt.Println("\n\n6. Logging de queries agregadas:")

	fmt.Println("\n   Contando usuários ativos...")
	count, err := genus.Table[User](verboseDB).
		Where(UserFields.IsActive.Eq(true)).
		Count(ctx)

	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("\n   Total: %d usuários ativos\n", count)
	}

	fmt.Println("\n\n=== Resumo das Funcionalidades de Logging ===")
	fmt.Println("✓ Logging automático de todas as queries SQL")
	fmt.Println("✓ Medição de tempo de execução (nanosegundos a segundos)")
	fmt.Println("✓ Modo verbose mostra parâmetros da query")
	fmt.Println("✓ Logging de erros com query e parâmetros")
	fmt.Println("✓ SQL formatado e limpo (sem quebras de linha)")
	fmt.Println("✓ Logger customizável (implemente core.Logger)")
	fmt.Println("✓ NoOpLogger para desabilitar logging")
	fmt.Println("✓ Transparência SQL total para debugging")
}
