package mysql

import (
	"testing"
)

func TestNew(t *testing.T) {
	d := New()
	if d == nil {
		t.Fatal("New() returned nil")
	}
}

func TestDialect_Placeholder(t *testing.T) {
	d := New()

	tests := []struct {
		name string
		n    int
		want string
	}{
		{"first", 1, "?"},
		{"second", 2, "?"},
		{"tenth", 10, "?"},
		{"zero", 0, "?"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := d.Placeholder(tt.n)
			if got != tt.want {
				t.Errorf("Placeholder(%d) = %q, want %q", tt.n, got, tt.want)
			}
		})
	}
}

func TestDialect_QuoteIdentifier(t *testing.T) {
	d := New()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple table", "users", "`users`"},
		{"column name", "id", "`id`"},
		{"with underscore", "user_name", "`user_name`"},
		{"empty string", "", "``"},
		{"reserved word", "select", "`select`"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := d.QuoteIdentifier(tt.input)
			if got != tt.want {
				t.Errorf("QuoteIdentifier(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestDialect_GetType(t *testing.T) {
	d := New()

	tests := []struct {
		name   string
		goType string
		want   string
	}{
		{"string", "string", "VARCHAR(255)"},
		{"int", "int", "INT"},
		{"int64", "int64", "BIGINT"},
		{"bool", "bool", "BOOLEAN"},
		{"time.Time", "time.Time", "DATETIME"},
		{"float64", "float64", "DOUBLE"},
		{"float32", "float32", "FLOAT"},
		{"unknown type fallback", "custom.Type", "TEXT"},
		{"empty string fallback", "", "TEXT"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := d.GetType(tt.goType)
			if got != tt.want {
				t.Errorf("GetType(%q) = %q, want %q", tt.goType, got, tt.want)
			}
		})
	}
}
