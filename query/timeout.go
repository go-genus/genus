package query

import (
	"context"
	"time"
)

// TimeoutBuilder adiciona timeout configurável às queries.
type TimeoutBuilder[T any] struct {
	builder *Builder[T]
	timeout time.Duration
}

// WithTimeout retorna um builder com timeout configurado.
//
// Exemplo:
//
//	users, err := genus.Table[User](db).
//	    Where(UserFields.Active.Eq(true)).
//	    WithTimeout(5 * time.Second).
//	    Find(ctx)
func (b *Builder[T]) WithTimeout(timeout time.Duration) *TimeoutBuilder[T] {
	return &TimeoutBuilder[T]{
		builder: b.clone(),
		timeout: timeout,
	}
}

// Find executa a query com timeout.
func (tb *TimeoutBuilder[T]) Find(ctx context.Context) ([]T, error) {
	ctx, cancel := context.WithTimeout(ctx, tb.timeout)
	defer cancel()
	return tb.builder.Find(ctx)
}

// First retorna o primeiro resultado com timeout.
func (tb *TimeoutBuilder[T]) First(ctx context.Context) (T, error) {
	ctx, cancel := context.WithTimeout(ctx, tb.timeout)
	defer cancel()
	return tb.builder.First(ctx)
}

// Count retorna a contagem com timeout.
func (tb *TimeoutBuilder[T]) Count(ctx context.Context) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, tb.timeout)
	defer cancel()
	return tb.builder.Count(ctx)
}

// DefaultQueryTimeout é o timeout padrão para queries (0 = sem timeout).
var DefaultQueryTimeout time.Duration = 0

// SetDefaultQueryTimeout configura o timeout padrão global.
func SetDefaultQueryTimeout(timeout time.Duration) {
	DefaultQueryTimeout = timeout
}
