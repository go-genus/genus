package core

import "context"

// contextKey é um tipo para chaves de contexto, evitando colisões.
type contextKey string

const (
	// usePrimaryKey é a chave de contexto para forçar uso do primary.
	usePrimaryKey contextKey = "genus_use_primary"
)

// WithPrimary retorna um contexto que força o uso do primary database.
// Útil quando você precisa ler dados que acabaram de ser escritos
// (read-after-write consistency).
//
// Exemplo:
//
//	// Criar um registro
//	db.DB().Create(ctx, &user)
//
//	// Ler imediatamente do primary para garantir consistência
//	users, _ := genus.Table[User](db).
//	    Where(UserFields.ID.Eq(user.ID)).
//	    Find(core.WithPrimary(ctx))
func WithPrimary(ctx context.Context) context.Context {
	return context.WithValue(ctx, usePrimaryKey, true)
}

// UsePrimary verifica se o contexto indica uso do primary database.
func UsePrimary(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	usePrimary, ok := ctx.Value(usePrimaryKey).(bool)
	return ok && usePrimary
}
