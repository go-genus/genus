package query

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/go-genus/genus/core"
)

// N1Detection representa uma detecção de N+1 query.
type N1Detection struct {
	Pattern     string    `json:"pattern"`
	Count       int       `json:"count"`
	Threshold   int       `json:"threshold"`
	FirstSeen   time.Time `json:"first_seen"`
	LastSeen    time.Time `json:"last_seen"`
	CallerFile  string    `json:"caller_file"`
	CallerLine  int       `json:"caller_line"`
	SampleQuery string    `json:"sample_query"`
	Suggestion  string    `json:"suggestion"`
}

// N1DetectorConfig configura o detector de N+1.
type N1DetectorConfig struct {
	// Enabled ativa a detecção.
	Enabled bool

	// Threshold é o número de queries similares para disparar alerta.
	Threshold int

	// TimeWindow é a janela de tempo para agrupar queries.
	TimeWindow time.Duration

	// OnDetection é chamado quando N+1 é detectado.
	OnDetection func(detection N1Detection)

	// IncludeStackTrace inclui stack trace na detecção.
	IncludeStackTrace bool

	// ExcludePatterns são patterns de queries a ignorar.
	ExcludePatterns []string
}

// DefaultN1DetectorConfig retorna configuração padrão.
func DefaultN1DetectorConfig() N1DetectorConfig {
	return N1DetectorConfig{
		Enabled:    true,
		Threshold:  5,
		TimeWindow: 100 * time.Millisecond,
	}
}

// queryRecord registra uma query para análise.
type queryRecord struct {
	pattern   string
	query     string
	timestamp time.Time
	caller    string
	line      int
}

// N1Detector detecta queries N+1.
type N1Detector struct {
	config     N1DetectorConfig
	mu         sync.RWMutex
	queries    []queryRecord
	detections map[string]*N1Detection
	lastClean  time.Time
}

// NewN1Detector cria um novo detector de N+1.
func NewN1Detector(config N1DetectorConfig) *N1Detector {
	if config.Threshold <= 0 {
		config.Threshold = 5
	}
	if config.TimeWindow <= 0 {
		config.TimeWindow = 100 * time.Millisecond
	}

	return &N1Detector{
		config:     config,
		detections: make(map[string]*N1Detection),
		lastClean:  time.Now(),
	}
}

// Record registra uma query para análise.
func (d *N1Detector) Record(query string) {
	if !d.config.Enabled {
		return
	}

	// Verifica exclusões
	for _, pattern := range d.config.ExcludePatterns {
		if strings.Contains(strings.ToLower(query), strings.ToLower(pattern)) {
			return
		}
	}

	pattern := d.normalizeQuery(query)
	now := time.Now()

	// Obtém caller
	var callerFile string
	var callerLine int
	if d.config.IncludeStackTrace {
		_, callerFile, callerLine, _ = runtime.Caller(2)
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// Limpa queries antigas periodicamente
	if now.Sub(d.lastClean) > d.config.TimeWindow*10 {
		d.cleanOldQueries(now)
		d.lastClean = now
	}

	// Registra query
	d.queries = append(d.queries, queryRecord{
		pattern:   pattern,
		query:     query,
		timestamp: now,
		caller:    callerFile,
		line:      callerLine,
	})

	// Conta queries similares na janela de tempo
	count := 0
	cutoff := now.Add(-d.config.TimeWindow)
	for _, q := range d.queries {
		if q.timestamp.After(cutoff) && q.pattern == pattern {
			count++
		}
	}

	// Verifica se ultrapassou o threshold
	if count >= d.config.Threshold {
		detection, exists := d.detections[pattern]
		if !exists {
			detection = &N1Detection{
				Pattern:     pattern,
				Threshold:   d.config.Threshold,
				FirstSeen:   now,
				SampleQuery: query,
				CallerFile:  callerFile,
				CallerLine:  callerLine,
				Suggestion:  d.generateSuggestion(query),
			}
			d.detections[pattern] = detection
		}

		detection.Count = count
		detection.LastSeen = now

		if d.config.OnDetection != nil && count == d.config.Threshold {
			d.config.OnDetection(*detection)
		}
	}
}

// normalizeQuery normaliza uma query para comparação.
func (d *N1Detector) normalizeQuery(query string) string {
	// Remove valores específicos, mantém estrutura
	normalized := strings.ToLower(query)

	// Remove strings entre aspas
	normalized = regexp.MustCompile(`'[^']*'`).ReplaceAllString(normalized, "?")

	// Remove números
	normalized = regexp.MustCompile(`\b\d+\b`).ReplaceAllString(normalized, "?")

	// Remove placeholders PostgreSQL
	normalized = regexp.MustCompile(`\$\d+`).ReplaceAllString(normalized, "?")

	// Remove espaços extras
	normalized = regexp.MustCompile(`\s+`).ReplaceAllString(normalized, " ")

	return strings.TrimSpace(normalized)
}

// generateSuggestion gera sugestão para resolver N+1.
func (d *N1Detector) generateSuggestion(query string) string {
	query = strings.ToLower(query)

	if strings.Contains(query, "where") && strings.Contains(query, "= ?") {
		// Provavelmente carregando relacionamentos um por um
		return "Consider using Preload() for eager loading or batch the IDs with WHERE id IN (...)"
	}

	if strings.Contains(query, "select") && strings.Contains(query, "from") {
		return "This appears to be a repeated SELECT. Use Preload() for relationships or batch queries with IN clause"
	}

	return "Consider batching these queries or using eager loading with Preload()"
}

// cleanOldQueries remove queries antigas.
func (d *N1Detector) cleanOldQueries(now time.Time) {
	cutoff := now.Add(-d.config.TimeWindow * 2)
	var newQueries []queryRecord

	for _, q := range d.queries {
		if q.timestamp.After(cutoff) {
			newQueries = append(newQueries, q)
		}
	}

	d.queries = newQueries
}

// GetDetections retorna todas as detecções.
func (d *N1Detector) GetDetections() []N1Detection {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var detections []N1Detection
	for _, det := range d.detections {
		detections = append(detections, *det)
	}

	return detections
}

// Clear limpa as detecções.
func (d *N1Detector) Clear() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.queries = nil
	d.detections = make(map[string]*N1Detection)
}

