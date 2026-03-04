package core

import (
	"testing"
)

func TestDefaultBatchConfig(t *testing.T) {
	config := DefaultBatchConfig()

	if config.BatchSize != 100 {
		t.Errorf("BatchSize = %d, want 100", config.BatchSize)
	}
	if config.SkipHooks {
		t.Error("SkipHooks = true, want false")
	}
}

func TestBatchConfig_CustomValues(t *testing.T) {
	config := BatchConfig{
		BatchSize: 50,
		SkipHooks: true,
	}

	if config.BatchSize != 50 {
		t.Errorf("BatchSize = %d, want 50", config.BatchSize)
	}
	if !config.SkipHooks {
		t.Error("SkipHooks = false, want true")
	}
}
