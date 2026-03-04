package cache

import (
	"context"
	"testing"
	"time"
)

func TestInMemoryCache(t *testing.T) {
	cache := NewInMemoryCache(100)
	ctx := context.Background()

	// Test Set and Get
	t.Run("set and get", func(t *testing.T) {
		err := cache.Set(ctx, "key1", []byte("value1"), time.Minute)
		if err != nil {
			t.Errorf("Set failed: %v", err)
		}

		value, err := cache.Get(ctx, "key1")
		if err != nil {
			t.Errorf("Get failed: %v", err)
		}
		if string(value) != "value1" {
			t.Errorf("Expected 'value1', got '%s'", string(value))
		}
	})

	// Test cache miss
	t.Run("cache miss", func(t *testing.T) {
		value, _ := cache.Get(ctx, "nonexistent")
		if value != nil {
			t.Errorf("Expected nil for cache miss, got %v", value)
		}
	})

	// Test Delete
	t.Run("delete", func(t *testing.T) {
		cache.Set(ctx, "key2", []byte("value2"), time.Minute)
		err := cache.Delete(ctx, "key2")
		if err != nil {
			t.Errorf("Delete failed: %v", err)
		}

		value, _ := cache.Get(ctx, "key2")
		if value != nil {
			t.Errorf("Expected nil after delete, got %v", value)
		}
	})

	// Test DeleteByPrefix
	t.Run("delete by prefix", func(t *testing.T) {
		cache.Set(ctx, "prefix:key1", []byte("v1"), time.Minute)
		cache.Set(ctx, "prefix:key2", []byte("v2"), time.Minute)
		cache.Set(ctx, "other:key3", []byte("v3"), time.Minute)

		err := cache.DeleteByPrefix(ctx, "prefix:")
		if err != nil {
			t.Errorf("DeleteByPrefix failed: %v", err)
		}

		value, _ := cache.Get(ctx, "prefix:key1")
		if value != nil {
			t.Errorf("prefix:key1 should be deleted")
		}

		value, _ = cache.Get(ctx, "prefix:key2")
		if value != nil {
			t.Errorf("prefix:key2 should be deleted")
		}

		value, err = cache.Get(ctx, "other:key3")
		if err != nil || value == nil {
			t.Errorf("other:key3 should still exist")
		}
	})

	// Test Clear
	t.Run("clear", func(t *testing.T) {
		cache.Set(ctx, "a", []byte("1"), time.Minute)
		cache.Set(ctx, "b", []byte("2"), time.Minute)

		err := cache.Clear(ctx)
		if err != nil {
			t.Errorf("Clear failed: %v", err)
		}

		value, _ := cache.Get(ctx, "a")
		if value != nil {
			t.Errorf("Expected cache to be empty after clear")
		}
	})

	// Test TTL expiration
	t.Run("ttl expiration", func(t *testing.T) {
		shortTTL := 50 * time.Millisecond
		cache.Set(ctx, "expires", []byte("soon"), shortTTL)

		// Should exist immediately
		value, _ := cache.Get(ctx, "expires")
		if value == nil {
			t.Errorf("Key should exist immediately after set")
		}

		// Wait for expiration
		time.Sleep(100 * time.Millisecond)

		// Should be expired now
		value, _ = cache.Get(ctx, "expires")
		if value != nil {
			t.Errorf("Key should be expired")
		}
	})

	// Test Stats
	t.Run("stats", func(t *testing.T) {
		cache.Clear(ctx)

		// Generate some hits and misses
		cache.Set(ctx, "x", []byte("y"), time.Minute)
		cache.Get(ctx, "x") // hit
		cache.Get(ctx, "x") // hit
		cache.Get(ctx, "z") // miss

		stats := cache.Stats()
		if stats.Hits < 2 {
			t.Errorf("Expected at least 2 hits, got %d", stats.Hits)
		}
		if stats.Misses < 1 {
			t.Errorf("Expected at least 1 miss, got %d", stats.Misses)
		}
	})
}

