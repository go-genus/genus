package query

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/GabrielOnRails/genus/core"
)

// Builder é o query builder genérico type-safe.
// T é o tipo do modelo sendo consultado.
type Builder[T any] struct {
	executor      core.Executor
	dialect       core.Dialect
	logger        core.Logger
	tableName     string
	conditions    []interface{} // Condition ou ConditionGroup
	orderBy       []OrderBy
	limit         *int
	offset        *int
	selectCols    []string
	disableScopes bool // Se true, não aplica scopes automáticos (como soft delete)
	joins         []JoinClause
	preloads      []string // Relacionamentos para eager loading
}

// OrderBy representa uma cláusula ORDER BY.
type OrderBy struct {
	Column string
	Desc   bool
}

// NewBuilder cria um novo query builder.
func NewBuilder[T any](executor core.Executor, dialect core.Dialect, logger core.Logger, tableName string) *Builder[T] {
	return &Builder[T]{
		executor:  executor,
		dialect:   dialect,
		logger:    logger,
		tableName: tableName,
	}
}

// clone cria uma cópia profunda do builder para garantir imutabilidade.
// Cada método que modifica o estado retorna um novo builder.
func (b *Builder[T]) clone() *Builder[T] {
	newBuilder := &Builder[T]{
		executor:      b.executor,
		dialect:       b.dialect,
		logger:        b.logger,
		tableName:     b.tableName,
		disableScopes: b.disableScopes,
	}

	// Copiar conditions
	if len(b.conditions) > 0 {
		newBuilder.conditions = make([]interface{}, len(b.conditions))
		copy(newBuilder.conditions, b.conditions)
	}

	// Copiar orderBy
	if len(b.orderBy) > 0 {
		newBuilder.orderBy = make([]OrderBy, len(b.orderBy))
		copy(newBuilder.orderBy, b.orderBy)
	}

	// Copiar limit
	if b.limit != nil {
		limitVal := *b.limit
		newBuilder.limit = &limitVal
	}

	// Copiar offset
	if b.offset != nil {
		offsetVal := *b.offset
		newBuilder.offset = &offsetVal
	}

	// Copiar selectCols
	if len(b.selectCols) > 0 {
		newBuilder.selectCols = make([]string, len(b.selectCols))
		copy(newBuilder.selectCols, b.selectCols)
	}

	// Copiar joins
	if len(b.joins) > 0 {
		newBuilder.joins = make([]JoinClause, len(b.joins))
		copy(newBuilder.joins, b.joins)
	}

	// Copiar preloads
	if len(b.preloads) > 0 {
		newBuilder.preloads = make([]string, len(b.preloads))
		copy(newBuilder.preloads, b.preloads)
	}

	return newBuilder
}

// Where adiciona uma condição WHERE.
// Aceita Condition ou ConditionGroup.
// IMUTÁVEL: Retorna um novo builder sem modificar o original.
func (b *Builder[T]) Where(condition interface{}) *Builder[T] {
	newBuilder := b.clone()
	newBuilder.conditions = append(newBuilder.conditions, condition)
	return newBuilder
}

// OrderByAsc adiciona ORDER BY ASC.
// IMUTÁVEL: Retorna um novo builder sem modificar o original.
func (b *Builder[T]) OrderByAsc(column string) *Builder[T] {
	newBuilder := b.clone()
	newBuilder.orderBy = append(newBuilder.orderBy, OrderBy{Column: column, Desc: false})
	return newBuilder
}

// OrderByDesc adiciona ORDER BY DESC.
// IMUTÁVEL: Retorna um novo builder sem modificar o original.
func (b *Builder[T]) OrderByDesc(column string) *Builder[T] {
	newBuilder := b.clone()
	newBuilder.orderBy = append(newBuilder.orderBy, OrderBy{Column: column, Desc: true})
	return newBuilder
}

// Limit define o LIMIT.
// IMUTÁVEL: Retorna um novo builder sem modificar o original.
func (b *Builder[T]) Limit(limit int) *Builder[T] {
	newBuilder := b.clone()
	newBuilder.limit = &limit
	return newBuilder
}

// Offset define o OFFSET.
// IMUTÁVEL: Retorna um novo builder sem modificar o original.
func (b *Builder[T]) Offset(offset int) *Builder[T] {
	newBuilder := b.clone()
	newBuilder.offset = &offset
	return newBuilder
}

