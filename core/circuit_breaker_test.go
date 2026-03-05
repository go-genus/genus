package core

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCircuitState_String(t *testing.T) {
	tests := []struct {
		state    CircuitState
		expected string
	}{
		{CircuitClosed, "CLOSED"},
		{CircuitOpen, "OPEN"},
		{CircuitHalfOpen, "HALF_OPEN"},
		{CircuitState(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.expected {
			t.Errorf("CircuitState(%d).String() = %q, want %q", tt.state, got, tt.expected)
		}
	}
}

func TestDefaultCircuitBreakerConfig(t *testing.T) {
	config := DefaultCircuitBreakerConfig()
	if config.FailureThreshold != 5 {
		t.Errorf("FailureThreshold = %d, want 5", config.FailureThreshold)
	}
	if config.SuccessThreshold != 2 {
		t.Errorf("SuccessThreshold = %d, want 2", config.SuccessThreshold)
	}
	if config.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", config.Timeout)
	}
}

func TestNewCircuitBreaker(t *testing.T) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig())
	if cb.State() != CircuitClosed {
		t.Errorf("initial state = %v, want CLOSED", cb.State())
	}
}

func TestNewCircuitBreaker_DefaultValues(t *testing.T) {
	// Zero values should get defaults
	cb := NewCircuitBreaker(CircuitBreakerConfig{})
	if cb.config.FailureThreshold != 5 {
		t.Errorf("FailureThreshold = %d, want 5", cb.config.FailureThreshold)
	}
	if cb.config.SuccessThreshold != 2 {
		t.Errorf("SuccessThreshold = %d, want 2", cb.config.SuccessThreshold)
	}
	if cb.config.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", cb.config.Timeout)
	}
}

func TestCircuitBreaker_Allow_Closed(t *testing.T) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig())
	if !cb.Allow() {
		t.Error("Closed circuit should allow requests")
	}
}

func TestCircuitBreaker_OpensAfterFailures(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 1,
		Timeout:          time.Hour,
	}
	cb := NewCircuitBreaker(config)

	err := errors.New("fail")
	cb.RecordFailure(err)
	cb.RecordFailure(err)
	if cb.State() != CircuitClosed {
		t.Error("should still be closed after 2 failures")
	}

	cb.RecordFailure(err)
	if cb.State() != CircuitOpen {
		t.Errorf("state = %v, want OPEN after 3 failures", cb.State())
	}
}

func TestCircuitBreaker_Allow_Open(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          time.Hour, // Long timeout so it stays open
	}
	cb := NewCircuitBreaker(config)

	cb.RecordFailure(errors.New("fail"))
	if cb.Allow() {
		t.Error("Open circuit should not allow requests")
	}
}

