package sharding

import (
	"context"
	"testing"
)

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
