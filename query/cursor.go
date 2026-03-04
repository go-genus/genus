package query

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/GabrielOnRails/genus/core"
)

// Cursor representa um cursor opaco para paginação.
type Cursor string

// CursorPage representa uma página de resultados com cursors.
type CursorPage[T any] struct {
	// Items são os itens da página atual.
	Items []T

	// HasNextPage indica se há mais itens após esta página.
	HasNextPage bool

	// HasPreviousPage indica se há itens antes desta página.
	HasPreviousPage bool

	// StartCursor é o cursor do primeiro item.
	StartCursor Cursor

	// EndCursor é o cursor do último item.
	EndCursor Cursor

	// TotalCount é o total de itens (opcional, requer query adicional).
	TotalCount *int64
}

// CursorConfig configura a paginação por cursor.
type CursorConfig struct {
	// OrderBy é a coluna usada para ordenação (obrigatório).
	// Deve ser uma coluna com valores únicos e ordenáveis (ex: id, created_at).
	OrderBy string

	// OrderDesc indica se a ordenação é descendente.
	OrderDesc bool

	// First retorna os primeiros N itens após o cursor.
	First int

	// After é o cursor após o qual buscar itens.
	After Cursor

	// Last retorna os últimos N itens antes do cursor.
	Last int

	// Before é o cursor antes do qual buscar itens.
	Before Cursor

	// IncludeTotalCount indica se deve contar o total (query adicional).
	IncludeTotalCount bool
}

// cursorData armazena os dados internos do cursor.
type cursorData struct {
	Column string      `json:"c"`
	Value  interface{} `json:"v"`
	ID     int64       `json:"i,omitempty"`
}

// EncodeCursor cria um cursor a partir de um valor.
func EncodeCursor(column string, value interface{}, id int64) Cursor {
	data := cursorData{
		Column: column,
		Value:  value,
		ID:     id,
	}
	jsonBytes, _ := json.Marshal(data)
	return Cursor(base64.URLEncoding.EncodeToString(jsonBytes))
}

// DecodeCursor decodifica um cursor.
func DecodeCursor(cursor Cursor) (*cursorData, error) {
	if cursor == "" {
		return nil, nil
	}

	jsonBytes, err := base64.URLEncoding.DecodeString(string(cursor))
	if err != nil {
		return nil, fmt.Errorf("invalid cursor format: %w", err)
	}

	var data cursorData
	if err := json.Unmarshal(jsonBytes, &data); err != nil {
		return nil, fmt.Errorf("invalid cursor data: %w", err)
	}

	return &data, nil
}

// IsValid verifica se o cursor é válido.
func (c Cursor) IsValid() bool {
	if c == "" {
		return true // Empty cursor is valid (first page)
	}
	_, err := DecodeCursor(c)
	return err == nil
}

// CursorBuilder adiciona suporte a cursor pagination ao Builder.
type CursorBuilder[T any] struct {
	builder *Builder[T]
	config  CursorConfig
}

// Paginate retorna um CursorBuilder para paginação por cursor.
//
// Exemplo:
//
//	page, _ := genus.Table[User](db).
//	    Where(UserFields.Active.Eq(true)).
//	    Paginate(query.CursorConfig{
//	        OrderBy: "created_at",
//	        OrderDesc: true,
//	        First: 10,
//	    }).
//	    Fetch(ctx)
//
//	for _, user := range page.Items {
//	    fmt.Println(user.Name)
//	}
//
//	// Próxima página
//	if page.HasNextPage {
//	    nextPage, _ := genus.Table[User](db).
//	        Paginate(query.CursorConfig{
//	            OrderBy: "created_at",
//	            OrderDesc: true,
//	            First: 10,
//	            After: page.EndCursor,
//	        }).
//	        Fetch(ctx)
//	}
func (b *Builder[T]) Paginate(config CursorConfig) *CursorBuilder[T] {
	return &CursorBuilder[T]{
		builder: b.clone(),
		config:  config,
	}
}

