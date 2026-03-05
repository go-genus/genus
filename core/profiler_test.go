package core

import (
	"context"
	"testing"
	"time"
)

func TestDefaultProfilerConfig(t *testing.T) {
	config := DefaultProfilerConfig()
	if config.Enabled {
		t.Error("Enabled should be false")
	}
	if config.SlowQueryThreshold != 100*time.Millisecond {
		t.Errorf("SlowQueryThreshold = %v, want 100ms", config.SlowQueryThreshold)
	}
	if config.MaxQueries != 1000 {
		t.Errorf("MaxQueries = %d, want 1000", config.MaxQueries)
	}
	if config.EnableStackTrace {
		t.Error("EnableStackTrace should be false")
	}
}

func TestNewProfiler(t *testing.T) {
	p := NewProfiler(DefaultProfilerConfig())
	if p == nil {
		t.Fatal("NewProfiler returned nil")
	}
}

func TestNewProfiler_DefaultMaxQueries(t *testing.T) {
	p := NewProfiler(ProfilerConfig{MaxQueries: 0})
	if p.config.MaxQueries != 1000 {
		t.Errorf("MaxQueries = %d, want 1000 (default)", p.config.MaxQueries)
	}
}

func TestProfiler_Record_Disabled(t *testing.T) {
	p := NewProfiler(ProfilerConfig{Enabled: false, MaxQueries: 100})
	p.Record(QueryStats{SQL: "SELECT 1"})

	if len(p.GetStats()) != 0 {
		t.Error("disabled profiler should not record stats")
	}
}

func TestProfiler_Record_Enabled(t *testing.T) {
	p := NewProfiler(ProfilerConfig{
		Enabled:            true,
		SlowQueryThreshold: 100 * time.Millisecond,
		MaxQueries:         100,
	})

	p.Record(QueryStats{
		SQL:      "SELECT 1",
		Duration: 50 * time.Millisecond,
	})

	stats := p.GetStats()
	if len(stats) != 1 {
		t.Fatalf("expected 1 stat, got %d", len(stats))
	}
	if stats[0].IsSlow {
		t.Error("50ms query should not be slow with 100ms threshold")
	}
}

func TestProfiler_Record_SlowQuery(t *testing.T) {
	slowQueryCalled := false
	p := NewProfiler(ProfilerConfig{
		Enabled:            true,
		SlowQueryThreshold: 10 * time.Millisecond,
		MaxQueries:         100,
		OnSlowQuery: func(stats QueryStats) {
			slowQueryCalled = true
		},
	})

	p.Record(QueryStats{
		SQL:      "SELECT sleep(1)",
		Duration: 200 * time.Millisecond,
	})

	stats := p.GetStats()
	if len(stats) != 1 {
		t.Fatal("expected 1 stat")
	}
	if !stats[0].IsSlow {
		t.Error("200ms query should be slow with 10ms threshold")
	}
	if !slowQueryCalled {
		t.Error("OnSlowQuery callback should have been called")
	}
}

func TestProfiler_Record_MaxQueries(t *testing.T) {
	p := NewProfiler(ProfilerConfig{
		Enabled:    true,
		MaxQueries: 10,
	})

	for i := 0; i < 15; i++ {
		p.Record(QueryStats{SQL: "SELECT 1", Duration: time.Millisecond})
	}

	stats := p.GetStats()
	if len(stats) > 10 {
		t.Errorf("len(stats) = %d, should be <= 10", len(stats))
	}
}

func TestProfiler_GetSlowQueries(t *testing.T) {
	p := NewProfiler(ProfilerConfig{
		Enabled:            true,
		SlowQueryThreshold: 50 * time.Millisecond,
		MaxQueries:         100,
	})

	p.Record(QueryStats{SQL: "fast", Duration: 10 * time.Millisecond})
	p.Record(QueryStats{SQL: "slow1", Duration: 100 * time.Millisecond})
	p.Record(QueryStats{SQL: "slow2", Duration: 200 * time.Millisecond})

	slow := p.GetSlowQueries()
	if len(slow) != 2 {
		t.Errorf("expected 2 slow queries, got %d", len(slow))
	}
}

func TestProfiler_Clear(t *testing.T) {
	p := NewProfiler(ProfilerConfig{Enabled: true, MaxQueries: 100})
	p.Record(QueryStats{SQL: "SELECT 1"})
	p.Clear()

	if len(p.GetStats()) != 0 {
		t.Error("Clear should remove all stats")
	}
}

func TestProfiler_Summary_Empty(t *testing.T) {
	p := NewProfiler(ProfilerConfig{Enabled: true, MaxQueries: 100})
	summary := p.Summary()
	if summary.TotalQueries != 0 {
		t.Errorf("TotalQueries = %d, want 0", summary.TotalQueries)
	}
}

