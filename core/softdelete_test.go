package core

import (
	"testing"
	"time"
)

func TestSoftDeleteModel_GetDeletedAt_Nil(t *testing.T) {
	m := &SoftDeleteModel{}
	if got := m.GetDeletedAt(); got != nil {
		t.Errorf("GetDeletedAt() = %v, want nil", got)
	}
}

func TestSoftDeleteModel_SetDeletedAt(t *testing.T) {
	m := &SoftDeleteModel{}
	now := time.Now()
	m.SetDeletedAt(&now)

	got := m.GetDeletedAt()
	if got == nil {
		t.Fatal("GetDeletedAt() should not be nil after SetDeletedAt")
	}
	if !got.Equal(now) {
		t.Errorf("GetDeletedAt() = %v, want %v", got, now)
	}
}

func TestSoftDeleteModel_SetDeletedAt_Nil(t *testing.T) {
	m := &SoftDeleteModel{}
	now := time.Now()
	m.SetDeletedAt(&now)
	m.SetDeletedAt(nil)

	if m.GetDeletedAt() != nil {
		t.Error("GetDeletedAt() should be nil after SetDeletedAt(nil)")
	}
}

func TestSoftDeleteModel_IsDeleted(t *testing.T) {
	m := &SoftDeleteModel{}
	if m.IsDeleted() {
		t.Error("new model should not be deleted")
	}

	now := time.Now()
	m.SetDeletedAt(&now)
	if !m.IsDeleted() {
		t.Error("model with deleted_at should be deleted")
	}

	m.SetDeletedAt(nil)
	if m.IsDeleted() {
		t.Error("model with nil deleted_at should not be deleted")
	}
}

func TestSoftDeleteModel_ImplementsSoftDeletable(t *testing.T) {
	var _ SoftDeletable = &SoftDeleteModel{}
}

func TestSoftDeleteModel_HasModelFields(t *testing.T) {
	m := &SoftDeleteModel{}
	m.ID = 1
	m.CreatedAt = time.Now()
	m.UpdatedAt = time.Now()

	if m.ID != 1 {
		t.Errorf("ID = %d, want 1", m.ID)
	}
}

func TestTestSoftDeleteModel_TableName(t *testing.T) {
	m := &TestSoftDeleteModel{}
	if m.TableName() != "soft_delete_models" {
		t.Errorf("TableName() = %q, want %q", m.TableName(), "soft_delete_models")
	}
}
