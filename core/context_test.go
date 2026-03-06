package core

import (
	"context"
	"testing"
)

func TestWithPrimary(t *testing.T) {
	ctx := context.Background()

	// Contexto original não deve indicar uso do primary
	if UsePrimary(ctx) {
		t.Error("UsePrimary(ctx) = true, want false for background context")
	}

	// Contexto com WithPrimary deve indicar uso do primary
	primaryCtx := WithPrimary(ctx)
	if !UsePrimary(primaryCtx) {
		t.Error("UsePrimary(WithPrimary(ctx)) = false, want true")
	}

	// Contexto original não deve ser afetado
	if UsePrimary(ctx) {
		t.Error("Original context was modified")
	}
}

func TestUsePrimary_NilContext(t *testing.T) {
	//nolint:staticcheck // testing nil context behavior intentionally
	if UsePrimary(nil) {
		t.Error("UsePrimary(nil) = true, want false")
	}
}

func TestWithPrimary_Nested(t *testing.T) {
	ctx := context.Background()

	// Encadear múltiplos WithPrimary
	ctx1 := WithPrimary(ctx)
	ctx2 := WithPrimary(ctx1)

	if !UsePrimary(ctx1) {
		t.Error("UsePrimary(ctx1) = false, want true")
	}
	if !UsePrimary(ctx2) {
		t.Error("UsePrimary(ctx2) = false, want true")
	}
}

func TestWithPrimary_WithCancel(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// WithPrimary deve funcionar com contextos derivados
	primaryCtx := WithPrimary(ctx)
	if !UsePrimary(primaryCtx) {
		t.Error("UsePrimary after WithCancel = false, want true")
	}
}
