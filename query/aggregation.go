package query

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/GabrielOnRails/genus/core"
)

// AggregateFunc representa uma função de agregação SQL.
type AggregateFunc string

const (
	AggregateFuncCount AggregateFunc = "COUNT"
	AggregateFuncSum   AggregateFunc = "SUM"
	AggregateFuncAvg   AggregateFunc = "AVG"
	AggregateFuncMax   AggregateFunc = "MAX"
	AggregateFuncMin   AggregateFunc = "MIN"
)

// AggregateColumn representa uma coluna com função de agregação.
type AggregateColumn struct {
	Func   AggregateFunc
	Column string
	Alias  string
}

// AggregateBuilder é o builder para queries de agregação type-safe.
type AggregateBuilder[T any] struct {
	executor      core.Executor
	dialect       core.Dialect
	logger        core.Logger
	tableName     string
	conditions    []interface{}
	aggregates    []AggregateColumn
	groupByFields []string
	havingConds   []interface{}
	orderBy       []OrderBy
	limit         *int
	offset        *int
	disableScopes bool
}

// AggregateResult contém os resultados de uma query de agregação.
type AggregateResult struct {
	values map[string]interface{}
}

// NewAggregateResult cria um novo AggregateResult com os valores fornecidos.
func NewAggregateResult(values map[string]interface{}) AggregateResult {
	return AggregateResult{values: values}
}

// Int64 retorna o valor como int64 para a chave especificada.
// Retorna 0 se a chave não existir ou o tipo não for compatível.
func (r AggregateResult) Int64(key string) int64 {
	if v, ok := r.values[key]; ok {
		switch val := v.(type) {
		case int64:
			return val
		case int:
			return int64(val)
		case int32:
			return int64(val)
		case float64:
			return int64(val)
		case float32:
			return int64(val)
		}
	}
	return 0
}

// Float64 retorna o valor como float64 para a chave especificada.
// Retorna 0 se a chave não existir ou o tipo não for compatível.
func (r AggregateResult) Float64(key string) float64 {
	if v, ok := r.values[key]; ok {
		switch val := v.(type) {
		case float64:
			return val
		case float32:
			return float64(val)
		case int64:
			return float64(val)
		case int:
			return float64(val)
		case int32:
			return float64(val)
		}
	}
	return 0
}

// String retorna o valor como string para a chave especificada.
// Retorna string vazia se a chave não existir ou o tipo não for compatível.
func (r AggregateResult) String(key string) string {
	if v, ok := r.values[key]; ok {
		switch val := v.(type) {
		case string:
			return val
		case []byte:
			return string(val)
		default:
			return fmt.Sprintf("%v", val)
		}
	}
	return ""
}

// Value retorna o valor raw para a chave especificada.
func (r AggregateResult) Value(key string) interface{} {
	return r.values[key]
}

// Has verifica se a chave existe no resultado.
func (r AggregateResult) Has(key string) bool {
	_, ok := r.values[key]
	return ok
}

// Keys retorna todas as chaves disponíveis no resultado.
func (r AggregateResult) Keys() []string {
	keys := make([]string, 0, len(r.values))
	for k := range r.values {
		keys = append(keys, k)
	}
	return keys
}

// Aggregate cria um novo AggregateBuilder a partir de um Builder existente.
// Herda condições WHERE do builder original.
//
// Exemplo:
//
//	result, _ := genus.Table[Order](db).
//	    Where(OrderFields.Status.Eq("paid")).
//	    Aggregate().
//	    CountAll().
//	    Sum("total").
//	    One(ctx)
func (b *Builder[T]) Aggregate() *AggregateBuilder[T] {
	return &AggregateBuilder[T]{
		executor:      b.executor,
		dialect:       b.dialect,
		logger:        b.logger,
		tableName:     b.tableName,
		conditions:    b.conditions,
		disableScopes: b.disableScopes,
	}
}

// clone cria uma cópia profunda do AggregateBuilder.
func (a *AggregateBuilder[T]) clone() *AggregateBuilder[T] {
	newBuilder := &AggregateBuilder[T]{
		executor:      a.executor,
		dialect:       a.dialect,
		logger:        a.logger,
		tableName:     a.tableName,
		disableScopes: a.disableScopes,
	}

	if len(a.conditions) > 0 {
		newBuilder.conditions = make([]interface{}, len(a.conditions))
		copy(newBuilder.conditions, a.conditions)
	}

	if len(a.aggregates) > 0 {
		newBuilder.aggregates = make([]AggregateColumn, len(a.aggregates))
		copy(newBuilder.aggregates, a.aggregates)
	}

	if len(a.groupByFields) > 0 {
		newBuilder.groupByFields = make([]string, len(a.groupByFields))
		copy(newBuilder.groupByFields, a.groupByFields)
	}

	if len(a.havingConds) > 0 {
		newBuilder.havingConds = make([]interface{}, len(a.havingConds))
		copy(newBuilder.havingConds, a.havingConds)
	}

	if len(a.orderBy) > 0 {
		newBuilder.orderBy = make([]OrderBy, len(a.orderBy))
		copy(newBuilder.orderBy, a.orderBy)
	}

	if a.limit != nil {
		limitVal := *a.limit
		newBuilder.limit = &limitVal
	}

	if a.offset != nil {
		offsetVal := *a.offset
		newBuilder.offset = &offsetVal
	}

	return newBuilder
}

