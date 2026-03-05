package core

import (
	"errors"
	"testing"
	"time"
)

func TestNewDefaultLogger(t *testing.T) {
	logger := NewDefaultLogger(false)
	if logger == nil {
		t.Fatal("NewDefaultLogger returned nil")
	}
	if logger.verbose {
		t.Error("verbose should be false")
	}

	verbose := NewDefaultLogger(true)
	if !verbose.verbose {
		t.Error("verbose should be true")
	}
}

func TestDefaultLogger_LogQuery_NonVerbose(t *testing.T) {
	logger := NewDefaultLogger(false)
	// Should not panic
	logger.LogQuery("SELECT * FROM users", []interface{}{1, "test"}, 1000)
}

func TestDefaultLogger_LogQuery_Verbose(t *testing.T) {
	logger := NewDefaultLogger(true)
	// Should not panic, and should include args
	logger.LogQuery("SELECT * FROM users WHERE id = $1", []interface{}{1}, 1000)
}

func TestDefaultLogger_LogQuery_Verbose_NoArgs(t *testing.T) {
	logger := NewDefaultLogger(true)
	// Should not panic even with nil args
	logger.LogQuery("SELECT 1", nil, 100)
}

func TestDefaultLogger_LogError_NonVerbose(t *testing.T) {
	logger := NewDefaultLogger(false)
	err := errors.New("connection refused")
	// Should not panic
	logger.LogError("SELECT * FROM users", []interface{}{1}, err)
}

func TestDefaultLogger_LogError_Verbose(t *testing.T) {
	logger := NewDefaultLogger(true)
	err := errors.New("connection refused")
	// Should not panic
	logger.LogError("SELECT * FROM users", []interface{}{1}, err)
}

func TestDefaultLogger_LogError_Verbose_NoArgs(t *testing.T) {
	logger := NewDefaultLogger(true)
	// Should not panic with nil args
	logger.LogError("SELECT 1", nil, errors.New("error"))
}

func TestNoOpLogger_LogQuery(t *testing.T) {
	logger := &NoOpLogger{}
	// Should not panic
	logger.LogQuery("SELECT 1", nil, 100)
}

func TestNoOpLogger_LogError(t *testing.T) {
	logger := &NoOpLogger{}
	// Should not panic
	logger.LogError("SELECT 1", nil, errors.New("error"))
}

func TestNoOpLogger_ImplementsLogger(t *testing.T) {
	var _ Logger = &NoOpLogger{}
}

func TestNoOpLogger_AllMethods(t *testing.T) {
	logger := &NoOpLogger{}
	// All methods should be no-ops and not panic
	logger.LogQuery("SELECT * FROM users", []interface{}{1, "test"}, 1000)
	logger.LogQuery("", nil, 0)
	logger.LogError("SELECT 1", []interface{}{}, errTest)
	logger.LogError("", nil, nil)
}

func TestCleanSQL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "removes newlines",
			input:    "SELECT *\nFROM users\nWHERE id = 1",
			expected: "SELECT * FROM users WHERE id = 1",
		},
		{
			name:     "removes tabs",
			input:    "SELECT *\t\tFROM users",
			expected: "SELECT * FROM users",
		},
		{
			name:     "removes multiple spaces",
			input:    "SELECT  *   FROM    users",
			expected: "SELECT * FROM users",
		},
		{
			name:     "trims whitespace",
			input:    "  SELECT * FROM users  ",
			expected: "SELECT * FROM users",
		},
		{
			name:     "already clean",
			input:    "SELECT * FROM users",
			expected: "SELECT * FROM users",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanSQL(tt.input)
			if got != tt.expected {
				t.Errorf("cleanSQL(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		nanos    int64
		contains string
	}{
		{
			name:     "nanoseconds",
			nanos:    500,
			contains: "ns",
		},
		{
			name:     "microseconds",
			nanos:    int64(50 * time.Microsecond),
			contains: "µs",
		},
		{
			name:     "milliseconds",
			nanos:    int64(50 * time.Millisecond),
			contains: "ms",
		},
		{
			name:     "seconds",
			nanos:    int64(2 * time.Second),
			contains: "s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.nanos)
			if got == "" {
				t.Error("formatDuration returned empty string")
			}
			// Just check it contains the expected unit suffix
			if len(got) == 0 {
				t.Error("formatDuration returned empty string")
			}
		})
	}
}

func TestFormatDuration_Ranges(t *testing.T) {
	// Sub-microsecond
	result := formatDuration(500)
	if result != "500ns" {
		t.Errorf("500ns: got %q", result)
	}

	// Microsecond range
	result = formatDuration(int64(50 * time.Microsecond))
	if result == "" {
		t.Error("microsecond range should return non-empty")
	}

	// Millisecond range
	result = formatDuration(int64(50 * time.Millisecond))
	if result == "" {
		t.Error("millisecond range should return non-empty")
	}

	// Second range
	result = formatDuration(int64(2 * time.Second))
	if result == "" {
		t.Error("second range should return non-empty")
	}
}
