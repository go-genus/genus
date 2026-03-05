package sharding

import (
	"context"
	"database/sql"
	"fmt"
	"hash/fnv"
	"sync"
	"sync/atomic"
)

// ShardKey representa uma chave de sharding.
type ShardKey interface {
	// Value retorna o valor usado para calcular o shard.
	Value() interface{}
}

// Int64ShardKey é uma chave de sharding baseada em int64.
type Int64ShardKey int64

func (k Int64ShardKey) Value() interface{} {
	return int64(k)
}

// StringShardKey é uma chave de sharding baseada em string.
type StringShardKey string

func (k StringShardKey) Value() interface{} {
	return string(k)
}

// ShardStrategy define como as chaves são mapeadas para shards.
type ShardStrategy interface {
	// GetShard retorna o índice do shard para uma chave.
	GetShard(key ShardKey, numShards int) int
}

// ModuloStrategy usa módulo simples para distribuir chaves.
type ModuloStrategy struct{}

func (s ModuloStrategy) GetShard(key ShardKey, numShards int) int {
	switch v := key.Value().(type) {
	case int64:
		return int(v % int64(numShards))
	case int:
		return v % numShards
	case string:
		h := fnv.New32a()
		h.Write([]byte(v))
		return int(h.Sum32()) % numShards
	default:
		// Fallback para string
		h := fnv.New32a()
		h.Write([]byte(fmt.Sprintf("%v", v)))
		return int(h.Sum32()) % numShards
	}
}

// ConsistentHashStrategy usa consistent hashing para distribuição.
// Minimiza redistribuição quando shards são adicionados/removidos.
type ConsistentHashStrategy struct {
	replicas int
	ring     []uint32
	nodes    map[uint32]int // hash -> shard index
	mu       sync.RWMutex
}

// NewConsistentHashStrategy cria uma estratégia de consistent hashing.
func NewConsistentHashStrategy(replicas int) *ConsistentHashStrategy {
	if replicas <= 0 {
		replicas = 100
	}
	return &ConsistentHashStrategy{
		replicas: replicas,
		nodes:    make(map[uint32]int),
	}
}

// Build constrói o ring de consistent hashing.
func (s *ConsistentHashStrategy) Build(numShards int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ring = make([]uint32, 0, numShards*s.replicas)
	s.nodes = make(map[uint32]int)

	for i := 0; i < numShards; i++ {
		for j := 0; j < s.replicas; j++ {
			h := fnv.New32a()
			h.Write([]byte(fmt.Sprintf("shard-%d-%d", i, j)))
			hash := h.Sum32()
			s.ring = append(s.ring, hash)
			s.nodes[hash] = i
		}
	}

	// Sort the ring
	s.sortRing()
}

func (s *ConsistentHashStrategy) sortRing() {
	// Simple insertion sort (ring is relatively small)
	for i := 1; i < len(s.ring); i++ {
		key := s.ring[i]
		j := i - 1
		for j >= 0 && s.ring[j] > key {
			s.ring[j+1] = s.ring[j]
			j--
		}
		s.ring[j+1] = key
	}
}

func (s *ConsistentHashStrategy) GetShard(key ShardKey, numShards int) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.ring) == 0 {
		// Fallback para módulo se ring não foi construído
		return ModuloStrategy{}.GetShard(key, numShards)
	}

	h := fnv.New32a()
	h.Write([]byte(fmt.Sprintf("%v", key.Value())))
	hash := h.Sum32()

	// Binary search para encontrar o primeiro hash >= key hash
	idx := s.binarySearch(hash)
	if idx >= len(s.ring) {
		idx = 0
	}

	return s.nodes[s.ring[idx]]
}

func (s *ConsistentHashStrategy) binarySearch(hash uint32) int {
	low, high := 0, len(s.ring)-1
	for low <= high {
		mid := (low + high) / 2
		if s.ring[mid] < hash {
			low = mid + 1
		} else {
			high = mid - 1
		}
	}
	return low
}

// sqlOpen é a função usada para abrir conexões SQL.
// Pode ser substituída em testes para simular falhas.
var sqlOpen = sql.Open

// Shard representa uma conexão de shard.
type Shard struct {
	Index int
	DB    *sql.DB
	DSN   string
}

// ShardConfig configura o sharding.
type ShardConfig struct {
	// DSNs é a lista de DSNs para cada shard.
	DSNs []string

	// Strategy é a estratégia de sharding (default: ModuloStrategy).
	Strategy ShardStrategy

	// KeyExtractor extrai a shard key de um contexto.
	// Se nil, usa a chave definida via WithShardKey.
	KeyExtractor func(ctx context.Context) (ShardKey, bool)
}

