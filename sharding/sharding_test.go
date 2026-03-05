package sharding

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// fakeDriver é um driver SQL fake para testar branches de erro.
type fakeDriver struct {
	openFunc func(name string) (driver.Conn, error)
}

func (d *fakeDriver) Open(name string) (driver.Conn, error) {
	if d.openFunc != nil {
		return d.openFunc(name)
	}
	return &fakeConn{}, nil
}

type fakeConn struct {
	failClose bool
}

func (c *fakeConn) Prepare(query string) (driver.Stmt, error) {
	return nil, fmt.Errorf("not implemented")
}

func (c *fakeConn) Close() error {
	if c.failClose {
		return fmt.Errorf("fake close error")
	}
	return nil
}

func (c *fakeConn) Begin() (driver.Tx, error) {
	return nil, fmt.Errorf("not implemented")
}

var (
	fakeFailSecondDriver = &fakeDriver{}
	fakeCloseErrDriver   = &fakeDriver{
		openFunc: func(name string) (driver.Conn, error) {
			return &fakeConn{failClose: true}, nil
		},
	}
)

func init() {
	sql.Register("fake_fail_second", fakeFailSecondDriver)
	sql.Register("fake_close_err", fakeCloseErrDriver)
}

// intShardKey retorna int (não int64) para cobrir o case int no ModuloStrategy.
type intShardKey int

func (k intShardKey) Value() interface{} {
	return int(k)
}

// floatShardKey retorna float64 para cobrir o case default no ModuloStrategy.
type floatShardKey float64

func (k floatShardKey) Value() interface{} {
	return float64(k)
}

func TestModuloStrategy(t *testing.T) {
	strategy := ModuloStrategy{}

	// Test int64 keys
	t.Run("int64 keys", func(t *testing.T) {
		tests := []struct {
			key       int64
			numShards int
			expected  int
		}{
			{0, 3, 0},
			{1, 3, 1},
			{2, 3, 2},
			{3, 3, 0},
			{10, 3, 1},
			{100, 5, 0},
		}

		for _, tt := range tests {
			result := strategy.GetShard(Int64ShardKey(tt.key), tt.numShards)
			if result != tt.expected {
				t.Errorf("GetShard(%d, %d) = %d, want %d", tt.key, tt.numShards, result, tt.expected)
			}
		}
	})

	// Test string key returns consistent result
	t.Run("string key consistency", func(t *testing.T) {
		key := StringShardKey("user-123")
		shard1 := strategy.GetShard(key, 5)
		shard2 := strategy.GetShard(key, 5)
		if shard1 != shard2 {
			t.Errorf("GetShard should return consistent results for same key")
		}
	})

	// Test string key returns valid shard index
	t.Run("string key valid index", func(t *testing.T) {
		key := StringShardKey("tenant-abc")
		shard := strategy.GetShard(key, 5)
		if shard < 0 || shard >= 5 {
			t.Errorf("GetShard should return valid index, got %d", shard)
		}
	})
}

func TestConsistentHashStrategy(t *testing.T) {
	strategy := NewConsistentHashStrategy(100)
	strategy.Build(5)

	// Test consistency
	t.Run("consistency", func(t *testing.T) {
		key := Int64ShardKey(12345)
		shard1 := strategy.GetShard(key, 5)
		shard2 := strategy.GetShard(key, 5)
		if shard1 != shard2 {
			t.Errorf("Consistent hash should return same shard for same key")
		}
	})

	// Test distribution
	t.Run("distribution", func(t *testing.T) {
		shardCounts := make(map[int]int)
		for i := 0; i < 1000; i++ {
			shard := strategy.GetShard(Int64ShardKey(int64(i)), 5)
			shardCounts[shard]++
		}

		// Each shard should have some keys (rough distribution check)
		for i := 0; i < 5; i++ {
			if shardCounts[i] == 0 {
				t.Errorf("Shard %d has no keys, distribution may be broken", i)
			}
		}
	})

	// Test without building ring (fallback to modulo)
	t.Run("fallback to modulo", func(t *testing.T) {
		emptyStrategy := NewConsistentHashStrategy(100)
		// Don't build the ring

		key := Int64ShardKey(10)
		shard := emptyStrategy.GetShard(key, 3)
		// Should fallback to modulo: 10 % 3 = 1
		if shard != 1 {
			t.Errorf("Expected shard 1 (fallback to modulo), got %d", shard)
		}
	})
}