// Where adiciona uma condição WHERE.
// IMUTÁVEL: Retorna um novo builder sem modificar o original.
func (a *AggregateBuilder[T]) Where(condition interface{}) *AggregateBuilder[T] {
	newBuilder := a.clone()
	newBuilder.conditions = append(newBuilder.conditions, condition)
	return newBuilder
}

// CountAll adiciona COUNT(*) à query.
// O resultado será acessível via result.Int64("count").
// IMUTÁVEL: Retorna um novo builder sem modificar o original.
func (a *AggregateBuilder[T]) CountAll() *AggregateBuilder[T] {
	newBuilder := a.clone()
	newBuilder.aggregates = append(newBuilder.aggregates, AggregateColumn{
		Func:   AggregateFuncCount,
		Column: "*",
		Alias:  "count",
	})
	return newBuilder
}

// Count adiciona COUNT(column) à query.
// O resultado será acessível via result.Int64("count_" + column).
// IMUTÁVEL: Retorna um novo builder sem modificar o original.
func (a *AggregateBuilder[T]) Count(column string) *AggregateBuilder[T] {
	newBuilder := a.clone()
	newBuilder.aggregates = append(newBuilder.aggregates, AggregateColumn{
		Func:   AggregateFuncCount,
		Column: column,
		Alias:  "count_" + sanitizeAlias(column),
	})
	return newBuilder
}

// Sum adiciona SUM(column) à query.
// O resultado será acessível via result.Float64("sum_" + column).
// IMUTÁVEL: Retorna um novo builder sem modificar o original.
func (a *AggregateBuilder[T]) Sum(column string) *AggregateBuilder[T] {
	newBuilder := a.clone()
	newBuilder.aggregates = append(newBuilder.aggregates, AggregateColumn{
		Func:   AggregateFuncSum,
		Column: column,
		Alias:  "sum_" + sanitizeAlias(column),
	})
	return newBuilder
}

// Avg adiciona AVG(column) à query.
// O resultado será acessível via result.Float64("avg_" + column).
// IMUTÁVEL: Retorna um novo builder sem modificar o original.
func (a *AggregateBuilder[T]) Avg(column string) *AggregateBuilder[T] {
	newBuilder := a.clone()
	newBuilder.aggregates = append(newBuilder.aggregates, AggregateColumn{
		Func:   AggregateFuncAvg,
		Column: column,
		Alias:  "avg_" + sanitizeAlias(column),
	})
	return newBuilder
}

// Max adiciona MAX(column) à query.
// O resultado será acessível via result.Value("max_" + column).
// IMUTÁVEL: Retorna um novo builder sem modificar o original.
func (a *AggregateBuilder[T]) Max(column string) *AggregateBuilder[T] {
	newBuilder := a.clone()
	newBuilder.aggregates = append(newBuilder.aggregates, AggregateColumn{
		Func:   AggregateFuncMax,
		Column: column,
		Alias:  "max_" + sanitizeAlias(column),
	})
	return newBuilder
}

// Min adiciona MIN(column) à query.
// O resultado será acessível via result.Value("min_" + column).
// IMUTÁVEL: Retorna um novo builder sem modificar o original.
func (a *AggregateBuilder[T]) Min(column string) *AggregateBuilder[T] {
	newBuilder := a.clone()
	newBuilder.aggregates = append(newBuilder.aggregates, AggregateColumn{
		Func:   AggregateFuncMin,
		Column: column,
		Alias:  "min_" + sanitizeAlias(column),
	})
	return newBuilder
}

// GroupBy adiciona colunas para agrupamento (GROUP BY).
// Colunas de grupo são incluídas automaticamente no resultado.
// IMUTÁVEL: Retorna um novo builder sem modificar o original.
func (a *AggregateBuilder[T]) GroupBy(columns ...string) *AggregateBuilder[T] {
	newBuilder := a.clone()
	newBuilder.groupByFields = append(newBuilder.groupByFields, columns...)
	return newBuilder
}

