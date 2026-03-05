package core

import (
	"testing"
	"time"
)

func TestDefaultPoolConfig(t *testing.T) {
	config := DefaultPoolConfig()

	if config.MaxOpenConns != 25 {
		t.Errorf("MaxOpenConns = %d, want 25", config.MaxOpenConns)
	}
	if config.MaxIdleConns != 10 {
		t.Errorf("MaxIdleConns = %d, want 10", config.MaxIdleConns)
	}
	if config.ConnMaxLifetime != 30*time.Minute {
		t.Errorf("ConnMaxLifetime = %v, want 30m", config.ConnMaxLifetime)
	}
	if config.ConnMaxIdleTime != 5*time.Minute {
		t.Errorf("ConnMaxIdleTime = %v, want 5m", config.ConnMaxIdleTime)
	}
}

func TestHighPerformancePoolConfig(t *testing.T) {
	config := HighPerformancePoolConfig()

	if config.MaxOpenConns != 100 {
		t.Errorf("MaxOpenConns = %d, want 100", config.MaxOpenConns)
	}
	if config.MaxIdleConns != 50 {
		t.Errorf("MaxIdleConns = %d, want 50", config.MaxIdleConns)
	}
	if config.ConnMaxLifetime != time.Hour {
		t.Errorf("ConnMaxLifetime = %v, want 1h", config.ConnMaxLifetime)
	}
	if config.ConnMaxIdleTime != 10*time.Minute {
		t.Errorf("ConnMaxIdleTime = %v, want 10m", config.ConnMaxIdleTime)
	}
}

func TestMinimalPoolConfig(t *testing.T) {
	config := MinimalPoolConfig()

	if config.MaxOpenConns != 5 {
		t.Errorf("MaxOpenConns = %d, want 5", config.MaxOpenConns)
	}
	if config.MaxIdleConns != 2 {
		t.Errorf("MaxIdleConns = %d, want 2", config.MaxIdleConns)
	}
	if config.ConnMaxLifetime != 15*time.Minute {
		t.Errorf("ConnMaxLifetime = %v, want 15m", config.ConnMaxLifetime)
	}
	if config.ConnMaxIdleTime != 2*time.Minute {
		t.Errorf("ConnMaxIdleTime = %v, want 2m", config.ConnMaxIdleTime)
	}
}

func TestPoolConfig_With_Methods(t *testing.T) {
	config := DefaultPoolConfig()

	// Test WithMaxOpenConns
	newConfig := config.WithMaxOpenConns(50)
	if newConfig.MaxOpenConns != 50 {
		t.Errorf("WithMaxOpenConns: MaxOpenConns = %d, want 50", newConfig.MaxOpenConns)
	}
	// Original should be unchanged (immutability check)
	if config.MaxOpenConns != 25 {
		t.Errorf("Original config changed: MaxOpenConns = %d, want 25", config.MaxOpenConns)
	}

	// Test WithMaxIdleConns
	newConfig = config.WithMaxIdleConns(20)
	if newConfig.MaxIdleConns != 20 {
		t.Errorf("WithMaxIdleConns: MaxIdleConns = %d, want 20", newConfig.MaxIdleConns)
	}

	// Test WithConnMaxLifetime
	newConfig = config.WithConnMaxLifetime(time.Hour)
	if newConfig.ConnMaxLifetime != time.Hour {
		t.Errorf("WithConnMaxLifetime: ConnMaxLifetime = %v, want 1h", newConfig.ConnMaxLifetime)
	}

	// Test WithConnMaxIdleTime
	newConfig = config.WithConnMaxIdleTime(10 * time.Minute)
	if newConfig.ConnMaxIdleTime != 10*time.Minute {
		t.Errorf("WithConnMaxIdleTime: ConnMaxIdleTime = %v, want 10m", newConfig.ConnMaxIdleTime)
	}
}

func TestPoolConfig_Chaining(t *testing.T) {
	config := DefaultPoolConfig().
		WithMaxOpenConns(100).
		WithMaxIdleConns(50).
		WithConnMaxLifetime(time.Hour).
		WithConnMaxIdleTime(15 * time.Minute)

	if config.MaxOpenConns != 100 {
		t.Errorf("MaxOpenConns = %d, want 100", config.MaxOpenConns)
	}
	if config.MaxIdleConns != 50 {
		t.Errorf("MaxIdleConns = %d, want 50", config.MaxIdleConns)
	}
	if config.ConnMaxLifetime != time.Hour {
		t.Errorf("ConnMaxLifetime = %v, want 1h", config.ConnMaxLifetime)
	}
	if config.ConnMaxIdleTime != 15*time.Minute {
		t.Errorf("ConnMaxIdleTime = %v, want 15m", config.ConnMaxIdleTime)
	}
}

func TestPoolConfig_Apply(t *testing.T) {
	sqlDB := getMockSQLDB()
	defer sqlDB.Close()

	config := DefaultPoolConfig()
	config.Apply(sqlDB)

	stats := sqlDB.Stats()
	// Just verify it doesn't panic and stats are accessible
	_ = stats
}

func TestPoolConfig_Apply_ZeroValues(t *testing.T) {
	sqlDB := getMockSQLDB()
	defer sqlDB.Close()

	config := PoolConfig{
		MaxOpenConns:    0,
		MaxIdleConns:    0,
		ConnMaxLifetime: 0,
		ConnMaxIdleTime: 0,
	}
	// Should not set anything when values are <= 0
	config.Apply(sqlDB)
}

func TestPoolConfig_Apply_AllPositive(t *testing.T) {
	sqlDB := getMockSQLDB()
	defer sqlDB.Close()

	config := PoolConfig{
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 30 * time.Minute,
		ConnMaxIdleTime: 5 * time.Minute,
	}
	config.Apply(sqlDB)
}
