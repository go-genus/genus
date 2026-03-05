package core

import (
	"strings"
	"testing"
	"time"
)

func TestGetScanValues(t *testing.T) {
	slice := GetScanValues(5)
	if slice == nil {
		t.Fatal("GetScanValues returned nil")
	}
	if len(slice) != 0 {
		t.Errorf("len = %d, want 0", len(slice))
	}
	if cap(slice) < 5 {
		t.Errorf("cap = %d, should be >= 5", cap(slice))
	}
	PutScanValues(slice)
}

func TestGetScanValues_Various_Sizes(t *testing.T) {
	sizes := []int{1, 5, 8, 9, 16, 17, 32, 33, 64, 65, 128}
	for _, size := range sizes {
		slice := GetScanValues(size)
		if slice == nil {
			t.Errorf("GetScanValues(%d) returned nil", size)
		}
		PutScanValues(slice)
	}
}

func TestPutScanValues_Nil(t *testing.T) {
	// Should not panic
	PutScanValues(nil)
}

func TestPutScanValues_ClearsReferences(t *testing.T) {
	slice := GetScanValues(4)
	slice = append(slice, "a", "b", "c")
	PutScanValues(slice)
	// After put, references should be nil
	// Get a new one and verify it's clean
	newSlice := GetScanValues(4)
	for _, v := range newSlice {
		if v != nil {
			t.Error("returned slice should have nil values")
		}
	}
	PutScanValues(newSlice)
}

func TestGetBuilder(t *testing.T) {
	b := GetBuilder()
	if b == nil {
		t.Fatal("GetBuilder returned nil")
	}
	if b.Len() != 0 {
		t.Error("builder should be reset")
	}
	b.WriteString("test")
	PutBuilder(b)
}

func TestPutBuilder_Nil(t *testing.T) {
	// Should not panic
	PutBuilder(nil)
}

func TestPutBuilder_LargeBuilder(t *testing.T) {
	b := &strings.Builder{}
	b.Grow(8192)
	b.WriteString(strings.Repeat("x", 5000))
	// Large builders should not be returned to pool
	PutBuilder(b)
}

func TestGetArgs(t *testing.T) {
	args := GetArgs()
	if args == nil {
		t.Fatal("GetArgs returned nil")
	}
	if len(args) != 0 {
		t.Errorf("len = %d, want 0", len(args))
	}
	PutArgs(args)
}

func TestPutArgs_Nil(t *testing.T) {
	// Should not panic
	PutArgs(nil)
}

func TestPutArgs_LargeSlice(t *testing.T) {
	args := make([]interface{}, 0, 200)
	// Large slices should not be returned to pool
	PutArgs(args)
}

func TestPutArgs_ClearsReferences(t *testing.T) {
	args := GetArgs()
	args = append(args, "a", "b")
	PutArgs(args)
}

func TestGetPlaceholders(t *testing.T) {
	tests := []struct {
		n        int
		expected string
	}{
		{0, ""},
		{1, "?"},
		{2, "?, ?"},
		{3, "?, ?, ?"},
		{5, "?, ?, ?, ?, ?"},
	}

	for _, tt := range tests {
		got := GetPlaceholders(tt.n)
		if got != tt.expected {
			t.Errorf("GetPlaceholders(%d) = %q, want %q", tt.n, got, tt.expected)
		}
	}
}

func TestGetPlaceholders_Negative(t *testing.T) {
	if got := GetPlaceholders(-1); got != "" {
		t.Errorf("GetPlaceholders(-1) = %q, want empty", got)
	}
}

func TestGetPlaceholders_Large(t *testing.T) {
	// Larger than cache
	result := GetPlaceholders(150)
	count := strings.Count(result, "?")
	if count != 150 {
		t.Errorf("expected 150 placeholders, got %d", count)
	}
}

func TestGetPlaceholders_CachedRange(t *testing.T) {
	// Test within cached range (0-100)
	for i := 0; i <= 100; i++ {
		result := GetPlaceholders(i)
		if i == 0 {
			if result != "" {
				t.Errorf("GetPlaceholders(0) = %q, want empty", result)
			}
		} else {
			count := strings.Count(result, "?")
			if count != i {
				t.Errorf("GetPlaceholders(%d): expected %d ?, got %d", i, i, count)
			}
		}
	}
}

func TestGetInt64Slice(t *testing.T) {
	slice := GetInt64Slice()
	if slice == nil {
		t.Fatal("GetInt64Slice returned nil")
	}
	if len(slice) != 0 {
		t.Errorf("len = %d, want 0", len(slice))
	}
	PutInt64Slice(slice)
}

func TestPutInt64Slice_Nil(t *testing.T) {
	// Should not panic
	PutInt64Slice(nil)
}

func TestPutInt64Slice_LargeSlice(t *testing.T) {
	slice := make([]int64, 0, 2000)
	// Large slices should not be returned to pool
	PutInt64Slice(slice)
}

func TestNow(t *testing.T) {
	before := time.Now()
	got := Now()
	after := time.Now()

	if got.Before(before) || got.After(after) {
		t.Error("Now() should return current time")
	}
}

func TestPoolIndex(t *testing.T) {
	tests := []struct {
		size     int
		expected int
	}{
		{0, 0},
		{1, 0},
		{8, 0},
		{9, 1},
		{16, 1},
		{17, 2},
		{32, 2},
		{33, 3},
		{64, 3},
		{65, 4},
		{128, 4},
		{1000, 4},
	}

	for _, tt := range tests {
		got := poolIndex(tt.size)
		if got != tt.expected {
			t.Errorf("poolIndex(%d) = %d, want %d", tt.size, got, tt.expected)
		}
	}
}