// Having adiciona uma condição HAVING.
// Usado para filtrar resultados agrupados baseado em funções de agregação.
// IMUTÁVEL: Retorna um novo builder sem modificar o original.
//
// Exemplo:
//
//	.Having(query.Condition{Field: "COUNT(*)", Operator: query.OpGt, Value: 5})
func (a *AggregateBuilder[T]) Having(condition interface{}) *AggregateBuilder[T] {
	newBuilder := a.clone()
	newBuilder.havingConds = append(newBuilder.havingConds, condition)
	return newBuilder
}

// OrderByAsc adiciona ORDER BY ASC.
// IMUTÁVEL: Retorna um novo builder sem modificar o original.
func (a *AggregateBuilder[T]) OrderByAsc(column string) *AggregateBuilder[T] {
	newBuilder := a.clone()
	newBuilder.orderBy = append(newBuilder.orderBy, OrderBy{Column: column, Desc: false})
	return newBuilder
}

// OrderByDesc adiciona ORDER BY DESC.
// IMUTÁVEL: Retorna um novo builder sem modificar o original.
func (a *AggregateBuilder[T]) OrderByDesc(column string) *AggregateBuilder[T] {
	newBuilder := a.clone()
	newBuilder.orderBy = append(newBuilder.orderBy, OrderBy{Column: column, Desc: true})
	return newBuilder
}

// Limit define o LIMIT.
// IMUTÁVEL: Retorna um novo builder sem modificar o original.
func (a *AggregateBuilder[T]) Limit(limit int) *AggregateBuilder[T] {
	newBuilder := a.clone()
	newBuilder.limit = &limit
	return newBuilder
}

// Offset define o OFFSET.
// IMUTÁVEL: Retorna um novo builder sem modificar o original.
func (a *AggregateBuilder[T]) Offset(offset int) *AggregateBuilder[T] {
	newBuilder := a.clone()
	newBuilder.offset = &offset
	return newBuilder
}

// One executa a query e retorna um único resultado de agregação.
// Útil para queries sem GROUP BY ou quando se espera apenas uma linha.
func (a *AggregateBuilder[T]) One(ctx context.Context) (AggregateResult, error) {
	query, args := a.buildQuery()

	start := time.Now()
	rows, err := a.executor.QueryContext(ctx, query, args...)
	duration := time.Since(start).Nanoseconds()

	if err != nil {
		a.logger.LogError(query, args, err)
		return AggregateResult{}, fmt.Errorf("failed to execute aggregate query: %w", err)
	}
	defer rows.Close()

	a.logger.LogQuery(query, args, duration)

	if !rows.Next() {
		return AggregateResult{}, sql.ErrNoRows
	}

	result, err := a.scanRow(rows)
	if err != nil {
		return AggregateResult{}, fmt.Errorf("failed to scan aggregate result: %w", err)
	}

	return result, nil
}