// Reset reinicia o detector.
func (d *N1Detector) Reset() {
	d.Clear()
}

// N1DetectorExecutor wrapa um Executor com detecção N+1.
type N1DetectorExecutor struct {
	executor core.Executor
	detector *N1Detector
}

// NewN1DetectorExecutor cria um executor com detecção N+1.
func NewN1DetectorExecutor(executor core.Executor, detector *N1Detector) *N1DetectorExecutor {
	return &N1DetectorExecutor{
		executor: executor,
		detector: detector,
	}
}

// ExecContext executa com detecção N+1.
func (e *N1DetectorExecutor) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	e.detector.Record(query)
	return e.executor.ExecContext(ctx, query, args...)
}

// QueryContext executa query com detecção N+1.
func (e *N1DetectorExecutor) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	e.detector.Record(query)
	return e.executor.QueryContext(ctx, query, args...)
}

// QueryRowContext executa query row com detecção N+1.
func (e *N1DetectorExecutor) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	e.detector.Record(query)
	return e.executor.QueryRowContext(ctx, query, args...)
}

// N1Report gera um relatório de N+1.
type N1Report struct {
	Detections     []N1Detection `json:"detections"`
	TotalQueries   int           `json:"total_queries"`
	ProblematicPct float64       `json:"problematic_pct"`
	Timestamp      time.Time     `json:"timestamp"`
}

// GenerateReport gera um relatório de N+1.
func (d *N1Detector) GenerateReport() N1Report {
	d.mu.RLock()
	defer d.mu.RUnlock()

	report := N1Report{
		TotalQueries: len(d.queries),
		Timestamp:    time.Now(),
	}

	for _, det := range d.detections {
		report.Detections = append(report.Detections, *det)
	}

	if report.TotalQueries > 0 {
		problematic := 0
		for _, det := range d.detections {
			problematic += det.Count
		}
		report.ProblematicPct = float64(problematic) / float64(report.TotalQueries) * 100
	}

	return report
}

// PrintReport imprime o relatório formatado.
func (d *N1Detector) PrintReport() string {
	report := d.GenerateReport()

	var sb strings.Builder
	sb.WriteString("=== N+1 Query Detection Report ===\n")
	sb.WriteString(fmt.Sprintf("Total Queries: %d\n", report.TotalQueries))
	sb.WriteString(fmt.Sprintf("Problematic: %.1f%%\n", report.ProblematicPct))
	sb.WriteString(fmt.Sprintf("Detections: %d\n\n", len(report.Detections)))

	for i, det := range report.Detections {
		sb.WriteString(fmt.Sprintf("--- Detection %d ---\n", i+1))
		sb.WriteString(fmt.Sprintf("Pattern: %s\n", det.Pattern))
		sb.WriteString(fmt.Sprintf("Count: %d (threshold: %d)\n", det.Count, det.Threshold))
		sb.WriteString(fmt.Sprintf("Sample: %s\n", det.SampleQuery))
		if det.CallerFile != "" {
			sb.WriteString(fmt.Sprintf("Location: %s:%d\n", det.CallerFile, det.CallerLine))
		}
		sb.WriteString(fmt.Sprintf("Suggestion: %s\n\n", det.Suggestion))
	}

	return sb.String()
}
