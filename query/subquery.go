package query

import (
	"fmt"
	"strings"

	"github.com/go-genus/genus/core"
)

// Subquery representa uma subquery que pode ser usada em condições WHERE.
// É type-safe e permite encadear com o query builder principal.
type Subquery struct {
	query string
	args  []interface{}
}

// SubqueryBuilder é o builder para construir subqueries type-safe.
type SubqueryBuilder[T any] struct {
	builder *Builder[T]
	column  string // Coluna a ser selecionada (para IN/NOT IN)
}

// Subquery converte o builder em uma subquery.
// Por padrão, seleciona a coluna "id".
//
// Exemplo:
//
//	subquery := genus.Table[Post](db).
//	    Where(PostFields.UserID.Eq(1)).
//	    Subquery()
//
//	users, _ := genus.Table[User](db).
//	    Where(UserFields.ID.In(subquery)).
//	    Find(ctx)
func (b *Builder[T]) Subquery() *SubqueryBuilder[T] {
	return &SubqueryBuilder[T]{
		builder: b.clone(),
		column:  "id",
	}
}

// Column define qual coluna será selecionada na subquery.
// Por padrão é "id".
//
// Exemplo:
//
//	subquery := genus.Table[Order](db).
//	    Where(OrderFields.Status.Eq("paid")).
//	    Subquery().
//	    Column("user_id")
func (sb *SubqueryBuilder[T]) Column(column string) *SubqueryBuilder[T] {
	sb.column = column
	return sb
}

// Build constrói a SQL e argumentos da subquery.
func (sb *SubqueryBuilder[T]) Build() (string, []interface{}) {
	// Aplica soft delete scope se aplicável
	queryBuilder := applySoftDeleteScope(sb.builder)

	var sqlBuilder strings.Builder
	var args []interface{}

	// SELECT column FROM table
	sqlBuilder.WriteString("SELECT ")
	sqlBuilder.WriteString(sb.builder.dialect.QuoteIdentifier(sb.column))
	sqlBuilder.WriteString(" FROM ")
	sqlBuilder.WriteString(sb.builder.dialect.QuoteIdentifier(sb.builder.tableName))

	// WHERE conditions
	if len(queryBuilder.conditions) > 0 {
		sqlBuilder.WriteString(" WHERE ")
		whereSQL, whereArgs := queryBuilder.buildWhereClause(queryBuilder.conditions)
		sqlBuilder.WriteString(whereSQL)
		args = append(args, whereArgs...)
	}

	// ORDER BY (opcional para subqueries, mas útil com LIMIT)
	if len(queryBuilder.orderBy) > 0 {
		sqlBuilder.WriteString(" ORDER BY ")
		orderParts := make([]string, len(queryBuilder.orderBy))
		for i, order := range queryBuilder.orderBy {
			if order.Desc {
				orderParts[i] = order.Column + " DESC"
			} else {
				orderParts[i] = order.Column + " ASC"
			}
		}
		sqlBuilder.WriteString(strings.Join(orderParts, ", "))
	}

	// LIMIT
	if queryBuilder.limit != nil {
		sqlBuilder.WriteString(fmt.Sprintf(" LIMIT %d", *queryBuilder.limit))
	}

	// OFFSET
	if queryBuilder.offset != nil {
		sqlBuilder.WriteString(fmt.Sprintf(" OFFSET %d", *queryBuilder.offset))
	}

	return sqlBuilder.String(), args
}

// ToSubquery retorna o Subquery para uso em condições.
func (sb *SubqueryBuilder[T]) ToSubquery() *Subquery {
	query, args := sb.Build()
	return &Subquery{query: query, args: args}
}

// SQL retorna a SQL da subquery.
func (s *Subquery) SQL() string {
	return s.query
}

// Args retorna os argumentos da subquery.
func (s *Subquery) Args() []interface{} {
	return s.args
}

// SubqueryCondition representa uma condição que usa uma subquery.
type SubqueryCondition struct {
	Field    string
	Operator Operator
	Subquery *Subquery
}

