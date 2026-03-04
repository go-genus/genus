package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Cache é a interface para implementações de cache.
// Permite plugar diferentes backends (in-memory, Redis, etc).
type Cache interface {
	// Get retorna o valor cacheado para a chave.
	// Retorna nil se a chave não existir ou estiver expirada.
	Get(ctx context.Context, key string) ([]byte, error)

	// Set armazena um valor no cache com TTL.
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error

	// Delete remove uma chave do cache.
	Delete(ctx context.Context, key string) error

	// DeleteByPrefix remove todas as chaves com o prefixo.
	// Útil para invalidar cache de uma tabela inteira.
	DeleteByPrefix(ctx context.Context, prefix string) error

	// Clear remove todas as entradas do cache.
	Clear(ctx context.Context) error

	// Stats retorna estatísticas do cache.
	Stats() CacheStats
}

// CacheStats contém estatísticas do cache.
type CacheStats struct {
	Hits       int64
	Misses     int64
	Sets       int64
	Deletes    int64
	Evictions  int64
	Size       int64
	HitRate    float64
}

// CacheConfig contém configurações do cache.
type CacheConfig struct {
	// Enabled indica se o cache está habilitado.
	Enabled bool

	// DefaultTTL é o tempo de vida padrão para entradas do cache.
	// Default: 5 minutos
	DefaultTTL time.Duration

	// MaxEntries é o número máximo de entradas no cache (para in-memory).
	// Default: 10000
	MaxEntries int

	// KeyPrefix é o prefixo para todas as chaves do cache.
	// Útil para namespace em ambientes compartilhados.
	KeyPrefix string
}

// DefaultCacheConfig retorna a configuração padrão do cache.
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		Enabled:    true,
		DefaultTTL: 5 * time.Minute,
		MaxEntries: 10000,
		KeyPrefix:  "genus:",
	}
}

// QueryCacheKey gera uma chave de cache única para uma query.
// A chave é baseada em: tabela, query SQL e argumentos.
func QueryCacheKey(prefix, table, query string, args []interface{}) string {
	hasher := sha256.New()
	hasher.Write([]byte(table))
	hasher.Write([]byte(query))

	// Serializa argumentos para o hash
	for _, arg := range args {
		argBytes, _ := json.Marshal(arg)
		hasher.Write(argBytes)
	}

	hash := hex.EncodeToString(hasher.Sum(nil))[:16]
	return fmt.Sprintf("%s%s:%s", prefix, table, hash)
}

// TableCachePrefix retorna o prefixo de cache para uma tabela.
// Usado para invalidar todas as queries de uma tabela.
func TableCachePrefix(prefix, table string) string {
	return fmt.Sprintf("%s%s:", prefix, table)
}

// cacheEntry representa uma entrada no cache in-memory.
type cacheEntry struct {
	value     []byte
	expiresAt time.Time
}

// InMemoryCache é uma implementação de cache em memória com LRU eviction.
type InMemoryCache struct {
	mu         sync.RWMutex
	entries    map[string]*cacheEntry
	maxEntries int
	stats      CacheStats
	order      []string // Para LRU simples
}

// NewInMemoryCache cria um novo cache em memória.
func NewInMemoryCache(maxEntries int) *InMemoryCache {
	if maxEntries <= 0 {
		maxEntries = 10000
	}
	return &InMemoryCache{
		entries:    make(map[string]*cacheEntry),
		maxEntries: maxEntries,
		order:      make([]string, 0, maxEntries),
	}
}

// Get retorna o valor cacheado para a chave.
func (c *InMemoryCache) Get(ctx context.Context, key string) ([]byte, error) {
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()

	if !ok {
		c.mu.Lock()
		c.stats.Misses++
		c.mu.Unlock()
		return nil, nil
	}

	// Verifica expiração
	if time.Now().After(entry.expiresAt) {
		c.Delete(ctx, key)
		c.mu.Lock()
		c.stats.Misses++
		c.mu.Unlock()
		return nil, nil
	}

	c.mu.Lock()
	c.stats.Hits++
	c.mu.Unlock()

	return entry.value, nil
}

// Set armazena um valor no cache.
func (c *InMemoryCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict se necessário
	for len(c.entries) >= c.maxEntries {
		c.evictOldest()
	}

	c.entries[key] = &cacheEntry{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
	c.order = append(c.order, key)
	c.stats.Sets++
	c.stats.Size = int64(len(c.entries))

	return nil
}

// evictOldest remove a entrada mais antiga (LRU simples).
func (c *InMemoryCache) evictOldest() {
	if len(c.order) == 0 {
		return
	}

	oldest := c.order[0]
	c.order = c.order[1:]
	delete(c.entries, oldest)
	c.stats.Evictions++
}

// Delete remove uma chave do cache.
func (c *InMemoryCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.entries[key]; ok {
		delete(c.entries, key)
		c.stats.Deletes++
		c.stats.Size = int64(len(c.entries))
	}

	return nil
}

// DeleteByPrefix remove todas as chaves com o prefixo.
func (c *InMemoryCache) DeleteByPrefix(ctx context.Context, prefix string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	keysToDelete := make([]string, 0)
	for key := range c.entries {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			keysToDelete = append(keysToDelete, key)
		}
	}

	for _, key := range keysToDelete {
		delete(c.entries, key)
		c.stats.Deletes++
	}

	c.stats.Size = int64(len(c.entries))
	return nil
}

// Clear remove todas as entradas do cache.
func (c *InMemoryCache) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*cacheEntry)
	c.order = make([]string, 0, c.maxEntries)
	c.stats.Size = 0

	return nil
}

// Stats retorna estatísticas do cache.
func (c *InMemoryCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := c.stats
	total := stats.Hits + stats.Misses
	if total > 0 {
		stats.HitRate = float64(stats.Hits) / float64(total)
	}

	return stats
}

// NoOpCache é uma implementação de cache que não faz nada.
// Útil para desabilitar cache sem modificar código.
type NoOpCache struct{}

// NewNoOpCache cria um novo cache no-op.
func NewNoOpCache() *NoOpCache {
	return &NoOpCache{}
}

func (c *NoOpCache) Get(ctx context.Context, key string) ([]byte, error) {
	return nil, nil
}

func (c *NoOpCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return nil
}

func (c *NoOpCache) Delete(ctx context.Context, key string) error {
	return nil
}

func (c *NoOpCache) DeleteByPrefix(ctx context.Context, prefix string) error {
	return nil
}

func (c *NoOpCache) Clear(ctx context.Context) error {
	return nil
}

func (c *NoOpCache) Stats() CacheStats {
	return CacheStats{}
}
