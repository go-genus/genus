package core

import (
	"fmt"
	"log"
	"strings"
	"time"
)

// DefaultLogger é a implementação padrão do Logger que escreve para stdout.
type DefaultLogger struct {
	verbose bool
}

// NewDefaultLogger cria um novo DefaultLogger.
// Se verbose for true, exibe os arguments da query também.
func NewDefaultLogger(verbose bool) *DefaultLogger {
	return &DefaultLogger{verbose: verbose}
}

// LogQuery registra uma query SQL executada com sucesso.
func (l *DefaultLogger) LogQuery(query string, args []interface{}, duration int64) {
	cleanQuery := cleanSQL(query)

	if l.verbose && len(args) > 0 {
		log.Printf("[GENUS] %s | %s | args: %v", formatDuration(duration), cleanQuery, args)
	} else {
		log.Printf("[GENUS] %s | %s", formatDuration(duration), cleanQuery)
	}
}

// LogError registra um erro durante a execução de uma query.
func (l *DefaultLogger) LogError(query string, args []interface{}, err error) {
	cleanQuery := cleanSQL(query)

	if l.verbose && len(args) > 0 {
		log.Printf("[GENUS ERROR] %s | args: %v | error: %v", cleanQuery, args, err)
	} else {
		log.Printf("[GENUS ERROR] %s | error: %v", cleanQuery, err)
	}
}

// NoOpLogger é um logger que não faz nada.
// Útil para testes ou quando você não quer logging.
type NoOpLogger struct{}

// LogQuery não faz nada.
func (n *NoOpLogger) LogQuery(query string, args []interface{}, duration int64) {}

// LogError não faz nada.
func (n *NoOpLogger) LogError(query string, args []interface{}, err error) {}

// Funções auxiliares

// cleanSQL remove espaços em branco extras e quebras de linha.
func cleanSQL(query string) string {
	// Remove quebras de linha e tabs
	clean := strings.ReplaceAll(query, "\n", " ")
	clean = strings.ReplaceAll(clean, "\t", " ")

	// Remove múltiplos espaços
	for strings.Contains(clean, "  ") {
		clean = strings.ReplaceAll(clean, "  ", " ")
	}

	return strings.TrimSpace(clean)
}

// formatDuration formata a duração em um formato legível.
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
