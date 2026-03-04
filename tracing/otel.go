package tracing

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/go-genus/genus/core"
)

// SpanKind representa o tipo de span.
type SpanKind int

const (
	SpanKindClient SpanKind = iota
	SpanKindServer
	SpanKindInternal
)

// SpanStatus representa o status de um span.
type SpanStatus int

const (
	SpanStatusOK SpanStatus = iota
	SpanStatusError
)

// Span representa um span de tracing.
type Span interface {
	// End finaliza o span.
	End()

	// SetStatus define o status do span.
	SetStatus(status SpanStatus, description string)

	// SetAttribute adiciona um atributo ao span.
	SetAttribute(key string, value interface{})

	// RecordError registra um erro no span.
	RecordError(err error)

	// AddEvent adiciona um evento ao span.
	AddEvent(name string, attributes map[string]interface{})
}

// Tracer é a interface para criar spans de tracing.
type Tracer interface {
	// Start cria um novo span.
	Start(ctx context.Context, name string, opts ...SpanOption) (context.Context, Span)
}

// SpanOption configura opções de span.
type SpanOption func(*spanConfig)

type spanConfig struct {
	kind       SpanKind
	attributes map[string]interface{}
}

// WithSpanKind define o tipo de span.
func WithSpanKind(kind SpanKind) SpanOption {
	return func(c *spanConfig) {
		c.kind = kind
	}
}

// WithAttributes adiciona atributos ao span.
func WithAttributes(attrs map[string]interface{}) SpanOption {
	return func(c *spanConfig) {
		for k, v := range attrs {
			c.attributes[k] = v
		}
	}
}

// noopSpan é um span que não faz nada.
type noopSpan struct{}

func (noopSpan) End()                                              {}
func (noopSpan) SetStatus(SpanStatus, string)                      {}
func (noopSpan) SetAttribute(string, interface{})                  {}
func (noopSpan) RecordError(error)                                 {}
func (noopSpan) AddEvent(string, map[string]interface{})           {}

// NoopTracer é um tracer que não faz nada.
type NoopTracer struct{}

func (NoopTracer) Start(ctx context.Context, name string, opts ...SpanOption) (context.Context, Span) {
	return ctx, noopSpan{}
}

// TracedExecutor wraps an executor with tracing.
type TracedExecutor struct {
	executor  core.Executor
	tracer    Tracer
	dbSystem  string // "postgresql", "mysql", "sqlite"
	dbName    string
	serverAddr string
}

// TracedExecutorConfig configura o TracedExecutor.
type TracedExecutorConfig struct {
	// Tracer é o tracer a ser usado. Se nil, usa NoopTracer.
	Tracer Tracer

	// DBSystem é o sistema de banco de dados (ex: "postgresql", "mysql", "sqlite").
	DBSystem string

	// DBName é o nome do banco de dados.
	DBName string

	// ServerAddr é o endereço do servidor (host:port).
	ServerAddr string
}

// NewTracedExecutor cria um executor com tracing.
func NewTracedExecutor(executor core.Executor, config TracedExecutorConfig) *TracedExecutor {
	tracer := config.Tracer
	if tracer == nil {
		tracer = NoopTracer{}
	}

	return &TracedExecutor{
		executor:   executor,
		tracer:     tracer,
		dbSystem:   config.DBSystem,
		dbName:     config.DBName,
		serverAddr: config.ServerAddr,
	}
}

func (te *TracedExecutor) startSpan(ctx context.Context, operation, query string) (context.Context, Span) {
	spanName := fmt.Sprintf("DB %s", operation)

	ctx, span := te.tracer.Start(ctx, spanName, WithSpanKind(SpanKindClient))

	// Standard DB semantic conventions
	span.SetAttribute("db.system", te.dbSystem)
	if te.dbName != "" {
		span.SetAttribute("db.name", te.dbName)
	}
	if te.serverAddr != "" {
		span.SetAttribute("server.address", te.serverAddr)
	}
	span.SetAttribute("db.operation", operation)

	// Trunca query se muito longa
	if len(query) > 1000 {
		query = query[:1000] + "..."
	}
	span.SetAttribute("db.statement", query)

	return ctx, span
}

// ExecContext executa uma query com tracing.
func (te *TracedExecutor) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	ctx, span := te.startSpan(ctx, "EXEC", query)
	defer span.End()

	start := time.Now()
	result, err := te.executor.ExecContext(ctx, query, args...)
	duration := time.Since(start)

	span.SetAttribute("db.duration_ms", duration.Milliseconds())

	if err != nil {
		span.SetStatus(SpanStatusError, err.Error())
		span.RecordError(err)
	} else {
		span.SetStatus(SpanStatusOK, "")
		if rowsAffected, raErr := result.RowsAffected(); raErr == nil {
			span.SetAttribute("db.rows_affected", rowsAffected)
		}
	}

	return result, err
}

