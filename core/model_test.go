package core

import (
	"testing"
	"time"
)

func TestModel_Fields(t *testing.T) {
	now := time.Now()
	m := Model{
		ID:        42,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if m.ID != 42 {
		t.Errorf("ID = %d, want 42", m.ID)
	}
	if !m.CreatedAt.Equal(now) {
		t.Error("CreatedAt mismatch")
	}
	if !m.UpdatedAt.Equal(now) {
		t.Error("UpdatedAt mismatch")
	}
}

func TestTableNamer_Interface(t *testing.T) {
	m := &TestModelWithTableName{}
	var tn TableNamer = m
	if tn.TableName() != "custom_table" {
		t.Errorf("TableName() = %q, want %q", tn.TableName(), "custom_table")
	}
}

func TestBeforeCreater_Interface(t *testing.T) {
	m := &TestModelWithHooks{}
	var bc BeforeCreater = m
	if err := bc.BeforeCreate(); err != nil {
		t.Errorf("BeforeCreate() returned error: %v", err)
	}
	if !m.beforeCreateCalled {
		t.Error("BeforeCreate() was not called")
	}
}

func TestAfterFinder_Interface(t *testing.T) {
	// Verify AfterFinder interface is defined
	type model struct {
		Model
	}
	// Just verify the interface signature compiles
	var _ AfterFinder = (*afterFinderImpl)(nil)
}

type afterFinderImpl struct{}

func (a *afterFinderImpl) AfterFind() error { return nil }
