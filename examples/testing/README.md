# Testing Patterns Example

Shows recommended patterns for testing code that uses the Genus ORM.

## What it demonstrates

- Repository pattern for testable database access
- Using interfaces to mock the ORM layer
- Table-driven tests with type-safe queries
- Setting up test databases with `sql.Open`

## Key concepts

```go
// Repository wraps Genus for testability
type UserRepository struct {
    db *genus.Genus
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*User, error) {
    return genus.Table[User](r.db).
        Where(UserFields.Email.Eq(email)).
        First(ctx)
}
```

## Running tests

```bash
go test ./examples/testing/
```
