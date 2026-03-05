package query

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestDefaultN1DetectorConfig(t *testing.T) {
	config := DefaultN1DetectorConfig()
	if !config.Enabled {
		t.Error("Enabled should be true")
	}
	if config.Threshold != 5 {
		t.Errorf("Threshold = %d, want 5", config.Threshold)
	}
	if config.TimeWindow != 100*time.Millisecond {
		t.Errorf("TimeWindow = %v, want 100ms", config.TimeWindow)
	}
}

func TestNewN1Detector(t *testing.T) {
	d := NewN1Detector(N1DetectorConfig{})
	if d == nil {
		t.Fatal("NewN1Detector returned nil")
	}
	// Default threshold
	if d.config.Threshold != 5 {
		t.Errorf("Threshold = %d, want 5 (default)", d.config.Threshold)
	}
	// Default time window
	if d.config.TimeWindow != 100*time.Millisecond {
		t.Errorf("TimeWindow = %v, want 100ms (default)", d.config.TimeWindow)
	}
}

func TestN1Detector_Record_Disabled(t *testing.T) {
	d := NewN1Detector(N1DetectorConfig{Enabled: false})
	d.Record("SELECT * FROM users WHERE id = 1")
	if len(d.queries) != 0 {
		t.Error("disabled detector should not record queries")
	}
}

func TestN1Detector_Record_ExcludePatterns(t *testing.T) {
	d := NewN1Detector(N1DetectorConfig{
		Enabled:         true,
		Threshold:       5,
		TimeWindow:      100 * time.Millisecond,
		ExcludePatterns: []string{"migrations"},
	})
	d.Record("SELECT * FROM migrations WHERE id = 1")
	if len(d.queries) != 0 {
		t.Error("excluded pattern should not be recorded")
	}
}

func TestN1Detector_Record_Detection(t *testing.T) {
	detected := false
	d := NewN1Detector(N1DetectorConfig{
		Enabled:    true,
		Threshold:  3,
		TimeWindow: 1 * time.Second,
		OnDetection: func(det N1Detection) {
			detected = true
		},
	})

	for i := 0; i < 5; i++ {
		d.Record("SELECT * FROM users WHERE id = $1")
	}

	if !detected {
		t.Error("should have detected N+1")
	}

	detections := d.GetDetections()
	if len(detections) == 0 {
		t.Error("should have at least 1 detection")
	}
}

func TestN1Detector_NormalizeQuery(t *testing.T) {
	d := NewN1Detector(DefaultN1DetectorConfig())

	tests := []struct {
		name  string
		query string
	}{
		{"removes numbers", "SELECT * FROM users WHERE id = 42"},
		{"removes strings", "SELECT * FROM users WHERE name = 'John'"},
		{"removes pg placeholders", "SELECT * FROM users WHERE id = $1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := d.normalizeQuery(tt.query)
			if strings.Contains(result, "42") || strings.Contains(result, "john") {
				t.Errorf("normalizeQuery should remove specific values, got %q", result)
			}
		})
	}
}

func TestN1Detector_GenerateSuggestion(t *testing.T) {
	d := NewN1Detector(DefaultN1DetectorConfig())

	// Query with WHERE = ?
	s1 := d.generateSuggestion("SELECT * FROM users WHERE id = ?")
	if !strings.Contains(s1, "Preload") {
		t.Errorf("suggestion should mention Preload, got %q", s1)
	}

	// Generic SELECT
	s2 := d.generateSuggestion("SELECT name FROM products")
	if !strings.Contains(s2, "Preload") {
		t.Errorf("suggestion should mention Preload, got %q", s2)
	}

	// Fallback
	s3 := d.generateSuggestion("SOMETHING ELSE")
	if s3 == "" {
		t.Error("should always return a suggestion")
	}
}

func TestN1Detector_Clear(t *testing.T) {
	d := NewN1Detector(N1DetectorConfig{
		Enabled:    true,
		Threshold:  3,
		TimeWindow: 1 * time.Second,
	})

	for i := 0; i < 5; i++ {
		d.Record("SELECT * FROM users WHERE id = $1")
	}

	d.Clear()

	if len(d.queries) != 0 {
		t.Error("Clear should empty queries")
	}
	if len(d.detections) != 0 {
		t.Error("Clear should empty detections")
	}
}

func TestN1Detector_Reset(t *testing.T) {
	d := NewN1Detector(N1DetectorConfig{
		Enabled:    true,
		Threshold:  3,
		TimeWindow: 1 * time.Second,
	})
	d.Record("SELECT 1")
	d.Reset()
	if len(d.queries) != 0 {
		t.Error("Reset should empty queries")
	}
}

func TestN1Detector_GetDetections(t *testing.T) {
	d := NewN1Detector(N1DetectorConfig{
		Enabled:    true,
		Threshold:  2,
		TimeWindow: 1 * time.Second,
	})

	for i := 0; i < 3; i++ {
		d.Record("SELECT * FROM users WHERE id = $1")
	}

	detections := d.GetDetections()
	if len(detections) == 0 {
		t.Error("should have detections")
	}
}