func TestCircuitBreaker_TransitionToHalfOpen(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          1 * time.Millisecond, // Very short timeout
	}
	cb := NewCircuitBreaker(config)

	cb.RecordFailure(errors.New("fail"))
	time.Sleep(5 * time.Millisecond)

	if !cb.Allow() {
		t.Error("should allow after timeout (half-open)")
	}
	if cb.State() != CircuitHalfOpen {
		t.Errorf("state = %v, want HALF_OPEN", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenToClose(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 2,
		Timeout:          1 * time.Millisecond,
	}
	cb := NewCircuitBreaker(config)

	cb.RecordFailure(errors.New("fail"))
	time.Sleep(5 * time.Millisecond)
	cb.Allow() // Transition to half-open

	cb.RecordSuccess()
	cb.RecordSuccess()

	if cb.State() != CircuitClosed {
		t.Errorf("state = %v, want CLOSED after successes", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenToOpen(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 2,
		Timeout:          1 * time.Millisecond,
	}
	cb := NewCircuitBreaker(config)

	cb.RecordFailure(errors.New("fail"))
	time.Sleep(5 * time.Millisecond)
	cb.Allow() // Transition to half-open

	cb.RecordFailure(errors.New("fail again"))

	if cb.State() != CircuitOpen {
		t.Errorf("state = %v, want OPEN after failure in half-open", cb.State())
	}
}

func TestCircuitBreaker_RecordSuccess_ResetsFailures(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 1,
		Timeout:          time.Hour,
	}
	cb := NewCircuitBreaker(config)

	cb.RecordFailure(errors.New("fail"))
	cb.RecordFailure(errors.New("fail"))
	cb.RecordSuccess() // Should reset failures

	// One more failure shouldn't open the circuit
	cb.RecordFailure(errors.New("fail"))
	if cb.State() != CircuitClosed {
		t.Error("circuit should still be closed after success reset")
	}
}

func TestCircuitBreaker_IsFailure_Filter(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          time.Hour,
		IsFailure: func(err error) bool {
			return err.Error() == "real failure"
		},
	}
	cb := NewCircuitBreaker(config)

	// This should not count as failure
	cb.RecordFailure(errors.New("not a real failure"))
	if cb.State() != CircuitClosed {
		t.Error("filtered failure should not open circuit")
	}

	// This should count
	cb.RecordFailure(errors.New("real failure"))
	if cb.State() != CircuitOpen {
		t.Error("real failure should open circuit")
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          time.Hour,
	}
	cb := NewCircuitBreaker(config)

	cb.RecordFailure(errors.New("fail"))
	if cb.State() != CircuitOpen {
		t.Fatal("expected open")
	}

	cb.Reset()
	if cb.State() != CircuitClosed {
		t.Errorf("state after Reset = %v, want CLOSED", cb.State())
	}
}

func TestCircuitBreaker_Stats(t *testing.T) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig())

	cb.RecordFailure(errors.New("fail"))
	cb.RecordFailure(errors.New("fail"))

	stats := cb.Stats()
	if stats.State != CircuitClosed {
		t.Errorf("stats.State = %v, want CLOSED", stats.State)
	}
	if stats.Failures != 2 {
		t.Errorf("stats.Failures = %d, want 2", stats.Failures)
	}
	if stats.LastFailureTime.IsZero() {
		t.Error("LastFailureTime should not be zero")
	}
}

func TestCircuitBreaker_OnStateChange(t *testing.T) {
	changes := []struct {
		from, to CircuitState
	}{}
	config := CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          time.Hour,
		OnStateChange: func(from, to CircuitState) {
			changes = append(changes, struct{ from, to CircuitState }{from, to})
		},
	}
	cb := NewCircuitBreaker(config)

	cb.RecordFailure(errors.New("fail"))

	if len(changes) != 1 {
		t.Fatalf("expected 1 state change, got %d", len(changes))
	}
	if changes[0].from != CircuitClosed || changes[0].to != CircuitOpen {
		t.Errorf("change = %v->%v, want CLOSED->OPEN", changes[0].from, changes[0].to)
	}
}

func TestErrCircuitOpen(t *testing.T) {
	if ErrCircuitOpen == nil {
		t.Error("ErrCircuitOpen should not be nil")
	}
	if ErrCircuitOpen.Error() != "circuit breaker is open" {
		t.Errorf("ErrCircuitOpen = %q", ErrCircuitOpen.Error())
	}
}

// CircuitBreakerExecutor tests

func TestNewCircuitBreakerExecutor(t *testing.T) {
	exec := newMockExecutor()
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig())
	cbe := NewCircuitBreakerExecutor(exec, cb)
	if cbe == nil {
		t.Fatal("NewCircuitBreakerExecutor returned nil")
	}
}

func TestCircuitBreakerExecutor_ExecContext_Success(t *testing.T) {
	exec := newMockExecutor()
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig())
	cbe := NewCircuitBreakerExecutor(exec, cb)

	result, err := cbe.ExecContext(context.Background(), "INSERT INTO t VALUES (1)")
	if err != nil {
		t.Errorf("ExecContext error = %v", err)
	}
	if result == nil {
		t.Error("result should not be nil")
	}
}

func TestCircuitBreakerExecutor_ExecContext_Error(t *testing.T) {
	exec := newMockExecutor()
	exec.execErr = errTest
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig())
	cbe := NewCircuitBreakerExecutor(exec, cb)

	_, err := cbe.ExecContext(context.Background(), "INSERT INTO t VALUES (1)")
	if err != errTest {
		t.Errorf("ExecContext error = %v, want %v", err, errTest)
	}
}

