package tracing

import (
	"context"
	"database/sql"
	"testing"
)

func TestNoopTracer(t *testing.T) {
	tracer := NoopTracer{}
	ctx := context.Background()

	ctx, span := tracer.Start(ctx, "test-span")
	if span == nil {
		t.Error("Expected non-nil span")
	}

	// These should not panic
	span.SetAttribute("key", "value")
	span.SetStatus(SpanStatusOK, "")
	span.RecordError(nil)
	span.AddEvent("event", nil)
	span.End()

	// Context should be unchanged
	if ctx == nil {
		t.Error("Context should not be nil")
	}
}

func TestSimpleTracer(t *testing.T) {
	var startCalled, endCalled bool
	var spanName string
	var endDuration int64
	var endError error

	tracer := NewSimpleTracer(SimpleTracerConfig{
		OnStart: func(ctx context.Context, name string) context.Context {
			startCalled = true
			spanName = name
			return ctx
		},
		OnEnd: func(name string, durationMs int64, err error) {
			endCalled = true
			endDuration = durationMs
			endError = err
		},
	})

	ctx := context.Background()
	ctx, span := tracer.Start(ctx, "test-span")

	if !startCalled {
		t.Error("OnStart should be called")
	}
	if spanName != "test-span" {
		t.Errorf("Expected span name 'test-span', got '%s'", spanName)
	}

	// Record an error
	testErr := sql.ErrNoRows
	span.RecordError(testErr)

	span.End()

	if !endCalled {
		t.Error("OnEnd should be called")
	}
	if endDuration < 0 {
		t.Error("Duration should be non-negative")
	}
	if endError != testErr {
		t.Errorf("Expected error %v, got %v", testErr, endError)
	}
}

func TestOTelAdapter(t *testing.T) {
	var startCalled, endCalled bool
	var attrKey, attrValue string
	var recordedErr error
	var statusOK bool
	var statusMsg string

	adapter := NewOTelAdapter(OTelAdapterConfig{
		StartFunc: func(ctx context.Context, name string) (context.Context, interface{}) {
			startCalled = true
			return ctx, "mock-span"
		},
		SetAttributeFunc: func(span interface{}, key string, value interface{}) {
			attrKey = key
			attrValue = value.(string)
		},
		RecordErrorFunc: func(span interface{}, err error) {
			recordedErr = err
		},
		SetStatusFunc: func(span interface{}, ok bool, msg string) {
			statusOK = ok
			statusMsg = msg
		},
		EndFunc: func(span interface{}) {
			endCalled = true
		},
	})

	ctx := context.Background()
	ctx, span := adapter.Start(ctx, "test-span")

	if !startCalled {
		t.Error("StartFunc should be called")
	}

	span.SetAttribute("testKey", "testValue")
	if attrKey != "testKey" || attrValue != "testValue" {
		t.Errorf("Expected attribute testKey=testValue, got %s=%s", attrKey, attrValue)
	}

	testErr := sql.ErrConnDone
	span.RecordError(testErr)
	if recordedErr != testErr {
		t.Errorf("Expected error %v, got %v", testErr, recordedErr)
	}

	span.SetStatus(SpanStatusOK, "")
	if !statusOK {
		t.Error("Expected status OK")
	}

	span.SetStatus(SpanStatusError, "something went wrong")
	if statusOK {
		t.Error("Expected status not OK")
	}
	if statusMsg != "something went wrong" {
		t.Errorf("Expected message 'something went wrong', got '%s'", statusMsg)
	}

	span.End()
	if !endCalled {
		t.Error("EndFunc should be called")
	}
}

func TestMetricsCollector(t *testing.T) {
	collector := NewMetricsCollector()

	// Record some queries
	collector.RecordQuery("SELECT", 10, nil)
	collector.RecordQuery("SELECT", 15, nil)
	collector.RecordQuery("INSERT", 5, nil)
	collector.RecordQuery("SELECT", 20, sql.ErrNoRows)

	metrics := collector.GetMetrics()

	if metrics.TotalQueries != 4 {
		t.Errorf("Expected 4 total queries, got %d", metrics.TotalQueries)
	}

	if metrics.TotalErrors != 1 {
		t.Errorf("Expected 1 error, got %d", metrics.TotalErrors)
	}

	if metrics.TotalDurationMs != 50 {
		t.Errorf("Expected 50ms total duration, got %d", metrics.TotalDurationMs)
	}

	if metrics.QueriesByOp["SELECT"] != 3 {
		t.Errorf("Expected 3 SELECT queries, got %d", metrics.QueriesByOp["SELECT"])
	}

	if metrics.QueriesByOp["INSERT"] != 1 {
		t.Errorf("Expected 1 INSERT query, got %d", metrics.QueriesByOp["INSERT"])
	}

	// Test reset
	collector.Reset()
	metrics = collector.GetMetrics()
	if metrics.TotalQueries != 0 {
		t.Errorf("Expected 0 queries after reset, got %d", metrics.TotalQueries)
	}
}

func TestSpanOptions(t *testing.T) {
	cfg := &spanConfig{attributes: make(map[string]interface{})}

	WithSpanKind(SpanKindClient)(cfg)
	if cfg.kind != SpanKindClient {
		t.Error("Expected SpanKindClient")
	}

	WithAttributes(map[string]interface{}{"a": 1, "b": "two"})(cfg)
	if cfg.attributes["a"] != 1 {
		t.Error("Expected attribute a=1")
	}
	if cfg.attributes["b"] != "two" {
		t.Error("Expected attribute b='two'")
	}
}
