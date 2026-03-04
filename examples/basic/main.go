package main

import (
	"context"
	"fmt"
	"log"

	"github.com/go-genus/genus"
	"github.com/go-genus/genus/core"
	"github.com/go-genus/genus/query"
)

// User is the user model with embedded Model.
type User struct {
	core.Model        // Embedded: provides ID, CreatedAt, UpdatedAt
	Name       string `db:"name"`
	Email      string `db:"email"`
	Age        int    `db:"age"`
	IsActive   bool   `db:"is_active"`
}

// TableName implements the TableNamer interface (optional).
func (u User) TableName() string {
	return "users"
}

// UserFields defines typed fields for type-safe queries.
// This is the typed "proxy" that allows queries like User.Name.Eq("Alice").
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

	// Connect to the database
	db, err := genus.Open("postgres", "host=localhost user=postgres password=postgres dbname=testdb sslmode=disable")
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	fmt.Println("=== Genus ORM - Type-Safe Demo ===")
	fmt.Println()

	// --- Example 1: Find with type-safe WHERE ---
	fmt.Println("1. Find users with name 'Alice':")

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

	// --- Example 2: Complex queries with AND/OR ---
	fmt.Println("\n2. Find users with age > 25 AND active:")

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

	// --- Example 3: Query with LIKE ---
	fmt.Println("\n3. Find users with email containing 'example.com':")

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

	// --- Example 4: IN query ---
	fmt.Println("\n4. Find users with age in [20, 25, 30]:")

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

	// --- Example 5: First (fetch only one) ---
	fmt.Println("\n5. Find first user with name 'Bob':")

	firstUser, err := genus.Table[User](db).
		Where(UserFields.Name.Eq("Bob")).
		First(ctx)

	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("   Found: %s (%s) - Age: %d\n", firstUser.Name, firstUser.Email, firstUser.Age)
	}

	// --- Example 6: Count ---
	fmt.Println("\n6. Count active users:")

	count, err := genus.Table[User](db).
		Where(UserFields.IsActive.Eq(true)).
		Count(ctx)

	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("   Total active users: %d\n", count)
	}

	// --- Example 7: Between ---
	fmt.Println("\n7. Find users with age between 20 and 30:")

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

	// --- Example 8: Limit and Offset ---
	fmt.Println("\n8. Find 3 users (pagination):")

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

	// --- Example 9: Create ---
	fmt.Println("\n9. Create new user:")

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

	// --- Example 10: Update ---
	fmt.Println("\n10. Update user:")

	if newUser.ID > 0 {
		newUser.Age = 29
		err = db.DB().Update(ctx, newUser)
		if err != nil {
			log.Printf("Error updating user: %v\n", err)
		} else {
			fmt.Printf("   Updated user %s to age %d\n", newUser.Name, newUser.Age)
		}
	}

	// --- Example 11: Delete ---
	fmt.Println("\n11. Delete user:")

	if newUser.ID > 0 {
		err = db.DB().Delete(ctx, newUser)
		if err != nil {
			log.Printf("Error deleting user: %v\n", err)
		} else {
			fmt.Printf("   Deleted user %s (ID: %d)\n", newUser.Name, newUser.ID)
		}
	}

	// --- Example 12: Complex queries with OR ---
	fmt.Println("\n12. Find users with name 'Alice' OR age > 30:")

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

	fmt.Println("\n=== Demo complete! ===")
	fmt.Println("\n--- Genus Features ---")
	fmt.Println("✓ Type-safe queries using Go Generics")
	fmt.Println("✓ Returns []T directly (no *[]T needed)")
	fmt.Println("✓ Typed fields (UserFields.Name.Eq, UserFields.Age.Gt, etc)")
	fmt.Println("✓ Zero reflection in queries (only in scanning)")
	fmt.Println("✓ Transparent and debuggable SQL queries")
	fmt.Println("✓ Context-aware (all functions receive context.Context)")
	fmt.Println("✓ Fluent API with method chaining")
}
