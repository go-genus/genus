package core

import (
	"context"
	"database/sql"
	"sync"
	"time"
)

// QueryStats representa estatísticas de uma query.
type QueryStats struct {
	SQL          string
	Duration     time.Duration
	Timestamp    time.Time
	RowsAffected int64
	RowsReturned int64
	Args         []interface{}
	Error        error
	IsSlow       bool
	CallerFile   string
	CallerLine   int
}

// ProfilerConfig configura o profiler de queries.
type ProfilerConfig struct {
	// Enabled ativa o profiler.
	Enabled bool

	// SlowQueryThreshold define o limite para queries lentas.
	SlowQueryThreshold time.Duration

	// MaxQueries define o número máximo de queries a armazenar.
	MaxQueries int

	// OnSlowQuery é chamado quando uma query lenta é detectada.
	OnSlowQuery func(stats QueryStats)

	// EnableStackTrace ativa captura de stack trace.
	EnableStackTrace bool
}

// DefaultProfilerConfig retorna configuração padrão.
func DefaultProfilerConfig() ProfilerConfig {
	return ProfilerConfig{
		Enabled:            false,
		SlowQueryThreshold: 100 * time.Millisecond,
		MaxQueries:         1000,
		EnableStackTrace:   false,
	}
}

// Profiler coleta estatísticas de queries.
type Profiler struct {
	config ProfilerConfig
	mu     sync.RWMutex
	stats  []QueryStats
}

// NewProfiler cria um novo profiler.
func NewProfiler(config ProfilerConfig) *Profiler {
	if config.MaxQueries <= 0 {
		config.MaxQueries = 1000
	}
	return &Profiler{
		config: config,
		stats:  make([]QueryStats, 0, config.MaxQueries),
	}
}

// Record registra estatísticas de uma query.
func (p *Profiler) Record(stats QueryStats) {
	if !p.config.Enabled {
		return
	}

	stats.IsSlow = stats.Duration >= p.config.SlowQueryThreshold

	p.mu.Lock()
	if len(p.stats) >= p.config.MaxQueries {
		// Remove as queries mais antigas
		p.stats = p.stats[len(p.stats)/2:]
	}
	p.stats = append(p.stats, stats)
	p.mu.Unlock()

	if stats.IsSlow && p.config.OnSlowQuery != nil {
		p.config.OnSlowQuery(stats)
	}
}

// GetStats retorna todas as estatísticas coletadas.
func (p *Profiler) GetStats() []QueryStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]QueryStats, len(p.stats))
	copy(result, p.stats)
	return result
}

// GetSlowQueries retorna apenas as queries lentas.
func (p *Profiler) GetSlowQueries() []QueryStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var slow []QueryStats
	for _, s := range p.stats {
		if s.IsSlow {
			slow = append(slow, s)
		}
	}
	return slow
}

// Clear limpa as estatísticas.
func (p *Profiler) Clear() {
	p.mu.Lock()
	p.stats = p.stats[:0]
	p.mu.Unlock()
}

// Summary retorna um resumo das estatísticas.
type ProfilerSummary struct {
	TotalQueries     int
	SlowQueries      int
	TotalDuration    time.Duration
	AverageDuration  time.Duration
	MaxDuration      time.Duration
	MinDuration      time.Duration
	QueriesPerSecond float64
}

// Summary retorna um resumo das estatísticas.
func (p *Profiler) Summary() ProfilerSummary {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.stats) == 0 {
		return ProfilerSummary{}
	}

	summary := ProfilerSummary{
		TotalQueries: len(p.stats),
		MinDuration:  p.stats[0].Duration,
	}

	var firstTime, lastTime time.Time
	for i, s := range p.stats {
		summary.TotalDuration += s.Duration
		if s.IsSlow {
			summary.SlowQueries++
		}
		if s.Duration > summary.MaxDuration {
			summary.MaxDuration = s.Duration
		}
		if s.Duration < summary.MinDuration {
			summary.MinDuration = s.Duration
		}
		if i == 0 {
			firstTime = s.Timestamp
		}
		lastTime = s.Timestamp
	}

	if summary.TotalQueries > 0 {
		summary.AverageDuration = summary.TotalDuration / time.Duration(summary.TotalQueries)
	}

	elapsed := lastTime.Sub(firstTime)
	if elapsed > 0 {
		summary.QueriesPerSecond = float64(summary.TotalQueries) / elapsed.Seconds()
	}

	return summary
}

// Enable ativa o profiler.
func (p *Profiler) Enable() {
	p.mu.Lock()
	p.config.Enabled = true
	p.mu.Unlock()
}

// Disable desativa o profiler.
func (p *Profiler) Disable() {
	p.mu.Lock()
	p.config.Enabled = false
	p.mu.Unlock()
}

// IsEnabled verifica se o profiler está ativo.
func (p *Profiler) IsEnabled() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.config.Enabled
}

// SetSlowQueryThreshold atualiza o limite para queries lentas.
func (p *Profiler) SetSlowQueryThreshold(threshold time.Duration) {
	p.mu.Lock()
	p.config.SlowQueryThreshold = threshold
	p.mu.Unlock()
}

// SetOnSlowQuery define o callback para queries lentas.
func (p *Profiler) SetOnSlowQuery(fn func(QueryStats)) {
	p.mu.Lock()
	p.config.OnSlowQuery = fn
	p.mu.Unlock()
}

// ProfiledExecutor wrapa um Executor com profiling.
type ProfiledExecutor struct {
	executor Executor
	profiler *Profiler
}

// NewProfiledExecutor cria um executor com profiling.
func NewProfiledExecutor(executor Executor, profiler *Profiler) *ProfiledExecutor {
	return &ProfiledExecutor{
		executor: executor,
		profiler: profiler,
	}
}

// ExecContext executa com profiling.
func (pe *ProfiledExecutor) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	start := time.Now()
	result, err := pe.executor.ExecContext(ctx, query, args...)
	duration := time.Since(start)

	stats := QueryStats{
		SQL:       query,
		Duration:  duration,
		Timestamp: start,
		Args:      args,
		Error:     err,
	}

	if result != nil {
		if rows, e := result.RowsAffected(); e == nil {
			stats.RowsAffected = rows
		}
	}

	pe.profiler.Record(stats)
	return result, err
}

// QueryContext executa query com profiling.
func (pe *ProfiledExecutor) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	start := time.Now()
	rows, err := pe.executor.QueryContext(ctx, query, args...)
	duration := time.Since(start)

	stats := QueryStats{
		SQL:       query,
		Duration:  duration,
		Timestamp: start,
		Args:      args,
		Error:     err,
	}

	pe.profiler.Record(stats)
	return rows, err
}

// QueryRowContext executa query row com profiling.
func (pe *ProfiledExecutor) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	start := time.Now()
	row := pe.executor.QueryRowContext(ctx, query, args...)
	duration := time.Since(start)

	stats := QueryStats{
		SQL:       query,
		Duration:  duration,
		Timestamp: start,
		Args:      args,
	}

	pe.profiler.Record(stats)
	return row
}
