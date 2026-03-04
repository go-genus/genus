package core

import (
	"fmt"
	"log"
	"strings"
	"time"
)

// DefaultLogger is the default Logger implementation that writes to stdout.
type DefaultLogger struct {
	verbose bool
}

// NewDefaultLogger creates a new DefaultLogger.
// If verbose is true, it also displays query arguments.
func NewDefaultLogger(verbose bool) *DefaultLogger {
	return &DefaultLogger{verbose: verbose}
}

// LogQuery logs a successfully executed SQL query.
func (l *DefaultLogger) LogQuery(query string, args []interface{}, duration int64) {
	cleanQuery := cleanSQL(query)

	if l.verbose && len(args) > 0 {
		log.Printf("[GENUS] %s | %s | args: %v", formatDuration(duration), cleanQuery, args)
	} else {
		log.Printf("[GENUS] %s | %s", formatDuration(duration), cleanQuery)
	}
}

// LogError logs an error during query execution.
func (l *DefaultLogger) LogError(query string, args []interface{}, err error) {
	cleanQuery := cleanSQL(query)

	if l.verbose && len(args) > 0 {
		log.Printf("[GENUS ERROR] %s | args: %v | error: %v", cleanQuery, args, err)
	} else {
		log.Printf("[GENUS ERROR] %s | error: %v", cleanQuery, err)
	}
}

// NoOpLogger is a logger that does nothing.
// Useful for tests or when you don't want logging.
type NoOpLogger struct{}

// LogQuery does nothing.
func (n *NoOpLogger) LogQuery(query string, args []interface{}, duration int64) {}

// LogError does nothing.
func (n *NoOpLogger) LogError(query string, args []interface{}, err error) {}

// Helper functions

// cleanSQL removes extra whitespace and line breaks.
func cleanSQL(query string) string {
	// Remove line breaks and tabs
	clean := strings.ReplaceAll(query, "\n", " ")
	clean = strings.ReplaceAll(clean, "\t", " ")

	// Remove multiple spaces
	for strings.Contains(clean, "  ") {
		clean = strings.ReplaceAll(clean, "  ", " ")
	}

	return strings.TrimSpace(clean)
}

// formatDuration formats the duration in a human-readable format.
func formatDuration(nanos int64) string {
	d := time.Duration(nanos)

	if d < time.Microsecond {
		return fmt.Sprintf("%dns", d.Nanoseconds())
	}
	if d < time.Millisecond {
		return fmt.Sprintf("%.2fµs", float64(d.Nanoseconds())/1000.0)
	}
	if d < time.Second {
		return fmt.Sprintf("%.2fms", float64(d.Nanoseconds())/1000000.0)
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}
