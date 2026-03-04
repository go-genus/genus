# Getting Started with Genus

This guide will help you set up Genus and write your first type-safe database queries in Go.

## Prerequisites

- Go 1.21 or higher
- A supported database: PostgreSQL, MySQL, or SQLite

## Installation

```bash
go get github.com/go-genus/genus@latest
```

## Step 1: Define Your Model

Models in Genus are regular Go structs with `db` tags that map fields to database columns.

```go
package models

import "github.com/go-genus/genus/core"

type User struct {
    core.Model              // Provides ID, CreatedAt, UpdatedAt
    Name     string `db:"name"`
    Email    string `db:"email"`
    Age      int    `db:"age"`
    IsActive bool   `db:"is_active"`
}

// TableName specifies the database table name (optional)
func (User) TableName() string {
    return "users"
}
```

### The `core.Model` Base

`core.Model` provides common fields:

```go
type Model struct {
    ID        int64      `db:"id"`
    CreatedAt time.Time  `db:"created_at"`
    UpdatedAt time.Time  `db:"updated_at"`
}
```

If you need soft deletes, use `core.SoftDeleteModel` instead:

```go
type User struct {
    core.SoftDeleteModel  // Adds DeletedAt field
    Name string `db:"name"`
}
```

## Step 2: Create Type-Safe Fields

Type-safe fields enable compile-time query validation. You can create them manually or generate them automatically.

### Option A: Manual Definition

```go
package models

import "github.com/go-genus/genus/query"

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
```

### Option B: Code Generation (Recommended)

Install the CLI:

```bash
go install github.com/go-genus/genus/cmd/genus@latest
```

Generate fields automatically:

```bash
genus generate ./models
```

This creates `user_fields.gen.go` with all typed fields.

## Step 3: Connect to the Database

### PostgreSQL

```go
package main

import (
    "context"
    "log"

    "github.com/go-genus/genus"
    _ "github.com/lib/pq"
)

func main() {
    db, err := genus.Open("postgres", "postgres://user:pass@localhost/dbname?sslmode=disable")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Use db...
}
```

### MySQL

```go
import _ "github.com/go-sql-driver/mysql"

db, err := genus.Open("mysql", "user:pass@tcp(localhost:3306)/dbname")
```

### SQLite

```go
import _ "github.com/mattn/go-sqlite3"

db, err := genus.Open("sqlite3", "file:mydb.sqlite?cache=shared")
```

## Step 4: CRUD Operations

### Create

```go
ctx := context.Background()

user := &User{
    Name:     "Alice",
    Email:    "alice@example.com",
    Age:      30,
    IsActive: true,
}

err := db.DB().Create(ctx, user)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Created user with ID: %d\n", user.ID)
```

### Read (Query)

```go
// Find all active users
users, err := genus.Table[User](db).
    Where(UserFields.IsActive.Eq(true)).
    Find(ctx)

// Find with multiple conditions
users, err := genus.Table[User](db).
    Where(UserFields.IsActive.Eq(true)).
    Where(UserFields.Age.Gte(18)).
    Where(UserFields.Name.Like("A%")).
    OrderByDesc("created_at").
    Limit(10).
    Find(ctx)

// Find first matching record
user, err := genus.Table[User](db).
    Where(UserFields.Email.Eq("alice@example.com")).
    First(ctx)

// Count records
count, err := genus.Table[User](db).
    Where(UserFields.IsActive.Eq(true)).
    Count(ctx)
```

### Update

```go
user.Name = "Alice Smith"
user.Age = 31

err := db.DB().Update(ctx, user)
```

### Delete

```go
// Soft delete (if using SoftDeleteModel)
err := db.DB().Delete(ctx, user)

// Hard delete
err := db.DB().ForceDelete(ctx, user)
```

## Step 5: Query Operators

Each field type has specific operators:

### StringField

```go
UserFields.Name.Eq("Alice")           // name = 'Alice'
UserFields.Name.Ne("Bob")             // name != 'Bob'
UserFields.Name.Like("A%")            // name LIKE 'A%'
UserFields.Name.NotLike("%test%")     // name NOT LIKE '%test%'
UserFields.Name.In([]string{"A", "B"}) // name IN ('A', 'B')
UserFields.Name.IsNull()              // name IS NULL
UserFields.Name.IsNotNull()           // name IS NOT NULL
```

### IntField / Int64Field

```go
UserFields.Age.Eq(30)                 // age = 30
UserFields.Age.Ne(25)                 // age != 25
UserFields.Age.Gt(18)                 // age > 18
UserFields.Age.Gte(21)                // age >= 21
UserFields.Age.Lt(65)                 // age < 65
UserFields.Age.Lte(60)                // age <= 60
UserFields.Age.Between(18, 65)        // age BETWEEN 18 AND 65
UserFields.Age.In([]int{25, 30, 35})  // age IN (25, 30, 35)
```

### BoolField

```go
UserFields.IsActive.Eq(true)          // is_active = true
UserFields.IsActive.Ne(false)         // is_active != false
```

## Step 6: Transactions

```go
err := db.DB().WithTx(ctx, func(tx *core.DB) error {
    user := &User{Name: "Alice", Email: "alice@example.com"}
    if err := tx.Create(ctx, user); err != nil {
        return err  // Rollback
    }

    profile := &Profile{UserID: user.ID, Bio: "Hello"}
    if err := tx.Create(ctx, profile); err != nil {
        return err  // Rollback
    }

    return nil  // Commit
})
```

## Step 7: Relationships

### Define Relationships

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
```

### Register Models

```go
func init() {
    genus.RegisterModels(&User{}, &Post{})
}
```

### Eager Loading

```go
// Load users with their posts (2 queries instead of N+1)
users, err := genus.Table[User](db).
    Preload("Posts").
    Find(ctx)

for _, user := range users {
    fmt.Printf("%s has %d posts\n", user.Name, len(user.Posts))
}
```

## Step 8: Migrations

### AutoMigrate (Development)

```go
import "github.com/go-genus/genus/migrate"

err := migrate.AutoMigrate(ctx, db.DB(), db.DB().Dialect(), User{}, Post{})
```

### Manual Migrations (Production)

```go
migrator := migrate.New(db.DB(), db.DB().Dialect(), db.DB().Logger(), migrate.Config{
    TableName: "schema_migrations",
})

migrator.Register(migrate.Migration{
    Version: 1,
    Name:    "create_users_table",
    Up: func(ctx context.Context, db core.Executor, dialect core.Dialect) error {
        _, err := db.ExecContext(ctx, `
            CREATE TABLE users (
                id SERIAL PRIMARY KEY,
                name VARCHAR(255) NOT NULL,
                email VARCHAR(255) UNIQUE NOT NULL,
                created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
            )
        `)
        return err
    },
    Down: func(ctx context.Context, db core.Executor, dialect core.Dialect) error {
        _, err := db.ExecContext(ctx, `DROP TABLE users`)
        return err
    },
})

err := migrator.Up(ctx)
```

## Next Steps

- [API Reference](https://pkg.go.dev/github.com/go-genus/genus)
- [Migration from GORM](./MIGRATION.md)
- [Examples](../examples/)

## Getting Help

- [GitHub Issues](https://github.com/go-genus/genus/issues)
- [GitHub Discussions](https://github.com/go-genus/genus/discussions)
