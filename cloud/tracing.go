package cloud

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/jaeger" //nolint:staticcheck // maintaining backward compatibility
	"go.opentelemetry.io/otel/exporters/zipkin" //nolint:staticcheck // maintaining backward compatibility
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

// TracingConfig configuração para distributed tracing.
type TracingConfig struct {
	ServiceName    string
	ServiceVersion string
	Environment    string

	// Jaeger
	JaegerEndpoint string // http://localhost:14268/api/traces

	// Zipkin
	ZipkinEndpoint string // http://localhost:9411/api/v2/spans

	// OTLP (OpenTelemetry Protocol)
	OTLPEndpoint string // localhost:4317

	// Sampling
	SampleRate float64 // 0.0 a 1.0, default 1.0 (100%)

	// Batching
	BatchTimeout time.Duration
	MaxBatchSize int
}

// TracingProvider gerencia o provider de tracing.
type TracingProvider struct {
	provider *sdktrace.TracerProvider
	tracer   trace.Tracer
	config   TracingConfig
}

// NewJaegerProvider cria um provider com Jaeger.
func NewJaegerProvider(config TracingConfig) (*TracingProvider, error) {
	if config.SampleRate == 0 {
		config.SampleRate = 1.0
	}
	if config.BatchTimeout == 0 {
		config.BatchTimeout = 5 * time.Second
	}
	if config.MaxBatchSize == 0 {
		config.MaxBatchSize = 512
	}

	// Cria o exporter Jaeger
	exp, err := jaeger.New(
		jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(config.JaegerEndpoint)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Jaeger exporter: %w", err)
	}

	return newTracingProvider(config, exp)
}

// NewZipkinProvider cria um provider com Zipkin.
func NewZipkinProvider(config TracingConfig) (*TracingProvider, error) {
	if config.SampleRate == 0 {
		config.SampleRate = 1.0
	}
	if config.BatchTimeout == 0 {
		config.BatchTimeout = 5 * time.Second
	}

	// Cria o exporter Zipkin
	exp, err := zipkin.New(config.ZipkinEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to create Zipkin exporter: %w", err)
	}

	return newTracingProvider(config, exp)
}

// newTracingProvider cria um provider com um exporter.
func newTracingProvider(config TracingConfig, exp sdktrace.SpanExporter) (*TracingProvider, error) {
	// Cria o resource com informações do serviço
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(config.ServiceName),
			semconv.ServiceVersion(config.ServiceVersion),
			attribute.String("environment", config.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Configura o sampler
	var sampler sdktrace.Sampler
	if config.SampleRate >= 1.0 {
		sampler = sdktrace.AlwaysSample()
	} else if config.SampleRate <= 0 {
		sampler = sdktrace.NeverSample()
	} else {
		sampler = sdktrace.TraceIDRatioBased(config.SampleRate)
	}

	// Cria o TracerProvider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp,
			sdktrace.WithBatchTimeout(config.BatchTimeout),
			sdktrace.WithMaxExportBatchSize(config.MaxBatchSize),
		),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	// Configura como provider global
	otel.SetTracerProvider(tp)

	// Configura propagação de contexto (W3C Trace Context + Baggage)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &TracingProvider{
		provider: tp,
		tracer:   tp.Tracer(config.ServiceName),
		config:   config,
	}, nil
}

// Tracer retorna o tracer.
func (tp *TracingProvider) Tracer() trace.Tracer {
	return tp.tracer
}

// Shutdown encerra o provider gracefully.
func (tp *TracingProvider) Shutdown(ctx context.Context) error {
	return tp.provider.Shutdown(ctx)
}

// TracedDB wraps sql.DB com tracing automático.
type TracedDB struct {
	db     *sql.DB
	tracer trace.Tracer
	dbName string
	dbType string
}

// NewTracedDB cria um DB com tracing.
func NewTracedDB(db *sql.DB, tracer trace.Tracer, dbName, dbType string) *TracedDB {
	return &TracedDB{
		db:     db,
		tracer: tracer,
		dbName: dbName,
		dbType: dbType,
	}
}