// Fetch executa a query e retorna uma página de resultados.
func (cb *CursorBuilder[T]) Fetch(ctx context.Context) (*CursorPage[T], error) {
	config := cb.config

	// Validação
	if config.OrderBy == "" {
		return nil, fmt.Errorf("OrderBy is required for cursor pagination")
	}

	if config.First > 0 && config.Last > 0 {
		return nil, fmt.Errorf("cannot use both First and Last")
	}

	if config.After != "" && config.Before != "" {
		return nil, fmt.Errorf("cannot use both After and Before")
	}

	limit := config.First
	if config.Last > 0 {
		limit = config.Last
	}
	if limit <= 0 {
		limit = 20 // Default
	}

	// Busca um item extra para determinar hasNextPage/hasPreviousPage
	queryLimit := limit + 1

	// Constrói a query
	queryBuilder := cb.builder.clone()

	// Aplica condição do cursor
	if config.After != "" {
		cursorData, err := DecodeCursor(config.After)
		if err != nil {
			return nil, err
		}
		if cursorData != nil {
			if config.OrderDesc {
				queryBuilder = queryBuilder.Where(Condition{
					Field:    config.OrderBy,
					Operator: OpLt,
					Value:    cursorData.Value,
				})
			} else {
				queryBuilder = queryBuilder.Where(Condition{
					Field:    config.OrderBy,
					Operator: OpGt,
					Value:    cursorData.Value,
				})
			}
		}
	}

	if config.Before != "" {
		cursorData, err := DecodeCursor(config.Before)
		if err != nil {
			return nil, err
		}
		if cursorData != nil {
			if config.OrderDesc {
				queryBuilder = queryBuilder.Where(Condition{
					Field:    config.OrderBy,
					Operator: OpGt,
					Value:    cursorData.Value,
				})
			} else {
				queryBuilder = queryBuilder.Where(Condition{
					Field:    config.OrderBy,
					Operator: OpLt,
					Value:    cursorData.Value,
				})
			}
		}
	}

	// Aplica ordenação
	if config.Last > 0 {
		// Para Last, invertemos a ordenação e depois revertemos os resultados
		if config.OrderDesc {
			queryBuilder = queryBuilder.OrderByAsc(config.OrderBy)
		} else {
			queryBuilder = queryBuilder.OrderByDesc(config.OrderBy)
		}
	} else {
		if config.OrderDesc {
			queryBuilder = queryBuilder.OrderByDesc(config.OrderBy)
		} else {
			queryBuilder = queryBuilder.OrderByAsc(config.OrderBy)
		}
	}

	// Aplica limit
	queryBuilder = queryBuilder.Limit(queryLimit)

	// Executa a query
	items, err := queryBuilder.Find(ctx)
	if err != nil {
		return nil, err
	}

	// Determina se há mais páginas
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}

	// Para Last, reverte os resultados
	if config.Last > 0 {
		reverseSlice(items)
	}

	// Constrói a página
	page := &CursorPage[T]{
		Items: items,
	}

	if config.First > 0 || config.After != "" {
		page.HasNextPage = hasMore
		page.HasPreviousPage = config.After != ""
	}

	if config.Last > 0 || config.Before != "" {
		page.HasPreviousPage = hasMore
		page.HasNextPage = config.Before != ""
	}

	// Gera cursors
	if len(items) > 0 {
		page.StartCursor = cb.generateCursor(items[0])
		page.EndCursor = cb.generateCursor(items[len(items)-1])
	}

	// Total count (opcional)
	if config.IncludeTotalCount {
		count, err := cb.builder.Count(ctx)
		if err == nil {
			page.TotalCount = &count
		}
	}

	return page, nil
}

// generateCursor gera um cursor para um item.
func (cb *CursorBuilder[T]) generateCursor(item T) Cursor {
	val := reflect.ValueOf(item)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	// Tenta obter o valor da coluna de ordenação
	var cursorValue interface{}
	var id int64

	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		dbTag := field.Tag.Get("db")

		if dbTag == cb.config.OrderBy {
			cursorValue = val.Field(i).Interface()
		}

		if dbTag == "id" || field.Name == "ID" {
			if val.Field(i).Kind() == reflect.Int64 {
				id = val.Field(i).Int()
			}
		}
	}

	// Se não encontrou diretamente, tenta em structs embedded
	if cursorValue == nil {
		cursorValue = cb.findFieldValue(val, cb.config.OrderBy)
	}
	if id == 0 {
		if idVal := cb.findFieldValue(val, "id"); idVal != nil {
			if v, ok := idVal.(int64); ok {
				id = v
			}
		}
	}

	return EncodeCursor(cb.config.OrderBy, cursorValue, id)
}

