package query

import "github.com/GabrielOnRails/genus/core"

// Scope é uma função que modifica um query builder.
// Scopes podem ser usados para aplicar condições globais a queries.
type Scope[T any] func(*Builder[T]) *Builder[T]

// applySoftDeleteScope adiciona automaticamente WHERE deleted_at IS NULL
// para models que implementam SoftDeletable, a menos que scopes estejam desabilitados.
func applySoftDeleteScope[T any](b *Builder[T]) *Builder[T] {
	// Verifica se o tipo T implementa SoftDeletable
	var zero T
	if _, ok := any(zero).(core.SoftDeletable); ok {
		// Se scopes não estiverem desabilitados, adiciona condição
		if !b.disableScopes {
			return b.Where(Condition{
				Field:    "deleted_at",
				Operator: OpIsNull,
			})
		}
	}
	return b
}
