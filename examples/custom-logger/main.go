package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/go-genus/genus"
	"github.com/go-genus/genus/core"
	"github.com/go-genus/genus/dialects/postgres"
	"github.com/go-genus/genus/query"
)

// User modelo de exemplo.
type User struct {
	core.Model
	Name  string `db:"name"`
	Email string `db:"email"`
	Age   int    `db:"age"`
}

var UserFields = struct {
	Name  query.StringField
	Email query.StringField
	Age   query.IntField
}{
	Name:  query.NewStringField("name"),
	Email: query.NewStringField("email"),
	Age:   query.NewIntField("age"),
}

// --- Custom Logger 1: JSON Logger ---
// Útil para enviar logs estruturados para sistemas de logging externos.

type JSONLogger struct{}

func (l *JSONLogger) LogQuery(query string, args []interface{}, duration int64) {
	logEntry := map[string]interface{}{
		"timestamp":   time.Now().Format(time.RFC3339),
		"level":       "INFO",
		"type":        "query",
		"sql":         query,
		"args":        args,
		"duration":    duration,
		"duration_ms": float64(duration) / 1000000.0,
	}

	jsonData, _ := json.Marshal(logEntry)
	fmt.Println(string(jsonData))
}

func (l *JSONLogger) LogError(query string, args []interface{}, err error) {
	logEntry := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"level":     "ERROR",
		"type":      "query_error",
		"sql":       query,
		"args":      args,
		"error":     err.Error(),
	}

	jsonData, _ := json.Marshal(logEntry)
	fmt.Println(string(jsonData))
}

// --- Custom Logger 2: File Logger ---
// Grava logs em arquivo.

type FileLogger struct {
	file *log.Logger
}

func NewFileLogger(filename string) (*FileLogger, error) {
	// Em produção, você abriria um arquivo real
	// file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	// Para este exemplo, usamos stdout

	return &FileLogger{
		file: log.New(log.Writer(), "[SQL] ", log.LstdFlags),
	}, nil
}

func (l *FileLogger) LogQuery(query string, args []interface{}, duration int64) {
	l.file.Printf("SUCCESS | %dµs | %s | args=%v", duration/1000, query, args)
}

func (l *FileLogger) LogError(query string, args []interface{}, err error) {
	l.file.Printf("ERROR | %s | args=%v | error=%v", query, args, err)
}

// --- Custom Logger 3: Metrics Logger ---
// Coleta métricas de performance.

type MetricsLogger struct {
	totalQueries int64
	totalErrors  int64
	totalTime    int64
}

func (l *MetricsLogger) LogQuery(query string, args []interface{}, duration int64) {
	l.totalQueries++
	l.totalTime += duration

	// Em produção, você enviaria para Prometheus, StatsD, etc
	avgTime := float64(l.totalTime) / float64(l.totalQueries) / 1000000.0
	log.Printf("[METRICS] Query #%d executed in %.2fms (avg: %.2fms)",
		l.totalQueries, float64(duration)/1000000.0, avgTime)
}

func (l *MetricsLogger) LogError(query string, args []interface{}, err error) {
	l.totalErrors++
	log.Printf("[METRICS] Error #%d: %v", l.totalErrors, err)
}

func (l *MetricsLogger) PrintStats() {
	fmt.Printf("\n=== Metrics Summary ===\n")
	fmt.Printf("Total Queries: %d\n", l.totalQueries)
	fmt.Printf("Total Errors:  %d\n", l.totalErrors)
	if l.totalQueries > 0 {
		avgTime := float64(l.totalTime) / float64(l.totalQueries) / 1000000.0
		fmt.Printf("Average Time:  %.2fms\n", avgTime)
		fmt.Printf("Total Time:    %.2fms\n", float64(l.totalTime)/1000000.0)
	}
	fmt.Printf("======================\n")
}

// --- Custom Logger 4: Conditional Logger ---
// Loga apenas queries lentas.

type SlowQueryLogger struct {
	thresholdMs float64
}

func NewSlowQueryLogger(thresholdMs float64) *SlowQueryLogger {
	return &SlowQueryLogger{thresholdMs: thresholdMs}
}