func TestCircuitBreakerExecutor_ExecContext_CircuitOpen(t *testing.T) {
	exec := newMockExecutor()
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          time.Hour,
	})
	cbe := NewCircuitBreakerExecutor(exec, cb)

	// Open the circuit
	cb.RecordFailure(errors.New("fail"))

	_, err := cbe.ExecContext(context.Background(), "INSERT INTO t VALUES (1)")
	if err != ErrCircuitOpen {
		t.Errorf("ExecContext error = %v, want ErrCircuitOpen", err)
	}
}

func TestCircuitBreakerExecutor_QueryContext_Success(t *testing.T) {
	exec := newMockExecutor()
	exec.queryErr = nil
	// queryRows is nil but that's ok since we're not checking rows
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig())
	cbe := NewCircuitBreakerExecutor(exec, cb)

	_, err := cbe.QueryContext(context.Background(), "SELECT 1")
	// queryRows is nil, so we get nil rows but no error from our mock
	if err != nil {
		t.Errorf("QueryContext error = %v", err)
	}
}

func TestCircuitBreakerExecutor_QueryContext_Error(t *testing.T) {
	exec := newMockExecutor()
	exec.queryErr = errTest
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig())
	cbe := NewCircuitBreakerExecutor(exec, cb)

	_, err := cbe.QueryContext(context.Background(), "SELECT 1")
	if err != errTest {
		t.Errorf("QueryContext error = %v, want %v", err, errTest)
	}
}

func TestCircuitBreakerExecutor_QueryContext_CircuitOpen(t *testing.T) {
	exec := newMockExecutor()
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          time.Hour,
	})
	cbe := NewCircuitBreakerExecutor(exec, cb)

	cb.RecordFailure(errors.New("fail"))

	_, err := cbe.QueryContext(context.Background(), "SELECT 1")
	if err != ErrCircuitOpen {
		t.Errorf("QueryContext error = %v, want ErrCircuitOpen", err)
	}
}

func TestCircuitBreakerExecutor_QueryRowContext_CircuitOpen(t *testing.T) {
	exec := newMockExecutor()
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          time.Hour,
	})
	cbe := NewCircuitBreakerExecutor(exec, cb)

	cb.RecordFailure(errors.New("fail"))

	row := cbe.QueryRowContext(context.Background(), "SELECT 1")
	if row != nil {
		t.Error("QueryRowContext should return nil when circuit is open")
	}
}

func TestCircuitBreakerExecutor_QueryRowContext_Success(t *testing.T) {
	exec := newMockExecutor()
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 5,
	})
	cbe := NewCircuitBreakerExecutor(exec, cb)

	row := cbe.QueryRowContext(context.Background(), "SELECT 1")
	// row will be nil because mockExecutor returns nil queryRow
	// But the circuit breaker path is exercised
	_ = row
}

func TestCircuitBreaker_SetState_SameState(t *testing.T) {
	changes := 0
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 5,
		OnStateChange: func(from, to CircuitState) {
			changes++
		},
	})

	// State is already Closed, setting to Closed should be a no-op
	cb.setState(CircuitClosed)
	if changes != 0 {
		t.Error("setState with same state should not trigger OnStateChange")
	}
}

func TestCircuitBreaker_Allow_DefaultState(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 5,
	})
	// Default state is closed, should allow
	if !cb.Allow() {
		t.Error("should allow in closed state")
	}
}

// Retry tests

func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()
	if config.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", config.MaxRetries)
	}
	if config.InitialBackoff != 100*time.Millisecond {
		t.Errorf("InitialBackoff = %v, want 100ms", config.InitialBackoff)
	}
	if config.MaxBackoff != 5*time.Second {
		t.Errorf("MaxBackoff = %v, want 5s", config.MaxBackoff)
	}
	if config.Multiplier != 2.0 {
		t.Errorf("Multiplier = %f, want 2.0", config.Multiplier)
	}
}

func TestDefaultIsRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"connection refused", errors.New("connection refused"), true},
		{"timeout", errors.New("timeout"), true},
		{"deadlock", errors.New("deadlock"), true},
		{"broken pipe", errors.New("broken pipe"), true},
		{"too many connections", errors.New("too many connections"), true},
		{"lock wait timeout", errors.New("lock wait timeout"), true},
		{"connection reset", errors.New("connection reset"), true},
		{"not retryable", errors.New("syntax error"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DefaultIsRetryable(tt.err); got != tt.want {
				t.Errorf("DefaultIsRetryable(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		s      string
		substr string
		want   bool
	}{
		{"hello world", "world", true},
		{"hello world", "hello", true},
		{"hello world", "lo wo", true},
		{"hello", "hello", true},
		{"hello", "world", false},
		{"", "", true},
		{"short", "longer string", false},
	}

	for _, tt := range tests {
		if got := contains(tt.s, tt.substr); got != tt.want {
			t.Errorf("contains(%q, %q) = %v, want %v", tt.s, tt.substr, got, tt.want)
		}
	}
}

func TestContainsInMiddle(t *testing.T) {
	tests := []struct {
		s      string
		substr string
		want   bool
	}{
		{"hello world", "lo wo", true},
		{"hello world", "xyz", false},
		{"abc", "b", true},
	}

	for _, tt := range tests {
		if got := containsInMiddle(tt.s, tt.substr); got != tt.want {
			t.Errorf("containsInMiddle(%q, %q) = %v, want %v", tt.s, tt.substr, got, tt.want)
		}
	}
}

func TestWithRetry_Success(t *testing.T) {
	config := RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 1 * time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
		Multiplier:     2.0,
		IsRetryable:    func(err error) bool { return true },
	}

	attempts := 0
	result, err := WithRetry[string](context.Background(), config, func() (string, error) {
		attempts++
		return "ok", nil
	})

	if err != nil {
		t.Errorf("WithRetry error = %v", err)
	}
	if result != "ok" {
		t.Errorf("result = %q, want %q", result, "ok")
	}
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1", attempts)
	}
}

func TestWithRetry_RetriesAndSucceeds(t *testing.T) {
	config := RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 1 * time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
		Multiplier:     2.0,
		IsRetryable:    func(err error) bool { return true },
	}

	attempts := 0
	result, err := WithRetry[string](context.Background(), config, func() (string, error) {
		attempts++
		if attempts < 3 {
			return "", errors.New("fail")
		}
		return "ok", nil
	})

	if err != nil {
		t.Errorf("WithRetry error = %v", err)
	}
	if result != "ok" {
		t.Errorf("result = %q, want %q", result, "ok")
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
}

func TestWithRetry_NonRetryableError(t *testing.T) {
	config := RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 1 * time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
		Multiplier:     2.0,
		IsRetryable:    func(err error) bool { return false },
	}

	attempts := 0
	_, err := WithRetry[string](context.Background(), config, func() (string, error) {
		attempts++
		return "", errors.New("non-retryable")
	})

	if err == nil {
		t.Error("should return error")
	}
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1 (should not retry)", attempts)
	}
}

func TestWithRetry_ContextCanceled(t *testing.T) {
	config := RetryConfig{
		MaxRetries:     10,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     1 * time.Second,
		Multiplier:     2.0,
		IsRetryable:    func(err error) bool { return true },
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := WithRetry[string](ctx, config, func() (string, error) {
		return "", errors.New("fail")
	})

	if err == nil {
		t.Error("should return error when context is canceled")
	}
}

func TestWithRetry_MaxBackoff(t *testing.T) {
	config := RetryConfig{
		MaxRetries:     5,
		InitialBackoff: 1 * time.Millisecond,
		MaxBackoff:     2 * time.Millisecond,
		Multiplier:     10.0,
		IsRetryable:    func(err error) bool { return true },
	}

	attempts := 0
	_, err := WithRetry[int](context.Background(), config, func() (int, error) {
		attempts++
		return 0, errors.New("fail")
	})

	if err == nil {
		t.Error("should return error after max retries")
	}
	if attempts != 6 { // 1 initial + 5 retries
		t.Errorf("attempts = %d, want 6", attempts)
	}
}