func TestContextShardKey(t *testing.T) {
	ctx := context.Background()

	// Test without shard key
	t.Run("no shard key", func(t *testing.T) {
		_, ok := ShardKeyFromContext(ctx)
		if ok {
			t.Error("Expected no shard key in empty context")
		}
	})

	// Test with int64 shard key
	t.Run("int64 shard key", func(t *testing.T) {
		ctx := WithShardKey(ctx, Int64ShardKey(42))
		key, ok := ShardKeyFromContext(ctx)
		if !ok {
			t.Error("Expected shard key in context")
		}
		if v, ok := key.Value().(int64); !ok || v != 42 {
			t.Errorf("Expected shard key value 42, got %v", key.Value())
		}
	})

	// Test with string shard key
	t.Run("string shard key", func(t *testing.T) {
		ctx := WithShardKey(ctx, StringShardKey("tenant-abc"))
		key, ok := ShardKeyFromContext(ctx)
		if !ok {
			t.Error("Expected shard key in context")
		}
		if v, ok := key.Value().(string); !ok || v != "tenant-abc" {
			t.Errorf("Expected shard key value 'tenant-abc', got %v", key.Value())
		}
	})
}

func TestShardKeyTypes(t *testing.T) {
	t.Run("Int64ShardKey", func(t *testing.T) {
		key := Int64ShardKey(123)
		if key.Value() != int64(123) {
			t.Errorf("Expected 123, got %v", key.Value())
		}
	})

	t.Run("StringShardKey", func(t *testing.T) {
		key := StringShardKey("test")
		if key.Value() != "test" {
			t.Errorf("Expected 'test', got %v", key.Value())
		}
	})
}

func TestMergeResults(t *testing.T) {
	t.Run("merge successful results", func(t *testing.T) {
		results := []ShardedResult[int]{
			{Results: []int{1, 2, 3}, ShardIndex: 0},
			{Results: []int{4, 5}, ShardIndex: 1},
			{Results: []int{6}, ShardIndex: 2},
		}

		merged, err := MergeResults(results)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if len(merged) != 6 {
			t.Errorf("Expected 6 results, got %d", len(merged))
		}
	})

	t.Run("merge with error", func(t *testing.T) {
		results := []ShardedResult[int]{
			{Results: []int{1, 2}, ShardIndex: 0},
			{Results: nil, ShardIndex: 1, Error: context.DeadlineExceeded},
		}

		merged, err := MergeResults(results)
		if err == nil {
			t.Error("Expected error from failed shard")
		}
		// Should still have results from successful shard
		if len(merged) != 2 {
			t.Errorf("Expected 2 results from successful shard, got %d", len(merged))
		}
	})

	t.Run("merge empty results", func(t *testing.T) {
		results := []ShardedResult[int]{}

		merged, err := MergeResults(results)
		if err != nil {
			t.Errorf("Expected no error for empty results, got %v", err)
		}
		if len(merged) != 0 {
			t.Errorf("Expected 0 results, got %d", len(merged))
		}
	})
}

func TestModuloStrategy_IntKey(t *testing.T) {
	strategy := ModuloStrategy{}

	key := intShardKey(10)
	shard := strategy.GetShard(key, 3)
	if shard != 1 {
		t.Errorf("Expected shard 1 for int key 10 %% 3, got %d", shard)
	}

	key2 := intShardKey(0)
	shard2 := strategy.GetShard(key2, 5)
	if shard2 != 0 {
		t.Errorf("Expected shard 0 for int key 0 %% 5, got %d", shard2)
	}
}

