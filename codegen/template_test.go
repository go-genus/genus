package codegen

import "testing"

func TestFieldConstructor(t *testing.T) {
	tests := []struct {
		fieldType string
		expected  string
	}{
		{"query.StringField", "query.NewStringField"},
		{"query.IntField", "query.NewIntField"},
		{"query.Int64Field", "query.NewInt64Field"},
		{"query.BoolField", "query.NewBoolField"},
		{"query.Float64Field", "query.NewFloat64Field"},
		{"query.OptionalStringField", "query.NewOptionalStringField"},
		{"query.OptionalIntField", "query.NewOptionalIntField"},
		{"query.OptionalInt64Field", "query.NewOptionalInt64Field"},
		{"query.OptionalBoolField", "query.NewOptionalBoolField"},
		{"query.OptionalFloat64Field", "query.NewOptionalFloat64Field"},
		{"query.UnknownField", "query.NewStringField"},
		{"", "query.NewStringField"},
	}

	for _, tt := range tests {
		result := fieldConstructor(tt.fieldType)
		if result != tt.expected {
			t.Errorf("fieldConstructor(%q) = %q, want %q", tt.fieldType, result, tt.expected)
		}
	}
}