// findFieldValue busca um valor de campo em structs embedded.
func (cb *CursorBuilder[T]) findFieldValue(val reflect.Value, dbTag string) interface{} {
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return nil
	}

	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		fieldVal := val.Field(i)

		if field.Tag.Get("db") == dbTag {
			return fieldVal.Interface()
		}

		// Verifica embedded structs
		if field.Anonymous && fieldVal.Kind() == reflect.Struct {
			if result := cb.findFieldValue(fieldVal, dbTag); result != nil {
				return result
			}
		}
	}

	return nil
}

// reverseSlice reverte um slice in-place.
func reverseSlice[T any](slice []T) {
	for i, j := 0, len(slice)-1; i < j; i, j = i+1, j-1 {
		slice[i], slice[j] = slice[j], slice[i]
	}
}

// PageInfo é compatível com GraphQL Relay spec.
type PageInfo struct {
	HasNextPage     bool   `json:"hasNextPage"`
	HasPreviousPage bool   `json:"hasPreviousPage"`
	StartCursor     Cursor `json:"startCursor,omitempty"`
	EndCursor       Cursor `json:"endCursor,omitempty"`
}

// Edge representa uma edge no padrão Relay.
type Edge[T any] struct {
	Node   T      `json:"node"`
	Cursor Cursor `json:"cursor"`
}

// Connection representa uma connection no padrão Relay.
type Connection[T any] struct {
	Edges      []Edge[T] `json:"edges"`
	PageInfo   PageInfo  `json:"pageInfo"`
	TotalCount *int64    `json:"totalCount,omitempty"`
}

// ToConnection converte CursorPage para o formato Relay Connection.
func (p *CursorPage[T]) ToConnection(cursorFunc func(T) Cursor) Connection[T] {
	edges := make([]Edge[T], len(p.Items))
	for i, item := range p.Items {
		edges[i] = Edge[T]{
			Node:   item,
			Cursor: cursorFunc(item),
		}
	}

	return Connection[T]{
		Edges: edges,
		PageInfo: PageInfo{
			HasNextPage:     p.HasNextPage,
			HasPreviousPage: p.HasPreviousPage,
			StartCursor:     p.StartCursor,
			EndCursor:       p.EndCursor,
		},
		TotalCount: p.TotalCount,
	}
}

// buildCursorSQL constrói a cláusula SQL para paginação por cursor.
func (b *Builder[T]) buildCursorSQL(config CursorConfig, dialect core.Dialect) (string, []interface{}, error) {
	var parts []string
	var args []interface{}
	argIndex := 1

	// WHERE clause para cursor
	if config.After != "" {
		data, err := DecodeCursor(config.After)
		if err != nil {
			return "", nil, err
		}
		if data != nil {
			op := ">"
			if config.OrderDesc {
				op = "<"
			}
			parts = append(parts, fmt.Sprintf("%s %s %s",
				dialect.QuoteIdentifier(config.OrderBy),
				op,
				dialect.Placeholder(argIndex)))
			args = append(args, data.Value)
			argIndex++
		}
	}

	if config.Before != "" {
		data, err := DecodeCursor(config.Before)
		if err != nil {
			return "", nil, err
		}
		if data != nil {
			op := "<"
			if config.OrderDesc {
				op = ">"
			}
			parts = append(parts, fmt.Sprintf("%s %s %s",
				dialect.QuoteIdentifier(config.OrderBy),
				op,
				dialect.Placeholder(argIndex)))
			args = append(args, data.Value)
			argIndex++
		}
	}

	return strings.Join(parts, " AND "), args, nil
}
