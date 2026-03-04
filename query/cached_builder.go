package query

import (
	"context"
	"encoding/json"
	"time"

	"github.com/GabrielOnRails/genus/cache"
	"github.com/GabrielOnRails/genus/core"
)

// CachedBuilder é um wrapper do Builder que adiciona suporte a cache.
type CachedBuilder[T any] struct {
	builder     *Builder[T]
	cache       cache.Cache
	cacheConfig cache.CacheConfig
	ttl         time.Duration
	skipCache   bool
}

// WithCache adiciona cache ao builder.
// O cache é usado para armazenar resultados de queries SELECT.
//
// Exemplo:
//
//	cache := cache.NewInMemoryCache(10000)
//	users, _ := genus.Table[User](db).
//	    WithCache(cache, cache.DefaultCacheConfig()).
//	    Where(UserFields.IsActive.Eq(true)).
//	    Find(ctx)
func (b *Builder[T]) WithCache(c cache.Cache, config cache.CacheConfig) *CachedBuilder[T] {
	return &CachedBuilder[T]{
		builder:     b,
		cache:       c,
		cacheConfig: config,
		ttl:         config.DefaultTTL,
		skipCache:   !config.Enabled,
	}
}

// clone cria uma cópia do CachedBuilder.
func (cb *CachedBuilder[T]) clone() *CachedBuilder[T] {
	return &CachedBuilder[T]{
		builder:     cb.builder.clone(),
		cache:       cb.cache,
		cacheConfig: cb.cacheConfig,
		ttl:         cb.ttl,
		skipCache:   cb.skipCache,
	}
}

// TTL define o TTL específico para esta query.
// Sobrescreve o DefaultTTL da configuração.
func (cb *CachedBuilder[T]) TTL(ttl time.Duration) *CachedBuilder[T] {
	newBuilder := cb.clone()
	newBuilder.ttl = ttl
	return newBuilder
}

// NoCache desabilita o cache para esta query específica.
func (cb *CachedBuilder[T]) NoCache() *CachedBuilder[T] {
	newBuilder := cb.clone()
	newBuilder.skipCache = true
	return newBuilder
}

// Where adiciona uma condição WHERE.
func (cb *CachedBuilder[T]) Where(condition interface{}) *CachedBuilder[T] {
	newBuilder := cb.clone()
	newBuilder.builder = cb.builder.Where(condition)
	return newBuilder
}

// OrderByAsc adiciona ORDER BY ASC.
func (cb *CachedBuilder[T]) OrderByAsc(column string) *CachedBuilder[T] {
	newBuilder := cb.clone()
	newBuilder.builder = cb.builder.OrderByAsc(column)
	return newBuilder
}

// OrderByDesc adiciona ORDER BY DESC.
func (cb *CachedBuilder[T]) OrderByDesc(column string) *CachedBuilder[T] {
	newBuilder := cb.clone()
	newBuilder.builder = cb.builder.OrderByDesc(column)
	return newBuilder
}

// Limit define o LIMIT.
func (cb *CachedBuilder[T]) Limit(limit int) *CachedBuilder[T] {
	newBuilder := cb.clone()
	newBuilder.builder = cb.builder.Limit(limit)
	return newBuilder
}

// Offset define o OFFSET.
func (cb *CachedBuilder[T]) Offset(offset int) *CachedBuilder[T] {
	newBuilder := cb.clone()
	newBuilder.builder = cb.builder.Offset(offset)
	return newBuilder
}

// Select define as colunas a serem selecionadas.
func (cb *CachedBuilder[T]) Select(columns ...string) *CachedBuilder[T] {
	newBuilder := cb.clone()
	newBuilder.builder = cb.builder.Select(columns...)
	return newBuilder
}

// WithTrashed inclui registros soft-deleted.
func (cb *CachedBuilder[T]) WithTrashed() *CachedBuilder[T] {
	newBuilder := cb.clone()
	newBuilder.builder = cb.builder.WithTrashed()
	return newBuilder
}

// OnlyTrashed retorna apenas registros soft-deleted.
func (cb *CachedBuilder[T]) OnlyTrashed() *CachedBuilder[T] {
	newBuilder := cb.clone()
	newBuilder.builder = cb.builder.OnlyTrashed()
	return newBuilder
}

// Preload especifica relacionamentos para eager loading.
func (cb *CachedBuilder[T]) Preload(relation string) *CachedBuilder[T] {
	newBuilder := cb.clone()
	newBuilder.builder = cb.builder.Preload(relation)
	return newBuilder
}

// Find executa a query e retorna um slice de T.
// Se o cache estiver habilitado, tenta buscar do cache primeiro.
func (cb *CachedBuilder[T]) Find(ctx context.Context) ([]T, error) {
	// Se cache desabilitado, executa diretamente
	if cb.skipCache || cb.cache == nil {
		return cb.builder.Find(ctx)
	}

	// Gera chave do cache
	query, args := cb.builder.buildSelectQuery()
	cacheKey := cache.QueryCacheKey(cb.cacheConfig.KeyPrefix, cb.builder.tableName, query, args)

	// Tenta buscar do cache
	cachedData, err := cb.cache.Get(ctx, cacheKey)
	if err != nil {
		// Erro no cache, executa query normalmente
		return cb.builder.Find(ctx)
	}

	if cachedData != nil {
		// Cache hit - deserializa e retorna
		var results []T
		if err := json.Unmarshal(cachedData, &results); err == nil {
			return results, nil
		}
		// Erro na deserialização, executa query normalmente
	}

	// Cache miss - executa query
	results, err := cb.builder.Find(ctx)
	if err != nil {
		return nil, err
	}

	// Armazena no cache
	if data, err := json.Marshal(results); err == nil {
		cb.cache.Set(ctx, cacheKey, data, cb.ttl)
	}

	return results, nil
}

// First retorna o primeiro resultado ou erro se não encontrado.
func (cb *CachedBuilder[T]) First(ctx context.Context) (T, error) {
	// Usa Limit(1) e Find para aproveitar o cache
	results, err := cb.Limit(1).Find(ctx)

	var zero T
	if err != nil {
		return zero, err
	}

	if len(results) == 0 {
		return zero, core.ErrNotFound
	}

	return results[0], nil
}

// Count retorna a contagem de registros.
// Count não usa cache por padrão pois geralmente precisa de dados atualizados.
func (cb *CachedBuilder[T]) Count(ctx context.Context) (int64, error) {
	return cb.builder.Count(ctx)
}

// Aggregate retorna um AggregateBuilder para operações de agregação.
func (cb *CachedBuilder[T]) Aggregate() *AggregateBuilder[T] {
	return cb.builder.Aggregate()
}