// ShardManager gerencia múltiplos shards.
type ShardManager struct {
	shards       []*Shard
	strategy     ShardStrategy
	keyExtractor func(ctx context.Context) (ShardKey, bool)
	counter      uint64 // para round-robin quando não há shard key
}

// NewShardManager cria um novo gerenciador de shards.
func NewShardManager(driver string, config ShardConfig) (*ShardManager, error) {
	if len(config.DSNs) == 0 {
		return nil, fmt.Errorf("at least one shard DSN is required")
	}

	shards := make([]*Shard, len(config.DSNs))
	for i, dsn := range config.DSNs {
		db, err := sqlOpen(driver, dsn)
		if err != nil {
			// Fecha conexões já abertas
			for j := 0; j < i; j++ {
				shards[j].DB.Close()
			}
			return nil, fmt.Errorf("failed to connect to shard %d: %w", i, err)
		}

		if err := db.Ping(); err != nil {
			db.Close()
			for j := 0; j < i; j++ {
				shards[j].DB.Close()
			}
			return nil, fmt.Errorf("failed to ping shard %d: %w", i, err)
		}

		shards[i] = &Shard{
			Index: i,
			DB:    db,
			DSN:   dsn,
		}
	}

	strategy := config.Strategy
	if strategy == nil {
		strategy = ModuloStrategy{}
	}

	// Se for consistent hash, constrói o ring
	if ch, ok := strategy.(*ConsistentHashStrategy); ok {
		ch.Build(len(shards))
	}

	return &ShardManager{
		shards:       shards,
		strategy:     strategy,
		keyExtractor: config.KeyExtractor,
	}, nil
}

// GetShard retorna o shard para uma chave específica.
func (sm *ShardManager) GetShard(key ShardKey) *Shard {
	idx := sm.strategy.GetShard(key, len(sm.shards))
	return sm.shards[idx]
}

// GetShardByIndex retorna o shard pelo índice.
func (sm *ShardManager) GetShardByIndex(idx int) *Shard {
	if idx < 0 || idx >= len(sm.shards) {
		return nil
	}
	return sm.shards[idx]
}

// GetShardFromContext extrai a shard key do contexto e retorna o shard.
func (sm *ShardManager) GetShardFromContext(ctx context.Context) *Shard {
	// Primeiro tenta o key extractor customizado
	if sm.keyExtractor != nil {
		if key, ok := sm.keyExtractor(ctx); ok {
			return sm.GetShard(key)
		}
	}

	// Depois tenta a chave do contexto
	if key, ok := ShardKeyFromContext(ctx); ok {
		return sm.GetShard(key)
	}

	// Fallback para round-robin
	idx := int(atomic.AddUint64(&sm.counter, 1) % uint64(len(sm.shards)))
	return sm.shards[idx]
}

// AllShards retorna todos os shards.
func (sm *ShardManager) AllShards() []*Shard {
	return sm.shards
}

// NumShards retorna o número de shards.
func (sm *ShardManager) NumShards() int {
	return len(sm.shards)
}

// Close fecha todas as conexões de shard.
func (sm *ShardManager) Close() error {
	var lastErr error
	for _, shard := range sm.shards {
		if err := shard.DB.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// Context keys para shard routing.
type shardKeyContextKey struct{}

// WithShardKey adiciona uma shard key ao contexto.
func WithShardKey(ctx context.Context, key ShardKey) context.Context {
	return context.WithValue(ctx, shardKeyContextKey{}, key)
}

// ShardKeyFromContext extrai a shard key do contexto.
func ShardKeyFromContext(ctx context.Context) (ShardKey, bool) {
	key, ok := ctx.Value(shardKeyContextKey{}).(ShardKey)
	return key, ok
}

// ShardedResult representa o resultado de uma query em múltiplos shards.
type ShardedResult[T any] struct {
	Results    []T
	ShardIndex int
	Error      error
}

// MergeResults combina resultados de múltiplos shards.
func MergeResults[T any](results []ShardedResult[T]) ([]T, error) {
	var merged []T
	var errors []error

	for _, r := range results {
		if r.Error != nil {
			errors = append(errors, fmt.Errorf("shard %d: %w", r.ShardIndex, r.Error))
			continue
		}
		merged = append(merged, r.Results...)
	}

	if len(errors) > 0 {
		// Retorna o primeiro erro
		return merged, errors[0]
	}

	return merged, nil
}