func TestCacheConfig(t *testing.T) {
	// Test default config
	t.Run("default config", func(t *testing.T) {
		config := DefaultCacheConfig()
		if config.DefaultTTL != 5*time.Minute {
			t.Errorf("Expected default TTL of 5 minutes")
		}
		if !config.Enabled {
			t.Error("Cache should be enabled by default")
		}
	})

	// Test config with custom TTL
	t.Run("custom TTL", func(t *testing.T) {
		config := DefaultCacheConfig()
		config.DefaultTTL = 10 * time.Minute
		if config.DefaultTTL != 10*time.Minute {
			t.Errorf("Expected TTL of 10 minutes")
		}
	})

	// Test disabled cache
	t.Run("disabled cache", func(t *testing.T) {
		config := CacheConfig{
			Enabled: false,
		}
		if config.Enabled {
			t.Error("Cache should be disabled")
		}
	})
}

func TestNoOpCache(t *testing.T) {
	cache := NewNoOpCache()
	ctx := context.Background()

	// All operations should succeed but do nothing
	err := cache.Set(ctx, "key", []byte("value"), time.Minute)
	if err != nil {
		t.Errorf("Set should not error: %v", err)
	}

	value, err := cache.Get(ctx, "key")
	if err != nil {
		t.Errorf("Get should not error: %v", err)
	}
	if value != nil {
		t.Errorf("NoOpCache should always return nil")
	}

	err = cache.Delete(ctx, "key")
	if err != nil {
		t.Errorf("Delete should not error: %v", err)
	}

	err = cache.DeleteByPrefix(ctx, "prefix")
	if err != nil {
		t.Errorf("DeleteByPrefix should not error: %v", err)
	}

	err = cache.Clear(ctx)
	if err != nil {
		t.Errorf("Clear should not error: %v", err)
	}

	stats := cache.Stats()
	if stats.Size != 0 {
		t.Errorf("NoOp cache should have size 0")
	}
}

func TestQueryCacheKey(t *testing.T) {
	// Test that same query generates same key
	t.Run("consistent keys", func(t *testing.T) {
		key1 := QueryCacheKey("genus:", "users", "SELECT * FROM users WHERE id = ?", []interface{}{1})
		key2 := QueryCacheKey("genus:", "users", "SELECT * FROM users WHERE id = ?", []interface{}{1})
		if key1 != key2 {
			t.Errorf("Same query should generate same key")
		}
	})

	// Test that different queries generate different keys
	t.Run("different args", func(t *testing.T) {
		key1 := QueryCacheKey("genus:", "users", "SELECT * FROM users WHERE id = ?", []interface{}{1})
		key2 := QueryCacheKey("genus:", "users", "SELECT * FROM users WHERE id = ?", []interface{}{2})
		if key1 == key2 {
			t.Errorf("Different args should generate different keys")
		}
	})

	// Test that different tables generate different keys
	t.Run("different tables", func(t *testing.T) {
		key1 := QueryCacheKey("genus:", "users", "SELECT * FROM users", nil)
		key2 := QueryCacheKey("genus:", "posts", "SELECT * FROM posts", nil)
		if key1 == key2 {
			t.Errorf("Different tables should generate different keys")
		}
	})
}

func TestTableCachePrefix(t *testing.T) {
	prefix := TableCachePrefix("genus:", "users")
	expected := "genus:users:"
	if prefix != expected {
		t.Errorf("Expected '%s', got '%s'", expected, prefix)
	}
}

func TestLRUEviction(t *testing.T) {
	cache := NewInMemoryCache(3) // Very small cache
	ctx := context.Background()

	// Fill the cache
	cache.Set(ctx, "a", []byte("1"), time.Hour)
	cache.Set(ctx, "b", []byte("2"), time.Hour)
	cache.Set(ctx, "c", []byte("3"), time.Hour)

	// This should evict the oldest entry
	cache.Set(ctx, "d", []byte("4"), time.Hour)

	// First entry should be evicted
	value, _ := cache.Get(ctx, "a")
	if value != nil {
		t.Error("Entry 'a' should have been evicted")
	}

	// Newer entries should still exist
	value, _ = cache.Get(ctx, "b")
	if value == nil {
		t.Error("Entry 'b' should still exist")
	}

	value, _ = cache.Get(ctx, "d")
	if value == nil {
		t.Error("Entry 'd' should exist")
	}

	// Check stats
	stats := cache.Stats()
	if stats.Evictions == 0 {
		t.Error("Should have at least one eviction")
	}
}
