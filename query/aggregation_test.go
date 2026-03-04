package query

import (
	"testing"
)

func TestAggregateResult_Int64(t *testing.T) {
	tests := []struct {
		name     string
		values   map[string]interface{}
		key      string
		expected int64
	}{
		{"int64 value", map[string]interface{}{"count": int64(42)}, "count", 42},
		{"int value", map[string]interface{}{"count": int(42)}, "count", 42},
		{"int32 value", map[string]interface{}{"count": int32(42)}, "count", 42},
		{"float64 value", map[string]interface{}{"count": float64(42.9)}, "count", 42},
		{"float32 value", map[string]interface{}{"count": float32(42.9)}, "count", 42},
		{"missing key", map[string]interface{}{"other": int64(42)}, "count", 0},
		{"nil map", map[string]interface{}{}, "count", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewAggregateResult(tt.values)
			got := result.Int64(tt.key)
			if got != tt.expected {
				t.Errorf("Int64(%q) = %d, want %d", tt.key, got, tt.expected)
			}
		})
	}
}

func TestAggregateResult_Float64(t *testing.T) {
	tests := []struct {
		name     string
		values   map[string]interface{}
		key      string
		expected float64
	}{
		{"float64 value", map[string]interface{}{"avg": float64(3.14)}, "avg", 3.14},
		{"float32 value", map[string]interface{}{"avg": float32(3.14)}, "avg", float64(float32(3.14))},
		{"int64 value", map[string]interface{}{"avg": int64(42)}, "avg", 42.0},
		{"int value", map[string]interface{}{"avg": int(42)}, "avg", 42.0},
		{"int32 value", map[string]interface{}{"avg": int32(42)}, "avg", 42.0},
		{"missing key", map[string]interface{}{"other": float64(3.14)}, "avg", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewAggregateResult(tt.values)
			got := result.Float64(tt.key)
			if got != tt.expected {
				t.Errorf("Float64(%q) = %f, want %f", tt.key, got, tt.expected)
			}
		})
	}
}

func TestAggregateResult_String(t *testing.T) {
	tests := []struct {
		name     string
		values   map[string]interface{}
		key      string
		expected string
	}{
		{"string value", map[string]interface{}{"name": "test"}, "name", "test"},
		{"[]byte value", map[string]interface{}{"name": []byte("test")}, "name", "test"},
		{"int value formatted", map[string]interface{}{"count": 42}, "count", "42"},
		{"missing key", map[string]interface{}{"other": "test"}, "name", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewAggregateResult(tt.values)
			got := result.String(tt.key)
			if got != tt.expected {
				t.Errorf("String(%q) = %q, want %q", tt.key, got, tt.expected)
			}
		})
	}
}

func TestAggregateResult_Has(t *testing.T) {
	result := NewAggregateResult(map[string]interface{}{
		"count": int64(42),
		"name":  "test",
	})

	if !result.Has("count") {
		t.Error("Has(count) = false, want true")
	}
	if !result.Has("name") {
		t.Error("Has(name) = false, want true")
	}
	if result.Has("missing") {
		t.Error("Has(missing) = true, want false")
	}
}

func TestAggregateResult_Keys(t *testing.T) {
	result := NewAggregateResult(map[string]interface{}{
		"count": int64(42),
		"sum":   float64(100.5),
	})

	keys := result.Keys()
	if len(keys) != 2 {
		t.Errorf("Keys() len = %d, want 2", len(keys))
	}

	// Verifica que ambas as chaves existem
	hasCount, hasSum := false, false
	for _, k := range keys {
		if k == "count" {
			hasCount = true
		}
		if k == "sum" {
			hasSum = true
		}
	}

	if !hasCount || !hasSum {
		t.Errorf("Keys() = %v, expected to contain 'count' and 'sum'", keys)
	}
}

func TestAggregateResult_Value(t *testing.T) {
	values := map[string]interface{}{
		"count": int64(42),
		"name":  "test",
	}
	result := NewAggregateResult(values)

	if result.Value("count") != int64(42) {
		t.Errorf("Value(count) = %v, want 42", result.Value("count"))
	}

	if result.Value("name") != "test" {
		t.Errorf("Value(name) = %v, want 'test'", result.Value("name"))
	}

	if result.Value("missing") != nil {
		t.Errorf("Value(missing) = %v, want nil", result.Value("missing"))
	}
}

func TestSanitizeAlias(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"user.name", "user_name"},
		{"some-column", "some_column"},
		{"user.first-name", "user_first_name"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeAlias(tt.input)
			if got != tt.expected {
				t.Errorf("sanitizeAlias(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