// In cria uma condição WHERE field IN (subquery).
// O argumento pode ser um *SubqueryBuilder ou *Subquery.
func (f Int64Field) InSubquery(sb interface{}) SubqueryCondition {
	return createSubqueryCondition(f.column, OpIn, sb)
}

// NotInSubquery cria uma condição WHERE field NOT IN (subquery).
func (f Int64Field) NotInSubquery(sb interface{}) SubqueryCondition {
	return createSubqueryCondition(f.column, OpNotIn, sb)
}

// InSubquery para StringField
func (f StringField) InSubquery(sb interface{}) SubqueryCondition {
	return createSubqueryCondition(f.column, OpIn, sb)
}

// NotInSubquery para StringField
func (f StringField) NotInSubquery(sb interface{}) SubqueryCondition {
	return createSubqueryCondition(f.column, OpNotIn, sb)
}

// InSubquery para IntField
func (f IntField) InSubquery(sb interface{}) SubqueryCondition {
	return createSubqueryCondition(f.column, OpIn, sb)
}

// NotInSubquery para IntField
func (f IntField) NotInSubquery(sb interface{}) SubqueryCondition {
	return createSubqueryCondition(f.column, OpNotIn, sb)
}

// createSubqueryCondition é um helper para criar condições com subquery.
func createSubqueryCondition(field string, op Operator, sb interface{}) SubqueryCondition {
	var subquery *Subquery

	switch v := sb.(type) {
	case *Subquery:
		subquery = v
	case *SubqueryBuilder[any]:
		subquery = v.ToSubquery()
	default:
		// Tenta converter para SubqueryBuilder genérico usando reflection
		// Por simplicidade, assume que tem método ToSubquery
		subquery = &Subquery{}
	}

	return SubqueryCondition{
		Field:    field,
		Operator: op,
		Subquery: subquery,
	}
}

// ExistsCondition representa uma condição EXISTS (subquery).
type ExistsCondition struct {
	Subquery *Subquery
	Not      bool
}

// Exists cria uma condição EXISTS (subquery).
func Exists(sb interface{}) ExistsCondition {
	return ExistsCondition{
		Subquery: extractSubquery(sb),
		Not:      false,
	}
}

// NotExists cria uma condição NOT EXISTS (subquery).
func NotExists(sb interface{}) ExistsCondition {
	return ExistsCondition{
		Subquery: extractSubquery(sb),
		Not:      true,
	}
}

// extractSubquery extrai Subquery de diferentes tipos.
func extractSubquery(sb interface{}) *Subquery {
	switch v := sb.(type) {
	case *Subquery:
		return v
	default:
		return &Subquery{}
	}
}

// buildSubqueryCondition constrói SQL para SubqueryCondition.
func (b *Builder[T]) buildSubqueryCondition(cond SubqueryCondition, argIndex *int) (string, []interface{}) {
	subSQL := cond.Subquery.SQL()
	subArgs := cond.Subquery.Args()

	// Reescreve placeholders para continuar a sequência
	rewrittenSQL := b.rewritePlaceholders(subSQL, argIndex, len(subArgs))

	op := "IN"
	if cond.Operator == OpNotIn {
		op = "NOT IN"
	}

	sql := fmt.Sprintf("%s %s (%s)", cond.Field, op, rewrittenSQL)
	return sql, subArgs
}

// buildExistsCondition constrói SQL para ExistsCondition.
func (b *Builder[T]) buildExistsCondition(cond ExistsCondition, argIndex *int) (string, []interface{}) {
	subSQL := cond.Subquery.SQL()
	subArgs := cond.Subquery.Args()

	// Reescreve placeholders
	rewrittenSQL := b.rewritePlaceholders(subSQL, argIndex, len(subArgs))

	prefix := "EXISTS"
	if cond.Not {
		prefix = "NOT EXISTS"
	}

	sql := fmt.Sprintf("%s (%s)", prefix, rewrittenSQL)
	return sql, subArgs
}

