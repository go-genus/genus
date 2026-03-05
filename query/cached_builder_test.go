package query

import (
	"testing"
	"time"

	"github.com/go-genus/genus/cache"
)

func TestBuilder_WithCache(t *testing.T) {
	b := newTestBuilder()
	config := cache.CacheConfig{
		Enabled:    true,
		DefaultTTL: 5 * time.Minute,
		KeyPrefix:  "test",
	}

	cb := b.WithCache(nil, config)
	if cb == nil {
		t.Fatal("WithCache returned nil")
	}
	if cb.ttl != 5*time.Minute {
		t.Errorf("ttl = %v, want 5m", cb.ttl)
	}
	if cb.skipCache {
		t.Error("skipCache should be false when Enabled=true")
	}
}

func TestBuilder_WithCache_Disabled(t *testing.T) {
	b := newTestBuilder()
	config := cache.CacheConfig{
		Enabled: false,
	}

	cb := b.WithCache(nil, config)
	if !cb.skipCache {
		t.Error("skipCache should be true when Enabled=false")
	}
}

func TestCachedBuilder_TTL(t *testing.T) {
	b := newTestBuilder()
	config := cache.CacheConfig{Enabled: true, DefaultTTL: 5 * time.Minute}
	cb := b.WithCache(nil, config).TTL(10 * time.Minute)
	if cb.ttl != 10*time.Minute {
		t.Errorf("TTL = %v, want 10m", cb.ttl)
	}
}

func TestCachedBuilder_NoCache(t *testing.T) {
	b := newTestBuilder()
	config := cache.CacheConfig{Enabled: true}
	cb := b.WithCache(nil, config).NoCache()
	if !cb.skipCache {
		t.Error("NoCache should set skipCache to true")
	}
}

func TestCachedBuilder_Where(t *testing.T) {
	b := newTestBuilder()
	config := cache.CacheConfig{Enabled: true}
	cb := b.WithCache(nil, config).
		Where(Condition{Field: "name", Operator: OpEq, Value: "John"})
	if cb == nil {
		t.Fatal("Where returned nil")
	}
}

func TestCachedBuilder_OrderByAsc(t *testing.T) {
	b := newTestBuilder()
	config := cache.CacheConfig{Enabled: true}
	cb := b.WithCache(nil, config).OrderByAsc("name")
	if cb == nil {
		t.Fatal("OrderByAsc returned nil")
	}
}

func TestCachedBuilder_OrderByDesc(t *testing.T) {
	b := newTestBuilder()
	config := cache.CacheConfig{Enabled: true}
	cb := b.WithCache(nil, config).OrderByDesc("name")
	if cb == nil {
		t.Fatal("OrderByDesc returned nil")
	}
}

func TestCachedBuilder_Limit(t *testing.T) {
	b := newTestBuilder()
	config := cache.CacheConfig{Enabled: true}
	cb := b.WithCache(nil, config).Limit(10)
	if cb == nil {
		t.Fatal("Limit returned nil")
	}
}

func TestCachedBuilder_Offset(t *testing.T) {
	b := newTestBuilder()
	config := cache.CacheConfig{Enabled: true}
	cb := b.WithCache(nil, config).Offset(20)
	if cb == nil {
		t.Fatal("Offset returned nil")
	}
}

func TestCachedBuilder_Select(t *testing.T) {
	b := newTestBuilder()
	config := cache.CacheConfig{Enabled: true}
	cb := b.WithCache(nil, config).Select("name", "email")
	if cb == nil {
		t.Fatal("Select returned nil")
	}
}

func TestCachedBuilder_WithTrashed(t *testing.T) {
	b := newTestBuilder()
	config := cache.CacheConfig{Enabled: true}
	cb := b.WithCache(nil, config).WithTrashed()
	if cb == nil {
		t.Fatal("WithTrashed returned nil")
	}
}

func TestCachedBuilder_OnlyTrashed(t *testing.T) {
	b := newTestBuilder()
	config := cache.CacheConfig{Enabled: true}
	cb := b.WithCache(nil, config).OnlyTrashed()
	if cb == nil {
		t.Fatal("OnlyTrashed returned nil")
	}
}

func TestCachedBuilder_Preload(t *testing.T) {
	b := newTestBuilder()
	config := cache.CacheConfig{Enabled: true}
	cb := b.WithCache(nil, config).Preload("Posts")
	if cb == nil {
		t.Fatal("Preload returned nil")
	}
}

func TestCachedBuilder_Aggregate(t *testing.T) {
	b := newTestBuilder()
	config := cache.CacheConfig{Enabled: true}
	agg := b.WithCache(nil, config).Aggregate()
	if agg == nil {
		t.Fatal("Aggregate returned nil")
	}
}

func TestCachedBuilder_Clone(t *testing.T) {
	b := newTestBuilder()
	config := cache.CacheConfig{Enabled: true, DefaultTTL: 5 * time.Minute}
	cb := b.WithCache(nil, config)
	cloned := cb.clone()
	if cloned.ttl != cb.ttl {
		t.Error("clone should preserve ttl")
	}
	if cloned.skipCache != cb.skipCache {
		t.Error("clone should preserve skipCache")
	}
}