func TestN1Detector_GenerateReport(t *testing.T) {
	d := NewN1Detector(N1DetectorConfig{
		Enabled:    true,
		Threshold:  2,
		TimeWindow: 1 * time.Second,
	})

	for i := 0; i < 3; i++ {
		d.Record("SELECT * FROM users WHERE id = $1")
	}

	report := d.GenerateReport()
	if report.TotalQueries == 0 {
		t.Error("TotalQueries should be > 0")
	}
	if report.ProblematicPct == 0 {
		t.Error("ProblematicPct should be > 0")
	}
}

func TestN1Detector_GenerateReport_Empty(t *testing.T) {
	d := NewN1Detector(DefaultN1DetectorConfig())
	report := d.GenerateReport()
	if report.TotalQueries != 0 {
		t.Errorf("TotalQueries = %d, want 0", report.TotalQueries)
	}
	if report.ProblematicPct != 0 {
		t.Errorf("ProblematicPct = %f, want 0", report.ProblematicPct)
	}
}

func TestN1Detector_PrintReport(t *testing.T) {
	d := NewN1Detector(N1DetectorConfig{
		Enabled:           true,
		Threshold:         2,
		TimeWindow:        1 * time.Second,
		IncludeStackTrace: true,
	})

	for i := 0; i < 3; i++ {
		d.Record("SELECT * FROM users WHERE id = $1")
	}

	report := d.PrintReport()
	if !strings.Contains(report, "N+1 Query Detection Report") {
		t.Error("report should contain title")
	}
	if !strings.Contains(report, "Detection") {
		t.Error("report should contain detection details")
	}
}

func TestN1DetectorExecutor(t *testing.T) {
	d := NewN1Detector(N1DetectorConfig{
		Enabled:    true,
		Threshold:  5,
		TimeWindow: 1 * time.Second,
	})

	exec := &mockExecutor{}
	wrapper := NewN1DetectorExecutor(exec, d)

	if wrapper == nil {
		t.Fatal("NewN1DetectorExecutor returned nil")
	}

	// The executor wraps calls - we can't test without a real DB
	// but we can verify the struct is set up correctly
	if wrapper.detector != d {
		t.Error("detector should be set")
	}
	if wrapper.executor != exec {
		t.Error("executor should be set")
	}
}

func TestN1DetectorExecutor_ExecContext(t *testing.T) {
	d := NewN1Detector(N1DetectorConfig{
		Enabled:    true,
		Threshold:  100,
		TimeWindow: 1 * time.Second,
	})

	called := false
	exec := &mockExecutor{
		execFn: func(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
			called = true
			return nil, nil
		},
	}
	wrapper := NewN1DetectorExecutor(exec, d)
	wrapper.ExecContext(context.Background(), "INSERT INTO users (name) VALUES ($1)", "John")

	if !called {
		t.Error("ExecContext should delegate to wrapped executor")
	}
	if len(d.queries) == 0 {
		t.Error("ExecContext should record query")
	}
}

func TestN1DetectorExecutor_QueryContext(t *testing.T) {
	d := NewN1Detector(N1DetectorConfig{
		Enabled:    true,
		Threshold:  100,
		TimeWindow: 1 * time.Second,
	})

	exec := &mockExecutor{
		queryFn: func(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
			return nil, fmt.Errorf("test error")
		},
	}
	wrapper := NewN1DetectorExecutor(exec, d)
	_, err := wrapper.QueryContext(context.Background(), "SELECT * FROM users")

	if err == nil {
		t.Error("should propagate error from executor")
	}
	if len(d.queries) == 0 {
		t.Error("QueryContext should record query")
	}
}

func TestN1DetectorExecutor_QueryRowContext(t *testing.T) {
	d := NewN1Detector(N1DetectorConfig{
		Enabled:    true,
		Threshold:  100,
		TimeWindow: 1 * time.Second,
	})

	exec := &mockExecutor{}
	wrapper := NewN1DetectorExecutor(exec, d)
	wrapper.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM users")

	if len(d.queries) == 0 {
		t.Error("QueryRowContext should record query")
	}
}

func TestN1Detector_CleanOldQueries(t *testing.T) {
	d := NewN1Detector(N1DetectorConfig{
		Enabled:    true,
		Threshold:  100, // High threshold to avoid detection
		TimeWindow: 10 * time.Millisecond,
	})

	// Record some queries
	d.Record("SELECT 1")
	d.Record("SELECT 2")

	// Wait for queries to expire
	time.Sleep(30 * time.Millisecond)

	// Force cleanup
	d.mu.Lock()
	d.cleanOldQueries(time.Now())
	d.mu.Unlock()

	if len(d.queries) != 0 {
		t.Errorf("cleanOldQueries should remove expired queries, got %d", len(d.queries))
	}
}
