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

func TestNoopSpanMethods(t *testing.T) {
	// Testa que todos os métodos do noopSpan não causam panic
	var s noopSpan
	s.End()
	s.SetStatus(SpanStatusOK, "ok")
	s.SetStatus(SpanStatusError, "err")
	s.SetAttribute("key", "value")
	s.RecordError(sql.ErrNoRows)
	s.AddEvent("event", map[string]interface{}{"k": "v"})
	s.AddEvent("event", nil)
}

func TestNewTracedExecutor(t *testing.T) {
	mock := &mockExecutor{}

	t.Run("com tracer nil usa NoopTracer", func(t *testing.T) {
		te := NewTracedExecutor(mock, TracedExecutorConfig{
			DBSystem: "postgresql",
			DBName:   "testdb",
		})
		if te.tracer == nil {
			t.Error("Tracer não deveria ser nil")
		}
		// Verifica que é NoopTracer
		if _, ok := te.tracer.(NoopTracer); !ok {
			t.Error("Esperava NoopTracer quando Tracer é nil")
		}
	})

	t.Run("com tracer fornecido", func(t *testing.T) {
		tracer := &mockTracer{}
		te := NewTracedExecutor(mock, TracedExecutorConfig{
			Tracer:     tracer,
			DBSystem:   "mysql",
			DBName:     "mydb",
			ServerAddr: "localhost:3306",
		})
		if te.tracer != tracer {
			t.Error("Tracer deveria ser o fornecido")
		}
		if te.dbSystem != "mysql" {
			t.Errorf("Esperava dbSystem 'mysql', got '%s'", te.dbSystem)
		}
		if te.dbName != "mydb" {
			t.Errorf("Esperava dbName 'mydb', got '%s'", te.dbName)
		}
		if te.serverAddr != "localhost:3306" {
			t.Errorf("Esperava serverAddr 'localhost:3306', got '%s'", te.serverAddr)
		}
	})
}

func TestTracedExecutorExecContext(t *testing.T) {
	t.Run("sucesso", func(t *testing.T) {
		mock := &mockExecutor{
			execResult: &mockResult{rowsAffected: 1},
		}
		tracer := &mockTracer{}
		te := NewTracedExecutor(mock, TracedExecutorConfig{
			Tracer:     tracer,
			DBSystem:   "postgresql",
			DBName:     "testdb",
			ServerAddr: "localhost:5432",
		})

		result, err := te.ExecContext(context.Background(), "INSERT INTO users (name) VALUES ($1)", "Alice")
		if err != nil {
			t.Fatalf("Erro inesperado: %v", err)
		}
		rows, _ := result.RowsAffected()
		if rows != 1 {
			t.Errorf("Esperava 1 row affected, got %d", rows)
		}
		if tracer.lastSpan == nil {
			t.Fatal("Span deveria ter sido criado")
		}
		if tracer.lastSpan.statusSet != SpanStatusOK {
			t.Error("Status deveria ser OK")
		}
		if tracer.lastSpan.attrs["db.system"] != "postgresql" {
			t.Error("Atributo db.system deveria ser postgresql")
		}
		if tracer.lastSpan.attrs["db.name"] != "testdb" {
			t.Error("Atributo db.name deveria ser testdb")
		}
		if tracer.lastSpan.attrs["server.address"] != "localhost:5432" {
			t.Error("Atributo server.address deveria ser localhost:5432")
		}
		if tracer.lastSpan.attrs["db.operation"] != "EXEC" {
			t.Error("Atributo db.operation deveria ser EXEC")
		}
		if _, ok := tracer.lastSpan.attrs["db.rows_affected"]; !ok {
			t.Error("Atributo db.rows_affected deveria estar presente")
		}
		if !tracer.lastSpan.ended {
			t.Error("Span deveria ter sido finalizado")
		}
	})

	t.Run("erro", func(t *testing.T) {
		mock := &mockExecutor{
			execErr: sql.ErrConnDone,
		}
		tracer := &mockTracer{}
		te := NewTracedExecutor(mock, TracedExecutorConfig{
			Tracer:   tracer,
			DBSystem: "postgresql",
		})

		_, err := te.ExecContext(context.Background(), "INSERT INTO users (name) VALUES ($1)", "Alice")
		if err != sql.ErrConnDone {
			t.Fatalf("Esperava ErrConnDone, got %v", err)
		}
		if tracer.lastSpan.statusSet != SpanStatusError {
			t.Error("Status deveria ser Error")
		}
		if tracer.lastSpan.recordedErr != sql.ErrConnDone {
			t.Error("Erro deveria ter sido registrado")
		}
	})

	t.Run("RowsAffected retorna erro", func(t *testing.T) {
		mock := &mockExecutor{
			execResult: &mockResult{rowsAffectedErr: sql.ErrNoRows},
		}
		tracer := &mockTracer{}
		te := NewTracedExecutor(mock, TracedExecutorConfig{
			Tracer:   tracer,
			DBSystem: "sqlite",
		})

		_, err := te.ExecContext(context.Background(), "DELETE FROM users")
		if err != nil {
			t.Fatalf("Erro inesperado: %v", err)
		}
		// db.rows_affected não deve estar presente quando RowsAffected dá erro
		if _, ok := tracer.lastSpan.attrs["db.rows_affected"]; ok {
			t.Error("Atributo db.rows_affected não deveria estar presente quando RowsAffected falha")
		}
	})
}