func (l *SlowQueryLogger) LogQuery(query string, args []interface{}, duration int64) {
	durationMs := float64(duration) / 1000000.0
	if durationMs > l.thresholdMs {
		log.Printf("[SLOW QUERY WARNING] %.2fms (threshold: %.2fms) | %s",
			durationMs, l.thresholdMs, query)
	}
}

func (l *SlowQueryLogger) LogError(query string, args []interface{}, err error) {
	log.Printf("[QUERY ERROR] %s | error: %v", query, err)
}

// --- Custom Logger 5: Composite Logger ---
// Combina múltiplos loggers em um só.

type CompositeLogger struct {
	loggers []core.Logger
}

func (c *CompositeLogger) LogQuery(query string, args []interface{}, duration int64) {
	for _, logger := range c.loggers {
		logger.LogQuery(query, args, duration)
	}
}

func (c *CompositeLogger) LogError(query string, args []interface{}, err error) {
	for _, logger := range c.loggers {
		logger.LogError(query, args, err)
	}
}

func main() {
	ctx := context.Background()

	fmt.Println("=== Genus ORM - Custom Logger Demo ===")
	fmt.Println()

	// Conecta ao banco
	sqlDB, err := sql.Open("postgres", "host=localhost user=postgres password=postgres dbname=testdb sslmode=disable")
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	// --- Demo 1: JSON Logger ---
	fmt.Println("1. JSON Logger (structured logging):")
	jsonDB := genus.NewWithLogger(sqlDB, postgres.New(), &JSONLogger{})

	genus.Table[User](jsonDB).
		Where(UserFields.Age.Gt(25)).
		Limit(2).
		Find(ctx)

	// --- Demo 2: File Logger ---
	fmt.Println("\n2. File Logger:")
	fileLogger, _ := NewFileLogger("queries.log")
	fileDB := genus.NewWithLogger(sqlDB, postgres.New(), fileLogger)

	genus.Table[User](fileDB).
		Where(UserFields.Name.Eq("Alice")).
		Find(ctx)

	// --- Demo 3: Metrics Logger ---
	fmt.Println("\n3. Metrics Logger (collects statistics):")
	metricsLogger := &MetricsLogger{}
	metricsDB := genus.NewWithLogger(sqlDB, postgres.New(), metricsLogger)

	// Executa várias queries
	genus.Table[User](metricsDB).Where(UserFields.Age.Gt(20)).Find(ctx)
	genus.Table[User](metricsDB).Where(UserFields.Age.Lt(50)).Count(ctx)
	genus.Table[User](metricsDB).Where(UserFields.Email.Like("%example%")).Find(ctx)

	// Mostra estatísticas
	metricsLogger.PrintStats()

	// --- Demo 4: Slow Query Logger ---
	fmt.Println("\n4. Slow Query Logger (logs only slow queries > 1ms):")
	slowLogger := NewSlowQueryLogger(1.0) // threshold: 1ms
	slowDB := genus.NewWithLogger(sqlDB, postgres.New(), slowLogger)

	genus.Table[User](slowDB).
		Where(UserFields.Age.Between(20, 40)).
		Find(ctx)

	// Forçar query mais lenta com JOIN ou agregação complexa
	genus.Table[User](slowDB).Count(ctx)

	// --- Demo 5: Combining Loggers (Composite Pattern) ---
	fmt.Println("\n5. Composite Logger (multiple loggers at once):")

	compositeLogger := &CompositeLogger{
		loggers: []core.Logger{
			core.NewDefaultLogger(false),
			metricsLogger,
			slowLogger,
		},
	}

	compositeDB := genus.NewWithLogger(sqlDB, postgres.New(), compositeLogger)
	genus.Table[User](compositeDB).
		Where(UserFields.Age.Gt(25)).
		Find(ctx)

	fmt.Println("\n=== Resumo ===")
	fmt.Println("✓ JSONLogger - Logs estruturados para sistemas externos")
	fmt.Println("✓ FileLogger - Grava logs em arquivo")
	fmt.Println("✓ MetricsLogger - Coleta estatísticas de performance")
	fmt.Println("✓ SlowQueryLogger - Alerta sobre queries lentas")
	fmt.Println("✓ CompositeLogger - Combina múltiplos loggers")
	fmt.Println("\nImplemente core.Logger para criar seu próprio logger!")
}