// All executa a query e retorna todos os resultados de agregação.
// Útil para queries com GROUP BY que retornam múltiplas linhas.
func (a *AggregateBuilder[T]) All(ctx context.Context) ([]AggregateResult, error) {
	query, args := a.buildQuery()

	start := time.Now()
	rows, err := a.executor.QueryContext(ctx, query, args...)
	duration := time.Since(start).Nanoseconds()

	if err != nil {
		a.logger.LogError(query, args, err)
		return nil, fmt.Errorf("failed to execute aggregate query: %w", err)
	}
	defer rows.Close()

	a.logger.LogQuery(query, args, duration)

	var results []AggregateResult
	for rows.Next() {
		result, err := a.scanRow(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan aggregate result: %w", err)
		}
		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return results, nil
}

// buildQuery constrói a query SQL de agregação.
func (a *AggregateBuilder[T]) buildQuery() (string, []interface{}) {
	var sb strings.Builder
	var args []interface{}

	// SELECT
	sb.WriteString("SELECT ")

	var selectParts []string

	// Adiciona GROUP BY columns primeiro
	for _, col := range a.groupByFields {
		selectParts = append(selectParts, a.dialect.QuoteIdentifier(col))
	}

	// Adiciona funções de agregação
	for _, agg := range a.aggregates {
		var aggSQL string
		if agg.Column == "*" {
			aggSQL = fmt.Sprintf("%s(*) AS %s", agg.Func, a.dialect.QuoteIdentifier(agg.Alias))
		} else {
			aggSQL = fmt.Sprintf("%s(%s) AS %s",
				agg.Func,
				a.dialect.QuoteIdentifier(agg.Column),
				a.dialect.QuoteIdentifier(agg.Alias))
		}
		selectParts = append(selectParts, aggSQL)
	}

	// Se não há agregações, usa COUNT(*)
	if len(selectParts) == 0 {
		selectParts = append(selectParts, "COUNT(*) AS count")
	}

	sb.WriteString(strings.Join(selectParts, ", "))

	// FROM
	sb.WriteString(" FROM ")
	sb.WriteString(a.dialect.QuoteIdentifier(a.tableName))

	// WHERE
	if len(a.conditions) > 0 {
		sb.WriteString(" WHERE ")
		whereSQL, whereArgs := a.buildWhereClause(a.conditions, 1)
		sb.WriteString(whereSQL)
		args = append(args, whereArgs...)
	}

	// GROUP BY
	if len(a.groupByFields) > 0 {
		sb.WriteString(" GROUP BY ")
		quotedFields := make([]string, len(a.groupByFields))
		for i, f := range a.groupByFields {
			quotedFields[i] = a.dialect.QuoteIdentifier(f)
		}
		sb.WriteString(strings.Join(quotedFields, ", "))
	}

	// HAVING
	if len(a.havingConds) > 0 {
		sb.WriteString(" HAVING ")
		argIndex := len(args) + 1
		havingSQL, havingArgs := a.buildWhereClause(a.havingConds, argIndex)
		sb.WriteString(havingSQL)
		args = append(args, havingArgs...)
	}

	// ORDER BY
	if len(a.orderBy) > 0 {
		sb.WriteString(" ORDER BY ")
		orderParts := make([]string, len(a.orderBy))
		for i, order := range a.orderBy {
			if order.Desc {
				orderParts[i] = order.Column + " DESC"
			} else {
				orderParts[i] = order.Column + " ASC"
			}
		}
		sb.WriteString(strings.Join(orderParts, ", "))
	}

	// LIMIT
	if a.limit != nil {
		sb.WriteString(fmt.Sprintf(" LIMIT %d", *a.limit))
	}

	// OFFSET
	if a.offset != nil {
		sb.WriteString(fmt.Sprintf(" OFFSET %d", *a.offset))
	}

	return sb.String(), args
}

// buildWhereClause constrói a cláusula WHERE/HAVING.
func (a *AggregateBuilder[T]) buildWhereClause(conditions []interface{}, startArgIndex int) (string, []interface{}) {
	if len(conditions) == 0 {
		return "", nil
	}

	var parts []string
	var args []interface{}
	argIndex := startArgIndex

	for _, cond := range conditions {
		switch c := cond.(type) {
		case Condition:
			sql, condArgs := a.buildCondition(c, &argIndex)
			parts = append(parts, sql)
			args = append(args, condArgs...)

		case ConditionGroup:
			sql, condArgs := a.buildConditionGroup(c, &argIndex)
			parts = append(parts, "("+sql+")")
			args = append(args, condArgs...)
		}
	}

	return strings.Join(parts, " AND "), args
}

// buildCondition constrói uma única condição.
func (a *AggregateBuilder[T]) buildCondition(cond Condition, argIndex *int) (string, []interface{}) {
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
			placeholders[i] = a.dialect.Placeholder(*argIndex)
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
			a.dialect.Placeholder(*argIndex),
			a.dialect.Placeholder(*argIndex+1))
		args = append(args, values[0], values[1])
		*argIndex += 2
		return sql, args

	default:
		sql := fmt.Sprintf("%s %s %s", cond.Field, cond.Operator, a.dialect.Placeholder(*argIndex))
		args = append(args, cond.Value)
		*argIndex++
		return sql, args
	}
}

// buildConditionGroup constrói um grupo de condições.
func (a *AggregateBuilder[T]) buildConditionGroup(group ConditionGroup, argIndex *int) (string, []interface{}) {
	var parts []string
	var args []interface{}

	for _, cond := range group.Conditions {
		switch c := cond.(type) {
		case Condition:
			sql, condArgs := a.buildCondition(c, argIndex)
			parts = append(parts, sql)
			args = append(args, condArgs...)

		case ConditionGroup:
			sql, condArgs := a.buildConditionGroup(c, argIndex)
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

// scanRow escaneia uma linha de resultado de agregação.
func (a *AggregateBuilder[T]) scanRow(rows *sql.Rows) (AggregateResult, error) {
	columns, err := rows.Columns()
	if err != nil {
		return AggregateResult{}, err
	}

	// Cria slice de ponteiros para os valores
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	if err := rows.Scan(valuePtrs...); err != nil {
		return AggregateResult{}, err
	}

	// Monta o map de resultados
	result := make(map[string]interface{})
	for i, col := range columns {
		// Remove quotes do nome da coluna se existirem
		cleanCol := strings.Trim(col, "\"'`")
		result[cleanCol] = values[i]
	}

	return AggregateResult{values: result}, nil
}

// sanitizeAlias limpa o nome da coluna para uso como alias.
func sanitizeAlias(column string) string {
	// Remove caracteres especiais e substitui pontos por underscore
	result := strings.ReplaceAll(column, ".", "_")
	result = strings.ReplaceAll(result, "-", "_")
	return result
}