func TestTracedExecutorQueryContext(t *testing.T) {
	t.Run("sucesso", func(t *testing.T) {
		mock := &mockExecutor{}
		tracer := &mockTracer{}
		te := NewTracedExecutor(mock, TracedExecutorConfig{
			Tracer:   tracer,
			DBSystem: "postgresql",
		})

		_, err := te.QueryContext(context.Background(), "SELECT * FROM users")
		if err != nil {
			t.Fatalf("Erro inesperado: %v", err)
		}
		if tracer.lastSpan.statusSet != SpanStatusOK {
			t.Error("Status deveria ser OK")
		}
		if tracer.lastSpan.attrs["db.operation"] != "SELECT" {
			t.Error("Atributo db.operation deveria ser SELECT")
		}
	})

	t.Run("erro", func(t *testing.T) {
		mock := &mockExecutor{
			queryErr: sql.ErrConnDone,
		}
		tracer := &mockTracer{}
		te := NewTracedExecutor(mock, TracedExecutorConfig{
			Tracer:   tracer,
			DBSystem: "postgresql",
		})

		_, err := te.QueryContext(context.Background(), "SELECT * FROM users")
		if err != sql.ErrConnDone {
			t.Fatalf("Esperava ErrConnDone, got %v", err)
		}
		if tracer.lastSpan.statusSet != SpanStatusError {
			t.Error("Status deveria ser Error")
		}
		if tracer.lastSpan.recordedErr != sql.ErrConnDone {
			t.Error("Erro deveria ter sido registrado")
		}
	})
}

func TestTracedExecutorQueryRowContext(t *testing.T) {
	mock := &mockExecutor{}
	tracer := &mockTracer{}
	te := NewTracedExecutor(mock, TracedExecutorConfig{
		Tracer:   tracer,
		DBSystem: "postgresql",
	})

	row := te.QueryRowContext(context.Background(), "SELECT id FROM users WHERE id = $1", 1)
	_ = row // QueryRowContext sempre retorna *sql.Row
	if tracer.lastSpan.statusSet != SpanStatusOK {
		t.Error("Status deveria ser OK")
	}
	if !tracer.lastSpan.ended {
		t.Error("Span deveria ter sido finalizado")
	}
}

func TestStartSpanTruncatesLongQuery(t *testing.T) {
	mock := &mockExecutor{}
	tracer := &mockTracer{}
	te := NewTracedExecutor(mock, TracedExecutorConfig{
		Tracer:   tracer,
		DBSystem: "postgresql",
	})

	// Cria uma query com mais de 1000 caracteres
	longQuery := "SELECT "
	for i := 0; i < 200; i++ {
		longQuery += "column_name, "
	}

	te.ExecContext(context.Background(), longQuery)

	stmt := tracer.lastSpan.attrs["db.statement"].(string)
	if len(stmt) != 1003 { // 1000 + "..."
		t.Errorf("Esperava query truncada com 1003 chars, got %d", len(stmt))
	}
	if stmt[len(stmt)-3:] != "..." {
		t.Error("Query truncada deveria terminar com '...'")
	}
}

func TestStartSpanWithoutDBNameAndServerAddr(t *testing.T) {
	mock := &mockExecutor{}
	tracer := &mockTracer{}
	te := NewTracedExecutor(mock, TracedExecutorConfig{
		Tracer:   tracer,
		DBSystem: "sqlite",
	})

	te.ExecContext(context.Background(), "SELECT 1")

	if _, ok := tracer.lastSpan.attrs["db.name"]; ok {
		t.Error("db.name não deveria estar presente quando vazio")
	}
	if _, ok := tracer.lastSpan.attrs["server.address"]; ok {
		t.Error("server.address não deveria estar presente quando vazio")
	}
}