// Select define as colunas a serem selecionadas.
// IMUTÁVEL: Retorna um novo builder sem modificar o original.
func (b *Builder[T]) Select(columns ...string) *Builder[T] {
	newBuilder := b.clone()
	newBuilder.selectCols = columns
	return newBuilder
}

// WithTrashed inclui registros soft-deleted nos resultados.
// Desabilita o scope automático de soft delete.
// IMUTÁVEL: Retorna um novo builder sem modificar o original.
func (b *Builder[T]) WithTrashed() *Builder[T] {
	newBuilder := b.clone()
	newBuilder.disableScopes = true
	return newBuilder
}

// OnlyTrashed retorna apenas registros soft-deleted.
// Adiciona condição WHERE deleted_at IS NOT NULL.
// IMUTÁVEL: Retorna um novo builder sem modificar o original.
func (b *Builder[T]) OnlyTrashed() *Builder[T] {
	newBuilder := b.clone()
	newBuilder.disableScopes = true
	return newBuilder.Where(Condition{
		Field:    "deleted_at",
		Operator: OpIsNotNull,
	})
}

// Join adiciona um INNER JOIN type-safe à query.
// O tipo genérico U especifica o model sendo joined.
// Exemplo: query.Join[Post](query.On("users.id", "posts.user_id"))
// IMUTÁVEL: Retorna um novo builder sem modificar o original.
func Join[U any](b *Builder[any], on JoinCondition) *Builder[any] {
	newBuilder := b.clone()
	var model U
	tableName := getTableName(model)

	join := JoinClause{
		Type:      InnerJoinType,
		Table:     tableName,
		Condition: on,
	}
	newBuilder.joins = append(newBuilder.joins, join)
	return newBuilder
}

// LeftJoin adiciona um LEFT JOIN type-safe à query.
// Funciona como Join mas retorna todas as linhas da tabela esquerda mesmo sem match.
// IMUTÁVEL: Retorna um novo builder sem modificar o original.
func LeftJoin[U any](b *Builder[any], on JoinCondition) *Builder[any] {
	newBuilder := b.clone()
	var model U
	tableName := getTableName(model)

	join := JoinClause{
		Type:      LeftJoinType,
		Table:     tableName,
		Condition: on,
	}
	newBuilder.joins = append(newBuilder.joins, join)
	return newBuilder
}

// RightJoin adiciona um RIGHT JOIN type-safe à query.
// Funciona como Join mas retorna todas as linhas da tabela direita mesmo sem match.
// IMUTÁVEL: Retorna um novo builder sem modificar o original.
func RightJoin[U any](b *Builder[any], on JoinCondition) *Builder[any] {
	newBuilder := b.clone()
	var model U
	tableName := getTableName(model)

	join := JoinClause{
		Type:      RightJoinType,
		Table:     tableName,
		Condition: on,
	}
	newBuilder.joins = append(newBuilder.joins, join)
	return newBuilder
}

// Preload especifica relacionamentos para eager loading.
// Evita o problema N+1 carregando relacionamentos de uma vez.
// Exemplo: Preload("Posts") ou Preload("Posts.Comments") para nested.
// IMUTÁVEL: Retorna um novo builder sem modificar o original.
func (b *Builder[T]) Preload(relation string) *Builder[T] {
	newBuilder := b.clone()
	newBuilder.preloads = append(newBuilder.preloads, relation)
	return newBuilder
}

