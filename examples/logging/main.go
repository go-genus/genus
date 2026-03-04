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

// User is the user model.
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

	// --- Example 1: Default Logging (non-verbose) ---
	fmt.Println("1. Default Logging (shows SQL and execution time):")
	fmt.Println("   Connecting to database...")

	db, err := genus.Open("postgres", "host=localhost user=postgres password=postgres dbname=testdb sslmode=disable")
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	fmt.Println("\n   Executing SELECT query...")
	users, err := genus.Table[User](db).
		Where(UserFields.Age.Gt(25)).
		Limit(5).
		Find(ctx)

	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("\n   Returned %d users\n", len(users))
	}

	// --- Example 2: Verbose Logging (shows arguments) ---
	fmt.Println("\n\n2. Verbose Logging (shows SQL, arguments and time):")

	// Create a new connection with verbose logger
	sqlDB, _ := sql.Open("postgres", "host=localhost user=postgres password=postgres dbname=testdb sslmode=disable")
	verboseDB := genus.NewWithLogger(sqlDB, postgres.New(), core.NewDefaultLogger(true))

	fmt.Println("\n   Executing complex query...")
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
		fmt.Printf("\n   Returned %d users\n", len(complexUsers))
	}

	// --- Example 3: CRUD Operations Logging ---
	fmt.Println("\n\n3. CRUD Operations Logging:")

	fmt.Println("\n   Creating new user...")
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
		fmt.Printf("\n   User created with ID: %d\n", newUser.ID)
	}

	if newUser.ID > 0 {
		fmt.Println("\n   Updating user...")
		newUser.Age = 31
		err = verboseDB.DB().Update(ctx, newUser)
		if err != nil {
			log.Printf("Error: %v\n", err)
		}

		fmt.Println("\n   Deleting user...")
		err = verboseDB.DB().Delete(ctx, newUser)
		if err != nil {
			log.Printf("Error: %v\n", err)
		}
	}

	// --- Example 4: Error Logging ---
	fmt.Println("\n\n4. SQL Error Logging:")

	fmt.Println("\n   Executing invalid query (non-existent table)...")

	// Force error by querying a table that doesn't exist
	type FakeModel struct {
		core.Model
		Name string `db:"name"`
	}

	_, err = genus.Table[FakeModel](verboseDB).Find(ctx)
	if err != nil {
		fmt.Printf("\n   Error captured (see log above)\n")
	}

	// --- Example 5: No Logging (NoOpLogger) ---
	fmt.Println("\n\n5. No logging (NoOpLogger):")

	silentDB := genus.NewWithLogger(sqlDB, postgres.New(), &core.NoOpLogger{})
	genus.Table[User](silentDB).
		Where(UserFields.Name.Eq("Alice")).
		Find(ctx)

	fmt.Println("   (No SQL log was displayed)")

	// --- Example 6: Count and Aggregate Queries ---
	fmt.Println("\n\n6. Aggregate Query Logging:")

	fmt.Println("\n   Counting active users...")
	count, err := genus.Table[User](verboseDB).
		Where(UserFields.IsActive.Eq(true)).
		Count(ctx)

	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("\n   Total: %d active users\n", count)
	}

	fmt.Println("\n\n=== Logging Features Summary ===")
	fmt.Println("✓ Automatic logging of all SQL queries")
	fmt.Println("✓ Execution time measurement (nanoseconds to seconds)")
	fmt.Println("✓ Verbose mode shows query parameters")
	fmt.Println("✓ Error logging with query and parameters")
	fmt.Println("✓ Clean formatted SQL (no line breaks)")
	fmt.Println("✓ Customizable logger (implement core.Logger)")
	fmt.Println("✓ NoOpLogger to disable logging")
	fmt.Println("✓ Full SQL transparency for debugging")
}