func TestOTelSpanWithFuncs(t *testing.T) {
	var endCalled bool
	var statusStatus SpanStatus
	var statusDesc string
	var attrKey string
	var attrVal interface{}
	var recordedErr error
	var eventName string
	var eventAttrs map[string]interface{}

	span := NewOTelSpan(
		func() { endCalled = true },
		func(status SpanStatus, desc string) { statusStatus = status; statusDesc = desc },
		func(key string, value interface{}) { attrKey = key; attrVal = value },
		func(err error) { recordedErr = err },
		func(name string, attrs map[string]interface{}) { eventName = name; eventAttrs = attrs },
	)

	span.SetAttribute("foo", "bar")
	if attrKey != "foo" || attrVal != "bar" {
		t.Error("SetAttribute deveria chamar setAttributeFunc")
	}

	span.SetStatus(SpanStatusError, "falhou")
	if statusStatus != SpanStatusError || statusDesc != "falhou" {
		t.Error("SetStatus deveria chamar setStatusFunc")
	}

	testErr := sql.ErrNoRows
	span.RecordError(testErr)
	if recordedErr != testErr {
		t.Error("RecordError deveria chamar recordErrorFunc")
	}

	attrs := map[string]interface{}{"k": "v"}
	span.AddEvent("myevent", attrs)
	if eventName != "myevent" {
		t.Error("AddEvent deveria chamar addEventFunc")
	}
	if eventAttrs["k"] != "v" {
		t.Error("AddEvent deveria passar attrs corretamente")
	}

	span.End()
	if !endCalled {
		t.Error("End deveria chamar endFunc")
	}
}

func TestOTelSpanWithNilFuncs(t *testing.T) {
	span := NewOTelSpan(nil, nil, nil, nil, nil)

	// Nenhum desses deve causar panic
	span.End()
	span.SetStatus(SpanStatusOK, "")
	span.SetAttribute("key", "value")
	span.RecordError(sql.ErrNoRows)
	span.AddEvent("event", nil)
}

func TestTracingLogger(t *testing.T) {
	t.Run("com logger", func(t *testing.T) {
		mock := &mockLogger{}
		tl := NewTracingLogger(mock, NoopTracer{})

		tl.LogQuery("SELECT 1", []interface{}{}, 10)
		if !mock.logQueryCalled {
			t.Error("LogQuery deveria ter sido chamado")
		}
		if mock.lastQuery != "SELECT 1" {
			t.Errorf("Esperava query 'SELECT 1', got '%s'", mock.lastQuery)
		}

		tl.LogError("SELECT 1", []interface{}{}, sql.ErrNoRows)
		if !mock.logErrorCalled {
			t.Error("LogError deveria ter sido chamado")
		}
		if mock.lastErr != sql.ErrNoRows {
			t.Error("Esperava erro ErrNoRows")
		}
	})

	t.Run("com logger nil", func(t *testing.T) {
		tl := NewTracingLogger(nil, NoopTracer{})

		// Não deve causar panic
		tl.LogQuery("SELECT 1", nil, 10)
		tl.LogError("SELECT 1", nil, sql.ErrNoRows)
	})
}

func TestOTelAdapterAddEvent(t *testing.T) {
	var eventName string
	var eventAttrs map[string]interface{}

	adapter := NewOTelAdapter(OTelAdapterConfig{
		StartFunc: func(ctx context.Context, name string) (context.Context, interface{}) {
			return ctx, "mock-span"
		},
		AddEventFunc: func(span interface{}, name string, attrs map[string]interface{}) {
			eventName = name
			eventAttrs = attrs
		},
	})

	_, span := adapter.Start(context.Background(), "test")
	attrs := map[string]interface{}{"key": "value"}
	span.AddEvent("test-event", attrs)

	if eventName != "test-event" {
		t.Errorf("Esperava event name 'test-event', got '%s'", eventName)
	}
	if eventAttrs["key"] != "value" {
		t.Error("Esperava atributo 'key'='value' no evento")
	}
}

func TestOTelAdapterStartWithNilStartFunc(t *testing.T) {
	adapter := NewOTelAdapter(OTelAdapterConfig{})

	ctx, span := adapter.Start(context.Background(), "test")
	if ctx == nil {
		t.Error("Context não deveria ser nil")
	}
	// Deve retornar noopSpan
	span.End()
	span.SetAttribute("key", "value")
	span.SetStatus(SpanStatusOK, "")
	span.RecordError(nil)
	span.AddEvent("event", nil)
}