// QueryContext executa uma query SELECT com tracing.
func (te *TracedExecutor) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	ctx, span := te.startSpan(ctx, "SELECT", query)
	defer span.End()

	start := time.Now()
	rows, err := te.executor.QueryContext(ctx, query, args...)
	duration := time.Since(start)

	span.SetAttribute("db.duration_ms", duration.Milliseconds())

	if err != nil {
		span.SetStatus(SpanStatusError, err.Error())
		span.RecordError(err)
	} else {
		span.SetStatus(SpanStatusOK, "")
	}

	return rows, err
}

// QueryRowContext executa uma query que retorna uma única linha com tracing.
func (te *TracedExecutor) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	_, span := te.startSpan(ctx, "SELECT", query)
	defer span.End()

	start := time.Now()
	row := te.executor.QueryRowContext(ctx, query, args...)
	duration := time.Since(start)

	span.SetAttribute("db.duration_ms", duration.Milliseconds())
	span.SetStatus(SpanStatusOK, "")

	return row
}

// OTelSpan wraps an OpenTelemetry span.
// Este tipo permite integração com go.opentelemetry.io/otel sem importar diretamente.
type OTelSpan struct {
	endFunc           func()
	setStatusFunc     func(status SpanStatus, description string)
	setAttributeFunc  func(key string, value interface{})
	recordErrorFunc   func(err error)
	addEventFunc      func(name string, attrs map[string]interface{})
}

func (s *OTelSpan) End() {
	if s.endFunc != nil {
		s.endFunc()
	}
}

func (s *OTelSpan) SetStatus(status SpanStatus, description string) {
	if s.setStatusFunc != nil {
		s.setStatusFunc(status, description)
	}
}

func (s *OTelSpan) SetAttribute(key string, value interface{}) {
	if s.setAttributeFunc != nil {
		s.setAttributeFunc(key, value)
	}
}

func (s *OTelSpan) RecordError(err error) {
	if s.recordErrorFunc != nil {
		s.recordErrorFunc(err)
	}
}

func (s *OTelSpan) AddEvent(name string, attrs map[string]interface{}) {
	if s.addEventFunc != nil {
		s.addEventFunc(name, attrs)
	}
}

// NewOTelSpan cria um OTelSpan com funções customizadas.
// Útil para adaptar spans de go.opentelemetry.io/otel.
func NewOTelSpan(
	endFunc func(),
	setStatusFunc func(SpanStatus, string),
	setAttributeFunc func(string, interface{}),
	recordErrorFunc func(error),
	addEventFunc func(string, map[string]interface{}),
) *OTelSpan {
	return &OTelSpan{
		endFunc:          endFunc,
		setStatusFunc:    setStatusFunc,
		setAttributeFunc: setAttributeFunc,
		recordErrorFunc:  recordErrorFunc,
		addEventFunc:     addEventFunc,
	}
}

// TracingLogger é um Logger que também adiciona tracing.
type TracingLogger struct {
	logger core.Logger
	tracer Tracer
}

// NewTracingLogger cria um logger com tracing integrado.
func NewTracingLogger(logger core.Logger, tracer Tracer) *TracingLogger {
	return &TracingLogger{
		logger: logger,
		tracer: tracer,
	}
}

func (tl *TracingLogger) LogQuery(query string, args []interface{}, duration int64) {
	if tl.logger != nil {
		tl.logger.LogQuery(query, args, duration)
	}
}

func (tl *TracingLogger) LogError(query string, args []interface{}, err error) {
	if tl.logger != nil {
		tl.logger.LogError(query, args, err)
	}
}

// QueryMetrics coleta métricas de queries.
type QueryMetrics struct {
	TotalQueries    int64
	TotalErrors     int64
	TotalDurationMs int64
	QueriesByOp     map[string]int64
}

// MetricsCollector coleta métricas de queries.
type MetricsCollector struct {
	metrics *QueryMetrics
}

// NewMetricsCollector cria um novo coletor de métricas.
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		metrics: &QueryMetrics{
			QueriesByOp: make(map[string]int64),
		},
	}
}

// RecordQuery registra uma query executada.
func (mc *MetricsCollector) RecordQuery(operation string, durationMs int64, err error) {
	mc.metrics.TotalQueries++
	mc.metrics.TotalDurationMs += durationMs
	mc.metrics.QueriesByOp[operation]++

	if err != nil {
		mc.metrics.TotalErrors++
	}
}

// GetMetrics retorna as métricas coletadas.
func (mc *MetricsCollector) GetMetrics() QueryMetrics {
	return *mc.metrics
}

// Reset limpa as métricas.
func (mc *MetricsCollector) Reset() {
	mc.metrics = &QueryMetrics{
		QueriesByOp: make(map[string]int64),
	}
}
