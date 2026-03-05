package query

import (
	"testing"
	"time"
)

func TestBuilder_WithTimeout(t *testing.T) {
	b := newTestBuilder()
	tb := b.WithTimeout(5 * time.Second)

	if tb == nil {
		t.Fatal("WithTimeout returned nil")
	}
	if tb.timeout != 5*time.Second {
		t.Errorf("timeout = %v, want 5s", tb.timeout)
	}
	if tb.builder == nil {
		t.Error("builder should not be nil")
	}
}

func TestSetDefaultQueryTimeout(t *testing.T) {
	original := DefaultQueryTimeout
	defer func() { DefaultQueryTimeout = original }()

	SetDefaultQueryTimeout(10 * time.Second)
	if DefaultQueryTimeout != 10*time.Second {
		t.Errorf("DefaultQueryTimeout = %v, want 10s", DefaultQueryTimeout)
	}

	SetDefaultQueryTimeout(0)
	if DefaultQueryTimeout != 0 {
		t.Errorf("DefaultQueryTimeout = %v, want 0", DefaultQueryTimeout)
	}
}
