package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/go-genus/genus"
	"github.com/go-genus/genus/core"
	"github.com/go-genus/genus/dialects/postgres"
	"github.com/go-genus/genus/query"
	_ "github.com/lib/pq"
)

// Product demonstra o uso de Optional[T] para campos nullable.
type Product struct {
	core.Model
	Name        string                  `db:"name"`
	Description core.Optional[string]   `db:"description"` // Pode ser NULL
	Price       float64                 `db:"price"`
	Discount    core.Optional[float64]  `db:"discount"`    // Pode ser NULL
	Stock       int                     `db:"stock"`
	MinStock    core.Optional[int]      `db:"min_stock"`   // Pode ser NULL
	Active      bool                    `db:"active"`
	Featured    core.Optional[bool]     `db:"featured"`    // Pode ser NULL
}

// ProductFields - campos tipados (normalmente gerados por genus generate)
var ProductFields = struct {
	ID          query.Int64Field
	Name        query.StringField
	Description query.OptionalStringField
	Price       query.Float64Field
	Discount    query.OptionalFloat64Field
	Stock       query.IntField
	MinStock    query.OptionalIntField
	Active      query.BoolField
	Featured    query.OptionalBoolField
	CreatedAt   query.StringField
	UpdatedAt   query.StringField
}{
	ID:          query.NewInt64Field("id"),
	Name:        query.NewStringField("name"),
	Description: query.NewOptionalStringField("description"),
	Price:       query.NewFloat64Field("price"),
	Discount:    query.NewOptionalFloat64Field("discount"),
	Stock:       query.NewIntField("stock"),
	MinStock:    query.NewOptionalIntField("min_stock"),
	Active:      query.NewBoolField("active"),
	Featured:    query.NewOptionalBoolField("featured"),
	CreatedAt:   query.NewStringField("created_at"),
	UpdatedAt:   query.NewStringField("updated_at"),
}

