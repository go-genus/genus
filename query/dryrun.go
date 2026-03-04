package query

import (
	"context"
	"fmt"
	"strings"
)

// DryRunResult contém o resultado de uma operação dry run.
type DryRunResult struct {
	// SQL é a query SQL gerada.
	SQL string

	// Args são os argumentos da query.
	Args []interface{}

	// FormattedSQL é a query com argumentos interpolados (para debug).
	FormattedSQL string

	// Operation é o tipo de operação (SELECT, INSERT, UPDATE, DELETE).
	Operation string

	// Table é o nome da tabela.
	Table string

	// EstimatedRows é uma estimativa de linhas afetadas (quando disponível).
	EstimatedRows *int64
}

// String retorna uma representação formatada do resultado.
func (r DryRunResult) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Operation: %s\n", r.Operation))
	sb.WriteString(fmt.Sprintf("Table: %s\n", r.Table))
	sb.WriteString(fmt.Sprintf("SQL: %s\n", r.SQL))
	if len(r.Args) > 0 {
		sb.WriteString(fmt.Sprintf("Args: %v\n", r.Args))
	}
	if r.FormattedSQL != "" {
		sb.WriteString(fmt.Sprintf("Formatted: %s\n", r.FormattedSQL))
	}
	return sb.String()
}

// DryRunBuilder permite visualizar queries sem executá-las.
type DryRunBuilder[T any] struct {
	builder *Builder[T]
}

// DryRun retorna um builder para visualizar queries sem executar.
//
// Exemplo:
//
//	result := genus.Table[User](db).
//	    Where(UserFields.Age.Gt(18)).
//	    OrderByDesc("created_at").
//	    Limit(10).
//	    DryRun().
//	    Select()
//
//	fmt.Println(result.SQL)
//	// SELECT * FROM "users" WHERE age > $1 ORDER BY created_at DESC LIMIT 10
//	fmt.Println(result.Args)
//	// [18]
func (b *Builder[T]) DryRun() *DryRunBuilder[T] {
	return &DryRunBuilder[T]{
		builder: b.clone(),
	}
}

// Select retorna o SQL de uma query SELECT.
func (drb *DryRunBuilder[T]) Select() DryRunResult {
	sql, args := drb.builder.buildDryRunSelectQuery()
	return DryRunResult{
		SQL:          sql,
		Args:         args,
		FormattedSQL: formatSQL(sql, args),
		Operation:    "SELECT",
		Table:        drb.builder.tableName,
	}
}

// Count retorna o SQL de uma query COUNT.
func (drb *DryRunBuilder[T]) Count() DryRunResult {
	// Cria query de count
	b := drb.builder.clone()
	b.selectCols = []string{"COUNT(*)"}
	b.orderBy = nil
	b.limit = nil
	b.offset = nil

	sql, args := b.buildDryRunSelectQuery()
	return DryRunResult{
		SQL:          sql,
		Args:         args,
		FormattedSQL: formatSQL(sql, args),
		Operation:    "COUNT",
		Table:        drb.builder.tableName,
	}
}

// First retorna o SQL de uma query para buscar o primeiro registro.
func (drb *DryRunBuilder[T]) First() DryRunResult {
	b := drb.builder.clone()
	limit := 1
	b.limit = &limit

	sql, args := b.buildDryRunSelectQuery()
	return DryRunResult{
		SQL:          sql,
		Args:         args,
		FormattedSQL: formatSQL(sql, args),
		Operation:    "SELECT",
		Table:        drb.builder.tableName,
	}
}

// Explain retorna o SQL com EXPLAIN prefixado.
func (drb *DryRunBuilder[T]) Explain() DryRunResult {
	sql, args := drb.builder.buildDryRunSelectQuery()
	explainSQL := "EXPLAIN " + sql

	return DryRunResult{
		SQL:          explainSQL,
		Args:         args,
		FormattedSQL: formatSQL(explainSQL, args),
		Operation:    "EXPLAIN",
		Table:        drb.builder.tableName,
	}
}

// ExplainAnalyze retorna o SQL com EXPLAIN ANALYZE prefixado.
// Nota: EXPLAIN ANALYZE executa a query, use com cuidado.
func (drb *DryRunBuilder[T]) ExplainAnalyze() DryRunResult {
	sql, args := drb.builder.buildDryRunSelectQuery()
	explainSQL := "EXPLAIN ANALYZE " + sql

	return DryRunResult{
		SQL:          explainSQL,
		Args:         args,
		FormattedSQL: formatSQL(explainSQL, args),
		Operation:    "EXPLAIN ANALYZE",
		Table:        drb.builder.tableName,
	}
}

