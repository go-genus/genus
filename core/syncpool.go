package core

import (
	"strings"
	"sync"
	"time"
)

// Object pools for frequently allocated items.
// These reduce GC pressure in hot paths.

// ========================================
// Scan Value Pools
// ========================================

// scanValuePools pools slices for row scanning.
// Key insight: most tables have < 20 columns.
var scanValuePools = [...]sync.Pool{
	{New: func() interface{} { return make([]interface{}, 0, 8) }},   // 0-8 columns
	{New: func() interface{} { return make([]interface{}, 0, 16) }},  // 9-16 columns
	{New: func() interface{} { return make([]interface{}, 0, 32) }},  // 17-32 columns
	{New: func() interface{} { return make([]interface{}, 0, 64) }},  // 33-64 columns
	{New: func() interface{} { return make([]interface{}, 0, 128) }}, // 65+ columns
}

// GetScanValues gets a pooled slice for scan values.
func GetScanValues(size int) []interface{} {
	idx := poolIndex(size)
	slice := scanValuePools[idx].Get().([]interface{})
	return slice[:0] // Reset length, keep capacity
}

// PutScanValues returns a scan values slice to the pool.
func PutScanValues(slice []interface{}) {
	if slice == nil {
		return
	}
	// Clear references to avoid memory leaks
	for i := range slice {
		slice[i] = nil
	}
	idx := poolIndex(cap(slice))
	scanValuePools[idx].Put(slice[:0]) //nolint:staticcheck // slice reuse is intentional for performance
}

// ========================================
// String Builder Pool
// ========================================

var builderPool = sync.Pool{
	New: func() interface{} {
		b := &strings.Builder{}
		b.Grow(256) // Pre-allocate for typical query size
		return b
	},
}

// GetBuilder gets a pooled strings.Builder.
func GetBuilder() *strings.Builder {
	b := builderPool.Get().(*strings.Builder)
	b.Reset()
	return b
}

// PutBuilder returns a strings.Builder to the pool.
func PutBuilder(b *strings.Builder) {
	if b == nil {
		return
	}
	// Only return reasonably sized builders to avoid memory bloat
	if b.Cap() < 4096 {
		builderPool.Put(b)
	}
}

// ========================================
// Args Slice Pool
// ========================================

var argsPool = sync.Pool{
	New: func() interface{} {
		return make([]interface{}, 0, 16)
	},
}

// GetArgs gets a pooled args slice.
func GetArgs() []interface{} {
	return argsPool.Get().([]interface{})[:0]
}

// PutArgs returns an args slice to the pool.
func PutArgs(args []interface{}) {
	if args == nil || cap(args) > 128 {
		return
	}
	// Clear references
	for i := range args {
		args[i] = nil
	}
	argsPool.Put(args[:0]) //nolint:staticcheck // slice reuse is intentional for performance
}

// ========================================
// Placeholder Pool (for IN clauses)
// ========================================

// Common placeholder strings cached for reuse
var placeholderCache = func() []string {
	cache := make([]string, 101) // 0-100 placeholders
	for i := 0; i <= 100; i++ {
		if i == 0 {
			cache[i] = ""
			continue
		}
		var b strings.Builder
		for j := 0; j < i; j++ {
			if j > 0 {
				b.WriteString(", ")
			}
			b.WriteString("?")
		}
		cache[i] = b.String()
	}
	return cache
}()

// GetPlaceholders returns a string of n placeholders: "?, ?, ?"
// Uses cached strings for common sizes.
func GetPlaceholders(n int) string {
	if n <= 0 {
		return ""
	}
	if n < len(placeholderCache) {
		return placeholderCache[n]
	}
	// Build for larger sizes
	var b strings.Builder
	b.Grow(n*3 - 2)
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("?")
	}
	return b.String()
}

// ========================================
// Int64 Slice Pool (for IDs)
// ========================================

var int64Pool = sync.Pool{
	New: func() interface{} {
		return make([]int64, 0, 64)
	},
}

// GetInt64Slice gets a pooled int64 slice.
func GetInt64Slice() []int64 {
	return int64Pool.Get().([]int64)[:0]
}

// PutInt64Slice returns an int64 slice to the pool.
func PutInt64Slice(slice []int64) {
	if slice == nil || cap(slice) > 1024 {
		return
	}
	int64Pool.Put(slice[:0]) //nolint:staticcheck // slice reuse is intentional for performance
}

// ========================================
// Time Helpers
// ========================================

var timeNow = time.Now // Allow mocking in tests

// Now returns current time. Can be replaced for testing.
func Now() time.Time {
	return timeNow()
}

// ========================================
// Helper Functions
// ========================================

// poolIndex returns the pool index for a given size.
func poolIndex(size int) int {
	switch {
	case size <= 8:
		return 0
	case size <= 16:
		return 1
	case size <= 32:
		return 2
	case size <= 64:
		return 3
	default:
		return 4
	}
}