func TestProfiler_Summary(t *testing.T) {
	p := NewProfiler(ProfilerConfig{
		Enabled:            true,
		SlowQueryThreshold: 50 * time.Millisecond,
		MaxQueries:         100,
	})

	now := time.Now()
	p.Record(QueryStats{SQL: "q1", Duration: 10 * time.Millisecond, Timestamp: now})
	p.Record(QueryStats{SQL: "q2", Duration: 100 * time.Millisecond, Timestamp: now.Add(time.Second)})
	p.Record(QueryStats{SQL: "q3", Duration: 30 * time.Millisecond, Timestamp: now.Add(2 * time.Second)})

	summary := p.Summary()
	if summary.TotalQueries != 3 {
		t.Errorf("TotalQueries = %d, want 3", summary.TotalQueries)
	}
	if summary.SlowQueries != 1 {
		t.Errorf("SlowQueries = %d, want 1", summary.SlowQueries)
	}
	if summary.MaxDuration != 100*time.Millisecond {
		t.Errorf("MaxDuration = %v, want 100ms", summary.MaxDuration)
	}
	if summary.MinDuration != 10*time.Millisecond {
		t.Errorf("MinDuration = %v, want 10ms", summary.MinDuration)
	}
	if summary.QueriesPerSecond == 0 {
		t.Error("QueriesPerSecond should be > 0")
	}
}

func TestProfiler_Enable_Disable(t *testing.T) {
	p := NewProfiler(ProfilerConfig{Enabled: false, MaxQueries: 100})

	if p.IsEnabled() {
		t.Error("should be disabled")
	}

	p.Enable()
	if !p.IsEnabled() {
		t.Error("should be enabled after Enable()")
	}

	p.Disable()
	if p.IsEnabled() {
		t.Error("should be disabled after Disable()")
	}
}

func TestProfiler_SetSlowQueryThreshold(t *testing.T) {
	p := NewProfiler(ProfilerConfig{
		Enabled:            true,
		SlowQueryThreshold: 100 * time.Millisecond,
		MaxQueries:         100,
	})

	p.SetSlowQueryThreshold(10 * time.Millisecond)
	p.Record(QueryStats{SQL: "q1", Duration: 50 * time.Millisecond})

	stats := p.GetStats()
	if len(stats) != 1 || !stats[0].IsSlow {
		t.Error("50ms query should be slow with 10ms threshold")
	}
}

func TestProfiler_SetOnSlowQuery(t *testing.T) {
	p := NewProfiler(ProfilerConfig{
		Enabled:            true,
		SlowQueryThreshold: 10 * time.Millisecond,
		MaxQueries:         100,
	})

	called := false
	p.SetOnSlowQuery(func(stats QueryStats) {
		called = true
	})

	p.Record(QueryStats{SQL: "slow", Duration: 100 * time.Millisecond})
	if !called {
		t.Error("SetOnSlowQuery callback should be called")
	}
}

// ProfiledExecutor tests

func TestNewProfiledExecutor(t *testing.T) {
	exec := newMockExecutor()
	profiler := NewProfiler(ProfilerConfig{Enabled: true, MaxQueries: 100})
	pe := NewProfiledExecutor(exec, profiler)
	if pe == nil {
		t.Fatal("NewProfiledExecutor returned nil")
	}
}

func TestProfiledExecutor_ExecContext(t *testing.T) {
	exec := newMockExecutor()
	profiler := NewProfiler(ProfilerConfig{Enabled: true, MaxQueries: 100})
	pe := NewProfiledExecutor(exec, profiler)

	result, err := pe.ExecContext(context.Background(), "INSERT INTO t VALUES (1)")
	if err != nil {
		t.Errorf("ExecContext error = %v", err)
	}
	if result == nil {
		t.Error("result should not be nil")
	}

	stats := profiler.GetStats()
	if len(stats) != 1 {
		t.Errorf("expected 1 stat, got %d", len(stats))
	}
	if stats[0].SQL != "INSERT INTO t VALUES (1)" {
		t.Errorf("SQL = %q", stats[0].SQL)
	}
}

func TestProfiledExecutor_ExecContext_Error(t *testing.T) {
	exec := newMockExecutor()
	exec.execErr = errTest
	profiler := NewProfiler(ProfilerConfig{Enabled: true, MaxQueries: 100})
	pe := NewProfiledExecutor(exec, profiler)

	_, err := pe.ExecContext(context.Background(), "INSERT INTO t VALUES (1)")
	if err != errTest {
		t.Errorf("error = %v, want %v", err, errTest)
	}

	stats := profiler.GetStats()
	if len(stats) != 1 {
		t.Fatal("should still record stats on error")
	}
	if stats[0].Error != errTest {
		t.Error("stats should contain the error")
	}
}

func TestProfiledExecutor_QueryContext(t *testing.T) {
	exec := newMockExecutor()
	profiler := NewProfiler(ProfilerConfig{Enabled: true, MaxQueries: 100})
	pe := NewProfiledExecutor(exec, profiler)

	_, err := pe.QueryContext(context.Background(), "SELECT 1")
	if err != nil {
		t.Errorf("QueryContext error = %v", err)
	}

	stats := profiler.GetStats()
	if len(stats) != 1 {
		t.Errorf("expected 1 stat, got %d", len(stats))
	}
}

func TestProfiledExecutor_QueryRowContext(t *testing.T) {
	exec := newMockExecutor()
	profiler := NewProfiler(ProfilerConfig{Enabled: true, MaxQueries: 100})
	pe := NewProfiledExecutor(exec, profiler)

	pe.QueryRowContext(context.Background(), "SELECT 1")

	stats := profiler.GetStats()
	if len(stats) != 1 {
		t.Errorf("expected 1 stat, got %d", len(stats))
	}
}

func TestProfiledExecutor_ImplementsExecutor(t *testing.T) {
	exec := newMockExecutor()
	profiler := NewProfiler(DefaultProfilerConfig())
	pe := NewProfiledExecutor(exec, profiler)
	var _ Executor = pe
}