// formatSQL formata SQL com argumentos interpolados (apenas para debug).
func formatSQL(sql string, args []interface{}) string {
	if len(args) == 0 {
		return sql
	}

	result := sql
	for i, arg := range args {
		placeholder := fmt.Sprintf("$%d", i+1)
		// Tenta também placeholder ?
		if !strings.Contains(result, placeholder) {
			placeholder = "?"
		}

		var replacement string
		switch v := arg.(type) {
		case string:
			replacement = fmt.Sprintf("'%s'", strings.ReplaceAll(v, "'", "''"))
		case nil:
			replacement = "NULL"
		default:
			replacement = fmt.Sprintf("%v", v)
		}

		result = strings.Replace(result, placeholder, replacement, 1)
	}

	return result
}

// ToSQL retorna o SQL e argumentos da query atual.
func (b *Builder[T]) ToSQL() (string, []interface{}) {
	return b.buildDryRunSelectQuery()
}

// buildDryRunSelectQuery constrói a query SELECT completa para dry run.
func (b *Builder[T]) buildDryRunSelectQuery() (string, []interface{}) {
	var sql strings.Builder
	var args []interface{}

	// SELECT
	sql.WriteString("SELECT ")
	if len(b.selectCols) > 0 {
		sql.WriteString(strings.Join(b.selectCols, ", "))
	} else {
		sql.WriteString("*")
	}

	// FROM
	sql.WriteString(" FROM ")
	sql.WriteString(b.dialect.QuoteIdentifier(b.tableName))

	// JOINs
	for _, join := range b.joins {
		sql.WriteString(" ")
		sql.WriteString(string(join.Type))
		sql.WriteString(" JOIN ")
		sql.WriteString(b.dialect.QuoteIdentifier(join.Table))
		if join.Alias != "" {
			sql.WriteString(" AS ")
			sql.WriteString(join.Alias)
		}
		sql.WriteString(" ON ")
		sql.WriteString(join.Condition.LeftColumn)
		sql.WriteString(" = ")
		sql.WriteString(join.Condition.RightColumn)
	}

	// WHERE
	if len(b.conditions) > 0 {
		sql.WriteString(" WHERE ")
		whereSQL, whereArgs := b.buildWhereClause(b.conditions)
		sql.WriteString(whereSQL)
		args = append(args, whereArgs...)
	}

	// ORDER BY
	if len(b.orderBy) > 0 {
		sql.WriteString(" ORDER BY ")
		orderParts := make([]string, len(b.orderBy))
		for i, order := range b.orderBy {
			if order.Desc {
				orderParts[i] = order.Column + " DESC"
			} else {
				orderParts[i] = order.Column + " ASC"
			}
		}
		sql.WriteString(strings.Join(orderParts, ", "))
	}

	// LIMIT
	if b.limit != nil {
		sql.WriteString(fmt.Sprintf(" LIMIT %d", *b.limit))
	}

	// OFFSET
	if b.offset != nil {
		sql.WriteString(fmt.Sprintf(" OFFSET %d", *b.offset))
	}

	return sql.String(), args
}

// ExplainResult contém o resultado de um EXPLAIN.
type ExplainResult struct {
	Plan          string
	Cost          float64
	Rows          int64
	Width         int
	ActualTime    float64
	ActualRows    int64
	PlanningTime  float64
	ExecutionTime float64
}

// Explain executa EXPLAIN e retorna o plano de execução.
func (b *Builder[T]) Explain(ctx context.Context) ([]ExplainResult, error) {
	sql, args := b.buildDryRunSelectQuery()
	explainSQL := "EXPLAIN " + sql

	rows, err := b.executor.QueryContext(ctx, explainSQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []ExplainResult
	for rows.Next() {
		var plan string
		if err := rows.Scan(&plan); err != nil {
			return nil, err
		}
		results = append(results, ExplainResult{Plan: plan})
	}

	return results, rows.Err()
}

// ExplainAnalyze executa EXPLAIN ANALYZE e retorna o plano com estatísticas reais.
// CUIDADO: Esta função EXECUTA a query!
func (b *Builder[T]) ExplainAnalyze(ctx context.Context) ([]ExplainResult, error) {
	sql, args := b.buildDryRunSelectQuery()
	explainSQL := "EXPLAIN ANALYZE " + sql

	rows, err := b.executor.QueryContext(ctx, explainSQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []ExplainResult
	for rows.Next() {
		var plan string
		if err := rows.Scan(&plan); err != nil {
			return nil, err
		}
		results = append(results, ExplainResult{Plan: plan})
	}

	return results, rows.Err()
}
