package tracing

import (
	"context"
	"time"
)

// OTelAdapter adapta a interface do OpenTelemetry para o Genus.
// Este adapter permite usar o OpenTelemetry SDK sem importar diretamente.
//
// Exemplo de uso com OpenTelemetry:
//
//	import (
//	    "go.opentelemetry.io/otel"
//	    "go.opentelemetry.io/otel/attribute"
//	    "go.opentelemetry.io/otel/codes"
//	    "go.opentelemetry.io/otel/trace"
//	)
//
//	otelTracer := otel.Tracer("genus")
//
//	adapter := tracing.NewOTelAdapter(
//	    func(ctx context.Context, name string) (context.Context, func()) {
//	        ctx, span := otelTracer.Start(ctx, name)
//	        return ctx, span.End
//	    },
//	    func(span interface{}, key string, value interface{}) {
//	        s := span.(trace.Span)
//	        s.SetAttributes(attribute.String(key, fmt.Sprintf("%v", value)))
//	    },
//	    func(span interface{}, err error) {
//	        s := span.(trace.Span)
//	        s.RecordError(err)
//	        s.SetStatus(codes.Error, err.Error())
//	    },
//	)
//
//	tracedExec := tracing.NewTracedExecutor(executor, tracing.TracedExecutorConfig{
//	    Tracer: adapter,
//	    DBSystem: "postgresql",
//	})
type OTelAdapter struct {
	startFunc      func(ctx context.Context, name string) (context.Context, interface{})
	setAttrFunc    func(span interface{}, key string, value interface{})
	recordErrFunc  func(span interface{}, err error)
	setStatusFunc  func(span interface{}, ok bool, msg string)
	endFunc        func(span interface{})
	addEventFunc   func(span interface{}, name string, attrs map[string]interface{})
}

// OTelAdapterConfig configura o adapter.
type OTelAdapterConfig struct {
	// StartFunc inicia um novo span e retorna (ctx, span_object).
	StartFunc func(ctx context.Context, name string) (context.Context, interface{})

	// SetAttributeFunc define um atributo no span.
	SetAttributeFunc func(span interface{}, key string, value interface{})

	// RecordErrorFunc registra um erro no span.
	RecordErrorFunc func(span interface{}, err error)

	// SetStatusFunc define o status do span.
	SetStatusFunc func(span interface{}, ok bool, msg string)

	// EndFunc finaliza o span.
	EndFunc func(span interface{})

	// AddEventFunc adiciona um evento ao span.
	AddEventFunc func(span interface{}, name string, attrs map[string]interface{})
}

// NewOTelAdapter cria um novo adapter para OpenTelemetry.
func NewOTelAdapter(config OTelAdapterConfig) *OTelAdapter {
	return &OTelAdapter{
		startFunc:     config.StartFunc,
		setAttrFunc:   config.SetAttributeFunc,
		recordErrFunc: config.RecordErrorFunc,
		setStatusFunc: config.SetStatusFunc,
		endFunc:       config.EndFunc,
		addEventFunc:  config.AddEventFunc,
	}
}

type otelAdapterSpan struct {
	adapter *OTelAdapter
	span    interface{}
}

func (s *otelAdapterSpan) End() {
	if s.adapter.endFunc != nil {
		s.adapter.endFunc(s.span)
	}
}

func (s *otelAdapterSpan) SetStatus(status SpanStatus, description string) {
	if s.adapter.setStatusFunc != nil {
		s.adapter.setStatusFunc(s.span, status == SpanStatusOK, description)
	}
}

func (s *otelAdapterSpan) SetAttribute(key string, value interface{}) {
	if s.adapter.setAttrFunc != nil {
		s.adapter.setAttrFunc(s.span, key, value)
	}
}

func (s *otelAdapterSpan) RecordError(err error) {
	if s.adapter.recordErrFunc != nil {
		s.adapter.recordErrFunc(s.span, err)
	}
}

func (s *otelAdapterSpan) AddEvent(name string, attrs map[string]interface{}) {
	if s.adapter.addEventFunc != nil {
		s.adapter.addEventFunc(s.span, name, attrs)
	}
}

// Start implementa Tracer.Start.
func (a *OTelAdapter) Start(ctx context.Context, name string, opts ...SpanOption) (context.Context, Span) {
	if a.startFunc == nil {
		return ctx, noopSpan{}
	}

	newCtx, span := a.startFunc(ctx, name)
	adapterSpan := &otelAdapterSpan{
		adapter: a,
		span:    span,
	}

	// Aplica opções
	cfg := &spanConfig{attributes: make(map[string]interface{})}
	for _, opt := range opts {
		opt(cfg)
	}

	// Define atributos iniciais
	for k, v := range cfg.attributes {
		adapterSpan.SetAttribute(k, v)
	}

	return newCtx, adapterSpan
}

// SimpleTracer é um tracer simples que usa callbacks.
// Útil para debugging ou integração com sistemas de logging.
type SimpleTracer struct {
	onStart func(ctx context.Context, name string) context.Context
	onEnd   func(name string, duration int64, err error)
}

// SimpleTracerConfig configura o SimpleTracer.
type SimpleTracerConfig struct {
	// OnStart é chamado quando um span é iniciado.
	OnStart func(ctx context.Context, name string) context.Context

	// OnEnd é chamado quando um span é finalizado.
	OnEnd func(name string, durationMs int64, err error)
}

// NewSimpleTracer cria um tracer simples com callbacks.
func NewSimpleTracer(config SimpleTracerConfig) *SimpleTracer {
	return &SimpleTracer{
		onStart: config.OnStart,
		onEnd:   config.OnEnd,
	}
}

type simpleSpan struct {
	tracer    *SimpleTracer
	name      string
	startTime int64
	err       error
}

func (s *simpleSpan) End() {
	if s.tracer.onEnd != nil {
		// Calcula duração
		duration := currentTimeMs() - s.startTime
		s.tracer.onEnd(s.name, duration, s.err)
	}
}

func (s *simpleSpan) SetStatus(status SpanStatus, description string) {
	// No-op para SimpleTracer
}

func (s *simpleSpan) SetAttribute(key string, value interface{}) {
	// No-op para SimpleTracer
}

func (s *simpleSpan) RecordError(err error) {
	s.err = err
}

func (s *simpleSpan) AddEvent(name string, attrs map[string]interface{}) {
	// No-op para SimpleTracer
}

func (t *SimpleTracer) Start(ctx context.Context, name string, opts ...SpanOption) (context.Context, Span) {
	if t.onStart != nil {
		ctx = t.onStart(ctx, name)
	}

	return ctx, &simpleSpan{
		tracer:    t,
		name:      name,
		startTime: currentTimeMs(),
	}
}

func currentTimeMs() int64 {
	return time.Now().UnixMilli()
}
