package main

import (
	"context"
	"fmt"
	"log"

	"github.com/go-genus/genus"
	"github.com/go-genus/genus/core"
	"github.com/go-genus/genus/query"
)

// User é o modelo de usuário com Model embedded.
type User struct {
	core.Model        // Embedded: fornece ID, CreatedAt, UpdatedAt
	Name       string `db:"name"`
	Email      string `db:"email"`
	Age        int    `db:"age"`
	IsActive   bool   `db:"is_active"`
}

// TableName implementa a interface TableNamer (opcional).
func (u User) TableName() string {
	return "users"
}

// UserFields define os campos tipados para queries type-safe.
// Este é o "proxy" tipado que permite queries como User.Name.Eq("Alice").
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

	// Conecta ao banco de dados
	db, err := genus.Open("postgres", "host=localhost user=postgres password=postgres dbname=testdb sslmode=disable")
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	fmt.Println("=== Genus ORM - Type-Safe Demo ===")
	fmt.Println()

	// --- Exemplo 1: Find com WHERE type-safe ---
	fmt.Println("1. Buscar usuários com nome 'Alice':")

	users, err := genus.Table[User](db).
		Where(UserFields.Name.Eq("Alice")).
		Find(ctx)

	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		for _, user := range users {
			fmt.Printf("   Found: %s (%s) - Age: %d\n", user.Name, user.Email, user.Age)
		}
	}

	// --- Exemplo 2: Queries complexas com AND/OR ---
	fmt.Println("\n2. Buscar usuários com idade > 25 E ativos:")

	activeAdults, err := genus.Table[User](db).
		Where(query.And(
			UserFields.Age.Gt(25),
			UserFields.IsActive.Eq(true),
		)).
		OrderByDesc("age").
		Find(ctx)

	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		for _, user := range activeAdults {
			fmt.Printf("   Found: %s - Age: %d, Active: %v\n", user.Name, user.Age, user.IsActive)
		}
	}

	// --- Exemplo 3: Query com LIKE ---
	fmt.Println("\n3. Buscar usuários com email contendo 'example.com':")

	emailUsers, err := genus.Table[User](db).
		Where(UserFields.Email.Like("%example.com")).
		Find(ctx)

	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		for _, user := range emailUsers {
			fmt.Printf("   Found: %s (%s)\n", user.Name, user.Email)
		}
	}

	// --- Exemplo 4: IN query ---
	fmt.Println("\n4. Buscar usuários com idade em [20, 25, 30]:")

	ageUsers, err := genus.Table[User](db).
		Where(UserFields.Age.In(20, 25, 30)).
		Find(ctx)

	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		for _, user := range ageUsers {
			fmt.Printf("   Found: %s - Age: %d\n", user.Name, user.Age)
		}
	}

	// --- Exemplo 5: First (buscar apenas um) ---
	fmt.Println("\n5. Buscar primeiro usuário com nome 'Bob':")

	firstUser, err := genus.Table[User](db).
		Where(UserFields.Name.Eq("Bob")).
		First(ctx)

	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("   Found: %s (%s) - Age: %d\n", firstUser.Name, firstUser.Email, firstUser.Age)
	}

	// --- Exemplo 6: Count ---
	fmt.Println("\n6. Contar usuários ativos:")

	count, err := genus.Table[User](db).
		Where(UserFields.IsActive.Eq(true)).
		Count(ctx)

	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("   Total active users: %d\n", count)
	}

	// --- Exemplo 7: Between ---
	fmt.Println("\n7. Buscar usuários com idade entre 20 e 30:")

	betweenUsers, err := genus.Table[User](db).
		Where(UserFields.Age.Between(20, 30)).
		Find(ctx)

	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		for _, user := range betweenUsers {
			fmt.Printf("   Found: %s - Age: %d\n", user.Name, user.Age)
		}
	}

	// --- Exemplo 8: Limit e Offset ---
	fmt.Println("\n8. Buscar 3 usuários (paginação):")

	pagedUsers, err := genus.Table[User](db).
		OrderByAsc("name").
		Limit(3).
		Offset(0).
		Find(ctx)

	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		for _, user := range pagedUsers {
			fmt.Printf("   Found: %s\n", user.Name)
		}
	}

	// --- Exemplo 9: Create ---
	fmt.Println("\n9. Criar novo usuário:")

	newUser := &User{
		Name:     "Charlie",
		Email:    "charlie@example.com",
		Age:      28,
		IsActive: true,
	}

	err = db.DB().Create(ctx, newUser)
	if err != nil {
		log.Printf("Error creating user: %v\n", err)
	} else {
		fmt.Printf("   Created user with ID: %d\n", newUser.ID)
	}

	// --- Exemplo 10: Update ---
	fmt.Println("\n10. Atualizar usuário:")

	if newUser.ID > 0 {
		newUser.Age = 29
		err = db.DB().Update(ctx, newUser)
		if err != nil {
			log.Printf("Error updating user: %v\n", err)
		} else {
			fmt.Printf("   Updated user %s to age %d\n", newUser.Name, newUser.Age)
		}
	}

	// --- Exemplo 11: Delete ---
	fmt.Println("\n11. Deletar usuário:")

	if newUser.ID > 0 {
		err = db.DB().Delete(ctx, newUser)
		if err != nil {
			log.Printf("Error deleting user: %v\n", err)
		} else {
			fmt.Printf("   Deleted user %s (ID: %d)\n", newUser.Name, newUser.ID)
		}
	}

	// --- Exemplo 12: Queries complexas com OR ---
	fmt.Println("\n12. Buscar usuários com nome 'Alice' OU idade > 30:")

	orUsers, err := genus.Table[User](db).
		Where(query.Or(
			UserFields.Name.Eq("Alice"),
			UserFields.Age.Gt(30),
		)).
		Find(ctx)

	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		for _, user := range orUsers {
			fmt.Printf("   Found: %s - Age: %d\n", user.Name, user.Age)
		}
	}

	fmt.Println("\n=== Demo completo! ===")
	fmt.Println("\n--- Características do Genus ---")
	fmt.Println("✓ Type-safe queries usando Go Generics")
	fmt.Println("✓ Retorna []T diretamente (não precisa de *[]T)")
	fmt.Println("✓ Campos tipados (UserFields.Name.Eq, UserFields.Age.Gt, etc)")
	fmt.Println("✓ Zero reflection em queries (apenas no scanning)")
	fmt.Println("✓ Queries SQL transparentes e debugáveis")
	fmt.Println("✓ Context-aware (todas funções recebem context.Context)")
	fmt.Println("✓ Fluent API com method chaining")
}