func TestModuloStrategy_DefaultKey(t *testing.T) {
	strategy := ModuloStrategy{}

	// floatShardKey retorna float64, que cai no default case
	key := floatShardKey(3.14)
	shard := strategy.GetShard(key, 5)
	if shard < 0 || shard >= 5 {
		t.Errorf("Expected valid shard index for float key, got %d", shard)
	}

	// Deve ser consistente
	shard2 := strategy.GetShard(key, 5)
	if shard != shard2 {
		t.Errorf("Expected consistent shard for same key, got %d and %d", shard, shard2)
	}
}

func TestNewConsistentHashStrategy_DefaultReplicas(t *testing.T) {
	// Quando replicas <= 0, deve usar 100 como default
	strategy := NewConsistentHashStrategy(0)
	if strategy.replicas != 100 {
		t.Errorf("Expected 100 replicas for 0 input, got %d", strategy.replicas)
	}

	strategy2 := NewConsistentHashStrategy(-5)
	if strategy2.replicas != 100 {
		t.Errorf("Expected 100 replicas for -5 input, got %d", strategy2.replicas)
	}
}

func TestConsistentHashStrategy_WrapAround(t *testing.T) {
	// Cria estratégia com poucos replicas para facilitar o wrap-around
	strategy := NewConsistentHashStrategy(1)
	strategy.Build(2)

	// Testa muitas chaves para garantir que alguma causa wrap-around
	// (idx >= len(s.ring) faz idx = 0)
	seen := make(map[int]bool)
	for i := 0; i < 1000; i++ {
		shard := strategy.GetShard(Int64ShardKey(int64(i)), 2)
		seen[shard] = true
		if shard < 0 || shard >= 2 {
			t.Errorf("Expected shard in [0, 2), got %d for key %d", shard, i)
		}
	}
}

// helper para criar ShardManager com SQLite in-memory
func newTestShardManager(t *testing.T, numShards int, strategy ShardStrategy) *ShardManager {
	t.Helper()

	dsns := make([]string, numShards)
	for i := range dsns {
		dsns[i] = ":memory:"
	}

	sm, err := NewShardManager("sqlite3", ShardConfig{
		DSNs:     dsns,
		Strategy: strategy,
	})
	if err != nil {
		t.Fatalf("Failed to create ShardManager: %v", err)
	}

	t.Cleanup(func() {
		sm.Close()
	})

	return sm
}

func TestNewShardManager(t *testing.T) {
	t.Run("with default strategy", func(t *testing.T) {
		sm := newTestShardManager(t, 3, nil)
		if sm.NumShards() != 3 {
			t.Errorf("Expected 3 shards, got %d", sm.NumShards())
		}
	})

	t.Run("with modulo strategy", func(t *testing.T) {
		sm := newTestShardManager(t, 2, ModuloStrategy{})
		if sm.NumShards() != 2 {
			t.Errorf("Expected 2 shards, got %d", sm.NumShards())
		}
	})

	t.Run("with consistent hash strategy", func(t *testing.T) {
		sm := newTestShardManager(t, 3, NewConsistentHashStrategy(50))
		if sm.NumShards() != 3 {
			t.Errorf("Expected 3 shards, got %d", sm.NumShards())
		}
	})

	t.Run("empty DSNs returns error", func(t *testing.T) {
		_, err := NewShardManager("sqlite3", ShardConfig{
			DSNs: []string{},
		})
		if err == nil {
			t.Error("Expected error for empty DSNs")
		}
	})

	t.Run("invalid driver returns error", func(t *testing.T) {
		_, err := NewShardManager("invalid_driver", ShardConfig{
			DSNs: []string{"some_dsn"},
		})
		if err == nil {
			t.Error("Expected error for invalid driver")
		}
	})
}

