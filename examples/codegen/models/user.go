package models

import (
	"time"

	"github.com/go-genus/genus/core"
)

// User representa um usuário do sistema.
// Este arquivo demonstra como usar tags db para code generation.
type User struct {
	core.Model
	Name      string                 `db:"name"`
	Email     string                 `db:"email"`
	Username  string                 `db:"username"`
	Bio       core.Optional[string]  `db:"bio"`
	Age       core.Optional[int]     `db:"age"`
	Verified  bool                   `db:"verified"`
	Premium   core.Optional[bool]    `db:"premium"`
	LastLogin core.Optional[int64]   `db:"last_login"`
	Rating    core.Optional[float64] `db:"rating"`
}

// TableName define o nome da tabela no banco de dados.
func (User) TableName() string {
	return "users"
}

// BeforeCreate é chamado antes de criar o usuário.
func (u *User) BeforeCreate() error {
	// Valida que o email não está vazio
	if u.Email == "" {
		return core.ErrValidation
	}
	return nil
}

// Product representa um produto no sistema.
type Product struct {
	core.Model
	Name        string                 `db:"name"`
	SKU         string                 `db:"sku"`
	Description core.Optional[string]  `db:"description"`
	Price       float64                `db:"price"`
	Discount    core.Optional[float64] `db:"discount"`
	Stock       int                    `db:"stock"`
	MinStock    core.Optional[int]     `db:"min_stock"`
	Active      bool                   `db:"active"`
	Featured    core.Optional[bool]    `db:"featured"`
}

// Category representa uma categoria de produtos.
type Category struct {
	ID          int64                 `db:"id"`
	Name        string                `db:"name"`
	Slug        string                `db:"slug"`
	Description core.Optional[string] `db:"description"`
	ParentID    core.Optional[int64]  `db:"parent_id"`
	Active      bool                  `db:"active"`
	CreatedAt   time.Time             `db:"created_at"`
	UpdatedAt   time.Time             `db:"updated_at"`
}

// Order representa um pedido.
type Order struct {
	core.Model
	UserID      int64                 `db:"user_id"`
	Status      string                `db:"status"`
	Total       float64               `db:"total"`
	Notes       core.Optional[string] `db:"notes"`
	ShippedAt   core.Optional[int64]  `db:"shipped_at"`
	DeliveredAt core.Optional[int64]  `db:"delivered_at"`
	Cancelled   bool                  `db:"cancelled"`
}
