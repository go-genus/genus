# Custom Logger Example

Shows how to implement custom loggers for the Genus ORM.

## What it demonstrates

- Implementing the `core.Logger` interface
- JSON structured logger for log aggregation systems
- Slow query detection logger with configurable thresholds
- Using `core.NewWithLogger()` to inject a custom logger

## Running

```bash
go run main.go
```

## Key concepts

```go
// Implement the Logger interface
type JSONLogger struct{}

func (l *JSONLogger) LogQuery(query string, args []interface{}, duration int64) {
    entry := map[string]interface{}{
        "sql":         query,
        "duration_ms": float64(duration) / 1_000_000,
    }
    json.NewEncoder(os.Stdout).Encode(entry)
}

// Use it
db := core.NewWithLogger(sqlDB, dialect, &JSONLogger{})
```