func TestShardManager_GetShard(t *testing.T) {
	sm := newTestShardManager(t, 3, nil)

	shard := sm.GetShard(Int64ShardKey(0))
	if shard.Index != 0 {
		t.Errorf("Expected shard index 0 for key 0, got %d", shard.Index)
	}

	shard1 := sm.GetShard(Int64ShardKey(1))
	if shard1.Index != 1 {
		t.Errorf("Expected shard index 1 for key 1, got %d", shard1.Index)
	}

	shard2 := sm.GetShard(Int64ShardKey(2))
	if shard2.Index != 2 {
		t.Errorf("Expected shard index 2 for key 2, got %d", shard2.Index)
	}
}

func TestShardManager_GetShardByIndex(t *testing.T) {
	sm := newTestShardManager(t, 3, nil)

	t.Run("valid index", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			shard := sm.GetShardByIndex(i)
			if shard == nil {
				t.Errorf("Expected shard for index %d, got nil", i)
			} else if shard.Index != i {
				t.Errorf("Expected shard.Index = %d, got %d", i, shard.Index)
			}
		}
	})

	t.Run("negative index returns nil", func(t *testing.T) {
		shard := sm.GetShardByIndex(-1)
		if shard != nil {
			t.Error("Expected nil for negative index")
		}
	})

	t.Run("out of bounds index returns nil", func(t *testing.T) {
		shard := sm.GetShardByIndex(3)
		if shard != nil {
			t.Error("Expected nil for out of bounds index")
		}
	})
}

