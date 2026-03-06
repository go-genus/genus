# Basic Example

A complete example showing fundamental Genus ORM operations.

## What it demonstrates

- Model definition with `core.Model` embedding
- Type-safe field definitions for compile-time query validation
- Connecting to a PostgreSQL database
- CRUD operations: Create, Find, Update, Delete
- Type-safe filtering with `UserFields.Name.Eq("Alice")`

## Running

```bash
# Ensure PostgreSQL is running
go run main.go
```

## Key concepts

```go
// Define a model
type User struct {
    core.Model
    Name  string `db:"name"`
    Email string `db:"email"`
}

// Define typed fields for compile-time safe queries
var UserFields = struct {
    Name  query.StringField
    Email query.StringField
}{
    Name:  query.NewStringField("name"),
    Email: query.NewStringField("email"),
}

// Query with type safety
users, err := genus.Table[User](db).
    Where(UserFields.Name.Eq("Alice")).
    Find(ctx)
```
