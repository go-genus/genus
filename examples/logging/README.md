# Logging Example

Demonstrates the built-in SQL logging capabilities of the Genus ORM.

## What it demonstrates

- Default logging (SQL + execution time)
- Verbose mode with detailed query arguments
- How logs appear during CRUD operations
- Toggling logging on/off

## Running

```bash
go run main.go
```

## Key concepts

```go
// Default logging
db, err := genus.Open("postgres", dsn)

// Verbose logging (includes query arguments)
db.SetVerbose(true)
```
