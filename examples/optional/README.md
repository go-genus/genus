# Optional Fields Example

Shows how to handle nullable database columns using `core.Optional[T]`.

## What it demonstrates

- Using `core.Optional[T]` for nullable fields (string, int, float64, bool)
- Type-safe optional field queries with `OptionalStringField`, `OptionalIntField`, etc.
- Creating records with and without optional values
- Filtering on nullable columns with `IsNull()` and `IsNotNull()`

## Running

```bash
go run main.go
```

## Key concepts

```go
type Product struct {
    core.Model
    Name        string                 `db:"name"`
    Description core.Optional[string]  `db:"description"` // Can be NULL
    Discount    core.Optional[float64] `db:"discount"`    // Can be NULL
}

// Set a value
product.Description = core.NewOptional("A great product")

// Leave as NULL
product.Discount = core.Optional[float64]{} // zero value = NULL

// Check if set
if product.Description.Valid {
    fmt.Println(product.Description.Value)
}
```