// Find executa a query e retorna um slice de T.
// Esta é a função mágica que retorna []T sem precisar de *[]T!
func (b *Builder[T]) Find(ctx context.Context) ([]T, error) {
	// Aplica soft delete scope automaticamente se aplicável
	queryBuilder := applySoftDeleteScope(b)

	query, args := queryBuilder.buildSelectQuery()

	start := time.Now()
	rows, err := queryBuilder.executor.QueryContext(ctx, query, args...)
	duration := time.Since(start).Nanoseconds()

	if err != nil {
		b.logger.LogError(query, args, err)
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	var results []T
	for rows.Next() {
		var item T
		if err := scanStruct(rows, &item); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Hook AfterFind
		if af, ok := any(&item).(core.AfterFinder); ok {
			if err := af.AfterFind(); err != nil {
				return nil, fmt.Errorf("AfterFind hook failed: %w", err)
			}
		}

		results = append(results, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	queryBuilder.logger.LogQuery(query, args, duration)

	// Executa preloads se houver
	if len(queryBuilder.preloads) > 0 {
		specs := make([]*PreloadSpec, len(queryBuilder.preloads))
		for i, rel := range queryBuilder.preloads {
			specs[i] = parsePreloadPath(rel)
		}

		if err := executePreload(ctx, queryBuilder.executor, queryBuilder.dialect, queryBuilder.logger, results, specs); err != nil {
			return nil, fmt.Errorf("preload failed: %w", err)
		}
	}

	return results, nil
}

// First retorna o primeiro resultado ou erro se não encontrado.
// IMUTÁVEL: Cria uma cópia do builder com LIMIT 1.
func (b *Builder[T]) First(ctx context.Context) (T, error) {
	// Cria um novo builder com limit 1 sem modificar o original
	limitedBuilder := b.Limit(1)
	results, err := limitedBuilder.Find(ctx)

	var zero T
	if err != nil {
		return zero, err
	}

	if len(results) == 0 {
		return zero, fmt.Errorf("no rows found")
	}

	return results[0], nil
}

// Count retorna a contagem de registros.
func (b *Builder[T]) Count(ctx context.Context) (int64, error) {
	// Aplica soft delete scope automaticamente se aplicável
	queryBuilder := applySoftDeleteScope(b)

	query, args := queryBuilder.buildCountQuery()

	var count int64
	start := time.Now()
	err := queryBuilder.executor.QueryRowContext(ctx, query, args...).Scan(&count)
	duration := time.Since(start).Nanoseconds()

	if err != nil {
		b.logger.LogError(query, args, err)
		return 0, fmt.Errorf("failed to count: %w", err)
	}

	b.logger.LogQuery(query, args, duration)
	return count, nil
}

// buildSelectQuery constrói a query SELECT.
func (b *Builder[T]) buildSelectQuery() (string, []interface{}) {
	var sb strings.Builder
	var args []interface{}

	// SELECT
	sb.WriteString("SELECT ")
	if len(b.selectCols) > 0 {
		sb.WriteString(strings.Join(b.selectCols, ", "))
	} else {
		// Se há JOINs, qualificar com o nome da tabela para evitar ambiguidade
		if len(b.joins) > 0 {
			sb.WriteString(b.dialect.QuoteIdentifier(b.tableName))
			sb.WriteString(".*")
		} else {
			sb.WriteString("*")
		}
	}

	// FROM
	sb.WriteString(" FROM ")
	sb.WriteString(b.dialect.QuoteIdentifier(b.tableName))

	// JOINs
	for _, join := range b.joins {
		sb.WriteString(" ")
		sb.WriteString(join.BuildSQL(b.dialect))
	}

	// WHERE
	if len(b.conditions) > 0 {
		sb.WriteString(" WHERE ")
		whereSQL, whereArgs := b.buildWhereClause(b.conditions)
		sb.WriteString(whereSQL)
		args = append(args, whereArgs...)
	}

	// ORDER BY
	if len(b.orderBy) > 0 {
		sb.WriteString(" ORDER BY ")
		orderParts := make([]string, len(b.orderBy))
		for i, order := range b.orderBy {
			if order.Desc {
				orderParts[i] = order.Column + " DESC"
			} else {
				orderParts[i] = order.Column + " ASC"
			}
		}
		sb.WriteString(strings.Join(orderParts, ", "))
	}

	// LIMIT
	if b.limit != nil {
		sb.WriteString(fmt.Sprintf(" LIMIT %d", *b.limit))
	}

	// OFFSET
	if b.offset != nil {
		sb.WriteString(fmt.Sprintf(" OFFSET %d", *b.offset))
	}

	return sb.String(), args
}

// buildCountQuery constrói a query COUNT.
func (b *Builder[T]) buildCountQuery() (string, []interface{}) {
	var sb strings.Builder
	var args []interface{}

	sb.WriteString("SELECT COUNT(*) FROM ")
	sb.WriteString(b.dialect.QuoteIdentifier(b.tableName))

	if len(b.conditions) > 0 {
		sb.WriteString(" WHERE ")
		whereSQL, whereArgs := b.buildWhereClause(b.conditions)
		sb.WriteString(whereSQL)
		args = append(args, whereArgs...)
	}

	return sb.String(), args
}

// buildWhereClause constrói a cláusula WHERE.
func (b *Builder[T]) buildWhereClause(conditions []interface{}) (string, []interface{}) {
	if len(conditions) == 0 {
		return "", nil
	}

	var parts []string
	var args []interface{}
	argIndex := 1

	for _, cond := range conditions {
		switch c := cond.(type) {
		case Condition:
			sql, condArgs := b.buildCondition(c, &argIndex)
			parts = append(parts, sql)
			args = append(args, condArgs...)

		case ConditionGroup:
			sql, condArgs := b.buildConditionGroup(c, &argIndex)
			parts = append(parts, "("+sql+")")
			args = append(args, condArgs...)
		}
	}

	return strings.Join(parts, " AND "), args
}

// buildCondition constrói uma única condição.
func (b *Builder[T]) buildCondition(cond Condition, argIndex *int) (string, []interface{}) {
	var args []interface{}

	switch cond.Operator {
	case OpIsNull:
		return fmt.Sprintf("%s IS NULL", cond.Field), args

	case OpIsNotNull:
		return fmt.Sprintf("%s IS NOT NULL", cond.Field), args

	case OpIn, OpNotIn:
		values := interfaceSlice(cond.Value)
		placeholders := make([]string, len(values))
		for i, v := range values {
			placeholders[i] = b.dialect.Placeholder(*argIndex)
			args = append(args, v)
			*argIndex++
		}
		op := "IN"
		if cond.Operator == OpNotIn {
			op = "NOT IN"
		}
		return fmt.Sprintf("%s %s (%s)", cond.Field, op, strings.Join(placeholders, ", ")), args

	case OpBetween:
		values := interfaceSlice(cond.Value)
		if len(values) != 2 {
			return "", args
		}
		sql := fmt.Sprintf("%s BETWEEN %s AND %s",
			cond.Field,
			b.dialect.Placeholder(*argIndex),
			b.dialect.Placeholder(*argIndex+1))
		args = append(args, values[0], values[1])
		*argIndex += 2
		return sql, args

	default:
		sql := fmt.Sprintf("%s %s %s", cond.Field, cond.Operator, b.dialect.Placeholder(*argIndex))
		args = append(args, cond.Value)
		*argIndex++
		return sql, args
	}
}

// buildConditionGroup constrói um grupo de condições.
func (b *Builder[T]) buildConditionGroup(group ConditionGroup, argIndex *int) (string, []interface{}) {
	var parts []string
	var args []interface{}

	for _, cond := range group.Conditions {
		switch c := cond.(type) {
		case Condition:
			sql, condArgs := b.buildCondition(c, argIndex)
			parts = append(parts, sql)
			args = append(args, condArgs...)

		case ConditionGroup:
			sql, condArgs := b.buildConditionGroup(c, argIndex)
			parts = append(parts, "("+sql+")")
			args = append(args, condArgs...)
		}
	}

	operator := " AND "
	if group.Operator == LogicalOr {
		operator = " OR "
	}

	return strings.Join(parts, operator), args
}

// interfaceSlice converte diferentes tipos de slice para []interface{}.
func interfaceSlice(value interface{}) []interface{} {
	switch v := value.(type) {
	case []string:
		result := make([]interface{}, len(v))
		for i, val := range v {
			result[i] = val
		}
		return result
	case []int:
		result := make([]interface{}, len(v))
		for i, val := range v {
			result[i] = val
		}
		return result
	case []int64:
		result := make([]interface{}, len(v))
		for i, val := range v {
			result[i] = val
		}
		return result
	case []bool:
		result := make([]interface{}, len(v))
		for i, val := range v {
			result[i] = val
		}
		return result
	case []interface{}:
		return v
	default:
		return []interface{}{value}
	}
}

// getTableName retorna o nome da tabela para um model.
// Usa TableNamer se implementado, caso contrário usa o nome do tipo.
func getTableName(model interface{}) string {
	if tn, ok := model.(core.TableNamer); ok {
		return tn.TableName()
	}

	t := reflect.TypeOf(model)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// Converte para snake_case e adiciona 's'
	name := t.Name()
	var result strings.Builder
	for i, r := range name {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}
