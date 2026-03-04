package core

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"time"
)

// CircuitState representa o estado do circuit breaker.
type CircuitState int

const (
	CircuitClosed   CircuitState = iota // Funcionando normalmente
	CircuitOpen                         // Circuito aberto, rejeitando requests
	CircuitHalfOpen                     // Testando se o serviço voltou
)

// String retorna o nome do estado.
func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "CLOSED"
	case CircuitOpen:
		return "OPEN"
	case CircuitHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// ErrCircuitOpen é retornado quando o circuit breaker está aberto.
var ErrCircuitOpen = errors.New("circuit breaker is open")

// CircuitBreakerConfig configura o circuit breaker.
type CircuitBreakerConfig struct {
	// FailureThreshold é o número de falhas consecutivas para abrir o circuito.
	FailureThreshold int

	// SuccessThreshold é o número de sucessos consecutivos para fechar o circuito.
	SuccessThreshold int

	// Timeout é o tempo que o circuito permanece aberto antes de testar novamente.
	Timeout time.Duration

	// OnStateChange é chamado quando o estado muda.
	OnStateChange func(from, to CircuitState)

	// IsFailure determina se um erro deve ser contado como falha.
	// Se nil, todos os erros são considerados falhas.
	IsFailure func(err error) bool
}

// DefaultCircuitBreakerConfig retorna configuração padrão.
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 2,
		Timeout:          30 * time.Second,
	}
}

// CircuitBreaker implementa o padrão circuit breaker para conexões de banco.
type CircuitBreaker struct {
	config          CircuitBreakerConfig
	state           CircuitState
	failures        int
	successes       int
	lastFailureTime time.Time
	mu              sync.RWMutex
}

// NewCircuitBreaker cria um novo circuit breaker.
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	if config.FailureThreshold <= 0 {
		config.FailureThreshold = 5
	}
	if config.SuccessThreshold <= 0 {
		config.SuccessThreshold = 2
	}
	if config.Timeout <= 0 {
		config.Timeout = 30 * time.Second
	}

	return &CircuitBreaker{
		config: config,
		state:  CircuitClosed,
	}
}

// State retorna o estado atual do circuit breaker.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Allow verifica se uma operação pode prosseguir.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		// Verifica se é hora de testar novamente
		if time.Since(cb.lastFailureTime) >= cb.config.Timeout {
			cb.setState(CircuitHalfOpen)
			return true
		}
		return false
	case CircuitHalfOpen:
		return true
	default:
		return false
	}
}

// RecordSuccess registra uma operação bem-sucedida.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures = 0
	cb.successes++

	if cb.state == CircuitHalfOpen && cb.successes >= cb.config.SuccessThreshold {
		cb.setState(CircuitClosed)
	}
}

// RecordFailure registra uma falha.
func (cb *CircuitBreaker) RecordFailure(err error) {
	// Verifica se é uma falha que deve ser contada
	if cb.config.IsFailure != nil && !cb.config.IsFailure(err) {
		return
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.successes = 0
	cb.failures++
	cb.lastFailureTime = time.Now()

	if cb.state == CircuitClosed && cb.failures >= cb.config.FailureThreshold {
		cb.setState(CircuitOpen)
	}

	if cb.state == CircuitHalfOpen {
		cb.setState(CircuitOpen)
	}
}

// setState muda o estado do circuit breaker.
func (cb *CircuitBreaker) setState(newState CircuitState) {
	oldState := cb.state
	if oldState == newState {
		return
	}

	cb.state = newState
	cb.successes = 0

	if newState == CircuitClosed {
		cb.failures = 0
	}

	if cb.config.OnStateChange != nil {
		cb.config.OnStateChange(oldState, newState)
	}
}

// Reset força o circuit breaker para o estado fechado.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.setState(CircuitClosed)
	cb.failures = 0
	cb.successes = 0
}

// Stats retorna estatísticas do circuit breaker.
type CircuitBreakerStats struct {
	State           CircuitState
	Failures        int
	Successes       int
	LastFailureTime time.Time
}