// commonAttributes retorna atributos comuns para spans de DB.
func (t *TracedDB) commonAttributes() []attribute.KeyValue {
	return []attribute.KeyValue{
		semconv.DBSystemKey.String(t.dbType),
		semconv.DBName(t.dbName),
	}
}

// QueryContext executa uma query com tracing.
func (t *TracedDB) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	ctx, span := t.tracer.Start(ctx, "db.query",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(t.commonAttributes()...),
		trace.WithAttributes(
			semconv.DBStatement(query),
			attribute.Int("db.args_count", len(args)),
		),
	)
	defer span.End()

	rows, err := t.db.QueryContext(ctx, query, args...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

	return rows, err
}

// ExecContext executa um comando com tracing.
func (t *TracedDB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	ctx, span := t.tracer.Start(ctx, "db.exec",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(t.commonAttributes()...),
		trace.WithAttributes(
			semconv.DBStatement(query),
			attribute.Int("db.args_count", len(args)),
		),
	)
	defer span.End()

	result, err := t.db.ExecContext(ctx, query, args...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else if result != nil {
		if affected, err := result.RowsAffected(); err == nil {
			span.SetAttributes(attribute.Int64("db.rows_affected", affected))
		}
	}

	return result, err
}

// QueryRowContext executa uma query que retorna uma única linha.
func (t *TracedDB) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	_, span := t.tracer.Start(ctx, "db.query_row",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(t.commonAttributes()...),
		trace.WithAttributes(
			semconv.DBStatement(query),
			attribute.Int("db.args_count", len(args)),
		),
	)
	defer span.End()

	return t.db.QueryRowContext(ctx, query, args...)
}

// BeginTx inicia uma transação com tracing.
func (t *TracedDB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*TracedTx, error) {
	ctx, span := t.tracer.Start(ctx, "db.begin_tx",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(t.commonAttributes()...),
	)

	tx, err := t.db.BeginTx(ctx, opts)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		span.End()
		return nil, err
	}

	return &TracedTx{
		tx:     tx,
		tracer: t.tracer,
		span:   span,
		attrs:  t.commonAttributes(),
	}, nil
}

// TracedTx é uma transação com tracing.
type TracedTx struct {
	tx     *sql.Tx
	tracer trace.Tracer
	span   trace.Span
	attrs  []attribute.KeyValue
}

// QueryContext executa uma query na transação.
func (t *TracedTx) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	ctx, span := t.tracer.Start(ctx, "tx.query",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(t.attrs...),
		trace.WithAttributes(
			semconv.DBStatement(query),
			attribute.Int("db.args_count", len(args)),
		),
	)
	defer span.End()

	rows, err := t.tx.QueryContext(ctx, query, args...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

	return rows, err
}

// ExecContext executa um comando na transação.
func (t *TracedTx) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	ctx, span := t.tracer.Start(ctx, "tx.exec",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(t.attrs...),
		trace.WithAttributes(
			semconv.DBStatement(query),
			attribute.Int("db.args_count", len(args)),
		),
	)
	defer span.End()

	result, err := t.tx.ExecContext(ctx, query, args...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

	return result, err
}

// Commit confirma a transação.
func (t *TracedTx) Commit() error {
	defer t.span.End()

	err := t.tx.Commit()
	if err != nil {
		t.span.RecordError(err)
		t.span.SetStatus(codes.Error, err.Error())
	} else {
		t.span.SetAttributes(attribute.String("tx.status", "committed"))
	}

	return err
}

// Rollback reverte a transação.
func (t *TracedTx) Rollback() error {
	defer t.span.End()

	err := t.tx.Rollback()
	t.span.SetAttributes(attribute.String("tx.status", "rolled_back"))
	if err != nil && err != sql.ErrTxDone {
		t.span.RecordError(err)
	}

	return err
}