// rewritePlaceholders reescreve $1, $2 ou ? para continuar a sequência de argumentos.
func (b *Builder[T]) rewritePlaceholders(sql string, argIndex *int, numArgs int) string {
	// Detecta se é placeholder PostgreSQL ($N) ou MySQL/SQLite (?)
	placeholder := b.dialect.Placeholder(1)

	if placeholder == "?" {
		// MySQL/SQLite - placeholders já são ?
		*argIndex += numArgs
		return sql
	}

	// PostgreSQL - precisa reescrever $1, $2, etc
	result := sql
	for i := numArgs; i >= 1; i-- {
		oldPlaceholder := fmt.Sprintf("$%d", i)
		newPlaceholder := fmt.Sprintf("$%d", *argIndex)
		result = strings.Replace(result, oldPlaceholder, newPlaceholder, 1)
		*argIndex++
	}

	return result
}

// RawSubquery cria uma subquery a partir de SQL raw.
// Use com cuidado - não há validação de tipo.
func RawSubquery(sql string, args ...interface{}) *Subquery {
	return &Subquery{
		query: sql,
		args:  args,
	}
}

// ScalarSubquery representa uma subquery que retorna um único valor.
// Útil para comparações como: WHERE price > (SELECT AVG(price) FROM products)
type ScalarSubquery struct {
	Subquery
}

// ScalarSubqueryBuilder constrói subqueries escalares.
type ScalarSubqueryBuilder[T any] struct {
	builder    *Builder[T]
	selectExpr string
}

// ScalarSubquery cria um builder para subquery escalar.
//
// Exemplo:
//
//	avgPrice := genus.Table[Product](db).
//	    ScalarSubquery("AVG(price)").
//	    ToScalar()
//
//	expensiveProducts, _ := genus.Table[Product](db).
//	    Where(ProductFields.Price.Gt(avgPrice)).
//	    Find(ctx)
func (b *Builder[T]) ScalarSubquery(selectExpr string) *ScalarSubqueryBuilder[T] {
	return &ScalarSubqueryBuilder[T]{
		builder:    b.clone(),
		selectExpr: selectExpr,
	}
}

// ToScalar constrói a subquery escalar.
func (ssb *ScalarSubqueryBuilder[T]) ToScalar() *ScalarSubquery {
	queryBuilder := applySoftDeleteScope(ssb.builder)

	var sqlBuilder strings.Builder
	var args []interface{}

	sqlBuilder.WriteString("SELECT ")
	sqlBuilder.WriteString(ssb.selectExpr)
	sqlBuilder.WriteString(" FROM ")
	sqlBuilder.WriteString(ssb.builder.dialect.QuoteIdentifier(ssb.builder.tableName))

	if len(queryBuilder.conditions) > 0 {
		sqlBuilder.WriteString(" WHERE ")
		whereSQL, whereArgs := queryBuilder.buildWhereClause(queryBuilder.conditions)
		sqlBuilder.WriteString(whereSQL)
		args = append(args, whereArgs...)
	}

	return &ScalarSubquery{
		Subquery: Subquery{
			query: sqlBuilder.String(),
			args:  args,
		},
	}
}

// SQL retorna a SQL da subquery escalar com parênteses.
func (s *ScalarSubquery) SQL() string {
	return "(" + s.query + ")"
}

// CorrelatedSubquery representa uma subquery correlacionada.
// A subquery referencia colunas da query externa.
type CorrelatedSubquery struct {
	Subquery
	correlation string // Expressão de correlação (ex: "outer.id = inner.user_id")
}

// CorrelatedSubqueryBuilder constrói subqueries correlacionadas.
type CorrelatedSubqueryBuilder[T any] struct {
	builder     *Builder[T]
	column      string
	correlation string
}