func main() {
	fmt.Println("=== Genus Optional[T] Example ===")
	fmt.Println()

	// 1. Demonstração de criação de Optional
	fmt.Println("1. Criando valores Optional:")

	// Valor presente
	description := core.Some("Um produto incrível")
	fmt.Printf("   Description: %v (presente: %v)\n", description.Get(), description.IsPresent())

	// Valor ausente
	discount := core.None[float64]()
	fmt.Printf("   Discount: presente=%v, valor=%v\n", discount.IsPresent(), discount.GetOrDefault(0.0))

	// De ponteiro
	minStockPtr := new(int)
	*minStockPtr = 10
	minStock := core.FromPtr(minStockPtr)
	fmt.Printf("   MinStock: %v (de ponteiro)\n", minStock.Get())

	minStockNil := core.FromPtr[int](nil)
	fmt.Printf("   MinStock (nil): presente=%v\n", minStockNil.IsPresent())

	// 2. Demonstração de JSON marshaling
	fmt.Println("\n2. JSON Marshaling:")

	product := Product{
		Model: core.Model{
			ID:        1,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Name:        "Laptop",
		Description: core.Some("High-performance laptop"),
		Price:       1299.99,
		Discount:    core.None[float64](), // Sem desconto
		Stock:       50,
		MinStock:    core.Some(10),
		Active:      true,
		Featured:    core.None[bool](), // Não definido
	}

	jsonData, _ := json.MarshalIndent(product, "   ", "  ")
	fmt.Printf("   Product JSON:\n   %s\n", string(jsonData))

	// 3. Demonstração de operações funcionais
	fmt.Println("\n3. Operações Funcionais:")

	// Map
	upperDesc := core.Map(description, func(s string) string {
		return fmt.Sprintf(">>> %s <<<", s)
	})
	fmt.Printf("   Map: %v\n", upperDesc.Get())

	// Filter
	highStock := minStock.Filter(func(stock int) bool {
		return stock > 5
	})
	fmt.Printf("   Filter (stock > 5): presente=%v, valor=%v\n", highStock.IsPresent(), highStock.GetOrZero())

	// IfPresent
	fmt.Print("   IfPresent: ")
	minStock.IfPresent(func(s int) {
		fmt.Printf("Min stock is %d\n", s)
	})

	// IfPresentOrElse
	fmt.Print("   IfPresentOrElse (discount): ")
	discount.IfPresentOrElse(
		func(d float64) {
			fmt.Printf("Discount: %.2f%%\n", d)
		},
		func() {
			fmt.Println("No discount available")
		},
	)

	// 4. Demonstração com banco de dados (simulado)
	fmt.Println("\n4. Uso com Banco de Dados:")
	fmt.Println("   Conectando ao banco...")

	db := setupTestDB()
	if db == nil {
		fmt.Println("   [SKIP] Database connection not available")
		fmt.Println("   To run database examples, set DATABASE_URL environment variable")
		return
	}
	defer db.Close()

	g := genus.NewWithLogger(db, postgres.New(), core.NewDefaultLogger(true))

	// Criar tabela
	createTable(db)

	// Inserir produto
	fmt.Println("\n   Inserindo produto...")
	if err := g.DB().Create(context.Background(), &product); err != nil {
		log.Printf("   Error creating product: %v", err)
		return
	}
	fmt.Printf("   Product criado com ID: %d\n", product.ID)

	// Buscar produtos com desconto
	fmt.Println("\n   Buscando produtos com desconto...")
	productsWithDiscount, err := genus.Table[Product](g).
		Where(ProductFields.Discount.IsNotNull()).
		Find(context.Background())

	if err != nil {
		log.Printf("   Error finding products: %v", err)
		return
	}
	fmt.Printf("   Produtos com desconto: %d\n", len(productsWithDiscount))

	// Buscar produtos sem descrição
	fmt.Println("\n   Buscando produtos sem descrição...")
	productsWithoutDesc, err := genus.Table[Product](g).
		Where(ProductFields.Description.IsNull()).
		Find(context.Background())

	if err != nil {
		log.Printf("   Error finding products: %v", err)
		return
	}
	fmt.Printf("   Produtos sem descrição: %d\n", len(productsWithoutDesc))

	// Atualizar produto
	fmt.Println("\n   Atualizando produto com desconto...")
	product.Discount = core.Some(15.5)
	if err := g.DB().Update(context.Background(), &product); err != nil {
		log.Printf("   Error updating product: %v", err)
		return
	}
	fmt.Println("   Produto atualizado!")

	// Buscar produto atualizado
	fmt.Println("\n   Buscando produto atualizado...")
	updated, err := genus.Table[Product](g).
		Where(ProductFields.ID.Eq(product.ID)).
		First(context.Background())

	if err != nil {
		log.Printf("   Error finding product: %v", err)
		return
	}

	fmt.Printf("   Produto: %s\n", updated.Name)
	fmt.Printf("   Desconto: %.2f%%\n", updated.Discount.GetOrDefault(0.0))

	if updated.Description.IsPresent() {
		fmt.Printf("   Descrição: %s\n", updated.Description.Get())
	}

	fmt.Println("\n=== Example completed successfully! ===")
}

func setupTestDB() *sql.DB {
	// Tenta conectar ao banco de dados
	// Se não conseguir, retorna nil (exemplo rodará em modo simulado)
	dsn := "user=postgres password=postgres dbname=genus_test sslmode=disable"
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil
	}

	return db
}

func createTable(db *sql.DB) {
	schema := `
	DROP TABLE IF EXISTS products;
	CREATE TABLE products (
		id SERIAL PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		description TEXT,
		price DECIMAL(10, 2) NOT NULL,
		discount DECIMAL(5, 2),
		stock INTEGER NOT NULL,
		min_stock INTEGER,
		active BOOLEAN NOT NULL DEFAULT true,
		featured BOOLEAN,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP NOT NULL DEFAULT NOW()
	);
	`

	if _, err := db.Exec(schema); err != nil {
		log.Printf("Warning: failed to create table: %v", err)
	}
}