// TracingMiddleware é um middleware HTTP para propagar contexto de tracing.
func TracingMiddleware(tracer trace.Tracer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extrai contexto de tracing dos headers
			ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

			// Cria span para a requisição
			ctx, span := tracer.Start(ctx, fmt.Sprintf("%s %s", r.Method, r.URL.Path),
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					semconv.HTTPMethod(r.Method),
					semconv.HTTPTarget(r.URL.Path),
					semconv.HTTPScheme(r.URL.Scheme),
					semconv.NetHostName(r.Host),
					attribute.String("http.user_agent", r.UserAgent()),
				),
			)
			defer span.End()

			// Wrapper para capturar status code
			rw := &responseWriter{ResponseWriter: w, statusCode: 200}

			// Executa o handler
			next.ServeHTTP(rw, r.WithContext(ctx))

			// Adiciona status code ao span
			span.SetAttributes(semconv.HTTPStatusCode(rw.statusCode))
			if rw.statusCode >= 400 {
				span.SetStatus(codes.Error, http.StatusText(rw.statusCode))
			}
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// SpanFromContext retorna o span do contexto.
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// AddSpanEvent adiciona um evento ao span atual.
func AddSpanEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.AddEvent(name, trace.WithAttributes(attrs...))
}

// SetSpanError marca o span como erro.
func SetSpanError(ctx context.Context, err error) {
	span := trace.SpanFromContext(ctx)
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

// TraceID retorna o trace ID do contexto.
func TraceID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	return span.SpanContext().TraceID().String()
}

// SpanID retorna o span ID do contexto.
func SpanID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	return span.SpanContext().SpanID().String()
}

// QueryTracer rastreia queries e suas durações.
type QueryTracer struct {
	tracer   trace.Tracer
	mu       sync.RWMutex
	queries  []TracedQuery
	maxStore int
}

// TracedQuery representa uma query rastreada.
type TracedQuery struct {
	TraceID   string
	SpanID    string
	Query     string
	Duration  time.Duration
	Error     error
	Timestamp time.Time
}

// NewQueryTracer cria um novo query tracer.
func NewQueryTracer(tracer trace.Tracer, maxStore int) *QueryTracer {
	if maxStore == 0 {
		maxStore = 1000
	}
	return &QueryTracer{
		tracer:   tracer,
		maxStore: maxStore,
	}
}

// Trace rastreia uma query.
func (qt *QueryTracer) Trace(ctx context.Context, query string, fn func() error) error {
	_, span := qt.tracer.Start(ctx, "db.query",
		trace.WithAttributes(semconv.DBStatement(query)),
	)
	defer span.End()

	start := time.Now()
	err := fn()
	duration := time.Since(start)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

	span.SetAttributes(attribute.Int64("db.duration_ms", duration.Milliseconds()))

	// Armazena a query rastreada
	qt.mu.Lock()
	if len(qt.queries) >= qt.maxStore {
		qt.queries = qt.queries[1:]
	}
	qt.queries = append(qt.queries, TracedQuery{
		TraceID:   span.SpanContext().TraceID().String(),
		SpanID:    span.SpanContext().SpanID().String(),
		Query:     query,
		Duration:  duration,
		Error:     err,
		Timestamp: start,
	})
	qt.mu.Unlock()

	return err
}

// GetSlowQueries retorna queries mais lentas que o threshold.
func (qt *QueryTracer) GetSlowQueries(threshold time.Duration) []TracedQuery {
	qt.mu.RLock()
	defer qt.mu.RUnlock()

	var slow []TracedQuery
	for _, q := range qt.queries {
		if q.Duration >= threshold {
			slow = append(slow, q)
		}
	}
	return slow
}

// GetRecentQueries retorna as queries mais recentes.
func (qt *QueryTracer) GetRecentQueries(limit int) []TracedQuery {
	qt.mu.RLock()
	defer qt.mu.RUnlock()

	if limit > len(qt.queries) {
		limit = len(qt.queries)
	}

	start := len(qt.queries) - limit
	result := make([]TracedQuery, limit)
	copy(result, qt.queries[start:])
	return result
}