// CorrelatedSubquery cria um builder para subquery correlacionada.
//
// Exemplo:
//
//	// Usuários com pelo menos um post
//	subquery := genus.Table[Post](db).
//	    CorrelatedSubquery("id").
//	    Correlate("users.id = posts.user_id")
//
//	users, _ := genus.Table[User](db).
//	    Where(Exists(subquery.ToSubquery())).
//	    Find(ctx)
func (b *Builder[T]) CorrelatedSubquery(column string) *CorrelatedSubqueryBuilder[T] {
	return &CorrelatedSubqueryBuilder[T]{
		builder: b.clone(),
		column:  column,
	}
}

// Correlate define a expressão de correlação.
func (csb *CorrelatedSubqueryBuilder[T]) Correlate(expr string) *CorrelatedSubqueryBuilder[T] {
	csb.correlation = expr
	return csb
}

// ToSubquery constrói a subquery correlacionada.
func (csb *CorrelatedSubqueryBuilder[T]) ToSubquery() *Subquery {
	queryBuilder := applySoftDeleteScope(csb.builder)

	var sqlBuilder strings.Builder
	var args []interface{}

	sqlBuilder.WriteString("SELECT ")
	sqlBuilder.WriteString(csb.builder.dialect.QuoteIdentifier(csb.column))
	sqlBuilder.WriteString(" FROM ")
	sqlBuilder.WriteString(csb.builder.dialect.QuoteIdentifier(csb.builder.tableName))

	// Adiciona correlação como primeira condição
	sqlBuilder.WriteString(" WHERE ")

	if csb.correlation != "" {
		sqlBuilder.WriteString(csb.correlation)
		if len(queryBuilder.conditions) > 0 {
			sqlBuilder.WriteString(" AND ")
		}
	}

	if len(queryBuilder.conditions) > 0 {
		whereSQL, whereArgs := queryBuilder.buildWhereClause(queryBuilder.conditions)
		sqlBuilder.WriteString(whereSQL)
		args = append(args, whereArgs...)
	}

	return &Subquery{
		query: sqlBuilder.String(),
		args:  args,
	}
}

// Funções helper para usar subqueries em Where

// WhereInSubquery adiciona condição WHERE field IN (subquery).
func (b *Builder[T]) WhereInSubquery(field string, subquery *Subquery) *Builder[T] {
	return b.whereSubquery(field, OpIn, subquery)
}

// WhereNotInSubquery adiciona condição WHERE field NOT IN (subquery).
func (b *Builder[T]) WhereNotInSubquery(field string, subquery *Subquery) *Builder[T] {
	return b.whereSubquery(field, OpNotIn, subquery)
}

// WhereExists adiciona condição WHERE EXISTS (subquery).
func (b *Builder[T]) WhereExists(subquery *Subquery) *Builder[T] {
	newBuilder := b.clone()
	newBuilder.conditions = append(newBuilder.conditions, ExistsCondition{
		Subquery: subquery,
		Not:      false,
	})
	return newBuilder
}

// WhereNotExists adiciona condição WHERE NOT EXISTS (subquery).
func (b *Builder[T]) WhereNotExists(subquery *Subquery) *Builder[T] {
	newBuilder := b.clone()
	newBuilder.conditions = append(newBuilder.conditions, ExistsCondition{
		Subquery: subquery,
		Not:      true,
	})
	return newBuilder
}

func (b *Builder[T]) whereSubquery(field string, op Operator, subquery *Subquery) *Builder[T] {
	newBuilder := b.clone()
	newBuilder.conditions = append(newBuilder.conditions, SubqueryCondition{
		Field:    field,
		Operator: op,
		Subquery: subquery,
	})
	return newBuilder
}

// Extensão do buildWhereClause para suportar subqueries
func (b *Builder[T]) buildConditionWithSubquery(cond interface{}, argIndex *int) (string, []interface{}) {
	switch c := cond.(type) {
	case SubqueryCondition:
		return b.buildSubqueryCondition(c, argIndex)
	case ExistsCondition:
		return b.buildExistsCondition(c, argIndex)
	default:
		return "", nil
	}
}

// GetDialect retorna o dialect do builder.
func (b *Builder[T]) GetDialect() core.Dialect {
	return b.dialect
}