// Stats retorna estatísticas atuais.
func (cb *CircuitBreaker) Stats() CircuitBreakerStats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return CircuitBreakerStats{
		State:           cb.state,
		Failures:        cb.failures,
		Successes:       cb.successes,
		LastFailureTime: cb.lastFailureTime,
	}
}

// CircuitBreakerExecutor wrapa um Executor com circuit breaker.
type CircuitBreakerExecutor struct {
	executor Executor
	cb       *CircuitBreaker
}

// NewCircuitBreakerExecutor cria um executor com circuit breaker.
func NewCircuitBreakerExecutor(executor Executor, cb *CircuitBreaker) *CircuitBreakerExecutor {
	return &CircuitBreakerExecutor{
		executor: executor,
		cb:       cb,
	}
}

// ExecContext executa com circuit breaker.
func (cbe *CircuitBreakerExecutor) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if !cbe.cb.Allow() {
		return nil, ErrCircuitOpen
	}

	result, err := cbe.executor.ExecContext(ctx, query, args...)
	if err != nil {
		cbe.cb.RecordFailure(err)
		return result, err
	}

	cbe.cb.RecordSuccess()
	return result, nil
}

// QueryContext executa query com circuit breaker.
func (cbe *CircuitBreakerExecutor) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if !cbe.cb.Allow() {
		return nil, ErrCircuitOpen
	}

	rows, err := cbe.executor.QueryContext(ctx, query, args...)
	if err != nil {
		cbe.cb.RecordFailure(err)
		return rows, err
	}

	cbe.cb.RecordSuccess()
	return rows, nil
}

// QueryRowContext executa query row com circuit breaker.
func (cbe *CircuitBreakerExecutor) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if !cbe.cb.Allow() {
		return nil // Will cause scan to fail
	}

	row := cbe.executor.QueryRowContext(ctx, query, args...)
	// Note: We can't check for errors here as Row doesn't expose them until Scan
	cbe.cb.RecordSuccess()
	return row
}

// RetryConfig configura política de retry.
type RetryConfig struct {
	// MaxRetries é o número máximo de tentativas.
	MaxRetries int

	// InitialBackoff é o tempo de espera inicial entre tentativas.
	InitialBackoff time.Duration

	// MaxBackoff é o tempo máximo de espera entre tentativas.
	MaxBackoff time.Duration

	// Multiplier é o multiplicador para backoff exponencial.
	Multiplier float64

	// IsRetryable determina se um erro deve ser retentado.
	IsRetryable func(err error) bool
}

// DefaultRetryConfig retorna configuração padrão de retry.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     5 * time.Second,
		Multiplier:     2.0,
		IsRetryable:    DefaultIsRetryable,
	}
}

// DefaultIsRetryable verifica se um erro pode ser retentado.
func DefaultIsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Erros de conexão geralmente podem ser retentados
	errStr := err.Error()
	retryableErrors := []string{
		"connection refused",
		"connection reset",
		"broken pipe",
		"timeout",
		"too many connections",
		"deadlock",
		"lock wait timeout",
	}

	for _, e := range retryableErrors {
		if contains(errStr, e) {
			return true
		}
	}

	return false
}

// contains verifica se s contém substr (case-insensitive).
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					containsInMiddle(s, substr)))
}

func containsInMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// WithRetry executa uma função com retry.
func WithRetry[T any](ctx context.Context, config RetryConfig, fn func() (T, error)) (T, error) {
	var lastErr error
	var zero T
	backoff := config.InitialBackoff

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		result, err := fn()
		if err == nil {
			return result, nil
		}

		lastErr = err

		if !config.IsRetryable(err) {
			return zero, err
		}

		if attempt < config.MaxRetries {
			select {
			case <-ctx.Done():
				return zero, ctx.Err()
			case <-time.After(backoff):
			}

			backoff = time.Duration(float64(backoff) * config.Multiplier)
			if backoff > config.MaxBackoff {
				backoff = config.MaxBackoff
			}
		}
	}

	return zero, lastErr
}
