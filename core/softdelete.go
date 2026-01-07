package core

import "time"

// SoftDeletable é uma interface implementada por models que suportam soft delete.
// Models que implementam esta interface terão DELETE convertido em UPDATE (set deleted_at).
type SoftDeletable interface {
	GetDeletedAt() *time.Time
	SetDeletedAt(t *time.Time)
	IsDeleted() bool
}

// SoftDeleteModel é um model base que adiciona suporte a soft delete.
// Use este ao invés de Model se quiser soft delete automático.
type SoftDeleteModel struct {
	Model
	DeletedAt Optional[time.Time] `db:"deleted_at"`
}

// GetDeletedAt retorna o timestamp de quando o registro foi deletado (soft delete).
func (m *SoftDeleteModel) GetDeletedAt() *time.Time {
	return m.DeletedAt.Ptr()
}

// SetDeletedAt define o timestamp de delete.
// Passar nil restaura o registro (undelete).
func (m *SoftDeleteModel) SetDeletedAt(t *time.Time) {
	if t == nil {
		m.DeletedAt = None[time.Time]()
	} else {
		m.DeletedAt = Some(*t)
	}
}

// IsDeleted verifica se o registro foi soft deleted.
func (m *SoftDeleteModel) IsDeleted() bool {
	return m.DeletedAt.IsPresent()
}