func TestShardManager_GetShardFromContext(t *testing.T) {
	sm := newTestShardManager(t, 3, nil)

	t.Run("with shard key in context", func(t *testing.T) {
		ctx := WithShardKey(context.Background(), Int64ShardKey(1))
		shard := sm.GetShardFromContext(ctx)
		if shard.Index != 1 {
			t.Errorf("Expected shard 1, got %d", shard.Index)
		}
	})

	t.Run("with custom key extractor", func(t *testing.T) {
		sm2, err := NewShardManager("sqlite3", ShardConfig{
			DSNs:     []string{":memory:", ":memory:", ":memory:"},
			Strategy: ModuloStrategy{},
			KeyExtractor: func(ctx context.Context) (ShardKey, bool) {
				return Int64ShardKey(2), true
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		defer sm2.Close()

		shard := sm2.GetShardFromContext(context.Background())
		if shard.Index != 2 {
			t.Errorf("Expected shard 2 from key extractor, got %d", shard.Index)
		}
	})

	t.Run("key extractor returns false falls through to context", func(t *testing.T) {
		sm2, err := NewShardManager("sqlite3", ShardConfig{
			DSNs:     []string{":memory:", ":memory:", ":memory:"},
			Strategy: ModuloStrategy{},
			KeyExtractor: func(ctx context.Context) (ShardKey, bool) {
				return nil, false
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		defer sm2.Close()

		ctx := WithShardKey(context.Background(), Int64ShardKey(1))
		shard := sm2.GetShardFromContext(ctx)
		if shard.Index != 1 {
			t.Errorf("Expected shard 1 from context key, got %d", shard.Index)
		}
	})

	t.Run("round-robin fallback", func(t *testing.T) {
		ctx := context.Background() // sem shard key

		// Round-robin deve retornar shards diferentes sequencialmente
		shard1 := sm.GetShardFromContext(ctx)
		shard2 := sm.GetShardFromContext(ctx)
		// Apenas verifica que retorna shards válidos
		if shard1.Index < 0 || shard1.Index >= 3 {
			t.Errorf("Expected valid shard index, got %d", shard1.Index)
		}
		if shard2.Index < 0 || shard2.Index >= 3 {
			t.Errorf("Expected valid shard index, got %d", shard2.Index)
		}
	})
}

func TestShardManager_AllShards(t *testing.T) {
	sm := newTestShardManager(t, 3, nil)

	shards := sm.AllShards()
	if len(shards) != 3 {
		t.Errorf("Expected 3 shards, got %d", len(shards))
	}

	for i, shard := range shards {
		if shard.Index != i {
			t.Errorf("Expected shard index %d, got %d", i, shard.Index)
		}
		if shard.DB == nil {
			t.Errorf("Expected non-nil DB for shard %d", i)
		}
	}
}

func TestShardManager_NumShards(t *testing.T) {
	sm := newTestShardManager(t, 5, nil)
	if sm.NumShards() != 5 {
		t.Errorf("Expected 5 shards, got %d", sm.NumShards())
	}
}

func TestShardManager_Close(t *testing.T) {
	sm, err := NewShardManager("sqlite3", ShardConfig{
		DSNs: []string{":memory:", ":memory:"},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = sm.Close()
	if err != nil {
		t.Errorf("Expected no error on close, got %v", err)
	}

	// Verificar que DBs foram fechadas (Ping deve falhar)
	for i, shard := range sm.shards {
		if err := shard.DB.Ping(); err == nil {
			t.Errorf("Expected ping to fail on closed shard %d", i)
		}
	}
}

func TestNewShardManager_PingFailure(t *testing.T) {
	// Usar um DSN que sql.Open aceita mas Ping falha
	// Para sqlite3, um path inválido como diretório causa falha no ping
	_, err := NewShardManager("sqlite3", ShardConfig{
		DSNs: []string{":memory:", "/nonexistent/path/to/db.sqlite?mode=ro"},
	})
	if err == nil {
		t.Error("Expected error when ping fails")
	}
}

func TestShardManager_CloseWithError(t *testing.T) {
	sm, err := NewShardManager("sqlite3", ShardConfig{
		DSNs: []string{":memory:", ":memory:"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Fecha o primeiro DB manualmente para causar duplo close
	sm.shards[0].DB.Close()
	// O Close do manager vai tentar fechar novamente, mas sql.DB.Close()
	// nao retorna erro em double close no sqlite3. Isso ao menos exercita o path.
	_ = sm.Close()
}

func TestShard_DSN(t *testing.T) {
	sm := newTestShardManager(t, 2, nil)

	for i, shard := range sm.AllShards() {
		if shard.DSN != ":memory:" {
			t.Errorf("Expected DSN ':memory:' for shard %d, got '%s'", i, shard.DSN)
		}
	}
}

func TestNewShardManager_ClosesOnFailure(t *testing.T) {
	// Primeiro shard ok, segundo falha no ping
	// Isso testa o cleanup de conexões já abertas
	_, err := NewShardManager("sqlite3", ShardConfig{
		DSNs: []string{":memory:", "/nonexistent/path/to/db.sqlite?mode=ro"},
	})
	if err == nil {
		t.Error("Expected error")
	}
}

func TestNewShardManager_OpenFailure(t *testing.T) {
	// Testa falha no sql.Open com driver que não existe
	_, err := NewShardManager("nonexistent_driver", ShardConfig{
		DSNs: []string{"dsn1", "dsn2"},
	})
	if err == nil {
		t.Error("Expected error for nonexistent driver")
	}
}

func TestShardManager_GetShardFromContext_NoKeyExtractorNoContextKey(t *testing.T) {
	// Sem keyExtractor e sem chave no contexto => round-robin
	sm, err := NewShardManager("sqlite3", ShardConfig{
		DSNs: []string{":memory:", ":memory:", ":memory:"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer sm.Close()

	ctx := context.Background()
	seen := make(map[int]bool)
	for i := 0; i < 6; i++ {
		shard := sm.GetShardFromContext(ctx)
		seen[shard.Index] = true
	}
	// Com 6 chamadas e 3 shards, round-robin deve ter visitado todos os shards
	if len(seen) != 3 {
		t.Errorf("Expected round-robin to visit all 3 shards, visited %d", len(seen))
	}
}

func TestNewShardManager_NilDSNs(t *testing.T) {
	_, err := NewShardManager("sqlite3", ShardConfig{
		DSNs: nil,
	})
	if err == nil {
		t.Error("Expected error for nil DSNs")
	}
}

func TestShardedResult_Fields(t *testing.T) {
	r := ShardedResult[string]{
		Results:    []string{"a", "b"},
		ShardIndex: 1,
		Error:      nil,
	}
	if r.ShardIndex != 1 {
		t.Errorf("Expected ShardIndex 1, got %d", r.ShardIndex)
	}
	if len(r.Results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(r.Results))
	}
}

func TestShardManager_CloseReturnsError(t *testing.T) {
	// Cria DBs usando o driver fake que retorna erro no Close.
	// Precisamos fazer Ping para forçar a criação de conexão real no pool,
	// caso contrário sql.DB.Close() não tem conexões idle para fechar.
	db1, err := sql.Open("fake_close_err", "dsn1")
	if err != nil {
		t.Fatal(err)
	}
	// Ping cria uma conexão no pool
	db1.Ping()

	db2, err := sql.Open("fake_close_err", "dsn2")
	if err != nil {
		t.Fatal(err)
	}
	db2.Ping()

	sm := &ShardManager{
		shards: []*Shard{
			{Index: 0, DB: db1, DSN: "dsn1"},
			{Index: 1, DB: db2, DSN: "dsn2"},
		},
		strategy: ModuloStrategy{},
	}

	closeErr := sm.Close()
	if closeErr == nil {
		t.Error("Expected error from Close when DB.Close() fails")
	}
}

func TestNewShardManager_OpenFailCleanup(t *testing.T) {
	// Configura o driver fake para falhar no segundo Open
	callCount := 0
	fakeFailSecondDriver.openFunc = func(name string) (driver.Conn, error) {
		callCount++
		if callCount > 1 {
			return nil, fmt.Errorf("fake open failure")
		}
		return &fakeConn{}, nil
	}
	defer func() {
		fakeFailSecondDriver.openFunc = nil
	}()

	// O primeiro shard vai abrir ok, mas Ping vai chamar Open internamente.
	// Na verdade, sql.Open nao chama driver.Open, ele so registra.
	// Ping e quem chama driver.Open. Entao o primeiro Ping vai usar o
	// primeiro Open (ok), e o segundo Ping vai usar o segundo Open (falha).
	// Isso testa o path de Ping falhando com cleanup.
	_, err := NewShardManager("fake_fail_second", ShardConfig{
		DSNs: []string{"dsn1", "dsn2"},
	})
	if err == nil {
		t.Error("Expected error when second shard fails")
	}
}

func TestNewShardManager_SqlOpenFailCleanup(t *testing.T) {
	// Testa o branch onde sql.Open falha no segundo shard,
	// forçando o cleanup dos shards já abertos.
	callCount := 0
	original := sqlOpen
	sqlOpen = func(driverName, dsn string) (*sql.DB, error) {
		callCount++
		if callCount == 2 {
			return nil, fmt.Errorf("simulated open failure")
		}
		return original(driverName, dsn)
	}
	defer func() {
		sqlOpen = original
	}()

	_, err := NewShardManager("sqlite3", ShardConfig{
		DSNs: []string{":memory:", ":memory:"},
	})
	if err == nil {
		t.Error("Expected error when sql.Open fails on second shard")
	}
}

func TestMergeResults_MultipleErrors(t *testing.T) {
	results := []ShardedResult[int]{
		{Results: nil, ShardIndex: 0, Error: sql.ErrNoRows},
		{Results: nil, ShardIndex: 1, Error: sql.ErrConnDone},
		{Results: []int{1}, ShardIndex: 2},
	}

	merged, err := MergeResults(results)
	if err == nil {
		t.Error("Expected error")
	}
	// Deve retornar resultados do shard que deu certo
	if len(merged) != 1 {
		t.Errorf("Expected 1 result from successful shard, got %d", len(merged))
	}
}