func TestOTelAdapterStartWithAttributes(t *testing.T) {
	var attrs []string

	adapter := NewOTelAdapter(OTelAdapterConfig{
		StartFunc: func(ctx context.Context, name string) (context.Context, interface{}) {
			return ctx, "mock-span"
		},
		SetAttributeFunc: func(span interface{}, key string, value interface{}) {
			attrs = append(attrs, key)
		},
	})

	_, _ = adapter.Start(context.Background(), "test", WithAttributes(map[string]interface{}{
		"attr1": "value1",
		"attr2": "value2",
	}))

	if len(attrs) < 2 {
		t.Errorf("Esperava ao menos 2 atributos definidos, got %d", len(attrs))
	}
}

func TestOTelAdapterSpanNilFuncs(t *testing.T) {
	// Adapter sem funções opcionais - não deve causar panic
	adapter := NewOTelAdapter(OTelAdapterConfig{
		StartFunc: func(ctx context.Context, name string) (context.Context, interface{}) {
			return ctx, "mock-span"
		},
	})

	_, span := adapter.Start(context.Background(), "test")
	span.End()
	span.SetStatus(SpanStatusOK, "")
	span.SetAttribute("key", "value")
	span.RecordError(sql.ErrNoRows)
	span.AddEvent("event", nil)
}

func TestSimpleSpanNoOpMethods(t *testing.T) {
	tracer := NewSimpleTracer(SimpleTracerConfig{})
	_, span := tracer.Start(context.Background(), "test")

	// Esses métodos são no-op, não devem causar panic
	span.SetStatus(SpanStatusOK, "ok")
	span.SetStatus(SpanStatusError, "err")
	span.SetAttribute("key", "value")
	span.AddEvent("event", map[string]interface{}{"k": "v"})
	span.End() // onEnd é nil, não deve causar panic
}

func TestSimpleTracerWithNilOnStart(t *testing.T) {
	var endCalled bool
	tracer := NewSimpleTracer(SimpleTracerConfig{
		OnEnd: func(name string, durationMs int64, err error) {
			endCalled = true
		},
	})

	_, span := tracer.Start(context.Background(), "test")
	span.End()
	if !endCalled {
		t.Error("OnEnd deveria ter sido chamado")
	}
}

// --- Mocks ---

type mockTracer struct {
	lastSpan *mockSpanRecord
}

type mockSpanRecord struct {
	attrs       map[string]interface{}
	statusSet   SpanStatus
	recordedErr error
	ended       bool
}

func (s *mockSpanRecord) End() {
	s.ended = true
}

func (s *mockSpanRecord) SetStatus(status SpanStatus, description string) {
	s.statusSet = status
}

func (s *mockSpanRecord) SetAttribute(key string, value interface{}) {
	s.attrs[key] = value
}

func (s *mockSpanRecord) RecordError(err error) {
	s.recordedErr = err
}

func (s *mockSpanRecord) AddEvent(name string, attrs map[string]interface{}) {}

func (mt *mockTracer) Start(ctx context.Context, name string, opts ...SpanOption) (context.Context, Span) {
	span := &mockSpanRecord{attrs: make(map[string]interface{})}
	mt.lastSpan = span
	return ctx, span
}

type mockExecutor struct {
	execResult *mockResult
	execErr    error
	queryErr   error
}

func (m *mockExecutor) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if m.execErr != nil {
		return nil, m.execErr
	}
	if m.execResult != nil {
		return m.execResult, nil
	}
	return &mockResult{}, nil
}

func (m *mockExecutor) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if m.queryErr != nil {
		return nil, m.queryErr
	}
	return nil, nil
}

func (m *mockExecutor) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return nil
}

type mockResult struct {
	lastInsertId    int64
	lastInsertIdErr error
	rowsAffected    int64
	rowsAffectedErr error
}

func (m *mockResult) LastInsertId() (int64, error) {
	return m.lastInsertId, m.lastInsertIdErr
}

func (m *mockResult) RowsAffected() (int64, error) {
	return m.rowsAffected, m.rowsAffectedErr
}

type mockLogger struct {
	logQueryCalled bool
	logErrorCalled bool
	lastQuery      string
	lastErr        error
}

func (m *mockLogger) LogQuery(query string, args []interface{}, duration int64) {
	m.logQueryCalled = true
	m.lastQuery = query
}

func (m *mockLogger) LogError(query string, args []interface{}, err error) {
	m.logErrorCalled = true
	m.lastErr = err
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
